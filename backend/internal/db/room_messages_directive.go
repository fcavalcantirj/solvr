package db

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// directiveChainMaxDepth bounds the supersede walk; a real chain is a handful of revisions.
const directiveChainMaxDepth = 200

// LatestDirective returns the directive in force (idx 92 step 1): the room's newest
// non-deleted pinned message, followed through the messages that supersede it (each
// revision supersedes the latest one) to the newest non-deleted revision. A revision
// therefore replaces a pinned directive without a re-pin, and a deleted revision falls
// back to the one before it. ErrMessageNotFound when the room has no pinned message.
//
// ListPinned (the /r/{slug}/pins listing) is unchanged: it lists what was pinned.
func (r *MessageRepository) LatestDirective(ctx context.Context, roomID uuid.UUID) (*models.Message, error) {
	query := `
		WITH RECURSIVE pin AS (
			SELECT id FROM messages
			 WHERE room_id = $1 AND pinned_at IS NOT NULL AND deleted_at IS NULL
			 ORDER BY pinned_at DESC, id DESC
			 LIMIT 1
		), chain AS (
			SELECT pin.id, 0 AS depth FROM pin
			UNION ALL
			SELECT s.id, chain.depth + 1
			  FROM messages s JOIN chain ON s.supersedes_entry_id = chain.id
			 WHERE s.room_id = $1 AND s.deleted_at IS NULL AND chain.depth < $2
		)
		SELECT ` + messageColumns + `
		  FROM messages
		 WHERE id = (SELECT id FROM chain ORDER BY depth DESC, id DESC LIMIT 1)`

	msg, err := scanMessage(r.pool.QueryRow(ctx, query, roomID, directiveChainMaxDepth))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrMessageNotFound
		}
		LogQueryError(ctx, "LatestDirective", "messages", err)
		return nil, err
	}
	return &msg, nil
}
