package db

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Room activity is a projection of the room timeline (migration 000112):
//
//   - rooms.message_count is the room's live (not soft-deleted) message entries;
//   - rooms.last_active_at is the later of the room's created_at and its newest message
//     entry (a soft-deleted message still happened, so it still counts as activity).
//
// Triggers on room_entries keep both in step with the timeline inside the same transaction
// as the entry write, so the API never bumps them itself and a caller that disappears after
// the insert cannot leave them behind. RebuildActivity recomputes them from room_entries;
// ActivityDrift lists the rooms whose stored values differ (read-only reconciliation).

// RoomActivityDrift is one room whose stored activity differs from its timeline.
type RoomActivityDrift struct {
	RoomID               uuid.UUID
	StoredMessageCount   int
	TimelineMessageCount int
	StoredLastActiveAt   time.Time
	TimelineLastActiveAt time.Time
}

// ActivityDrift returns the rooms (one room when roomID is set) whose stored
// message_count or last_active_at differs from what their timeline says, ordered by id.
func (r *RoomRepository) ActivityDrift(ctx context.Context, roomID *uuid.UUID) ([]RoomActivityDrift, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT room_id, stored_message_count, timeline_message_count,
		       stored_last_active_at, timeline_last_active_at
		FROM room_activity_drift($1)`, roomID)
	if err != nil {
		LogQueryError(ctx, "ActivityDrift", "rooms", err)
		return nil, err
	}
	defer rows.Close()
	var out []RoomActivityDrift
	for rows.Next() {
		var d RoomActivityDrift
		if err := rows.Scan(&d.RoomID, &d.StoredMessageCount, &d.TimelineMessageCount,
			&d.StoredLastActiveAt, &d.TimelineLastActiveAt); err != nil {
			return nil, fmt.Errorf("scan room activity drift: %w", err)
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// RebuildActivity recomputes message_count and last_active_at from the timeline for one
// room, or for every room when roomID is nil, and returns how many rooms it repaired.
// A consistent room is not rewritten, so a repeated rebuild returns 0.
func (r *RoomRepository) RebuildActivity(ctx context.Context, roomID *uuid.UUID) (int, error) {
	var repaired int
	if err := r.pool.QueryRow(ctx, `SELECT rebuild_room_activity($1)`, roomID).Scan(&repaired); err != nil {
		LogQueryError(ctx, "RebuildActivity", "rooms", err)
		return 0, err
	}
	return repaired, nil
}
