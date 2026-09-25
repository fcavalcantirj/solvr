package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/a2aproject/a2a-go/a2a"
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
// Presence changes made here are announced to the other instances, and theirs are shown
// on this instance's streams (see presenceRelay).
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
	onPresence := presenceRelay(ctx, pool, hubMgr)

	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		pool.ListenRooms(ctx, RoomRelayApplicationName, backoff, db.RoomListener{
			OnListening: onListening, OnEntry: onEntry, OnPresence: onPresence,
		})
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

// presenceNotifyTimeout bounds sending one presence change notice.
const presenceNotifyTimeout = 5 * time.Second

// presenceRelay makes hubMgr announce the presence changes made on this instance to the
// others, and returns the handler that shows theirs here. A remote join carries the
// agent's card read from its live presence row, as a local join does; it is read only
// when this instance has a hub (someone to show it to) for the room.
func presenceRelay(ctx context.Context, pool *db.Pool, hubMgr *hub.HubManager) func(payload string) {
	hubMgr.SetPresenceNotifier(func(c hub.PresenceChange) {
		payload, err := json.Marshal(c)
		if err != nil {
			return
		}
		nctx, cancel := context.WithTimeout(ctx, presenceNotifyTimeout)
		defer cancel()
		if err := pool.NotifyRoomPresence(nctx, string(payload)); err != nil {
			slog.Warn("presence change not announced to other instances", "error", err,
				"room", c.RoomID.String(), "agent", c.AgentName)
		}
	})
	presenceRepo := db.NewAgentPresenceRepository(pool)
	return func(payload string) {
		var c hub.PresenceChange
		if err := json.Unmarshal([]byte(payload), &c); err != nil {
			return
		}
		var card *a2a.AgentCard
		if c.Joined && c.Origin != hubMgr.InstanceID() && hubMgr.Get(c.RoomID) != nil {
			raw, found, err := presenceRepo.LiveCard(ctx, c.RoomID.UUID(), c.AgentName)
			if err == nil && found && len(raw) > 0 {
				var parsed a2a.AgentCard
				if json.Unmarshal(raw, &parsed) == nil {
					card = &parsed
				}
			}
		}
		hubMgr.ApplyRemotePresence(c, card)
	}
}
