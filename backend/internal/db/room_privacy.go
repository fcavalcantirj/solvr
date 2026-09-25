package db

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

// IsPrivateRoom reports whether the room identified by a UUID string is private (is_private).
// A soft-deleted room still reports its privacy: deleting a private room must not make what
// was saved from it public. A room that no longer exists at all (hard-deleted, e.g. by the
// expiry reaper) reports false with ErrRoomNotFound so callers can decide.
func (r *RoomRepository) IsPrivateRoom(ctx context.Context, roomID string) (bool, error) {
	var isPrivate bool
	err := r.pool.QueryRow(ctx,
		`SELECT is_private FROM rooms WHERE id = $1::uuid`,
		roomID,
	).Scan(&isPrivate)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, ErrRoomNotFound
		}
		return false, err
	}
	return isPrivate, nil
}
