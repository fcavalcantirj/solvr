package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/fcavalcantirj/solvr/internal/auth"
	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
)


// Error types for agent operations
var (
	ErrDuplicateAgentID   = errors.New("agent ID already exists")
	ErrDuplicateAgentName = errors.New("agent name already exists")
	ErrAgentNotFound      = errors.New("agent not found")
)

// AgentRepositoryInterface defines the database operations for agents.
// This interface also satisfies auth.AgentDB for API key validation.
type AgentRepositoryInterface interface {
	Create(ctx context.Context, agent *models.Agent) error
	FindByID(ctx context.Context, id string) (*models.Agent, error)
	FindByName(ctx context.Context, name string) (*models.Agent, error)
	Update(ctx context.Context, agent *models.Agent) error
	GetAgentStats(ctx context.Context, agentID string) (*models.AgentStats, error)
	UpdateAPIKeyHash(ctx context.Context, agentID, hash string) error
	RevokeAPIKey(ctx context.Context, agentID string) error
	GetActivity(ctx context.Context, agentID string, page, perPage int) ([]models.ActivityItem, int, error)
	// Agent-Human Linking methods (AGENT-LINKING requirement)
	LinkHuman(ctx context.Context, agentID, humanID string) error
	AddKarma(ctx context.Context, agentID string, amount int) error
	GrantHumanBackedBadge(ctx context.Context, agentID string) error
	// API key validation method (implements auth.AgentDB interface)
	// FIX-002: Required for API key authentication middleware
	GetAgentByAPIKeyHash(ctx context.Context, key string) (*models.Agent, error)
}

// ClaimTokenRepositoryInterface defines database operations for claim tokens.
// Per AGENT-LINKING requirement: agents generate claim tokens for human linking.
type ClaimTokenRepositoryInterface interface {
	Create(ctx context.Context, token *models.ClaimToken) error
	FindByToken(ctx context.Context, token string) (*models.ClaimToken, error)
	FindActiveByAgentID(ctx context.Context, agentID string) (*models.ClaimToken, error)
	MarkUsed(ctx context.Context, tokenID, humanID string) error
}

// AgentsHandler handles agent-related HTTP requests.
type AgentsHandler struct {
	repo           AgentRepositoryInterface
	claimTokenRepo ClaimTokenRepositoryInterface
	jwtSecret      string
	baseURL        string // Base URL for claim URLs (e.g., "https://solvr.dev")
}

// NewAgentsHandler creates a new AgentsHandler.
func NewAgentsHandler(repo AgentRepositoryInterface, jwtSecret string) *AgentsHandler {
	return &AgentsHandler{
		repo:      repo,
		jwtSecret: jwtSecret,
		baseURL:   "https://solvr.dev", // Default base URL
	}
}

// SetClaimTokenRepository sets the claim token repository for agent-human linking.
func (h *AgentsHandler) SetClaimTokenRepository(repo ClaimTokenRepositoryInterface) {
	h.claimTokenRepo = repo
}

// SetBaseURL sets the base URL for generating claim URLs.
func (h *AgentsHandler) SetBaseURL(url string) {
	h.baseURL = url
}

// CreateAgentRequest is the request body for creating an agent.
type CreateAgentRequest struct {
	ID          string   `json:"id"`
	DisplayName string   `json:"display_name"`
	Bio         string   `json:"bio,omitempty"`
	Specialties []string `json:"specialties,omitempty"`
	AvatarURL   string   `json:"avatar_url,omitempty"`
}

// CreateAgentResponse is the response for creating an agent.
type CreateAgentResponse struct {
	Data struct {
		Agent  models.Agent `json:"agent"`
		APIKey string       `json:"api_key"`
	} `json:"data"`
	Meta map[string]interface{} `json:"meta,omitempty"`
}

// GetAgentResponse is the response for getting an agent.
type GetAgentResponse struct {
	Data struct {
		Agent models.Agent      `json:"agent"`
		Stats models.AgentStats `json:"stats"`
	} `json:"data"`
	Meta map[string]interface{} `json:"meta,omitempty"`
}

// UpdateAgentRequest is the request body for updating an agent.
type UpdateAgentRequest struct {
	DisplayName *string  `json:"display_name,omitempty"`
	Bio         *string  `json:"bio,omitempty"`
	Specialties []string `json:"specialties,omitempty"`
	AvatarURL   *string  `json:"avatar_url,omitempty"`
}

