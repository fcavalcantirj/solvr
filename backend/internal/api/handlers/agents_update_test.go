package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/auth"
	"github.com/fcavalcantirj/solvr/internal/models"
)

func TestUpdateAgent_Success(t *testing.T) {
	repo := NewMockAgentRepository()
	handler := NewAgentsHandler(repo, "test-jwt-secret")

	humanID := "user-123"
	repo.agents["my_agent"] = &models.Agent{
		ID:          "my_agent",
		DisplayName: "Old Name",
		HumanID:     &humanID,
		Bio:         "Old bio",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	reqBody := UpdateAgentRequest{
		DisplayName: strPtr("New Name"),
		Bio:         strPtr("New bio"),
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPatch, "/v1/agents/my_agent", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = addJWTClaimsToContext(req, "user-123", "user@example.com", "user")

	rr := httptest.NewRecorder()
	handler.UpdateAgent(rr, req, "my_agent")

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp GetAgentResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Data.Agent.DisplayName != "New Name" {
		t.Errorf("expected display name 'New Name', got '%s'", resp.Data.Agent.DisplayName)
	}
	if resp.Data.Agent.Bio != "New bio" {
		t.Errorf("expected bio 'New bio', got '%s'", resp.Data.Agent.Bio)
	}
}

func TestUpdateAgent_NotOwner(t *testing.T) {
	repo := NewMockAgentRepository()
	handler := NewAgentsHandler(repo, "test-jwt-secret")

	humanID := "other-user"
	repo.agents["my_agent"] = &models.Agent{
		ID:          "my_agent",
		DisplayName: "Agent",
		HumanID:     &humanID,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	reqBody := UpdateAgentRequest{
		DisplayName: strPtr("New Name"),
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPatch, "/v1/agents/my_agent", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = addJWTClaimsToContext(req, "user-123", "user@example.com", "user") // Different user

	rr := httptest.NewRecorder()
	handler.UpdateAgent(rr, req, "my_agent")

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected status 403, got %d: %s", rr.Code, rr.Body.String())
	}

	var errResp map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&errResp)
	errorObj := errResp["error"].(map[string]interface{})
	if errorObj["code"] != "FORBIDDEN" {
		t.Errorf("expected error code FORBIDDEN, got %s", errorObj["code"])
	}
}

func TestUpdateAgent_NoAuth(t *testing.T) {
	repo := NewMockAgentRepository()
	handler := NewAgentsHandler(repo, "test-jwt-secret")

	humanID := "user-123"
	repo.agents["my_agent"] = &models.Agent{
		ID:      "my_agent",
		HumanID: &humanID,
	}

	reqBody := UpdateAgentRequest{
		DisplayName: strPtr("New Name"),
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPatch, "/v1/agents/my_agent", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	// No JWT claims

	rr := httptest.NewRecorder()
	handler.UpdateAgent(rr, req, "my_agent")

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rr.Code)
	}
}

func TestUpdateAgent_NotFound(t *testing.T) {
	repo := NewMockAgentRepository()
	handler := NewAgentsHandler(repo, "test-jwt-secret")

	reqBody := UpdateAgentRequest{
		DisplayName: strPtr("New Name"),
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPatch, "/v1/agents/nonexistent", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = addJWTClaimsToContext(req, "user-123", "user@example.com", "user")

	rr := httptest.NewRecorder()
	handler.UpdateAgent(rr, req, "nonexistent")

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", rr.Code)
	}
}

func TestRegenerateAPIKey_Success(t *testing.T) {
	repo := NewMockAgentRepository()
	handler := NewAgentsHandler(repo, "test-jwt-secret")

	humanID := "user-123"
	repo.agents["my_agent"] = &models.Agent{
		ID:         "my_agent",
		HumanID:    &humanID,
		APIKeyHash: "old_hash",
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/agents/my_agent/api-key", nil)
	req = addJWTClaimsToContext(req, "user-123", "user@example.com", "user")

	rr := httptest.NewRecorder()
	handler.RegenerateAPIKey(rr, req, "my_agent")

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&resp)

	data := resp["data"].(map[string]interface{})
	apiKey := data["api_key"].(string)
	if apiKey == "" || len(apiKey) < 10 || apiKey[:6] != "solvr_" {
		t.Errorf("expected new API key with solvr_ prefix, got '%s'", apiKey)
	}
}

func TestRegenerateAPIKey_NotOwner(t *testing.T) {
	repo := NewMockAgentRepository()
	handler := NewAgentsHandler(repo, "test-jwt-secret")

	humanID := "other-user"
	repo.agents["my_agent"] = &models.Agent{
		ID:      "my_agent",
		HumanID: &humanID,
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/agents/my_agent/api-key", nil)
	req = addJWTClaimsToContext(req, "user-123", "user@example.com", "user")

	rr := httptest.NewRecorder()
	handler.RegenerateAPIKey(rr, req, "my_agent")

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected status 403, got %d", rr.Code)
	}
}

func TestRevokeAPIKey_Success(t *testing.T) {
	repo := NewMockAgentRepository()
	handler := NewAgentsHandler(repo, "test-jwt-secret")

	humanID := "user-123"
	repo.agents["my_agent"] = &models.Agent{
		ID:         "my_agent",
		HumanID:    &humanID,
		APIKeyHash: "some_hash",
	}

	req := httptest.NewRequest(http.MethodDelete, "/v1/agents/my_agent/api-key", nil)
	req = addJWTClaimsToContext(req, "user-123", "user@example.com", "user")

	rr := httptest.NewRecorder()
	handler.RevokeAPIKey(rr, req, "my_agent")

	if rr.Code != http.StatusNoContent {
		t.Errorf("expected status 204, got %d: %s", rr.Code, rr.Body.String())
	}

	// Verify API key hash is cleared
	if repo.agents["my_agent"].APIKeyHash != "" {
		t.Error("expected API key hash to be cleared")
	}
}

func TestRevokeAPIKey_NotOwner(t *testing.T) {
	repo := NewMockAgentRepository()
	handler := NewAgentsHandler(repo, "test-jwt-secret")

	humanID := "other-user"
	repo.agents["my_agent"] = &models.Agent{
		ID:      "my_agent",
		HumanID: &humanID,
	}

	req := httptest.NewRequest(http.MethodDelete, "/v1/agents/my_agent/api-key", nil)
	req = addJWTClaimsToContext(req, "user-123", "user@example.com", "user")

	rr := httptest.NewRecorder()
	handler.RevokeAPIKey(rr, req, "my_agent")

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected status 403, got %d", rr.Code)
	}
}

