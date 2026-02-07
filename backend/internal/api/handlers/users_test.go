package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// ============================================================================
// Mock repositories for users handler tests
// ============================================================================

// MockUsersAgentRepository implements the agent repository interface for testing.
type MockUsersAgentRepository struct {
	agents map[string][]*models.Agent // humanID -> agents
}

func NewMockUsersAgentRepository() *MockUsersAgentRepository {
	return &MockUsersAgentRepository{
		agents: make(map[string][]*models.Agent),
	}
}

func (m *MockUsersAgentRepository) FindByHumanID(ctx context.Context, humanID string) ([]*models.Agent, error) {
	agents, ok := m.agents[humanID]
	if !ok {
		return []*models.Agent{}, nil
	}
	return agents, nil
}

// ============================================================================
// Tests for GET /v1/users/{id}/agents endpoint (prd-v4 requirement)
// ============================================================================

// TestGetUserAgents_Success tests that the endpoint returns agents for a user.
// Per prd-v4: "Returns correct agents for user with claimed agents"
func TestGetUserAgents_Success(t *testing.T) {
	agentRepo := NewMockUsersAgentRepository()
	handler := NewUsersHandler(nil, nil)
	handler.SetAgentRepository(agentRepo)

	humanID := "user-123"
	agentRepo.agents[humanID] = []*models.Agent{
		{
			ID:                  "agent_one",
			DisplayName:         "Agent One",
			HumanID:             &humanID,
			Bio:                 "First agent",
			Model:               "claude-opus-4",
			Karma:               100,
			HasHumanBackedBadge: true,
			CreatedAt:           time.Now(),
			UpdatedAt:           time.Now(),
		},
		{
			ID:                  "agent_two",
			DisplayName:         "Agent Two",
			HumanID:             &humanID,
			Bio:                 "Second agent",
			Model:               "gpt-4",
			Karma:               50,
			HasHumanBackedBadge: true,
			CreatedAt:           time.Now(),
			UpdatedAt:           time.Now(),
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/users/user-123/agents", nil)
	rr := httptest.NewRecorder()

	// Set up chi context with URL param
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", humanID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	handler.GetUserAgents(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Data []models.Agent `json:"data"`
		Meta struct {
			Total   int `json:"total"`
			Page    int `json:"page"`
			PerPage int `json:"per_page"`
		} `json:"meta"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Verify we got both agents
	if len(resp.Data) != 2 {
		t.Errorf("expected 2 agents, got %d", len(resp.Data))
	}

	// Verify meta
	if resp.Meta.Total != 2 {
		t.Errorf("expected total 2, got %d", resp.Meta.Total)
	}

	// Verify first agent data
	if resp.Data[0].ID != "agent_one" {
		t.Errorf("expected first agent ID 'agent_one', got '%s'", resp.Data[0].ID)
	}

	// Verify api_key_hash is NOT included (security)
	for _, agent := range resp.Data {
		if agent.APIKeyHash != "" {
			t.Error("SECURITY VIOLATION: api_key_hash should not be included in response")
		}
	}
}

// TestGetUserAgents_Empty tests that the endpoint returns empty array for users with no agents.
// Per prd-v4: "Returns empty array for users with no agents"
func TestGetUserAgents_Empty(t *testing.T) {
	agentRepo := NewMockUsersAgentRepository()
	handler := NewUsersHandler(nil, nil)
	handler.SetAgentRepository(agentRepo)

	humanID := "user-no-agents"
	// No agents added for this user

	req := httptest.NewRequest(http.MethodGet, "/v1/users/user-no-agents/agents", nil)
	rr := httptest.NewRecorder()

	// Set up chi context with URL param
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", humanID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	handler.GetUserAgents(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Data []models.Agent `json:"data"`
		Meta struct {
			Total int `json:"total"`
		} `json:"meta"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Verify empty array (not null)
	if resp.Data == nil {
		t.Error("expected empty array [], got null")
	}

	if len(resp.Data) != 0 {
		t.Errorf("expected 0 agents, got %d", len(resp.Data))
	}

	if resp.Meta.Total != 0 {
		t.Errorf("expected total 0, got %d", resp.Meta.Total)
	}
}

// TestGetUserAgents_MissingUserID tests that the endpoint handles missing user ID.
func TestGetUserAgents_MissingUserID(t *testing.T) {
	agentRepo := NewMockUsersAgentRepository()
	handler := NewUsersHandler(nil, nil)
	handler.SetAgentRepository(agentRepo)

	req := httptest.NewRequest(http.MethodGet, "/v1/users//agents", nil)
	rr := httptest.NewRecorder()

	// Set up chi context with empty URL param
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	handler.GetUserAgents(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d: %s", rr.Code, rr.Body.String())
	}
}
