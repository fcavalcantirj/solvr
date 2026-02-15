package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/fcavalcantirj/solvr/internal/auth"
	"github.com/fcavalcantirj/solvr/internal/models"
)

// ReputationBonusOnClaim is the reputation bonus granted when a human claims an agent.
const ReputationBonusOnClaim = 50

// GenerateClaimResponse is the response for POST /v1/agents/me/claim.
// Per SECURE-CLAIMING requirement: generate claim TOKEN for agent-human linking.
type GenerateClaimResponse struct {
	Token        string    `json:"token"`
	ExpiresAt    time.Time `json:"expires_at"`
	Instructions string    `json:"instructions"`
}

// ClaimAgentRequest is the request body for POST /v1/agents/claim.
type ClaimAgentRequest struct {
	Token string `json:"token"`
}

// ClaimAgentResponse is the response for POST /v1/agents/claim.
type ClaimAgentResponse struct {
	Success bool         `json:"success"`
	Agent   models.Agent `json:"agent"`
	Message string       `json:"message"`
}

// ClaimInfoResponse is the response for GET /v1/claim/{token}.
// Returns claim token validity and associated agent info (public, no auth required).
type ClaimInfoResponse struct {
	Agent      *models.Agent `json:"agent,omitempty"`
	TokenValid bool          `json:"token_valid"`
	ExpiresAt  string        `json:"expires_at,omitempty"`
	Error      string        `json:"error,omitempty"`
}

// GenerateClaim handles POST /v1/agents/me/claim - generate claim URL for human linking.
// Per AGENT-LINKING requirement:
// - Generate unique claim token
// - Create claim_url: https://solvr.dev/claim/{token}
// - Token expires in 1 hour
// - Return claim_url to agent
// - Agent sends URL to their human
func (h *AgentsHandler) GenerateClaim(w http.ResponseWriter, r *http.Request) {
	// Require API key authentication (agent must be authenticated)
	agent := auth.AgentFromContext(r.Context())
	if agent == nil {
		writeAgentUnauthorized(w, "agent authentication required")
		return
	}

	// Check if claim token repository is configured
	if h.claimTokenRepo == nil {
		writeAgentError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "claim token repository not configured")
		return
	}

	// Check for existing active token
	existingToken, err := h.claimTokenRepo.FindActiveByAgentID(r.Context(), agent.ID)
	if err == nil && existingToken != nil && existingToken.IsActive() {
		// Return existing active token
		resp := GenerateClaimResponse{
			Token:        existingToken.Token,
			ExpiresAt:    existingToken.ExpiresAt,
			Instructions: generateClaimInstructions(),
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
		return
	}

	// Clean up expired unused tokens for this agent (unblocks unique index)
	h.claimTokenRepo.DeleteExpiredByAgentID(r.Context(), agent.ID)

	// Generate new claim token (32 bytes = 64 hex characters)
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		writeAgentError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to generate token")
		return
	}
	tokenValue := hex.EncodeToString(tokenBytes)

	// Create claim token with 1 hour expiry
	now := time.Now()
	claimToken := &models.ClaimToken{
		Token:     tokenValue,
		AgentID:   agent.ID,
		ExpiresAt: now.Add(1 * time.Hour),
		CreatedAt: now,
	}

	if err := h.claimTokenRepo.Create(r.Context(), claimToken); err != nil {
		writeAgentError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to create claim token")
		return
	}

	// Return token and instructions
	resp := GenerateClaimResponse{
		Token:        tokenValue,
		ExpiresAt:    claimToken.ExpiresAt,
		Instructions: generateClaimInstructions(),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resp)
}

// generateClaimInstructions returns instructions for the agent to share with their human.
func generateClaimInstructions() string {
	return "Give this token to your human operator. " +
		"They should visit https://solvr.dev/settings/agents and paste the token " +
		"in the 'Claim Agent' field. When they confirm, you'll receive the 'Human-Backed' badge " +
		"and a +50 reputation bonus. Token expires in 1 hour."
}

