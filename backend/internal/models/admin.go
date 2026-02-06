// Package models defines data structures for the Solvr API.
package models

import (
	"time"

	"github.com/google/uuid"
)

// Flag represents a content flag/report per SPEC.md Part 8.4
type Flag struct {
	ID           uuid.UUID  `json:"id"`
	TargetType   string     `json:"target_type"`   // post, comment, answer, approach, response
	TargetID     uuid.UUID  `json:"target_id"`
	ReporterType string     `json:"reporter_type"` // human, agent, system
	ReporterID   string     `json:"reporter_id"`
	Reason       string     `json:"reason"`        // spam, offensive, duplicate, incorrect, low_quality, other
	Details      string     `json:"details,omitempty"`
	Status       string     `json:"status"`        // pending, reviewed, dismissed, actioned
	ReviewedBy   *uuid.UUID `json:"reviewed_by,omitempty"`
	ReviewedAt   *time.Time `json:"reviewed_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
}

// FlagListOptions contains options for listing flags
type FlagListOptions struct {
	Status     string // Filter by status
	TargetType string // Filter by target type
	Page       int
	PerPage    int
}

// ValidFlagStatuses defines the valid flag statuses
var ValidFlagStatuses = []string{"pending", "reviewed", "dismissed", "actioned"}

// ValidFlagReasons defines the valid flag reasons
var ValidFlagReasons = []string{"spam", "offensive", "duplicate", "incorrect", "low_quality", "other"}

// ValidFlagTargetTypes defines the valid flag target types
var ValidFlagTargetTypes = []string{"post", "comment", "answer", "approach", "response"}

// IsValidFlagStatus checks if a status is valid
func IsValidFlagStatus(status string) bool {
	for _, s := range ValidFlagStatuses {
		if s == status {
			return true
		}
	}
	return false
}

// IsValidFlagReason checks if a reason is valid
func IsValidFlagReason(reason string) bool {
	for _, r := range ValidFlagReasons {
		if r == reason {
			return true
		}
	}
	return false
}

// AuditLog represents an admin action audit log entry per SPEC.md Part 16
type AuditLog struct {
	ID         uuid.UUID              `json:"id"`
	AdminID    uuid.UUID              `json:"admin_id"`
	Action     string                 `json:"action"`
	TargetType string                 `json:"target_type,omitempty"`
	TargetID   *uuid.UUID             `json:"target_id,omitempty"`
	Details    map[string]interface{} `json:"details,omitempty"`
	IPAddress  string                 `json:"ip_address,omitempty"`
	CreatedAt  time.Time              `json:"created_at"`
}

// AuditListOptions contains options for listing audit log entries
type AuditListOptions struct {
	Action   string    // Filter by action
	AdminID  *uuid.UUID // Filter by admin
	FromDate *time.Time
	ToDate   *time.Time
	Page     int
	PerPage  int
}

// AdminStats represents system statistics for the admin dashboard per SPEC.md Part 16.3
type AdminStats struct {
	UsersCount     int `json:"users_count"`
	AgentsCount    int `json:"agents_count"`
	PostsCount     int `json:"posts_count"`
	RateLimitHits  int `json:"rate_limit_hits"`
	FlagsCount     int `json:"flags_count,omitempty"`
	ActiveUsers24h int `json:"active_users_24h,omitempty"`
}

// UserListOptions contains options for listing users
type UserListOptions struct {
	Query   string // Search query
	Status  string // Filter by status
	Page    int
	PerPage int
}

// AgentListOptions contains options for listing agents
type AgentListOptions struct {
	Query   string     // Search query
	Status  string     // Filter by status (active, pending, all)
	OwnerID *uuid.UUID // Filter by owner
	Sort    string     // Sort order: newest, oldest, karma, posts
	Page    int
	PerPage int
}

// AgentWithPostCount is an Agent with computed post count for listing.
// Per API-001: GET /v1/agents includes post_count for each agent.
type AgentWithPostCount struct {
	ID                  string     `json:"id"`
	DisplayName         string     `json:"display_name"`
	Bio                 string     `json:"bio,omitempty"`
	Status              string     `json:"status"`
	Karma               int        `json:"karma"`
	PostCount           int        `json:"post_count"`
	CreatedAt           time.Time  `json:"created_at"`
	HasHumanBackedBadge bool       `json:"has_human_backed_badge"`
	AvatarURL           string     `json:"avatar_url,omitempty"`
}

// FlagAction represents an action to take on a flag
type FlagAction struct {
	Action string `json:"action"` // warn, hide, delete
	Reason string `json:"reason,omitempty"`
}

// ValidFlagActions defines the valid actions for flags
var ValidFlagActions = []string{"warn", "hide", "delete"}

// IsValidFlagAction checks if an action is valid
func IsValidFlagAction(action string) bool {
	for _, a := range ValidFlagActions {
		if a == action {
			return true
		}
	}
	return false
}

// UserStatus represents user account status
type UserStatus string

const (
	UserStatusActive    UserStatus = "active"
	UserStatusSuspended UserStatus = "suspended"
	UserStatusBanned    UserStatus = "banned"
)

// AgentStatus represents agent status
type AgentStatus string

const (
	AgentStatusActive    AgentStatus = "active"
	AgentStatusSuspended AgentStatus = "suspended"
)
