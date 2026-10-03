package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/db"
)

// MockStatsRepository implements StatsRepositoryInterface for testing
type MockStatsRepository struct {
	ActivePosts        int
	TotalAgents        int
	PostedToday        int
	HumansCount        int
	TotalPosts         int
	TotalContributions int
	CrystallizedPosts  int
	TrendingPosts      []any
	TrendingTags       []any
}

func (m *MockStatsRepository) GetAllStats(ctx context.Context) (*db.AllStatsResult, error) {
	return &db.AllStatsResult{
		ActivePosts:        m.ActivePosts,
		TotalAgents:        m.TotalAgents,
		PostedToday:        m.PostedToday,
		HumansCount:        m.HumansCount,
		TotalPosts:         m.TotalPosts,
		TotalContributions: m.TotalContributions,
		CrystallizedPosts:  m.CrystallizedPosts,
	}, nil
}

func (m *MockStatsRepository) GetActivePostsCount(ctx context.Context) (int, error) {
	return m.ActivePosts, nil
}

func (m *MockStatsRepository) GetAgentsCount(ctx context.Context) (int, error) {
	return m.TotalAgents, nil
}

func (m *MockStatsRepository) GetPostedTodayCount(ctx context.Context) (int, error) {
	return m.PostedToday, nil
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

func (m *MockStatsRepository) GetHumansCount(ctx context.Context) (int, error) {
	return m.HumansCount, nil
}

func (m *MockStatsRepository) GetTotalPostsCount(ctx context.Context) (int, error) {
	return m.TotalPosts, nil
}

func (m *MockStatsRepository) GetTotalContributionsCount(ctx context.Context) (int, error) {
	return m.TotalContributions, nil
}

func TestStatsHandler_GetStats(t *testing.T) {
	tests := []struct {
		name           string
		mockRepo       *MockStatsRepository
		expectedStatus int
		checkResponse  func(t *testing.T, body map[string]interface{})
	}{
		{
			name: "returns the seven stats fields and none of the retired solved figures",
			mockRepo: &MockStatsRepository{
				ActivePosts:        147,
				TotalAgents:        23,
				PostedToday:        25,
				HumansCount:        156,
				TotalPosts:         500,
				TotalContributions: 320,
				CrystallizedPosts:  7,
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body map[string]interface{}) {
				data := body["data"].(map[string]interface{})
				checks := map[string]int{
					"active_posts":        147,
					"total_agents":        23,
					"posted_today":        25,
					"humans_count":        156,
					"total_posts":         500,
					"total_contributions": 320,
					"crystallized_posts":  7,
				}
				for field, expected := range checks {
					got := int(data[field].(float64))
					if got != expected {
						t.Errorf("expected %s=%d, got %d", field, expected, got)
					}
				}
				for _, retired := range []string{"solved_today", "problems_solved", "questions_answered"} {
					if _, ok := data[retired]; ok {
						t.Errorf("%s was retired with the legacy post types (idx 68) and must not be returned", retired)
					}
				}
			},
		},
		{
			name: "returns zero stats for empty database",
			mockRepo: &MockStatsRepository{
				ActivePosts: 0,
				TotalAgents: 0,
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

func TestGetStats_CacheControl(t *testing.T) {
	mockRepo := &MockStatsRepository{}
	handler := NewStatsHandler(mockRepo)
	req := httptest.NewRequest("GET", "/v1/stats", nil)
	rec := httptest.NewRecorder()

	handler.GetStats(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	cc := rec.Header().Get("Cache-Control")
	if cc != "public, max-age=30" {
		t.Errorf("expected Cache-Control 'public, max-age=30', got %q", cc)
	}
}

func TestGetTrending_CacheControl(t *testing.T) {
	mockRepo := &MockStatsRepository{}
	handler := NewStatsHandler(mockRepo)
	req := httptest.NewRequest("GET", "/v1/stats/trending", nil)
	rec := httptest.NewRecorder()

	handler.GetTrending(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	cc := rec.Header().Get("Cache-Control")
	if cc != "public, max-age=30" {
		t.Errorf("expected Cache-Control 'public, max-age=30', got %q", cc)
	}
}
