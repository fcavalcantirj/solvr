package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	apimiddleware "github.com/fcavalcantirj/solvr/internal/api/middleware"
	"github.com/fcavalcantirj/solvr/internal/auth"
	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/hub"
	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/go-chi/chi/v5"
)

// maxMessageContentLen is the maximum allowed content length for a message (64KB).
const maxMessageContentLen = 65536

// invalidEntryReferenceMsg is returned when a reply, supersede or addressed participant
// does not belong to the posting room.
const invalidEntryReferenceMsg = "reply_to_entry_id, supersedes_entry_id and addressed_member_ids must reference this room"

// archivedRoomMessage explains why a write to a finished room is refused. Shared by
// the agent and human message gates so the recovery instruction is consistent.
const archivedRoomMessage = "this room is finished; an owner must reopen it before new messages can be posted"

// RoomMessagesHandler handles HTTP requests for room message operations.
// The presenceRepo field supports D-28 (implicit heartbeat on message posting).
type RoomMessagesHandler struct {
	msgRepo      *db.MessageRepository
	roomRepo     *db.RoomRepository
	presenceRepo *db.AgentPresenceRepository
	eventRepo    *db.RoomEventRepository
	hubMgr       *hub.HubManager
	// funnel records the first_two_way_exchange connection-funnel step. Optional:
	// nil when the funnel is not wired, in which case posting records nothing extra.
	funnel *db.FunnelEventRepository

	// testRoomLookup overrides room-by-slug lookup in unit tests (nil in production).
	testRoomLookup func(ctx context.Context, slug string) (*models.Room, error)
	// testMsgCreate overrides message creation in unit tests (nil in production).
	testMsgCreate func(ctx context.Context, params models.CreateMessageParams) (*models.Message, error)
}

// NewRoomMessagesHandler creates a new RoomMessagesHandler with all required dependencies.
// The presenceRepo is needed for D-28: message posting implicitly renews agent presence.
// The eventRepo records the two-way exchange milestone the homepage reports.
func NewRoomMessagesHandler(
	msgRepo *db.MessageRepository,
	roomRepo *db.RoomRepository,
	presenceRepo *db.AgentPresenceRepository,
	eventRepo *db.RoomEventRepository,
	hubMgr *hub.HubManager,
) *RoomMessagesHandler {
	return &RoomMessagesHandler{
		msgRepo:      msgRepo,
		roomRepo:     roomRepo,
		presenceRepo: presenceRepo,
		eventRepo:    eventRepo,
		hubMgr:       hubMgr,
	}
}

// SetFunnelRecorder wires the connection-funnel recorder so PostMessage records
// the server-side first_two_way_exchange step. Optional: with no recorder wired,
// message posting behaves exactly as before.
func (h *RoomMessagesHandler) SetFunnelRecorder(funnel *db.FunnelEventRepository) {
	h.funnel = funnel
}

// recordActivationMilestone marks the room the first time it carries a two-way
// exchange, which is what the homepage's "rooms with two-way exchanges" counts,
// and records the parallel first_two_way_exchange connection-funnel step so the
// funnel and the homepage agree on when a room activated.
//
// It is deliberately best-effort: a room whose milestone could not be written
// is a statistic that is briefly short, never a message that failed to post.
func (h *RoomMessagesHandler) recordActivationMilestone(ctx context.Context, room *models.Room) {
	if room == nil {
		return
	}
	if h.eventRepo != nil {
		if _, err := h.eventRepo.RecordActivation(ctx, room.ID); err != nil {
			slog.Error("failed to record room activation milestone", "error", err, "room_id", room.ID)
		}
	}
	if h.funnel != nil {
		if _, err := h.funnel.RecordFirstTwoWayExchange(ctx, room.ID); err != nil {
			slog.Warn("failed to record first_two_way_exchange funnel step", "error", err, "room_id", room.ID)
		}
	}
}

