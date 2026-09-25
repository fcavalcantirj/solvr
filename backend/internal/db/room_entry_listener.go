package db

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// RoomEntryChannel is the notification channel migration 000103 signals, at commit, with
// the room id of every inserted timeline entry.
const RoomEntryChannel = "solvr_room_entries"

// RoomPresenceChannel carries presence change notices between API instances (see
// hub.PresenceChange). Unlike RoomEntryChannel these are not wakeups for a durable
// record: presence is read from agent_presence, and a lost notice only costs a live
// stream one presence frame.
const RoomPresenceChannel = "solvr_room_presence"

// NotifyRoomPresence sends payload on RoomPresenceChannel to every listening instance.
func (p *Pool) NotifyRoomPresence(ctx context.Context, payload string) error {
	_, err := p.pool.Exec(ctx, "SELECT pg_notify($1, $2)", RoomPresenceChannel, payload)
	return err
}

// RoomListener receives the room notifications of every instance sharing the database.
type RoomListener struct {
	// OnListening runs after every (re)LISTEN; see ListenRooms.
	OnListening func()
	// OnEntry receives the room id of each committed timeline entry.
	OnEntry func(roomID uuid.UUID)
	// OnPresence receives each presence change notice payload (nil: not listened to).
	OnPresence func(payload string)
}

// ListenRoomEntries is ListenRooms for timeline entries only.
func (p *Pool) ListenRoomEntries(ctx context.Context, applicationName string, backoff time.Duration, onListening func(), onEntry func(roomID uuid.UUID)) {
	p.ListenRooms(ctx, applicationName, backoff, RoomListener{OnListening: onListening, OnEntry: onEntry})
}

// ListenRooms holds one dedicated connection (outside the pool, so it never takes a pool
// slot) LISTENing on RoomEntryChannel (and RoomPresenceChannel when l.OnPresence is set),
// and hands each notification to l. It also LISTENs on OverviewChannel and runs the
// pool's OnOverviewChanged hooks for each notice and after every (re)LISTEN. It blocks until ctx ends. When the connection drops
// it reconnects after backoff and calls l.OnListening again: notifications sent while it
// was down are lost, so OnListening is where the caller catches up from its cursors.
// OnListening also runs after the first LISTEN, which is when the instance starts hearing
// other instances.
func (p *Pool) ListenRooms(ctx context.Context, applicationName string, backoff time.Duration, l RoomListener) {
	for ctx.Err() == nil {
		err := p.listenOnce(ctx, applicationName, l)
		if ctx.Err() != nil {
			return
		}
		slog.Warn("room entry listener disconnected; reconnecting", "error", err, "backoff", backoff)
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
	}
}

func (p *Pool) listenOnce(ctx context.Context, applicationName string, l RoomListener) error {
	cfg := p.pool.Config().ConnConfig.Copy()
	if cfg.RuntimeParams == nil {
		cfg.RuntimeParams = map[string]string{}
	}
	cfg.RuntimeParams["application_name"] = applicationName
	conn, err := pgx.ConnectConfig(ctx, cfg)
	if err != nil {
		return err
	}
	defer conn.Close(context.Background())
	if _, err := conn.Exec(ctx, "LISTEN "+RoomEntryChannel); err != nil {
		return err
	}
	if l.OnPresence != nil {
		if _, err := conn.Exec(ctx, "LISTEN "+RoomPresenceChannel); err != nil {
			return err
		}
	}
	if _, err := conn.Exec(ctx, "LISTEN "+OverviewChannel); err != nil {
		return err
	}
	// Overview notices sent while the listener was down are lost: drop the snapshots.
	p.overview.run()
	l.OnListening()
	for {
		n, err := conn.WaitForNotification(ctx)
		if err != nil {
			return err
		}
		if n.Channel == OverviewChannel {
			p.overview.run()
			continue
		}
		if n.Channel == RoomPresenceChannel {
			l.OnPresence(n.Payload)
			continue
		}
		if id, perr := uuid.Parse(n.Payload); perr == nil {
			l.OnEntry(id)
		}
	}
}
