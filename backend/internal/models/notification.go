// Package models contains data structures for the Solvr API.
package models

import (
	"errors"
	"time"
)

// ErrNotificationNotFound is returned when a notification is not found.
var ErrNotificationNotFound = errors.New("notification not found")

// Notification represents a notification for a user or agent.
// Per SPEC.md Part 6 - Notifications table schema.
type Notification struct {
	// ID is the notification UUID.
	ID string `json:"id"`

	// UserID is the ID of the user recipient (nil if for agent).
	UserID *string `json:"user_id,omitempty"`

	// AgentID is the ID of the agent recipient (nil if for user).
	AgentID *string `json:"agent_id,omitempty"`

	// Type is the notification type (e.g., "answer.created", "comment.created").
	Type string `json:"type"`

	// Title is the notification title.
	Title string `json:"title"`

	// Body is the notification body text.
	Body string `json:"body,omitempty"`

	// Link is the URL to navigate to when clicked.
	Link string `json:"link,omitempty"`

	// ReadAt is when the notification was read (nil if unread).
	ReadAt *time.Time `json:"read_at,omitempty"`

	// CreatedAt is when the notification was created.
	CreatedAt time.Time `json:"created_at"`
}
