package models

import "time"

// AuthMethod represents one authentication method for a user.
// Users can have multiple auth methods (email + Google + GitHub).
type AuthMethod struct {
	ID             string    `json:"id"`
	UserID         string    `json:"user_id"`
	AuthProvider   string    `json:"auth_provider"`    // "email", "github", "google"
	AuthProviderID string    `json:"auth_provider_id"` // NULL for email/password, provider ID for OAuth
	PasswordHash   string    `json:"-"`                // Only for email method, never serialize
	CreatedAt      time.Time `json:"created_at"`
	LastUsedAt     time.Time `json:"last_used_at"`
}
