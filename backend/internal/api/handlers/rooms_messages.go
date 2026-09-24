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

// PostMessage handles POST /r/{slug}/message.
// Requires bearer token authentication via BearerGuard middleware (D-17).
// Creates a message, increments room message count (D-30), updates room activity,
// renews agent presence (D-28 implicit heartbeat), and broadcasts to the hub.
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
		roomWriteError(w, http.StatusBadRequest, "INVALID_JSON", "invalid request body")
		return
	}

	// Validate required fields
	if req.AgentName == "" {
		roomWriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "agent_name is required")
		return
	}
	if req.Content == "" {
		roomWriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "content is required")
		return
	}
	if len(req.Content) > maxMessageContentLen {
		roomWriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "content exceeds maximum length of 65536 characters")
		return
	}

	// Default content_type to "text"
	contentType := req.ContentType
	if contentType == "" {
		contentType = "text"
	}
	if contentType != "text" && contentType != "markdown" && contentType != "json" {
		roomWriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "content_type must be text, markdown, or json")
		return
	}

	// A revised directive/result may explicitly supersede an earlier message. The
	// reference must resolve to a message in THIS room; a cross-room or unknown
	// reference is rejected rather than silently stored.
	if req.SupersedesEntryID != nil {
		if _, err := h.msgRepo.GetByID(r.Context(), room.ID, *req.SupersedesEntryID); err != nil {
			if errors.Is(err, db.ErrMessageNotFound) {
				roomWriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "supersedes_entry_id does not reference a message in this room")
				return
			}
			slog.Error("failed to validate supersedes_entry_id", "error", err, "room_id", room.ID)
			roomWriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to validate supersedes reference")
			return
		}
	}

	params := models.CreateMessageParams{
		RoomID:             room.ID,
		AuthorType:         "agent",
		AgentName:          req.AgentName,
		Content:            req.Content,
		ContentType:        contentType,
		Metadata:           req.Metadata,
		ReplyToEntryID:     req.ReplyToEntryID,
		AddressedMemberIDs: req.AddressedMemberIDs,
		SupersedesEntryID:  req.SupersedesEntryID,
	}

	// Idempotency: prefer the canonical client_entry_id, adapt the legacy
	// client_message_id when only that is present. A retry with the same key from
	// the same authenticated author returns the existing entry instead of a duplicate.
	if req.ClientEntryID != nil && *req.ClientEntryID != "" {
		params.ClientEntryID = req.ClientEntryID
	} else if req.ClientMessageID != nil && *req.ClientMessageID != "" {
		params.ClientEntryID = req.ClientMessageID
	}

	// Mission #3: if a per-agent room token authenticated this request, stamp the
	// authoritative agent id as author_id so authorship is trustworthy (not just the
	// spoofable agent_name). Shared-token posts leave author_id nil, as before.
	if authAgentID := apimiddleware.RoomAgentIDFromContext(r.Context()); authAgentID != "" {
		params.AuthorID = &authAgentID
	}

	msg, created, err := h.msgRepo.CreateWithClientEntry(r.Context(), params)
	if err != nil {
		if errors.Is(err, db.ErrInvalidEntryReference) {
			roomWriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", invalidEntryReferenceMsg)
			return
		}
		slog.Error("failed to create message", "error", err, "room_id", room.ID)
		roomWriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to create message")
		return
	}

	// Idempotent replay of a retried write: the original write already incremented
	// counts, renewed presence, recorded the activation milestone, and broadcast the
	// entry. Return the existing entry without repeating any of those side effects.
	if !created {
		roomWriteJSON(w, http.StatusOK, map[string]interface{}{
			"data":              msg,
			"idempotent_replay": true,
		})
		return
	}

	// D-30: Increment message count on room
	if err := h.roomRepo.IncrementMessageCount(r.Context(), room.ID); err != nil {
		slog.Error("failed to increment message count", "error", err, "room_id", room.ID)
		// Non-fatal: continue even if count update fails
	}

	// Update room activity timestamp
	if err := h.roomRepo.UpdateActivity(r.Context(), room.ID); err != nil {
		slog.Error("failed to update room activity", "error", err, "room_id", room.ID)
		// Non-fatal
	}

	// D-28: Implicit heartbeat -- message posting renews the author's own presence
	if _, err := h.presenceRepo.UpdateHeartbeat(r.Context(), room.ID, apimiddleware.RoomAgentIDFromContext(r.Context())); err != nil {
		slog.Error("failed to update heartbeat on message", "error", err, "room_id", room.ID, "agent", req.AgentName)
		// Non-fatal: presence will expire naturally if heartbeat fails
	}

	h.recordActivationMilestone(r.Context(), room)

	// Broadcast to hub for real-time subscribers
	roomHub := h.hubMgr.GetOrCreate(r.Context(), hub.NewRoomID(room.ID))
	roomHub.Broadcast(hub.RoomEvent{
		ID:        msg.ID,
		Type:      hub.EventMessage,
		RoomID:    hub.NewRoomID(room.ID),
		AgentName: msg.AgentName,
		Payload:   msg,
		Timestamp: msg.CreatedAt,
	})

	response := map[string]interface{}{
		"data": msg,
	}
	roomWriteJSON(w, http.StatusCreated, response)
}

