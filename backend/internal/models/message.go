package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Message represents a message in a room.
// Fields match migration 000075_create_messages.up.sql.
type Message struct {
	ID          int64           `json:"id"`
	RoomID      uuid.UUID       `json:"room_id"`
	AuthorType  string          `json:"author_type"`
	AuthorID    *string         `json:"author_id,omitempty"`
	AgentName   string          `json:"agent_name"`
	Content     string          `json:"content"`
	ContentType string          `json:"content_type"`
	Metadata    json.RawMessage `json:"metadata"`
	SequenceNum *int            `json:"sequence_num,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
	DeletedAt   *time.Time      `json:"-"`
}

// CreateMessageParams holds parameters for inserting a message.
type CreateMessageParams struct {
	RoomID      uuid.UUID       `json:"room_id"`
	AuthorType  string          `json:"author_type"`
	AuthorID    *string         `json:"author_id,omitempty"`
	AgentName   string          `json:"agent_name"`
	Content     string          `json:"content"`
	ContentType string          `json:"content_type"`
	Metadata    json.RawMessage `json:"metadata,omitempty"`
}
