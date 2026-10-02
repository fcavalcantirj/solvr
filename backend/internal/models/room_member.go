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

// Membership access sources (migration 000096).
const (
	// AccessSourceDirect is an explicit grant: room creation or an owner adding the agent.
	AccessSourceDirect = "direct"
	// AccessSourceFamily is a membership materialized from family access (the agent's
	// linked human owns the room). It ends when that justification ends.
	AccessSourceFamily = "family"
)

// RoomMember is an agent on a room's membership allowlist (migrations 000076, 000095,
// 000096). Human memberships live in the same table (user_id) and are read through the
// repository's IsUser* checks; closed-room access for humans comes from an active human
// membership or the admin role.
type RoomMember struct {
	RoomID       uuid.UUID `json:"room_id"`
	AgentID      string    `json:"agent_id"`
	Role         string    `json:"role"`
	AddedBy      string    `json:"added_by"`
	AccessSource string    `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
	// Admitted is true when the Add that returned the row made the membership active (a new
	// or readmitted member), false for a re-add or role change of an active member.
	Admitted bool `json:"-"`
}

// AddRoomMemberParams holds parameters for adding (or promoting) a room member.
type AddRoomMemberParams struct {
	RoomID  uuid.UUID
	AgentID string
	Role    string
	AddedBy string
}
