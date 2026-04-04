package models

import (
	"time"

	"github.com/google/uuid"
)

// Room represents a room in the Solvr platform.
// Fields match migration 000073_create_rooms.up.sql.
type Room struct {
	ID           uuid.UUID  `json:"id"`
	Slug         string     `json:"slug"`
	DisplayName  string     `json:"display_name"`
	Description  *string    `json:"description,omitempty"`
	Category     *string    `json:"category,omitempty"`
	Tags         []string   `json:"tags"`
	IsPrivate    bool       `json:"is_private"`
	OwnerID      *uuid.UUID `json:"owner_id,omitempty"`
	TokenHash    string     `json:"-"`
	MessageCount int        `json:"message_count"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
	LastActiveAt time.Time  `json:"last_active_at"`
	ExpiresAt    *time.Time `json:"expires_at,omitempty"`
	DeletedAt    *time.Time `json:"-"`
}

// RoomWithStats extends Room with computed fields for list responses.
type RoomWithStats struct {
	Room
	LiveAgentCount         int     `json:"live_agent_count"`
	UniqueParticipantCount int     `json:"unique_participant_count"`
	OwnerDisplayName       *string `json:"owner_display_name,omitempty"`
}

// CreateRoomParams holds parameters for creating a room.
type CreateRoomParams struct {
	Slug        string     `json:"slug,omitempty"`
	DisplayName string     `json:"display_name"`
	Description *string    `json:"description,omitempty"`
	Category    *string    `json:"category,omitempty"`
	Tags        []string   `json:"tags,omitempty"`
	IsPrivate   bool       `json:"is_private"`
	OwnerID     uuid.UUID  `json:"owner_id"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
}

// UpdateRoomParams holds parameters for updating a room.
type UpdateRoomParams struct {
	DisplayName *string  `json:"display_name,omitempty"`
	Description *string  `json:"description,omitempty"`
	Category    *string  `json:"category,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	IsPrivate   *bool    `json:"is_private,omitempty"`
}
