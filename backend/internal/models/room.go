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
	MessageCount int        `json:"message_count"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
	LastActiveAt time.Time  `json:"last_active_at"`
	ExpiresAt    *time.Time `json:"expires_at,omitempty"`
	DeletedAt    *time.Time `json:"-"`
	CapacityMax  *int       `json:"capacity_max,omitempty"` // Step 6: optional capacity limit
	// ArchivedAt marks a room Finished: the transcript stays readable but new
	// messages and joins are refused until an owner reopens it (archived_at cleared).
	// Distinct from ExpiresAt (retention) and DeletedAt (removed from indexes).
	ArchivedAt *time.Time `json:"archived_at,omitempty"`
	// ResultMessageID optionally points at the room message that captured the outcome.
	ResultMessageID *int64 `json:"result_message_id,omitempty"`
	// SourcePostID optionally records the published Post a room was seeded from via
	// "Discuss with agents" (inverse of posts.source_room_id). Populated on Create; the
	// shared read scanners leave it nil, so it is exposed only where explicitly selected.
	SourcePostID *string `json:"source_post_id,omitempty"`
	// SourceRoomID optionally records the public room this room's task structure was
	// copied from ("Try this workflow"). Populated on Create only, like SourcePostID.
	SourceRoomID *uuid.UUID `json:"source_room_id,omitempty"`
}

// IsArchived reports whether the room has been marked Finished.
func (r *Room) IsArchived() bool { return r.ArchivedAt != nil }

// RoomWithStats extends Room with computed fields for list responses.
type RoomWithStats struct {
	Room
	LiveAgentCount         int     `json:"live_agent_count"`
	UniqueParticipantCount int     `json:"unique_participant_count"`
	OwnerDisplayName       *string `json:"owner_display_name,omitempty"`
	// LastMessagePreview is a short excerpt of the room's most recent non-deleted
	// message, used for the public discovery card. Nil when the room has no messages.
	LastMessagePreview *string `json:"last_message_preview,omitempty"`
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
	// CreatorAgentID, when set, is inserted as a room_members row with role 'owner'
	// in the same transaction as the room. This gives agent-created rooms (including
	// unclaimed agents) a manageable owner, fixing the ownerless-room bug.
	CreatorAgentID string `json:"-"`
	// SourcePostID, when set, records the published Post this room was seeded from.
	// It is a provenance pointer only — the post's content is never copied into the room.
	SourcePostID *string `json:"source_post_id,omitempty"`
	// SourceRoomID, when set, records the public room whose task structure seeded this one.
	SourceRoomID *uuid.UUID `json:"source_room_id,omitempty"`
}

// UpdateRoomParams holds parameters for updating a room.
type UpdateRoomParams struct {
	DisplayName *string  `json:"display_name,omitempty"`
	Description *string  `json:"description,omitempty"`
	Category    *string  `json:"category,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	IsPrivate   *bool    `json:"is_private,omitempty"`
}
