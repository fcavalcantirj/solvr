package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// WebhookDeliveryStatus is where one queued delivery stands.
type WebhookDeliveryStatus string

// A delivery is pending until a send is answered 2xx (delivered) or its last retry fails
// (failed).
const (
	WebhookDeliveryPending   WebhookDeliveryStatus = "pending"
	WebhookDeliveryDelivered WebhookDeliveryStatus = "delivered"
	WebhookDeliveryFailed    WebhookDeliveryStatus = "failed"
)

// WebhookDelivery is one notification event queued for one webhook (SPEC.md Part 12.3). It
// is recorded with the notification, at most once per webhook and notification, and every
// attempt sends the same ID, event and data: a retry never looks like a second event.
type WebhookDelivery struct {
	ID             uuid.UUID
	WebhookID      uuid.UUID
	NotificationID uuid.UUID
	Event          string
	SchemaVersion  int
	// Data is the event's data, fixed when the delivery was queued.
	Data       json.RawMessage
	OccurredAt time.Time
	// Attempts is how many sends were made before this one.
	Attempts int
	Status   WebhookDeliveryStatus
	// Webhook is the subscription it goes to, Secret opened for the signature.
	Webhook Webhook
}

// WebhookDeliveryAttempt is the outcome of one send. Delivered is a 2xx answer; otherwise
// NextAttemptAt schedules the retry, or is nil when the attempt was the last one.
type WebhookDeliveryAttempt struct {
	Delivered     bool
	StatusCode    *int
	Error         string
	NextAttemptAt *time.Time
}

// WebhookDeliveryPayload is the JSON body of every delivery (SPEC.md Part 12.3): ID is the
// delivery ID (X-Solvr-Delivery-ID), the same on every attempt, so a receiver acts on it once.
type WebhookDeliveryPayload struct {
	ID            string          `json:"id"`
	Event         string          `json:"event"`
	SchemaVersion int             `json:"schema_version"`
	Timestamp     string          `json:"timestamp"`
	Data          json.RawMessage `json:"data"`
}
