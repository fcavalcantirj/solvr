package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// MockAgentRepositoryWithNameLookup extends the mock to support name lookups.
type MockAgentRepositoryWithNameLookup struct {
	*MockAgentRepository
	agentsByName map[string]*models.Agent
}

func NewMockAgentRepositoryWithNameLookup() *MockAgentRepositoryWithNameLookup {
	return &MockAgentRepositoryWithNameLookup{
		MockAgentRepository: NewMockAgentRepository(),
		agentsByName:        make(map[string]*models.Agent),
	}
}

func (m *MockAgentRepositoryWithNameLookup) FindByName(ctx context.Context, name string) (*models.Agent, error) {
	agent, exists := m.agentsByName[name]
	if !exists {
		return nil, ErrAgentNotFound
	}
	return agent, nil
}

func (m *MockAgentRepositoryWithNameLookup) Create(ctx context.Context, agent *models.Agent) error {
	// Check for duplicate name
	if _, exists := m.agentsByName[agent.DisplayName]; exists {
		return ErrDuplicateAgentName
	}
	// Check for duplicate ID
	if _, exists := m.agents[agent.ID]; exists {
		return ErrDuplicateAgentID
	}
	m.agents[agent.ID] = agent
	m.agentsByName[agent.DisplayName] = agent
	return nil
}

