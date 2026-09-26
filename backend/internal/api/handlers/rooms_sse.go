package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"sync/atomic"
	"time"

	apimiddleware "github.com/fcavalcantirj/solvr/internal/api/middleware"
	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/hub"
	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// globalSSEConnections tracks the total active SSE connections for D-05 limit.
var globalSSEConnections int64

// MaxGlobalSSEConnections is the global SSE connection limit per D-05.
const MaxGlobalSSEConnections int64 = 1000

// Stream timing the public contract publishes (OpenAPI x-solvr-conventions.streams).
const (
	// SSEHeartbeatInterval is the default period of the ": heartbeat" comment and the
	// access re-check (D-04).
	SSEHeartbeatInterval = 30 * time.Second
	// SSEMaxLifetime is how long one stream connection lives before the server ends it
	// and the client reconnects with its cursor (D-03).
	SSEMaxLifetime = 30 * time.Minute
)

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
	hubMgr    *hub.HubManager
	msgRepo   *db.MessageRepository
	entryRepo *db.RoomEntryRepository
	roomRepo  *db.RoomRepository

	// heartbeatInterval is the heartbeat (and access re-check) period; 0 means 30s.
	heartbeatInterval time.Duration

	// testRoomLookup overrides room-by-slug lookup in unit tests (nil in production).
	testRoomLookup func(ctx context.Context, slug string) (*models.Room, error)
}

// NewRoomSSEHandler creates a new SSE handler for room event streaming.
// roomRepo is required for PublicStream (resolves room by slug without bearer token).
// entryRepo supplies the typed events a reconnecting stream replays alongside messages.
func NewRoomSSEHandler(hubMgr *hub.HubManager, msgRepo *db.MessageRepository, entryRepo *db.RoomEntryRepository, roomRepo *db.RoomRepository) *RoomSSEHandler {
	return &RoomSSEHandler{
		hubMgr:    hubMgr,
		msgRepo:   msgRepo,
		entryRepo: entryRepo,
		roomRepo:  roomRepo,
	}
}

// resolveSSERoomBySlug looks up a room by slug, using testRoomLookup in unit tests.
func (h *RoomSSEHandler) resolveSSERoomBySlug(ctx context.Context, slug string) (*models.Room, error) {
	if h.testRoomLookup != nil {
		return h.testRoomLookup(ctx, slug)
	}
	return h.roomRepo.GetBySlug(ctx, slug)
}

// Stream handles GET /r/{slug}/stream -- SSE stream of room activity.
//
// The handler:
// 1. Checks http.Flusher support (required for SSE)
// 2. Enforces global SSE connection limit (D-05: 1000 max)
// 3. Sets SSE headers including X-Accel-Buffering: no (D-02)
// 4. Replays missed timeline entries (messages and typed events) via Last-Event-ID (D-07)
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

	h.streamRoom(w, r, room)
}

// PublicStream handles GET /v1/rooms/{slug}/stream -- the canonical room SSE stream.
//
// Unlike Stream (the /r adapter, room token only), this endpoint:
//   - Is authorized by RoomPolicyGuard (anonymous for public rooms; human, agent-account
//     or room-scoped credentials otherwise), which injects the resolved room
//   - Falls back to resolving the room by slug when no guard ran (unit tests)
//   - Supports ?lastEventId= query param in addition to Last-Event-ID header
//
// All other behaviour (connection limits, heartbeat, max lifetime, replay) is
// identical to Stream. T-16-04: global connection limit and per-room capacity
// check remain enforced to prevent resource exhaustion.
func (h *RoomSSEHandler) PublicStream(w http.ResponseWriter, r *http.Request) {
	room := apimiddleware.RoomFromContext(r.Context())
	if room == nil {
		slug := chi.URLParam(r, "slug")
		if slug == "" {
			http.Error(w, `{"error":{"code":"VALIDATION_ERROR","message":"slug is required"}}`, http.StatusBadRequest)
			return
		}

		var err error
		room, err = h.resolveSSERoomBySlug(r.Context(), slug)
		if err != nil {
			if errors.Is(err, db.ErrRoomNotFound) {
				http.Error(w, `{"error":{"code":"NOT_FOUND","message":"room not found"}}`, http.StatusNotFound)
				return
			}
			http.Error(w, `{"error":{"code":"INTERNAL_ERROR","message":"failed to get room"}}`, http.StatusInternalServerError)
			return
		}
	}

	// Support ?lastEventId= query param for initial browser connect.
	// Browsers can pass it in the URL when the EventSource API doesn't send
	// Last-Event-ID automatically on the first connection.
	if lastID := r.URL.Query().Get("lastEventId"); lastID != "" {
		r.Header.Set("Last-Event-ID", lastID)
	}

	h.streamRoom(w, r, room)
}

