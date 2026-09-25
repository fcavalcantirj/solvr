package hub

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
)

// RelayPageSize bounds one read of committed timeline entries: a backlog of any length is
// relayed page by page, so catching up never holds more than this many frames in memory.
const RelayPageSize = 100

// relayReadTimeout bounds one database read of the relay.
const relayReadTimeout = 5 * time.Second

// EntrySource reads a room's committed timeline. It is the durable delivery record the
// relay fans out from: a notification only says "this room has new entries", and the
// relay reads what is committed after its cursor.
type EntrySource interface {
	// MaxSequence is the room's latest committed timeline sequence (0 when empty).
	MaxSequence(ctx context.Context, roomID uuid.UUID) (int, error)
	// FramesAfter returns up to limit stream frames of the entries after sequence after,
	// in sequence order, each with its Sequence set.
	FramesAfter(ctx context.Context, roomID uuid.UUID, after, limit int) ([]RoomEvent, error)
}

// roomCursor is one room's relay position on this instance: the last timeline sequence
// fanned out to the local hub. One drain at a time runs per room; a wakeup arriving
// during a drain marks it dirty so the drain reads again instead of starting a second.
type roomCursor struct {
	seq     int
	known   bool
	running bool
	dirty   bool
}

// relay turns wakeups into ordered, exactly-once local delivery of committed entries.
type relay struct {
	src EntrySource

	mu      sync.Mutex
	cursors map[RoomID]*roomCursor
	active  int
	idle    *sync.Cond
}

// EnableRelay makes the database the delivery record for timeline entries: Publish no
// longer fans out the frame it is given, it wakes the room's drain, which reads every
// entry committed after the room's cursor and broadcasts it once, in sequence order.
// Entries written by another API instance reach this one the same way through Announce
// (driven by a cross-instance notification) or WakeAll (after notifications may have
// been missed). Call once, before serving requests.
func (m *HubManager) EnableRelay(src EntrySource) {
	r := &relay{src: src, cursors: make(map[RoomID]*roomCursor)}
	r.idle = sync.NewCond(&r.mu)
	m.mu.Lock()
	m.relay = r
	m.mu.Unlock()
}

// Publish announces a committed timeline entry of the room. With a relay the frame is
// ignored and the committed row is read back (see EnableRelay); without one the frame is
// broadcast directly to this instance's subscribers.
func (m *HubManager) Publish(id RoomID, evt RoomEvent) {
	if m.relayOrNil() != nil {
		m.Announce(id)
		return
	}
	m.GetOrCreate(m.ctx, id).Broadcast(evt)
}

// Announce wakes the relay for a room that has new committed entries. A room without a
// hub on this instance has no local subscribers, so there is nothing to do.
func (m *HubManager) Announce(id RoomID) {
	r := m.relayOrNil()
	h := m.Get(id)
	if r == nil || h == nil {
		return
	}
	r.wake(m.ctx, h)
}

// WakeAll wakes the relay for every room with a hub on this instance. Used when
// notifications may have been lost (a listener reconnect) and as a periodic sweep.
func (m *HubManager) WakeAll() {
	r := m.relayOrNil()
	if r == nil {
		return
	}
	m.mu.RLock()
	hubs := make([]*RoomHub, 0, len(m.hubs))
	for _, h := range m.hubs {
		hubs = append(hubs, h)
	}
	m.mu.RUnlock()
	for _, h := range hubs {
		r.wake(m.ctx, h)
	}
}

// WaitIdle blocks until no relay drain is running.
func (m *HubManager) WaitIdle() {
	r := m.relayOrNil()
	if r == nil {
		return
	}
	r.mu.Lock()
	for r.active > 0 {
		r.idle.Wait()
	}
	r.mu.Unlock()
}

func (m *HubManager) relayOrNil() *relay {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.relay
}

// start records the room's current position when its hub is created, so history
// committed before this instance had subscribers is not re-broadcast as live traffic.
// Called with the manager lock held, before the hub is visible to any caller.
func (r *relay) start(ctx context.Context, id RoomID) {
	c := &roomCursor{}
	rctx, cancel := context.WithTimeout(ctx, relayReadTimeout)
	defer cancel()
	if seq, err := r.src.MaxSequence(rctx, id.UUID()); err == nil {
		c.seq, c.known = seq, true
	}
	r.mu.Lock()
	r.cursors[id] = c
	r.mu.Unlock()
}

// wake runs the room's drain, or marks the running drain dirty.
func (r *relay) wake(ctx context.Context, h *RoomHub) {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.cursors[h.ID]
	if !ok {
		c = &roomCursor{}
		r.cursors[h.ID] = c
	}
	if c.running {
		c.dirty = true
		return
	}
	c.running = true
	r.active++
	go r.drain(ctx, h, c)
}

// drain fans out every entry committed after the cursor, a page at a time, advancing the
// cursor only past frames handed to the hub. A failed read leaves the cursor in place so
// the next wakeup retries from the same entry.
func (r *relay) drain(ctx context.Context, h *RoomHub, c *roomCursor) {
	defer func() {
		r.mu.Lock()
		c.running = false
		r.active--
		r.idle.Broadcast()
		r.mu.Unlock()
	}()
	for {
		r.mu.Lock()
		c.dirty = false
		seq, known := c.seq, c.known
		r.mu.Unlock()

		seq, known = r.readAll(ctx, h, seq, known)

		r.mu.Lock()
		c.seq, c.known = seq, known
		again := c.dirty
		r.mu.Unlock()
		if !again {
			return
		}
	}
}

// readAll broadcasts the pages after seq and returns the new cursor. An unknown cursor
// (the start read failed) is first re-established at the room's latest sequence.
func (r *relay) readAll(ctx context.Context, h *RoomHub, seq int, known bool) (int, bool) {
	if !known {
		rctx, cancel := context.WithTimeout(ctx, relayReadTimeout)
		latest, err := r.src.MaxSequence(rctx, h.ID.UUID())
		cancel()
		if err != nil {
			h.logger.Warn("relay: cannot establish room cursor", "room", h.ID.String(), "error", err)
			return seq, false
		}
		return latest, true
	}
	for {
		rctx, cancel := context.WithTimeout(ctx, relayReadTimeout)
		frames, err := r.src.FramesAfter(rctx, h.ID.UUID(), seq, RelayPageSize)
		cancel()
		if err != nil {
			h.logger.Warn("relay: read failed, will retry on next wakeup", "room", h.ID.String(), "error", err)
			return seq, true
		}
		for _, evt := range frames {
			select {
			case h.broadcast <- broadcastCmd{event: evt}:
				seq = evt.Sequence
			case <-h.done:
				return seq, true
			}
		}
		if len(frames) < RelayPageSize {
			return seq, true
		}
	}
}