// RegisterAgentRequest is the request body for agent self-registration.
// Per AGENT-ONBOARDING requirement: agents can self-register without human auth.
type RegisterAgentRequest struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// RegisterAgentResponse is the response for agent self-registration.
type RegisterAgentResponse struct {
	Success   bool         `json:"success"`
	Agent     models.Agent `json:"agent"`
	APIKey    string       `json:"api_key"`
	Important string       `json:"important,omitempty"`
}

// validAgentID matches alphanumeric characters and underscores only, max 50 chars.
var validAgentID = regexp.MustCompile(`^[a-zA-Z0-9_]+$`)

// validAgentName matches alphanumeric characters and underscores only, 3-30 chars.
var validAgentName = regexp.MustCompile(`^[a-zA-Z0-9_]+$`)

// validateAgentID validates the agent ID format per SPEC.md Part 2.7.
func validateAgentID(id string) error {
	if id == "" {
		return errors.New("id is required")
	}
	if len(id) > 50 {
		return errors.New("id must not exceed 50 characters")
	}
	if !validAgentID.MatchString(id) {
		return errors.New("id must contain only alphanumeric characters and underscores")
	}
	return nil
}

// validateAgentName validates the agent name for self-registration.
// Per AGENT-ONBOARDING requirement: Name must be 3-30 chars, alphanumeric + underscore.
func validateAgentName(name string) error {
	if name == "" {
		return errors.New("name is required")
	}
	if len(name) < 3 {
		return errors.New("name must be at least 3 characters")
	}
	if len(name) > 30 {
		return errors.New("name must not exceed 30 characters")
	}
	if !validAgentName.MatchString(name) {
		return errors.New("name must contain only alphanumeric characters and underscores")
	}
	return nil
}

// generateAgentID creates a unique agent ID from the name.
func generateAgentID(name string) string {
	return "agent_" + name
}

// RegisterAgent handles POST /v1/agents/register - agent self-registration.
// Per AGENT-ONBOARDING: agents can self-register without human auth.
func (h *AgentsHandler) RegisterAgent(w http.ResponseWriter, r *http.Request) {
	// Parse request body
	var req RegisterAgentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAgentValidationError(w, "invalid JSON body")
		return
	}

	// Validate name format
	if err := validateAgentName(req.Name); err != nil {
		writeAgentValidationError(w, err.Error())
		return
	}

	// Validate description length
	if len(req.Description) > 500 {
		writeAgentValidationError(w, "description must not exceed 500 characters")
		return
	}

	// Generate unique agent ID from name
	agentID := generateAgentID(req.Name)

	// Generate API key
	apiKey := auth.GenerateAPIKey()
	apiKeyHash, err := auth.HashAPIKey(apiKey)
	if err != nil {
		writeAgentError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to generate API key")
		return
	}

	// Create agent (no human_id for self-registered agents)
	now := time.Now()
	agent := &models.Agent{
		ID:          agentID,
		DisplayName: req.Name,
		HumanID:     nil, // Self-registered, no human owner
		Bio:         req.Description,
		Specialties: []string{},
		AvatarURL:   "",
		APIKeyHash:  apiKeyHash,
		Status:      "active", // Active immediately per requirement
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := h.repo.Create(r.Context(), agent); err != nil {
		// FIX-027: Check for both local errors (mock) and db errors (real DB)
		if errors.Is(err, ErrDuplicateAgentID) || errors.Is(err, ErrDuplicateAgentName) ||
			errors.Is(err, db.ErrDuplicateAgentID) {
			// Generate suggestions by checking existence against repository
			checkExists := func(name string) bool {
				_, findErr := h.repo.FindByName(r.Context(), name)
				return findErr == nil // Name exists if no error
			}
			writeDuplicateNameError(w, req.Name, checkExists)
			return
		}
		writeAgentError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to create agent")
		return
	}

	// Return response with API key (shown only once per requirement)
	resp := RegisterAgentResponse{
		Success:   true,
		Agent:     *agent,
		APIKey:    apiKey,
		Important: "⚠️ SAVE YOUR API KEY! Shown only once.",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resp)
}

