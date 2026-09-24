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
// added_by. Idempotent by (room_id, agent_id).
func (r *RoomMemberRepository) Add(ctx context.Context, params models.AddRoomMemberParams) (*models.RoomMember, error) {
	role := params.Role
	if role == "" {
		role = models.RoleMember
	}
	query := `
		INSERT INTO room_members (room_id, agent_id, role, added_by)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (room_id, agent_id)
		DO UPDATE SET role = EXCLUDED.role, added_by = EXCLUDED.added_by,
			created_at = CASE WHEN room_members.revoked_at IS NULL THEN room_members.created_at ELSE NOW() END,
			revoked_at = NULL
		RETURNING room_id, agent_id, role, added_by, created_at
	`
	var m models.RoomMember
	err := r.pool.QueryRow(ctx, query, params.RoomID, params.AgentID, role, params.AddedBy).Scan(
		&m.RoomID, &m.AgentID, &m.Role, &m.AddedBy, &m.CreatedAt,
	)
	if err != nil {
		if isFinalOwnerViolation(err) {
			return nil, ErrLastRoomOwner
		}
		LogQueryError(ctx, "Add", "room_members", err)
		return nil, err
	}
	return &m, nil
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
	query := `
		SELECT room_id, agent_id, role, added_by, created_at
		FROM room_members
		WHERE room_id = $1 AND agent_id = $2 AND revoked_at IS NULL
	`
	var m models.RoomMember
	err := r.pool.QueryRow(ctx, query, roomID, agentID).Scan(
		&m.RoomID, &m.AgentID, &m.Role, &m.AddedBy, &m.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrRoomMemberNotFound
		}
		LogQueryError(ctx, "Get", "room_members", err)
		return nil, err
	}
	return &m, nil
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
	query := `
		SELECT room_id, agent_id, role, added_by, created_at
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
		var m models.RoomMember
		if err := rows.Scan(&m.RoomID, &m.AgentID, &m.Role, &m.AddedBy, &m.CreatedAt); err != nil {
			LogQueryError(ctx, "ListByRoom.Scan", "room_members", err)
			return nil, fmt.Errorf("scan room member: %w", err)
		}
		members = append(members, m)
	}
	return members, rows.Err()
}

// isFinalOwnerViolation reports whether err is the room_members_final_owner guard of
// migration 000095 (a live room losing its last active owner).
func isFinalOwnerViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.ConstraintName == "room_members_final_owner"
}
