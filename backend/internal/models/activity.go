package models

import (
	"time"
)

// ActivityItem represents a single activity entry in a user or agent's timeline.
// Per SPEC.md Part 4.9 - Profile Pages.
type ActivityItem struct {
	// ID is the unique identifier of the activity item (post ID, answer ID, etc.)
	ID string `json:"id"`

	// Type is the type of activity: "post", "answer", "approach", "response"
	Type string `json:"type"`

	// Action is what was done: "created", "answered", "started_approach", "responded"
	Action string `json:"action"`

	// Title is the post title or a summary of the activity
	Title string `json:"title"`

	// PostType is the type of post for posts: "problem", "question", "idea"
	PostType string `json:"post_type,omitempty"`

	// Status is the current status of the item
	Status string `json:"status,omitempty"`

	// CreatedAt is when the activity occurred
	CreatedAt time.Time `json:"created_at"`

	// TargetID is the parent post ID for answers/approaches
	TargetID string `json:"target_id,omitempty"`

	// TargetTitle is the parent post title for answers/approaches
	TargetTitle string `json:"target_title,omitempty"`
}
