package hub

import (
	"context"
	"log/slog"
	"sync"
)

// HubManager manages the lifecycle of per-room RoomHub instances.
// It lazily creates hubs on first access and can remove them when rooms become
// idle or are deleted. GetOrCreate is the primary entry point for the room
// endpoint handlers to obtain the hub for a given room.
//
// HubManager is safe for concurrent use.
type HubManager struct {
	mu            sync.RWMutex
	hubs          map[RoomID]*RoomHub
	ctx           context.Context // long-lived context for hub goroutines
	registry      *PresenceRegistry
	logger        *slog.Logger
	maxSSEPerRoom int // per-room SSE subscriber limit passed to each new RoomHub
}

// NewHubManager creates an empty HubManager backed by the given PresenceRegistry.
// ctx is the long-lived context that all hub goroutines inherit — cancel it to shut
// down every hub (typically at server shutdown).
// maxSSEPerRoom is the per-room SSE subscriber limit enforced by each RoomHub
// (see ErrRoomAtCapacity). Pass 0 to disable the limit (not recommended for production).
func NewHubManager(ctx context.Context, registry *PresenceRegistry, logger *slog.Logger, maxSSEPerRoom int) *HubManager {
	return &HubManager{
		hubs:          make(map[RoomID]*RoomHub),
		ctx:           ctx,
		registry:      registry,
		logger:        logger,
		maxSSEPerRoom: maxSSEPerRoom,
	}
}

// GetOrCreate returns the RoomHub for the given room, creating and starting it
// if it does not yet exist. The hub goroutine is bound to the manager's long-lived
// context (not the caller's request context) so it survives individual HTTP requests.
//
// The returned hub is guaranteed to have its Run goroutine already started.
func (m *HubManager) GetOrCreate(_ context.Context, id RoomID) *RoomHub {
	// Fast path: hub already exists.
	m.mu.RLock()
	if h, ok := m.hubs[id]; ok {
		m.mu.RUnlock()
		return h
	}
	m.mu.RUnlock()

	// Slow path: create and register a new hub.
	m.mu.Lock()
	defer m.mu.Unlock()

	// Double-check after acquiring write lock (another goroutine may have beaten us).
	if h, ok := m.hubs[id]; ok {
		return h
	}

	h := NewRoomHub(id, m.registry, m.logger, m.maxSSEPerRoom)
	m.hubs[id] = h
	go h.Run(m.ctx, m.registry)
	m.logger.Info("hub created", "room", id.String())
	return h
}

// Get returns the RoomHub for the given room, or nil if it has not been created.
// Does not create a hub -- use GetOrCreate for that.
func (m *HubManager) Get(id RoomID) *RoomHub {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.hubs[id]
}

// Remove removes a hub from the manager's registry. The hub's goroutine is NOT
// stopped here -- callers are responsible for canceling the hub's context before
// or after removing it. This method is used when a room is deleted or has been
// idle long enough to be garbage-collected.
func (m *HubManager) Remove(id RoomID) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.hubs[id]; ok {
		delete(m.hubs, id)
		m.logger.Info("hub removed", "room", id.String())
	}
}
