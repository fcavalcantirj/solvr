package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// ErrRoomMemberNotFound is returned when a membership row does not exist.
var ErrRoomMemberNotFound = errors.New("room member not found")

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
		DO UPDATE SET role = EXCLUDED.role, added_by = EXCLUDED.added_by
		RETURNING room_id, agent_id, role, added_by, created_at
	`
	var m models.RoomMember
	err := r.pool.QueryRow(ctx, query, params.RoomID, params.AgentID, role, params.AddedBy).Scan(
		&m.RoomID, &m.AgentID, &m.Role, &m.AddedBy, &m.CreatedAt,
	)
	if err != nil {
		LogQueryError(ctx, "Add", "room_members", err)
		return nil, err
	}
	return &m, nil
}

// Remove deletes a membership row. Returns ErrRoomMemberNotFound if the agent was
// not a member.
func (r *RoomMemberRepository) Remove(ctx context.Context, roomID uuid.UUID, agentID string) error {
	result, err := r.pool.Exec(ctx, `DELETE FROM room_members WHERE room_id = $1 AND agent_id = $2`, roomID, agentID)
	if err != nil {
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
		WHERE room_id = $1 AND agent_id = $2
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
		`SELECT EXISTS(SELECT 1 FROM room_members WHERE room_id = $1 AND agent_id = $2)`,
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
		`SELECT EXISTS(SELECT 1 FROM room_members WHERE room_id = $1 AND agent_id = $2 AND role = 'owner')`,
		roomID, agentID,
	).Scan(&exists)
	if err != nil {
		LogQueryError(ctx, "IsOwner", "room_members", err)
		return false, err
	}
	return exists, nil
}

// ListByRoom returns all members of a room ordered by role (owners first) then join time.
func (r *RoomMemberRepository) ListByRoom(ctx context.Context, roomID uuid.UUID) ([]models.RoomMember, error) {
	query := `
		SELECT room_id, agent_id, role, added_by, created_at
		FROM room_members
		WHERE room_id = $1
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