// postMessageRequest is the JSON body for POST /r/{slug}/message.
type postMessageRequest struct {
	AgentName          string          `json:"agent_name"`
	Content            string          `json:"content"`
	ContentType        string          `json:"content_type,omitempty"`
	Metadata           json.RawMessage `json:"metadata,omitempty"`
	ReplyToEntryID     *int64          `json:"reply_to_entry_id,omitempty"`
	AddressedMemberIDs json.RawMessage `json:"addressed_member_ids,omitempty"`
	// SupersedesEntryID marks this message as a revised directive/result that
	// explicitly supersedes an earlier message in the same room.
	SupersedesEntryID *int64 `json:"supersedes_entry_id,omitempty"`
	// ClientEntryID is the canonical idempotency key for a write. ClientMessageID is
	// the legacy field name, adapted to ClientEntryID when the canonical one is absent.
	ClientEntryID   *string `json:"client_entry_id,omitempty"`
	ClientMessageID *string `json:"client_message_id,omitempty"`
}

// postHumanMessageRequest is the JSON body for POST /v1/rooms/{slug}/messages (human comment).
type postHumanMessageRequest struct {
	Content            string          `json:"content"`
	ReplyToEntryID     *int64          `json:"reply_to_entry_id,omitempty"`
	AddressedMemberIDs json.RawMessage `json:"addressed_member_ids,omitempty"`
}

// PostMessage handles POST /r/{slug}/message: the legacy agent transport adapter into
// submitMessage, the same submission path as POST /v1/rooms/{slug}/entries.
// Requires a per-agent room token via BearerGuard (D-17). The body's agent_name is kept
// as the display label; author_id always comes from the token. client_message_id is
// adapted to the canonical client_entry_id when only the legacy field is present.
func (h *RoomMessagesHandler) PostMessage(w http.ResponseWriter, r *http.Request) {
	room := apimiddleware.RoomFromContext(r.Context())
	if room == nil {
		roomWriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "room context missing")
		return
	}
	if room.IsArchived() {
		roomWriteError(w, http.StatusConflict, "ROOM_ARCHIVED", archivedRoomMessage)
		return
	}

	var req postMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		roomWriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid request body")
		return
	}
	if req.AgentName == "" {
		roomWriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "agent_name is required")
		return
	}

	sub := messageSubmission{
		AuthorType:         "agent",
		Label:              req.AgentName,
		Content:            req.Content,
		ContentType:        req.ContentType,
		Metadata:           req.Metadata,
		ReplyToEntryID:     req.ReplyToEntryID,
		AddressedMemberIDs: req.AddressedMemberIDs,
		SupersedesEntryID:  req.SupersedesEntryID,
		ClientEntryID:      req.ClientEntryID,
	}
	if sub.ClientEntryID == nil || *sub.ClientEntryID == "" {
		sub.ClientEntryID = req.ClientMessageID
	}
	// Mission #3: the per-agent room token's agent id is the authoritative author.
	if authAgentID := apimiddleware.RoomAgentIDFromContext(r.Context()); authAgentID != "" {
		sub.AuthorID = &authAgentID
	}

	msg, created, serr := h.submitMessage(r.Context(), room, sub)
	if serr != nil {
		serr.write(w)
		return
	}
	if !created {
		roomWriteJSON(w, http.StatusOK, map[string]interface{}{
			"data":              msg,
			"idempotent_replay": true,
		})
		return
	}
	roomWriteJSON(w, http.StatusCreated, map[string]interface{}{"data": msg})
}

