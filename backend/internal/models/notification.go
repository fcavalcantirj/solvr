// Package models contains data structures for the Solvr API.
package models

import (
	"errors"
	"time"
)

// ErrNotificationNotFound is returned when a notification is not found.
var ErrNotificationNotFound = errors.New("notification not found")

// NotificationSchemaVersion is the version of the notification event contract (SPEC.md Part
// 5.6): an event written under it has a type from the documented set and names the canonical
// post and reply it is about in Subject. Version 0 marks a row written outside the contract
// (retired producers, rows recorded before it): its type may be a retired name and it has no
// subject.
const NotificationSchemaVersion = 1

// Notification event types of schema version 1 that name a canonical subject.
// post.approved / post.rejected name the post; reply.removed / reply.flagged name the reply
// and its post. blog_post_rejected (the blog is not knowledge) names no subject.
const (
	NotificationPostApproved = "post.approved"
	NotificationPostRejected = "post.rejected"
	NotificationReplyRemoved = "reply.removed"
	NotificationReplyFlagged = "reply.flagged"
)

// NotificationSubject names the canonical post and reply a notification event is about.
type NotificationSubject struct {
	PostID  *string `json:"post_id,omitempty"`
	ReplyID *string `json:"reply_id,omitempty"`
}

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

	// SchemaVersion is the event contract the notification was written under
	// (NotificationSchemaVersion), 0 when outside it.
	SchemaVersion int `json:"schema_version"`

	// Subject names the canonical post and reply the event is about.
	Subject NotificationSubject `json:"subject"`
}

// NotificationFilters holds optional query filters for listing notifications.
type NotificationFilters struct {
	// Unread filters to only unread notifications (read_at IS NULL) when non-nil and true.
	Unread *bool
	// Type filters by notification type when non-empty.
	Type string
}
