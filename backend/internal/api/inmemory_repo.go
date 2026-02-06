package api

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/fcavalcantirj/solvr/internal/auth"
	"github.com/fcavalcantirj/solvr/internal/db"
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

// GetAgentByAPIKeyHash finds an agent by checking the API key against stored hashes.
// This implements the auth.AgentDB interface for API key validation.
// Returns nil, nil if no matching agent is found.
func (r *InMemoryAgentRepository) GetAgentByAPIKeyHash(ctx context.Context, key string) (*models.Agent, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Iterate through all agents and compare the key against each stored hash
	for _, agent := range r.agents {
		if agent.APIKeyHash == "" {
			continue
		}
		// Use bcrypt comparison
		if err := auth.CompareAPIKey(key, agent.APIKeyHash); err == nil {
			agentCopy := *agent
			return &agentCopy, nil
		}
	}

	// No matching agent found
	return nil, nil
}

// List returns a paginated list of agents.
// Per API-001: GET /v1/agents - list registered agents.
func (r *InMemoryAgentRepository) List(ctx context.Context, opts models.AgentListOptions) ([]models.AgentWithPostCount, int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Collect all agents
	var agents []models.AgentWithPostCount
	for _, agent := range r.agents {
		// Filter by status
		if opts.Status != "" && opts.Status != "all" && agent.Status != opts.Status {
			continue
		}

		agents = append(agents, models.AgentWithPostCount{
			ID:                  agent.ID,
			DisplayName:         agent.DisplayName,
			Bio:                 agent.Bio,
			Status:              agent.Status,
			Karma:               agent.Karma,
			PostCount:           0, // In-memory doesn't track posts
			CreatedAt:           agent.CreatedAt,
			HasHumanBackedBadge: agent.HasHumanBackedBadge,
			AvatarURL:           agent.AvatarURL,
		})
	}

	total := len(agents)

	// Apply pagination
	page := opts.Page
	if page < 1 {
		page = 1
	}
	perPage := opts.PerPage
	if perPage < 1 {
		perPage = 20
	}
	if perPage > 100 {
		perPage = 100
	}

	start := (page - 1) * perPage
	if start >= len(agents) {
		return []models.AgentWithPostCount{}, total, nil
	}

	end := start + perPage
	if end > len(agents) {
		end = len(agents)
	}

	return agents[start:end], total, nil
}

// Error types for claim token operations
var (
	errClaimTokenNotFound = errors.New("claim token not found")
)

// InMemoryClaimTokenRepository is an in-memory implementation of ClaimTokenRepositoryInterface.
// Used for testing when no database is available.
type InMemoryClaimTokenRepository struct {
	mu     sync.RWMutex
	tokens map[string]*models.ClaimToken
}

// NewInMemoryClaimTokenRepository creates a new in-memory claim token repository.
func NewInMemoryClaimTokenRepository() *InMemoryClaimTokenRepository {
	return &InMemoryClaimTokenRepository{
		tokens: make(map[string]*models.ClaimToken),
	}
}

// Create creates a new claim token in memory.
func (r *InMemoryClaimTokenRepository) Create(ctx context.Context, token *models.ClaimToken) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Set ID if not set
	if token.ID == "" {
		token.ID = token.Token[:8] // Use first 8 chars of token as ID
	}
	if token.CreatedAt.IsZero() {
		token.CreatedAt = time.Now()
	}

	tokenCopy := *token
	r.tokens[token.Token] = &tokenCopy
	return nil
}

// FindByToken finds a claim token by token value.
func (r *InMemoryClaimTokenRepository) FindByToken(ctx context.Context, tokenValue string) (*models.ClaimToken, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	token, exists := r.tokens[tokenValue]
	if !exists {
		return nil, errClaimTokenNotFound
	}

	tokenCopy := *token
	return &tokenCopy, nil
}

