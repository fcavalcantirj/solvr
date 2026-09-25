package hub

import "sync"

// accessWatchers are the open streams of this instance waiting to re-authorize when the
// access to their room changes (a member removed, a token revoked, the room made private
// or deleted). A signal carries no decision: each stream re-reads its own authorization.
type accessWatchers struct {
	mu    sync.Mutex
	rooms map[RoomID]map[chan struct{}]struct{}
}

// WatchAccess registers a watcher for access changes to room id. The returned channel
// receives a (coalesced) signal after each change; call cancel when the stream ends.
func (m *HubManager) WatchAccess(id RoomID) (<-chan struct{}, func()) {
	ch := make(chan struct{}, 1)
	w := &m.access
	w.mu.Lock()
	if w.rooms == nil {
		w.rooms = make(map[RoomID]map[chan struct{}]struct{})
	}
	if w.rooms[id] == nil {
		w.rooms[id] = make(map[chan struct{}]struct{})
	}
	w.rooms[id][ch] = struct{}{}
	w.mu.Unlock()
	var once sync.Once
	return ch, func() {
		once.Do(func() {
			w.mu.Lock()
			defer w.mu.Unlock()
			delete(w.rooms[id], ch)
			if len(w.rooms[id]) == 0 {
				delete(w.rooms, id)
			}
		})
	}
}

// AccessChanged signals every watcher of room id to re-authorize.
func (m *HubManager) AccessChanged(id RoomID) {
	m.access.mu.Lock()
	defer m.access.mu.Unlock()
	for ch := range m.access.rooms[id] {
		signalAccess(ch)
	}
}

// AccessChangedAll signals every watcher of every room: used when access changes may
// have been missed (the cross-instance listener reconnected).
func (m *HubManager) AccessChangedAll() {
	m.access.mu.Lock()
	defer m.access.mu.Unlock()
	for _, watchers := range m.access.rooms {
		for ch := range watchers {
			signalAccess(ch)
		}
	}
}

// signalAccess never blocks: a watcher with a pending signal will re-check anyway.
func signalAccess(ch chan struct{}) {
	select {
	case ch <- struct{}{}:
	default:
	}
}
