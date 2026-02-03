package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/auth"
	"github.com/fcavalcantirj/solvr/internal/models"
)

// MockAgentRepository implements AgentRepositoryInterface for testing.
type MockAgentRepository struct {
	agents       map[string]*models.Agent
	createCalled bool
	createErr    error
	findErr      error
	updateErr    error
}

func NewMockAgentRepository() *MockAgentRepository {
	return &MockAgentRepository{
		agents: make(map[string]*models.Agent),
	}
}

func (m *MockAgentRepository) Create(ctx context.Context, agent *models.Agent) error {
	m.createCalled = true
	if m.createErr != nil {
		return m.createErr
	}
	// Check for duplicate
	if _, exists := m.agents[agent.ID]; exists {
		return ErrDuplicateAgentID
	}
	m.agents[agent.ID] = agent
	return nil
}

func (m *MockAgentRepository) FindByID(ctx context.Context, id string) (*models.Agent, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	agent, exists := m.agents[id]
	if !exists {
		return nil, ErrAgentNotFound
	}
	return agent, nil
}

func (m *MockAgentRepository) FindByName(ctx context.Context, name string) (*models.Agent, error) {
	for _, agent := range m.agents {
		if agent.DisplayName == name {
			return agent, nil
		}
	}
	return nil, ErrAgentNotFound
}

func (m *MockAgentRepository) Update(ctx context.Context, agent *models.Agent) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	if _, exists := m.agents[agent.ID]; !exists {
		return ErrAgentNotFound
	}
	m.agents[agent.ID] = agent
	return nil
}

func (m *MockAgentRepository) GetAgentStats(ctx context.Context, agentID string) (*models.AgentStats, error) {
	return &models.AgentStats{
		ProblemsSolved:      5,
		ProblemsContributed: 10,
		QuestionsAsked:      3,
		QuestionsAnswered:   15,
		AnswersAccepted:     8,
		IdeasPosted:         2,
		ResponsesGiven:      20,
		UpvotesReceived:     100,
		Reputation:          1250,
	}, nil
}

func (m *MockAgentRepository) UpdateAPIKeyHash(ctx context.Context, agentID, hash string) error {
	agent, exists := m.agents[agentID]
	if !exists {
		return ErrAgentNotFound
	}
	agent.APIKeyHash = hash
	return nil
}

func (m *MockAgentRepository) RevokeAPIKey(ctx context.Context, agentID string) error {
	agent, exists := m.agents[agentID]
	if !exists {
		return ErrAgentNotFound
	}
	agent.APIKeyHash = ""
	return nil
}

func (m *MockAgentRepository) GetActivity(ctx context.Context, agentID string, page, perPage int) ([]models.ActivityItem, int, error) {
	if _, exists := m.agents[agentID]; !exists {
		return nil, 0, ErrAgentNotFound
	}
	// Return empty by default for basic mock
	return []models.ActivityItem{}, 0, nil
}

// LinkHuman links an agent to a human (AGENT-LINKING requirement).
func (m *MockAgentRepository) LinkHuman(ctx context.Context, agentID, humanID string) error {
	agent, exists := m.agents[agentID]
	if !exists {
		return ErrAgentNotFound
	}
	agent.HumanID = &humanID
	now := time.Now()
	agent.HumanClaimedAt = &now
	return nil
}

// AddKarma adds karma points to an agent (AGENT-LINKING requirement).
func (m *MockAgentRepository) AddKarma(ctx context.Context, agentID string, amount int) error {
	agent, exists := m.agents[agentID]
	if !exists {
		return ErrAgentNotFound
	}
	agent.Karma += amount
	return nil
}

// GrantHumanBackedBadge grants the Human-Backed badge to an agent (AGENT-LINKING requirement).
func (m *MockAgentRepository) GrantHumanBackedBadge(ctx context.Context, agentID string) error {
	agent, exists := m.agents[agentID]
	if !exists {
		return ErrAgentNotFound
	}
	agent.HasHumanBackedBadge = true
	return nil
}

// Helper to add JWT claims to request context
func addJWTClaimsToContext(r *http.Request, userID, email, role string) *http.Request {
	claims := &auth.Claims{
		UserID:    userID,
		Email:     email,
		Role:      role,
		ExpiresAt: time.Now().Add(15 * time.Minute),
		IssuedAt:  time.Now(),
	}
	ctx := auth.ContextWithClaims(r.Context(), claims)
	return r.WithContext(ctx)
}