// FindActiveByAgentID finds an active claim token for an agent.
func (r *InMemoryClaimTokenRepository) FindActiveByAgentID(ctx context.Context, agentID string) (*models.ClaimToken, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, token := range r.tokens {
		if token.AgentID == agentID && token.IsActive() {
			tokenCopy := *token
			return &tokenCopy, nil
		}
	}
	return nil, errClaimTokenNotFound
}

// MarkUsed marks a claim token as used.
func (r *InMemoryClaimTokenRepository) MarkUsed(ctx context.Context, tokenID, humanID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, token := range r.tokens {
		if token.ID == tokenID {
			now := time.Now()
			token.UsedAt = &now
			token.UsedByHumanID = &humanID
			return nil
		}
	}
	return errClaimTokenNotFound
}


// InMemoryPostRepository is an in-memory implementation of PostsRepositoryInterface.
// Used for testing when no database is available.
type InMemoryPostRepository struct {
	mu    sync.RWMutex
	posts map[string]*models.Post
}

// NewInMemoryPostRepository creates a new in-memory post repository.
func NewInMemoryPostRepository() *InMemoryPostRepository {
	return &InMemoryPostRepository{
		posts: make(map[string]*models.Post),
	}
}

// List returns posts matching the given options.
func (r *InMemoryPostRepository) List(ctx context.Context, opts models.PostListOptions) ([]models.PostWithAuthor, int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var results []models.PostWithAuthor
	for _, post := range r.posts {
		if post.DeletedAt != nil {
			continue
		}

		// Apply filters
		if opts.Type != "" && post.Type != opts.Type {
			continue
		}
		if opts.Status != "" && post.Status != opts.Status {
			continue
		}

		results = append(results, models.PostWithAuthor{
			Post: *post,
			Author: models.PostAuthor{
				Type: post.PostedByType,
				ID:   post.PostedByID,
			},
			VoteScore: post.Upvotes - post.Downvotes,
		})
	}

	total := len(results)

	// Apply pagination
	start := (opts.Page - 1) * opts.PerPage
	if start > len(results) {
		return []models.PostWithAuthor{}, total, nil
	}
	end := start + opts.PerPage
	if end > len(results) {
		end = len(results)
	}

	return results[start:end], total, nil
}

// FindByID returns a single post by ID.
func (r *InMemoryPostRepository) FindByID(ctx context.Context, id string) (*models.PostWithAuthor, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	post, exists := r.posts[id]
	if !exists || post.DeletedAt != nil {
		return nil, db.ErrPostNotFound
	}

	return &models.PostWithAuthor{
		Post: *post,
		Author: models.PostAuthor{
			Type: post.PostedByType,
			ID:   post.PostedByID,
		},
		VoteScore: post.Upvotes - post.Downvotes,
	}, nil
}

// Create creates a new post and returns it.
func (r *InMemoryPostRepository) Create(ctx context.Context, post *models.Post) (*models.Post, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Generate ID if not set
	if post.ID == "" {
		post.ID = "post_" + time.Now().Format("20060102150405")
	}

	now := time.Now()
	if post.CreatedAt.IsZero() {
		post.CreatedAt = now
	}
	post.UpdatedAt = now

	postCopy := *post
	r.posts[post.ID] = &postCopy
	return &postCopy, nil
}

// Update updates an existing post and returns it.
func (r *InMemoryPostRepository) Update(ctx context.Context, post *models.Post) (*models.Post, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	existing, exists := r.posts[post.ID]
	if !exists || existing.DeletedAt != nil {
		return nil, db.ErrPostNotFound
	}

	post.UpdatedAt = time.Now()
	postCopy := *post
	r.posts[post.ID] = &postCopy
	return &postCopy, nil
}

// Delete soft-deletes a post by ID.
func (r *InMemoryPostRepository) Delete(ctx context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	post, exists := r.posts[id]
	if !exists || post.DeletedAt != nil {
		return db.ErrPostNotFound
	}

	now := time.Now()
	post.DeletedAt = &now
	return nil
}

