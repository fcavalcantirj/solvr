package models

import "time"

// EmailRecipient holds the minimum user data needed for email broadcasts.
// Used by UserRepository.ListActiveEmails and the broadcast handler.
type EmailRecipient struct {
	ID           string `json:"id"`
	Email        string `json:"email"`
	DisplayName  string `json:"display_name"`
	ReferralCode string `json:"referral_code"`
}

// EmailBroadcast represents a record in the email_broadcast_logs table.
// Each broadcast creates one log entry that tracks send progress and completion.
type EmailBroadcast struct {
	ID              string     `json:"id"`
	Subject         string     `json:"subject"`
	BodyHTML        string     `json:"body_html"`
	BodyText        string     `json:"body_text,omitempty"`
	TotalRecipients int        `json:"total_recipients"`
	SentCount       int        `json:"sent_count"`
	FailedCount     int        `json:"failed_count"`
	Status          string     `json:"status"`
	StartedAt       time.Time  `json:"started_at"`
	CompletedAt     *time.Time `json:"completed_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
}

// Email delivery (SPEC.md 16.6): what Solvr sent, what the provider reported about it, and
// the addresses Solvr no longer mails.
const (
	// EmailKindBroadcast is a message of POST /admin/email/broadcast: the marketing sender,
	// the branded footer and the List-Unsubscribe headers.
	EmailKindBroadcast = "broadcast"
	// EmailKindTransactional is a notice of POST /admin/email/notice: its own sender, no
	// footer and no List-Unsubscribe headers.
	EmailKindTransactional = "transactional"

	// EmailStatusSent is a message the provider accepted; EmailStatusFailed one it refused.
	// The provider's later events move a message to the event's own name (delivered,
	// delivery_delayed, bounced, complained, failed, suppressed).
	EmailStatusSent   = "sent"
	EmailStatusFailed = "failed"

	// EmailSuppressedHardBounce and EmailSuppressedComplaint are why Solvr no longer mails an
	// address (users.email_suppressed_reason).
	EmailSuppressedHardBounce = "hard_bounce"
	EmailSuppressedComplaint  = "complaint"
)

// EmailMessage is one email Solvr sent, or tried to send: a row of email_messages.
type EmailMessage struct {
	Kind              string // EmailKindBroadcast or EmailKindTransactional
	BroadcastID       string // the broadcast it belongs to; empty for a notice
	UserID            string // the recipient
	ProviderMessageID string // the provider's id for it; empty when the send failed
	Subject           string
	Status            string // EmailStatusSent or EmailStatusFailed
}

// EmailEvent is one event the provider reported about a message: a row of email_events.
type EmailEvent struct {
	OccurredAt        time.Time
	SvixID            string // the webhook message id: the same on every retry of one event
	ProviderMessageID string // the message the event is about; empty when it names none
	EventType         string // the provider's event type, e.g. email.bounced
	BounceType        string // a bounce's type (Permanent, Transient, Undetermined); else empty
}

// EmailEventEffect is what an event changes besides being stored.
type EmailEventEffect struct {
	Status      string   // the message's new status; empty changes nothing
	Suppress    string   // EmailSuppressedHardBounce or EmailSuppressedComplaint; empty suppresses no one
	Recipients  []string // the addresses the event is about, lower-cased
	Unsubscribe bool     // a complaint also unsubscribes the address from broadcasts
}
