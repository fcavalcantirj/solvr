package auth

import (
	"context"
	"strings"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// AgentDB defines the interface for agent database operations.
// This allows for easy mocking in tests and decoupling from the actual database.
type AgentDB interface {
	// GetAgentByAPIKeyHash finds an agent by checking the API key against stored hashes.
	// Returns nil, nil if no matching agent is found.
	GetAgentByAPIKeyHash(ctx context.Context, key string) (*models.Agent, error)
}

// APIKeyValidator validates API keys against the database.
type APIKeyValidator struct {
	db AgentDB
}

// NewAPIKeyValidator creates a new APIKeyValidator with the given database.
func NewAPIKeyValidator(db AgentDB) *APIKeyValidator {
	return &APIKeyValidator{db: db}
}

// ValidateAPIKey validates an API key and returns the associated agent.
// Returns an AuthError if the key is invalid, malformed, or not found.
func (v *APIKeyValidator) ValidateAPIKey(ctx context.Context, key string) (*models.Agent, error) {
	// Check for empty key
	if key == "" {
		return nil, NewAuthError(ErrCodeInvalidAPIKey, "API key is required")
	}

	// Check for solvr_ prefix
	if !IsAPIKey(key) {
		return nil, NewAuthError(ErrCodeInvalidAPIKey, "invalid API key format")
	}

	// Query database for agent with matching key
	agent, err := v.db.GetAgentByAPIKeyHash(ctx, key)
	if err != nil {
		return nil, NewAuthError(ErrCodeInvalidAPIKey, "failed to validate API key")
	}

	// No matching agent found
	if agent == nil {
		return nil, NewAuthError(ErrCodeInvalidAPIKey, "invalid API key")
	}

	return agent, nil
}

// IsAPIKey checks if a string looks like a Solvr API key (has the solvr_ prefix).
func IsAPIKey(key string) bool {
	return strings.HasPrefix(key, APIKeyPrefix)
}
