package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	apimiddleware "github.com/fcavalcantirj/solvr/internal/api/middleware"
	"github.com/fcavalcantirj/solvr/internal/auth"
	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// RoomHandler handles HTTP requests for room CRUD operations.
type RoomHandler struct {
	roomRepo       *db.RoomRepository
	msgRepo        *db.MessageRepository
	presenceRepo   *db.AgentPresenceRepository
	memberRepo     *db.RoomMemberRepository
	agentTokenRepo *db.RoomAgentTokenRepository
	eventRepo      *db.RoomEventRepository
	// funnel records the room_created connection-funnel step. Optional: nil in
	// unit tests and wherever the funnel is not wired, in which case it is a no-op.
	funnel *db.FunnelEventRepository
}

// SetFunnelRecorder wires the connection-funnel recorder so CreateRoom records
// the server-side room_created step. Optional (like SetPostLookup): with no
// recorder wired, room creation records nothing and behaves exactly as before.
func (h *RoomHandler) SetFunnelRecorder(funnel *db.FunnelEventRepository) {
	h.funnel = funnel
}

// NewRoomHandler creates a new RoomHandler with the required repositories.
func NewRoomHandler(
	roomRepo *db.RoomRepository,
	msgRepo *db.MessageRepository,
	presenceRepo *db.AgentPresenceRepository,
	memberRepo *db.RoomMemberRepository,
	agentTokenRepo *db.RoomAgentTokenRepository,
	eventRepo *db.RoomEventRepository,
) *RoomHandler {
	return &RoomHandler{
		roomRepo:       roomRepo,
		msgRepo:        msgRepo,
		presenceRepo:   presenceRepo,
		memberRepo:     memberRepo,
		agentTokenRepo: agentTokenRepo,
		eventRepo:      eventRepo,
	}
}

