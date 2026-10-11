package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// ============================================================================
// Mock repository for ListUsers tests
// ============================================================================

// MockUserListRepository implements user repository interface for list tests.
type MockUserListRepository struct {
	users             []models.UserListItem
	total             int
	totalBackedAgents int
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

func (m *MockUserListRepository) GetAggregateStats(ctx context.Context) (int, error) {
	return m.totalBackedAgents, nil
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
			Reputation:  150,
			AgentsCount: 3,
			CreatedAt:   now,
		},
		{
			ID:          "user-2",
			Username:    "bob",
			DisplayName: "Bob Jones",
			AvatarURL:   "https://example.com/bob.jpg",
			Reputation:  75,
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
			Reputation:  i * 10,
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
// Tests for total_backed_agents in GET /v1/users response meta (prd-v5)
// ============================================================================

// TestListUsers_AggregateStats tests that the response meta includes total_backed_agents.
// Per prd-v5: API must return platform-wide total_backed_agents in meta so frontend
// can show accurate stats without summing the current page.
func TestListUsers_AggregateStats(t *testing.T) {
	userListRepo := NewMockUserListRepository()
	handler := NewUsersHandler(nil, nil)
	handler.SetUserListRepository(userListRepo)

	now := time.Now()
	userListRepo.users = []models.UserListItem{
		{
			ID:          "user-1",
			Username:    "alice",
			DisplayName: "Alice",
			Reputation:  100,
			AgentsCount: 2,
			CreatedAt:   now,
		},
	}
	userListRepo.total = 1
	userListRepo.totalBackedAgents = 42

	req := httptest.NewRequest(http.MethodGet, "/v1/users", nil)
	rr := httptest.NewRecorder()

	handler.ListUsers(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Data []models.UserListItem `json:"data"`
		Meta struct {
			Total             int `json:"total"`
			TotalBackedAgents int `json:"total_backed_agents"`
		} `json:"meta"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Meta.TotalBackedAgents != 42 {
		t.Errorf("expected total_backed_agents 42, got %d", resp.Meta.TotalBackedAgents)
	}
}
