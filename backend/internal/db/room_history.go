package db

import (
	"context"
	"fmt"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/google/uuid"
)

// ListSequenceRange returns a room's live messages whose sequence falls in [from, to],
// in sequence order: one fixed segment of the crawlable transcript (task idx 81).
// Events and deleted messages are left out; they never shift the range.
func (r *MessageRepository) ListSequenceRange(ctx context.Context, roomID uuid.UUID, from, to int) ([]models.Message, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT `+messageColumns+`
		FROM messages
		WHERE room_id = $1 AND sequence_num BETWEEN $2 AND $3 AND deleted_at IS NULL
		ORDER BY sequence_num ASC
	`, roomID, from, to)
	if err != nil {
		LogQueryError(ctx, "ListSequenceRange", "messages", err)
		return nil, err
	}
	defer rows.Close()

	messages := []models.Message{}
	for rows.Next() {
		msg, err := scanMessage(rows)
		if err != nil {
			LogQueryError(ctx, "ListSequenceRange.Scan", "messages", err)
			return nil, fmt.Errorf("scan message: %w", err)
		}
		messages = append(messages, msg)
	}
	return messages, rows.Err()
}
