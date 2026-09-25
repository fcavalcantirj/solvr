package db

import (
	"context"
	"sync"
)

// OverviewChannel carries "the public overview changed" notices between API instances: a
// room went private, was archived, reopened or deleted. Each instance drops its cached
// overview snapshots on a notice. Like RoomPresenceChannel it is not a durable record:
// the snapshots are re-read from the database, and a notice lost while an instance's
// listener was down is covered by dropping the snapshots on every (re)LISTEN.
const OverviewChannel = "solvr_overview_changed"

// overviewHooks are the local snapshot drops registered on one Pool (one API instance).
type overviewHooks struct {
	mu  sync.RWMutex
	fns []func()
}

func (h *overviewHooks) run() {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, fn := range h.fns {
		fn()
	}
}

// OnOverviewChanged registers fn to run whenever the public overview changes, on this
// instance or on any other sharing the database (the latter needs ListenRooms running).
func (p *Pool) OnOverviewChanged(fn func()) {
	p.overview.mu.Lock()
	defer p.overview.mu.Unlock()
	p.overview.fns = append(p.overview.fns, fn)
}

// OverviewChanged runs this instance's OnOverviewChanged hooks at once, then notifies the
// other instances. Call it after the change is committed.
func (p *Pool) OverviewChanged(ctx context.Context) error {
	p.overview.run()
	_, err := p.pool.Exec(ctx, "SELECT pg_notify($1, '')", OverviewChannel)
	return err
}
