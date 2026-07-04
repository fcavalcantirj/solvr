package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// RoomEvent is a typed, queryable coordination signal in a room (mission #4).
// Fields match migration 000078_create_room_events.up.sql. Distinct from a chat
// Message and from a RoomClaim lock: an event is an append-only announcement such as
// CLAIM / BUILDING / PR / MERGED / RELEASE.
type RoomEvent struct {
	ID        int64           `json:"id"`
	RoomID    uuid.UUID       `json:"room_id"`
	EventType string          `json:"type"`
	Issue     string          `json:"issue"`
	Actor     string          `json:"actor"`
	Payload   json.RawMessage `json:"payload"`
	CreatedAt time.Time       `json:"created_at"`
}

// CreateRoomEventParams holds parameters for appending a room event.
type CreateRoomEventParams struct {
	RoomID    uuid.UUID
	EventType string
	Issue     string
	Actor     string
	Payload   json.RawMessage
}

// QueryRoomEventsParams filters a room event query. Empty EventType/Issue mean
// "no filter on that field".
type QueryRoomEventsParams struct {
	RoomID    uuid.UUID
	EventType string
	Issue     string
	Limit     int
}
