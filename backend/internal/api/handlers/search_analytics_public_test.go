package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/models"
)

func TestSearchAnalyticsHandler_GetPublicSearchStats(t *testing.T) {
	mock := &mockSearchAnalyticsReader{
		summary: models.SearchAnalytics{
			TotalSearches: 100,
			BySearcherType: map[string]int{
				"human":     40,
				"agent":     50,
				"anonymous": 10,
			},
		},
		trending: []models.TrendingSearch{
			{Query: "golang error handling", Count: 15, AvgResults: 5.0, AvgDuration: 200.0},
			{Query: "postgres connection pool", Count: 8, AvgResults: 3.0, AvgDuration: 150.0},
		},
	}

	handler := NewSearchAnalyticsHandler(mock)

	// No admin auth required
	req := httptest.NewRequest("GET", "/v1/stats/search", nil)
	rr := httptest.NewRecorder()

	handler.GetPublicSearchStats(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Data struct {
			TotalSearches7d int `json:"total_searches_7d"`
			AgentSearches7d int `json:"agent_searches_7d"`
			HumanSearches7d int `json:"human_searches_7d"`
			TrendingQueries []struct {
				Query string `json:"query"`
				Count int    `json:"count"`
			} `json:"trending_queries"`
		} `json:"data"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Data.TotalSearches7d != 100 {
		t.Errorf("expected total_searches_7d=100, got %d", resp.Data.TotalSearches7d)
	}
	if resp.Data.AgentSearches7d != 50 {
		t.Errorf("expected agent_searches_7d=50, got %d", resp.Data.AgentSearches7d)
	}
	if resp.Data.HumanSearches7d != 40 {
		t.Errorf("expected human_searches_7d=40, got %d", resp.Data.HumanSearches7d)
	}
	if len(resp.Data.TrendingQueries) != 2 {
		t.Errorf("expected 2 trending queries, got %d", len(resp.Data.TrendingQueries))
	}
	if len(resp.Data.TrendingQueries) > 0 && resp.Data.TrendingQueries[0].Query != "golang error handling" {
		t.Errorf("expected first query 'golang error handling', got '%s'", resp.Data.TrendingQueries[0].Query)
	}
}

func TestSearchAnalyticsHandler_GetPublicSearchStats_NoAuth(t *testing.T) {
	// Public endpoint should work without any auth headers
	mock := &mockSearchAnalyticsReader{
		summary: models.SearchAnalytics{
			BySearcherType: map[string]int{},
		},
		trending: []models.TrendingSearch{},
	}

	handler := NewSearchAnalyticsHandler(mock)

	req := httptest.NewRequest("GET", "/v1/stats/search", nil)
	rr := httptest.NewRecorder()

	handler.GetPublicSearchStats(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200 (no auth needed), got %d", rr.Code)
	}
}

func TestSearchAnalyticsHandler_GetPublicSearchStats_EmptyData(t *testing.T) {
	mock := &mockSearchAnalyticsReader{
		summary: models.SearchAnalytics{
			TotalSearches:  0,
			BySearcherType: map[string]int{},
		},
		trending: []models.TrendingSearch{},
	}

	handler := NewSearchAnalyticsHandler(mock)

	req := httptest.NewRequest("GET", "/v1/stats/search", nil)
	rr := httptest.NewRecorder()

	handler.GetPublicSearchStats(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	var resp struct {
		Data struct {
			TotalSearches7d int `json:"total_searches_7d"`
			AgentSearches7d int `json:"agent_searches_7d"`
			HumanSearches7d int `json:"human_searches_7d"`
			TrendingQueries []struct {
				Query string `json:"query"`
				Count int    `json:"count"`
			} `json:"trending_queries"`
		} `json:"data"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Data.TotalSearches7d != 0 {
		t.Errorf("expected total_searches_7d=0, got %d", resp.Data.TotalSearches7d)
	}
	if resp.Data.TrendingQueries == nil {
		t.Error("expected trending_queries to be empty array, got nil")
	}
}

func TestSearchAnalyticsHandler_GetPublicSearchStats_StripsAvgDuration(t *testing.T) {
	// Verify that avg_duration and avg_results are NOT exposed in public response
	mock := &mockSearchAnalyticsReader{
		summary: models.SearchAnalytics{
			TotalSearches:  50,
			AvgDurationMs:  175.5, // Should NOT appear in public response
			BySearcherType: map[string]int{"agent": 30},
		},
		trending: []models.TrendingSearch{
			{Query: "test query", Count: 10, AvgResults: 5.0, AvgDuration: 200.0},
		},
	}

	handler := NewSearchAnalyticsHandler(mock)

	req := httptest.NewRequest("GET", "/v1/stats/search", nil)
	rr := httptest.NewRecorder()

	handler.GetPublicSearchStats(rr, req)

	// Parse raw JSON to check no sensitive fields leak
	var raw map[string]any
	json.NewDecoder(rr.Body).Decode(&raw)
	data := raw["data"].(map[string]any)

	if _, exists := data["avg_duration_ms"]; exists {
		t.Error("public response should NOT contain avg_duration_ms")
	}
	if _, exists := data["zero_result_rate"]; exists {
		t.Error("public response should NOT contain zero_result_rate")
	}

	queries := data["trending_queries"].([]any)
	if len(queries) > 0 {
		q := queries[0].(map[string]any)
		if _, exists := q["avg_results"]; exists {
			t.Error("trending queries should NOT contain avg_results")
		}
		if _, exists := q["avg_duration_ms"]; exists {
			t.Error("trending queries should NOT contain avg_duration_ms")
		}
	}
}
