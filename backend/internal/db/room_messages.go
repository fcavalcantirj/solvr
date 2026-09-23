package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// ErrMessageNotFound is returned when a message lookup finds no matching,
// non-deleted message in the given room.
var ErrMessageNotFound = errors.New("message not found")

// messageColumns is the shared SELECT column list for reading a message. It
// intentionally omits client_entry_id, which is a write/lookup-only idempotency
// key and is never surfaced in read responses.
const messageColumns = `id, room_id, author_type, author_id, agent_name, content, content_type, metadata, reply_to_entry_id, addressed_member_ids, sequence_num, created_at`

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
	msg, _, err := r.CreateWithClientEntry(ctx, params)
	return msg, err
}

// CreateWithClientEntry inserts a message and reports whether a new row was created.
//
// When params.ClientEntryID is set for an authenticated author (AuthorID != nil),
// the write is idempotent: a retry with the same (room_id, author_id, client_entry_id)
// returns the already-persisted entry with created=false instead of duplicating it.
// This lets an agent that lost the response to its write safely retry without posting
// the message twice. Writes without a client entry id, or from a shared token
// (AuthorID == nil), always create a new row.
func (r *MessageRepository) CreateWithClientEntry(ctx context.Context, params models.CreateMessageParams) (*models.Message, bool, error) {
	dedupable := params.ClientEntryID != nil && *params.ClientEntryID != "" && params.AuthorID != nil

	if dedupable {
		existing, err := r.getByClientEntryID(ctx, params.RoomID, *params.AuthorID, *params.ClientEntryID)
		if err == nil {
			return existing, false, nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return nil, false, err
		}
	}

	query := `
		INSERT INTO messages (room_id, author_type, author_id, agent_name, content, content_type, metadata, reply_to_entry_id, addressed_member_ids, client_entry_id, sequence_num)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10,
			(SELECT COALESCE(MAX(sequence_num), 0) + 1 FROM messages WHERE room_id = $1 AND deleted_at IS NULL)
		)
		RETURNING id, room_id, author_type, author_id, agent_name, content, content_type, metadata, reply_to_entry_id, addressed_member_ids, sequence_num, created_at, deleted_at
	`

	// Default metadata to empty JSON object if nil (DB column is NOT NULL DEFAULT '{}')
	metadata := params.Metadata
	if metadata == nil {
		metadata = json.RawMessage(`{}`)
	}

	var msg models.Message
	err := r.pool.QueryRow(ctx, query,
		params.RoomID,
		params.AuthorType,
		params.AuthorID,
		params.AgentName,
		params.Content,
		params.ContentType,
		metadata,
		params.ReplyToEntryID,
		params.AddressedMemberIDs,
		params.ClientEntryID,
	).Scan(
		&msg.ID,
		&msg.RoomID,
		&msg.AuthorType,
		&msg.AuthorID,
		&msg.AgentName,
		&msg.Content,
		&msg.ContentType,
		&msg.Metadata,
		&msg.ReplyToEntryID,
		&msg.AddressedMemberIDs,
		&msg.SequenceNum,
		&msg.CreatedAt,
		&msg.DeletedAt,
	)
	if err != nil {
		// A concurrent identical retry races past the pre-check above; the unique
		// index rejects the second insert. Return the entry the winner persisted.
		var pgErr *pgconn.PgError
		if dedupable && errors.As(err, &pgErr) && pgErr.Code == "23505" {
			if existing, gerr := r.getByClientEntryID(ctx, params.RoomID, *params.AuthorID, *params.ClientEntryID); gerr == nil {
				return existing, false, nil
			}
		}
		LogQueryError(ctx, "Create", "messages", err)
		return nil, false, err
	}

	return &msg, true, nil
}

