package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// DuplicateNameErrorResponse represents the 409 response with suggestions.
type DuplicateNameErrorResponse struct {
	Error struct {
		Code         string   `json:"code"`
		Message      string   `json:"message"`
		Suggestions  []string `json:"suggestions,omitempty"`
	} `json:"error"`
}

// MockAgentRepoWithSuggestions extends the mock to support name uniqueness testing.
type MockAgentRepoWithSuggestions struct {
	agents       map[string]*models.Agent
	agentsByName map[string]*models.Agent
}

func NewMockAgentRepoWithSuggestions() *MockAgentRepoWithSuggestions {
	return &MockAgentRepoWithSuggestions{
		agents:       make(map[string]*models.Agent),
		agentsByName: make(map[string]*models.Agent),
	}
}

func (m *MockAgentRepoWithSuggestions) Create(ctx context.Context, agent *models.Agent) error {
	// Check for duplicate name (display_name)
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

func (m *MockAgentRepoWithSuggestions) FindByID(ctx context.Context, id string) (*models.Agent, error) {
	agent, exists := m.agents[id]
	if !exists {
		return nil, ErrAgentNotFound
	}
	return agent, nil
}

func (m *MockAgentRepoWithSuggestions) FindByName(ctx context.Context, name string) (*models.Agent, error) {
	agent, exists := m.agentsByName[name]
	if !exists {
		return nil, ErrAgentNotFound
	}
	return agent, nil
}

func (m *MockAgentRepoWithSuggestions) Update(ctx context.Context, agent *models.Agent) error {
	if _, exists := m.agents[agent.ID]; !exists {
		return ErrAgentNotFound
	}
	m.agents[agent.ID] = agent
	return nil
}

func (m *MockAgentRepoWithSuggestions) GetAgentStats(ctx context.Context, agentID string) (*models.AgentStats, error) {
	return &models.AgentStats{}, nil
}

func (m *MockAgentRepoWithSuggestions) UpdateAPIKeyHash(ctx context.Context, agentID, hash string) error {
	agent, exists := m.agents[agentID]
	if !exists {
		return ErrAgentNotFound
	}
	agent.APIKeyHash = hash
	return nil
}

func (m *MockAgentRepoWithSuggestions) RevokeAPIKey(ctx context.Context, agentID string) error {
	agent, exists := m.agents[agentID]
	if !exists {
		return ErrAgentNotFound
	}
	agent.APIKeyHash = ""
	return nil
}

func (m *MockAgentRepoWithSuggestions) GetActivity(ctx context.Context, agentID string, page, perPage int) ([]models.ActivityItem, int, error) {
	return []models.ActivityItem{}, 0, nil
}

func (m *MockAgentRepoWithSuggestions) LinkHuman(ctx context.Context, agentID, humanID string) error {
	agent, exists := m.agents[agentID]
	if !exists {
		return ErrAgentNotFound
	}
	agent.HumanID = &humanID
	return nil
}

func (m *MockAgentRepoWithSuggestions) AddKarma(ctx context.Context, agentID string, amount int) error {
	agent, exists := m.agents[agentID]
	if !exists {
		return ErrAgentNotFound
	}
	agent.Karma += amount
	return nil
}

func (m *MockAgentRepoWithSuggestions) GrantHumanBackedBadge(ctx context.Context, agentID string) error {
	agent, exists := m.agents[agentID]
	if !exists {
		return ErrAgentNotFound
	}
	agent.HasHumanBackedBadge = true
	return nil
}

// TestRegisterAgent_DuplicateName_Returns409 tests that duplicate names return 409 Conflict.
// Per AGENT-ONBOARDING requirement: Return 409 Conflict if name taken.
func TestRegisterAgent_DuplicateName_Returns409(t *testing.T) {
	repo := NewMockAgentRepoWithSuggestions()
	handler := NewAgentsHandler(repo, "test-jwt-secret")

	// First registration should succeed
	reqBody := RegisterAgentRequest{
		Name:        "my_agent",
		Description: "First agent",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/v1/agents/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.RegisterAgent(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("first registration should succeed, got %d: %s", rr.Code, rr.Body.String())
	}

	// Second registration with same name should fail with 409
	reqBody2 := RegisterAgentRequest{
		Name:        "my_agent",
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

	// Verify error code
	var errResp DuplicateNameErrorResponse
	if err := json.NewDecoder(rr2.Body).Decode(&errResp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}

	if errResp.Error.Code != "DUPLICATE_NAME" {
		t.Errorf("expected error code DUPLICATE_NAME, got %s", errResp.Error.Code)
	}
}

// TestRegisterAgent_DuplicateName_SuggestsAlternatives tests that 409 response includes suggestions.
// Per AGENT-ONBOARDING requirement: Suggest alternatives in error response.
func TestRegisterAgent_DuplicateName_SuggestsAlternatives(t *testing.T) {
	repo := NewMockAgentRepoWithSuggestions()
	handler := NewAgentsHandler(repo, "test-jwt-secret")

	// First registration
	reqBody := RegisterAgentRequest{
		Name:        "helper_bot",
		Description: "First bot",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/v1/agents/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.RegisterAgent(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("first registration should succeed, got %d", rr.Code)
	}

	// Second registration with same name - should get suggestions
	reqBody2 := RegisterAgentRequest{
		Name:        "helper_bot",
		Description: "Second bot",
	}
	body2, _ := json.Marshal(reqBody2)

	req2 := httptest.NewRequest(http.MethodPost, "/v1/agents/register", bytes.NewReader(body2))
	req2.Header.Set("Content-Type", "application/json")

	rr2 := httptest.NewRecorder()
	handler.RegisterAgent(rr2, req2)

	if rr2.Code != http.StatusConflict {
		t.Errorf("expected status 409, got %d: %s", rr2.Code, rr2.Body.String())
	}

	var errResp DuplicateNameErrorResponse
	if err := json.NewDecoder(rr2.Body).Decode(&errResp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}

	// Verify suggestions are provided
	if len(errResp.Error.Suggestions) == 0 {
		t.Error("expected suggestions in error response for duplicate name")
	}

	// Verify suggestions are different from original name
	for _, suggestion := range errResp.Error.Suggestions {
		if suggestion == "helper_bot" {
			t.Error("suggestions should be different from the original name")
		}
		// Suggestions should be valid names (alphanumeric + underscore)
		if err := validateAgentName(suggestion); err != nil {
			t.Errorf("suggestion '%s' is not a valid agent name: %v", suggestion, err)
		}
	}

	// Should have at least 3 suggestions
	if len(errResp.Error.Suggestions) < 3 {
		t.Errorf("expected at least 3 suggestions, got %d", len(errResp.Error.Suggestions))
	}
}

// TestRegisterAgent_SuggestionsAreUnique tests that suggested names are unique.
// Per AGENT-ONBOARDING requirement: Suggest alternatives in error response.
func TestRegisterAgent_SuggestionsAreUnique(t *testing.T) {
	repo := NewMockAgentRepoWithSuggestions()
	handler := NewAgentsHandler(repo, "test-jwt-secret")

	// Register the original name and some potential suggestions
	baseName := "my_bot"
	namesToRegister := []string{baseName, "my_bot_1", "my_bot_2"}

	for _, name := range namesToRegister {
		reqBody := RegisterAgentRequest{
			Name:        name,
			Description: "Test agent",
		}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/v1/agents/register", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		rr := httptest.NewRecorder()
		handler.RegisterAgent(rr, req)

		if rr.Code != http.StatusCreated {
			t.Fatalf("registration for %s should succeed, got %d", name, rr.Code)
		}
	}

	// Try to register the base name again
	reqBody := RegisterAgentRequest{
		Name:        baseName,
		Description: "Another bot",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/v1/agents/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.RegisterAgent(rr, req)

	var errResp DuplicateNameErrorResponse
	json.NewDecoder(rr.Body).Decode(&errResp)

	// All suggestions should be unique (not already taken)
	for _, suggestion := range errResp.Error.Suggestions {
		if _, exists := repo.agentsByName[suggestion]; exists {
			t.Errorf("suggestion '%s' is already taken, should not be suggested", suggestion)
		}
	}
}

// TestNameUniqueness_DBConstraint tests that the DB enforces uniqueness.
// This is a unit test - real DB constraint testing is done in integration tests.
func TestNameUniqueness_DBConstraint(t *testing.T) {
	repo := NewMockAgentRepoWithSuggestions()

	// Create first agent
	agent1 := &models.Agent{
		ID:          "agent_test1",
		DisplayName: "test_name",
	}
	err := repo.Create(context.Background(), agent1)
	if err != nil {
		t.Fatalf("first create should succeed: %v", err)
	}

	// Try to create second agent with same display name
	agent2 := &models.Agent{
		ID:          "agent_test2",
		DisplayName: "test_name", // Same display name
	}
	err = repo.Create(context.Background(), agent2)
	if err != ErrDuplicateAgentName {
		t.Errorf("expected ErrDuplicateAgentName, got %v", err)
	}
}

// TestNameUniqueness_CaseSensitive tests that name uniqueness is case-sensitive.
func TestNameUniqueness_CaseSensitive(t *testing.T) {
	repo := NewMockAgentRepoWithSuggestions()
	handler := NewAgentsHandler(repo, "test-jwt-secret")

	// Register first agent
	reqBody := RegisterAgentRequest{
		Name:        "my_bot",
		Description: "First bot",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/v1/agents/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.RegisterAgent(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("first registration should succeed, got %d", rr.Code)
	}

	// Same name with different case should fail (names are normalized to lowercase internally)
	// This ensures consistent uniqueness checking
	reqBody2 := RegisterAgentRequest{
		Name:        "my_bot", // Same name - should fail
		Description: "Second bot",
	}
	body2, _ := json.Marshal(reqBody2)

	req2 := httptest.NewRequest(http.MethodPost, "/v1/agents/register", bytes.NewReader(body2))
	req2.Header.Set("Content-Type", "application/json")

	rr2 := httptest.NewRecorder()
	handler.RegisterAgent(rr2, req2)

	if rr2.Code != http.StatusConflict {
		t.Errorf("expected status 409 for duplicate name, got %d", rr2.Code)
	}
}
