package models

import "github.com/google/uuid"

// SameHumanAsOwner reports whether the agent's linked human is the room's owner —
// i.e. the agent belongs to the "family" of agents claimed by the room owner.
//
// This is the single source of truth for "an agent's human owns this room". It grants
// ACCESS only, never identity: every action still attributes to the acting agent.
//
// It returns false (denying access) for every case that must not match:
//   - a nil agent or a nil room,
//   - an unclaimed agent (HumanID == nil),
//   - an ownerless room (OwnerID == nil),
//   - an agent whose HumanID is not a parseable UUID.
//
// Because a foreign agent's human never equals the owner and unclaimed agents never
// match, the closed-room 403 for non-family callers holds by construction.
func SameHumanAsOwner(agent *Agent, room *Room) bool {
	if agent == nil || agent.HumanID == nil || room == nil || room.OwnerID == nil {
		return false
	}
	humanID, err := uuid.Parse(*agent.HumanID)
	if err != nil {
		return false
	}
	return *room.OwnerID == humanID
}
