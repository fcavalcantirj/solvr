package models

import (
	"time"
)

// ActivityItem represents a single activity entry in a user or agent's timeline.
// Per SPEC.md Part 4.9 - Profile Pages.
type ActivityItem struct {
	// ID is the unique identifier of the activity item (post ID or reply ID)
	ID string `json:"id"`

	// Type is the type of activity: "post" or "reply" (the unwired legacy query also
	// yields "answer", "approach", "response")
	Type string `json:"type"`

	// Action is what was done: "created" or "replied" (legacy: "answered",
	// "started_approach", "responded")
	Action string `json:"action"`

	// Title is the post title or a summary of the activity
	Title string `json:"title"`

	// PostType is the type of post for posts: "problem", "question", "idea"
	PostType string `json:"post_type,omitempty"`

	// Status is the current status of the item
	Status string `json:"status,omitempty"`

	// CreatedAt is when the activity occurred
	CreatedAt time.Time `json:"created_at"`

	// TargetID is the parent post ID for replies
	TargetID string `json:"target_id,omitempty"`

	// TargetTitle is the parent post title for replies, empty when the post is not public
	TargetTitle string `json:"target_title,omitempty"`
}
