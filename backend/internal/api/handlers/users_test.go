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

// ============================================================================
// Mock repository for ListUsers tests
// ============================================================================

// MockUserListRepository implements user repository interface for list tests.
type MockUserListRepository struct {
	users []models.UserListItem
	total int
}

func NewMockUserListRepository() *MockUserListRepository {
	return &MockUserListRepository{
		users: []models.UserListItem{},
	}
}

func (m *MockUserListRepository) List(ctx context.Context, opts models.PublicUserListOptions) ([]models.UserListItem, int, error) {
	// Apply offset/limit
	start := opts.Offset
	if start > len(m.users) {
		start = len(m.users)
	}
	end := start + opts.Limit
	if end > len(m.users) {
		end = len(m.users)
	}
	return m.users[start:end], m.total, nil
}

// ============================================================================
// Tests for GET /v1/users endpoint (prd-v4 requirement)
// ============================================================================

// TestListUsers_Success tests that the endpoint returns users list.
// Per prd-v4: Response includes id, username, display_name, avatar_url, karma, agents_count, created_at
func TestListUsers_Success(t *testing.T) {
	userListRepo := NewMockUserListRepository()
	handler := NewUsersHandler(nil, nil)
	handler.SetUserListRepository(userListRepo)

	now := time.Now()
	userListRepo.users = []models.UserListItem{
		{
			ID:          "user-1",
			Username:    "alice",
			DisplayName: "Alice Smith",
			AvatarURL:   "https://example.com/alice.jpg",
			Karma:       150,
			AgentsCount: 3,
			CreatedAt:   now,
		},
		{
			ID:          "user-2",
			Username:    "bob",
			DisplayName: "Bob Jones",
			AvatarURL:   "https://example.com/bob.jpg",
			Karma:       75,
			AgentsCount: 1,
			CreatedAt:   now.Add(-time.Hour),
		},
	}
	userListRepo.total = 2

	req := httptest.NewRequest(http.MethodGet, "/v1/users", nil)
	rr := httptest.NewRecorder()

	handler.ListUsers(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Data []models.UserListItem `json:"data"`
		Meta struct {
			Total  int `json:"total"`
			Limit  int `json:"limit"`
			Offset int `json:"offset"`
		} `json:"meta"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Verify we got both users
	if len(resp.Data) != 2 {
		t.Errorf("expected 2 users, got %d", len(resp.Data))
	}

	// Verify meta
	if resp.Meta.Total != 2 {
		t.Errorf("expected total 2, got %d", resp.Meta.Total)
	}

	// Verify first user data
	if resp.Data[0].ID != "user-1" {
		t.Errorf("expected first user ID 'user-1', got '%s'", resp.Data[0].ID)
	}

	if resp.Data[0].Username != "alice" {
		t.Errorf("expected username 'alice', got '%s'", resp.Data[0].Username)
	}

	if resp.Data[0].Karma != 150 {
		t.Errorf("expected karma 150, got %d", resp.Data[0].Karma)
	}

	if resp.Data[0].AgentsCount != 3 {
		t.Errorf("expected agents_count 3, got %d", resp.Data[0].AgentsCount)
	}
}

// TestListUsers_WithPagination tests limit and offset parameters.
// Per prd-v4: Support query params limit (default 20, max 100), offset
func TestListUsers_WithPagination(t *testing.T) {
	userListRepo := NewMockUserListRepository()
	handler := NewUsersHandler(nil, nil)
	handler.SetUserListRepository(userListRepo)

	// Create 5 users
	now := time.Now()
	for i := 1; i <= 5; i++ {
		userListRepo.users = append(userListRepo.users, models.UserListItem{
			ID:          "user-" + string(rune('0'+i)),
			Username:    "user" + string(rune('0'+i)),
			DisplayName: "User " + string(rune('0'+i)),
			Karma:       i * 10,
			AgentsCount: i,
			CreatedAt:   now,
		})
	}
	userListRepo.total = 5

	// Request with limit=2 and offset=1
	req := httptest.NewRequest(http.MethodGet, "/v1/users?limit=2&offset=1", nil)
	rr := httptest.NewRecorder()

	handler.ListUsers(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Data []models.UserListItem `json:"data"`
		Meta struct {
			Total  int `json:"total"`
			Limit  int `json:"limit"`
			Offset int `json:"offset"`
		} `json:"meta"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Verify pagination
	if len(resp.Data) != 2 {
		t.Errorf("expected 2 users with limit=2, got %d", len(resp.Data))
	}

	if resp.Meta.Limit != 2 {
		t.Errorf("expected limit 2, got %d", resp.Meta.Limit)
	}

	if resp.Meta.Offset != 1 {
		t.Errorf("expected offset 1, got %d", resp.Meta.Offset)
	}

	if resp.Meta.Total != 5 {
		t.Errorf("expected total 5, got %d", resp.Meta.Total)
	}
}

// TestListUsers_LimitMax100 tests that limit is capped at 100.
// Per prd-v4: limit max 100
func TestListUsers_LimitMax100(t *testing.T) {
	userListRepo := NewMockUserListRepository()
	handler := NewUsersHandler(nil, nil)
	handler.SetUserListRepository(userListRepo)

	userListRepo.total = 0

	// Request with limit=200 (should be capped to 100)
	req := httptest.NewRequest(http.MethodGet, "/v1/users?limit=200", nil)
	rr := httptest.NewRecorder()

	handler.ListUsers(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Meta struct {
			Limit int `json:"limit"`
		} `json:"meta"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Verify limit was capped
	if resp.Meta.Limit != 100 {
		t.Errorf("expected limit capped to 100, got %d", resp.Meta.Limit)
	}
}

// TestListUsers_DefaultLimit20 tests that default limit is 20.
// Per prd-v4: limit default 20
func TestListUsers_DefaultLimit20(t *testing.T) {
	userListRepo := NewMockUserListRepository()
	handler := NewUsersHandler(nil, nil)
	handler.SetUserListRepository(userListRepo)

	userListRepo.total = 0

	// Request without limit param
	req := httptest.NewRequest(http.MethodGet, "/v1/users", nil)
	rr := httptest.NewRecorder()

	handler.ListUsers(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Meta struct {
			Limit int `json:"limit"`
		} `json:"meta"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Verify default limit
	if resp.Meta.Limit != 20 {
		t.Errorf("expected default limit 20, got %d", resp.Meta.Limit)
	}
}

// TestListUsers_Empty tests that the endpoint returns empty array when no users.
func TestListUsers_Empty(t *testing.T) {
	userListRepo := NewMockUserListRepository()
	handler := NewUsersHandler(nil, nil)
	handler.SetUserListRepository(userListRepo)

	userListRepo.total = 0

	req := httptest.NewRequest(http.MethodGet, "/v1/users", nil)
	rr := httptest.NewRecorder()

	handler.ListUsers(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Data []models.UserListItem `json:"data"`
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
		t.Errorf("expected 0 users, got %d", len(resp.Data))
	}

	if resp.Meta.Total != 0 {
		t.Errorf("expected total 0, got %d", resp.Meta.Total)
	}
}
