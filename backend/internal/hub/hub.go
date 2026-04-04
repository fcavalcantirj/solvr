package hub

import (
	"context"
	"errors"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/a2aproject/a2a-go/a2a"
)

// ErrRoomAtCapacity is returned by Subscribe when the room's SSE subscriber
// limit (MaxSSEPerRoom) has been reached. Callers should respond with 503.
var ErrRoomAtCapacity = errors.New("room SSE subscriber limit reached")

// subscribeCmd is sent to the hub's command loop to add a subscriber.
type subscribeCmd struct {
	agentName string
	card      *a2a.AgentCard
	ch        chan<- RoomEvent
	resp      chan error
}

// unsubscribeCmd is sent to the hub's command loop to remove a subscriber.
type unsubscribeCmd struct {
	agentName string
}

// broadcastCmd is sent to the hub's command loop to fanout an event.
type broadcastCmd struct {
	event RoomEvent
}

// RoomHub is a per-room goroutine that serializes all subscriber state mutations.
// It owns the subscriber map exclusively -- no external locks are needed because
// all mutations go through the command channels processed by Run.
//
// The hub exposes three public channels via Subscribe/Unsubscribe/Broadcast:
//   - subscribe channel: register a new subscriber
//   - unsubscribe channel: remove a subscriber
//   - broadcast channel: fanout an event to all current subscribers
//
// The hub also emits all events (join/leave/message) to an internal events
// channel (size 256) that the SSE handler consumes for real-time browser push
// delivery.
type RoomHub struct {
	// ID is the room this hub serves. Immutable after construction.
	ID RoomID

	subscribe   chan subscribeCmd
	unsubscribe chan unsubscribeCmd
	broadcast   chan broadcastCmd

	// events is a buffered channel consumed by the SSE fanout handler.
	// Size 256 allows transient bursts without blocking the hub goroutine.
	events chan RoomEvent

	// done is closed when the hub goroutine exits (ctx canceled).
	done chan struct{}

	// sseCount tracks the number of active SSE subscribers atomically.
	// Reads are lock-free; writes are done inside the hub goroutine for subscribe
	// and unsubscribe, but atomic ops allow safe external reads if needed.
	sseCount atomic.Int32

	// maxSSECount is the per-room SSE subscriber limit from config.
	// Immutable after construction. Subscribe returns ErrRoomAtCapacity when
	// sseCount >= maxSSECount.
	maxSSECount int32

	logger *slog.Logger
}

// NewRoomHub creates a RoomHub for the given room. Call Run in a goroutine to start it.
// The registry parameter is passed to Run -- it is not stored on the hub to avoid
// requiring a mutex (the hub goroutine is the only writer; registry has its own RWMutex).
// maxSSEPerRoom sets the per-room SSE subscriber limit; Subscribe returns ErrRoomAtCapacity
// when this limit is reached. Pass 0 to disable the limit (not recommended for production).
func NewRoomHub(id RoomID, _ *PresenceRegistry, logger *slog.Logger, maxSSEPerRoom int) *RoomHub {
	return &RoomHub{
		ID:          id,
		subscribe:   make(chan subscribeCmd),
		unsubscribe: make(chan unsubscribeCmd),
		broadcast:   make(chan broadcastCmd, 64),
		events:      make(chan RoomEvent, 256),
		done:        make(chan struct{}),
		maxSSECount: int32(maxSSEPerRoom),
		logger:      logger,
	}
}

