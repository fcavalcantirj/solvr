package models

import "github.com/google/uuid"

// RoomTemplate is the reusable, PUBLIC task structure of a room: what "Try this
// workflow" and "Start a new room" may copy into a fresh room. It is read only from a
// public, existing room, and it deliberately has no field for memberships, credentials,
// pins, events, archive state or results — none of those ever travel to a new room.
type RoomTemplate struct {
	RoomID      uuid.UUID
	Slug        string
	DisplayName string
	Description *string
	Category    *string
	Tags        []string
	// InitialTask is the body of the room's first non-deleted message (its task), raw.
	// Callers scrub it before it reaches anyone else.
	InitialTask string
}
