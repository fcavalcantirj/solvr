package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/fcavalcantirj/solvr/internal/auth"
	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
)

// Error types for agent operations
var (
	ErrDuplicateAgentID   = errors.New("agent ID already exists")
	ErrDuplicateAgentName = errors.New("agent name already exists")
	ErrAgentNotFound      = db.ErrAgentNotFound // one sentinel, so a repository miss is a 404 everywhere
	ErrDuplicateAMCPAID   = errors.New("amcp_aid already in use by another agent")
)

// AgentRepositoryInterface defines the database operations for agents.
// This interface also satisfies auth.AgentDB for API key validation.
type AgentRepositoryInterface interface {
	Create(ctx context.Context, agent *models.Agent) error
	FindByID(ctx context.Context, id string) (*models.Agent, error)
	FindByName(ctx context.Context, name string) (*models.Agent, error)
	FindByHumanID(ctx context.Context, humanID string) ([]*models.Agent, error)
	Update(ctx context.Context, agent *models.Agent) error
	GetAgentStats(ctx context.Context, agentID string) (*models.AgentStats, error)
	UpdateAPIKeyHash(ctx context.Context, agentID, hash, keySHA256 string) error
	RevokeAPIKey(ctx context.Context, agentID string) error
	GetActivity(ctx context.Context, agentID string, page, perPage int) ([]models.ActivityItem, int, error)
	// Agent-Human Linking methods (AGENT-LINKING requirement)
	LinkHuman(ctx context.Context, agentID, humanID string) error
	AddReputation(ctx context.Context, agentID string, amount int) error
	GrantReputationOnce(ctx context.Context, agentID, grantKey string, points int) (bool, error)
	GrantHumanBackedBadge(ctx context.Context, agentID string) error
	// API key validation method (implements auth.AgentDB interface)
	// FIX-002: Required for API key authentication middleware
	GetAgentByAPIKeyHash(ctx context.Context, key string) (*models.Agent, error)
	// List agents (API-001 requirement)
	List(ctx context.Context, opts models.AgentListOptions) ([]models.AgentWithPostCount, int, error)
	// Count stats for agents listing
	CountActive(ctx context.Context) (int, error)
	CountHumanBacked(ctx context.Context) (int, error)
	// Soft delete agent (PRD-v5 Task 22)
	Delete(ctx context.Context, id string) error
	// Heartbeat liveness tracking
	UpdateLastSeen(ctx context.Context, id string) error
	// KERI identity management
	UpdateIdentity(ctx context.Context, agentID string, amcpAID *string, keriPublicKey *string) (*models.Agent, error)
}

// ClaimTokenRepositoryInterface defines database operations for claim tokens.
// Per AGENT-LINKING requirement: agents generate claim tokens for human linking.
type ClaimTokenRepositoryInterface interface {
	Create(ctx context.Context, token *models.ClaimToken) error
	FindByToken(ctx context.Context, token string) (*models.ClaimToken, error)
	FindActiveByAgentID(ctx context.Context, agentID string) (*models.ClaimToken, error)
	MarkUsed(ctx context.Context, tokenID, humanID string) error
	DeleteExpiredByAgentID(ctx context.Context, agentID string) (int64, error)
	DeleteUnusedByAgentID(ctx context.Context, agentID string) (int64, error)
}

// RoomOwnerBackfiller backfills room ownership when an agent is claimed by a human.
// Rooms an agent created while unclaimed have no human owner membership; after the claim
// links a human, that human should own those rooms so family-scoped access starts working.
type RoomOwnerBackfiller interface {
	BackfillOwnerFromMembership(ctx context.Context, agentID, humanID string) (int64, error)
}

