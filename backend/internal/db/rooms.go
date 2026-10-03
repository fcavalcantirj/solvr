package db

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// ErrRoomSlugExists is returned when a room with the same slug already exists.
var ErrRoomSlugExists = errors.New("room slug already exists")

// ErrRoomNotFound is returned when a room is not found.
var ErrRoomNotFound = errors.New("room not found")

// humanOwnerOf selects a room's earliest active human owner membership. rooms.owner_id
// is retired (000097); this derived value is what API responses expose as owner_id.
func humanOwnerOf(roomIDExpr string) string {
	return `SELECT rm.user_id FROM room_members rm
		WHERE rm.room_id = ` + roomIDExpr + ` AND rm.user_id IS NOT NULL
		  AND rm.role = 'owner' AND rm.revoked_at IS NULL
		ORDER BY rm.created_at, rm.id LIMIT 1`
}

// roomColumns is the column list read by scanRoom/scanRoomRow; owner_id is derived.
var roomColumns = `id, slug, display_name, description, category, tags, is_private,
	(` + humanOwnerOf("rooms.id") + `) AS owner_id,
	message_count, created_at, updated_at, last_active_at, expires_at, deleted_at,
	archived_at, result_message_id`

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
// Returns the created room. Agents obtain per-agent room tokens via the handshake.
func (r *RoomRepository) Create(ctx context.Context, params models.CreateRoomParams) (*models.Room, error) {
	// Generate slug if not provided
	slug := params.Slug
	if slug == "" {
		slug = slugify(params.DisplayName)
	}

	// Normalize tags
	tags := params.Tags
	if tags == nil {
		tags = []string{}
	}

	query := `
		INSERT INTO rooms (slug, display_name, description, category, tags, is_private, message_count, expires_at, source_post_id, source_room_id)
		VALUES ($1, $2, $3, $4, $5, $6, 0, $7, $8, $9)
		RETURNING id, slug, display_name, description, category, tags, is_private,
			message_count, created_at, updated_at, last_active_at, expires_at, deleted_at,
			archived_at, result_message_id, source_post_id, source_room_id
	`

	// A room and its owner-membership rows are created atomically: if a membership
	// insert fails, the room is rolled back so we never leave a room without a
	// manageable owner. room_members is the only ownership store (000097).
	var room models.Room
	txErr := r.pool.WithTx(ctx, func(tx Tx) error {
		scanErr := tx.QueryRow(ctx, query,
			slug,
			params.DisplayName,
			params.Description,
			params.Category,
			tags,
			params.IsPrivate,
			params.ExpiresAt,
			params.SourcePostID,
			params.SourceRoomID,
		).Scan(
			&room.ID,
			&room.Slug,
			&room.DisplayName,
			&room.Description,
			&room.Category,
			&room.Tags,
			&room.IsPrivate,
			&room.MessageCount,
			&room.CreatedAt,
			&room.UpdatedAt,
			&room.LastActiveAt,
			&room.ExpiresAt,
			&room.DeletedAt,
			&room.ArchivedAt,
			&room.ResultMessageID,
			&room.SourcePostID,
			&room.SourceRoomID,
		)
		if scanErr != nil {
			return scanErr
		}

		// Register the human owner (uuid.Nil means no human owner).
		if params.OwnerID != uuid.Nil {
			if _, mErr := tx.Exec(ctx,
				`INSERT INTO room_members (room_id, user_id, role, added_by)
				 VALUES ($1, $2, 'owner', 'system')`,
				room.ID, params.OwnerID,
			); mErr != nil {
				return mErr
			}
			ownerID := params.OwnerID
			room.OwnerID = &ownerID
		}

		// Register the creating agent as room owner (mission #1/#3 owner fix).
		if params.CreatorAgentID != "" {
			_, mErr := tx.Exec(ctx,
				`INSERT INTO room_members (room_id, agent_id, role, added_by)
				 VALUES ($1, $2, 'owner', 'system')
				 ON CONFLICT (room_id, agent_id) DO UPDATE SET role = 'owner'`,
				room.ID, params.CreatorAgentID,
			)
			if mErr != nil {
				return mErr
			}
		}
		return nil
	})
	if txErr != nil {
		var pgErr *pgconn.PgError
		if errors.As(txErr, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName != "" && pgErr.TableName == "rooms" {
			return nil, ErrRoomSlugExists
		}
		// Slug uniqueness violations may surface without table metadata depending on driver path.
		if errors.As(txErr, &pgErr) && pgErr.Code == "23505" {
			return nil, ErrRoomSlugExists
		}
		LogQueryError(ctx, "Create", "rooms", txErr)
		return nil, txErr
	}

	return &room, nil
}

// roomExistsPredicate is what makes a room exist for every room surface: not deleted and
// not past its expires_at. An expired room is gone at once, not only after the presence
// reaper deletes it.
const roomExistsPredicate = `deleted_at IS NULL AND (expires_at IS NULL OR expires_at > NOW())`

// GetBySlug returns a room by its slug.
// Returns ErrRoomNotFound if the room doesn't exist, is soft-deleted or has expired.
func (r *RoomRepository) GetBySlug(ctx context.Context, slug string) (*models.Room, error) {
	query := `
		SELECT ` + roomColumns + `
		FROM rooms
		WHERE slug = $1 AND ` + roomExistsPredicate + `
	`
	return r.scanRoom(ctx, "GetBySlug", query, slug)
}

// GetByID returns a room by its ID.
// Returns ErrRoomNotFound if the room doesn't exist, is soft-deleted or has expired.
func (r *RoomRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Room, error) {
	query := `
		SELECT ` + roomColumns + `
		FROM rooms
		WHERE id = $1 AND ` + roomExistsPredicate + `
	`
	return r.scanRoom(ctx, "GetByID", query, id)
}

// RoomListParams controls public room discovery listing. The zero value lists
// recent, non-archived public rooms — the same behaviour the old List(limit,
// offset) exposed.
type RoomListParams struct {
	Limit  int
	Offset int
	// Sort is "recent" (default, most recent activity first) or "active"
	// (rooms with the most live agents first, recent activity as tie-breaker).
	Sort string
	// Query, when non-empty, matches display_name or description (case-insensitive)
	// and also surfaces archived rooms so a searcher can still find a finished room.
	Query string
	// IncludeArchived surfaces archived rooms in the default browse view.
	IncludeArchived bool
}

// emptyAbandonedInterval defines how long an empty room (no messages, no live
// agents) must sit inactive before it is treated as abandoned and hidden from
// public discovery. Freshly created empty rooms (an agent is still expected to
// join) stay listed.
const emptyAbandonedInterval = "1 hour"

// List returns recent public rooms with default discovery filters. It delegates
// to ListFiltered so there is a single query path; callers wanting sort/search
// use ListFiltered directly.
func (r *RoomRepository) List(ctx context.Context, limit, offset int) ([]models.RoomWithStats, error) {
	return r.ListFiltered(ctx, RoomListParams{Limit: limit, Offset: offset})
}

// roomParticipantCount is a room's unique participants: the distinct authenticated authors of its
// live messages, the same figure as COUNT(DISTINCT author_id) over them. It walks
// idx_room_entries_room_author one author at a time (migration 000126), so it costs one index
// probe per participant rather than a read of every message the room holds (idx 77 slice 14:
// 75 ms -> 0.3 ms for a 23k-message room).
func roomParticipantCount(roomIDExpr string) string {
	return `(WITH RECURSIVE pa(author_id) AS (
			(SELECT e.author_id FROM room_entries e
			  WHERE e.room_id = ` + roomIDExpr + ` AND e.kind = 'message' AND e.deleted_at IS NULL
			    AND e.author_id IS NOT NULL
			  ORDER BY e.author_id LIMIT 1)
			UNION ALL
			SELECT (SELECT e.author_id FROM room_entries e
			         WHERE e.room_id = ` + roomIDExpr + ` AND e.kind = 'message' AND e.deleted_at IS NULL
			           AND e.author_id > pa.author_id
			         ORDER BY e.author_id LIMIT 1)
			  FROM pa WHERE pa.author_id IS NOT NULL)
		SELECT COUNT(author_id) FROM pa)`
}

// ListFiltered returns public rooms for discovery, honouring sort, search and
// archived visibility. It always excludes private, soft-deleted, expired and
// empty-abandoned rooms. Uses correlated subqueries (no N+1 per D-34) and
// includes live_agent_count (D-05 presence), unique_participant_count (D-05),
// owner_display_name (D-10), and a last-message preview for the room card.
func (r *RoomRepository) ListFiltered(ctx context.Context, params RoomListParams) ([]models.RoomWithStats, error) {
	limit := params.Limit
	if limit <= 0 {
		limit = 20
	}
	offset := params.Offset
	if offset < 0 {
		offset = 0
	}

	// WHERE clauses common to every discovery query: never leak private,
	// deleted or expired rooms, and drop empty rooms nobody came back to.
	where := []string{
		"r.deleted_at IS NULL",
		"r.is_private = FALSE",
		"(r.expires_at IS NULL OR r.expires_at > NOW())",
		`NOT (
			r.message_count = 0
			AND r.last_active_at < NOW() - INTERVAL '` + emptyAbandonedInterval + `'
			AND NOT EXISTS (
				SELECT 1 FROM agent_presence ap
				WHERE ap.room_id = r.id
				  AND ap.last_seen > NOW() - (ap.ttl_seconds || ' seconds')::interval
			)
		)`,
	}

	args := []any{}
	argN := 1

	// Archived rooms are hidden from the default browse but discoverable via an
	// explicit search or the include-archived option.
	searching := params.Query != ""
	if !params.IncludeArchived && !searching {
		where = append(where, "r.archived_at IS NULL")
	}

	if searching {
		where = append(where, fmt.Sprintf("(r.display_name ILIKE $%d OR r.description ILIKE $%d)", argN, argN))
		args = append(args, "%"+params.Query+"%")
		argN++
	}

	orderBy := "ORDER BY %[1]slast_active_at DESC, %[1]sid DESC"
	if params.Sort == "active" {
		orderBy = "ORDER BY %[1]slive_agent_count DESC, %[1]slast_active_at DESC, %[1]sid DESC"
	}

	limitArg := argN
	offsetArg := argN + 1
	args = append(args, limit, offset)

	// The page is chosen from rooms alone; only its rows then read their participants, preview
	// and owner, so an offset page does not pay for the rooms it skips.
	query := `
		SELECT p.id, p.slug, p.display_name, p.description, p.category, p.tags,
			p.is_private, ho.user_id, p.message_count, p.created_at, p.updated_at,
			p.last_active_at, p.expires_at, p.live_agent_count,
			` + roomParticipantCount("p.id") + ` AS unique_participant_count,
			u.display_name AS owner_display_name,
			(SELECT LEFT(m.content, 200) FROM messages m
			 WHERE m.room_id = p.id AND m.deleted_at IS NULL
			 ORDER BY m.created_at DESC, m.id DESC LIMIT 1
			) AS last_message_preview
		FROM (
			SELECT r.*,
				(SELECT COUNT(DISTINCT agent_name) FROM agent_presence ap
				 WHERE ap.room_id = r.id
				   AND ap.last_seen > NOW() - (ap.ttl_seconds || ' seconds')::interval
				) AS live_agent_count
			FROM rooms r
			WHERE ` + strings.Join(where, " AND ") + `
			` + fmt.Sprintf(orderBy, "") + `
			LIMIT $` + strconv.Itoa(limitArg) + ` OFFSET $` + strconv.Itoa(offsetArg) + `
		) p
		LEFT JOIN LATERAL (` + humanOwnerOf("p.id") + `) ho ON TRUE
		LEFT JOIN users u ON u.id = ho.user_id
		` + fmt.Sprintf(orderBy, "p.")

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		LogQueryError(ctx, "ListFiltered", "rooms", err)
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
			&rws.LastMessagePreview,
		)
		if err != nil {
			LogQueryError(ctx, "ListFiltered.Scan", "rooms", err)
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

// ListByOwner returns existing (not deleted, not expired) rooms where the specified user
// holds an active owner membership, ordered by created_at DESC.
func (r *RoomRepository) ListByOwner(ctx context.Context, ownerID uuid.UUID) ([]models.Room, error) {
	query := `
		SELECT ` + roomColumns + `
		FROM rooms
		WHERE ` + roomExistsPredicate + ` AND EXISTS (
			SELECT 1 FROM room_members rm
			WHERE rm.room_id = rooms.id AND rm.user_id = $1
			  AND rm.role = 'owner' AND rm.revoked_at IS NULL)
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
// Always sets updated_at = NOW(). expected is the version (updated_at) the
// caller read: the write lands only while the room is still at it, in the same
// statement, so two edits that read one version cannot both land (spec.json
// idx 74 step 5); it returns a *models.VersionConflictError with the current
// version otherwise. A nil expected writes unconditionally.
func (r *RoomRepository) Update(ctx context.Context, roomID uuid.UUID, params models.UpdateRoomParams, expected *time.Time) (*models.Room, error) {
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
		  AND ($%d::timestamptz IS NULL OR updated_at = $%d::timestamptz)
		RETURNING `+roomColumns+`
	`, strings.Join(setClauses, ", "), argIdx, argIdx+1, argIdx+1)
	args = append(args, roomID, expected)

	room, err := r.scanRoomFromRow(ctx, "Update", query, args...)
	if errors.Is(err, ErrRoomNotFound) && expected != nil {
		var current time.Time
		verr := r.pool.QueryRow(ctx,
			"SELECT updated_at FROM rooms WHERE id = $1 AND deleted_at IS NULL", roomID).Scan(&current)
		if verr == nil {
			return nil, &models.VersionConflictError{Current: current}
		}
		if !errors.Is(verr, pgx.ErrNoRows) {
			return nil, fmt.Errorf("read room version: %w", verr)
		}
	}
	return room, err
}

// BackfillOwnerFromMembership makes the given human an active owner of every live room
// where the agent holds an active 'owner' membership and no human owns the room yet
// (rooms the agent created while unclaimed). Used when a human claims the agent so those
// pre-claim rooms join the human's family scope. humanID is cast to uuid in SQL.
// Returns the number of rooms that gained a human owner. Idempotent.
func (r *RoomRepository) BackfillOwnerFromMembership(ctx context.Context, agentID, humanID string) (int64, error) {
	query := `
		INSERT INTO room_members (room_id, user_id, role, added_by)
		SELECT r.id, $1::uuid, 'owner', 'system'
		FROM rooms r
		JOIN room_members am ON am.room_id = r.id AND am.agent_id = $2
		  AND am.role = 'owner' AND am.revoked_at IS NULL
		WHERE r.deleted_at IS NULL AND NOT EXISTS (
			SELECT 1 FROM room_members hm
			WHERE hm.room_id = r.id AND hm.user_id IS NOT NULL
			  AND hm.role = 'owner' AND hm.revoked_at IS NULL)
		ON CONFLICT (room_id, user_id) DO UPDATE SET role = 'owner', revoked_at = NULL`
	result, err := r.pool.Exec(ctx, query, humanID, agentID)
	if err != nil {
		LogQueryError(ctx, "BackfillOwnerFromMembership", "rooms", err)
		return 0, err
	}
	return result.RowsAffected(), nil
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

// Archive marks a room Finished: it records archived_at (preserving the original
// timestamp on a re-archive via COALESCE) and, optionally, the message that
// captured the result. The transcript stays readable; the message/join gates
// enforced in the handlers refuse new activity until Reopen clears the state.
// Archiving never touches deleted_at or expires_at — it is distinct from both.
func (r *RoomRepository) Archive(ctx context.Context, roomID uuid.UUID, resultMessageID *int64) (*models.Room, error) {
	query := `
		UPDATE rooms
		SET archived_at = COALESCE(archived_at, NOW()),
			result_message_id = $2,
			updated_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL
		RETURNING ` + roomColumns + `
	`
	return r.scanRoomFromRow(ctx, "Archive", query, roomID, resultMessageID)
}

// Reopen clears the archived state so the room becomes Live again. The recorded
// result_message_id is retained as history.
func (r *RoomRepository) Reopen(ctx context.Context, roomID uuid.UUID) (*models.Room, error) {
	query := `
		UPDATE rooms
		SET archived_at = NULL,
			updated_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL
		RETURNING ` + roomColumns + `
	`
	return r.scanRoomFromRow(ctx, "Reopen", query, roomID)
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
		&room.MessageCount,
		&room.CreatedAt,
		&room.UpdatedAt,
		&room.LastActiveAt,
		&room.ExpiresAt,
		&room.DeletedAt,
		&room.ArchivedAt,
		&room.ResultMessageID,
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
		&room.MessageCount,
		&room.CreatedAt,
		&room.UpdatedAt,
		&room.LastActiveAt,
		&room.ExpiresAt,
		&room.DeletedAt,
		&room.ArchivedAt,
		&room.ResultMessageID,
	)
}
