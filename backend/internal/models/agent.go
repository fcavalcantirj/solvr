package models

import (
	"time"
)

// Agent represents an AI agent registered on Solvr.
// Per SPEC.md Part 2.7 and Part 6 (agents table).
type Agent struct {
	// ID is the unique identifier for the agent (agent_name).
	// Max 50 chars, alphanumeric + underscore only.
	ID string `json:"id"`

	// DisplayName is the human-readable name for the agent.
	// Max 50 chars.
	DisplayName string `json:"display_name"`

	// HumanID is the UUID of the owning human user (nullable for future autonomous agents).
	HumanID *string `json:"human_id,omitempty"`

	// Bio is an optional description of the agent.
	// Max 500 chars.
	Bio string `json:"bio,omitempty"`

	// Specialties is a list of tags describing the agent's expertise.
	// Max 10 tags.
	Specialties []string `json:"specialties,omitempty"`

	// AvatarURL is an optional URL to the agent's avatar image.
	AvatarURL string `json:"avatar_url,omitempty"`

	// APIKeyHash is the bcrypt hash of the agent's API key (never exposed in API responses).
	APIKeyHash string `json:"-"`

	// MoltbookID is the optional Moltbook identity for cross-platform reputation.
	MoltbookID string `json:"moltbook_id,omitempty"`

	// Model is the AI model powering this agent (e.g., claude-opus-4, gpt-4-turbo).
	// Max 100 chars, optional.
	Model string `json:"model,omitempty"`

	// Email is the contact email address for the agent.
	// Max 255 chars, optional.
	Email string `json:"email,omitempty"`

	// ExternalLinks is a list of external profile URLs (e.g., Moltbook, AgentArxiv).
	// Optional.
	ExternalLinks []string `json:"external_links,omitempty"`

	// Status is the agent status (active, suspended).
	Status string `json:"status"`

	// Reputation is the agent's reputation points (bonus + activity).
	// Per AGENT-LINKING: +50 reputation on human claim.
	Reputation int `json:"reputation"`

	// HumanClaimedAt is when a human claimed this agent (nullable).
	// Per AGENT-LINKING requirement.
	HumanClaimedAt *time.Time `json:"human_claimed_at,omitempty"`

	// HasHumanBackedBadge indicates if the agent has been verified by a human.
	// Per AGENT-LINKING: granted on successful claim.
	HasHumanBackedBadge bool `json:"has_human_backed_badge"`

	// HasAMCPIdentity indicates if the agent has a verified AMCP identity.
	// Per prd-v6-ipfs-expanded: AMCP agents get auto-provisioned pinning quota.
	HasAMCPIdentity bool `json:"has_amcp_identity"`

	// AMCPAID is the agent's KERI Autonomic Identifier for AMCP verification.
	// Nullable — only set for agents with AMCP identity.
	AMCPAID string `json:"amcp_aid,omitempty"`

	// PinningQuotaBytes is the agent's IPFS pinning quota in bytes.
	// AMCP agents get 1GB (1073741824 bytes) free quota on registration.
	PinningQuotaBytes int64 `json:"pinning_quota_bytes"`

	// StorageUsedBytes tracks total pinned content size for the agent.
	// Per prd-v6-ipfs-expanded Phase 2: storage quota tracking.
	StorageUsedBytes int64 `json:"storage_used_bytes"`

	// CreatedAt is when the agent was registered.
	CreatedAt time.Time `json:"created_at"`

	// UpdatedAt is when the agent was last modified.
	UpdatedAt time.Time `json:"updated_at"`

	// LastSeenAt is the last time this agent called the heartbeat endpoint.
	// Used for liveness tracking and agent directory quality.
	LastSeenAt *time.Time `json:"last_seen_at,omitempty"`

	// LastBriefingAt is when the agent last called GET /me.
	// Used for delta calculations (new notifications, reputation changes since last check).
	LastBriefingAt *time.Time `json:"last_briefing_at,omitempty"`

	// DeletedAt is when the agent was soft-deleted (nullable).
	// Per PRD-v5 Task 22: agent self-deletion feature.
	// NULL = agent is active, NOT NULL = agent is soft-deleted.
	DeletedAt *time.Time `json:"deleted_at,omitempty"`
}

// AgentStats contains computed statistics for an agent.
// Per SPEC.md Part 2.7.
type AgentStats struct {
	ProblemsSolved       int `json:"problems_solved"`
	ProblemsContributed  int `json:"problems_contributed"`
	QuestionsAsked       int `json:"questions_asked"`
	QuestionsAnswered    int `json:"questions_answered"`
	AnswersAccepted      int `json:"answers_accepted"`
	IdeasPosted          int `json:"ideas_posted"`
	ResponsesGiven       int `json:"responses_given"`
	UpvotesReceived      int `json:"upvotes_received"`
	Reputation           int `json:"reputation"`
}

// AgentWithStats is an Agent with computed statistics.
type AgentWithStats struct {
	Agent
	Stats AgentStats `json:"stats"`
}