// TestUpdateAgent_NeverReturnsAPIKey verifies that PATCH /v1/agents/:id never returns api_key or api_key_hash.
// This is a CRITICAL security requirement per SPEC.md Part 5.6.
func TestUpdateAgent_NeverReturnsAPIKey(t *testing.T) {
	repo := NewMockAgentRepository()
	handler := NewAgentsHandler(repo, "test-jwt-secret")

	// Create agent with an API key hash
	humanID := "user-123"
	repo.agents["my_agent"] = &models.Agent{
		ID:          "my_agent",
		DisplayName: "Test Agent",
		HumanID:     &humanID,
		APIKeyHash:  "hashed_secret_key_value",
	}

	reqBody := strings.NewReader(`{"display_name":"Updated Agent"}`)
	req := httptest.NewRequest(http.MethodPatch, "/v1/agents/my_agent", reqBody)
	req = addJWTClaimsToContext(req, "user-123", "user@example.com", "user")

	rr := httptest.NewRecorder()
	handler.UpdateAgent(rr, req, "my_agent")

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	body := rr.Body.String()

	// SECURITY: Verify api_key is NOT in response
	if strings.Contains(body, "api_key") {
		t.Error("SECURITY VIOLATION: response contains 'api_key' field")
	}

	// SECURITY: Verify api_key_hash is NOT in response
	if strings.Contains(body, "api_key_hash") {
		t.Error("SECURITY VIOLATION: response contains 'api_key_hash' field")
	}

	// SECURITY: Verify the actual hash value is NOT in response
	if strings.Contains(body, "hashed_secret_key_value") {
		t.Error("SECURITY VIOLATION: response contains the API key hash value")
	}
}

// Helper function for string pointers
func strPtr(s string) *string { return &s }

// ============================================================================
// Tests for reputation bonus when model is set (prd-v4 requirement)
// ============================================================================