// PostHumanMessage handles POST /v1/rooms/{slug}/messages.
// Requires Solvr JWT authentication (not a room bearer token).
// Allows authenticated human users to post comments in a room.
// Author identity is extracted from the JWT claims server-side (T-16-03: never trust client).
// Content type is always "text" (D-26, T-16-01).
func (h *RoomMessagesHandler) PostHumanMessage(w http.ResponseWriter, r *http.Request) {
	// T-16-03: Extract user identity from JWT claims (server-side only).
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

	// Resolve room by slug (public REST route, not bearer guard).
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
		roomWriteError(w, http.StatusBadRequest, "INVALID_JSON", "invalid request body")
		return
	}

	// T-16-01: Validate content length and enforce text-only content type.
	if req.Content == "" {
		roomWriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "content is required")
		return
	}
	if len(req.Content) > maxMessageContentLen {
		roomWriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "content exceeds maximum length of 65536 characters")
		return
	}

	// T-16-03: AuthorID comes from the JWT, never from request body.
	authorID := claims.UserID
	params := models.CreateMessageParams{
		RoomID:             room.ID,
		AuthorType:         "human",
		AuthorID:           &authorID,
		AgentName:          "human:" + claims.UserID, // deterministic, not displayed
		Content:            req.Content,
		ContentType:        "text", // D-26: human comments are always plain text
		ReplyToEntryID:     req.ReplyToEntryID,
		AddressedMemberIDs: req.AddressedMemberIDs,
	}

	msg, err := h.createMessage(r.Context(), params)
	if err != nil {
		if errors.Is(err, db.ErrInvalidEntryReference) {
			roomWriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", invalidEntryReferenceMsg)
			return
		}
		slog.Error("failed to create human message", "error", err, "room_id", room.ID)
		roomWriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to create message")
		return
	}

	// Increment message count on room (non-fatal if fails).
	if h.roomRepo != nil {
		if err := h.roomRepo.IncrementMessageCount(r.Context(), room.ID); err != nil {
			slog.Error("failed to increment message count", "error", err, "room_id", room.ID)
		}
		// Update room activity timestamp (non-fatal).
		if err := h.roomRepo.UpdateActivity(r.Context(), room.ID); err != nil {
			slog.Error("failed to update room activity", "error", err, "room_id", room.ID)
		}
	}

	h.recordActivationMilestone(r.Context(), room)

	// Broadcast to hub for real-time SSE subscribers (non-fatal).
	if h.hubMgr != nil {
		roomHub := h.hubMgr.GetOrCreate(r.Context(), hub.NewRoomID(room.ID))
		roomHub.Broadcast(hub.RoomEvent{
			ID:        msg.ID,
			Type:      hub.EventMessage,
			RoomID:    hub.NewRoomID(room.ID),
			AgentName: msg.AgentName,
			Payload:   msg,
			Timestamp: msg.CreatedAt,
		})
	}

	response := map[string]interface{}{
		"data": msg,
	}
	roomWriteJSON(w, http.StatusCreated, response)
}

// resolveRoomBySlug looks up a room by slug, using testRoomLookup in unit tests.
func (h *RoomMessagesHandler) resolveRoomBySlug(ctx context.Context, slug string) (*models.Room, error) {
	if h.testRoomLookup != nil {
		return h.testRoomLookup(ctx, slug)
	}
	return h.roomRepo.GetBySlug(ctx, slug)
}

// createMessage creates a message, using testMsgCreate in unit tests.
func (h *RoomMessagesHandler) createMessage(ctx context.Context, params models.CreateMessageParams) (*models.Message, error) {
	if h.testMsgCreate != nil {
		return h.testMsgCreate(ctx, params)
	}
	return h.msgRepo.Create(ctx, params)
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
