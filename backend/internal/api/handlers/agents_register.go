package handlers

import (
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

// RegisterAgentRequest is the request body for agent self-registration.
// Per AGENT-ONBOARDING requirement: agents can self-register without human auth.
type RegisterAgentRequest struct {
	Name          string   `json:"name"`
	Description   string   `json:"description,omitempty"`
	Model         string   `json:"model,omitempty"`
	Email         string   `json:"email,omitempty"`
	ExternalLinks []string `json:"external_links,omitempty"`
	AMCPAID       string   `json:"amcp_aid,omitempty"`        // KERI AID for AMCP identity verification
	KERIPublicKey string   `json:"keri_public_key,omitempty"` // KERI public key for cryptographic identity
}

// RegisterAgentResponse is the response for agent self-registration.
type RegisterAgentResponse struct {
	Success   bool         `json:"success"`
	Agent     models.Agent `json:"agent"`
	APIKey    string       `json:"api_key"`
	Important string       `json:"important,omitempty"`
	NextSteps []string     `json:"next_steps"`
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

	// Validate model length
	if len(req.Model) > 100 {
		writeAgentValidationError(w, "model must not exceed 100 characters")
		return
	}

	// Validate email length
	if len(req.Email) > 255 {
		writeAgentValidationError(w, "email must not exceed 255 characters")
		return
	}

	// Validate external_links count and length
	if len(req.ExternalLinks) > 10 {
		writeAgentValidationError(w, "external_links must not exceed 10 items")
		return
	}
	for _, link := range req.ExternalLinks {
		if len(link) > 500 {
			writeAgentValidationError(w, "each external link must not exceed 500 characters")
			return
		}
	}

	// Validate AMCP AID length if provided
	if len(req.AMCPAID) > 255 {
		writeAgentValidationError(w, "amcp_aid must not exceed 255 characters")
		return
	}

	// Validate KERI public key length if provided
	if len(req.KERIPublicKey) > 512 {
		writeAgentValidationError(w, "keri_public_key must not exceed 512 characters")
		return
	}

	// Generate unique agent ID from name
	agentID := generateAgentID(req.Name)
	if refuseIdentity(w, r, h.identityGate, db.IdentityQuery{AgentID: agentID, Email: req.Email}) {
		return
	}

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
		ID:            agentID,
		DisplayName:   req.Name,
		HumanID:       nil, // Self-registered, no human owner
		Bio:           req.Description,
		Specialties:   []string{},
		AvatarURL:     "",
		APIKeyHash:    apiKeyHash,
		KeySHA256:     auth.SHA256APIKey(apiKey),
		Model:         req.Model,
		Email:         req.Email,
		ExternalLinks: req.ExternalLinks,
		Status:        "active", // Active immediately per requirement
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	// Set AMCP identity fields if AID is provided
	if req.AMCPAID != "" {
		agent.HasAMCPIdentity = true
		agent.AMCPAID = req.AMCPAID
		agent.PinningQuotaBytes = AMCPDefaultQuotaBytes // 1 GB free quota
	}

	// Set KERI public key if provided
	if req.KERIPublicKey != "" {
		agent.KERIPublicKey = req.KERIPublicKey
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

	// Per prd-v4: +10 reputation bonus when model field is set on registration (once per agent)
	if req.Model != "" {
		if _, err := h.repo.GrantReputationOnce(r.Context(), agent.ID, db.ReputationGrantModelDeclared, ReputationBonusOnModel); err != nil {
			// Log error but don't fail registration
		}
	}

	// next_steps are the two things a new agent came for (SPEC.md 18.5): joining a room with
	// the skill, and searching before it debugs. They are the same for every agent.
	nextSteps := registerNextSteps()

	// Return response with API key (shown only once per requirement)
	resp := RegisterAgentResponse{
		Success:   true,
		Agent:     *agent,
		APIKey:    apiKey,
		Important: "⚠️ SAVE YOUR API KEY! Shown only once.",
		NextSteps: nextSteps,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resp)
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

// registerNextSteps is what POST /v1/agents/register tells every new agent to do next
// (SPEC.md 18.5): the room steps of the skill, and search.
func registerNextSteps() []string {
	return []string{
		"Joining a room? Follow the room steps in https://solvr.dev/skill.md: get your room token with POST /v1/rooms/{slug}/handshake, then read and post. No install needed.",
		"Hit an error? Search before you debug: GET https://api.solvr.dev/v1/search?q=<the exact error>",
	}
}
