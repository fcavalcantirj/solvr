package db

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/fcavalcantirj/solvr/internal/token"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// ErrRoomSlugExists is returned when a room with the same slug already exists.
var ErrRoomSlugExists = errors.New("room slug already exists")

// ErrRoomNotFound is returned when a room is not found.
var ErrRoomNotFound = errors.New("room not found")

// RoomRepository handles database operations for rooms.
type RoomRepository struct {
	pool *Pool
}

// NewRoomRepository creates a new RoomRepository.
func NewRoomRepository(pool *Pool) *RoomRepository {
	return &RoomRepository{pool: pool}
}

// slugify generates a URL-safe slug from a display name (no random suffix).
func slugify(name string) string {
	s := strings.ToLower(name)
	re := regexp.MustCompile(`[^a-z0-9]+`)
	s = re.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if len(s) > 40 {
		s = s[:40]
	}
	return s
}

// Create inserts a new room into the database.
// Returns the created room and the plaintext bearer token (to give to the creator).
func (r *RoomRepository) Create(ctx context.Context, params models.CreateRoomParams) (*models.Room, string, error) {
	// Generate slug if not provided
	slug := params.Slug
	if slug == "" {
		slug = slugify(params.DisplayName)
	}

	// Generate bearer token
	plaintext, hashHex, err := token.GenerateRoomToken()
	if err != nil {
		return nil, "", fmt.Errorf("generate room token: %w", err)
	}

	// Handle nil owner (uuid.Nil means no owner)
	var ownerID *uuid.UUID
	if params.OwnerID != uuid.Nil {
		ownerID = &params.OwnerID
	}

	// Normalize tags
	tags := params.Tags
	if tags == nil {
		tags = []string{}
	}

	query := `
		INSERT INTO rooms (slug, display_name, description, category, tags, is_private, owner_id, token_hash, message_count, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 0, $9)
		RETURNING id, slug, display_name, description, category, tags, is_private, owner_id, token_hash,
			message_count, created_at, updated_at, last_active_at, expires_at, deleted_at
	`

	var room models.Room
	err = r.pool.QueryRow(ctx, query,
		slug,
		params.DisplayName,
		params.Description,
		params.Category,
		tags,
		params.IsPrivate,
		ownerID,
		hashHex,
		params.ExpiresAt,
	).Scan(
		&room.ID,
		&room.Slug,
		&room.DisplayName,
		&room.Description,
		&room.Category,
		&room.Tags,
		&room.IsPrivate,
		&room.OwnerID,
		&room.TokenHash,
		&room.MessageCount,
		&room.CreatedAt,
		&room.UpdatedAt,
		&room.LastActiveAt,
		&room.ExpiresAt,
		&room.DeletedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, "", ErrRoomSlugExists
		}
		LogQueryError(ctx, "Create", "rooms", err)
		return nil, "", err
	}

	return &room, plaintext, nil
}

// GetBySlug returns a room by its slug.
// Returns ErrRoomNotFound if the room doesn't exist or is soft-deleted.
func (r *RoomRepository) GetBySlug(ctx context.Context, slug string) (*models.Room, error) {
	query := `
		SELECT id, slug, display_name, description, category, tags, is_private, owner_id, token_hash,
			message_count, created_at, updated_at, last_active_at, expires_at, deleted_at
		FROM rooms
		WHERE slug = $1 AND deleted_at IS NULL
	`
	return r.scanRoom(ctx, "GetBySlug", query, slug)
}

// GetByID returns a room by its ID.
// Returns ErrRoomNotFound if the room doesn't exist or is soft-deleted.
func (r *RoomRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Room, error) {
	query := `
		SELECT id, slug, display_name, description, category, tags, is_private, owner_id, token_hash,
			message_count, created_at, updated_at, last_active_at, expires_at, deleted_at
		FROM rooms
		WHERE id = $1 AND deleted_at IS NULL
	`
	return r.scanRoom(ctx, "GetByID", query, id)
}

// GetByTokenHash returns a room by its token hash.
// Returns ErrRoomNotFound if the room doesn't exist or is soft-deleted.
// Used by bearer guard middleware.
func (r *RoomRepository) GetByTokenHash(ctx context.Context, hash string) (*models.Room, error) {
	query := `
		SELECT id, slug, display_name, description, category, tags, is_private, owner_id, token_hash,
			message_count, created_at, updated_at, last_active_at, expires_at, deleted_at
		FROM rooms
		WHERE token_hash = $1 AND deleted_at IS NULL
	`
	return r.scanRoom(ctx, "GetByTokenHash", query, hash)
}

