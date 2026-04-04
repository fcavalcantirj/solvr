package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/a2aproject/a2a-go/a2a"
	apimiddleware "github.com/fcavalcantirj/solvr/internal/api/middleware"
	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/hub"
	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/go-chi/chi/v5"
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

	var req joinRoomRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		roomWriteError(w, http.StatusBadRequest, "INVALID_JSON", "invalid request body")
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

	// Upsert presence in database
	params := models.UpsertAgentPresenceParams{
		RoomID:     room.ID,
		AgentName:  req.AgentName,
		CardJSON:   req.Card,
		TTLSeconds: ttl,
	}
	record, err := h.presenceRepo.Upsert(r.Context(), params)
	if err != nil {
		slog.Error("failed to upsert presence", "error", err, "room_id", room.ID, "agent", req.AgentName)
		roomWriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to join room")
		return
	}

	// Parse card for in-memory registry and hub subscription
	var agentCard *a2a.AgentCard
	if len(req.Card) > 0 {
		agentCard = &a2a.AgentCard{}
		if err := json.Unmarshal(req.Card, agentCard); err != nil {
			// Card is optional metadata; log but don't fail
			slog.Warn("failed to parse agent card", "error", err, "agent", req.AgentName)
			agentCard = nil
		}
	}

	// Add to in-memory registry
	roomID := hub.NewRoomID(room.ID)
	h.registry.Add(roomID, req.AgentName, agentCard)

	// Subscribe to hub for real-time events
	_, err = h.hubMgr.GetOrCreate(r.Context(), roomID).Subscribe(req.AgentName, agentCard)
	if err != nil {
		slog.Error("failed to subscribe to hub", "error", err, "room_id", room.ID, "agent", req.AgentName)
		// Non-fatal: DB presence is already recorded
	}

	response := map[string]interface{}{
		"data": record,
	}
	roomWriteJSON(w, http.StatusOK, response)
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
		roomWriteError(w, http.StatusBadRequest, "INVALID_JSON", "invalid request body")
		return
	}

	if req.AgentName == "" {
		roomWriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "agent_name is required")
		return
	}

	// Update heartbeat in database
	if err := h.presenceRepo.UpdateHeartbeat(r.Context(), room.ID, req.AgentName); err != nil {
		slog.Error("failed to update heartbeat", "error", err, "room_id", room.ID, "agent", req.AgentName)
		roomWriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to update heartbeat")
		return
	}

	// Update last_seen in in-memory registry
	h.registry.UpdateLastSeen(hub.NewRoomID(room.ID), req.AgentName)

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

	// Look up in the in-memory registry for the full card
	card, found := h.registry.ExtendedCard(hub.NewRoomID(room.ID), agentName)
	if !found {
		roomWriteError(w, http.StatusNotFound, "NOT_FOUND", "agent not found in room")
		return
	}

	response := map[string]interface{}{
		"data": card,
	}
	roomWriteJSON(w, http.StatusOK, response)
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
		roomWriteError(w, http.StatusBadRequest, "INVALID_JSON", "invalid request body")
		return
	}

	if req.AgentName == "" {
		roomWriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "agent_name is required")
		return
	}

	roomID := hub.NewRoomID(room.ID)

	// Remove from database
	if err := h.presenceRepo.Remove(r.Context(), room.ID, req.AgentName); err != nil {
		slog.Error("failed to remove presence", "error", err, "room_id", room.ID, "agent", req.AgentName)
		roomWriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to leave room")
		return
	}

	// Remove from in-memory registry
	h.registry.Remove(roomID, req.AgentName)

	// Unsubscribe from hub (emits presence_leave event per D-27)
	if roomHub := h.hubMgr.Get(roomID); roomHub != nil {
		roomHub.Unsubscribe(req.AgentName)
	}

	response := map[string]interface{}{
		"data": map[string]bool{
			"ok": true,
		},
	}
	roomWriteJSON(w, http.StatusOK, response)
}
