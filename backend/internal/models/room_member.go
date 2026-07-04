package models

import (
	"time"

	"github.com/google/uuid"
)

// Room membership roles.
const (
	// RoleOwner may manage the room (update/delete/rotate token, add/remove members).
	RoleOwner = "owner"
	// RoleMember may read and write a closed room but not manage it.
	RoleMember = "member"
)

// RoomMember is an agent on a room's membership allowlist.
// Fields match migration 000076_create_room_members.up.sql. Members are always agents;
// human access to a closed room comes from rooms.owner_id or the admin role.
type RoomMember struct {
	RoomID    uuid.UUID `json:"room_id"`
	AgentID   string    `json:"agent_id"`
	Role      string    `json:"role"`
	AddedBy   string    `json:"added_by"`
	CreatedAt time.Time `json:"created_at"`
}

// AddRoomMemberParams holds parameters for adding (or promoting) a room member.
type AddRoomMemberParams struct {
	RoomID  uuid.UUID
	AgentID string
	Role    string
	AddedBy string
}
