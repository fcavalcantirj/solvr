package models

import (
	"time"
)

// User represents a human user on Solvr.
// Per SPEC.md Part 2.8 and Part 6 (users table).
type User struct {
	// ID is the unique UUID for the user.
	ID string `json:"id"`

	// Username is the unique handle for the user.
	// Max 30 chars.
	Username string `json:"username"`

	// DisplayName is the human-readable name.
	// Max 50 chars.
	DisplayName string `json:"display_name"`

	// Email is the user's email address (unique).
	Email string `json:"email"`

	// AuthProvider is the OAuth provider (github, google).
	AuthProvider string `json:"auth_provider"`

	// AuthProviderID is the ID from the OAuth provider.
	AuthProviderID string `json:"auth_provider_id"`

	// AvatarURL is an optional URL to the user's avatar.
	AvatarURL string `json:"avatar_url,omitempty"`

	// Bio is an optional description of the user.
	// Max 500 chars.
	Bio string `json:"bio,omitempty"`

	// Role is the user's role (user, admin).
	Role string `json:"role"`

	// Status is the account status (active, suspended, banned).
	Status string `json:"status"`

	// CreatedAt is when the user was created.
	CreatedAt time.Time `json:"created_at"`

	// UpdatedAt is when the user was last modified.
	UpdatedAt time.Time `json:"updated_at"`
}

// UserStats contains computed statistics for a user.
type UserStats struct {
	PostsCreated    int `json:"posts_created"`
	AnswersGiven    int `json:"answers_given"`
	AnswersAccepted int `json:"answers_accepted"`
	UpvotesReceived int `json:"upvotes_received"`
	Reputation      int `json:"reputation"`
}

// UserWithStats is a User with computed statistics.
type UserWithStats struct {
	User
	Stats UserStats `json:"stats"`
}

// AuthProvider constants
const (
	AuthProviderGitHub = "github"
	AuthProviderGoogle = "google"
)

// UserRole constants
const (
	UserRoleUser  = "user"
	UserRoleAdmin = "admin"
)
