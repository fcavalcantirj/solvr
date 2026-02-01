package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"time"

	"github.com/fcavalcantirj/solvr/internal/auth"
	"github.com/fcavalcantirj/solvr/internal/models"
)

// Error types for agent operations
var (
	ErrDuplicateAgentID = errors.New("agent ID already exists")
	ErrAgentNotFound    = errors.New("agent not found")
)

// AgentRepositoryInterface defines the database operations for agents.
type AgentRepositoryInterface interface {
	Create(ctx context.Context, agent *models.Agent) error
	FindByID(ctx context.Context, id string) (*models.Agent, error)
	Update(ctx context.Context, agent *models.Agent) error
	GetAgentStats(ctx context.Context, agentID string) (*models.AgentStats, error)
	UpdateAPIKeyHash(ctx context.Context, agentID, hash string) error
	RevokeAPIKey(ctx context.Context, agentID string) error
}

// AgentsHandler handles agent-related HTTP requests.
type AgentsHandler struct {
	repo      AgentRepositoryInterface
	jwtSecret string
}

// NewAgentsHandler creates a new AgentsHandler.
func NewAgentsHandler(repo AgentRepositoryInterface, jwtSecret string) *AgentsHandler {
	return &AgentsHandler{
		repo:      repo,
		jwtSecret: jwtSecret,
	}
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

// validAgentID matches alphanumeric characters and underscores only, max 50 chars.
var validAgentID = regexp.MustCompile(`^[a-zA-Z0-9_]+$`)

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
		if errors.Is(err, ErrDuplicateAgentID) {
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
		if errors.Is(err, ErrAgentNotFound) {
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
		if errors.Is(err, ErrAgentNotFound) {
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
		if errors.Is(err, ErrAgentNotFound) {
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
		if errors.Is(err, ErrAgentNotFound) {
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
