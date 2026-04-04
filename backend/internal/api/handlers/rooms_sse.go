package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"sync/atomic"
	"time"

	apimiddleware "github.com/fcavalcantirj/solvr/internal/api/middleware"
	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/hub"
	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/google/uuid"
)

// globalSSEConnections tracks the total active SSE connections for D-05 limit.
var globalSSEConnections int64

// MaxGlobalSSEConnections is the global SSE connection limit per D-05.
const MaxGlobalSSEConnections int64 = 1000

// sseRoomContextKey is the context key for the room resolved by bearer guard middleware.
// Plan 03 provides the BearerGuard middleware that sets this value.
type sseRoomContextKey struct{}

// SSERoomFromContext extracts the room set by bearer guard middleware.
// Returns nil if not set. Plan 03 (bearer_guard.go) provides the middleware
// that populates this context value.
func SSERoomFromContext(ctx context.Context) *models.Room {
	room, _ := ctx.Value(sseRoomContextKey{}).(*models.Room)
	return room
}

// SSERoomToContext stores the room in the request context.
// Used by bearer guard middleware (Plan 03) to pass the resolved room to handlers.
func SSERoomToContext(ctx context.Context, room *models.Room) context.Context {
	return context.WithValue(ctx, sseRoomContextKey{}, room)
}

// RoomSSEHandler serves Server-Sent Events for room activity.
// It provides real-time streaming of room events (messages, presence changes)
// with Last-Event-ID replay, heartbeat pings, and connection limits.
type RoomSSEHandler struct {
	hubMgr  *hub.HubManager
	msgRepo *db.MessageRepository
}

// NewRoomSSEHandler creates a new SSE handler for room event streaming.
func NewRoomSSEHandler(hubMgr *hub.HubManager, msgRepo *db.MessageRepository) *RoomSSEHandler {
	return &RoomSSEHandler{
		hubMgr:  hubMgr,
		msgRepo: msgRepo,
	}
}

// Stream handles GET /r/{slug}/stream -- SSE stream of room activity.
//
// The handler:
// 1. Checks http.Flusher support (required for SSE)
// 2. Enforces global SSE connection limit (D-05: 1000 max)
// 3. Sets SSE headers including X-Accel-Buffering: no (D-02)
// 4. Replays missed messages via Last-Event-ID header (D-07)
// 5. Subscribes to the room hub for real-time events (D-12: lazy room creation)
// 6. Streams events with 30s heartbeat (D-04) and 30-min max lifetime (D-03)
//
// SSE event types (D-06):
//   - message: new message posted to the room
//   - presence_join: agent joined the room
//   - presence_leave: agent left or was reaped
//   - room_update: room metadata changed
func (h *RoomSSEHandler) Stream(w http.ResponseWriter, r *http.Request) {
	// The room must be set in context by bearer guard middleware (Plan 03).
	// Uses apimiddleware.RoomFromContext to read from the same context key
	// that BearerGuard sets (middleware.RoomContextKey).
	room := apimiddleware.RoomFromContext(r.Context())
	if room == nil {
		http.Error(w, `{"error":{"code":"NOT_FOUND","message":"room not found in context"}}`, http.StatusNotFound)
		return
	}

	// Step 1: Check flusher support (required for SSE streaming).
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	// Step 2: Global connection limit (D-05).
	current := atomic.AddInt64(&globalSSEConnections, 1)
	defer atomic.AddInt64(&globalSSEConnections, -1)
	if current > MaxGlobalSSEConnections {
		http.Error(w, `{"error":{"code":"SERVICE_UNAVAILABLE","message":"SSE connection limit reached"}}`, http.StatusServiceUnavailable)
		return
	}

	// Step 3: Set SSE headers (D-02) and flush immediately so the client
	// receives the 200 status + headers before any events arrive.
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	// Step 4: Last-Event-ID replay (D-07).
	// If the client reconnects with a Last-Event-ID header, replay missed messages
	// from the database using cursor-based pagination on BIGSERIAL message ID.
	if lastID := r.Header.Get("Last-Event-ID"); lastID != "" {
		afterID, parseErr := strconv.ParseInt(lastID, 10, 64)
		if parseErr == nil && afterID > 0 {
			msgs, listErr := h.msgRepo.ListAfter(r.Context(), room.ID, afterID, 100)
			if listErr == nil {
				for _, msg := range msgs {
					writeSSEEvent(w, flusher, hub.RoomEvent{
						ID:        msg.ID,
						Type:      hub.EventMessage,
						RoomID:    hub.NewRoomID(room.ID),
						AgentName: msg.AgentName,
						Payload:   msg,
						Timestamp: msg.CreatedAt,
					})
				}
			}
		}
	}

	// Step 5: Subscribe to hub (D-12: lazy room creation).
	// Browser subscribers use a unique pseudo-agent name prefixed with _browser_
	// so they don't appear in agent discovery or presence lists.
	subscriberName := "_browser_" + uuid.New().String()[:8]
	roomHub := h.hubMgr.GetOrCreate(r.Context(), hub.NewRoomID(room.ID))
	ch, err := roomHub.Subscribe(subscriberName, nil)
	if err != nil {
		// ErrRoomAtCapacity — per-room SSE limit reached.
		http.Error(w, `{"error":{"code":"SERVICE_UNAVAILABLE","message":"room at capacity"}}`, http.StatusServiceUnavailable)
		return
	}
	defer roomHub.Unsubscribe(subscriberName)

	// Step 6: Event loop with heartbeat (D-03: 30-min max, D-04: 30s heartbeat).
	maxLifetime, cancel := context.WithTimeout(r.Context(), 30 * time.Minute)
	defer cancel()

	heartbeat := time.NewTicker(30 * time.Second)
	defer heartbeat.Stop()

	for {
		select {
		case evt, ok := <-ch:
			if !ok {
				// Hub shut down or we were unsubscribed.
				return
			}
			writeSSEEvent(w, flusher, evt)

		case <-heartbeat.C:
			// D-04: Send heartbeat comment to keep connection alive and detect dead clients.
			fmt.Fprintf(w, ": heartbeat\n\n")
			flusher.Flush()

		case <-maxLifetime.Done():
			// D-03: Send retry directive before closing so the client auto-reconnects.
			fmt.Fprintf(w, "retry: 1000\n\n")
			flusher.Flush()
			return
		}
	}
}

// writeSSEEvent serializes a RoomEvent as an SSE frame and flushes it.
//
// SSE frame format:
//
//	id: <BIGSERIAL message ID>   (only for messages, enables Last-Event-ID reconnection)
//	event: <event type>          (message, presence_join, presence_leave, room_update)
//	data: <JSON payload>
func writeSSEEvent(w http.ResponseWriter, flusher http.Flusher, evt hub.RoomEvent) {
	if evt.ID > 0 {
		fmt.Fprintf(w, "id: %d\n", evt.ID)
	}
	fmt.Fprintf(w, "event: %s\n", evt.Type)
	data, err := json.Marshal(evt)
	if err != nil {
		// Best-effort: skip malformed events rather than breaking the stream.
		return
	}
	fmt.Fprintf(w, "data: %s\n\n", data)
	flusher.Flush()
}
