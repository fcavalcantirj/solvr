package api

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/hub"
)

// RoomRelayApplicationName is the Postgres application_name of each instance's room
// relay listener connection (visible in pg_stat_activity).
const RoomRelayApplicationName = "solvr-room-relay"

// Room relay defaults.
const (
	defaultRelayReconnectBackoff = 2 * time.Second
	defaultRelaySweepInterval    = 30 * time.Second
)

// RoomRelayOptions tunes the cross-instance room relay. Zero values take the defaults.
type RoomRelayOptions struct {
	// ReconnectBackoff is the wait before re-LISTENing after the listener connection drops.
	ReconnectBackoff time.Duration
	// SweepInterval is how often every local room re-reads its cursor even without a
	// notification, bounding how long any missed wakeup can delay delivery. Negative
	// disables the sweep.
	SweepInterval time.Duration
}

// RoomRelay is a running cross-instance room relay.
type RoomRelay struct {
	ready chan struct{}
	wg    sync.WaitGroup
}

// Ready is closed once the instance first listens for other instances' entries.
func (r *RoomRelay) Ready() <-chan struct{} { return r.ready }

// Wait blocks until the relay stopped (its context ended).
func (r *RoomRelay) Wait() { r.wg.Wait() }

// StartRoomRelay connects this API instance to every other one sharing the database.
// Every committed timeline entry notifies (migration 000103); the listener turns each
// notification into a wakeup of that room's relay drain in hubMgr, which reads the
// committed entries after its cursor. After a reconnect, and on every sweep, all local
// rooms catch up from their cursors, so a lost notification delays an entry but never
// loses or duplicates it. The router must have enabled the hub relay (mountRoomRoutes).
func StartRoomRelay(ctx context.Context, pool *db.Pool, hubMgr *hub.HubManager, opts RoomRelayOptions) *RoomRelay {
	backoff := opts.ReconnectBackoff
	if backoff <= 0 {
		backoff = defaultRelayReconnectBackoff
	}
	sweep := opts.SweepInterval
	if sweep == 0 {
		sweep = defaultRelaySweepInterval
	}

	r := &RoomRelay{ready: make(chan struct{})}
	var readyOnce sync.Once
	onListening := func() {
		readyOnce.Do(func() { close(r.ready) })
		hubMgr.WakeAll()
	}
	onEntry := func(roomID uuid.UUID) { hubMgr.Announce(hub.NewRoomID(roomID)) }

	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		pool.ListenRoomEntries(ctx, RoomRelayApplicationName, backoff, onListening, onEntry)
	}()
	if sweep > 0 {
		r.wg.Add(1)
		go func() {
			defer r.wg.Done()
			ticker := time.NewTicker(sweep)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					hubMgr.WakeAll()
				}
			}
		}()
	}
	return r
}
