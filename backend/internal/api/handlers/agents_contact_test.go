package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/auth"
	"github.com/fcavalcantirj/solvr/internal/models"
)

// addAgentToContext adds an agent to the request context (simulating API key middleware).
func addAgentToContext(r *http.Request, agent *models.Agent) *http.Request {
	ctx := auth.ContextWithAgent(r.Context(), agent)
	return r.WithContext(ctx)
}

// ============================================================================
// Tests for email and external_links fields (TDD)
// ============================================================================

// TestRegisterAgent_WithEmailAndExternalLinks tests that agents can register with email and external_links.
func TestRegisterAgent_WithEmailAndExternalLinks(t *testing.T) {
	repo := NewMockAgentRepository()
	handler := NewAgentsHandler(repo, "test-jwt-secret")

	reqBody := map[string]interface{}{
		"name":           "TestAgent",
		"description":    "A test agent",
		"model":          "claude-opus-4",
		"email":          "testagent@example.com",
		"external_links": []string{"https://github.com/testagent", "https://twitter.com/testagent"},
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/v1/agents/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.RegisterAgent(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp RegisterAgentResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Verify email and external_links are set
	if resp.Agent.Email != "testagent@example.com" {
		t.Errorf("expected email 'testagent@example.com', got '%s'", resp.Agent.Email)
	}
	if len(resp.Agent.ExternalLinks) != 2 {
		t.Errorf("expected 2 external links, got %d", len(resp.Agent.ExternalLinks))
	}
	if len(resp.Agent.ExternalLinks) > 0 && resp.Agent.ExternalLinks[0] != "https://github.com/testagent" {
		t.Errorf("expected first link 'https://github.com/testagent', got '%s'", resp.Agent.ExternalLinks[0])
	}
}

// TestRegisterAgent_EmailValidation tests that email is validated.
func TestRegisterAgent_EmailValidation(t *testing.T) {
	repo := NewMockAgentRepository()
	handler := NewAgentsHandler(repo, "test-jwt-secret")

	// Email too long (> 255 chars)
	longEmail := strings.Repeat("a", 250) + "@example.com"
	reqBody := map[string]interface{}{
		"name":  "TestAgent",
		"email": longEmail,
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/v1/agents/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.RegisterAgent(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for email too long, got %d: %s", rr.Code, rr.Body.String())
	}
}

// TestRegisterAgent_ExternalLinksValidation tests that external_links are validated.
func TestRegisterAgent_ExternalLinksValidation(t *testing.T) {
	repo := NewMockAgentRepository()
	handler := NewAgentsHandler(repo, "test-jwt-secret")

	// Too many external links (> 10)
	links := make([]string, 11)
	for i := range links {
		links[i] = "https://example.com/" + strconv.Itoa(i)
	}

	reqBody := map[string]interface{}{
		"name":           "TestAgent",
		"external_links": links,
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/v1/agents/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.RegisterAgent(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for too many external links, got %d: %s", rr.Code, rr.Body.String())
	}
}

// TestUpdateAgent_WithEmailAndExternalLinks tests that agents can update email and external_links via API key.
func TestUpdateAgent_WithEmailAndExternalLinks(t *testing.T) {
	repo := NewMockAgentRepository()
	handler := NewAgentsHandler(repo, "test-jwt-secret")

	// Create agent with API key
	apiKey := "sak_testapikey123"
	apiKeyHash, _ := auth.HashAPIKey(apiKey)
	repo.agents["my_agent"] = &models.Agent{
		ID:          "my_agent",
		DisplayName: "Test Agent",
		APIKeyHash:  apiKeyHash,
		Email:       "",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// Update with email and external_links
	reqBody := map[string]interface{}{
		"email":          "updated@example.com",
		"external_links": []string{"https://moltbook.ai/my_agent"},
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPatch, "/v1/agents/my_agent", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = addAgentToContext(req, repo.agents["my_agent"])

	rr := httptest.NewRecorder()
	handler.UpdateAgent(rr, req, "my_agent")

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp GetAgentResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Verify email and external_links are updated
	if resp.Data.Agent.Email != "updated@example.com" {
		t.Errorf("expected email 'updated@example.com', got '%s'", resp.Data.Agent.Email)
	}
	if len(resp.Data.Agent.ExternalLinks) != 1 {
		t.Errorf("expected 1 external link, got %d", len(resp.Data.Agent.ExternalLinks))
	}
	if len(resp.Data.Agent.ExternalLinks) > 0 && resp.Data.Agent.ExternalLinks[0] != "https://moltbook.ai/my_agent" {
		t.Errorf("expected link 'https://moltbook.ai/my_agent', got '%s'", resp.Data.Agent.ExternalLinks[0])
	}
}

// TestUpdateAgent_ClearsEmailWhenEmpty tests that setting email to empty string clears it.
func TestUpdateAgent_ClearsEmailWhenEmpty(t *testing.T) {
	repo := NewMockAgentRepository()
	handler := NewAgentsHandler(repo, "test-jwt-secret")

	humanID := "user-123"
	repo.agents["my_agent"] = &models.Agent{
		ID:          "my_agent",
		DisplayName: "Test Agent",
		HumanID:     &humanID,
		Email:       "existing@example.com",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// Update with empty email (using map to test field handling)
	reqBody := map[string]interface{}{
		"email": "",
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
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Verify email is cleared
	if resp.Data.Agent.Email != "" {
		t.Errorf("expected email to be cleared, got '%s'", resp.Data.Agent.Email)
	}
}

// TestUpdateAgent_EmailValidation tests that email is validated on update.
func TestUpdateAgent_EmailValidation(t *testing.T) {
	repo := NewMockAgentRepository()
	handler := NewAgentsHandler(repo, "test-jwt-secret")

	humanID := "user-123"
	repo.agents["my_agent"] = &models.Agent{
		ID:          "my_agent",
		DisplayName: "Test Agent",
		HumanID:     &humanID,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// Email too long (> 255 chars)
	longEmail := strings.Repeat("a", 250) + "@example.com"
	reqBody := map[string]interface{}{
		"email": longEmail,
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPatch, "/v1/agents/my_agent", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = addJWTClaimsToContext(req, "user-123", "user@example.com", "user")

	rr := httptest.NewRecorder()
	handler.UpdateAgent(rr, req, "my_agent")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for email too long, got %d: %s", rr.Code, rr.Body.String())
	}
}

// TestUpdateAgent_ExternalLinksValidation tests that external_links are validated on update.
func TestUpdateAgent_ExternalLinksValidation(t *testing.T) {
	repo := NewMockAgentRepository()
	handler := NewAgentsHandler(repo, "test-jwt-secret")

	humanID := "user-123"
	repo.agents["my_agent"] = &models.Agent{
		ID:          "my_agent",
		DisplayName: "Test Agent",
		HumanID:     &humanID,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// Too many external links (> 10)
	links := make([]string, 11)
	for i := range links {
		links[i] = "https://example.com/" + strconv.Itoa(i)
	}

	reqBody := map[string]interface{}{
		"external_links": links,
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPatch, "/v1/agents/my_agent", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = addJWTClaimsToContext(req, "user-123", "user@example.com", "user")

	rr := httptest.NewRecorder()
	handler.UpdateAgent(rr, req, "my_agent")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for too many external links, got %d: %s", rr.Code, rr.Body.String())
	}
}
