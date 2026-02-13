package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// MockStatsRepository implements StatsRepositoryInterface for testing
type MockStatsRepository struct {
	ActivePosts        int
	TotalAgents        int
	SolvedToday        int
	PostedToday        int
	ProblemsSolved     int
	QuestionsAnswered  int
	HumansCount        int
	TotalPosts         int
	TotalContributions int
	TrendingPosts      []any
	TrendingTags       []any
	// Problems stats
	ProblemsStatsResult      map[string]any
	ProblemsStatsErr         error
	RecentlySolvedResult     []map[string]any
	RecentlySolvedErr        error
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

func (m *MockStatsRepository) GetProblemsSolvedCount(ctx context.Context) (int, error) {
	return m.ProblemsSolved, nil
}

func (m *MockStatsRepository) GetQuestionsAnsweredCount(ctx context.Context) (int, error) {
	return m.QuestionsAnswered, nil
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

func (m *MockStatsRepository) GetIdeasCountByStatus(ctx context.Context) (map[string]int, error) {
	return map[string]int{}, nil
}

func (m *MockStatsRepository) GetFreshSparks(ctx context.Context, limit int) ([]map[string]any, error) {
	return []map[string]any{}, nil
}

func (m *MockStatsRepository) GetReadyToDevelop(ctx context.Context, limit int) ([]map[string]any, error) {
	return []map[string]any{}, nil
}

func (m *MockStatsRepository) GetTopSparklers(ctx context.Context, limit int) ([]map[string]any, error) {
	return []map[string]any{}, nil
}

func (m *MockStatsRepository) GetIdeaPipelineStats(ctx context.Context) (map[string]any, error) {
	return map[string]any{}, nil
}

func (m *MockStatsRepository) GetRecentlyRealized(ctx context.Context, limit int) ([]map[string]any, error) {
	return []map[string]any{}, nil
}

func (m *MockStatsRepository) GetProblemsStats(ctx context.Context) (map[string]any, error) {
	if m.ProblemsStatsErr != nil {
		return nil, m.ProblemsStatsErr
	}
	if m.ProblemsStatsResult != nil {
		return m.ProblemsStatsResult, nil
	}
	return map[string]any{
		"total_problems":      0,
		"solved_count":        0,
		"active_approaches":   0,
		"avg_solve_time_days": 0,
	}, nil
}

func (m *MockStatsRepository) GetRecentlySolvedProblems(ctx context.Context, limit int) ([]map[string]any, error) {
	if m.RecentlySolvedErr != nil {
		return nil, m.RecentlySolvedErr
	}
	if m.RecentlySolvedResult != nil {
		if limit > len(m.RecentlySolvedResult) {
			return m.RecentlySolvedResult, nil
		}
		return m.RecentlySolvedResult[:limit], nil
	}
	return []map[string]any{}, nil
}

func TestStatsHandler_GetStats(t *testing.T) {
	tests := []struct {
		name           string
		mockRepo       *MockStatsRepository
		expectedStatus int
		checkResponse  func(t *testing.T, body map[string]interface{})
	}{
		{
			name: "returns all nine stats fields",
			mockRepo: &MockStatsRepository{
				ActivePosts:        147,
				TotalAgents:        23,
				SolvedToday:        12,
				PostedToday:        25,
				ProblemsSolved:     42,
				QuestionsAnswered:  18,
				HumansCount:        156,
				TotalPosts:         500,
				TotalContributions: 320,
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body map[string]interface{}) {
				data := body["data"].(map[string]interface{})
				checks := map[string]int{
					"active_posts":        147,
					"total_agents":        23,
					"solved_today":        12,
					"posted_today":        25,
					"problems_solved":     42,
					"questions_answered":  18,
					"humans_count":        156,
					"total_posts":         500,
					"total_contributions": 320,
				}
				for field, expected := range checks {
					got := int(data[field].(float64))
					if got != expected {
						t.Errorf("expected %s=%d, got %d", field, expected, got)
					}
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

func TestStatsHandler_GetProblemsStats(t *testing.T) {
	t.Run("returns all four stats fields", func(t *testing.T) {
		mockRepo := &MockStatsRepository{
			ProblemsStatsResult: map[string]any{
				"total_problems":      42,
				"solved_count":        15,
				"active_approaches":   23,
				"avg_solve_time_days": 7,
			},
		}
		handler := NewStatsHandler(mockRepo)
		req := httptest.NewRequest("GET", "/v1/stats/problems", nil)
		rec := httptest.NewRecorder()

		handler.GetProblemsStats(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rec.Code)
		}

		var body map[string]interface{}
		if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		data := body["data"].(map[string]interface{})
		checks := map[string]int{
			"total_problems":      42,
			"solved_count":        15,
			"active_approaches":   23,
			"avg_solve_time_days": 7,
		}
		for field, expected := range checks {
			got := int(data[field].(float64))
			if got != expected {
				t.Errorf("expected %s=%d, got %d", field, expected, got)
			}
		}
	})

	t.Run("returns zeros for empty database", func(t *testing.T) {
		mockRepo := &MockStatsRepository{
			ProblemsStatsResult: map[string]any{
				"total_problems":      0,
				"solved_count":        0,
				"active_approaches":   0,
				"avg_solve_time_days": 0,
			},
		}
		handler := NewStatsHandler(mockRepo)
		req := httptest.NewRequest("GET", "/v1/stats/problems", nil)
		rec := httptest.NewRecorder()

		handler.GetProblemsStats(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rec.Code)
		}

		var body map[string]interface{}
		if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		data := body["data"].(map[string]interface{})
		if int(data["total_problems"].(float64)) != 0 {
			t.Errorf("expected total_problems=0, got %v", data["total_problems"])
		}
	})

	t.Run("returns 500 on repository error", func(t *testing.T) {
		mockRepo := &MockStatsRepository{
			ProblemsStatsErr: fmt.Errorf("database error"),
		}
		handler := NewStatsHandler(mockRepo)
		req := httptest.NewRequest("GET", "/v1/stats/problems", nil)
		rec := httptest.NewRecorder()

		handler.GetProblemsStats(rec, req)

		if rec.Code != http.StatusInternalServerError {
			t.Errorf("expected status 500, got %d", rec.Code)
		}

		var body map[string]interface{}
		if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		errObj := body["error"].(map[string]interface{})
		if errObj["code"] != "INTERNAL_ERROR" {
			t.Errorf("expected error code INTERNAL_ERROR, got %v", errObj["code"])
		}
	})

	t.Run("includes recently_solved in response", func(t *testing.T) {
		mockRepo := &MockStatsRepository{
			ProblemsStatsResult: map[string]any{
				"total_problems":      10,
				"solved_count":        3,
				"active_approaches":   5,
				"avg_solve_time_days": 2,
			},
			RecentlySolvedResult: []map[string]any{
				{"id": "p1", "title": "Fix auth bug", "solver_name": "agent-x", "solver_type": "agent", "time_to_solve_days": 3},
				{"id": "p2", "title": "Memory leak", "solver_name": "alice", "solver_type": "human", "time_to_solve_days": 1},
			},
		}
		handler := NewStatsHandler(mockRepo)
		req := httptest.NewRequest("GET", "/v1/stats/problems", nil)
		rec := httptest.NewRecorder()

		handler.GetProblemsStats(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rec.Code)
		}

		var body map[string]interface{}
		if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		data := body["data"].(map[string]interface{})
		recentlySolved := data["recently_solved"].([]interface{})
		if len(recentlySolved) != 2 {
			t.Errorf("expected 2 recently solved, got %d", len(recentlySolved))
		}

		first := recentlySolved[0].(map[string]interface{})
		if first["title"] != "Fix auth bug" {
			t.Errorf("expected first title 'Fix auth bug', got %v", first["title"])
		}
		if first["solver_type"] != "agent" {
			t.Errorf("expected solver_type 'agent', got %v", first["solver_type"])
		}
	})

	t.Run("recently_solved is empty array when none exist", func(t *testing.T) {
		mockRepo := &MockStatsRepository{
			ProblemsStatsResult: map[string]any{
				"total_problems":      5,
				"solved_count":        0,
				"active_approaches":   2,
				"avg_solve_time_days": 0,
			},
		}
		handler := NewStatsHandler(mockRepo)
		req := httptest.NewRequest("GET", "/v1/stats/problems", nil)
		rec := httptest.NewRecorder()

		handler.GetProblemsStats(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rec.Code)
		}

		var body map[string]interface{}
		if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		data := body["data"].(map[string]interface{})
		recentlySolved := data["recently_solved"].([]interface{})
		if len(recentlySolved) != 0 {
			t.Errorf("expected 0 recently solved, got %d", len(recentlySolved))
		}
	})
}
