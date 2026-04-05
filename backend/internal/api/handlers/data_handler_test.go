package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// mockDataAnalyticsReader implements DataAnalyticsReaderInterface for testing.
type mockDataAnalyticsReader struct {
	trending   []models.TrendingSearch
	breakdown  models.DataBreakdown
	categories []models.DataCategory
	err        error
	callCount  int
}

func (m *mockDataAnalyticsReader) GetTrendingPublic(_ context.Context, _ string, _ int, _ bool) ([]models.TrendingSearch, error) {
	m.callCount++
	return m.trending, m.err
}

func (m *mockDataAnalyticsReader) GetBreakdown(_ context.Context, _ string, _ bool) (models.DataBreakdown, error) {
	m.callCount++
	return m.breakdown, m.err
}

func (m *mockDataAnalyticsReader) GetCategories(_ context.Context, _ string, _ bool) ([]models.DataCategory, error) {
	m.callCount++
	return m.categories, m.err
}

// TestDataHandler_Trending_Success verifies GET /v1/data/trending returns 200 with trending data.
func TestDataHandler_Trending_Success(t *testing.T) {
	mock := &mockDataAnalyticsReader{
		trending: []models.TrendingSearch{
			{Query: "golang error handling", Count: 15, AvgResults: 5.0, AvgDuration: 200.0},
			{Query: "postgres connection", Count: 8, AvgResults: 3.0, AvgDuration: 150.0},
		},
	}
	handler := NewDataHandler(mock)

	req := httptest.NewRequest("GET", "/v1/data/trending?window=24h", nil)
	rr := httptest.NewRecorder()
	handler.GetTrending(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Data struct {
			Trending []struct {
				Query string `json:"query"`
				Count int    `json:"count"`
			} `json:"trending"`
			Window string `json:"window"`
		} `json:"data"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(resp.Data.Trending) != 2 {
		t.Errorf("expected 2 trending results, got %d", len(resp.Data.Trending))
	}
	if resp.Data.Window != "24h" {
		t.Errorf("expected window=24h, got %q", resp.Data.Window)
	}
}

// TestDataHandler_Trending_DefaultWindow verifies missing window param defaults to 24h.
func TestDataHandler_Trending_DefaultWindow(t *testing.T) {
	mock := &mockDataAnalyticsReader{
		trending: []models.TrendingSearch{},
	}
	handler := NewDataHandler(mock)

	req := httptest.NewRequest("GET", "/v1/data/trending", nil)
	rr := httptest.NewRecorder()
	handler.GetTrending(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Data struct {
			Window string `json:"window"`
		} `json:"data"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Data.Window != "24h" {
		t.Errorf("expected default window=24h, got %q", resp.Data.Window)
	}
}

// TestDataHandler_Trending_InvalidWindow verifies invalid window returns 400.
func TestDataHandler_Trending_InvalidWindow(t *testing.T) {
	handler := NewDataHandler(&mockDataAnalyticsReader{})

	req := httptest.NewRequest("GET", "/v1/data/trending?window=invalid", nil)
	rr := httptest.NewRecorder()
	handler.GetTrending(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}
}

// TestDataHandler_Breakdown_Success verifies GET /v1/data/breakdown returns 200 with breakdown data.
func TestDataHandler_Breakdown_Success(t *testing.T) {
	mock := &mockDataAnalyticsReader{
		breakdown: models.DataBreakdown{
			TotalSearches:  150,
			ZeroResultRate: 0.12,
			BySearcherType: map[string]int{
				"agent":     80,
				"human":     50,
				"anonymous": 20,
			},
		},
	}
	handler := NewDataHandler(mock)

	req := httptest.NewRequest("GET", "/v1/data/breakdown?window=7d", nil)
	rr := httptest.NewRecorder()
	handler.GetBreakdown(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Data struct {
			TotalSearches  int            `json:"total_searches"`
			ZeroResultRate float64        `json:"zero_result_rate"`
			BySearcherType map[string]int `json:"by_searcher_type"`
			Window         string         `json:"window"`
		} `json:"data"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Data.TotalSearches != 150 {
		t.Errorf("expected total_searches=150, got %d", resp.Data.TotalSearches)
	}
	if resp.Data.Window != "7d" {
		t.Errorf("expected window=7d, got %q", resp.Data.Window)
	}
	if resp.Data.BySearcherType["agent"] != 80 {
		t.Errorf("expected agent=80, got %d", resp.Data.BySearcherType["agent"])
	}
}

// TestDataHandler_Categories_Success verifies GET /v1/data/categories returns 200 with category data.
func TestDataHandler_Categories_Success(t *testing.T) {
	mock := &mockDataAnalyticsReader{
		categories: []models.DataCategory{
			{Category: "problems", SearchCount: 120},
			{Category: "unfiltered", SearchCount: 80},
		},
	}
	handler := NewDataHandler(mock)

	req := httptest.NewRequest("GET", "/v1/data/categories?window=1h", nil)
	rr := httptest.NewRecorder()
	handler.GetCategories(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Data struct {
			Categories []struct {
				Category    string `json:"category"`
				SearchCount int    `json:"search_count"`
			} `json:"categories"`
			Window string `json:"window"`
		} `json:"data"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(resp.Data.Categories) != 2 {
		t.Errorf("expected 2 categories, got %d", len(resp.Data.Categories))
	}
	if resp.Data.Window != "1h" {
		t.Errorf("expected window=1h, got %q", resp.Data.Window)
	}
}

// TestDataHandler_Trending_NoAuth verifies that no auth header is required (public endpoint).
func TestDataHandler_Trending_NoAuth(t *testing.T) {
	mock := &mockDataAnalyticsReader{
		trending: []models.TrendingSearch{},
	}
	handler := NewDataHandler(mock)

	// No Authorization header, no X-Admin-API-Key — should still return 200
	req := httptest.NewRequest("GET", "/v1/data/trending", nil)
	rr := httptest.NewRecorder()
	handler.GetTrending(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200 (no auth needed), got %d: %s", rr.Code, rr.Body.String())
	}
}

// TestDataHandler_Trending_CacheHit verifies that repeated calls within 60s return cached response
// (repo is called only once).
func TestDataHandler_Trending_CacheHit(t *testing.T) {
	mock := &mockDataAnalyticsReader{
		trending: []models.TrendingSearch{
			{Query: "cached query", Count: 5, AvgResults: 2.0, AvgDuration: 100.0},
		},
	}
	handler := NewDataHandler(mock)

	// First call — should populate cache
	req1 := httptest.NewRequest("GET", "/v1/data/trending?window=24h", nil)
	rr1 := httptest.NewRecorder()
	handler.GetTrending(rr1, req1)
	if rr1.Code != http.StatusOK {
		t.Fatalf("first call: expected 200, got %d", rr1.Code)
	}

	// Second call — should hit cache
	req2 := httptest.NewRequest("GET", "/v1/data/trending?window=24h", nil)
	rr2 := httptest.NewRecorder()
	handler.GetTrending(rr2, req2)
	if rr2.Code != http.StatusOK {
		t.Fatalf("second call: expected 200, got %d", rr2.Code)
	}

	// Repo should only have been called once
	if mock.callCount != 1 {
		t.Errorf("expected repo called 1 time (cache hit), got %d", mock.callCount)
	}
}

// TestDataHandler_Trending_IncludeBots verifies include_bots=true param is handled.
func TestDataHandler_Trending_IncludeBots(t *testing.T) {
	mock := &mockDataAnalyticsReader{
		trending: []models.TrendingSearch{
			{Query: "bot query", Count: 200, AvgResults: 0.5, AvgDuration: 50.0},
		},
	}
	handler := NewDataHandler(mock)

	// include_bots=true should use a different cache key and call repo
	req := httptest.NewRequest("GET", "/v1/data/trending?window=24h&include_bots=true", nil)
	rr := httptest.NewRecorder()
	handler.GetTrending(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if mock.callCount != 1 {
		t.Errorf("expected repo called once with include_bots=true, got %d", mock.callCount)
	}
}

// TestDataHandler_Trending_StripsAvgFields verifies avg_results and avg_duration are NOT in public response.
func TestDataHandler_Trending_StripsAvgFields(t *testing.T) {
	mock := &mockDataAnalyticsReader{
		trending: []models.TrendingSearch{
			{Query: "test", Count: 10, AvgResults: 5.0, AvgDuration: 200.0},
		},
	}
	handler := NewDataHandler(mock)

	req := httptest.NewRequest("GET", "/v1/data/trending?window=24h", nil)
	rr := httptest.NewRecorder()
	handler.GetTrending(rr, req)

	var raw map[string]any
	json.NewDecoder(rr.Body).Decode(&raw)
	data := raw["data"].(map[string]any)
	trending := data["trending"].([]any)
	if len(trending) > 0 {
		item := trending[0].(map[string]any)
		if _, exists := item["avg_results"]; exists {
			t.Error("public trending should NOT contain avg_results")
		}
		if _, exists := item["avg_duration_ms"]; exists {
			t.Error("public trending should NOT contain avg_duration_ms")
		}
	}
}
