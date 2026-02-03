package api

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// Error types for agent operations
var (
	errDuplicateAgentID   = errors.New("agent ID already exists")
	errDuplicateAgentName = errors.New("agent name already exists")
	errAgentNotFound      = errors.New("agent not found")
)

// InMemoryAgentRepository is an in-memory implementation of AgentRepositoryInterface.
// Used for testing when no database is available.
type InMemoryAgentRepository struct {
	mu           sync.RWMutex
	agents       map[string]*models.Agent
	agentsByName map[string]*models.Agent
}

// NewInMemoryAgentRepository creates a new in-memory agent repository.
func NewInMemoryAgentRepository() *InMemoryAgentRepository {
	return &InMemoryAgentRepository{
		agents:       make(map[string]*models.Agent),
		agentsByName: make(map[string]*models.Agent),
	}
}

// Create creates a new agent in memory.
func (r *InMemoryAgentRepository) Create(ctx context.Context, agent *models.Agent) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Check for duplicate ID
	if _, exists := r.agents[agent.ID]; exists {
		return errDuplicateAgentID
	}

	// Check for duplicate name
	if _, exists := r.agentsByName[agent.DisplayName]; exists {
		return errDuplicateAgentName
	}

	// Set timestamps if not already set
	now := time.Now()
	if agent.CreatedAt.IsZero() {
		agent.CreatedAt = now
	}
	if agent.UpdatedAt.IsZero() {
		agent.UpdatedAt = now
	}

	// Store agent
	agentCopy := *agent
	r.agents[agent.ID] = &agentCopy
	r.agentsByName[agent.DisplayName] = &agentCopy

	return nil
}

// FindByID finds an agent by ID.
func (r *InMemoryAgentRepository) FindByID(ctx context.Context, id string) (*models.Agent, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	agent, exists := r.agents[id]
	if !exists {
		return nil, errAgentNotFound
	}

	agentCopy := *agent
	return &agentCopy, nil
}

// FindByName finds an agent by display name.
func (r *InMemoryAgentRepository) FindByName(ctx context.Context, name string) (*models.Agent, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	agent, exists := r.agentsByName[name]
	if !exists {
		return nil, errAgentNotFound
	}

	agentCopy := *agent
	return &agentCopy, nil
}

// Update updates an existing agent.
func (r *InMemoryAgentRepository) Update(ctx context.Context, agent *models.Agent) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	existing, exists := r.agents[agent.ID]
	if !exists {
		return errAgentNotFound
	}

	// Update the agent
	agent.UpdatedAt = time.Now()
	agentCopy := *agent
	r.agents[agent.ID] = &agentCopy

	// Update name index if name changed
	if existing.DisplayName != agent.DisplayName {
		delete(r.agentsByName, existing.DisplayName)
		r.agentsByName[agent.DisplayName] = &agentCopy
	}

	return nil
}

// GetAgentStats returns stats for an agent.
func (r *InMemoryAgentRepository) GetAgentStats(ctx context.Context, agentID string) (*models.AgentStats, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if _, exists := r.agents[agentID]; !exists {
		return nil, errAgentNotFound
	}

	// Return empty stats for in-memory implementation
	return &models.AgentStats{}, nil
}

// UpdateAPIKeyHash updates the API key hash for an agent.
func (r *InMemoryAgentRepository) UpdateAPIKeyHash(ctx context.Context, agentID, hash string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	agent, exists := r.agents[agentID]
	if !exists {
		return errAgentNotFound
	}

	agent.APIKeyHash = hash
	agent.UpdatedAt = time.Now()
	return nil
}

// RevokeAPIKey revokes the API key for an agent.
func (r *InMemoryAgentRepository) RevokeAPIKey(ctx context.Context, agentID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	agent, exists := r.agents[agentID]
	if !exists {
		return errAgentNotFound
	}

	agent.APIKeyHash = ""
	agent.UpdatedAt = time.Now()
	return nil
}

// GetActivity returns activity for an agent.
func (r *InMemoryAgentRepository) GetActivity(ctx context.Context, agentID string, page, perPage int) ([]models.ActivityItem, int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if _, exists := r.agents[agentID]; !exists {
		return nil, 0, errAgentNotFound
	}

	// Return empty activity for in-memory implementation
	return []models.ActivityItem{}, 0, nil
}

// LinkHuman links an agent to a human.
func (r *InMemoryAgentRepository) LinkHuman(ctx context.Context, agentID, humanID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	agent, exists := r.agents[agentID]
	if !exists {
		return errAgentNotFound
	}

	agent.HumanID = &humanID
	now := time.Now()
	agent.HumanClaimedAt = &now
	agent.UpdatedAt = now
	return nil
}

// AddKarma adds karma to an agent.
func (r *InMemoryAgentRepository) AddKarma(ctx context.Context, agentID string, amount int) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	agent, exists := r.agents[agentID]
	if !exists {
		return errAgentNotFound
	}

	agent.Karma += amount
	agent.UpdatedAt = time.Now()
	return nil
}

// GrantHumanBackedBadge grants the human-backed badge to an agent.
func (r *InMemoryAgentRepository) GrantHumanBackedBadge(ctx context.Context, agentID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	agent, exists := r.agents[agentID]
	if !exists {
		return errAgentNotFound
	}

	agent.HasHumanBackedBadge = true
	agent.UpdatedAt = time.Now()
	return nil
}
