package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// ErrRoomMemberNotFound is returned when a membership row does not exist.
var ErrRoomMemberNotFound = errors.New("room member not found")

// ErrLastRoomOwner is returned when a change would leave a live room without an active
// owner. Hand ownership to another member first, or delete the room.
var ErrLastRoomOwner = errors.New("room must keep at least one owner")

// RoomMemberRepository handles the room membership allowlist (mission #1 ACL, #3 identity).
type RoomMemberRepository struct {
	pool *Pool
}

// NewRoomMemberRepository creates a new RoomMemberRepository.
func NewRoomMemberRepository(pool *Pool) *RoomMemberRepository {
	return &RoomMemberRepository{pool: pool}
}

// Add inserts a member or, if the agent is already a member, updates its role and
// added_by. Idempotent by (room_id, agent_id). An empty Role adds a new or readmitted
// agent as a member and keeps the role of an active one, so a plain re-add (or its
// retry) never demotes an owner; only an explicit Role changes it.
func (r *RoomMemberRepository) Add(ctx context.Context, params models.AddRoomMemberParams) (*models.RoomMember, error) {
	role := params.Role
	keepRole := role == ""
	if keepRole {
		role = models.RoleMember
	}
	query := `
		INSERT INTO room_members (room_id, agent_id, role, added_by)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (room_id, agent_id)
		DO UPDATE SET role = CASE WHEN $5 AND room_members.revoked_at IS NULL THEN room_members.role ELSE EXCLUDED.role END,
			added_by = EXCLUDED.added_by, access_source = 'direct',
			created_at = CASE WHEN room_members.revoked_at IS NULL THEN room_members.created_at ELSE NOW() END,
			revoked_at = NULL
		RETURNING ` + memberColumns + `
	`
	m, err := scanMember(r.pool.QueryRow(ctx, query, params.RoomID, params.AgentID, role, params.AddedBy, keepRole))
	if err != nil {
		if isFinalOwnerViolation(err) {
			return nil, ErrLastRoomOwner
		}
		LogQueryError(ctx, "Add", "room_members", err)
		return nil, err
	}
	return m, nil
}

// AddFamily materializes a membership for an agent admitted through family access (its
// linked human owns the room). The row is tagged 'family' so it ends with that
// justification (migration 000096). An existing active membership is returned
// unchanged: a direct grant or owner role is never downgraded.
func (r *RoomMemberRepository) AddFamily(ctx context.Context, roomID uuid.UUID, agentID string) (*models.RoomMember, error) {
	query := `
		INSERT INTO room_members (room_id, agent_id, role, added_by, access_source)
		VALUES ($1, $2, 'member', $2, 'family')
		ON CONFLICT (room_id, agent_id)
		DO UPDATE SET role = CASE WHEN room_members.revoked_at IS NULL THEN room_members.role ELSE 'member' END,
			added_by = CASE WHEN room_members.revoked_at IS NULL THEN room_members.added_by ELSE EXCLUDED.added_by END,
			access_source = CASE WHEN room_members.revoked_at IS NULL THEN room_members.access_source ELSE 'family' END,
			created_at = CASE WHEN room_members.revoked_at IS NULL THEN room_members.created_at ELSE NOW() END,
			revoked_at = NULL
		RETURNING ` + memberColumns + `
	`
	m, err := scanMember(r.pool.QueryRow(ctx, query, roomID, agentID))
	if err != nil {
		LogQueryError(ctx, "AddFamily", "room_members", err)
		return nil, err
	}
	return m, nil
}

// Remove revokes an active membership: the row stays with revoked_at set (and the
// agent's per-agent room token is deleted by trigger) until an explicit readmission via
// Add. Returns ErrRoomMemberNotFound if the agent was not an active member, and
// ErrLastRoomOwner if it is the live room's final owner.
func (r *RoomMemberRepository) Remove(ctx context.Context, roomID uuid.UUID, agentID string) error {
	result, err := r.pool.Exec(ctx,
		`UPDATE room_members SET revoked_at = NOW()
		 WHERE room_id = $1 AND agent_id = $2 AND revoked_at IS NULL`, roomID, agentID)
	if err != nil {
		if isFinalOwnerViolation(err) {
			return ErrLastRoomOwner
		}
		LogQueryError(ctx, "Remove", "room_members", err)
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrRoomMemberNotFound
	}
	return nil
}

// Get returns a single membership row, or ErrRoomMemberNotFound.
func (r *RoomMemberRepository) Get(ctx context.Context, roomID uuid.UUID, agentID string) (*models.RoomMember, error) {
	query := `SELECT ` + memberColumns + `
		FROM room_members
		WHERE room_id = $1 AND agent_id = $2 AND revoked_at IS NULL
	`
	m, err := scanMember(r.pool.QueryRow(ctx, query, roomID, agentID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrRoomMemberNotFound
		}
		LogQueryError(ctx, "Get", "room_members", err)
		return nil, err
	}
	return m, nil
}