// PostHumanMessage handles POST /v1/rooms/{slug}/messages: the human comment adapter
// into submitMessage. Requires Solvr JWT authentication; the author is taken from the
// JWT claims server-side (T-16-03) and content is always plain text (D-26).
func (h *RoomMessagesHandler) PostHumanMessage(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromContext(r.Context())
	if claims == nil {
		roomWriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}

	slug := chi.URLParam(r, "slug")
	if slug == "" {
		roomWriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "slug is required")
		return
	}

	room, err := h.resolveRoomBySlug(r.Context(), slug)
	if err != nil {
		if errors.Is(err, db.ErrRoomNotFound) {
			roomWriteError(w, http.StatusNotFound, "NOT_FOUND", "room not found")
			return
		}
		slog.Error("failed to get room for human message", "error", err, "slug", slug)
		roomWriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get room")
		return
	}
	if room.IsArchived() {
		roomWriteError(w, http.StatusConflict, "ROOM_ARCHIVED", archivedRoomMessage)
		return
	}

	var req postHumanMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		roomWriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid request body")
		return
	}

	authorID := claims.UserID
	msg, _, serr := h.submitMessage(r.Context(), room, messageSubmission{
		AuthorType:         "human",
		AuthorID:           &authorID,
		Label:              "human:" + claims.UserID, // deterministic, not displayed
		Content:            req.Content,
		ContentType:        "text",
		ReplyToEntryID:     req.ReplyToEntryID,
		AddressedMemberIDs: req.AddressedMemberIDs,
	})
	if serr != nil {
		serr.write(w)
		return
	}
	roomWriteJSON(w, http.StatusCreated, map[string]interface{}{"data": msg})
}

// resolveRoomBySlug looks up a room by slug, using testRoomLookup in unit tests.
func (h *RoomMessagesHandler) resolveRoomBySlug(ctx context.Context, slug string) (*models.Room, error) {
	if h.testRoomLookup != nil {
		return h.testRoomLookup(ctx, slug)
	}
	return h.roomRepo.GetBySlug(ctx, slug)
}

// ListMessages handles GET /r/{slug}/messages and GET /v1/rooms/{slug}/messages.
// Supports cursor-based pagination per D-35: ?after=<message_id>&limit=<n>.
func (h *RoomMessagesHandler) ListMessages(w http.ResponseWriter, r *http.Request) {
	// Room can come from BearerGuard (A2A route) or direct lookup (public route)
	room := apimiddleware.RoomFromContext(r.Context())
	if room == nil {
		// Public route: look up by slug
		slug := chi.URLParam(r, "slug")
		if slug == "" {
			roomWriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "slug is required")
			return
		}
		var err error
		room, err = h.roomRepo.GetBySlug(r.Context(), slug)
		if err != nil {
			roomWriteError(w, http.StatusNotFound, "NOT_FOUND", "room not found")
			return
		}
	}

	// Parse pagination params. `after` pages forward for cursor polling/replay;
	// `before` pages backward to load earlier history anchored on a known id.
	var afterID, beforeID int64
	limit := 100
	if a := r.URL.Query().Get("after"); a != "" {
		if parsed, err := strconv.ParseInt(a, 10, 64); err == nil && parsed > 0 {
			afterID = parsed
		}
	}
	if b := r.URL.Query().Get("before"); b != "" {
		if parsed, err := strconv.ParseInt(b, 10, 64); err == nil && parsed > 0 {
			beforeID = parsed
		}
	}
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 500 {
			limit = parsed
		}
	}

	var messages []models.Message
	var err error
	switch {
	case afterID > 0:
		messages, err = h.msgRepo.ListAfter(r.Context(), room.ID, afterID, limit)
	case beforeID > 0:
		messages, err = h.msgRepo.ListBefore(r.Context(), room.ID, beforeID, limit)
	default:
		messages, err = h.msgRepo.ListRecent(r.Context(), room.ID, limit)
	}
	if err != nil {
		slog.Error("failed to list messages", "error", err, "room_id", room.ID)
		roomWriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to list messages")
		return
	}

	// Build cursor info
	var nextCursor *int64
	if len(messages) > 0 {
		lastID := messages[len(messages)-1].ID
		nextCursor = &lastID
	}

	_ = nextCursor // Available for future pagination header support

	response := map[string]interface{}{
		"data": messages,
	}
	roomWriteJSON(w, http.StatusOK, response)
}

