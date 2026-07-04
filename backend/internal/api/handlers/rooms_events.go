package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	apimiddleware "github.com/fcavalcantirj/solvr/internal/api/middleware"
	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/hub"
	"github.com/fcavalcantirj/solvr/internal/models"
)

// maxEventPayloadBytes bounds a typed-event payload (matches the DB CHECK).
const maxEventPayloadBytes = 16384

// RoomEventsHandler serves typed room events (mission #4) on the A2A namespace.
// All handlers run behind BearerGuard, so the room is in context.
type RoomEventsHandler struct {
	eventRepo *db.RoomEventRepository
	hubMgr    *hub.HubManager
}

// NewRoomEventsHandler creates a new RoomEventsHandler.
func NewRoomEventsHandler(eventRepo *db.RoomEventRepository, hubMgr *hub.HubManager) *RoomEventsHandler {
	return &RoomEventsHandler{eventRepo: eventRepo, hubMgr: hubMgr}
}

type postEventRequest struct {
	Type    string          `json:"type"`
	Issue   string          `json:"issue,omitempty"`
	Actor   string          `json:"actor"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

// PostEvent handles POST /r/{slug}/events — append a typed coordination event.
// Body: {type, issue?, actor, payload?}. The event is persisted and broadcast to the
// room's SSE stream.
func (h *RoomEventsHandler) PostEvent(w http.ResponseWriter, r *http.Request) {
	room := apimiddleware.RoomFromContext(r.Context())
	if room == nil {
		roomWriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "room context missing")
		return
	}
	var req postEventRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		roomWriteError(w, http.StatusBadRequest, "INVALID_JSON", "invalid request body")
		return
	}
	if req.Type == "" {
		roomWriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "type is required")
		return
	}
	if req.Actor == "" {
		roomWriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "actor is required")
		return
	}
	if len(req.Payload) > maxEventPayloadBytes {
		roomWriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "payload exceeds maximum size of 16384 bytes")
		return
	}

	event, err := h.eventRepo.Create(r.Context(), models.CreateRoomEventParams{
		RoomID:    room.ID,
		EventType: req.Type,
		Issue:     req.Issue,
		Actor:     req.Actor,
		Payload:   req.Payload,
	})
	if err != nil {
		slog.Error("failed to create room event", "error", err, "room_id", room.ID, "type", req.Type)
		roomWriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to create event")
		return
	}

	// Broadcast to SSE subscribers so live consumers see the event immediately.
	if h.hubMgr != nil {
		roomHub := h.hubMgr.GetOrCreate(r.Context(), hub.NewRoomID(room.ID))
		roomHub.Broadcast(hub.RoomEvent{
			ID:        event.ID,
			Type:      hub.EventTyped,
			RoomID:    hub.NewRoomID(room.ID),
			AgentName: event.Actor,
			EventName: event.EventType,
			Issue:     event.Issue,
			Payload:   event,
			Timestamp: event.CreatedAt,
		})
	}

	roomWriteJSON(w, http.StatusCreated, map[string]any{"data": event})
}

// ListEvents handles GET /r/{slug}/events?type=&issue=&limit= — query typed events,
// newest first, optionally filtered by type and/or issue.
func (h *RoomEventsHandler) ListEvents(w http.ResponseWriter, r *http.Request) {
	room := apimiddleware.RoomFromContext(r.Context())
	if room == nil {
		roomWriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "room context missing")
		return
	}
	limit := 100
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	events, err := h.eventRepo.Query(r.Context(), models.QueryRoomEventsParams{
		RoomID:    room.ID,
		EventType: r.URL.Query().Get("type"),
		Issue:     r.URL.Query().Get("issue"),
		Limit:     limit,
	})
	if err != nil {
		slog.Error("failed to query room events", "error", err, "room_id", room.ID)
		roomWriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to query events")
		return
	}
	roomWriteJSON(w, http.StatusOK, map[string]any{"data": events})
}