// streamRoom contains the shared SSE streaming logic used by both Stream and PublicStream.
func (h *RoomSSEHandler) streamRoom(w http.ResponseWriter, r *http.Request, room *models.Room) {
	// Step 1: Check flusher support (required for SSE streaming).
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	if !rejectUnknownQuery(w, r.URL.Query(), "type", "issue", "after", "lastEventId", "access_token") {
		return
	}

	// ?after=<id> is a query alias for the Last-Event-ID reconnect cursor (mission #5),
	// convenient for clients (curl, non-EventSource) that cannot resend the header. The
	// explicit Last-Event-ID header, if present, wins.
	if after := r.URL.Query().Get("after"); after != "" && r.Header.Get("Last-Event-ID") == "" {
		r.Header.Set("Last-Event-ID", after)
	}

	// Server-side filters (mission #5): ?type= matches the hub event type or a typed
	// event name (e.g. CLAIM); ?issue= matches a typed event's issue. Empty = no filter.
	typeFilter := r.URL.Query().Get("type")
	issueFilter := r.URL.Query().Get("issue")

	// Step 2: Global connection limit (D-05 / T-16-04).
	current := atomic.AddInt64(&globalSSEConnections, 1)
	defer atomic.AddInt64(&globalSSEConnections, -1)
	if current > MaxGlobalSSEConnections {
		http.Error(w, `{"error":{"code":"SERVICE_UNAVAILABLE","message":"SSE connection limit reached"}}`, http.StatusServiceUnavailable)
		return
	}

	// Step 3: Subscribe to hub (D-12: lazy room creation) BEFORE the reconnect replay, so
	// an entry committed while the replay reads is buffered rather than lost; live frames
	// the replay already delivered are skipped below.
	// Browser subscribers use a unique pseudo-agent name prefixed with _browser_
	// so they don't appear in agent discovery or presence lists.
	var ch <-chan hub.RoomEvent
	if h.hubMgr != nil {
		subscriberName := "_browser_" + uuid.New().String()[:8]
		roomHub := h.hubMgr.GetOrCreate(r.Context(), hub.NewRoomID(room.ID))
		sub, err := roomHub.SubscribeStream(subscriberName)
		if err != nil {
			// ErrRoomAtCapacity — per-room SSE limit reached (T-16-04).
			http.Error(w, `{"error":{"code":"SERVICE_UNAVAILABLE","message":"room at capacity"}}`, http.StatusServiceUnavailable)
			return
		}
		defer roomHub.Unsubscribe(subscriberName)
		ch = sub
	}

	// Step 4: Set SSE headers (D-02) and flush immediately so the client
	// receives the 200 status + headers before any events arrive.
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	// Step 5: Last-Event-ID replay (D-07). A reconnect replays every timeline entry after
	// the cursor that matches the stream filters, in sequence order. Filters are applied
	// in the query, so non-matching traffic never uses up the replay budget. When more
	// than maxSSEReplay frames are owed, the stream ends after that many with a retry
	// directive and the client resumes from the last delivered id — never a silent hole.
	var lastSeq int
	if lastID := r.Header.Get("Last-Event-ID"); lastID != "" {
		afterID, parseErr := strconv.ParseInt(lastID, 10, 64)
		if parseErr == nil && afterID > 0 {
			frames, complete := h.replayAfter(r.Context(), room, afterID, typeFilter, issueFilter)
			for _, evt := range frames {
				writeSSEEvent(w, flusher, evt)
				lastSeq = evt.Sequence
			}
			if !complete {
				fmt.Fprintf(w, "retry: 1000\n\n")
				flusher.Flush()
				return
			}
		}
	}

	// Access is re-authorized while the stream is open (a member removed, a token revoked,
	// the room made private or deleted): on every access-change signal for this room and
	// on every heartbeat, so a lost signal is bounded by the heartbeat interval. Checking
	// once right after watching closes the gap between the guard's check and the watch.
	recheck := apimiddleware.RoomAccessRecheckFromContext(r.Context())
	var accessCh <-chan struct{}
	if recheck != nil && h.hubMgr != nil {
		var stopWatch func()
		accessCh, stopWatch = h.hubMgr.WatchAccess(hub.NewRoomID(room.ID))
		defer stopWatch()
		if end := streamEndReason(r.Context(), recheck, room); end != nil {
			end.write(w, flusher)
			return
		}
	}

	if ch == nil {
		// No hub available (e.g. test without hub). Hold open until context cancels.
		<-r.Context().Done()
		return
	}

	// Step 6: Event loop with heartbeat (D-03: 30-min max, D-04: 30s heartbeat).
	maxLifetime, cancel := context.WithTimeout(r.Context(), SSEMaxLifetime)
	defer cancel()

	interval := h.heartbeatInterval
	if interval <= 0 {
		interval = SSEHeartbeatInterval
	}
	heartbeat := time.NewTicker(interval)
	defer heartbeat.Stop()

	for {
		select {
		case evt, ok := <-ch:
			if !ok {
				// Hub shut down, or this stream fell a full buffer behind and the hub closed
				// it: ask the client to reconnect; it resumes after its Last-Event-ID.
				fmt.Fprintf(w, "retry: 1000\n\n")
				flusher.Flush()
				return
			}
			if evt.Sequence > 0 && evt.Sequence <= lastSeq {
				continue // already delivered by the replay
			}
			if evt.Matches(typeFilter, issueFilter) {
				writeSSEEvent(w, flusher, evt)
			}

		case <-accessCh:
			if end := streamEndReason(r.Context(), recheck, room); end != nil {
				end.write(w, flusher)
				return
			}

		case <-heartbeat.C:
			if recheck != nil {
				if end := streamEndReason(r.Context(), recheck, room); end != nil {
					end.write(w, flusher)
					return
				}
			}
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

// streamEnd is why the server ends an open stream. No retry directive follows it: a
// reconnect is refused by the guard until the caller has the access, or a new token, again.
type streamEnd struct{ event, code, message string }

var (
	// endAccessRevoked: the caller lost read access (member removed, token revoked, room
	// made private or deleted).
	endAccessRevoked = &streamEnd{"access_revoked", "ACCESS_REVOKED", "read access to this room was removed"}
	// endCredentialRotated: an explicit rotation replaced the room token the stream was
	// opened with. The caller keeps its access and recovers by handshaking again.
	endCredentialRotated = &streamEnd{"credential_rotated", apimiddleware.CodeCredentialRotated,
		"this room token was replaced by a rotation; handshake again with your agent API key for a new one"}
)

func (e *streamEnd) write(w http.ResponseWriter, flusher http.Flusher) {
	fmt.Fprintf(w, "event: %s\ndata: {\"code\":%q,\"message\":%q}\n\n", e.event, e.code, e.message)
	flusher.Flush()
}

// streamEndReason re-runs the guard's decision for an open stream and returns why it must
// end, or nil while the caller is still authorized. A decision that cannot be read
// (database error) keeps the stream: the next signal or heartbeat retries.
func streamEndReason(ctx context.Context, recheck apimiddleware.RoomAccessRecheck, room *models.Room) *streamEnd {
	ok, err := recheck(ctx)
	switch {
	case errors.Is(err, apimiddleware.ErrRoomCredentialRotated):
		return endCredentialRotated
	case err != nil:
		if ctx.Err() == nil {
			slog.Warn("room stream access recheck failed; keeping the stream", "error", err, "room_id", room.ID)
		}
		return nil
	case !ok:
		return endAccessRevoked
	}
	return nil
}

// maxSSEReplay caps how many missed entries one connection replays; a longer gap ends the
// stream with a retry directive so the client reconnects from its last delivered id.
const maxSSEReplay = SSEMaxReplayFrames

// SSEMaxReplayFrames is maxSSEReplay as published in the contract.
const SSEMaxReplayFrames = 1000

// sseReplayPage is the read size of one replay query.
const sseReplayPage = 100

// replayAfter returns the timeline entries after the entry afterID that match the stream
// filters, as stream frames in sequence order, built exactly as the live broadcasts build
// them. complete is false when maxSSEReplay frames were returned and more are owed.
// Best-effort: a failed read ends the replay rather than breaking the stream.
func (h *RoomSSEHandler) replayAfter(ctx context.Context, room *models.Room, afterID int64, typeFilter, issueFilter string) ([]hub.RoomEvent, bool) {
	if h.entryRepo == nil {
		return nil, true
	}
	p, ok := replayPageParams(typeFilter, issueFilter)
	if !ok {
		return nil, true
	}
	after, err := h.entryRepo.SequenceAfterEntry(ctx, room.ID, afterID)
	if err != nil {
		return nil, true
	}
	p.RoomID = room.ID
	var frames []hub.RoomEvent
	for len(frames) < maxSSEReplay {
		p.AfterSequence, p.Limit = after, min(sseReplayPage, maxSSEReplay-len(frames))
		entries, err := h.entryRepo.ListPage(ctx, p)
		if err != nil {
			return frames, true
		}
		for i := range entries {
			frames = append(frames, timelineHubEvent(&entries[i]))
		}
		if len(entries) < p.Limit {
			return frames, true
		}
		after = entries[len(entries)-1].Sequence
	}
	more, err := h.entryRepo.ListPage(ctx, models.RoomEntryPageParams{
		RoomID: room.ID, AfterSequence: after, Kind: p.Kind, EventType: p.EventType, Issue: p.Issue, Limit: 1,
	})
	return frames, err != nil || len(more) == 0
}

// replayPageParams translates the stream's ?type= / ?issue= filters (hub.RoomEvent.Matches)
// into a timeline query. ok=false means no timeline entry can match (for example
// ?type=presence_join, or a message filter combined with an issue).
func replayPageParams(typeFilter, issueFilter string) (models.RoomEntryPageParams, bool) {
	p := models.RoomEntryPageParams{Issue: issueFilter}
	switch typeFilter {
	case "":
		if issueFilter != "" {
			p.Kind = models.RoomEntryKindEvent // messages carry no issue
		}
	case string(hub.EventMessage):
		if issueFilter != "" {
			return p, false
		}
		p.Kind = models.RoomEntryKindMessage
	case string(hub.EventTyped):
		p.Kind = models.RoomEntryKindEvent
	case string(hub.EventPresenceJoin), string(hub.EventPresenceLeave), string(hub.EventRoomUpdate):
		return p, false
	default:
		p.Kind, p.EventType = models.RoomEntryKindEvent, typeFilter
	}
	return p, true
}

// timelineHubEvent is the stream frame of a timeline entry: the typed-event frame, or the
// message frame carrying the message envelope the live broadcast carries.
func timelineHubEvent(e *models.RoomEntry) hub.RoomEvent {
	if e.Kind == models.RoomEntryKindEvent {
		return typedHubEvent(e)
	}
	return messageHubEvent(messageFromEntry(e))
}

// writeSSEEvent serializes a RoomEvent as an SSE frame and flushes it.
//
// SSE frame format:
//
//	id: <entry ID>               (every message and typed-event frame; Last-Event-ID cursor)
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