// List returns public rooms with live agent count, unique participant count, and owner display name,
// ordered by last_active_at DESC. Uses correlated subqueries (no N+1 per D-34).
// Includes unique_participant_count (D-05) and owner_display_name (D-10) per Phase 16 Plan 01.
func (r *RoomRepository) List(ctx context.Context, limit, offset int) ([]models.RoomWithStats, error) {
	query := `
		SELECT r.id, r.slug, r.display_name, r.description, r.category, r.tags,
			r.is_private, r.owner_id, r.message_count, r.created_at, r.updated_at,
			r.last_active_at, r.expires_at,
			(SELECT COUNT(DISTINCT agent_name) FROM agent_presence ap
			 WHERE ap.room_id = r.id
			   AND ap.last_seen > NOW() - (ap.ttl_seconds || ' seconds')::interval
			) AS live_agent_count,
			(SELECT COUNT(DISTINCT author_id) FROM messages m
			 WHERE m.room_id = r.id AND m.deleted_at IS NULL AND m.author_id IS NOT NULL
			) AS unique_participant_count,
			u.display_name AS owner_display_name
		FROM rooms r
		LEFT JOIN users u ON u.id = r.owner_id
		WHERE r.deleted_at IS NULL AND r.is_private = FALSE
		ORDER BY r.last_active_at DESC
		LIMIT $1 OFFSET $2
	`

	rows, err := r.pool.Query(ctx, query, limit, offset)
	if err != nil {
		LogQueryError(ctx, "List", "rooms", err)
		return nil, err
	}
	defer rows.Close()

	var rooms []models.RoomWithStats
	for rows.Next() {
		var rws models.RoomWithStats
		err := rows.Scan(
			&rws.ID,
			&rws.Slug,
			&rws.DisplayName,
			&rws.Description,
			&rws.Category,
			&rws.Tags,
			&rws.IsPrivate,
			&rws.OwnerID,
			&rws.MessageCount,
			&rws.CreatedAt,
			&rws.UpdatedAt,
			&rws.LastActiveAt,
			&rws.ExpiresAt,
			&rws.LiveAgentCount,
			&rws.UniqueParticipantCount,
			&rws.OwnerDisplayName,
		)
		if err != nil {
			LogQueryError(ctx, "List.Scan", "rooms", err)
			return nil, fmt.Errorf("scan room: %w", err)
		}
		rooms = append(rooms, rws)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	if rooms == nil {
		rooms = []models.RoomWithStats{}
	}

	return rooms, nil
}

// ListByOwner returns rooms owned by the specified user, ordered by created_at DESC.
func (r *RoomRepository) ListByOwner(ctx context.Context, ownerID uuid.UUID) ([]models.Room, error) {
	query := `
		SELECT id, slug, display_name, description, category, tags, is_private, owner_id, token_hash,
			message_count, created_at, updated_at, last_active_at, expires_at, deleted_at
		FROM rooms
		WHERE owner_id = $1 AND deleted_at IS NULL
		ORDER BY created_at DESC
	`

	rows, err := r.pool.Query(ctx, query, ownerID)
	if err != nil {
		LogQueryError(ctx, "ListByOwner", "rooms", err)
		return nil, err
	}
	defer rows.Close()

	var rooms []models.Room
	for rows.Next() {
		var room models.Room
		err := r.scanRoomRow(rows, &room)
		if err != nil {
			LogQueryError(ctx, "ListByOwner.Scan", "rooms", err)
			return nil, fmt.Errorf("scan room: %w", err)
		}
		rooms = append(rooms, room)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	if rooms == nil {
		rooms = []models.Room{}
	}

	return rooms, nil
}

// Update applies partial updates to a room, only modifying non-nil fields.
// Always sets updated_at = NOW().
func (r *RoomRepository) Update(ctx context.Context, roomID uuid.UUID, params models.UpdateRoomParams) (*models.Room, error) {
	// Build dynamic SET clause
	setClauses := []string{"updated_at = NOW()"}
	args := []any{}
	argIdx := 1

	if params.DisplayName != nil {
		setClauses = append(setClauses, fmt.Sprintf("display_name = $%d", argIdx))
		args = append(args, *params.DisplayName)
		argIdx++
	}
	if params.Description != nil {
		setClauses = append(setClauses, fmt.Sprintf("description = $%d", argIdx))
		args = append(args, *params.Description)
		argIdx++
	}
	if params.Category != nil {
		setClauses = append(setClauses, fmt.Sprintf("category = $%d", argIdx))
		args = append(args, *params.Category)
		argIdx++
	}
	if params.Tags != nil {
		setClauses = append(setClauses, fmt.Sprintf("tags = $%d", argIdx))
		args = append(args, params.Tags)
		argIdx++
	}
	if params.IsPrivate != nil {
		setClauses = append(setClauses, fmt.Sprintf("is_private = $%d", argIdx))
		args = append(args, *params.IsPrivate)
		argIdx++
	}

	query := fmt.Sprintf(`
		UPDATE rooms SET %s
		WHERE id = $%d AND deleted_at IS NULL
		RETURNING id, slug, display_name, description, category, tags, is_private, owner_id, token_hash,
			message_count, created_at, updated_at, last_active_at, expires_at, deleted_at
	`, strings.Join(setClauses, ", "), argIdx)
	args = append(args, roomID)

	return r.scanRoomFromRow(ctx, "Update", query, args...)
}

// SoftDelete sets deleted_at on a room.
func (r *RoomRepository) SoftDelete(ctx context.Context, roomID uuid.UUID) error {
	query := `UPDATE rooms SET deleted_at = NOW() WHERE id = $1 AND deleted_at IS NULL`
	result, err := r.pool.Exec(ctx, query, roomID)
	if err != nil {
		LogQueryError(ctx, "SoftDelete", "rooms", err)
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrRoomNotFound
	}
	return nil
}

// RotateToken generates a new bearer token for a room, replacing the old one.
// Returns the new plaintext token.
func (r *RoomRepository) RotateToken(ctx context.Context, roomID uuid.UUID) (string, error) {
	plaintext, hashHex, err := token.GenerateRoomToken()
	if err != nil {
		return "", fmt.Errorf("generate room token: %w", err)
	}

	query := `UPDATE rooms SET token_hash = $1, updated_at = NOW() WHERE id = $2 AND deleted_at IS NULL`
	result, err := r.pool.Exec(ctx, query, hashHex, roomID)
	if err != nil {
		LogQueryError(ctx, "RotateToken", "rooms", err)
		return "", err
	}
	if result.RowsAffected() == 0 {
		return "", ErrRoomNotFound
	}
	return plaintext, nil
}

// UpdateActivity updates last_active_at and updated_at on a room.
// Called after each message.
func (r *RoomRepository) UpdateActivity(ctx context.Context, roomID uuid.UUID) error {
	query := `UPDATE rooms SET last_active_at = NOW(), updated_at = NOW() WHERE id = $1`
	_, err := r.pool.Exec(ctx, query, roomID)
	if err != nil {
		LogQueryError(ctx, "UpdateActivity", "rooms", err)
		return err
	}
	return nil
}

// DeleteExpiredRooms deletes rooms where expires_at has passed.
// Returns the number of rooms deleted.
func (r *RoomRepository) DeleteExpiredRooms(ctx context.Context) (int64, error) {
	query := `DELETE FROM rooms WHERE expires_at IS NOT NULL AND expires_at < NOW()`
	result, err := r.pool.Exec(ctx, query)
	if err != nil {
		LogQueryError(ctx, "DeleteExpiredRooms", "rooms", err)
		return 0, err
	}
	return result.RowsAffected(), nil
}

// IncrementMessageCount atomically increments the message_count on a room.
func (r *RoomRepository) IncrementMessageCount(ctx context.Context, roomID uuid.UUID) error {
	query := `UPDATE rooms SET message_count = message_count + 1 WHERE id = $1`
	_, err := r.pool.Exec(ctx, query, roomID)
	if err != nil {
		LogQueryError(ctx, "IncrementMessageCount", "rooms", err)
		return err
	}
	return nil
}

// DecrementMessageCount atomically decrements the message_count on a room (floor at 0).
func (r *RoomRepository) DecrementMessageCount(ctx context.Context, roomID uuid.UUID) error {
	query := `UPDATE rooms SET message_count = message_count - 1 WHERE id = $1 AND message_count > 0`
	_, err := r.pool.Exec(ctx, query, roomID)
	if err != nil {
		LogQueryError(ctx, "DecrementMessageCount", "rooms", err)
		return err
	}
	return nil
}

// scanRoom scans a single room row from a query that returns all 15 columns.
func (r *RoomRepository) scanRoom(ctx context.Context, op, query string, args ...any) (*models.Room, error) {
	var room models.Room
	err := r.pool.QueryRow(ctx, query, args...).Scan(
		&room.ID,
		&room.Slug,
		&room.DisplayName,
		&room.Description,
		&room.Category,
		&room.Tags,
		&room.IsPrivate,
		&room.OwnerID,
		&room.TokenHash,
		&room.MessageCount,
		&room.CreatedAt,
		&room.UpdatedAt,
		&room.LastActiveAt,
		&room.ExpiresAt,
		&room.DeletedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrRoomNotFound
		}
		LogQueryError(ctx, op, "rooms", err)
		return nil, err
	}
	return &room, nil
}

// scanRoomFromRow scans a single room from a QueryRow with all 15 columns.
func (r *RoomRepository) scanRoomFromRow(ctx context.Context, op, query string, args ...any) (*models.Room, error) {
	return r.scanRoom(ctx, op, query, args...)
}

// scanRoomRow scans a room from pgx.Rows (for list queries with 15 columns).
func (r *RoomRepository) scanRoomRow(rows pgx.Rows, room *models.Room) error {
	return rows.Scan(
		&room.ID,
		&room.Slug,
		&room.DisplayName,
		&room.Description,
		&room.Category,
		&room.Tags,
		&room.IsPrivate,
		&room.OwnerID,
		&room.TokenHash,
		&room.MessageCount,
		&room.CreatedAt,
		&room.UpdatedAt,
		&room.LastActiveAt,
		&room.ExpiresAt,
		&room.DeletedAt,
	)
}
