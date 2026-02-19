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

// MockSearchRepository implements SearchRepositoryInterface for testing.
type MockSearchRepository struct {
	results     []models.SearchResult
	total       int
	method      string
	searchErr   error
	searchQuery string
	searchOpts  models.SearchOptions
}

func NewMockSearchRepository() *MockSearchRepository {
	return &MockSearchRepository{
		results: []models.SearchResult{},
	}
}

func (m *MockSearchRepository) Search(ctx context.Context, query string, opts models.SearchOptions) ([]models.SearchResult, int, string, error) {
	m.searchQuery = query
	m.searchOpts = opts
	if m.searchErr != nil {
		return nil, 0, "", m.searchErr
	}
	method := m.method
	if method == "" {
		method = "fulltext"
	}
	return m.results, m.total, method, nil
}

// SetResults configures mock to return specific results.
func (m *MockSearchRepository) SetResults(results []models.SearchResult, total int) {
	m.results = results
	m.total = total
}

// SetMethod configures mock to return a specific search method.
func (m *MockSearchRepository) SetMethod(method string) {
	m.method = method
}

// SetError configures mock to return an error.
func (m *MockSearchRepository) SetError(err error) {
	m.searchErr = err
}

// TestSearch_MissingQuery tests that search returns 400 if q param is missing.
func TestSearch_MissingQuery(t *testing.T) {
	repo := NewMockSearchRepository()
	handler := NewSearchHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/search", nil)
	w := httptest.NewRecorder()

	handler.Search(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	errObj, ok := resp["error"].(map[string]interface{})
	if !ok {
		t.Fatal("expected error object in response")
	}

	if errObj["code"] != "VALIDATION_ERROR" {
		t.Errorf("expected error code VALIDATION_ERROR, got %v", errObj["code"])
	}
}

// TestSearch_EmptyQuery tests that search returns 400 if q param is empty.
func TestSearch_EmptyQuery(t *testing.T) {
	repo := NewMockSearchRepository()
	handler := NewSearchHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/search?q=", nil)
	w := httptest.NewRecorder()

	handler.Search(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	errObj, ok := resp["error"].(map[string]interface{})
	if !ok {
		t.Fatal("expected error object in response")
	}

	if errObj["code"] != "VALIDATION_ERROR" {
		t.Errorf("expected error code VALIDATION_ERROR, got %v", errObj["code"])
	}
}

// TestSearch_Success tests successful search with results.
func TestSearch_Success(t *testing.T) {
	repo := NewMockSearchRepository()
	now := time.Now()
	repo.SetResults([]models.SearchResult{
		{
			ID:           "post-123",
			Type:         "problem",
			Title:        "Race condition in PostgreSQL",
			Snippet:      "encountering a <mark>race condition</mark>",
			Tags:         []string{"postgresql", "async"},
			Status:       "solved",
			AuthorID:     "agent_claude",
			AuthorType:   "agent",
			AuthorName:   "Claude",
			Score:        0.95,
			VoteScore:        42,
			AnswersCount: 5,
			CreatedAt:    now,
		},
	}, 1)

	handler := NewSearchHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/search?q=race+condition", nil)
	w := httptest.NewRecorder()

	handler.Search(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	data, ok := resp["data"].([]interface{})
	if !ok {
		t.Fatal("expected data array in response")
	}

	if len(data) != 1 {
		t.Errorf("expected 1 result, got %d", len(data))
	}

	meta, ok := resp["meta"].(map[string]interface{})
	if !ok {
		t.Fatal("expected meta object in response")
	}

	if meta["query"] != "race condition" {
		t.Errorf("expected query 'race condition', got %v", meta["query"])
	}

	if meta["total"].(float64) != 1 {
		t.Errorf("expected total 1, got %v", meta["total"])
	}
}

// TestSearch_TypeFilterProblem tests filtering by type=problem.
func TestSearch_TypeFilterProblem(t *testing.T) {
	repo := NewMockSearchRepository()
	repo.SetResults([]models.SearchResult{}, 0)

	handler := NewSearchHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/search?q=test&type=problem", nil)
	w := httptest.NewRecorder()

	handler.Search(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	// Verify the repository received the correct type filter
	if repo.searchOpts.Type != "problem" {
		t.Errorf("expected type filter 'problem', got '%s'", repo.searchOpts.Type)
	}
}

// TestSearch_TypeFilterQuestion tests filtering by type=question.
func TestSearch_TypeFilterQuestion(t *testing.T) {
	repo := NewMockSearchRepository()
	repo.SetResults([]models.SearchResult{}, 0)

	handler := NewSearchHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/search?q=test&type=question", nil)
	w := httptest.NewRecorder()

	handler.Search(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	if repo.searchOpts.Type != "question" {
		t.Errorf("expected type filter 'question', got '%s'", repo.searchOpts.Type)
	}
}

// TestSearch_TypeFilterIdea tests filtering by type=idea.
func TestSearch_TypeFilterIdea(t *testing.T) {
	repo := NewMockSearchRepository()
	repo.SetResults([]models.SearchResult{}, 0)

	handler := NewSearchHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/search?q=test&type=idea", nil)
	w := httptest.NewRecorder()

	handler.Search(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	if repo.searchOpts.Type != "idea" {
		t.Errorf("expected type filter 'idea', got '%s'", repo.searchOpts.Type)
	}
}

// TestSearch_TagsFilter tests filtering by tags.
func TestSearch_TagsFilter(t *testing.T) {
	repo := NewMockSearchRepository()
	repo.SetResults([]models.SearchResult{}, 0)

	handler := NewSearchHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/search?q=test&tags=go,postgresql", nil)
	w := httptest.NewRecorder()

	handler.Search(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	if len(repo.searchOpts.Tags) != 2 {
		t.Errorf("expected 2 tags, got %d", len(repo.searchOpts.Tags))
	}

	if repo.searchOpts.Tags[0] != "go" || repo.searchOpts.Tags[1] != "postgresql" {
		t.Errorf("expected tags [go, postgresql], got %v", repo.searchOpts.Tags)
	}
}

// TestSearch_StatusFilter tests filtering by status.
func TestSearch_StatusFilter(t *testing.T) {
	repo := NewMockSearchRepository()
	repo.SetResults([]models.SearchResult{}, 0)

	handler := NewSearchHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/search?q=test&status=solved", nil)
	w := httptest.NewRecorder()

	handler.Search(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	if repo.searchOpts.Status != "solved" {
		t.Errorf("expected status filter 'solved', got '%s'", repo.searchOpts.Status)
	}
}

// TestSearch_AuthorFilter tests filtering by author.
func TestSearch_AuthorFilter(t *testing.T) {
	repo := NewMockSearchRepository()
	repo.SetResults([]models.SearchResult{}, 0)

	handler := NewSearchHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/search?q=test&author=agent_claude", nil)
	w := httptest.NewRecorder()

	handler.Search(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	if repo.searchOpts.Author != "agent_claude" {
		t.Errorf("expected author filter 'agent_claude', got '%s'", repo.searchOpts.Author)
	}
}

// TestSearch_AuthorTypeFilter tests filtering by author_type.
func TestSearch_AuthorTypeFilter(t *testing.T) {
	repo := NewMockSearchRepository()
	repo.SetResults([]models.SearchResult{}, 0)

	handler := NewSearchHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/search?q=test&author_type=agent", nil)
	w := httptest.NewRecorder()

	handler.Search(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	if repo.searchOpts.AuthorType != "agent" {
		t.Errorf("expected author_type filter 'agent', got '%s'", repo.searchOpts.AuthorType)
	}
}

// TestSearch_SortNewest tests sorting by newest.
func TestSearch_SortNewest(t *testing.T) {
	repo := NewMockSearchRepository()
	repo.SetResults([]models.SearchResult{}, 0)

	handler := NewSearchHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/search?q=test&sort=newest", nil)
	w := httptest.NewRecorder()

	handler.Search(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	if repo.searchOpts.Sort != "newest" {
		t.Errorf("expected sort 'newest', got '%s'", repo.searchOpts.Sort)
	}
}

// TestSearch_SortVotes tests sorting by votes.
func TestSearch_SortVotes(t *testing.T) {
	repo := NewMockSearchRepository()
	repo.SetResults([]models.SearchResult{}, 0)

	handler := NewSearchHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/search?q=test&sort=votes", nil)
	w := httptest.NewRecorder()

	handler.Search(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	if repo.searchOpts.Sort != "votes" {
		t.Errorf("expected sort 'votes', got '%s'", repo.searchOpts.Sort)
	}
}

// TestSearch_SortActivity tests sorting by activity.
func TestSearch_SortActivity(t *testing.T) {
	repo := NewMockSearchRepository()
	repo.SetResults([]models.SearchResult{}, 0)

	handler := NewSearchHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/search?q=test&sort=activity", nil)
	w := httptest.NewRecorder()

	handler.Search(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	if repo.searchOpts.Sort != "activity" {
		t.Errorf("expected sort 'activity', got '%s'", repo.searchOpts.Sort)
	}
}

// TestSearch_DefaultSort tests that default sort is relevance.
func TestSearch_DefaultSort(t *testing.T) {
	repo := NewMockSearchRepository()
	repo.SetResults([]models.SearchResult{}, 0)

	handler := NewSearchHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/search?q=test", nil)
	w := httptest.NewRecorder()

	handler.Search(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	if repo.searchOpts.Sort != "relevance" {
		t.Errorf("expected default sort 'relevance', got '%s'", repo.searchOpts.Sort)
	}
}

// TestSearch_Pagination tests pagination parameters.
func TestSearch_Pagination(t *testing.T) {
	repo := NewMockSearchRepository()
	repo.SetResults([]models.SearchResult{}, 100)

	handler := NewSearchHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/search?q=test&page=2&per_page=10", nil)
	w := httptest.NewRecorder()

	handler.Search(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	if repo.searchOpts.Page != 2 {
		t.Errorf("expected page 2, got %d", repo.searchOpts.Page)
	}

	if repo.searchOpts.PerPage != 10 {
		t.Errorf("expected per_page 10, got %d", repo.searchOpts.PerPage)
	}
}

// TestSearch_PaginationDefaults tests default pagination values.
func TestSearch_PaginationDefaults(t *testing.T) {
	repo := NewMockSearchRepository()
	repo.SetResults([]models.SearchResult{}, 0)

	handler := NewSearchHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/search?q=test", nil)
	w := httptest.NewRecorder()

	handler.Search(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	if repo.searchOpts.Page != 1 {
		t.Errorf("expected default page 1, got %d", repo.searchOpts.Page)
	}

	if repo.searchOpts.PerPage != 20 {
		t.Errorf("expected default per_page 20, got %d", repo.searchOpts.PerPage)
	}
}

// TestSearch_PerPageMax tests that per_page is capped at 50.
func TestSearch_PerPageMax(t *testing.T) {
	repo := NewMockSearchRepository()
	repo.SetResults([]models.SearchResult{}, 0)

	handler := NewSearchHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/search?q=test&per_page=100", nil)
	w := httptest.NewRecorder()

	handler.Search(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	if repo.searchOpts.PerPage != 50 {
		t.Errorf("expected per_page capped at 50, got %d", repo.searchOpts.PerPage)
	}
}

// TestSearch_HasMore tests the has_more flag in pagination.
func TestSearch_HasMore(t *testing.T) {
	repo := NewMockSearchRepository()
	repo.SetResults([]models.SearchResult{
		{ID: "1", Type: "problem", Title: "Test", CreatedAt: time.Now()},
	}, 50) // 50 total results

	handler := NewSearchHandler(repo)

	// Page 1, per_page 20 - should have more
	req := httptest.NewRequest(http.MethodGet, "/v1/search?q=test&page=1&per_page=20", nil)
	w := httptest.NewRecorder()

	handler.Search(w, req)

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	meta := resp["meta"].(map[string]interface{})
	if meta["has_more"] != true {
		t.Errorf("expected has_more true when more results exist")
	}
}

// TestSearch_HasMoreFalse tests has_more is false on last page.
func TestSearch_HasMoreFalse(t *testing.T) {
	repo := NewMockSearchRepository()
	repo.SetResults([]models.SearchResult{
		{ID: "1", Type: "problem", Title: "Test", CreatedAt: time.Now()},
	}, 1) // Only 1 result total

	handler := NewSearchHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/search?q=test&page=1&per_page=20", nil)
	w := httptest.NewRecorder()

	handler.Search(w, req)

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	meta := resp["meta"].(map[string]interface{})
	if meta["has_more"] != false {
		t.Errorf("expected has_more false when no more results")
	}
}

// TestSearch_IncludesTookMs tests that took_ms is included in response.
func TestSearch_IncludesTookMs(t *testing.T) {
	repo := NewMockSearchRepository()
	repo.SetResults([]models.SearchResult{}, 0)

	handler := NewSearchHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/search?q=test", nil)
	w := httptest.NewRecorder()

	handler.Search(w, req)

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	meta := resp["meta"].(map[string]interface{})
	if _, ok := meta["took_ms"]; !ok {
		t.Error("expected took_ms in meta")
	}
}

// TestSearch_DateFilters tests from_date and to_date filters.
func TestSearch_DateFilters(t *testing.T) {
	repo := NewMockSearchRepository()
	repo.SetResults([]models.SearchResult{}, 0)

	handler := NewSearchHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/search?q=test&from_date=2026-01-01&to_date=2026-01-31", nil)
	w := httptest.NewRecorder()

	handler.Search(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	expectedFrom := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	if !repo.searchOpts.FromDate.Equal(expectedFrom) {
		t.Errorf("expected from_date %v, got %v", expectedFrom, repo.searchOpts.FromDate)
	}

	expectedTo := time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC)
	if !repo.searchOpts.ToDate.Equal(expectedTo) {
		t.Errorf("expected to_date %v, got %v", expectedTo, repo.searchOpts.ToDate)
	}
}

// TestSearch_InvalidDateFormat tests that invalid date format returns 400.
func TestSearch_InvalidDateFormat(t *testing.T) {
	repo := NewMockSearchRepository()
	handler := NewSearchHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/search?q=test&from_date=invalid", nil)
	w := httptest.NewRecorder()

	handler.Search(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

// TestSearch_EmptyResults tests search with no results.
func TestSearch_EmptyResults(t *testing.T) {
	repo := NewMockSearchRepository()
	repo.SetResults([]models.SearchResult{}, 0)

	handler := NewSearchHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/search?q=nonexistent", nil)
	w := httptest.NewRecorder()

	handler.Search(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	data := resp["data"].([]interface{})
	if len(data) != 0 {
		t.Errorf("expected empty results, got %d", len(data))
	}
}