// AgentsHandler handles agent-related HTTP requests.
type AgentsHandler struct {
	repo           AgentRepositoryInterface
	claimTokenRepo ClaimTokenRepositoryInterface
	roomBackfiller RoomOwnerBackfiller
	jwtSecret      string
	baseURL        string // Base URL for claim URLs (e.g., "https://solvr.dev")
	identityGate   IdentityRefuser
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

// SetRoomRepository sets the repository used to backfill room ownership on claim.
func (h *AgentsHandler) SetRoomRepository(repo RoomOwnerBackfiller) {
	h.roomBackfiller = repo
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
	DisplayName   *string   `json:"display_name,omitempty"`
	Bio           *string   `json:"bio,omitempty"`
	Specialties   []string  `json:"specialties,omitempty"`
	AvatarURL     *string   `json:"avatar_url,omitempty"`
	Model         *string   `json:"model,omitempty"`
	Email         *string   `json:"email,omitempty"`
	ExternalLinks *[]string `json:"external_links,omitempty"`
}

// AMCPDefaultQuotaBytes is the default pinning quota for AMCP-enabled agents (1 GB).
const AMCPDefaultQuotaBytes = int64(1073741824)

// UpdateIdentityRequest is the request body for PATCH /v1/agents/me/identity.
// Per prd-v5: Agent-only endpoint for updating AMCP identity fields.
type UpdateIdentityRequest struct {
	AMCPAID       *string `json:"amcp_aid,omitempty"`
	KERIPublicKey *string `json:"keri_public_key,omitempty"`
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
		KeySHA256:   auth.SHA256APIKey(apiKey),
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

	resp := h.agentResponse(r.Context(), agent)
	// A public answer: the contact e-mail is the agent's and its owner's (SPEC.md 2.7).
	resp.Data.Agent = resp.Data.Agent.Public()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

// UpdateAgent handles PATCH /v1/agents/:id - update agent profile.
// Requires authentication and ownership verification per SPEC.md Part 5.6.
func (h *AgentsHandler) UpdateAgent(w http.ResponseWriter, r *http.Request, agentID string) {
	// Per prd-v4: Support both JWT (human owner) and API key (agent updating itself) auth
	claims := auth.ClaimsFromContext(r.Context())
	agentFromCtx := auth.AgentFromContext(r.Context())

	if claims == nil && agentFromCtx == nil {
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

	// Verify ownership: human_id matches JWT user ID OR agent is updating itself via API key
	isOwner := false
	if claims != nil && agent.HumanID != nil && *agent.HumanID == claims.UserID {
		isOwner = true
	}
	if agentFromCtx != nil && agentFromCtx.ID == agentID {
		isOwner = true
	}
	if !isOwner {
		writeAgentError(w, http.StatusForbidden, "FORBIDDEN", "you do not own this agent")
		return
	}

	// Parse request body
	var req UpdateAgentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAgentValidationError(w, "invalid JSON body")
		return
	}

	// Track if model was previously empty (for reputation bonus)
	previousModelEmpty := agent.Model == ""

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
	if req.Model != nil {
		if len(*req.Model) > 100 {
			writeAgentValidationError(w, "model must not exceed 100 characters")
			return
		}
		agent.Model = *req.Model
	}
	if req.Email != nil {
		if len(*req.Email) > 255 {
			writeAgentValidationError(w, "email must not exceed 255 characters")
			return
		}
		agent.Email = *req.Email
	}
	if req.ExternalLinks != nil {
		if len(*req.ExternalLinks) > 10 {
			writeAgentValidationError(w, "external_links must not exceed 10 items")
			return
		}
		for _, link := range *req.ExternalLinks {
			if len(link) > 500 {
				writeAgentValidationError(w, "each external link must not exceed 500 characters")
				return
			}
		}
		agent.ExternalLinks = *req.ExternalLinks
	}

	agent.UpdatedAt = time.Now()

	if err := h.repo.Update(r.Context(), agent); err != nil {
		writeAgentError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to update agent")
		return
	}

	// Per prd-v4: +10 reputation bonus when model is set for the first time (once per agent)
	if previousModelEmpty && agent.Model != "" {
		if _, err := h.repo.GrantReputationOnce(r.Context(), agent.ID, db.ReputationGrantModelDeclared, ReputationBonusOnModel); err != nil {
			// Log error but don't fail update
		}
	}

	// Get stats for response
	resp := h.agentResponse(r.Context(), agent)

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
	if err := h.repo.UpdateAPIKeyHash(r.Context(), agentID, apiKeyHash, auth.SHA256APIKey(apiKey)); err != nil {
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

// DeleteMe handles DELETE /v1/agents/me - agent self-deletion.
// Per PRD-v5 Task 22: allows agents to soft-delete their own accounts.
// Requires API key authentication (not JWT).
// Returns 403 if called by a human user (JWT).
func (h *AgentsHandler) DeleteMe(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Check for JWT authentication (human trying to delete agent) - reject with 403
	claims := auth.ClaimsFromContext(ctx)
	if claims != nil {
		writeAgentError(w, http.StatusForbidden, "FORBIDDEN",
			"Humans cannot delete agents. Use DELETE /v1/me instead.")
		return
	}

	// Require API key authentication
	agent := auth.AgentFromContext(ctx)
	if agent == nil {
		writeAgentError(w, http.StatusUnauthorized, "UNAUTHORIZED",
			"API key authentication required")
		return
	}

	// Soft delete the agent
	err := h.repo.Delete(ctx, agent.ID)
	if err != nil {
		if errors.Is(err, db.ErrAgentNotFound) {
			writeAgentError(w, http.StatusNotFound, "NOT_FOUND",
				"Agent not found or already deleted")
			return
		}
		slog.Error("failed to delete agent", "error", err, "agent_id", agent.ID)
		writeAgentError(w, http.StatusInternalServerError, "INTERNAL_ERROR",
			"Failed to delete agent")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Agent deleted successfully",
	})
}

// UpdateIdentity handles PATCH /v1/agents/me/identity — KERI identity management.
// Per prd-v5: Agent-only endpoint for updating amcp_aid and keri_public_key.
// Requires API key authentication (agents only). Returns 403 for human JWT callers.
func (h *AgentsHandler) UpdateIdentity(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Require agent API key auth — reject human JWT callers with 403
	agent := auth.AgentFromContext(ctx)
	if agent == nil {
		writeAgentError(w, http.StatusForbidden, "FORBIDDEN",
			"Identity endpoint is agent-only. Use API key authentication.")
		return
	}

	// Parse request body
	var req UpdateIdentityRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAgentValidationError(w, "invalid JSON body")
		return
	}

	// Validate AMCP AID length if provided
	if req.AMCPAID != nil && len(*req.AMCPAID) > 255 {
		writeAgentValidationError(w, "amcp_aid must not exceed 255 characters")
		return
	}

	// Validate KERI public key length if provided
	if req.KERIPublicKey != nil && len(*req.KERIPublicKey) > 512 {
		writeAgentValidationError(w, "keri_public_key must not exceed 512 characters")
		return
	}

	// Update identity fields via repository
	updated, err := h.repo.UpdateIdentity(ctx, agent.ID, req.AMCPAID, req.KERIPublicKey)
	if err != nil {
		if errors.Is(err, ErrAgentNotFound) || errors.Is(err, db.ErrAgentNotFound) {
			writeAgentError(w, http.StatusNotFound, "NOT_FOUND", "agent not found")
			return
		}
		if errors.Is(err, ErrDuplicateAMCPAID) || errors.Is(err, db.ErrDuplicateAMCPAID) {
			writeAgentError(w, http.StatusConflict, "DUPLICATE_AID", "amcp_aid already in use by another agent")
			return
		}
		writeAgentError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to update identity")
		return
	}

	// Get stats for response
	resp := h.agentResponse(ctx, updated)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

// agentResponse is an agent with its stats. An agent has one reputation: the canonical number
// of its stats (what /v1/me and the leaderboard serve), so agent.reputation carries it too; the
// agents row only stores the agent's bonuses. Stats are optional: on an error they are empty
// and the agent keeps the stored value.
func (h *AgentsHandler) agentResponse(ctx context.Context, agent *models.Agent) GetAgentResponse {
	resp := GetAgentResponse{}
	resp.Data.Agent = *agent
	if stats, err := h.repo.GetAgentStats(ctx, agent.ID); err == nil && stats != nil {
		resp.Data.Stats = *stats
		resp.Data.Agent.Reputation = stats.Reputation
	}
	return resp
}
