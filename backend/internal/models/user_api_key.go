package models

import (
	"time"
)

// UserAPIKey represents an API key for a human user.
// Per prd-v2.json API-KEYS requirements.
// Users can have multiple API keys with different names for different purposes.
type UserAPIKey struct {
	// ID is the unique UUID for the API key.
	ID string `json:"id"`

	// UserID is the ID of the user who owns this key.
	UserID string `json:"user_id"`

	// Name is a user-provided name for the key (e.g., "Production", "Development").
	// Max 100 chars.
	Name string `json:"name"`

	// KeyHash is the bcrypt hash of the API key.
	// NEVER expose this in JSON responses.
	KeyHash string `json:"-"`

	// LastUsedAt tracks when the key was last used (for security audit).
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`

	// RevokedAt is set when the key is revoked (soft delete).
	// NULL means the key is active.
	RevokedAt *time.Time `json:"revoked_at,omitempty"`

	// CreatedAt is when the key was created.
	CreatedAt time.Time `json:"created_at"`

	// UpdatedAt is when the key was last modified.
	UpdatedAt time.Time `json:"updated_at"`
}

// UserAPIKeyWithPreview is used when returning a list of keys.
// Shows only a preview of the key (first and last few chars) for identification.
type UserAPIKeyWithPreview struct {
	ID         string     `json:"id"`
	UserID     string     `json:"user_id"`
	Name       string     `json:"name"`
	KeyPreview string     `json:"key_preview"` // e.g., "solvr_...abc123"
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	RevokedAt  *time.Time `json:"revoked_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

// UserAPIKeyCreated is returned when creating a new key.
// Contains the full key which is shown only once.
type UserAPIKeyCreated struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Name      string    `json:"name"`
	Key       string    `json:"key"` // Full key, shown only once!
	CreatedAt time.Time `json:"created_at"`
}

// IsRevoked returns true if the key has been revoked.
func (k *UserAPIKey) IsRevoked() bool {
	return k.RevokedAt != nil
}

// IsActive returns true if the key is active (not revoked).
func (k *UserAPIKey) IsActive() bool {
	return k.RevokedAt == nil
}