// GetMessage handles GET /v1/rooms/{slug}/messages/{id} and GET /r/{slug}/messages/{id}.
// It is the stable single-message lookup deep links rely on: it returns the correct
// message even when it falls outside the initial recent-history page. Room access is
// enforced upstream (access guard / bearer guard) and GetByID is room-scoped, so a
// deep link can never fetch another room's message.
func (h *RoomMessagesHandler) GetMessage(w http.ResponseWriter, r *http.Request) {
	// Room can come from BearerGuard (A2A route) or the read access guard; fall back
	// to a slug lookup on the public REST route.
	room := apimiddleware.RoomFromContext(r.Context())
	if room == nil {
		slug := chi.URLParam(r, "slug")
		if slug == "" {
			roomWriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "slug is required")
			return
		}
		var err error
		room, err = h.roomRepo.GetBySlug(r.Context(), slug)
		if err != nil {
			roomWriteError(w, http.StatusNotFound, "NOT_FOUND", "room not found")
			return
		}
	}

	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		roomWriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid message id")
		return
	}

	msg, err := h.msgRepo.GetByID(r.Context(), room.ID, id)
	if err != nil {
		if errors.Is(err, db.ErrMessageNotFound) {
			roomWriteError(w, http.StatusNotFound, "NOT_FOUND", "message not found")
			return
		}
		slog.Error("failed to get message", "error", err, "room_id", room.ID, "message_id", id)
		roomWriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get message")
		return
	}

	roomWriteJSON(w, http.StatusOK, map[string]interface{}{"data": msg})
}

// PinMessage handles POST /r/{slug}/messages/{id}/pin. An authenticated room
// participant pins a directive or result so participants can retrieve it without
// scanning the transcript. Idempotent; room access is enforced upstream by
// BearerGuard, so a caller here already holds a valid room token.
func (h *RoomMessagesHandler) PinMessage(w http.ResponseWriter, r *http.Request) {
	h.setMessagePinned(w, r, true)
}

// UnpinMessage handles DELETE /r/{slug}/messages/{id}/pin.
func (h *RoomMessagesHandler) UnpinMessage(w http.ResponseWriter, r *http.Request) {
	h.setMessagePinned(w, r, false)
}

// setMessagePinned pins or unpins a message scoped to the bearer-authenticated room.
func (h *RoomMessagesHandler) setMessagePinned(w http.ResponseWriter, r *http.Request, pin bool) {
	room := apimiddleware.RoomFromContext(r.Context())
	if room == nil {
		roomWriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "room context missing")
		return
	}

	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		roomWriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid message id")
		return
	}

	var msg *models.Message
	if pin {
		msg, err = h.msgRepo.Pin(r.Context(), room.ID, id)
	} else {
		msg, err = h.msgRepo.Unpin(r.Context(), room.ID, id)
	}
	if err != nil {
		if errors.Is(err, db.ErrMessageNotFound) {
			roomWriteError(w, http.StatusNotFound, "NOT_FOUND", "message not found")
			return
		}
		slog.Error("failed to set message pin", "error", err, "room_id", room.ID, "message_id", id)
		roomWriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to update pin")
		return
	}
	publishPinChange(r.Context(), h.hubMgr, h.msgRepo, room, msg)

	roomWriteJSON(w, http.StatusOK, map[string]interface{}{"data": msg})
}

// ListPinnedMessages handles GET /r/{slug}/pins. Returns the room's pinned
// directives/results newest-first. Room access is enforced upstream.
func (h *RoomMessagesHandler) ListPinnedMessages(w http.ResponseWriter, r *http.Request) {
	room := apimiddleware.RoomFromContext(r.Context())
	if room == nil {
		slug := chi.URLParam(r, "slug")
		if slug == "" {
			roomWriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "slug is required")
			return
		}
		var err error
		room, err = h.roomRepo.GetBySlug(r.Context(), slug)
		if err != nil {
			roomWriteError(w, http.StatusNotFound, "NOT_FOUND", "room not found")
			return
		}
	}

	pinned, err := h.msgRepo.ListPinned(r.Context(), room.ID)
	if err != nil {
		slog.Error("failed to list pinned messages", "error", err, "room_id", room.ID)
		roomWriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to list pinned messages")
		return
	}

	roomWriteJSON(w, http.StatusOK, map[string]interface{}{"data": pinned})
}
