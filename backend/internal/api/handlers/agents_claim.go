package handlers

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/fcavalcantirj/solvr/internal/auth"
	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
)

// ReputationBonusOnClaim is the reputation bonus granted when a human claims an agent.
const ReputationBonusOnClaim = 50

// ReputationBonusOnModel is the reputation bonus granted once when an agent declares a model.
const ReputationBonusOnModel = 10

// AtomicAgentClaimer is implemented by the database claim-token repository. It keeps the
// agent link, claim rewards, and token consumption in one transaction.
type AtomicAgentClaimer interface {
	ClaimAgent(ctx context.Context, tokenID, agentID, humanID string, reputationBonus int) error
}

// GenerateClaimResponse is the response for POST /v1/agents/me/claim.
// Per SECURE-CLAIMING requirement: generate claim TOKEN for agent-human linking.
type GenerateClaimResponse struct {
	Token        string    `json:"token"`
	ClaimURL     string    `json:"claim_url"`
	ExpiresAt    time.Time `json:"expires_at"`
	Instructions string    `json:"instructions"`
}

// ClaimAgentRequest is the request body for POST /v1/agents/claim.
type ClaimAgentRequest struct {
	Token string `json:"token"`
}

// ClaimLookupRequest is the request body for POST /v1/agents/claim/lookup.
type ClaimLookupRequest struct {
	Token string `json:"token"`
}

// maxClaimLookupBody caps the lookup body: a claim token is 64 characters.
const maxClaimLookupBody = 4 << 10

// ClaimAgentResponse is the response for POST /v1/agents/claim.
type ClaimAgentResponse struct {
	Success bool         `json:"success"`
	Agent   models.Agent `json:"agent"`
	Message string       `json:"message"`
}

