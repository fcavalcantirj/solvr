package db

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// RecentRoomsCaller names whose rooms ListRecentForCaller lists. A human sets UserID; an
// agent sets AgentID and, when it is claimed, FamilyOwnerID (its human).
type RecentRoomsCaller struct {
	UserID        *uuid.UUID
	AgentID       string
	FamilyOwnerID *uuid.UUID
}

// recentRoomsLimit bounds the list: it is a way back to recent work, not an archive.
const recentRoomsLimit = 100

// ListRecentForCaller returns the existing rooms the caller works in, most recently active
// first (idx 92 step 1): every room the human or the agent holds an active membership in
// (any role), plus — for an agent — the rooms its human OWNS (family access, Part 24.4).
// A room the agent's human merely joined is not listed: the agent could not read it.
func (r *RoomRepository) ListRecentForCaller(ctx context.Context, c RecentRoomsCaller) ([]models.Room, error) {
	if c.UserID == nil && c.AgentID == "" {
		return []models.Room{}, nil
	}
	var agentID any
	if c.AgentID != "" {
		agentID = c.AgentID
	}
	query := `
		SELECT ` + roomColumns + `
		FROM rooms
		WHERE ` + roomExistsPredicate + ` AND EXISTS (
			SELECT 1 FROM room_members rm
			 WHERE rm.room_id = rooms.id AND rm.revoked_at IS NULL
			   AND (   (rm.user_id = $1::uuid)
			        OR (rm.agent_id = $2::text)
			        OR (rm.user_id = $3::uuid AND rm.role = 'owner')))
		ORDER BY last_active_at DESC NULLS LAST, created_at DESC
		LIMIT ` + fmt.Sprint(recentRoomsLimit)

	rows, err := r.pool.Query(ctx, query, c.UserID, agentID, c.FamilyOwnerID)
	if err != nil {
		LogQueryError(ctx, "ListRecentForCaller", "rooms", err)
		return nil, err
	}
	defer rows.Close()

	rooms := []models.Room{}
	for rows.Next() {
		var room models.Room
		if err := r.scanRoomRow(rows, &room); err != nil {
			LogQueryError(ctx, "ListRecentForCaller.Scan", "rooms", err)
			return nil, fmt.Errorf("scan room: %w", err)
		}
		rooms = append(rooms, room)
	}
	return rooms, rows.Err()
}