func TestCreateAgent_Success(t *testing.T) {
	repo := NewMockAgentRepository()
	handler := NewAgentsHandler(repo, "test-jwt-secret")

	reqBody := CreateAgentRequest{
		ID:          "my_agent",
		DisplayName: "My Agent",
		Bio:         "A helpful AI assistant",
		Specialties: []string{"golang", "testing"},
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/v1/agents", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = addJWTClaimsToContext(req, "user-123", "user@example.com", "user")

	rr := httptest.NewRecorder()
	handler.CreateAgent(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp CreateAgentResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Data.Agent.ID != "my_agent" {
		t.Errorf("expected agent ID 'my_agent', got '%s'", resp.Data.Agent.ID)
	}
	if resp.Data.Agent.DisplayName != "My Agent" {
		t.Errorf("expected display name 'My Agent', got '%s'", resp.Data.Agent.DisplayName)
	}
	if resp.Data.APIKey == "" {
		t.Error("expected API key in response, got empty")
	}
	if len(resp.Data.APIKey) < 10 || resp.Data.APIKey[:6] != "solvr_" {
		t.Errorf("expected API key with solvr_ prefix, got '%s'", resp.Data.APIKey)
	}
	if resp.Data.Agent.HumanID == nil || *resp.Data.Agent.HumanID != "user-123" {
		t.Error("expected human_id to be set from JWT claims")
	}
}

func TestCreateAgent_NoAuth(t *testing.T) {
	repo := NewMockAgentRepository()
	handler := NewAgentsHandler(repo, "test-jwt-secret")

	reqBody := CreateAgentRequest{
		ID:          "my_agent",
		DisplayName: "My Agent",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/v1/agents", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	// No JWT claims added

	rr := httptest.NewRecorder()
	handler.CreateAgent(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rr.Code)
	}
}

func TestCreateAgent_InvalidIDFormat(t *testing.T) {
	repo := NewMockAgentRepository()
	handler := NewAgentsHandler(repo, "test-jwt-secret")

	testCases := []struct {
		name string
		id   string
	}{
		{"contains space", "my agent"},
		{"contains hyphen", "my-agent"},
		{"contains special char", "my@agent"},
		{"contains dot", "my.agent"},
		{"empty", ""},
		{"too long", "this_is_a_very_long_agent_id_that_exceeds_fifty_characters_limit"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			reqBody := CreateAgentRequest{
				ID:          tc.id,
				DisplayName: "My Agent",
			}
			body, _ := json.Marshal(reqBody)

			req := httptest.NewRequest(http.MethodPost, "/v1/agents", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req = addJWTClaimsToContext(req, "user-123", "user@example.com", "user")

			rr := httptest.NewRecorder()
			handler.CreateAgent(rr, req)

			if rr.Code != http.StatusBadRequest {
				t.Errorf("expected status 400, got %d: %s", rr.Code, rr.Body.String())
			}

			var errResp map[string]interface{}
			json.NewDecoder(rr.Body).Decode(&errResp)
			errorObj := errResp["error"].(map[string]interface{})
			if errorObj["code"] != "INVALID_ID" && errorObj["code"] != "VALIDATION_ERROR" {
				t.Errorf("expected error code INVALID_ID or VALIDATION_ERROR, got %s", errorObj["code"])
			}
		})
	}
}

func TestCreateAgent_ValidIDFormats(t *testing.T) {
	testCases := []string{
		"myagent",
		"my_agent",
		"MyAgent123",
		"agent_123_test",
		"a",
		"a_1",
	}

	for _, id := range testCases {
		t.Run(id, func(t *testing.T) {
			repo := NewMockAgentRepository()
			handler := NewAgentsHandler(repo, "test-jwt-secret")

			reqBody := CreateAgentRequest{
				ID:          id,
				DisplayName: "My Agent",
			}
			body, _ := json.Marshal(reqBody)

			req := httptest.NewRequest(http.MethodPost, "/v1/agents", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req = addJWTClaimsToContext(req, "user-123", "user@example.com", "user")

			rr := httptest.NewRecorder()
			handler.CreateAgent(rr, req)

			if rr.Code != http.StatusCreated {
				t.Errorf("expected status 201 for valid ID '%s', got %d: %s", id, rr.Code, rr.Body.String())
			}
		})
	}
}

func TestCreateAgent_DuplicateID(t *testing.T) {
	repo := NewMockAgentRepository()
	handler := NewAgentsHandler(repo, "test-jwt-secret")

	// Create first agent
	repo.agents["my_agent"] = &models.Agent{
		ID:          "my_agent",
		DisplayName: "Existing Agent",
	}

	reqBody := CreateAgentRequest{
		ID:          "my_agent",
		DisplayName: "New Agent",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/v1/agents", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = addJWTClaimsToContext(req, "user-123", "user@example.com", "user")

	rr := httptest.NewRecorder()
	handler.CreateAgent(rr, req)

	if rr.Code != http.StatusConflict {
		t.Errorf("expected status 409, got %d: %s", rr.Code, rr.Body.String())
	}

	var errResp map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&errResp)
	errorObj := errResp["error"].(map[string]interface{})
	if errorObj["code"] != "DUPLICATE_ID" {
		t.Errorf("expected error code DUPLICATE_ID, got %s", errorObj["code"])
	}
}

func TestCreateAgent_MissingDisplayName(t *testing.T) {
	repo := NewMockAgentRepository()
	handler := NewAgentsHandler(repo, "test-jwt-secret")

	reqBody := CreateAgentRequest{
		ID: "my_agent",
		// DisplayName missing
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/v1/agents", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = addJWTClaimsToContext(req, "user-123", "user@example.com", "user")

	rr := httptest.NewRecorder()
	handler.CreateAgent(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestCreateAgent_InvalidJSON(t *testing.T) {
	repo := NewMockAgentRepository()
	handler := NewAgentsHandler(repo, "test-jwt-secret")

	req := httptest.NewRequest(http.MethodPost, "/v1/agents", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	req = addJWTClaimsToContext(req, "user-123", "user@example.com", "user")

	rr := httptest.NewRecorder()
	handler.CreateAgent(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}
}

func TestGetAgent_Success(t *testing.T) {
	repo := NewMockAgentRepository()
	handler := NewAgentsHandler(repo, "test-jwt-secret")

	humanID := "user-123"
	repo.agents["test_agent"] = &models.Agent{
		ID:          "test_agent",
		DisplayName: "Test Agent",
		HumanID:     &humanID,
		Bio:         "A test agent",
		Specialties: []string{"testing"},
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/agents/test_agent", nil)
	rr := httptest.NewRecorder()

	handler.GetAgent(rr, req, "test_agent")

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp GetAgentResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Data.Agent.ID != "test_agent" {
		t.Errorf("expected agent ID 'test_agent', got '%s'", resp.Data.Agent.ID)
	}
	if resp.Data.Stats.Reputation != 1250 {
		t.Errorf("expected reputation 1250, got %d", resp.Data.Stats.Reputation)
	}
}

func TestGetAgent_NotFound(t *testing.T) {
	repo := NewMockAgentRepository()
	handler := NewAgentsHandler(repo, "test-jwt-secret")

	req := httptest.NewRequest(http.MethodGet, "/v1/agents/nonexistent", nil)
	rr := httptest.NewRecorder()

	handler.GetAgent(rr, req, "nonexistent")

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", rr.Code)
	}

	var errResp map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&errResp)
	errorObj := errResp["error"].(map[string]interface{})
	if errorObj["code"] != "NOT_FOUND" {
		t.Errorf("expected error code NOT_FOUND, got %s", errorObj["code"])
	}
}

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

// TestGetAgent_NeverReturnsAPIKey verifies that GET /v1/agents/:id never returns api_key or api_key_hash.
// This is a CRITICAL security requirement per SPEC.md Part 5.6.
func TestGetAgent_NeverReturnsAPIKey(t *testing.T) {
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

	req := httptest.NewRequest(http.MethodGet, "/v1/agents/my_agent", nil)
	rr := httptest.NewRecorder()
	handler.GetAgent(rr, req, "my_agent")

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
func strPtr(s string) *string {
	return &s
}
