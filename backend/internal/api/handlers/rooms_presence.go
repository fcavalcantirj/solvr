package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/a2aproject/a2a-go/a2a"
	apimiddleware "github.com/fcavalcantirj/solvr/internal/api/middleware"
	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/hub"
	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// maxCardJSONBytes is the maximum allowed card_json size (D-33 and migration CHECK).
const maxCardJSONBytes = 16384

// defaultTTLSeconds is the default presence TTL (10 minutes per STATE.md decision, not Quorum's 300s).
const defaultTTLSeconds = 600

// RoomPresenceHandler handles HTTP requests for agent presence in rooms.
type RoomPresenceHandler struct {
	presenceRepo *db.AgentPresenceRepository
	roomRepo     *db.RoomRepository
	hubMgr       *hub.HubManager
	registry     *hub.PresenceRegistry
	// funnel records the participant_joined connection-funnel step. Optional:
	// nil when the funnel is not wired, in which case joining records nothing.
	funnel *db.FunnelEventRepository
}

// SetFunnelRecorder wires the connection-funnel recorder so JoinRoom records the
// server-side participant_joined step. Optional: with no recorder, join behaves
// exactly as before.
func (h *RoomPresenceHandler) SetFunnelRecorder(funnel *db.FunnelEventRepository) {
	h.funnel = funnel
}

// NewRoomPresenceHandler creates a new RoomPresenceHandler.
func NewRoomPresenceHandler(
	presenceRepo *db.AgentPresenceRepository,
	roomRepo *db.RoomRepository,
	hubMgr *hub.HubManager,
	registry *hub.PresenceRegistry,
) *RoomPresenceHandler {
	return &RoomPresenceHandler{
		presenceRepo: presenceRepo,
		roomRepo:     roomRepo,
		hubMgr:       hubMgr,
		registry:     registry,
	}
}

// joinRoomRequest is the JSON body for POST /r/{slug}/join.
type joinRoomRequest struct {
	AgentName  string          `json:"agent_name"`
	Card       json.RawMessage `json:"card,omitempty"`
	TTLSeconds int             `json:"ttl_seconds,omitempty"`
}

// JoinRoom handles POST /r/{slug}/join.
// Requires bearer token authentication via BearerGuard middleware.
// Registers agent presence in the database and in-memory registry.
func (h *RoomPresenceHandler) JoinRoom(w http.ResponseWriter, r *http.Request) {
	room := apimiddleware.RoomFromContext(r.Context())
	if room == nil {
		roomWriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "room context missing")
		return
	}
	if room.IsArchived() {
		roomWriteError(w, http.StatusConflict, "ROOM_ARCHIVED", "this room is finished; an owner must reopen it before new agents can join")
		return
	}

	var req joinRoomRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		roomWriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid request body")
		return
	}

	if req.AgentName == "" {
		roomWriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "agent_name is required")
		return
	}

	// Default TTL to 600s (10 min) per STATE.md decision, not Quorum's 300s
	ttl := req.TTLSeconds
	if ttl <= 0 {
		ttl = defaultTTLSeconds
	}

	// Validate card_json size (D-33)
	if len(req.Card) > maxCardJSONBytes {
		roomWriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "card exceeds maximum size of 16384 bytes")
		return
	}

	// Presence belongs to the authenticated member (migration 000099): one row per
	// (room, agent), agent_name is only its display label.
	agentID := apimiddleware.RoomAgentIDFromContext(r.Context())
	if agentID == "" {
		roomWriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "a per-agent room token is required to join")
		return
	}
	previousName, err := h.presenceRepo.CurrentName(r.Context(), room.ID, agentID)
	if err != nil {
		slog.Error("failed to read presence", "error", err, "room_id", room.ID, "agent_id", agentID)
		roomWriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to join room")
		return
	}

	params := models.UpsertAgentPresenceParams{
		RoomID:     room.ID,
		AgentID:    agentID,
		AgentName:  req.AgentName,
		CardJSON:   req.Card,
		TTLSeconds: ttl,
	}
	record, err := h.presenceRepo.Upsert(r.Context(), params)
	if errors.Is(err, db.ErrPresenceNotMember) {
		roomWriteError(w, http.StatusForbidden, "FORBIDDEN", "only an active member of this room can join it")
		return
	}
	if errors.Is(err, db.ErrPresenceNameTaken) {
		roomWriteError(w, http.StatusConflict, "AGENT_NAME_TAKEN", "another member of this room is present under that agent_name")
		return
	}
	if err != nil {
		slog.Error("failed to upsert presence", "error", err, "room_id", room.ID, "agent", req.AgentName)
		roomWriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to join room")
		return
	}

	roomID := hub.NewRoomID(room.ID)
	if previousName != "" && previousName != req.AgentName {
		// A repeat join under a new label replaces the member's old entry on every instance.
		h.hubMgr.Left(roomID, previousName)
	}

	h.recordParticipantJoinedFunnel(r.Context(), room.ID)

	// Parse card for in-memory registry and hub subscription
	agentCard := parseAgentCard(req.Card, req.AgentName)

	// Add to in-memory registry
	h.registry.Add(roomID, req.AgentName, agentCard)

	// Subscribe to hub for real-time events
	_, err = h.hubMgr.GetOrCreate(r.Context(), roomID).Subscribe(req.AgentName, agentCard)
	if err != nil {
		slog.Error("failed to subscribe to hub", "error", err, "room_id", room.ID, "agent", req.AgentName)
		// Non-fatal: DB presence is already recorded
	}
	// Streams on the other instances announce the join too.
	h.hubMgr.Joined(roomID, req.AgentName)

	response := map[string]interface{}{
		"data": record,
	}
	roomWriteJSON(w, http.StatusOK, response)
}

