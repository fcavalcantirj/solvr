package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// MockStatsRepository implements StatsRepositoryInterface for testing
type MockStatsRepository struct {
	ActivePosts   int
	TotalAgents   int
	SolvedToday   int
	TrendingPosts []any
	TrendingTags  []any
}

func (m *MockStatsRepository) GetActivePostsCount(ctx context.Context) (int, error) {
	return m.ActivePosts, nil
}

func (m *MockStatsRepository) GetAgentsCount(ctx context.Context) (int, error) {
	return m.TotalAgents, nil
}

func (m *MockStatsRepository) GetSolvedTodayCount(ctx context.Context) (int, error) {
	return m.SolvedToday, nil
}

func (m *MockStatsRepository) GetTrendingPosts(ctx context.Context, limit int) ([]any, error) {
	if limit > len(m.TrendingPosts) {
		return m.TrendingPosts, nil
	}
	return m.TrendingPosts[:limit], nil
}

func (m *MockStatsRepository) GetTrendingTags(ctx context.Context, limit int) ([]any, error) {
	if limit > len(m.TrendingTags) {
		return m.TrendingTags, nil
	}
	return m.TrendingTags[:limit], nil
}

func TestStatsHandler_GetStats(t *testing.T) {
	tests := []struct {
		name           string
		mockRepo       *MockStatsRepository
		expectedStatus int
		checkResponse  func(t *testing.T, body map[string]interface{})
	}{
		{
			name: "returns all stats",
			mockRepo: &MockStatsRepository{
				ActivePosts: 147,
				TotalAgents: 23,
				SolvedToday: 12,
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body map[string]interface{}) {
				data := body["data"].(map[string]interface{})
				if int(data["active_posts"].(float64)) != 147 {
					t.Errorf("expected active_posts=147, got %v", data["active_posts"])
				}
				if int(data["total_agents"].(float64)) != 23 {
					t.Errorf("expected total_agents=23, got %v", data["total_agents"])
				}
				if int(data["solved_today"].(float64)) != 12 {
					t.Errorf("expected solved_today=12, got %v", data["solved_today"])
				}
			},
		},
		{
			name: "returns zero stats for empty database",
			mockRepo: &MockStatsRepository{
				ActivePosts: 0,
				TotalAgents: 0,
				SolvedToday: 0,
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body map[string]interface{}) {
				data := body["data"].(map[string]interface{})
				if int(data["active_posts"].(float64)) != 0 {
					t.Errorf("expected active_posts=0, got %v", data["active_posts"])
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewStatsHandler(tt.mockRepo)
			req := httptest.NewRequest("GET", "/v1/stats", nil)
			rec := httptest.NewRecorder()

			handler.GetStats(rec, req)

			if rec.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, rec.Code)
			}

			var body map[string]interface{}
			if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}

			if tt.checkResponse != nil {
				tt.checkResponse(t, body)
			}
		})
	}
}

func TestStatsHandler_GetTrending(t *testing.T) {
	mockRepo := &MockStatsRepository{
		TrendingPosts: []any{
			map[string]any{"id": "1", "title": "Hot Post 1", "type": "idea", "response_count": 142, "vote_score": 50},
			map[string]any{"id": "2", "title": "Hot Post 2", "type": "question", "response_count": 98, "vote_score": 30},
			map[string]any{"id": "3", "title": "Hot Post 3", "type": "idea", "response_count": 87, "vote_score": 25},
		},
		TrendingTags: []any{
			map[string]any{"name": "async", "count": 234, "growth": 12},
			map[string]any{"name": "golang", "count": 189, "growth": 8},
			map[string]any{"name": "react", "count": 156, "growth": -2},
		},
	}

	handler := NewStatsHandler(mockRepo)
	req := httptest.NewRequest("GET", "/v1/stats/trending", nil)
	rec := httptest.NewRecorder()

	handler.GetTrending(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	data := body["data"].(map[string]interface{})
	posts := data["posts"].([]interface{})
	tags := data["tags"].([]interface{})

	if len(posts) != 3 {
		t.Errorf("expected 3 trending posts, got %d", len(posts))
	}
	if len(tags) != 3 {
		t.Errorf("expected 3 trending tags, got %d", len(tags))
	}
}

// Types are defined in stats.go - reusing them here
