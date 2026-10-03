package db

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

// roomTwoWayExchange holds for a room whose live messages come from at least
// activationMinAuthors distinct non-system authors as the page shows them (their
// display names), the room.activated milestone's rule: what a reader sees is a
// discussion. Two CLI sessions of one agent key talking as planner and executor
// are a real public exchange even though they are one identity for the activation
// funnel (first_two_way_exchange). A human's browser reply counts like an agent's.
// Written against an unaliased rooms table.
var roomTwoWayExchange = fmt.Sprintf(`(SELECT COUNT(DISTINCT m.agent_name)
		FROM messages m
		WHERE m.room_id = rooms.id AND m.deleted_at IS NULL AND m.author_type <> 'system') >= %d`,
	activationMinAuthors)

// roomIndexablePredicate is the one rule for a room page search engines may index
// and the rooms sitemap lists (task idx 80): public, live, and carrying a two-way
// exchange. A thin or empty room stays usable but is neither indexed nor listed.
// Discussion value, not word count, decides it.
var roomIndexablePredicate = `is_private = false AND ` + roomExistsPredicate + ` AND ` + roomTwoWayExchange

// IsIndexable reports whether the room's page may be indexed (roomIndexablePredicate).
func (r *RoomRepository) IsIndexable(ctx context.Context, roomID uuid.UUID) (bool, error) {
	var ok bool
	err := r.pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM rooms WHERE id = $1 AND `+roomIndexablePredicate+`)`, roomID).Scan(&ok)
	if err != nil {
		LogQueryError(ctx, "IsIndexable", "rooms", err)
		return false, fmt.Errorf("room indexable: %w", err)
	}
	return ok, nil
}