// createRoomRequest is the JSON body for POST /v1/rooms.
type createRoomRequest struct {
	DisplayName string   `json:"display_name"`
	Description *string  `json:"description,omitempty"`
	Category    *string  `json:"category,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	Slug        string   `json:"slug,omitempty"`
	IsPrivate   bool     `json:"is_private"`
	// SourcePostID, when present, records the published Post this room was seeded from
	// ("Discuss with agents"). It is a provenance pointer only — the post's content is
	// never copied into the room. Must be a valid UUID when supplied.
	SourcePostID *string `json:"source_post_id,omitempty"`
	// FlowID is the non-secret connection-funnel identifier the planner prompt carried
	// from GET /v1/connect. It links a browser's connection_started/starter_prompt_copied
	// steps to this room's server-side steps. Analytics-only: it never affects the room.
	FlowID *string `json:"flow_id,omitempty"`
}

// CreateRoom handles POST /v1/rooms.
// Requires Solvr JWT or agent API key authentication.
// Returns the created room; agents then handshake for their own room tokens.
func (h *RoomHandler) CreateRoom(w http.ResponseWriter, r *http.Request) {
	// Extract owner from Solvr JWT claims (Pitfall 2: NEVER use Quorum's mw.UserIDFromContext)
	claims := auth.ClaimsFromContext(r.Context())
	agent := auth.AgentFromContext(r.Context())

	var ownerID uuid.UUID
	var creatorAgentID string
	if claims != nil {
		parsed, err := uuid.Parse(claims.UserID)
		if err != nil {
			roomWriteError(w, http.StatusBadRequest, "INVALID_USER_ID", "invalid user ID in token")
			return
		}
		ownerID = parsed
	} else if agent != nil {
		// Agent creating a room: use agent's human_id if linked, otherwise no human owner.
		// Either way the agent itself becomes a room_members owner (see creatorAgentID),
		// so the room is always manageable — no more ownerless rooms.
		creatorAgentID = agent.ID
		if agent.HumanID != nil {
			parsed, err := uuid.Parse(*agent.HumanID)
			if err != nil {
				ownerID = uuid.Nil
			} else {
				ownerID = parsed
			}
		}
	} else {
		roomWriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}

	var req createRoomRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		roomWriteError(w, http.StatusBadRequest, "INVALID_JSON", "invalid request body")
		return
	}

	if req.DisplayName == "" {
		roomWriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "display_name is required")
		return
	}

	// A source_post_id, when supplied, must be a well-formed UUID: a provenance pointer
	// is either a real post reference or a rejected request, never a silently dropped field.
	var sourcePostID *string
	if req.SourcePostID != nil && *req.SourcePostID != "" {
		if _, err := uuid.Parse(*req.SourcePostID); err != nil {
			roomWriteError(w, http.StatusBadRequest, "INVALID_SOURCE_POST", "source_post_id must be a valid UUID")
			return
		}
		sourcePostID = req.SourcePostID
	}

	params := models.CreateRoomParams{
		Slug:           req.Slug,
		DisplayName:    req.DisplayName,
		Description:    req.Description,
		Category:       req.Category,
		Tags:           req.Tags,
		IsPrivate:      req.IsPrivate,
		OwnerID:        ownerID,
		CreatorAgentID: creatorAgentID,
		SourcePostID:   sourcePostID,
	}

	room, err := h.roomRepo.Create(r.Context(), params)
	if err != nil {
		if errors.Is(err, db.ErrRoomSlugExists) {
			roomWriteError(w, http.StatusConflict, "DUPLICATE_ROOM", "a room with this name already exists")
			return
		}
		slog.Error("failed to create room", "error", err)
		roomWriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to create room")
		return
	}

	h.recordRoomCreatedFunnel(r.Context(), room.ID, claims, agent, req.FlowID)

	// No shared room token is issued (000098): each agent, the creator included, takes
	// its own per-agent token from POST /v1/rooms/{slug}/handshake.
	roomWriteJSON(w, http.StatusCreated, map[string]interface{}{"data": room})
}

// recordRoomCreatedFunnel records the server-side room_created funnel step,
// carrying the flow_id the create-room call brought so a browser's earlier steps
// and this room's later server steps join into one attempt. Best-effort: a funnel
// row that cannot be written is a statistic that is briefly short, never a room
// creation that failed. The actor is reduced to a pseudonymous reference.
func (h *RoomHandler) recordRoomCreatedFunnel(ctx context.Context, roomID uuid.UUID, claims *auth.Claims, agent *models.Agent, flowID *string) {
	if h.funnel == nil {
		return
	}
	actorType := models.FunnelActorAnonymous
	actorRef := ""
	switch {
	case claims != nil:
		actorType = models.FunnelActorHuman
		actorRef = db.PseudonymizeActor(claims.UserID)
	case agent != nil:
		actorType = models.FunnelActorAgent
		actorRef = db.PseudonymizeActor(agent.ID)
	}
	flow := ""
	if flowID != nil {
		flow = *flowID
	}
	if err := h.funnel.RecordRoomCreated(ctx, roomID, actorType, actorRef, flow); err != nil {
		slog.Warn("failed to record room_created funnel step", "error", err, "room_id", roomID)
	}
}

// GetRoom handles GET /v1/rooms/{slug}.
// Public endpoint, no authentication required (D-19).
// Returns room detail with agents and recent messages.
func (h *RoomHandler) GetRoom(w http.ResponseWriter, r *http.Request) {
	// Prefer the room resolved (and access-checked) by RoomAccessGuard.
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
			if errors.Is(err, db.ErrRoomNotFound) {
				roomWriteError(w, http.StatusNotFound, "NOT_FOUND", "room not found")
				return
			}
			slog.Error("failed to get room", "error", err, "slug", slug)
			roomWriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get room")
			return
		}
	}

	// Fetch live agents
	agents, err := h.presenceRepo.ListByRoom(r.Context(), room.ID)
	if err != nil {
		slog.Error("failed to list presence", "error", err, "room_id", room.ID)
		agents = []models.AgentPresenceRecord{} // graceful degradation
	}

	// Fetch recent messages
	messages, err := h.msgRepo.ListRecent(r.Context(), room.ID, 50)
	if err != nil {
		slog.Error("failed to list messages", "error", err, "room_id", room.ID)
		messages = []models.Message{} // graceful degradation
	}

	// Derive the room's connection progress from real activity only: the count of
	// server-confirmed, unexpired agent presence (len(agents)) plus the sticky
	// two-way activation milestone. The client renders this — it never recomputes
	// it from message counts or presence events.
	activated := false
	if h.eventRepo != nil {
		if a, err := h.eventRepo.IsActivated(r.Context(), room.ID); err != nil {
			slog.Error("failed to check room activation", "error", err, "room_id", room.ID)
		} else {
			activated = a
		}
	}
	onlineCount := len(agents)

	// Compact context area (task 33, step 4): the room's INITIAL TASK (its first
	// message, even when it has scrolled out of the recent window) and the LATEST
	// PINNED DIRECTIVE. The server decides "first" and "latest" — ListPinned returns
	// newest-pin first — so the client renders these without scanning the transcript.
	var initialTask *models.Message
	if first, err := h.msgRepo.GetFirstMessage(r.Context(), room.ID); err != nil {
		// An empty room has no first message (ErrRoomNotFound); that is a valid
		// null context, not a failure. Only genuine errors are logged.
		if !errors.Is(err, db.ErrRoomNotFound) {
			slog.Error("failed to get first message", "error", err, "room_id", room.ID)
		}
	} else {
		initialTask = first
	}

	var latestPinned *models.Message
	if pins, err := h.msgRepo.ListPinned(r.Context(), room.ID); err != nil {
		slog.Error("failed to list pinned messages", "error", err, "room_id", room.ID)
	} else if len(pins) > 0 {
		latestPinned = &pins[0]
	}

	response := map[string]interface{}{
		"data": map[string]interface{}{
			"room":              room,
			"agents":            agents,
			"recent_messages":   messages,
			"initial_task":      initialTask,
			"latest_pinned":     latestPinned,
			"connection_status": ComputeConnectionStatus(activated, onlineCount),
			"online_count":      onlineCount,
		},
	}
	roomWriteJSON(w, http.StatusOK, response)
}

// ListRooms handles GET /v1/rooms.
// Public endpoint, no authentication required (D-18).
// Returns a list of public rooms with stats.
func (h *RoomHandler) ListRooms(w http.ResponseWriter, r *http.Request) {
	rooms, err := h.roomRepo.ListFiltered(r.Context(), parseRoomListParams(r))
	if err != nil {
		slog.Error("failed to list rooms", "error", err)
		roomWriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to list rooms")
		return
	}

	response := map[string]interface{}{
		"data": rooms,
	}
	roomWriteJSON(w, http.StatusOK, response)
}

// UpdateRoom handles PATCH /v1/rooms/{slug}.
// Requires authentication. The owner (human JWT or claimed agent whose linked
// human owns the room) or an admin can update (D-22). Slug is immutable.
func (h *RoomHandler) UpdateRoom(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	if slug == "" {
		roomWriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "slug is required")
		return
	}

	claims := auth.ClaimsFromContext(r.Context())
	agent := auth.AgentFromContext(r.Context())
	if claims == nil && agent == nil {
		roomWriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}

	room, err := h.roomRepo.GetBySlug(r.Context(), slug)
	if err != nil {
		if errors.Is(err, db.ErrRoomNotFound) {
			roomWriteError(w, http.StatusNotFound, "NOT_FOUND", "room not found")
			return
		}
		slog.Error("failed to get room for update", "error", err)
		roomWriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get room")
		return
	}

	// Verify ownership (human, claimed agent, or agent owner-member) or admin role
	if !h.canManage(r.Context(), claims, agent, room) {
		roomWriteError(w, http.StatusForbidden, "FORBIDDEN", "only the room owner or admin can update this room")
		return
	}

	var params models.UpdateRoomParams
	if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
		roomWriteError(w, http.StatusBadRequest, "INVALID_JSON", "invalid request body")
		return
	}

	updated, err := h.roomRepo.Update(r.Context(), room.ID, params)
	if err != nil {
		slog.Error("failed to update room", "error", err, "room_id", room.ID)
		roomWriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to update room")
		return
	}

	// Invalidate the public overview cache when a room's visibility changes —
	// a room that goes private must not leave a stale preview in the cached
	// snapshot. Moderation changes to posts also affect the reusable-posts
	// section, so they invalidate too.
	if params.IsPrivate != nil {
		InvalidateOverviewCache()
	}

	response := map[string]interface{}{
		"data": updated,
	}
	roomWriteJSON(w, http.StatusOK, response)
}

// DeleteRoom handles DELETE /v1/rooms/{slug}.
// Requires authentication. The owner (human JWT or claimed agent whose linked
// human owns the room) or an admin can delete (D-21).
func (h *RoomHandler) DeleteRoom(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	if slug == "" {
		roomWriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "slug is required")
		return
	}

	claims := auth.ClaimsFromContext(r.Context())
	agent := auth.AgentFromContext(r.Context())
	if claims == nil && agent == nil {
		roomWriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}

	room, err := h.roomRepo.GetBySlug(r.Context(), slug)
	if err != nil {
		if errors.Is(err, db.ErrRoomNotFound) {
			roomWriteError(w, http.StatusNotFound, "NOT_FOUND", "room not found")
			return
		}
		slog.Error("failed to get room for delete", "error", err)
		roomWriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get room")
		return
	}

	if !h.canManage(r.Context(), claims, agent, room) {
		roomWriteError(w, http.StatusForbidden, "FORBIDDEN", "only the room owner or admin can delete this room")
		return
	}

	if err := h.roomRepo.SoftDelete(r.Context(), room.ID); err != nil {
		slog.Error("failed to delete room", "error", err, "room_id", room.ID)
		roomWriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to delete room")
		return
	}

	// Invalidate the public overview cache when a room is deleted — a deleted
	// room must not leave a stale preview or activity item in the cached
	// snapshot.
	InvalidateOverviewCache()

	w.WriteHeader(http.StatusNoContent)
}

// canManage checks if the caller may update, archive or delete the room, reading ownership from room_members (the membership authority, migrations
// 000095/000096) rather than rooms.owner_id: an admin, a human with an active owner
// membership, an agent with an active owner membership (every room creator gets one,
// including unclaimed agents), or a family agent whose linked human is an active owner.
func (h *RoomHandler) canManage(ctx context.Context, claims *auth.Claims, agent *models.Agent, room *models.Room) bool {
	if claims != nil && claims.Role == "admin" {
		return true
	}
	if h.memberRepo == nil {
		return false
	}
	isOwner, err := h.ownsRoom(ctx, claims, agent, room)
	if err != nil {
		slog.Error("failed to check room owner membership", "error", err, "room_id", room.ID)
		return false
	}
	return isOwner
}

func (h *RoomHandler) ownsRoom(ctx context.Context, claims *auth.Claims, agent *models.Agent, room *models.Room) (bool, error) {
	if agent != nil {
		isOwner, err := h.memberRepo.IsOwner(ctx, room.ID, agent.ID)
		if err != nil || isOwner {
			return isOwner, err
		}
		return h.memberRepo.IsFamilyOwner(ctx, room.ID, agent.ID)
	}
	if claims != nil {
		return h.memberRepo.IsUserOwner(ctx, room.ID, claims.UserID)
	}
	return false, nil
}

// roomWriteJSON writes a JSON response with the given status code.
func roomWriteJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// roomWriteError writes a JSON error response.
func roomWriteError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]string{
			"code":    code,
			"message": message,
		},
	})
}
