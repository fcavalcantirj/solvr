// Package models contains data structures for the Solvr application.
package models

import (
	"time"

	"github.com/google/uuid"
)

// WebhookStatus represents the status of a webhook.
type WebhookStatus string

// Webhook status constants per SPEC.md Part 12.3.
const (
	WebhookStatusActive   WebhookStatus = "active"
	WebhookStatusPaused   WebhookStatus = "paused"
	WebhookStatusFailing  WebhookStatus = "failing"
	WebhookStatusDisabled WebhookStatus = "disabled"
)

// ValidWebhookStatuses lists all valid webhook statuses.
var ValidWebhookStatuses = []WebhookStatus{
	WebhookStatusActive,
	WebhookStatusPaused,
	WebhookStatusFailing,
	WebhookStatusDisabled,
}

// IsValidWebhookStatus checks if a status is valid.
func IsValidWebhookStatus(status string) bool {
	for _, s := range ValidWebhookStatuses {
		if string(s) == status {
			return true
		}
	}
	return false
}

// WebhookEventType represents the type of webhook event.
type WebhookEventType string

// The events a webhook subscribes to are the notification events of schema version 1
// (NotificationSchemaVersion, SPEC.md Part 5.6 and 12.3): a webhook delivers the agent's
// notifications of the subscribed types as they are recorded.
const (
	WebhookEventPostApproved     WebhookEventType = NotificationPostApproved
	WebhookEventPostRejected     WebhookEventType = NotificationPostRejected
	WebhookEventReplyRemoved     WebhookEventType = NotificationReplyRemoved
	WebhookEventReplyFlagged     WebhookEventType = NotificationReplyFlagged
	WebhookEventBlogPostRejected WebhookEventType = NotificationBlogPostRejected
)

// ValidWebhookEventTypes lists all valid webhook event types.
var ValidWebhookEventTypes = []WebhookEventType{
	WebhookEventPostApproved,
	WebhookEventPostRejected,
	WebhookEventReplyRemoved,
	WebhookEventReplyFlagged,
	WebhookEventBlogPostRejected,
}

// RetiredWebhookEventTypes are the event names of the problem/question/idea model. Nothing
// produces them: a subscription to one is refused with a migration error, never stored.
var RetiredWebhookEventTypes = []WebhookEventType{
	"answer.created",
	"comment.created",
	"approach.stuck",
	"problem.solved",
	"mention",
}

// IsValidWebhookEventType checks if an event type is valid.
func IsValidWebhookEventType(eventType string) bool {
	for _, e := range ValidWebhookEventTypes {
		if string(e) == eventType {
			return true
		}
	}
	return false
}

// IsRetiredWebhookEventType reports whether eventType is a retired event name.
func IsRetiredWebhookEventType(eventType string) bool {
	for _, e := range RetiredWebhookEventTypes {
		if string(e) == eventType {
			return true
		}
	}
	return false
}

// ValidateWebhookEvents validates a list of event types.
// Returns the first invalid event type, or empty string if all valid.
func ValidateWebhookEvents(events []string) string {
	for _, e := range events {
		if !IsValidWebhookEventType(e) {
			return e
		}
	}
	return ""
}

// Webhook represents a webhook subscription per SPEC.md Part 12.3.
type Webhook struct {
	ID        uuid.UUID     `json:"id"`
	AgentID   string        `json:"agent_id"`
	URL       string        `json:"url"`
	Events    []string      `json:"events"`
	Status    WebhookStatus `json:"status"`

	// Secret hash is never included in JSON responses
	SecretHash string `json:"-"`

	// Secret is the signing secret in the clear: set by a create or a secret change, and by
	// the delivery queue for the send. The repository stores it only sealed; it is never
	// included in JSON responses.
	Secret string `json:"-"`

	// Failure tracking
	ConsecutiveFailures int        `json:"consecutive_failures"`
	LastFailureAt       *time.Time `json:"last_failure_at,omitempty"`
	LastSuccessAt       *time.Time `json:"last_success_at,omitempty"`

	// Timestamps
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// WebhookPayload represents the payload sent to a webhook.
// Per SPEC.md Part 12.3.
type WebhookPayload struct {
	Event     string                 `json:"event"`
	Timestamp string                 `json:"timestamp"`
	Data      map[string]interface{} `json:"data"`
	Signature string                 `json:"signature,omitempty"`
}

// CreateWebhookRequest is the request body for creating a webhook.
// Per SPEC.md Part 12.3.
type CreateWebhookRequest struct {
	URL    string   `json:"url"`
	Events []string `json:"events"`
	Secret string   `json:"secret"`
}

// UpdateWebhookRequest is the request body for updating a webhook.
// Per SPEC.md Part 12.3.
type UpdateWebhookRequest struct {
	URL    *string  `json:"url,omitempty"`
	Events []string `json:"events,omitempty"`
	Secret *string  `json:"secret,omitempty"`
	Status *string  `json:"status,omitempty"`
}

// WebhookListOptions contains options for listing webhooks.
type WebhookListOptions struct {
	AgentID string
	Status  string
	Page    int
	PerPage int
}