// Run starts the hub event loop. It must be called exactly once in a goroutine.
// The hub exits when ctx is canceled, closing all subscriber channels.
// registry is used to maintain the persistent in-memory presence records in sync
// with the hub's subscriber map.
func (h *RoomHub) Run(ctx context.Context, registry *PresenceRegistry) {
	// subscribers maps agentName -> buffered event channel.
	// This map is ONLY touched inside this goroutine -- no external locking needed.
	subscribers := make(map[string]chan<- RoomEvent)

	h.logger.Debug("hub started", "room", h.ID.String())

	for {
		select {
		case cmd := <-h.subscribe:
			// Enforce per-room SSE connection limit before accepting the subscriber.
			if h.maxSSECount > 0 && h.sseCount.Load() >= h.maxSSECount {
				cmd.resp <- ErrRoomAtCapacity
				break
			}

			// Register new subscriber.
			subscribers[cmd.agentName] = cmd.ch
			registry.Add(h.ID, cmd.agentName, cmd.card)
			h.sseCount.Add(1)

			// Announce the joining agent to all existing subscribers (not itself).
			evt := RoomEvent{
				Type:      EventPresenceJoin,
				RoomID:    h.ID,
				AgentName: cmd.agentName,
				Payload:   cmd.card,
				Timestamp: time.Now(),
			}
			for name, ch := range subscribers {
				if name == cmd.agentName {
					continue
				}
				select {
				case ch <- evt:
				default:
					// Slow consumer -- drop event rather than blocking the hub.
					h.logger.Warn("subscriber channel full, dropping presence_join event",
						"room", h.ID.String(), "target", name)
				}
			}
			// Emit to the SSE events channel (non-blocking).
			h.emitToEvents(evt)
			cmd.resp <- nil

		case cmd := <-h.unsubscribe:
			ch, ok := subscribers[cmd.agentName]
			if !ok {
				// Agent was already removed -- no-op.
				break
			}
			delete(subscribers, cmd.agentName)
			close(ch)
			registry.Remove(h.ID, cmd.agentName)
			h.sseCount.Add(-1)

			// Announce departure to remaining subscribers.
			evt := RoomEvent{
				Type:      EventPresenceLeave,
				RoomID:    h.ID,
				AgentName: cmd.agentName,
				Timestamp: time.Now(),
			}
			for _, remainingCh := range subscribers {
				select {
				case remainingCh <- evt:
				default:
					h.logger.Warn("subscriber channel full, dropping presence_leave event",
						"room", h.ID.String())
				}
			}
			h.emitToEvents(evt)

		case cmd := <-h.broadcast:
			// Fanout the message to all subscribers.
			for name, ch := range subscribers {
				select {
				case ch <- cmd.event:
				default:
					h.logger.Warn("subscriber channel full, dropping message event",
						"room", h.ID.String(), "target", name)
				}
			}
			h.emitToEvents(cmd.event)

		case <-ctx.Done():
			// Clean shutdown: close all subscriber channels so they can drain.
			for _, ch := range subscribers {
				close(ch)
			}
			h.sseCount.Store(0)
			close(h.done)
			h.logger.Debug("hub stopped", "room", h.ID.String())
			return
		}
	}
}

// Subscribe adds an agent to the room and returns a channel on which the agent
// will receive future RoomEvents. The channel is buffered (size 64) to prevent
// slow consumers from stalling the hub goroutine.
//
// The returned channel is closed when the agent unsubscribes or the hub shuts down.
func (h *RoomHub) Subscribe(agentName string, card *a2a.AgentCard) (<-chan RoomEvent, error) {
	// Buffered per research recommendation: avoid hub goroutine starvation from
	// slow HTTP/SSE consumers.
	ch := make(chan RoomEvent, 64)
	resp := make(chan error, 1)
	h.subscribe <- subscribeCmd{
		agentName: agentName,
		card:      card,
		ch:        ch,
		resp:      resp,
	}
	return ch, <-resp
}

// Unsubscribe removes an agent from the room. The subscriber's channel is closed
// by the hub goroutine. Callers should drain and discard the channel after calling Unsubscribe.
func (h *RoomHub) Unsubscribe(agentName string) {
	h.unsubscribe <- unsubscribeCmd{agentName: agentName}
}

// Broadcast sends an event to all current subscribers in the room.
// Slow consumers are skipped (event dropped) to avoid blocking the hub.
func (h *RoomHub) Broadcast(event RoomEvent) {
	h.broadcast <- broadcastCmd{event: event}
}

// Events returns the read-only events channel. SSE handlers consume
// this to push join/leave/message notifications to browser clients.
// The channel is buffered at size 256.
func (h *RoomHub) Events() <-chan RoomEvent {
	return h.events
}

// Done returns a channel that is closed when the hub goroutine exits.
func (h *RoomHub) Done() <-chan struct{} {
	return h.done
}

// emitToEvents sends an event to the non-subscriber events channel (non-blocking).
func (h *RoomHub) emitToEvents(evt RoomEvent) {
	select {
	case h.events <- evt:
	default:
		h.logger.Warn("events channel full, dropping event",
			"room", h.ID.String(), "type", string(evt.Type))
	}
}
