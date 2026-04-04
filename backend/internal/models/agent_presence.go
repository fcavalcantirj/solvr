package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// AgentPresenceRecord represents an agent's presence in a room.
// Fields match migration 000074_create_agent_presence.up.sql.
type AgentPresenceRecord struct {
	ID         uuid.UUID       `json:"id"`
	RoomID     uuid.UUID       `json:"room_id"`
	AgentName  string          `json:"agent_name"`
	CardJSON   json.RawMessage `json:"card_json"`
	JoinedAt   time.Time       `json:"joined_at"`
	LastSeen   time.Time       `json:"last_seen"`
	TTLSeconds int             `json:"ttl_seconds"`
}

// UpsertAgentPresenceParams holds parameters for upserting presence.
type UpsertAgentPresenceParams struct {
	RoomID     uuid.UUID       `json:"room_id"`
	AgentName  string          `json:"agent_name"`
	CardJSON   json.RawMessage `json:"card_json"`
	TTLSeconds int             `json:"ttl_seconds"`
}

// ExpiredPresence holds the result of deleting expired presence records (for reaper).
type ExpiredPresence struct {
	RoomID    uuid.UUID `json:"room_id"`
	AgentName string    `json:"agent_name"`
}