// recordParticipantJoinedFunnel records the server-side participant_joined step
// for the authenticated agent, deduped per (room, agent) and stamped with its
// 1-based join ordinal. It uses the AUTHORITATIVE agent id from the room token,
// not the spoofable agent_name, so an agent using two display names is one
// participant. A shared-token join (no per-agent id) is skipped rather than
// mis-attributed. Best-effort: a missed step never fails the join.
func (h *RoomPresenceHandler) recordParticipantJoinedFunnel(ctx context.Context, roomID uuid.UUID) {
	if h.funnel == nil {
		return
	}
	agentID := apimiddleware.RoomAgentIDFromContext(ctx)
	if agentID == "" {
		return
	}
	if _, _, err := h.funnel.RecordParticipantJoined(ctx, roomID, models.FunnelActorAgent, db.PseudonymizeActor(agentID)); err != nil {
		slog.Warn("failed to record participant_joined funnel step", "error", err, "room_id", roomID)
	}
}

// heartbeatRequest is the JSON body for POST /r/{slug}/heartbeat.
type heartbeatRequest struct {
	AgentName string `json:"agent_name"`
}

// Heartbeat handles POST /r/{slug}/heartbeat.
// Requires bearer token authentication via BearerGuard middleware (D-28).
// Renews agent presence TTL in both database and in-memory registry.
func (h *RoomPresenceHandler) Heartbeat(w http.ResponseWriter, r *http.Request) {
	room := apimiddleware.RoomFromContext(r.Context())
	if room == nil {
		roomWriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "room context missing")
		return
	}

	var req heartbeatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		roomWriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid request body")
		return
	}

	if req.AgentName == "" {
		roomWriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "agent_name is required")
		return
	}

	// Renew the authenticated member's own presence (agent_name cannot select another
	// member's row). Not present = nothing to renew.
	name, err := h.presenceRepo.UpdateHeartbeat(r.Context(), room.ID, apimiddleware.RoomAgentIDFromContext(r.Context()))
	if err != nil {
		slog.Error("failed to update heartbeat", "error", err, "room_id", room.ID, "agent", req.AgentName)
		roomWriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to update heartbeat")
		return
	}

	// Update last_seen in in-memory registry
	if name != "" {
		h.registry.UpdateLastSeen(hub.NewRoomID(room.ID), name)
	}

	response := map[string]interface{}{
		"data": map[string]bool{
			"ok": true,
		},
	}
	roomWriteJSON(w, http.StatusOK, response)
}

