package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Message represents a message in a room.
// Fields match migrations 000075_create_messages.up.sql and 000084_add_group_reply_fields_to_messages.up.sql.
type Message struct {
	ID                 int64           `json:"id"`
	RoomID             uuid.UUID       `json:"room_id"`
	AuthorType         string          `json:"author_type"`
	AuthorID           *string         `json:"author_id,omitempty"`
	AgentName          string          `json:"agent_name"`
	Content            string          `json:"content"`
	ContentType        string          `json:"content_type"`
	Metadata           json.RawMessage `json:"metadata"`
	ReplyToEntryID     *int64          `json:"reply_to_entry_id,omitempty"`
	AddressedMemberIDs json.RawMessage `json:"addressed_member_ids,omitempty"`
	SequenceNum        *int            `json:"sequence_num,omitempty"`
	CreatedAt          time.Time       `json:"created_at"`
	DeletedAt          *time.Time      `json:"-"`
}

// CreateMessageParams holds parameters for inserting a message.
type CreateMessageParams struct {
	RoomID             uuid.UUID       `json:"room_id"`
	AuthorType         string          `json:"author_type"`
	AuthorID           *string         `json:"author_id,omitempty"`
	AgentName          string          `json:"agent_name"`
	Content            string          `json:"content"`
	ContentType        string          `json:"content_type"`
	Metadata           json.RawMessage `json:"metadata,omitempty"`
	ReplyToEntryID     *int64          `json:"reply_to_entry_id,omitempty"`
	AddressedMemberIDs json.RawMessage `json:"addressed_member_ids,omitempty"`
}