// Vote records a vote on a post.
func (r *InMemoryPostRepository) Vote(ctx context.Context, postID, voterType, voterID, direction string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	post, exists := r.posts[postID]
	if !exists || post.DeletedAt != nil {
		return db.ErrPostNotFound
	}

	// For simplicity, just update vote counts
	if direction == "up" {
		post.Upvotes++
	} else if direction == "down" {
		post.Downvotes++
	}

	return nil
}

// InMemorySearchRepository is an in-memory implementation of SearchRepositoryInterface.
// Used for testing when no database is available.
type InMemorySearchRepository struct {
	mu    sync.RWMutex
	posts map[string]*models.Post
}

// NewInMemorySearchRepository creates a new in-memory search repository.
func NewInMemorySearchRepository() *InMemorySearchRepository {
	return &InMemorySearchRepository{
		posts: make(map[string]*models.Post),
	}
}

// Search performs a full-text search in memory.
// Returns empty results for testing - in production use the DB-backed version.
func (r *InMemorySearchRepository) Search(ctx context.Context, query string, opts models.SearchOptions) ([]models.SearchResult, int, error) {
	// Return empty results for in-memory testing
	// Real search requires PostgreSQL full-text search
	return []models.SearchResult{}, 0, nil
}

// InMemoryFeedRepository is an in-memory implementation of FeedRepositoryInterface.
// Used for testing when no database is available.
type InMemoryFeedRepository struct {
	mu    sync.RWMutex
	posts map[string]*models.Post
}

// NewInMemoryFeedRepository creates a new in-memory feed repository.
func NewInMemoryFeedRepository() *InMemoryFeedRepository {
	return &InMemoryFeedRepository{
		posts: make(map[string]*models.Post),
	}
}

// GetRecentActivity returns recent posts.
// Returns empty results for in-memory testing.
func (r *InMemoryFeedRepository) GetRecentActivity(ctx context.Context, page, perPage int) ([]models.FeedItem, int, error) {
	// Return empty results for in-memory testing
	return []models.FeedItem{}, 0, nil
}

// GetStuckProblems returns problems that need help.
// Returns empty results for in-memory testing.
func (r *InMemoryFeedRepository) GetStuckProblems(ctx context.Context, page, perPage int) ([]models.FeedItem, int, error) {
	// Return empty results for in-memory testing
	return []models.FeedItem{}, 0, nil
}

// GetUnansweredQuestions returns questions with zero answers.
// Returns empty results for in-memory testing.
func (r *InMemoryFeedRepository) GetUnansweredQuestions(ctx context.Context, page, perPage int) ([]models.FeedItem, int, error) {
	// Return empty results for in-memory testing
	return []models.FeedItem{}, 0, nil
}

// InMemoryUserRepository is an in-memory implementation of MeUserRepositoryInterface.
// Used for testing when no database is available.
type InMemoryUserRepository struct {
	mu    sync.RWMutex
	users map[string]*models.User
	stats map[string]*models.UserStats
}

// NewInMemoryUserRepository creates a new in-memory user repository.
func NewInMemoryUserRepository() *InMemoryUserRepository {
	return &InMemoryUserRepository{
		users: make(map[string]*models.User),
		stats: make(map[string]*models.UserStats),
	}
}

// FindByID finds a user by their ID.
// Returns nil, nil if not found (matching DB behavior for MeHandler).
func (r *InMemoryUserRepository) FindByID(ctx context.Context, id string) (*models.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	user, exists := r.users[id]
	if !exists {
		return nil, nil
	}

	userCopy := *user
	return &userCopy, nil
}

// GetUserStats returns statistics for a user.
// Returns empty stats if not found.
func (r *InMemoryUserRepository) GetUserStats(ctx context.Context, userID string) (*models.UserStats, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	stats, exists := r.stats[userID]
	if !exists {
		return &models.UserStats{}, nil
	}

	statsCopy := *stats
	return &statsCopy, nil
}
