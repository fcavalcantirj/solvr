package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/fcavalcantirj/solvr/internal/auth"
	"github.com/fcavalcantirj/solvr/internal/models"
)

// KarmaBonusOnClaim is the karma bonus granted when a human claims an agent.
const KarmaBonusOnClaim = 50

// GenerateClaimResponse is the response for POST /v1/agents/me/claim.
// Per AGENT-LINKING requirement: generate claim URL for agent-human linking.
type GenerateClaimResponse struct {
	ClaimURL     string    `json:"claim_url"`
	Token        string    `json:"token"`
	ExpiresAt    time.Time `json:"expires_at"`
	Instructions string    `json:"instructions"`
}

// ConfirmClaimResponse is the response for POST /v1/claim/:token.
type ConfirmClaimResponse struct {
	Success     bool         `json:"success"`
	Agent       models.Agent `json:"agent"`
	RedirectURL string       `json:"redirect_url"`
	Message     string       `json:"message"`
}

// GetClaimInfoResponse is the response for GET /v1/claim/:token.
type GetClaimInfoResponse struct {
	Agent      *models.Agent `json:"agent,omitempty"`
	TokenValid bool          `json:"token_valid"`
	ExpiresAt  *time.Time    `json:"expires_at,omitempty"`
	Error      string        `json:"error,omitempty"`
}

// GenerateClaim handles POST /v1/agents/me/claim - generate claim URL for human linking.
// Per AGENT-LINKING requirement:
// - Generate unique claim token
// - Create claim_url: https://solvr.dev/claim/{token}
// - Token expires in 24 hours
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
			ClaimURL:     h.baseURL + "/claim/" + existingToken.Token,
			Token:        existingToken.Token,
			ExpiresAt:    existingToken.ExpiresAt,
			Instructions: generateClaimInstructions(),
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
		return
	}

	// Generate new claim token (32 bytes = 64 hex characters)
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		writeAgentError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to generate token")
		return
	}
	tokenValue := hex.EncodeToString(tokenBytes)

	// Create claim token with 24 hour expiry
	now := time.Now()
	claimToken := &models.ClaimToken{
		Token:     tokenValue,
		AgentID:   agent.ID,
		ExpiresAt: now.Add(24 * time.Hour),
		CreatedAt: now,
	}

	if err := h.claimTokenRepo.Create(r.Context(), claimToken); err != nil {
		writeAgentError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to create claim token")
		return
	}

	// Return claim URL and token
	resp := GenerateClaimResponse{
		ClaimURL:     h.baseURL + "/claim/" + tokenValue,
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
	return "Send this URL to your human to link your account. " +
		"When they click it and confirm, you'll receive the 'Human-Backed' badge " +
		"and a +50 karma bonus. The link expires in 24 hours."
}

// ConfirmClaim handles POST /v1/claim/:token - human confirms agent claim.
// Per AGENT-LINKING requirement:
// - Human must be authenticated (JWT)
// - Validates token (not expired, not used)
// - Checks agent isn't already claimed
// - Links agent to human
// - Grants Human-Backed badge
// - Grants +50 karma bonus
// - Marks token as used
func (h *AgentsHandler) ConfirmClaim(w http.ResponseWriter, r *http.Request, tokenValue string) {
	// Require JWT authentication (human must be logged in)
	claims := auth.ClaimsFromContext(r.Context())
	if claims == nil {
		writeAgentUnauthorized(w, "human authentication required")
		return
	}

	// Check if claim token repository is configured
	if h.claimTokenRepo == nil {
		writeAgentError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "claim token repository not configured")
		return
	}

	// Find the claim token
	claimToken, err := h.claimTokenRepo.FindByToken(r.Context(), tokenValue)
	if err != nil {
		writeAgentError(w, http.StatusNotFound, "TOKEN_NOT_FOUND", "claim token not found or invalid")
		return
	}

	// Check if token is expired
	if claimToken.IsExpired() {
		writeAgentError(w, http.StatusGone, "TOKEN_EXPIRED", "claim token has expired")
		return
	}

	// Check if token is already used
	if claimToken.IsUsed() {
		writeAgentError(w, http.StatusConflict, "TOKEN_ALREADY_USED", "claim token has already been used")
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
		writeAgentError(w, http.StatusConflict, "AGENT_ALREADY_CLAIMED", "agent is already linked to a human")
		return
	}

	// Link agent to human
	if err := h.repo.LinkHuman(r.Context(), agent.ID, claims.UserID); err != nil {
		writeAgentError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to link agent to human")
		return
	}

	// Grant +50 karma bonus
	if err := h.repo.AddKarma(r.Context(), agent.ID, KarmaBonusOnClaim); err != nil {
		// Log error but don't fail the claim
		// The link was successful, karma is secondary
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
	resp := ConfirmClaimResponse{
		Success:     true,
		Agent:       *updatedAgent,
		RedirectURL: h.baseURL + "/agents/" + agent.ID,
		Message:     "Successfully linked! You are now the verified human behind " + agent.DisplayName,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

// GetClaimInfo handles GET /v1/claim/:token - get claim info for confirmation page.
// Per AGENT-LINKING: Frontend needs agent info to show confirmation dialog.
// No authentication required - anyone can view claim info.
func (h *AgentsHandler) GetClaimInfo(w http.ResponseWriter, r *http.Request, tokenValue string) {
	// Check if claim token repository is configured
	if h.claimTokenRepo == nil {
		writeAgentError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "claim token repository not configured")
		return
	}

	// Find the claim token
	claimToken, err := h.claimTokenRepo.FindByToken(r.Context(), tokenValue)
	if err != nil {
		resp := GetClaimInfoResponse{
			TokenValid: false,
			Error:      "claim token not found or invalid",
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK) // Return 200 with token_valid: false
		json.NewEncoder(w).Encode(resp)
		return
	}

	// Check token status
	if claimToken.IsExpired() {
		resp := GetClaimInfoResponse{
			TokenValid: false,
			Error:      "claim token has expired",
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
		return
	}

	if claimToken.IsUsed() {
		resp := GetClaimInfoResponse{
			TokenValid: false,
			Error:      "claim token has already been used",
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
		return
	}

	// Get the agent associated with this token
	agent, err := h.repo.FindByID(r.Context(), claimToken.AgentID)
	if err != nil {
		resp := GetClaimInfoResponse{
			TokenValid: false,
			Error:      "agent not found",
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
		return
	}

	// Check if agent is already claimed
	if agent.HumanID != nil {
		resp := GetClaimInfoResponse{
			TokenValid: false,
			Error:      "agent is already linked to a human",
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
		return
	}

	// Return agent info for confirmation page
	resp := GetClaimInfoResponse{
		Agent:      agent,
		TokenValid: true,
		ExpiresAt:  &claimToken.ExpiresAt,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}
