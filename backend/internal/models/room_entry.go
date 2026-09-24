package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Room entry kinds. A room_entries row is either a freeform message or a typed,
// machine-readable coordination event; both share one ordered per-room timeline.
const (
	RoomEntryKindMessage = "message"
	RoomEntryKindEvent   = "event"
)

// RoomEntry is one row of the unified room timeline (task: "Store room messages and
// coordination events in one ordered room timeline"). It replaces the parallel
// messages and room_events tables: a message carries Body/ContentType and optional
// reply/addressing; an event carries EventType/Issue and its Extension payload.
type RoomEntry struct {
	ID       int64     `json:"id"`
	RoomID   uuid.UUID `json:"room_id"`
	Sequence int       `json:"sequence"`
	Kind     string    `json:"kind"`

	// Authenticated actor reference (nil for shared-token or system writes) and the
	// historical display label shown in the transcript.
	AuthorType *string `json:"author_type,omitempty"`
	AuthorID   *string `json:"author_id,omitempty"`
	ActorLabel string  `json:"actor_label"`

	// Message fields.
	Body               *string         `json:"body,omitempty"`
	ContentType        string          `json:"content_type"`
	ReplyToEntryID     *int64          `json:"reply_to_entry_id,omitempty"`
	AddressedMemberIDs json.RawMessage `json:"addressed_member_ids,omitempty"`
	SupersedesEntryID  *int64          `json:"supersedes_entry_id,omitempty"`
	PinnedAt           *time.Time      `json:"pinned_at,omitempty"`

	// Event fields.
	EventType *string `json:"event_type,omitempty"`
	Issue     string  `json:"issue,omitempty"`

	// Bounded extension data: message metadata or event payload.
	Extension json.RawMessage `json:"extension,omitempty"`

	CreatedAt time.Time  `json:"created_at"`
	DeletedAt *time.Time `json:"deleted_at,omitempty"`
}

// CreateRoomEntryParams is the input to RoomEntryRepository.Create.
type CreateRoomEntryParams struct {
	RoomID uuid.UUID
	Kind   string

	AuthorType *string
	AuthorID   *string
	ActorLabel string

	Body               *string
	ContentType        string
	ReplyToEntryID     *int64
	AddressedMemberIDs json.RawMessage
	SupersedesEntryID  *int64

	EventType *string
	Issue     string

	Extension json.RawMessage

	// ClientEntryID makes the write idempotent for an authenticated author: a retry
	// with the same (room_id, author_id, client_entry_id) returns the existing entry.
	ClientEntryID *string
}

// QueryRoomEntryEventsParams filters the event view of the timeline. Empty EventType
// or Issue means "any"; both retain the original room_events issue/type filters.
type QueryRoomEntryEventsParams struct {
	RoomID    uuid.UUID
	EventType string
	Issue     string
	Limit     int
}