// CreateAgent handles POST /v1/agents - create a new agent.
// Requires human JWT authentication per SPEC.md Part 5.6.
func (h *AgentsHandler) CreateAgent(w http.ResponseWriter, r *http.Request) {
	// Require JWT authentication
	claims := auth.ClaimsFromContext(r.Context())
	if claims == nil {
		writeAgentUnauthorized(w, "authentication required")
		return
	}

	// Parse request body
	var req CreateAgentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAgentValidationError(w, "invalid JSON body")
		return
	}

	// Validate ID format
	if err := validateAgentID(req.ID); err != nil {
		writeAgentError(w, http.StatusBadRequest, "INVALID_ID", err.Error())
		return
	}

	// Validate display name
	if req.DisplayName == "" {
		writeAgentValidationError(w, "display_name is required")
		return
	}
	if len(req.DisplayName) > 50 {
		writeAgentValidationError(w, "display_name must not exceed 50 characters")
		return
	}

	// Validate bio length
	if len(req.Bio) > 500 {
		writeAgentValidationError(w, "bio must not exceed 500 characters")
		return
	}

	// Validate specialties count
	if len(req.Specialties) > 10 {
		writeAgentValidationError(w, "specialties must not exceed 10 items")
		return
	}

	// Generate API key
	apiKey := auth.GenerateAPIKey()
	apiKeyHash, err := auth.HashAPIKey(apiKey)
	if err != nil {
		writeAgentError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to generate API key")
		return
	}

	// Create agent
	humanID := claims.UserID
	now := time.Now()
	agent := &models.Agent{
		ID:          req.ID,
		DisplayName: req.DisplayName,
		HumanID:     &humanID,
		Bio:         req.Bio,
		Specialties: req.Specialties,
		AvatarURL:   req.AvatarURL,
		APIKeyHash:  apiKeyHash,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := h.repo.Create(r.Context(), agent); err != nil {
		// FIX-027: Check for both local ErrDuplicateAgentID (mock) and db.ErrDuplicateAgentID (real DB)
		if errors.Is(err, ErrDuplicateAgentID) || errors.Is(err, db.ErrDuplicateAgentID) {
			writeAgentError(w, http.StatusConflict, "DUPLICATE_ID", "agent ID already exists")
			return
		}
		writeAgentError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to create agent")
		return
	}

	// Return response with API key (only shown once)
	resp := CreateAgentResponse{}
	resp.Data.Agent = *agent
	resp.Data.APIKey = apiKey

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resp)
}

