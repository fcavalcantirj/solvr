package db

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

// IsPrivateRoom reports whether the room identified by a UUID string is private (is_private).
// A missing or deleted room reports false with ErrRoomNotFound so callers can decide; the
// post publish-gate treats a lookup error as "do not block" to stay fail-open for edits.
func (r *RoomRepository) IsPrivateRoom(ctx context.Context, roomID string) (bool, error) {
	var isPrivate bool
	err := r.pool.QueryRow(ctx,
		`SELECT is_private FROM rooms WHERE id = $1::uuid AND deleted_at IS NULL`,
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