// getByClientEntryID returns the non-deleted message an author already persisted
// under a client entry id in a room, or pgx.ErrNoRows if none exists.
func (r *MessageRepository) getByClientEntryID(ctx context.Context, roomID uuid.UUID, authorID, clientEntryID string) (*models.Message, error) {
	query := `SELECT ` + messageColumns + `
		FROM messages
		WHERE room_id = $1 AND author_id = $2 AND client_entry_id = $3 AND deleted_at IS NULL
		ORDER BY id ASC
		LIMIT 1`

	var msg models.Message
	err := r.pool.QueryRow(ctx, query, roomID, authorID, clientEntryID).Scan(
		&msg.ID,
		&msg.RoomID,
		&msg.AuthorType,
		&msg.AuthorID,
		&msg.AgentName,
		&msg.Content,
		&msg.ContentType,
		&msg.Metadata,
		&msg.ReplyToEntryID,
		&msg.AddressedMemberIDs,
		&msg.SequenceNum,
		&msg.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &msg, nil
}

// GetByID returns a single non-deleted message by id, scoped to the room so a deep
// link cannot fetch another room's content. Returns ErrMessageNotFound when the id
// does not exist in the room (or is deleted).
func (r *MessageRepository) GetByID(ctx context.Context, roomID uuid.UUID, id int64) (*models.Message, error) {
	query := `SELECT ` + messageColumns + `
		FROM messages
		WHERE room_id = $1 AND id = $2 AND deleted_at IS NULL`

	var msg models.Message
	err := r.pool.QueryRow(ctx, query, roomID, id).Scan(
		&msg.ID,
		&msg.RoomID,
		&msg.AuthorType,
		&msg.AuthorID,
		&msg.AgentName,
		&msg.Content,
		&msg.ContentType,
		&msg.Metadata,
		&msg.ReplyToEntryID,
		&msg.AddressedMemberIDs,
		&msg.SequenceNum,
		&msg.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrMessageNotFound
		}
		LogQueryError(ctx, "GetByID", "messages", err)
		return nil, err
	}
	return &msg, nil
}

// ListBefore returns up to `limit` non-deleted messages with id < beforeID, in
// chronological (ascending) order. It powers "load earlier history" without gaps:
// the caller anchors on the oldest id it holds and pages backwards.
func (r *MessageRepository) ListBefore(ctx context.Context, roomID uuid.UUID, beforeID int64, limit int) ([]models.Message, error) {
	if limit <= 0 {
		limit = 100
	}

	query := `SELECT ` + messageColumns + `
		FROM messages
		WHERE room_id = $1 AND id < $2 AND deleted_at IS NULL
		ORDER BY id DESC
		LIMIT $3`

	rows, err := r.pool.Query(ctx, query, roomID, beforeID, limit)
	if err != nil {
		LogQueryError(ctx, "ListBefore", "messages", err)
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
			&msg.ReplyToEntryID,
			&msg.AddressedMemberIDs,
			&msg.SequenceNum,
			&msg.CreatedAt,
		)
		if err != nil {
			LogQueryError(ctx, "ListBefore.Scan", "messages", err)
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

	// Reverse to chronological order (the query fetches DESC, we want ASC).
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}

	return messages, nil
}

// ListAfter returns messages after a given ID using cursor-based pagination.
// If afterID is 0, returns from the beginning. Default limit is 100 if not specified.
func (r *MessageRepository) ListAfter(ctx context.Context, roomID uuid.UUID, afterID int64, limit int) ([]models.Message, error) {
	if limit <= 0 {
		limit = 100
	}

	query := `
		SELECT id, room_id, author_type, author_id, agent_name, content, content_type, metadata, reply_to_entry_id, addressed_member_ids, sequence_num, created_at
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
			&msg.ReplyToEntryID,
			&msg.AddressedMemberIDs,
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
		SELECT id, room_id, author_type, author_id, agent_name, content, content_type, metadata, reply_to_entry_id, addressed_member_ids, sequence_num, created_at
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
			&msg.ReplyToEntryID,
			&msg.AddressedMemberIDs,
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

// GetFirstMessage returns the earliest non-deleted message in a room.
// Used by the room-specific connect endpoint to infer the planner identity and the
// initial task from the room's first author.
func (r *MessageRepository) GetFirstMessage(ctx context.Context, roomID uuid.UUID) (*models.Message, error) {
	query := `
		SELECT id, room_id, author_type, author_id, agent_name, content, content_type, metadata,
		       reply_to_entry_id, addressed_member_ids, sequence_num, created_at
		FROM messages
		WHERE room_id = $1 AND deleted_at IS NULL
		ORDER BY id ASC
		LIMIT 1
	`

	var msg models.Message
	err := r.pool.QueryRow(ctx, query, roomID).Scan(
		&msg.ID,
		&msg.RoomID,
		&msg.AuthorType,
		&msg.AuthorID,
		&msg.AgentName,
		&msg.Content,
		&msg.ContentType,
		&msg.Metadata,
		&msg.ReplyToEntryID,
		&msg.AddressedMemberIDs,
		&msg.SequenceNum,
		&msg.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrRoomNotFound
		}
		LogQueryError(ctx, "GetFirstMessage", "messages", err)
		return nil, err
	}
	return &msg, nil
}