// GetAgent handles GET /v1/agents/:id - get agent profile with stats.
func (h *AgentsHandler) GetAgent(w http.ResponseWriter, r *http.Request, agentID string) {
	agent, err := h.repo.FindByID(r.Context(), agentID)
	if err != nil {
		// FIX-026: Check for both local ErrAgentNotFound (mock) and db.ErrAgentNotFound (real DB)
		if errors.Is(err, ErrAgentNotFound) || errors.Is(err, db.ErrAgentNotFound) {
			writeAgentError(w, http.StatusNotFound, "NOT_FOUND", "agent not found")
			return
		}
		writeAgentError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get agent")
		return
	}

	stats, err := h.repo.GetAgentStats(r.Context(), agentID)
	if err != nil {
		// Stats are optional, use empty stats on error
		stats = &models.AgentStats{}
	}

	resp := GetAgentResponse{}
	resp.Data.Agent = *agent
	resp.Data.Stats = *stats

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

// UpdateAgent handles PATCH /v1/agents/:id - update agent profile.
// Requires authentication and ownership verification per SPEC.md Part 5.6.
func (h *AgentsHandler) UpdateAgent(w http.ResponseWriter, r *http.Request, agentID string) {
	// Require JWT authentication
	claims := auth.ClaimsFromContext(r.Context())
	if claims == nil {
		writeAgentUnauthorized(w, "authentication required")
		return
	}

	// Get existing agent
	agent, err := h.repo.FindByID(r.Context(), agentID)
	if err != nil {
		// FIX-026: Check for both local ErrAgentNotFound (mock) and db.ErrAgentNotFound (real DB)
		if errors.Is(err, ErrAgentNotFound) || errors.Is(err, db.ErrAgentNotFound) {
			writeAgentError(w, http.StatusNotFound, "NOT_FOUND", "agent not found")
			return
		}
		writeAgentError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get agent")
		return
	}

	// Verify ownership (human_id must match JWT user ID)
	if agent.HumanID == nil || *agent.HumanID != claims.UserID {
		writeAgentError(w, http.StatusForbidden, "FORBIDDEN", "you do not own this agent")
		return
	}

	// Parse request body
	var req UpdateAgentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAgentValidationError(w, "invalid JSON body")
		return
	}

	// Update allowed fields
	if req.DisplayName != nil {
		if len(*req.DisplayName) > 50 {
			writeAgentValidationError(w, "display_name must not exceed 50 characters")
			return
		}
		agent.DisplayName = *req.DisplayName
	}
	if req.Bio != nil {
		if len(*req.Bio) > 500 {
			writeAgentValidationError(w, "bio must not exceed 500 characters")
			return
		}
		agent.Bio = *req.Bio
	}
	if req.Specialties != nil {
		if len(req.Specialties) > 10 {
			writeAgentValidationError(w, "specialties must not exceed 10 items")
			return
		}
		agent.Specialties = req.Specialties
	}
	if req.AvatarURL != nil {
		agent.AvatarURL = *req.AvatarURL
	}

	agent.UpdatedAt = time.Now()

	if err := h.repo.Update(r.Context(), agent); err != nil {
		writeAgentError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to update agent")
		return
	}

	// Get stats for response
	stats, err := h.repo.GetAgentStats(r.Context(), agentID)
	if err != nil {
		stats = &models.AgentStats{}
	}

	resp := GetAgentResponse{}
	resp.Data.Agent = *agent
	resp.Data.Stats = *stats

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

// RegenerateAPIKey handles POST /v1/agents/:id/api-key - regenerate API key.
// Requires authentication and ownership verification per SPEC.md Part 5.6.
func (h *AgentsHandler) RegenerateAPIKey(w http.ResponseWriter, r *http.Request, agentID string) {
	// Require JWT authentication
	claims := auth.ClaimsFromContext(r.Context())
	if claims == nil {
		writeAgentUnauthorized(w, "authentication required")
		return
	}

	// Get existing agent
	agent, err := h.repo.FindByID(r.Context(), agentID)
	if err != nil {
		// FIX-026: Check for both local ErrAgentNotFound (mock) and db.ErrAgentNotFound (real DB)
		if errors.Is(err, ErrAgentNotFound) || errors.Is(err, db.ErrAgentNotFound) {
			writeAgentError(w, http.StatusNotFound, "NOT_FOUND", "agent not found")
			return
		}
		writeAgentError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get agent")
		return
	}

	// Verify ownership
	if agent.HumanID == nil || *agent.HumanID != claims.UserID {
		writeAgentError(w, http.StatusForbidden, "FORBIDDEN", "you do not own this agent")
		return
	}

	// Generate new API key
	apiKey := auth.GenerateAPIKey()
	apiKeyHash, err := auth.HashAPIKey(apiKey)
	if err != nil {
		writeAgentError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to generate API key")
		return
	}

	// Update hash in database (invalidates old key)
	if err := h.repo.UpdateAPIKeyHash(r.Context(), agentID, apiKeyHash); err != nil {
		writeAgentError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to update API key")
		return
	}

	// Return new API key (only shown once)
	resp := map[string]interface{}{
		"data": map[string]interface{}{
			"api_key": apiKey,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

// RevokeAPIKey handles DELETE /v1/agents/:id/api-key - revoke API key.
// Requires authentication and ownership verification per SPEC.md Part 5.6.
func (h *AgentsHandler) RevokeAPIKey(w http.ResponseWriter, r *http.Request, agentID string) {
	// Require JWT authentication
	claims := auth.ClaimsFromContext(r.Context())
	if claims == nil {
		writeAgentUnauthorized(w, "authentication required")
		return
	}

	// Get existing agent
	agent, err := h.repo.FindByID(r.Context(), agentID)
	if err != nil {
		// FIX-026: Check for both local ErrAgentNotFound (mock) and db.ErrAgentNotFound (real DB)
		if errors.Is(err, ErrAgentNotFound) || errors.Is(err, db.ErrAgentNotFound) {
			writeAgentError(w, http.StatusNotFound, "NOT_FOUND", "agent not found")
			return
		}
		writeAgentError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get agent")
		return
	}

	// Verify ownership
	if agent.HumanID == nil || *agent.HumanID != claims.UserID {
		writeAgentError(w, http.StatusForbidden, "FORBIDDEN", "you do not own this agent")
		return
	}

	// Revoke API key (set hash to NULL)
	if err := h.repo.RevokeAPIKey(r.Context(), agentID); err != nil {
		writeAgentError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to revoke API key")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ActivityResponse is the response structure for the activity endpoint.
type ActivityResponse struct {
	Data []models.ActivityItem `json:"data"`
	Meta struct {
		Total   int  `json:"total"`
		Page    int  `json:"page"`
		PerPage int  `json:"per_page"`
		HasMore bool `json:"has_more"`
	} `json:"meta"`
}

// GetActivity handles GET /v1/agents/:id/activity - activity history.
// Per SPEC.md Part 4.9 and Part 5.6.
func (h *AgentsHandler) GetActivity(w http.ResponseWriter, r *http.Request, agentID string) {
	// Parse pagination parameters with defaults per SPEC.md Part 5.6
	page := 1
	perPage := 20
	maxPerPage := 50

	if p := r.URL.Query().Get("page"); p != "" {
		if parsed, err := strconv.Atoi(p); err == nil && parsed > 0 {
			page = parsed
		}
	}
	if pp := r.URL.Query().Get("per_page"); pp != "" {
		if parsed, err := strconv.Atoi(pp); err == nil && parsed > 0 {
			perPage = parsed
		}
	}

	// Cap per_page at max per SPEC.md Part 5.6
	if perPage > maxPerPage {
		perPage = maxPerPage
	}

	// Get activity from repository
	activities, total, err := h.repo.GetActivity(r.Context(), agentID, page, perPage)
	if err != nil {
		// FIX-026: Check for both local ErrAgentNotFound (mock) and db.ErrAgentNotFound (real DB)
		if errors.Is(err, ErrAgentNotFound) || errors.Is(err, db.ErrAgentNotFound) {
			writeAgentError(w, http.StatusNotFound, "NOT_FOUND", "agent not found")
			return
		}
		writeAgentError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get activity")
		return
	}

	// Calculate has_more
	hasMore := page*perPage < total

	// Build response
	resp := ActivityResponse{}
	resp.Data = activities
	if resp.Data == nil {
		resp.Data = []models.ActivityItem{} // Ensure empty array, not null
	}
	resp.Meta.Total = total
	resp.Meta.Page = page
	resp.Meta.PerPage = perPage
	resp.Meta.HasMore = hasMore

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

// writeAgentError writes an error response.
func writeAgentError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]interface{}{
			"code":    code,
			"message": message,
		},
	})
}

// writeAgentUnauthorized writes a 401 Unauthorized error.
func writeAgentUnauthorized(w http.ResponseWriter, message string) {
	writeAgentError(w, http.StatusUnauthorized, "UNAUTHORIZED", message)
}

// writeAgentValidationError writes a 400 Validation Error.
func writeAgentValidationError(w http.ResponseWriter, message string) {
	writeAgentError(w, http.StatusBadRequest, "VALIDATION_ERROR", message)
}

// generateNameSuggestions generates alternative name suggestions for a duplicate name.
// Per AGENT-ONBOARDING requirement: Suggest alternatives in error response.
func generateNameSuggestions(baseName string, checkExists func(string) bool) []string {
	suggestions := []string{}
	maxSuggestions := 5

	// Strategy 1: Add numeric suffix
	for i := 1; len(suggestions) < maxSuggestions && i <= 100; i++ {
		candidate := baseName + "_" + strconv.Itoa(i)
		// Ensure candidate doesn't exceed 30 char limit
		if len(candidate) <= 30 && !checkExists(candidate) {
			suggestions = append(suggestions, candidate)
		}
	}

	// Strategy 2: Add common suffixes if we still need more suggestions
	suffixes := []string{"_bot", "_ai", "_helper", "_v2", "_new"}
	for _, suffix := range suffixes {
		if len(suggestions) >= maxSuggestions {
			break
		}
		candidate := baseName + suffix
		// Ensure candidate doesn't exceed 30 char limit and suffix isn't already in name
		if len(candidate) <= 30 && !checkExists(candidate) && !strings.HasSuffix(baseName, suffix) {
			suggestions = append(suggestions, candidate)
		}
	}

	// Strategy 3: Truncate name if needed and add suffix
	if len(suggestions) < 3 && len(baseName) > 20 {
		truncated := baseName[:20]
		for i := 1; len(suggestions) < maxSuggestions && i <= 10; i++ {
			candidate := truncated + "_" + strconv.Itoa(i)
			if !checkExists(candidate) {
				suggestions = append(suggestions, candidate)
			}
		}
	}

	return suggestions
}

// writeDuplicateNameError writes a 409 Conflict error with name suggestions.
// Per AGENT-ONBOARDING requirement: Return 409 Conflict if name taken with alternatives.
func writeDuplicateNameError(w http.ResponseWriter, baseName string, checkExists func(string) bool) {
	suggestions := generateNameSuggestions(baseName, checkExists)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusConflict)

	response := map[string]interface{}{
		"error": map[string]interface{}{
			"code":        "DUPLICATE_NAME",
			"message":     "agent name already exists",
			"suggestions": suggestions,
		},
	}

	json.NewEncoder(w).Encode(response)
}