// ListPresence handles GET /r/{slug}/agents and GET /v1/rooms/{slug}/agents.
// Returns live agents in the room (those within their TTL window).
func (h *RoomPresenceHandler) ListPresence(w http.ResponseWriter, r *http.Request) {
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

	records, err := h.presenceRepo.ListByRoom(r.Context(), room.ID)
	if err != nil {
		slog.Error("failed to list presence", "error", err, "room_id", room.ID)
		roomWriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to list agents")
		return
	}

	response := map[string]interface{}{
		"data": records,
	}
	roomWriteJSON(w, http.StatusOK, response)
}

// GetAgentCard handles GET /r/{slug}/agents/{agent_name}.
// Returns the agent card for a specific agent in the room.
func (h *RoomPresenceHandler) GetAgentCard(w http.ResponseWriter, r *http.Request) {
	room := apimiddleware.RoomFromContext(r.Context())
	if room == nil {
		roomWriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "room context missing")
		return
	}

	agentName := chi.URLParam(r, "agent_name")
	if agentName == "" {
		roomWriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "agent_name is required")
		return
	}

	// Read the card from unexpired database presence, not this instance's memory: the
	// agent may have joined through another API instance.
	raw, found, err := h.presenceRepo.LiveCard(r.Context(), room.ID, agentName)
	if err != nil {
		slog.Error("failed to read agent card", "error", err, "room_id", room.ID, "agent", agentName)
		roomWriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to read agent card")
		return
	}
	if !found {
		roomWriteError(w, http.StatusNotFound, "NOT_FOUND", "agent not found in room")
		return
	}
	card := parseAgentCard(raw, agentName)

	response := map[string]interface{}{
		"data": card,
	}
	roomWriteJSON(w, http.StatusOK, response)
}

// parseAgentCard decodes a stored or submitted card. The card is optional metadata: an
// empty or undecodable one is nil, never an error.
func parseAgentCard(raw json.RawMessage, agentName string) *a2a.AgentCard {
	if len(raw) == 0 {
		return nil
	}
	card := &a2a.AgentCard{}
	if err := json.Unmarshal(raw, card); err != nil {
		slog.Warn("failed to parse agent card", "error", err, "agent", agentName)
		return nil
	}
	return card
}

// leaveRoomRequest is the JSON body for POST /r/{slug}/leave.
type leaveRoomRequest struct {
	AgentName string `json:"agent_name"`
}

// LeaveRoom handles POST /r/{slug}/leave.
// Requires bearer token authentication via BearerGuard middleware.
// Removes agent from database, in-memory registry, and hub (emits presence_leave per D-27).
func (h *RoomPresenceHandler) LeaveRoom(w http.ResponseWriter, r *http.Request) {
	room := apimiddleware.RoomFromContext(r.Context())
	if room == nil {
		roomWriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "room context missing")
		return
	}

	var req leaveRoomRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		roomWriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid request body")
		return
	}

	if req.AgentName == "" {
		roomWriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "agent_name is required")
		return
	}

	roomID := hub.NewRoomID(room.ID)

	// Remove the authenticated member's own presence (agent_name cannot select another
	// member's row).
	name, err := h.presenceRepo.Remove(r.Context(), room.ID, apimiddleware.RoomAgentIDFromContext(r.Context()))
	if err != nil {
		slog.Error("failed to remove presence", "error", err, "room_id", room.ID, "agent", req.AgentName)
		roomWriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to leave room")
		return
	}

	if name != "" {
		// Drops the in-memory entry and emits presence_leave (D-27) on every instance,
		// wherever the agent joined.
		h.hubMgr.Left(roomID, name)
	}

	response := map[string]interface{}{
		"data": map[string]bool{
			"ok": true,
		},
	}
	roomWriteJSON(w, http.StatusOK, response)
}
