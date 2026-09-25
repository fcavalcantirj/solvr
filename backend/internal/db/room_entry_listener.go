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

// ListenRoomEntries holds one dedicated connection (outside the pool, so it never takes a
// pool slot) LISTENing on RoomEntryChannel, and calls onEntry with the room id of each
// notification. It blocks until ctx ends. When the connection drops it reconnects after
// backoff and calls onListening again: notifications sent while it was down are lost, so
// onListening is where the caller catches up from its cursors. onListening also runs
// after the first LISTEN, which is when the instance starts hearing other instances.
func (p *Pool) ListenRoomEntries(ctx context.Context, applicationName string, backoff time.Duration, onListening func(), onEntry func(roomID uuid.UUID)) {
	for ctx.Err() == nil {
		err := p.listenOnce(ctx, applicationName, onListening, onEntry)
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

func (p *Pool) listenOnce(ctx context.Context, applicationName string, onListening func(), onEntry func(uuid.UUID)) error {
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
	onListening()
	for {
		n, err := conn.WaitForNotification(ctx)
		if err != nil {
			return err
		}
		if id, perr := uuid.Parse(n.Payload); perr == nil {
			onEntry(id)
		}
	}
}