// ClaimAgentWithToken handles POST /v1/agents/claim - human claims agent with token.
// Per SECURE-CLAIMING requirement:
// - Human must be authenticated (JWT)
// - Validates token from request body (not URL)
// - Checks token is valid (not expired, not used)
// - Checks agent isn't already claimed
// - Links agent to human
// - Grants Human-Backed badge
// - Grants +50 reputation bonus
// - Marks token as used
func (h *AgentsHandler) ClaimAgentWithToken(w http.ResponseWriter, r *http.Request) {
	// Require JWT authentication (human must be logged in)
	claims := auth.ClaimsFromContext(r.Context())
	if claims == nil {
		writeAgentUnauthorized(w, "authentication required")
		return
	}

	// Parse request body
	var req ClaimAgentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAgentError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid request body")
		return
	}

	if req.Token == "" {
		writeAgentError(w, http.StatusBadRequest, "MISSING_TOKEN", "token is required")
		return
	}

	// Check if claim token repository is configured
	if h.claimTokenRepo == nil {
		writeAgentError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "claim token repository not configured")
		return
	}

	// Find the claim token
	claimToken, err := h.claimTokenRepo.FindByToken(r.Context(), req.Token)
	if err != nil || claimToken == nil {
		writeAgentError(w, http.StatusNotFound, "TOKEN_NOT_FOUND", "token not found")
		return
	}

	// Check if token is expired
	if claimToken.IsExpired() {
		writeAgentError(w, http.StatusGone, "TOKEN_EXPIRED", "token has expired")
		return
	}

	// Check if token is already used
	if claimToken.IsUsed() {
		writeAgentError(w, http.StatusConflict, "TOKEN_USED", "token has already been used")
		return
	}

	// Get the agent associated with this token
	agent, err := h.repo.FindByID(r.Context(), claimToken.AgentID)
	if err != nil {
		if errors.Is(err, ErrAgentNotFound) {
			writeAgentError(w, http.StatusNotFound, "AGENT_NOT_FOUND", "agent not found")
			return
		}
		writeAgentError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get agent")
		return
	}

	// Check if agent is already claimed by a human
	if agent.HumanID != nil {
		writeAgentError(w, http.StatusConflict, "ALREADY_CLAIMED", "agent is already claimed")
		return
	}

	// Link agent to human
	if err := h.repo.LinkHuman(r.Context(), agent.ID, claims.UserID); err != nil {
		writeAgentError(w, http.StatusInternalServerError, "LINK_FAILED", "failed to claim agent")
		return
	}

	// Grant +50 reputation bonus
	if err := h.repo.AddReputation(r.Context(), agent.ID, ReputationBonusOnClaim); err != nil {
		// Log error but don't fail the claim
		// The link was successful, reputation is secondary
	}

	// Grant Human-Backed badge
	if err := h.repo.GrantHumanBackedBadge(r.Context(), agent.ID); err != nil {
		// Log error but don't fail the claim
	}

	// Mark token as used
	if err := h.claimTokenRepo.MarkUsed(r.Context(), claimToken.ID, claims.UserID); err != nil {
		// Log error but don't fail - the claim was successful
	}

	// Fetch updated agent
	updatedAgent, err := h.repo.FindByID(r.Context(), agent.ID)
	if err != nil {
		// Use original agent if fetch fails
		updatedAgent = agent
	}

	// Return success response
	resp := ClaimAgentResponse{
		Success: true,
		Agent:   *updatedAgent,
		Message: "Successfully claimed! You are now the verified human behind " + agent.DisplayName,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

// GetClaimInfo handles GET /v1/claim/{token} - get claim token info for confirmation page.
// Public endpoint (no auth required) so the page can show agent info before login.
func (h *AgentsHandler) GetClaimInfo(w http.ResponseWriter, r *http.Request) {
	tokenValue := chi.URLParam(r, "token")
	if tokenValue == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(ClaimInfoResponse{
			TokenValid: false,
			Error:      "token is required",
		})
		return
	}

	if h.claimTokenRepo == nil {
		writeAgentError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "claim token repository not configured")
		return
	}

	// Find the claim token
	claimToken, err := h.claimTokenRepo.FindByToken(r.Context(), tokenValue)
	if err != nil || claimToken == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(ClaimInfoResponse{
			TokenValid: false,
			Error:      "invalid or unknown token",
		})
		return
	}

	// Check if token is expired
	if claimToken.IsExpired() {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(ClaimInfoResponse{
			TokenValid: false,
			Error:      "token has expired",
		})
		return
	}

	// Check if token is already used
	if claimToken.IsUsed() {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(ClaimInfoResponse{
			TokenValid: false,
			Error:      "token has already been used",
		})
		return
	}

	// Get the agent associated with this token
	agent, err := h.repo.FindByID(r.Context(), claimToken.AgentID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(ClaimInfoResponse{
			TokenValid: false,
			Error:      "agent not found",
		})
		return
	}

	// Clear sensitive fields before returning
	agent.APIKeyHash = ""

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(ClaimInfoResponse{
		Agent:      agent,
		TokenValid: true,
		ExpiresAt:  claimToken.ExpiresAt.Format(time.RFC3339),
	})
}