// ClaimInfoResponse is the response for POST /v1/agents/claim/lookup.
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
// - Create claim_url: https://solvr.dev/claim#token={token}
// - Token expires in 4 hours
// - Return claim_url to agent
// - Agent sends URL to their human
//
// The token rides in the URL fragment, which a browser never sends to a server, so it stays
// out of access logs and proxies; a path or query would be recorded by the frontend host and
// by analytics.
func (h *AgentsHandler) GenerateClaim(w http.ResponseWriter, r *http.Request) {
	// Require API key authentication (agent must be authenticated)
	agent := auth.AgentFromContext(r.Context())
	if agent == nil {
		writeAgentUnauthorized(w, "agent authentication required")
		return
	}
	if refuseIdentity(w, r, h.identityGate, db.IdentityQuery{AgentID: agent.ID}) {
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
		if existingToken.Token != "" {
			// Return existing active token: asking again must not kill the link already sent.
			resp := GenerateClaimResponse{
				Token:        existingToken.Token,
				ClaimURL:     h.claimURL(existingToken.Token),
				ExpiresAt:    existingToken.ExpiresAt,
				Instructions: generateClaimInstructions(),
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(resp)
			return
		}
		// A live token whose value cannot be recovered (issued before tokens were stored
		// sealed, or under a server secret that has changed) cannot be shown again. Replace
		// it rather than hand out a blank link; the one-unused-token-per-agent index needs
		// the old row gone first.
		if _, err := h.claimTokenRepo.DeleteUnusedByAgentID(r.Context(), agent.ID); err != nil {
			writeAgentError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to replace claim token")
			return
		}
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

	// Create claim token with 4 hour expiry
	now := time.Now()
	claimToken := &models.ClaimToken{
		Token:     tokenValue,
		AgentID:   agent.ID,
		ExpiresAt: now.Add(4 * time.Hour),
		CreatedAt: now,
	}

	if err := h.claimTokenRepo.Create(r.Context(), claimToken); err != nil {
		writeAgentError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to create claim token")
		return
	}

	// Return token and instructions
	resp := GenerateClaimResponse{
		Token:        tokenValue,
		ClaimURL:     h.claimURL(tokenValue),
		ExpiresAt:    claimToken.ExpiresAt,
		Instructions: generateClaimInstructions(),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resp)
}

// claimURL is the link an agent gives its human. The token follows the # so no server sees it.
func (h *AgentsHandler) claimURL(token string) string {
	return h.baseURL + "/claim#token=" + token
}

// generateClaimInstructions returns instructions for the agent to share with their human.
func generateClaimInstructions() string {
	return "Give this token to your human operator. " +
		"They should visit https://solvr.dev/settings/agents and paste the token " +
		"in the 'Claim Agent' field. When they confirm, you'll receive the 'Human-Backed' badge " +
		"and a +50 reputation bonus. Token expires in 4 hours."
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
		writeAgentError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid request body")
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
	if refuseIdentity(w, r, h.identityGate, db.IdentityQuery{AgentID: agent.ID, Email: claims.Email}) {
		return
	}

	// Check if agent is already claimed by a human
	if agent.HumanID != nil {
		writeAgentError(w, http.StatusConflict, "ALREADY_CLAIMED", "agent is already claimed")
		return
	}

	// Production repositories commit the link, rewards, and token consumption together.
	// The fallback preserves compatibility with focused handler mocks that do not own a DB
	// transaction; it is not used by the router's database-backed repository.
	atomicClaimer, atomic := h.claimTokenRepo.(AtomicAgentClaimer)
	if atomic {
		err = atomicClaimer.ClaimAgent(r.Context(), claimToken.ID, agent.ID, claims.UserID, ReputationBonusOnClaim)
	} else {
		err = h.repo.LinkHuman(r.Context(), agent.ID, claims.UserID)
	}
	if err != nil {
		if errors.Is(err, db.ErrAgentAlreadyClaimed) {
			writeAgentError(w, http.StatusConflict, "ALREADY_CLAIMED", "agent is already claimed")
			return
		}
		if errors.Is(err, db.ErrClaimTokenNotFound) {
			writeAgentError(w, http.StatusConflict, "TOKEN_USED", "token has already been used")
			return
		}
		writeAgentError(w, http.StatusInternalServerError, "LINK_FAILED", "failed to claim agent")
		return
	}

	// Backfill human owner memberships on rooms this agent created while unclaimed so they join the
	// human's family scope. Non-fatal: the claim already succeeded and must not fail if
	// the backfill errors.
	if h.roomBackfiller != nil {
		if n, err := h.roomBackfiller.BackfillOwnerFromMembership(r.Context(), agent.ID, claims.UserID); err != nil {
			slog.Warn("claim: room owner backfill failed", "error", err, "agent", agent.ID)
		} else if n > 0 {
			slog.Info("claim: backfilled room ownership", "agent", agent.ID, "rooms", n)
		}
	}

	if !atomic {
		// Unit-test repositories use the legacy individual operations. The real repository
		// took all three actions inside ClaimAgent above.
		_ = h.repo.AddReputation(r.Context(), agent.ID, ReputationBonusOnClaim)
		_ = h.repo.GrantHumanBackedBadge(r.Context(), agent.ID)
		_ = h.claimTokenRepo.MarkUsed(r.Context(), claimToken.ID, claims.UserID)
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

// LookupClaim handles POST /v1/agents/claim/lookup - get claim token info for the
// confirmation page. Public (no auth) so the page can show the agent before login.
// The token is read from the JSON body, never from the URL: a URL is recorded by every proxy,
// log and analytics tool between the browser and here.
func (h *AgentsHandler) LookupClaim(w http.ResponseWriter, r *http.Request) {
	var req ClaimLookupRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxClaimLookupBody)).Decode(&req); err != nil {
		writeAgentError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid request body")
		return
	}
	tokenValue := req.Token
	if tokenValue == "" {
		writeAgentError(w, http.StatusBadRequest, "MISSING_TOKEN", "token is required")
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