// TestUpdateAgent_ModelReputationBonus_FirstTime tests that updating model for first time gives +10 reputation.
// Per prd-v4: Agent updating model first time gets +10 reputation.
func TestUpdateAgent_ModelReputationBonus_FirstTime(t *testing.T) {
	repo := NewMockAgentRepository()
	handler := NewAgentsHandler(repo, "test-jwt-secret")

	humanID := "user-123"
	repo.agents["my_agent"] = &models.Agent{
		ID:          "my_agent",
		DisplayName: "Test Agent",
		HumanID:     &humanID,
		Reputation:  0,  // Start with 0 reputation
		Model:       "", // No model initially
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	reqBody := UpdateAgentRequest{
		Model: strPtr("claude-opus-4"),
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPatch, "/v1/agents/my_agent", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = addJWTClaimsToContext(req, "user-123", "user@example.com", "user")

	rr := httptest.NewRecorder()
	handler.UpdateAgent(rr, req, "my_agent")

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp GetAgentResponse
	json.NewDecoder(rr.Body).Decode(&resp)

	// Verify agent got +10 reputation for setting model: the bonus is stored on the agent
	// (the response carries the canonical reputation of its stats).
	if got := repo.agents["my_agent"].Reputation; got != 10 {
		t.Errorf("expected stored reputation 10 after setting model, got %d", got)
	}

	// Verify model is set
	if resp.Data.Agent.Model != "claude-opus-4" {
		t.Errorf("expected model 'claude-opus-4', got '%s'", resp.Data.Agent.Model)
	}
}

// TestUpdateAgent_ModelReputationBonus_NoBonus tests that changing model does NOT give bonus again.
// Per prd-v4: Agent changing model does not get bonus again.
func TestUpdateAgent_ModelReputationBonus_NoBonus(t *testing.T) {
	repo := NewMockAgentRepository()
	handler := NewAgentsHandler(repo, "test-jwt-secret")

	humanID := "user-123"
	repo.agents["my_agent"] = &models.Agent{
		ID:          "my_agent",
		DisplayName: "Test Agent",
		HumanID:     &humanID,
		Reputation:  50,      // Already has reputation
		Model:       "gpt-4", // Already has model
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	reqBody := UpdateAgentRequest{
		Model: strPtr("claude-opus-4"), // Changing to different model
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPatch, "/v1/agents/my_agent", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = addJWTClaimsToContext(req, "user-123", "user@example.com", "user")

	rr := httptest.NewRecorder()
	handler.UpdateAgent(rr, req, "my_agent")

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp GetAgentResponse
	json.NewDecoder(rr.Body).Decode(&resp)

	// Verify reputation did NOT change (no bonus for changing model)
	if got := repo.agents["my_agent"].Reputation; got != 50 {
		t.Errorf("expected stored reputation 50 (unchanged), got %d", got)
	}

	// Verify model was changed
	if resp.Data.Agent.Model != "claude-opus-4" {
		t.Errorf("expected model 'claude-opus-4', got '%s'", resp.Data.Agent.Model)
	}
}

// ============================================================================
// Tests for PATCH /v1/agents/{id} API key auth (prd-v4 requirement)
// ============================================================================

// TestUpdateAgent_WithAPIKey tests that an agent can update itself using API key auth.
// Per prd-v4: Handler should verify caller owns the agent (human_id matches or agent's own API key)
func TestUpdateAgent_WithAPIKey(t *testing.T) {
	repo := NewMockAgentRepository()
	handler := NewAgentsHandler(repo, "test-jwt-secret")

	// Create agent with API key
	apiKey := "sak_testapikey123"
	apiKeyHash, _ := auth.HashAPIKey(apiKey)
	repo.agents["my_agent"] = &models.Agent{
		ID:          "my_agent",
		DisplayName: "Test Agent",
		APIKeyHash:  apiKeyHash,
		Bio:         "Original bio",
		Model:       "gpt-4",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	reqBody := UpdateAgentRequest{
		Bio: strPtr("Updated bio via API key"),
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPatch, "/v1/agents/my_agent", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	// Add agent to context (simulating API key middleware)
	req = addAgentToContext(req, repo.agents["my_agent"])

	rr := httptest.NewRecorder()
	handler.UpdateAgent(rr, req, "my_agent")

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp GetAgentResponse
	json.NewDecoder(rr.Body).Decode(&resp)

	// Verify bio was updated
	if resp.Data.Agent.Bio != "Updated bio via API key" {
		t.Errorf("expected bio 'Updated bio via API key', got '%s'", resp.Data.Agent.Bio)
	}
}

// TestUpdateAgent_APIKey_WrongAgent tests that an agent cannot update a different agent.
// Per prd-v4: Agent can only update itself via API key.
func TestUpdateAgent_APIKey_WrongAgent(t *testing.T) {
	repo := NewMockAgentRepository()
	handler := NewAgentsHandler(repo, "test-jwt-secret")

	// Create two agents
	apiKey := "sak_testapikey123"
	apiKeyHash, _ := auth.HashAPIKey(apiKey)
	repo.agents["agent_one"] = &models.Agent{
		ID:          "agent_one",
		DisplayName: "Agent One",
		APIKeyHash:  apiKeyHash,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	repo.agents["agent_two"] = &models.Agent{
		ID:          "agent_two",
		DisplayName: "Agent Two",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	reqBody := UpdateAgentRequest{
		Bio: strPtr("Trying to update another agent"),
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPatch, "/v1/agents/agent_two", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	// Add agent_one to context (trying to update agent_two)
	req = addAgentToContext(req, repo.agents["agent_one"])

	rr := httptest.NewRecorder()
	handler.UpdateAgent(rr, req, "agent_two")

	// Should be forbidden
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected status 403, got %d: %s", rr.Code, rr.Body.String())
	}
}