// TestRegisterAgent_Success tests successful agent self-registration.
// Per AGENT-ONBOARDING requirement: agents can self-register without human auth.
func TestRegisterAgent_Success(t *testing.T) {
	repo := NewMockAgentRepositoryWithNameLookup()
	handler := NewAgentsHandler(repo, "test-jwt-secret")

	reqBody := RegisterAgentRequest{
		Name:        "my_test_agent",
		Description: "A helpful AI assistant",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/v1/agents/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	// NOTE: No JWT auth required for self-registration

	rr := httptest.NewRecorder()
	handler.RegisterAgent(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp RegisterAgentResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Verify success flag
	if !resp.Success {
		t.Error("expected success=true")
	}

	// Verify agent data
	if resp.Agent.DisplayName != "my_test_agent" {
		t.Errorf("expected display name 'my_test_agent', got '%s'", resp.Agent.DisplayName)
	}

	// Verify agent ID was generated
	if resp.Agent.ID == "" {
		t.Error("expected agent ID to be generated")
	}

	// Verify API key was returned
	if resp.APIKey == "" {
		t.Error("expected API key in response, got empty")
	}
	if len(resp.APIKey) < 10 || !strings.HasPrefix(resp.APIKey, "solvr_") {
		t.Errorf("expected API key with solvr_ prefix, got '%s'", resp.APIKey)
	}

	// Verify agent has no human_id (self-registered)
	if resp.Agent.HumanID != nil {
		t.Error("expected human_id to be nil for self-registered agent")
	}

	// Verify important warning is included
	if resp.Important == "" {
		t.Error("expected 'important' warning about saving API key")
	}
}

// TestRegisterAgent_NameValidation tests name validation rules.
// Per AGENT-ONBOARDING requirement: Name must be 3-30 chars, alphanumeric + underscore.
func TestRegisterAgent_NameValidation(t *testing.T) {
	repo := NewMockAgentRepositoryWithNameLookup()
	handler := NewAgentsHandler(repo, "test-jwt-secret")

	testCases := []struct {
		name        string
		agentName   string
		expectError bool
		errorCode   string
	}{
		{"valid name", "my_agent", false, ""},
		{"valid with numbers", "agent123", false, ""},
		{"valid underscore", "my_agent_2", false, ""},
		{"too short", "ab", true, "VALIDATION_ERROR"},
		{"too long", "this_is_a_very_long_name_that_exceeds_thirty_characters", true, "VALIDATION_ERROR"},
		{"contains space", "my agent", true, "VALIDATION_ERROR"},
		{"contains hyphen", "my-agent", true, "VALIDATION_ERROR"},
		{"contains special char", "my@agent", true, "VALIDATION_ERROR"},
		{"empty", "", true, "VALIDATION_ERROR"},
		{"starts with number", "123agent", false, ""}, // Numbers are allowed
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			reqBody := RegisterAgentRequest{
				Name:        tc.agentName,
				Description: "A test agent",
			}
			body, _ := json.Marshal(reqBody)

			req := httptest.NewRequest(http.MethodPost, "/v1/agents/register", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()
			handler.RegisterAgent(rr, req)

			if tc.expectError {
				if rr.Code != http.StatusBadRequest {
					t.Errorf("expected status 400, got %d: %s", rr.Code, rr.Body.String())
				}
				if tc.errorCode != "" && !strings.Contains(rr.Body.String(), tc.errorCode) {
					t.Errorf("expected error code %s in response", tc.errorCode)
				}
			} else {
				if rr.Code != http.StatusCreated {
					t.Errorf("expected status 201, got %d: %s", rr.Code, rr.Body.String())
				}
			}
		})
	}
}

// TestRegisterAgent_DescriptionValidation tests description validation.
// Per AGENT-ONBOARDING requirement: Description max 500 chars.
func TestRegisterAgent_DescriptionValidation(t *testing.T) {
	repo := NewMockAgentRepositoryWithNameLookup()
	handler := NewAgentsHandler(repo, "test-jwt-secret")

	// Test description too long
	longDesc := strings.Repeat("a", 501)
	reqBody := RegisterAgentRequest{
		Name:        "my_agent",
		Description: longDesc,
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/v1/agents/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.RegisterAgent(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for too long description, got %d", rr.Code)
	}
}

// TestRegisterAgent_DuplicateName tests that duplicate names are rejected.
// Per AGENT-ONBOARDING requirement: Name must be unique.
func TestRegisterAgent_DuplicateName(t *testing.T) {
	repo := NewMockAgentRepositoryWithNameLookup()
	handler := NewAgentsHandler(repo, "test-jwt-secret")

	// First registration should succeed
	reqBody := RegisterAgentRequest{
		Name:        "unique_agent",
		Description: "First agent",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/v1/agents/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.RegisterAgent(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("first registration should succeed, got %d", rr.Code)
	}

	// Second registration with same name should fail
	reqBody2 := RegisterAgentRequest{
		Name:        "unique_agent",
		Description: "Second agent",
	}
	body2, _ := json.Marshal(reqBody2)

	req2 := httptest.NewRequest(http.MethodPost, "/v1/agents/register", bytes.NewReader(body2))
	req2.Header.Set("Content-Type", "application/json")

	rr2 := httptest.NewRecorder()
	handler.RegisterAgent(rr2, req2)

	if rr2.Code != http.StatusConflict {
		t.Errorf("expected status 409 for duplicate name, got %d: %s", rr2.Code, rr2.Body.String())
	}
}

// TestRegisterAgent_APIKeyShownOnce tests that API key is shown only once.
// Per AGENT-ONBOARDING requirement: Return key ONCE (never retrievable).
func TestRegisterAgent_APIKeyShownOnce(t *testing.T) {
	repo := NewMockAgentRepositoryWithNameLookup()
	handler := NewAgentsHandler(repo, "test-jwt-secret")

	reqBody := RegisterAgentRequest{
		Name:        "api_key_test",
		Description: "Testing API key",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/v1/agents/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.RegisterAgent(rr, req)

	var resp RegisterAgentResponse
	json.NewDecoder(rr.Body).Decode(&resp)

	// Verify API key is returned
	if resp.APIKey == "" {
		t.Fatal("API key should be returned on registration")
	}

	// Verify the stored hash is NOT the raw key
	storedAgent := repo.agents[resp.Agent.ID]
	if storedAgent == nil {
		t.Fatal("agent should be stored in repository")
	}
	if storedAgent.APIKeyHash == resp.APIKey {
		t.Error("stored API key hash should NOT be the raw API key")
	}
	if storedAgent.APIKeyHash == "" {
		t.Error("API key hash should be stored")
	}
}

// TestRegisterAgent_InvalidJSON tests invalid JSON handling.
func TestRegisterAgent_InvalidJSON(t *testing.T) {
	repo := NewMockAgentRepositoryWithNameLookup()
	handler := NewAgentsHandler(repo, "test-jwt-secret")

	req := httptest.NewRequest(http.MethodPost, "/v1/agents/register", strings.NewReader("not valid json"))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.RegisterAgent(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for invalid JSON, got %d", rr.Code)
	}
}

// TestRegisterAgent_AgentActiveImmediately tests that agent is active immediately.
// Per AGENT-ONBOARDING requirement: Agent active immediately (no claim needed).
func TestRegisterAgent_AgentActiveImmediately(t *testing.T) {
	repo := NewMockAgentRepositoryWithNameLookup()
	handler := NewAgentsHandler(repo, "test-jwt-secret")

	reqBody := RegisterAgentRequest{
		Name:        "active_agent",
		Description: "Should be active immediately",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/v1/agents/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.RegisterAgent(rr, req)

	var resp RegisterAgentResponse
	json.NewDecoder(rr.Body).Decode(&resp)

	// Verify agent status is active
	if resp.Agent.Status != "active" {
		t.Errorf("expected agent status 'active', got '%s'", resp.Agent.Status)
	}
}

// ============================================================================
// Tests for karma bonus when model is set (prd-v4 requirement)
// ============================================================================

// TestRegisterAgent_ModelKarmaBonus tests that agents get +10 karma when model is set.
// Per prd-v4: Agent with model on registration starts with 10 karma.
func TestRegisterAgent_ModelKarmaBonus(t *testing.T) {
	repo := NewMockAgentRepositoryWithNameLookup()
	handler := NewAgentsHandler(repo, "test-jwt-secret")

	reqBody := RegisterAgentRequest{
		Name:        "karma_model_agent",
		Description: "Agent with model",
		Model:       "claude-opus-4",
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
	json.NewDecoder(rr.Body).Decode(&resp)

	// Verify agent has +10 karma for providing model
	if resp.Agent.Karma != 10 {
		t.Errorf("expected karma 10 for agent with model, got %d", resp.Agent.Karma)
	}

	// Verify model is set
	if resp.Agent.Model != "claude-opus-4" {
		t.Errorf("expected model 'claude-opus-4', got '%s'", resp.Agent.Model)
	}
}

// TestRegisterAgent_NoModelNoKarmaBonus tests that agents without model start with 0 karma.
// Per prd-v4: Agent without model starts with 0 karma.
func TestRegisterAgent_NoModelNoKarmaBonus(t *testing.T) {
	repo := NewMockAgentRepositoryWithNameLookup()
	handler := NewAgentsHandler(repo, "test-jwt-secret")

	reqBody := RegisterAgentRequest{
		Name:        "no_model_agent",
		Description: "Agent without model",
		// Model is not set
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
	json.NewDecoder(rr.Body).Decode(&resp)

	// Verify agent has 0 karma (no model bonus)
	if resp.Agent.Karma != 0 {
		t.Errorf("expected karma 0 for agent without model, got %d", resp.Agent.Karma)
	}
}
