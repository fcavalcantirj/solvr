package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// mockSearchAnalyticsReader implements SearchAnalyticsReaderInterface for testing.
type mockSearchAnalyticsReader struct {
	trending    []models.TrendingSearch
	zeroResults []models.TrendingSearch
	summary     models.SearchAnalytics
	err         error
}

func (m *mockSearchAnalyticsReader) GetTrending(_ context.Context, _ int, _ int) ([]models.TrendingSearch, error) {
	return m.trending, m.err
}

func (m *mockSearchAnalyticsReader) GetZeroResults(_ context.Context, _ int, _ int) ([]models.TrendingSearch, error) {
	return m.zeroResults, m.err
}

func (m *mockSearchAnalyticsReader) GetSummary(_ context.Context, _ int) (models.SearchAnalytics, error) {
	return m.summary, m.err
}

func TestSearchAnalyticsHandler_GetTrending(t *testing.T) {
	// Set admin key for auth
	os.Setenv("ADMIN_API_KEY", "test-admin-key")
	defer os.Unsetenv("ADMIN_API_KEY")

	mock := &mockSearchAnalyticsReader{
		trending: []models.TrendingSearch{
			{Query: "golang error", Count: 10, AvgResults: 5.0, AvgDuration: 200.0},
			{Query: "database connection", Count: 7, AvgResults: 3.0, AvgDuration: 150.0},
		},
		zeroResults: []models.TrendingSearch{
			{Query: "missing topic", Count: 3, AvgResults: 0, AvgDuration: 100.0},
		},
	}

	handler := NewSearchAnalyticsHandler(mock)

	req := httptest.NewRequest("GET", "/admin/search-analytics/trending?days=7&limit=10", nil)
	req.Header.Set("X-Admin-API-Key", "test-admin-key")
	rr := httptest.NewRecorder()

	handler.GetTrending(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	body := rr.Body.String()
	if body == "" {
		t.Error("expected non-empty response body")
	}
}

func TestSearchAnalyticsHandler_GetTrending_NoAuth(t *testing.T) {
	os.Setenv("ADMIN_API_KEY", "test-admin-key")
	defer os.Unsetenv("ADMIN_API_KEY")

	handler := NewSearchAnalyticsHandler(&mockSearchAnalyticsReader{})

	req := httptest.NewRequest("GET", "/admin/search-analytics/trending", nil)
	rr := httptest.NewRecorder()

	handler.GetTrending(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rr.Code)
	}
}

func TestSearchAnalyticsHandler_GetTrending_WrongKey(t *testing.T) {
	os.Setenv("ADMIN_API_KEY", "test-admin-key")
	defer os.Unsetenv("ADMIN_API_KEY")

	handler := NewSearchAnalyticsHandler(&mockSearchAnalyticsReader{})

	req := httptest.NewRequest("GET", "/admin/search-analytics/trending", nil)
	req.Header.Set("X-Admin-API-Key", "wrong-key")
	rr := httptest.NewRecorder()

	handler.GetTrending(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected status 403, got %d", rr.Code)
	}
}

func TestSearchAnalyticsHandler_GetSummary(t *testing.T) {
	os.Setenv("ADMIN_API_KEY", "test-admin-key")
	defer os.Unsetenv("ADMIN_API_KEY")

	mock := &mockSearchAnalyticsReader{
		summary: models.SearchAnalytics{
			TotalSearches:  100,
			UniqueQueries:  50,
			AvgDurationMs:  175.5,
			ZeroResultRate: 0.12,
			BySearcherType: map[string]int{"human": 40, "agent": 50, "anonymous": 10},
			TopQueries: []models.TrendingSearch{
				{Query: "error handling", Count: 15},
			},
			TopZeroResults: []models.TrendingSearch{
				{Query: "nonexistent topic", Count: 5},
			},
		},
	}

	handler := NewSearchAnalyticsHandler(mock)

	req := httptest.NewRequest("GET", "/admin/search-analytics/summary?days=30", nil)
	req.Header.Set("X-Admin-API-Key", "test-admin-key")
	rr := httptest.NewRecorder()

	handler.GetSummary(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestSearchAnalyticsHandler_GetSummary_DefaultDays(t *testing.T) {
	os.Setenv("ADMIN_API_KEY", "test-admin-key")
	defer os.Unsetenv("ADMIN_API_KEY")

	mock := &mockSearchAnalyticsReader{
		summary: models.SearchAnalytics{
			BySearcherType: map[string]int{},
			TopQueries:     []models.TrendingSearch{},
			TopZeroResults: []models.TrendingSearch{},
		},
	}

	handler := NewSearchAnalyticsHandler(mock)

	// No days param — should default to 30
	req := httptest.NewRequest("GET", "/admin/search-analytics/summary", nil)
	req.Header.Set("X-Admin-API-Key", "test-admin-key")
	rr := httptest.NewRecorder()

	handler.GetSummary(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}
}
