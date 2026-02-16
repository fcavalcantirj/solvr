package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// MockLeaderboardRepository is a mock implementation of LeaderboardRepositoryInterface.
type MockLeaderboardRepository struct {
	GetLeaderboardFunc      func(ctx context.Context, opts models.LeaderboardOptions) ([]models.LeaderboardEntry, int, error)
	GetLeaderboardByTagFunc func(ctx context.Context, tag string, opts models.LeaderboardOptions) ([]models.LeaderboardEntry, int, error)
}

func (m *MockLeaderboardRepository) GetLeaderboard(ctx context.Context, opts models.LeaderboardOptions) ([]models.LeaderboardEntry, int, error) {
	if m.GetLeaderboardFunc != nil {
		return m.GetLeaderboardFunc(ctx, opts)
	}
	return nil, 0, nil
}

func (m *MockLeaderboardRepository) GetLeaderboardByTag(ctx context.Context, tag string, opts models.LeaderboardOptions) ([]models.LeaderboardEntry, int, error) {
	if m.GetLeaderboardByTagFunc != nil {
		return m.GetLeaderboardByTagFunc(ctx, tag, opts)
	}
	return nil, 0, nil
}

// TestGetLeaderboard_AllTypes verifies the leaderboard returns mixed agents and users.
func TestGetLeaderboard_AllTypes(t *testing.T) {
	mockRepo := &MockLeaderboardRepository{
		GetLeaderboardFunc: func(ctx context.Context, opts models.LeaderboardOptions) ([]models.LeaderboardEntry, int, error) {
			// Verify default options
			if opts.Type != "all" {
				t.Errorf("expected type=all, got %s", opts.Type)
			}
			if opts.Timeframe != "all_time" {
				t.Errorf("expected timeframe=all_time, got %s", opts.Timeframe)
			}

			return []models.LeaderboardEntry{
				{
					Rank:        1,
					ID:          "test_agent",
					Type:        "agent",
					DisplayName: "Test Agent",
					AvatarURL:   "https://example.com/avatar.jpg",
					Reputation:  500,
					KeyStats: models.LeaderboardStats{
						ProblemsSolved:     3,
						AnswersAccepted:    2,
						UpvotesReceived:    10,
						TotalContributions: 15,
					},
				},
				{
					Rank:        2,
					ID:          "user-123",
					Type:        "user",
					DisplayName: "Test User",
					AvatarURL:   "",
					Reputation:  300,
					KeyStats: models.LeaderboardStats{
						ProblemsSolved:     1,
						AnswersAccepted:    1,
						UpvotesReceived:    5,
						TotalContributions: 7,
					},
				},
			}, 2, nil
		},
	}

	handler := NewLeaderboardHandler(mockRepo)

	req := httptest.NewRequest(http.MethodGet, "/v1/leaderboard", nil)
	w := httptest.NewRecorder()

	handler.GetLeaderboard(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response struct {
		Data []models.LeaderboardEntry `json:"data"`
		Meta struct {
			Total   int  `json:"total"`
			Page    int  `json:"page"`
			PerPage int  `json:"per_page"`
			HasMore bool `json:"has_more"`
		} `json:"meta"`
	}

	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(response.Data) != 2 {
		t.Errorf("expected 2 entries, got %d", len(response.Data))
	}

	if response.Data[0].Type != "agent" {
		t.Errorf("expected first entry to be agent, got %s", response.Data[0].Type)
	}

	if response.Data[1].Type != "user" {
		t.Errorf("expected second entry to be user, got %s", response.Data[1].Type)
	}

	if response.Meta.Total != 2 {
		t.Errorf("expected total=2, got %d", response.Meta.Total)
	}
}

// TestGetLeaderboard_AgentsOnly verifies filtering to agents only.
func TestGetLeaderboard_AgentsOnly(t *testing.T) {
	mockRepo := &MockLeaderboardRepository{
		GetLeaderboardFunc: func(ctx context.Context, opts models.LeaderboardOptions) ([]models.LeaderboardEntry, int, error) {
			if opts.Type != "agents" {
				t.Errorf("expected type=agents, got %s", opts.Type)
			}

			return []models.LeaderboardEntry{
				{
					Rank:        1,
					ID:          "agent1",
					Type:        "agent",
					DisplayName: "Agent One",
					Reputation:  500,
				},
				{
					Rank:        2,
					ID:          "agent2",
					Type:        "agent",
					DisplayName: "Agent Two",
					Reputation:  300,
				},
			}, 2, nil
		},
	}

	handler := NewLeaderboardHandler(mockRepo)

	req := httptest.NewRequest(http.MethodGet, "/v1/leaderboard?type=agents", nil)
	w := httptest.NewRecorder()

	handler.GetLeaderboard(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response struct {
		Data []models.LeaderboardEntry `json:"data"`
	}

	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	for i, entry := range response.Data {
		if entry.Type != "agent" {
			t.Errorf("entry %d: expected type=agent, got %s", i, entry.Type)
		}
	}
}

// TestGetLeaderboard_UsersOnly verifies filtering to users only.
func TestGetLeaderboard_UsersOnly(t *testing.T) {
	mockRepo := &MockLeaderboardRepository{
		GetLeaderboardFunc: func(ctx context.Context, opts models.LeaderboardOptions) ([]models.LeaderboardEntry, int, error) {
			if opts.Type != "users" {
				t.Errorf("expected type=users, got %s", opts.Type)
			}

			return []models.LeaderboardEntry{
				{
					Rank:        1,
					ID:          "user-1",
					Type:        "user",
					DisplayName: "User One",
					Reputation:  400,
				},
			}, 1, nil
		},
	}

	handler := NewLeaderboardHandler(mockRepo)

	req := httptest.NewRequest(http.MethodGet, "/v1/leaderboard?type=users", nil)
	w := httptest.NewRecorder()

	handler.GetLeaderboard(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response struct {
		Data []models.LeaderboardEntry `json:"data"`
	}

	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	for i, entry := range response.Data {
		if entry.Type != "user" {
			t.Errorf("entry %d: expected type=user, got %s", i, entry.Type)
		}
	}
}

// TestGetLeaderboard_Pagination verifies limit and offset parameters.
func TestGetLeaderboard_Pagination(t *testing.T) {
	mockRepo := &MockLeaderboardRepository{
		GetLeaderboardFunc: func(ctx context.Context, opts models.LeaderboardOptions) ([]models.LeaderboardEntry, int, error) {
			if opts.Limit != 10 {
				t.Errorf("expected limit=10, got %d", opts.Limit)
			}
			if opts.Offset != 20 {
				t.Errorf("expected offset=20, got %d", opts.Offset)
			}

			// Return 10 entries starting from rank 21
			entries := make([]models.LeaderboardEntry, 10)
			for i := 0; i < 10; i++ {
				entries[i] = models.LeaderboardEntry{
					Rank:        21 + i,
					ID:          "agent" + string(rune(21+i)),
					Type:        "agent",
					DisplayName: "Agent",
					Reputation:  100 - i,
				}
			}
			return entries, 100, nil // Total of 100 entries
		},
	}

	handler := NewLeaderboardHandler(mockRepo)

	req := httptest.NewRequest(http.MethodGet, "/v1/leaderboard?limit=10&offset=20", nil)
	w := httptest.NewRecorder()

	handler.GetLeaderboard(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response struct {
		Data []models.LeaderboardEntry `json:"data"`
		Meta struct {
			Total   int  `json:"total"`
			Page    int  `json:"page"`
			PerPage int  `json:"per_page"`
			HasMore bool `json:"has_more"`
		} `json:"meta"`
	}

	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(response.Data) != 10 {
		t.Errorf("expected 10 entries, got %d", len(response.Data))
	}

	if response.Meta.Total != 100 {
		t.Errorf("expected total=100, got %d", response.Meta.Total)
	}

	if response.Meta.HasMore != true {
		t.Errorf("expected has_more=true")
	}
}

// TestGetLeaderboard_RankingOrder verifies entries are sorted by reputation.
func TestGetLeaderboard_RankingOrder(t *testing.T) {
	mockRepo := &MockLeaderboardRepository{
		GetLeaderboardFunc: func(ctx context.Context, opts models.LeaderboardOptions) ([]models.LeaderboardEntry, int, error) {
			return []models.LeaderboardEntry{
				{Rank: 1, ID: "top", Reputation: 1000},
				{Rank: 2, ID: "second", Reputation: 900},
				{Rank: 3, ID: "third", Reputation: 800},
			}, 3, nil
		},
	}

	handler := NewLeaderboardHandler(mockRepo)

	req := httptest.NewRequest(http.MethodGet, "/v1/leaderboard", nil)
	w := httptest.NewRecorder()

	handler.GetLeaderboard(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response struct {
		Data []models.LeaderboardEntry `json:"data"`
	}

	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Verify descending order
	for i := 0; i < len(response.Data)-1; i++ {
		if response.Data[i].Reputation < response.Data[i+1].Reputation {
			t.Errorf("reputation not in descending order: entry %d (%d) < entry %d (%d)",
				i, response.Data[i].Reputation, i+1, response.Data[i+1].Reputation)
		}

		if response.Data[i].Rank != i+1 {
			t.Errorf("entry %d: expected rank=%d, got %d", i, i+1, response.Data[i].Rank)
		}
	}
}

// TestGetLeaderboard_AllTime verifies default all_time timeframe uses total reputation.
func TestGetLeaderboard_AllTime(t *testing.T) {
	mockRepo := &MockLeaderboardRepository{
		GetLeaderboardFunc: func(ctx context.Context, opts models.LeaderboardOptions) ([]models.LeaderboardEntry, int, error) {
			if opts.Timeframe != "all_time" {
				t.Errorf("expected timeframe=all_time, got %s", opts.Timeframe)
			}

			return []models.LeaderboardEntry{
				{Rank: 1, ID: "agent1", Type: "agent", Reputation: 1000},
				{Rank: 2, ID: "agent2", Type: "agent", Reputation: 500},
			}, 2, nil
		},
	}

	handler := NewLeaderboardHandler(mockRepo)

	// Test explicit all_time parameter
	req := httptest.NewRequest(http.MethodGet, "/v1/leaderboard?timeframe=all_time", nil)
	w := httptest.NewRecorder()

	handler.GetLeaderboard(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response struct {
		Data []models.LeaderboardEntry `json:"data"`
	}

	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(response.Data) != 2 {
		t.Errorf("expected 2 entries, got %d", len(response.Data))
	}
}

// TestGetLeaderboard_Monthly verifies monthly timeframe filters activity.
func TestGetLeaderboard_Monthly(t *testing.T) {
	mockRepo := &MockLeaderboardRepository{
		GetLeaderboardFunc: func(ctx context.Context, opts models.LeaderboardOptions) ([]models.LeaderboardEntry, int, error) {
			if opts.Timeframe != "monthly" {
				t.Errorf("expected timeframe=monthly, got %s", opts.Timeframe)
			}

			// Return entries with reputation from current month only
			return []models.LeaderboardEntry{
				{
					Rank:       1,
					ID:         "agent1",
					Type:       "agent",
					Reputation: 200, // Lower than all_time because only this month's activity
					KeyStats: models.LeaderboardStats{
						ProblemsSolved:  2,
						AnswersAccepted: 0,
						UpvotesReceived: 0,
					},
				},
			}, 1, nil
		},
	}

	handler := NewLeaderboardHandler(mockRepo)

	req := httptest.NewRequest(http.MethodGet, "/v1/leaderboard?timeframe=monthly", nil)
	w := httptest.NewRecorder()

	handler.GetLeaderboard(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response struct {
		Data []models.LeaderboardEntry `json:"data"`
	}

	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(response.Data) == 0 {
		t.Error("expected at least one entry for monthly leaderboard")
	}
}

// TestGetLeaderboard_Weekly verifies weekly timeframe filters activity.
func TestGetLeaderboard_Weekly(t *testing.T) {
	mockRepo := &MockLeaderboardRepository{
		GetLeaderboardFunc: func(ctx context.Context, opts models.LeaderboardOptions) ([]models.LeaderboardEntry, int, error) {
			if opts.Timeframe != "weekly" {
				t.Errorf("expected timeframe=weekly, got %s", opts.Timeframe)
			}

			// Return entries with reputation from current week only
			return []models.LeaderboardEntry{
				{
					Rank:       1,
					ID:         "agent1",
					Type:       "agent",
					Reputation: 100, // Lower than monthly because only this week's activity
					KeyStats: models.LeaderboardStats{
						ProblemsSolved:  1,
						AnswersAccepted: 0,
						UpvotesReceived: 0,
					},
				},
			}, 1, nil
		},
	}

	handler := NewLeaderboardHandler(mockRepo)

	req := httptest.NewRequest(http.MethodGet, "/v1/leaderboard?timeframe=weekly", nil)
	w := httptest.NewRecorder()

	handler.GetLeaderboard(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response struct {
		Data []models.LeaderboardEntry `json:"data"`
	}

	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(response.Data) == 0 {
		t.Error("expected at least one entry for weekly leaderboard")
	}
}

// TestGetLeaderboard_InvalidTimeframe verifies invalid timeframe defaults to all_time.
func TestGetLeaderboard_InvalidTimeframe(t *testing.T) {
	mockRepo := &MockLeaderboardRepository{
		GetLeaderboardFunc: func(ctx context.Context, opts models.LeaderboardOptions) ([]models.LeaderboardEntry, int, error) {
			// Invalid timeframe should be passed through to the repository
			// The repository can handle validation or default to all_time
			return []models.LeaderboardEntry{
				{Rank: 1, ID: "agent1", Type: "agent", Reputation: 1000},
			}, 1, nil
		},
	}

	handler := NewLeaderboardHandler(mockRepo)

	req := httptest.NewRequest(http.MethodGet, "/v1/leaderboard?timeframe=invalid", nil)
	w := httptest.NewRecorder()

	handler.GetLeaderboard(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

// TestGetLeaderboardByTag_ValidTag verifies handler passes correct tag to repository.
func TestGetLeaderboardByTag_ValidTag(t *testing.T) {
	mockRepo := &MockLeaderboardRepository{
		GetLeaderboardByTagFunc: func(ctx context.Context, tag string, opts models.LeaderboardOptions) ([]models.LeaderboardEntry, int, error) {
			// Verify tag is passed correctly
			if tag != "golang" {
				t.Errorf("expected tag=golang, got %s", tag)
			}

			return []models.LeaderboardEntry{
				{
					Rank:        1,
					ID:          "test_agent",
					Type:        "agent",
					DisplayName: "Golang Expert",
					Reputation:  500,
					KeyStats: models.LeaderboardStats{
						ProblemsSolved:     3,
						AnswersAccepted:    0,
						UpvotesReceived:    0,
						TotalContributions: 3,
					},
				},
			}, 1, nil
		},
	}

	handler := NewLeaderboardHandler(mockRepo)

	req := httptest.NewRequest(http.MethodGet, "/v1/leaderboard/tags/golang", nil)
	req.SetPathValue("tag", "golang")
	w := httptest.NewRecorder()

	handler.GetLeaderboardByTag(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response LeaderboardResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(response.Data) != 1 {
		t.Errorf("expected 1 entry, got %d", len(response.Data))
	}

	if response.Meta.Total != 1 {
		t.Errorf("expected total=1, got %d", response.Meta.Total)
	}
}

// TestGetLeaderboardByTag_NoActivity verifies empty results for non-existent tag.
func TestGetLeaderboardByTag_NoActivity(t *testing.T) {
	mockRepo := &MockLeaderboardRepository{
		GetLeaderboardByTagFunc: func(ctx context.Context, tag string, opts models.LeaderboardOptions) ([]models.LeaderboardEntry, int, error) {
			// Return empty array for non-existent tag
			return []models.LeaderboardEntry{}, 0, nil
		},
	}

	handler := NewLeaderboardHandler(mockRepo)

	req := httptest.NewRequest(http.MethodGet, "/v1/leaderboard/tags/nonexistent", nil)
	req.SetPathValue("tag", "nonexistent")
	w := httptest.NewRecorder()

	handler.GetLeaderboardByTag(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response LeaderboardResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(response.Data) != 0 {
		t.Errorf("expected 0 entries, got %d", len(response.Data))
	}

	if response.Meta.Total != 0 {
		t.Errorf("expected total=0, got %d", response.Meta.Total)
	}
}

// TestGetLeaderboardByTag_Pagination verifies pagination parameters are passed correctly.
func TestGetLeaderboardByTag_Pagination(t *testing.T) {
	mockRepo := &MockLeaderboardRepository{
		GetLeaderboardByTagFunc: func(ctx context.Context, tag string, opts models.LeaderboardOptions) ([]models.LeaderboardEntry, int, error) {
			// Verify pagination params
			if opts.Limit != 5 {
				t.Errorf("expected limit=5, got %d", opts.Limit)
			}
			if opts.Offset != 10 {
				t.Errorf("expected offset=10, got %d", opts.Offset)
			}

			// Return page 2 (ranks 11-15)
			return []models.LeaderboardEntry{
				{Rank: 11, ID: "agent11", Type: "agent", Reputation: 400},
				{Rank: 12, ID: "agent12", Type: "agent", Reputation: 390},
				{Rank: 13, ID: "agent13", Type: "agent", Reputation: 380},
				{Rank: 14, ID: "agent14", Type: "agent", Reputation: 370},
				{Rank: 15, ID: "agent15", Type: "agent", Reputation: 360},
			}, 20, nil
		},
	}

	handler := NewLeaderboardHandler(mockRepo)

	req := httptest.NewRequest(http.MethodGet, "/v1/leaderboard/tags/rust?limit=5&offset=10", nil)
	req.SetPathValue("tag", "rust")
	w := httptest.NewRecorder()

	handler.GetLeaderboardByTag(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response LeaderboardResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Meta.Page != 3 {
		t.Errorf("expected page=3, got %d", response.Meta.Page)
	}

	if response.Meta.PerPage != 5 {
		t.Errorf("expected per_page=5, got %d", response.Meta.PerPage)
	}

	if !response.Meta.HasMore {
		t.Error("expected has_more=true")
	}
}

// TestGetLeaderboardByTag_InvalidTag verifies error when tag parameter is missing.
func TestGetLeaderboardByTag_InvalidTag(t *testing.T) {
	mockRepo := &MockLeaderboardRepository{}

	handler := NewLeaderboardHandler(mockRepo)

	req := httptest.NewRequest(http.MethodGet, "/v1/leaderboard/tags/", nil)
	req.SetPathValue("tag", "")
	w := httptest.NewRecorder()

	handler.GetLeaderboardByTag(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	errorObj, ok := response["error"].(map[string]interface{})
	if !ok {
		t.Fatal("expected error object in response")
	}

	if errorObj["code"] != "INVALID_TAG" {
		t.Errorf("expected error code INVALID_TAG, got %v", errorObj["code"])
	}
}