// IsMember reports whether the agent is on the room's allowlist (any role).
func (r *RoomMemberRepository) IsMember(ctx context.Context, roomID uuid.UUID, agentID string) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM room_members WHERE room_id = $1 AND agent_id = $2 AND revoked_at IS NULL)`,
		roomID, agentID,
	).Scan(&exists)
	if err != nil {
		LogQueryError(ctx, "IsMember", "room_members", err)
		return false, err
	}
	return exists, nil
}

// IsOwner reports whether the agent is a member with the owner role.
func (r *RoomMemberRepository) IsOwner(ctx context.Context, roomID uuid.UUID, agentID string) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM room_members WHERE room_id = $1 AND agent_id = $2 AND role = 'owner' AND revoked_at IS NULL)`,
		roomID, agentID,
	).Scan(&exists)
	if err != nil {
		LogQueryError(ctx, "IsOwner", "room_members", err)
		return false, err
	}
	return exists, nil
}

// ListByRoom returns the active agent members of a room ordered by role (owners first) then join time.
func (r *RoomMemberRepository) ListByRoom(ctx context.Context, roomID uuid.UUID) ([]models.RoomMember, error) {
	query := `SELECT ` + memberColumns + `
		FROM room_members
		WHERE room_id = $1 AND agent_id IS NOT NULL AND revoked_at IS NULL
		ORDER BY (role = 'owner') DESC, created_at ASC
	`
	rows, err := r.pool.Query(ctx, query, roomID)
	if err != nil {
		LogQueryError(ctx, "ListByRoom", "room_members", err)
		return nil, err
	}
	defer rows.Close()

	members := []models.RoomMember{}
	for rows.Next() {
		m, err := scanMember(rows)
		if err != nil {
			LogQueryError(ctx, "ListByRoom.Scan", "room_members", err)
			return nil, fmt.Errorf("scan room member: %w", err)
		}
		members = append(members, *m)
	}
	return members, rows.Err()
}

// IsFamilyOwner reports whether the agent has family access to the room: its CURRENT
// linked human is a live account holding an active owner membership there. Derived per
// request, so unclaimed agents, foreign agents, and links to a human who no longer owns
// the room (or whose account was deleted) never match.
func (r *RoomMemberRepository) IsFamilyOwner(ctx context.Context, roomID uuid.UUID, agentID string) (bool, error) {
	return r.exists(ctx, "IsFamilyOwner", `
		SELECT EXISTS(
			SELECT 1 FROM agents a
			JOIN users u ON u.id = a.human_id AND u.deleted_at IS NULL
			JOIN room_members m ON m.user_id = a.human_id
			WHERE a.id = $2 AND m.room_id = $1 AND m.role = 'owner' AND m.revoked_at IS NULL)`,
		roomID, agentID)
}

// IsUserOwner reports whether the human holds an active owner membership in the room.
// A malformed user id is simply not an owner.
func (r *RoomMemberRepository) IsUserOwner(ctx context.Context, roomID uuid.UUID, userID string) (bool, error) {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return false, nil
	}
	return r.exists(ctx, "IsUserOwner",
		`SELECT EXISTS(SELECT 1 FROM room_members WHERE room_id = $1 AND user_id = $2 AND role = 'owner' AND revoked_at IS NULL)`,
		roomID, uid)
}

// IsUserMember reports whether the human holds any active membership in the room.
func (r *RoomMemberRepository) IsUserMember(ctx context.Context, roomID uuid.UUID, userID string) (bool, error) {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return false, nil
	}
	return r.exists(ctx, "IsUserMember",
		`SELECT EXISTS(SELECT 1 FROM room_members WHERE room_id = $1 AND user_id = $2 AND revoked_at IS NULL)`,
		roomID, uid)
}

func (r *RoomMemberRepository) exists(ctx context.Context, op, query string, args ...any) (bool, error) {
	var ok bool
	if err := r.pool.QueryRow(ctx, query, args...).Scan(&ok); err != nil {
		LogQueryError(ctx, op, "room_members", err)
		return false, err
	}
	return ok, nil
}

const memberColumns = `room_id, agent_id, role, added_by, access_source, created_at`

func scanMember(row pgx.Row) (*models.RoomMember, error) {
	var m models.RoomMember
	if err := row.Scan(&m.RoomID, &m.AgentID, &m.Role, &m.AddedBy, &m.AccessSource, &m.CreatedAt); err != nil {
		return nil, err
	}
	return &m, nil
}

// isFinalOwnerViolation reports whether err is the room_members_final_owner guard of
// migration 000095 (a live room losing its last active owner).
func isFinalOwnerViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.ConstraintName == "room_members_final_owner"
}
