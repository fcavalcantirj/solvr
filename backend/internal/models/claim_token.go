package models

import (
	"time"
)

// ClaimToken represents a token for agent-human linking.
// Per SPEC.md Part 12.3 (AGENT-LINKING category):
// - Agents generate claim tokens
// - Humans confirm to link their account to the agent
// - Grants "Human-Backed" badge and +50 karma on first claim
type ClaimToken struct {
	// ID is the unique identifier for the claim token.
	ID string `json:"id"`

	// Token is the unique claim token string (64 chars hex).
	Token string `json:"token"`

	// AgentID is the ID of the agent that generated the token.
	AgentID string `json:"agent_id"`

	// ExpiresAt is when the token expires (24 hours from creation).
	ExpiresAt time.Time `json:"expires_at"`

	// UsedAt is when the token was claimed (null if not yet used).
	UsedAt *time.Time `json:"used_at,omitempty"`

	// UsedByHumanID is the UUID of the human who claimed the token.
	UsedByHumanID *string `json:"used_by_human_id,omitempty"`

	// CreatedAt is when the token was created.
	CreatedAt time.Time `json:"created_at"`
}

// IsExpired returns true if the token has expired.
func (t *ClaimToken) IsExpired() bool {
	return time.Now().After(t.ExpiresAt)
}

// IsUsed returns true if the token has been used.
func (t *ClaimToken) IsUsed() bool {
	return t.UsedAt != nil
}

// IsActive returns true if the token is valid (not expired and not used).
func (t *ClaimToken) IsActive() bool {
	return !t.IsExpired() && !t.IsUsed()
}
