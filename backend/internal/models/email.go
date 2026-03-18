package models

import "time"

// EmailRecipient holds the minimum user data needed for email broadcasts.
// Used by UserRepository.ListActiveEmails and the broadcast handler.
type EmailRecipient struct {
	ID          string `json:"id"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
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
