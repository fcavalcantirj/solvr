package db

import (
	"context"
	"fmt"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/google/uuid"
)

// MessageRepository handles database operations for room messages.
type MessageRepository struct {
	pool *Pool
}

// NewMessageRepository creates a new MessageRepository.
func NewMessageRepository(pool *Pool) *MessageRepository {
	return &MessageRepository{pool: pool}
}

// Create inserts a new message with a concurrent-safe sequence_num.
// The sequence_num is assigned via a subquery: COALESCE(MAX(sequence_num), 0) + 1
// within the same INSERT, which is serialized by PostgreSQL under concurrent writes.
func (r *MessageRepository) Create(ctx context.Context, params models.CreateMessageParams) (*models.Message, error) {
	query := `
		INSERT INTO messages (room_id, author_type, author_id, agent_name, content, content_type, metadata, sequence_num)
		VALUES ($1, $2, $3, $4, $5, $6, $7,
			(SELECT COALESCE(MAX(sequence_num), 0) + 1 FROM messages WHERE room_id = $1 AND deleted_at IS NULL)
		)
		RETURNING id, room_id, author_type, author_id, agent_name, content, content_type, metadata, sequence_num, created_at, deleted_at
	`

	var msg models.Message
	err := r.pool.QueryRow(ctx, query,
		params.RoomID,
		params.AuthorType,
		params.AuthorID,
		params.AgentName,
		params.Content,
		params.ContentType,
		params.Metadata,
	).Scan(
		&msg.ID,
		&msg.RoomID,
		&msg.AuthorType,
		&msg.AuthorID,
		&msg.AgentName,
		&msg.Content,
		&msg.ContentType,
		&msg.Metadata,
		&msg.SequenceNum,
		&msg.CreatedAt,
		&msg.DeletedAt,
	)
	if err != nil {
		LogQueryError(ctx, "Create", "messages", err)
		return nil, err
	}

	return &msg, nil
}

// ListAfter returns messages after a given ID using cursor-based pagination.
// If afterID is 0, returns from the beginning. Default limit is 100 if not specified.
func (r *MessageRepository) ListAfter(ctx context.Context, roomID uuid.UUID, afterID int64, limit int) ([]models.Message, error) {
	if limit <= 0 {
		limit = 100
	}

	query := `
		SELECT id, room_id, author_type, author_id, agent_name, content, content_type, metadata, sequence_num, created_at
		FROM messages
		WHERE room_id = $1 AND id > $2 AND deleted_at IS NULL
		ORDER BY id ASC
		LIMIT $3
	`

	rows, err := r.pool.Query(ctx, query, roomID, afterID, limit)
	if err != nil {
		LogQueryError(ctx, "ListAfter", "messages", err)
		return nil, err
	}
	defer rows.Close()

	var messages []models.Message
	for rows.Next() {
		var msg models.Message
		err := rows.Scan(
			&msg.ID,
			&msg.RoomID,
			&msg.AuthorType,
			&msg.AuthorID,
			&msg.AgentName,
			&msg.Content,
			&msg.ContentType,
			&msg.Metadata,
			&msg.SequenceNum,
			&msg.CreatedAt,
		)
		if err != nil {
			LogQueryError(ctx, "ListAfter.Scan", "messages", err)
			return nil, fmt.Errorf("scan message: %w", err)
		}
		messages = append(messages, msg)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	if messages == nil {
		messages = []models.Message{}
	}

	return messages, nil
}

// ListRecent returns the most recent messages for a room, in chronological order.
// Fetches the N most recent (ORDER BY id DESC LIMIT $2), then reverses for chronological output.
func (r *MessageRepository) ListRecent(ctx context.Context, roomID uuid.UUID, limit int) ([]models.Message, error) {
	if limit <= 0 {
		limit = 100
	}

	query := `
		SELECT id, room_id, author_type, author_id, agent_name, content, content_type, metadata, sequence_num, created_at
		FROM messages
		WHERE room_id = $1 AND deleted_at IS NULL
		ORDER BY id DESC
		LIMIT $2
	`

	rows, err := r.pool.Query(ctx, query, roomID, limit)
	if err != nil {
		LogQueryError(ctx, "ListRecent", "messages", err)
		return nil, err
	}
	defer rows.Close()

	var messages []models.Message
	for rows.Next() {
		var msg models.Message
		err := rows.Scan(
			&msg.ID,
			&msg.RoomID,
			&msg.AuthorType,
			&msg.AuthorID,
			&msg.AgentName,
			&msg.Content,
			&msg.ContentType,
			&msg.Metadata,
			&msg.SequenceNum,
			&msg.CreatedAt,
		)
		if err != nil {
			LogQueryError(ctx, "ListRecent.Scan", "messages", err)
			return nil, fmt.Errorf("scan message: %w", err)
		}
		messages = append(messages, msg)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	if messages == nil {
		messages = []models.Message{}
	}

	// Reverse to chronological order (the query fetches DESC, we want ASC)
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}

	return messages, nil
}
