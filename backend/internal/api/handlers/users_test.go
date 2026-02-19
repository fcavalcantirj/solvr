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

// MockUsersUserRepository implements the user repository interface for testing.
type MockUsersUserRepository struct {
	users map[string]*models.User
	stats map[string]*models.UserStats
}

func NewMockUsersUserRepository() *MockUsersUserRepository {
	return &MockUsersUserRepository{
		users: make(map[string]*models.User),
		stats: make(map[string]*models.UserStats),
	}
}

func (m *MockUsersUserRepository) FindByID(ctx context.Context, id string) (*models.User, error) {
	user, ok := m.users[id]
	if !ok {
		return nil, nil
	}
	return user, nil
}

func (m *MockUsersUserRepository) Update(ctx context.Context, user *models.User) (*models.User, error) {
	m.users[user.ID] = user
	return user, nil
}

func (m *MockUsersUserRepository) GetUserStats(ctx context.Context, userID string) (*models.UserStats, error) {
	stats, ok := m.stats[userID]
	if !ok {
		return &models.UserStats{}, nil
	}
	return stats, nil
}

// MockUsersAgentRepository implements the agent repository interface for testing.
type MockUsersAgentRepository struct {
	agents map[string][]*models.Agent    // humanID -> agents
	stats  map[string]*models.AgentStats // agentID -> stats
}

func NewMockUsersAgentRepository() *MockUsersAgentRepository {
	return &MockUsersAgentRepository{
		agents: make(map[string][]*models.Agent),
		stats:  make(map[string]*models.AgentStats),
	}
}

func (m *MockUsersAgentRepository) FindByHumanID(ctx context.Context, humanID string) ([]*models.Agent, error) {
	agents, ok := m.agents[humanID]
	if !ok {
		return []*models.Agent{}, nil
	}
	return agents, nil
}

