package hub

import "time"

// EventType identifies the kind of event emitted by a RoomHub.
type EventType string

const (
	// EventPresenceJoin is emitted when an agent subscribes to a room (D-06).
	EventPresenceJoin EventType = "presence_join"

	// EventPresenceLeave is emitted when an agent unsubscribes from a room (D-06).
	EventPresenceLeave EventType = "presence_leave"

	// EventMessage is emitted when an agent broadcasts a message to a room (D-06).
	EventMessage EventType = "message"

	// EventRoomUpdate is emitted when room metadata changes (D-06).
	EventRoomUpdate EventType = "room_update"

	// Legacy aliases kept for backward compatibility with Quorum hub internals.
	EventAgentJoined EventType = "presence_join"
	EventAgentLeft   EventType = "presence_leave"
)

// RoomEvent is emitted by the hub on agent join, leave, or message.
//
// The Payload field carries type-specific data: for presence_join it is an
// *a2a.AgentCard; for message it is the raw A2A message; for presence_leave
// it is nil.
type RoomEvent struct {
	// ID is the BIGSERIAL message ID for Last-Event-ID SSE reconnection support (D-07).
	ID        int64     `json:"id,omitempty"`
	Type      EventType `json:"type"`
	RoomID    RoomID    `json:"room_id"`
	AgentName string    `json:"agent_name,omitempty"`
	Payload   any       `json:"payload,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}
