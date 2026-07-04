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

	// EventTyped is emitted when a typed coordination event (CLAIM/BUILDING/PR/
	// MERGED/RELEASE, etc.) is appended to a room (mission #4). The specific typed
	// name is carried in RoomEvent.EventName and the issue in RoomEvent.Issue.
	EventTyped EventType = "event"

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
	// ID is the BIGSERIAL id for Last-Event-ID SSE reconnection support (D-07): the
	// message id for message events, or the room_events id for typed events.
	ID        int64     `json:"id,omitempty"`
	Type      EventType `json:"type"`
	RoomID    RoomID    `json:"room_id"`
	AgentName string    `json:"agent_name,omitempty"`
	// EventName is the typed coordination-event name (e.g. "CLAIM") when Type is
	// EventTyped; empty otherwise. Used by the SSE ?type= filter (mission #5).
	EventName string `json:"event,omitempty"`
	// Issue is the optional issue reference a typed event targets; used by the SSE
	// ?issue= filter (mission #5).
	Issue     string    `json:"issue,omitempty"`
	Payload   any       `json:"payload,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// Matches reports whether the event passes the given SSE stream filters. An empty
// filter matches everything on that dimension. A type filter matches either the hub
// Type ("message", "event", "presence_join", …) or the typed EventName ("CLAIM", …),
// so ?type=CLAIM and ?type=message both work.
func (e RoomEvent) Matches(typeFilter, issueFilter string) bool {
	if typeFilter != "" && string(e.Type) != typeFilter && e.EventName != typeFilter {
		return false
	}
	if issueFilter != "" && e.Issue != issueFilter {
		return false
	}
	return true
}