func (m *MockUsersAgentRepository) GetAgentStats(ctx context.Context, agentID string) (*models.AgentStats, error) {
	stats, ok := m.stats[agentID]
	if !ok {
		return &models.AgentStats{}, nil
	}
	return stats, nil
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

	humanID := "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
	agentRepo.agents[humanID] = []*models.Agent{
		{
			ID:                  "agent_one",
			DisplayName:         "Agent One",
			HumanID:             &humanID,
			Bio:                 "First agent",
			Model:               "claude-opus-4",
			Reputation:               100,
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
			Reputation:               50,
			HasHumanBackedBadge: true,
			CreatedAt:           time.Now(),
			UpdatedAt:           time.Now(),
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/users/"+humanID+"/agents", nil)
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

	humanID := "b2c3d4e5-f6a7-8901-bcde-f12345678901"
	// No agents added for this user

	req := httptest.NewRequest(http.MethodGet, "/v1/users/"+humanID+"/agents", nil)
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
// Per prd-v4: Response includes id, username, display_name, avatar_url, reputation, agents_count, created_at
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
			Reputation:       150,
			AgentsCount: 3,
			CreatedAt:   now,
		},
		{
			ID:          "user-2",
			Username:    "bob",
			DisplayName: "Bob Jones",
			AvatarURL:   "https://example.com/bob.jpg",
			Reputation:       75,
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

	if resp.Data[0].Reputation != 150 {
		t.Errorf("expected reputation 150, got %d", resp.Data[0].Reputation)
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
			Reputation:       i * 10,
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

// TestListUsers_HasMoreField tests that meta.has_more is returned correctly.
// Bug 7: Users page pagination broken — has_more was missing from API response.
// MUST FAIL before the fix (has_more field missing from UsersListResponse.Meta).
func TestListUsers_HasMoreField(t *testing.T) {
	userListRepo := NewMockUserListRepository()
	handler := NewUsersHandler(nil, nil)
	handler.SetUserListRepository(userListRepo)

	// Set up 25 total users but only populate 20 in the slice (as if DB returned page 1)
	now := time.Now()
	for i := 1; i <= 20; i++ {
		userListRepo.users = append(userListRepo.users, models.UserListItem{
			ID:        "user-" + string(rune('A'+i-1)),
			Username:  "user" + string(rune('A'+i-1)),
			CreatedAt: now,
		})
	}
	userListRepo.total = 25 // 25 total, only 20 returned → has_more must be true

	// Page 1: limit=20, offset=0 → has_more should be true (5 more remain)
	req := httptest.NewRequest(http.MethodGet, "/v1/users?limit=20&offset=0", nil)
	rr := httptest.NewRecorder()
	handler.ListUsers(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp1 struct {
		Meta struct {
			Total   int  `json:"total"`
			HasMore bool `json:"has_more"`
		} `json:"meta"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&resp1); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !resp1.Meta.HasMore {
		t.Errorf("expected has_more=true when total=25 and limit=20 offset=0, got false")
	}

	// Page 2: limit=20, offset=20 → has_more should be false (nothing more)
	req2 := httptest.NewRequest(http.MethodGet, "/v1/users?limit=20&offset=20", nil)
	rr2 := httptest.NewRecorder()
	handler.ListUsers(rr2, req2)

	if rr2.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr2.Code, rr2.Body.String())
	}

	var resp2 struct {
		Meta struct {
			Total   int  `json:"total"`
			HasMore bool `json:"has_more"`
		} `json:"meta"`
	}
	if err := json.NewDecoder(rr2.Body).Decode(&resp2); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp2.Meta.HasMore {
		t.Errorf("expected has_more=false when total=25 and limit=20 offset=20, got true")
	}
}

// ============================================================================
// Tests for GET /v1/users/{id} UUID validation
// ============================================================================

// TestGetUserProfile_InvalidUUID tests that non-UUID IDs return 400 BAD_REQUEST.
// Regression test: /v1/users/me was returning 500 because "me" is not a valid UUID.
func TestGetUserProfile_InvalidUUID(t *testing.T) {
	userRepo := NewMockUsersUserRepository()
	handler := NewUsersHandler(userRepo, nil)

	tests := []struct {
		name string
		id   string
	}{
		{"literal me", "me"},
		{"random string", "not-a-uuid"},
		{"partial uuid", "8ec0e613-d4ec"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/v1/users/invalid", nil)
			rr := httptest.NewRecorder()

			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("id", tt.id)
			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

			handler.GetUserProfile(rr, req)

			if rr.Code != http.StatusBadRequest {
				t.Errorf("expected status 400 for id=%q, got %d: %s", tt.id, rr.Code, rr.Body.String())
			}

			var resp struct {
				Error struct {
					Code string `json:"code"`
				} `json:"error"`
			}
			if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}

			if resp.Error.Code != "BAD_REQUEST" {
				t.Errorf("expected BAD_REQUEST code, got %s", resp.Error.Code)
			}
		})
	}
}

// TestGetUserProfile_ValidUUID tests that valid UUID IDs work correctly.
func TestGetUserProfile_ValidUUID(t *testing.T) {
	userRepo := NewMockUsersUserRepository()
	handler := NewUsersHandler(userRepo, nil)

	validID := "8ec0e613-d4ec-489b-a6ab-b62e4a1600ec"
	userRepo.users[validID] = &models.User{
		ID:          validID,
		Username:    "testuser",
		DisplayName: "Test User",
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/users/"+validID, nil)
	rr := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", validID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	handler.GetUserProfile(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200 for valid UUID, got %d: %s", rr.Code, rr.Body.String())
	}
}

// ============================================================================
// Tests for computed reputation in GET /v1/users/{id}/agents
// ============================================================================

// TestGetUserAgents_ReturnsComputedReputation verifies that the response
// includes the dynamically computed reputation, not the stored bonus.
func TestGetUserAgents_ReturnsComputedReputation(t *testing.T) {
	agentRepo := NewMockUsersAgentRepository()
	handler := NewUsersHandler(nil, nil)
	handler.SetAgentRepository(agentRepo)

	humanID := "c3d4e5f6-a7b8-9012-cdef-123456789012"
	agentRepo.agents[humanID] = []*models.Agent{
		{
			ID:          "agent_computed",
			DisplayName: "Computed Rep Agent",
			HumanID:     &humanID,
			Reputation:  50, // Stored bonus only
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
	}
	// Mock computed stats: real reputation is 520 (includes activity)
	agentRepo.stats["agent_computed"] = &models.AgentStats{
		Reputation:          520,
		ProblemsContributed: 18,
		IdeasPosted:         30,
		UpvotesReceived:     6,
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/users/"+humanID+"/agents", nil)
	rr := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", humanID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	handler.GetUserAgents(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Data []models.Agent `json:"data"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(resp.Data) != 1 {
		t.Fatalf("expected 1 agent, got %d", len(resp.Data))
	}

	// Should return computed reputation (520), not stored bonus (50)
	if resp.Data[0].Reputation != 520 {
		t.Errorf("expected computed reputation 520, got %d", resp.Data[0].Reputation)
	}
}

// TestGetUserAgents_OrderedByComputedReputation verifies that agents are
// ordered by their computed reputation (highest first), not stored bonus.
func TestGetUserAgents_OrderedByComputedReputation(t *testing.T) {
	agentRepo := NewMockUsersAgentRepository()
	handler := NewUsersHandler(nil, nil)
	handler.SetAgentRepository(agentRepo)

	humanID := "d4e5f6a7-b8c9-0123-defa-234567890123"
	agentRepo.agents[humanID] = []*models.Agent{
		{
			ID:          "agent_low_rep",
			DisplayName: "Low Rep",
			HumanID:     &humanID,
			Reputation:  50, // Same stored bonus
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			ID:          "agent_high_rep",
			DisplayName: "High Rep",
			HumanID:     &humanID,
			Reputation:  50, // Same stored bonus
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
	}
	// Different computed reputations
	agentRepo.stats["agent_low_rep"] = &models.AgentStats{Reputation: 80}
	agentRepo.stats["agent_high_rep"] = &models.AgentStats{Reputation: 520}

	req := httptest.NewRequest(http.MethodGet, "/v1/users/"+humanID+"/agents", nil)
	rr := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", humanID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	handler.GetUserAgents(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Data []models.Agent `json:"data"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(resp.Data) != 2 {
		t.Fatalf("expected 2 agents, got %d", len(resp.Data))
	}

	// Highest computed reputation should be first
	if resp.Data[0].ID != "agent_high_rep" {
		t.Errorf("expected first agent to be 'agent_high_rep' (rep 520), got '%s'", resp.Data[0].ID)
	}
	if resp.Data[1].ID != "agent_low_rep" {
		t.Errorf("expected second agent to be 'agent_low_rep' (rep 80), got '%s'", resp.Data[1].ID)
	}
}
