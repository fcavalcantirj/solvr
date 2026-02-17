package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
)

// TestSearchIntegration_FullFlow tests the complete search integration:
// Create posts → Search → Verify results → Test filters → Test pagination
// This verifies the search functionality works end-to-end.
func TestSearchIntegration_FullFlow(t *testing.T) {
	repo := NewMockSearchRepositoryWithPosts()
	handler := NewSearchHandler(repo)

	// Step 1: Setup test data - create posts with different types and attributes
	t.Run("Step1_SetupTestData", func(t *testing.T) {
		now := time.Now()

		// Add various posts to the mock repository
		repo.AddPost(models.SearchResult{
			ID:           "problem-1",
			Type:         "problem",
			Title:        "Race condition in PostgreSQL async queries",
			Snippet:      "Encountering a <mark>race condition</mark> when running async queries",
			Tags:         []string{"postgresql", "async", "go"},
			Status:       "solved",
			AuthorID:     "agent_claude",
			AuthorType:   "agent",
			AuthorName:   "Claude",
			Score:        0.95,
			Votes:        42,
			AnswersCount: 5,
			CreatedAt:    now.Add(-24 * time.Hour),
		})

		repo.AddPost(models.SearchResult{
			ID:           "problem-2",
			Type:         "problem",
			Title:        "Memory leak in async worker pool",
			Snippet:      "Memory usage grows when using <mark>async</mark> worker pool",
			Tags:         []string{"memory", "async", "python"},
			Status:       "open",
			AuthorID:     "user-123",
			AuthorType:   "human",
			AuthorName:   "John Developer",
			Score:        0.88,
			Votes:        15,
			AnswersCount: 2,
			CreatedAt:    now.Add(-12 * time.Hour),
		})

		repo.AddPost(models.SearchResult{
			ID:           "question-1",
			Type:         "question",
			Title:        "How to handle async errors in Go?",
			Snippet:      "What's the best way to handle <mark>async</mark> errors in Go?",
			Tags:         []string{"go", "async", "error-handling"},
			Status:       "answered",
			AuthorID:     "agent_helper",
			AuthorType:   "agent",
			AuthorName:   "Helper Bot",
			Score:        0.82,
			Votes:        28,
			AnswersCount: 3,
			CreatedAt:    now.Add(-6 * time.Hour),
		})

		repo.AddPost(models.SearchResult{
			ID:           "idea-1",
			Type:         "idea",
			Title:        "Async error handling patterns",
			Snippet:      "Thoughts on <mark>async</mark> error handling patterns",
			Tags:         []string{"patterns", "async"},
			Status:       "active",
			AuthorID:     "user-456",
			AuthorType:   "human",
			AuthorName:   "Jane Coder",
			Score:        0.75,
			Votes:        10,
			AnswersCount: 0,
			CreatedAt:    now.Add(-1 * time.Hour),
		})

		if len(repo.posts) != 4 {
			t.Fatalf("Step 1 failed: expected 4 posts, got %d", len(repo.posts))
		}

		t.Logf("Step 1 passed: Created 4 test posts")
	})

	// Step 2: Basic search - search for "async" returns all matching posts
	t.Run("Step2_BasicSearch", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/search?q=async", nil)
		w := httptest.NewRecorder()

		handler.Search(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("Step 2 failed: expected status 200, got %d: %s", w.Code, w.Body.String())
		}

		var resp SearchResponse
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("Step 2 failed: failed to decode response: %v", err)
		}

		// Should find all 4 posts that contain "async"
		if len(resp.Data) != 4 {
			t.Errorf("Step 2: expected 4 results, got %d", len(resp.Data))
		}

		// Meta should reflect search query
		if resp.Meta.Query != "async" {
			t.Errorf("Step 2: expected query 'async', got '%s'", resp.Meta.Query)
		}

		if resp.Meta.Total != 4 {
			t.Errorf("Step 2: expected total 4, got %d", resp.Meta.Total)
		}

		// Verify took_ms is included
		if resp.Meta.TookMs < 0 {
			t.Error("Step 2: expected valid took_ms in response")
		}

		t.Logf("Step 2 passed: Basic search returned %d results in %dms", len(resp.Data), resp.Meta.TookMs)
	})

	// Step 3: Type filter - filter by type=problem
	t.Run("Step3_TypeFilter", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/search?q=async&type=problem", nil)
		w := httptest.NewRecorder()

		handler.Search(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("Step 3 failed: expected status 200, got %d", w.Code)
		}

		var resp SearchResponse
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("Step 3 failed: failed to decode response: %v", err)
		}

		// Should find 2 problems
		if len(resp.Data) != 2 {
			t.Errorf("Step 3: expected 2 problem results, got %d", len(resp.Data))
		}

		// Verify all results are problems
		for _, result := range resp.Data {
			if result.Type != "problem" {
				t.Errorf("Step 3: expected type 'problem', got '%s'", result.Type)
			}
		}

		t.Logf("Step 3 passed: Type filter returned %d problems", len(resp.Data))
	})

	// Step 4: Status filter - filter by status=solved
	t.Run("Step4_StatusFilter", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/search?q=async&status=solved", nil)
		w := httptest.NewRecorder()

		handler.Search(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("Step 4 failed: expected status 200, got %d", w.Code)
		}

		var resp SearchResponse
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("Step 4 failed: failed to decode response: %v", err)
		}

		// Should find 1 solved post
		if len(resp.Data) != 1 {
			t.Errorf("Step 4: expected 1 solved result, got %d", len(resp.Data))
		}

		if len(resp.Data) > 0 && resp.Data[0].Status != "solved" {
			t.Errorf("Step 4: expected status 'solved', got '%s'", resp.Data[0].Status)
		}

		t.Logf("Step 4 passed: Status filter returned %d solved posts", len(resp.Data))
	})

	// Step 5: Tags filter - filter by tags=go
	t.Run("Step5_TagsFilter", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/search?q=async&tags=go", nil)
		w := httptest.NewRecorder()

		handler.Search(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("Step 5 failed: expected status 200, got %d", w.Code)
		}

		var resp SearchResponse
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("Step 5 failed: failed to decode response: %v", err)
		}

		// Should find 2 posts with "go" tag
		if len(resp.Data) != 2 {
			t.Errorf("Step 5: expected 2 results with 'go' tag, got %d", len(resp.Data))
		}

		// Verify all results have "go" tag
		for _, result := range resp.Data {
			hasGoTag := false
			for _, tag := range result.Tags {
				if tag == "go" {
					hasGoTag = true
					break
				}
			}
			if !hasGoTag {
				t.Errorf("Step 5: expected 'go' tag in result, got %v", result.Tags)
			}
		}

		t.Logf("Step 5 passed: Tags filter returned %d posts with 'go' tag", len(resp.Data))
	})

	// Step 6: Author type filter - filter by author_type=agent
	t.Run("Step6_AuthorTypeFilter", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/search?q=async&author_type=agent", nil)
		w := httptest.NewRecorder()

		handler.Search(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("Step 6 failed: expected status 200, got %d", w.Code)
		}

		var resp SearchResponse
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("Step 6 failed: failed to decode response: %v", err)
		}

		// Should find 2 posts by agents
		if len(resp.Data) != 2 {
			t.Errorf("Step 6: expected 2 agent results, got %d", len(resp.Data))
		}

		// Verify all results are from agents
		for _, result := range resp.Data {
			if result.Author.Type != "agent" {
				t.Errorf("Step 6: expected author type 'agent', got '%s'", result.Author.Type)
			}
		}

		t.Logf("Step 6 passed: Author type filter returned %d agent posts", len(resp.Data))
	})

	// Step 7: Sort by votes
	t.Run("Step7_SortByVotes", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/search?q=async&sort=votes", nil)
		w := httptest.NewRecorder()

		handler.Search(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("Step 7 failed: expected status 200, got %d", w.Code)
		}

		var resp SearchResponse
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("Step 7 failed: failed to decode response: %v", err)
		}

		if len(resp.Data) < 2 {
			t.Fatalf("Step 7: expected at least 2 results, got %d", len(resp.Data))
		}

		// Verify sorted by votes descending
		for i := 1; i < len(resp.Data); i++ {
			if resp.Data[i].Votes > resp.Data[i-1].Votes {
				t.Errorf("Step 7: results not sorted by votes, got %d > %d",
					resp.Data[i].Votes, resp.Data[i-1].Votes)
			}
		}

		t.Logf("Step 7 passed: Sort by votes works correctly")
	})

	// Step 8: Sort by newest
	t.Run("Step8_SortByNewest", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/search?q=async&sort=newest", nil)
		w := httptest.NewRecorder()

		handler.Search(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("Step 8 failed: expected status 200, got %d", w.Code)
		}

		var resp SearchResponse
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("Step 8 failed: failed to decode response: %v", err)
		}

		if len(resp.Data) < 2 {
			t.Fatalf("Step 8: expected at least 2 results, got %d", len(resp.Data))
		}

		// Verify sorted by created_at descending (newest first)
		for i := 1; i < len(resp.Data); i++ {
			if resp.Data[i].CreatedAt.After(resp.Data[i-1].CreatedAt) {
				t.Errorf("Step 8: results not sorted by newest, newer item found after older")
			}
		}

		t.Logf("Step 8 passed: Sort by newest works correctly")
	})

	// Step 9: Pagination
	t.Run("Step9_Pagination", func(t *testing.T) {
		// First page with 2 per page
		req := httptest.NewRequest(http.MethodGet, "/v1/search?q=async&page=1&per_page=2", nil)
		w := httptest.NewRecorder()

		handler.Search(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("Step 9 failed: expected status 200, got %d", w.Code)
		}

		var resp SearchResponse
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("Step 9 failed: failed to decode response: %v", err)
		}

		if len(resp.Data) != 2 {
			t.Errorf("Step 9: expected 2 results on page 1, got %d", len(resp.Data))
		}

		if resp.Meta.Page != 1 {
			t.Errorf("Step 9: expected page 1, got %d", resp.Meta.Page)
		}

		if resp.Meta.PerPage != 2 {
			t.Errorf("Step 9: expected per_page 2, got %d", resp.Meta.PerPage)
		}

		if resp.Meta.Total != 4 {
			t.Errorf("Step 9: expected total 4, got %d", resp.Meta.Total)
		}

		if !resp.Meta.HasMore {
			t.Error("Step 9: expected has_more=true on first page")
		}

		t.Logf("Step 9 passed: Pagination works correctly")
	})

	// Step 10: Second page
	t.Run("Step10_SecondPage", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/search?q=async&page=2&per_page=2", nil)
		w := httptest.NewRecorder()

		handler.Search(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("Step 10 failed: expected status 200, got %d", w.Code)
		}

		var resp SearchResponse
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("Step 10 failed: failed to decode response: %v", err)
		}

		if len(resp.Data) != 2 {
			t.Errorf("Step 10: expected 2 results on page 2, got %d", len(resp.Data))
		}

		if resp.Meta.Page != 2 {
			t.Errorf("Step 10: expected page 2, got %d", resp.Meta.Page)
		}

		if resp.Meta.HasMore {
			t.Error("Step 10: expected has_more=false on last page")
		}

		t.Logf("Step 10 passed: Second page works correctly")
	})

	// Step 11: Combined filters
	t.Run("Step11_CombinedFilters", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/search?q=async&type=problem&tags=go&sort=votes", nil)
		w := httptest.NewRecorder()

		handler.Search(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("Step 11 failed: expected status 200, got %d", w.Code)
		}

		var resp SearchResponse
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("Step 11 failed: failed to decode response: %v", err)
		}

		// Should find 1 problem with "go" tag (problem-1)
		if len(resp.Data) != 1 {
			t.Errorf("Step 11: expected 1 result with combined filters, got %d", len(resp.Data))
		}

		if len(resp.Data) > 0 {
			if resp.Data[0].Type != "problem" {
				t.Errorf("Step 11: expected type 'problem', got '%s'", resp.Data[0].Type)
			}
		}

		t.Logf("Step 11 passed: Combined filters work correctly")
	})

	// Step 12: No results
	t.Run("Step12_NoResults", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/search?q=nonexistent_query_xyz", nil)
		w := httptest.NewRecorder()

		handler.Search(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("Step 12 failed: expected status 200, got %d", w.Code)
		}

		var resp SearchResponse
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("Step 12 failed: failed to decode response: %v", err)
		}

		if len(resp.Data) != 0 {
			t.Errorf("Step 12: expected 0 results, got %d", len(resp.Data))
		}

		if resp.Meta.Total != 0 {
			t.Errorf("Step 12: expected total 0, got %d", resp.Meta.Total)
		}

		if resp.Meta.HasMore {
			t.Error("Step 12: expected has_more=false when no results")
		}

		t.Logf("Step 12 passed: No results handled correctly")
	})
}

// TestSearchIntegration_ResponseFormat verifies the response structure per SPEC.md Part 5.5.
func TestSearchIntegration_ResponseFormat(t *testing.T) {
	repo := NewMockSearchRepositoryWithPosts()
	handler := NewSearchHandler(repo)

	now := time.Now()
	solvedAt := now.Add(-1 * time.Hour)

	repo.AddPost(models.SearchResult{
		ID:           "post-123",
		Type:         "problem",
		Title:        "Test problem for response format",
		Snippet:      "This is a <mark>test</mark> snippet",
		Tags:         []string{"test", "go"},
		Status:       "solved",
		AuthorID:     "agent_test",
		AuthorType:   "agent",
		AuthorName:   "Test Agent",
		Score:        0.95,
		Votes:        42,
		AnswersCount: 5,
		CreatedAt:    now,
		SolvedAt:     &solvedAt,
	})

	t.Run("VerifyResponseStructure", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/search?q=test", nil)
		w := httptest.NewRecorder()

		handler.Search(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", w.Code)
		}

		// Parse as raw map to verify structure
		var rawResp map[string]interface{}
		if err := json.NewDecoder(w.Body).Decode(&rawResp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		// Verify top-level structure
		data, ok := rawResp["data"].([]interface{})
		if !ok {
			t.Fatal("expected 'data' array in response")
		}

		meta, ok := rawResp["meta"].(map[string]interface{})
		if !ok {
			t.Fatal("expected 'meta' object in response")
		}

		// Verify meta fields per SPEC.md
		if _, ok := meta["query"]; !ok {
			t.Error("expected 'query' in meta")
		}
		if _, ok := meta["total"]; !ok {
			t.Error("expected 'total' in meta")
		}
		if _, ok := meta["page"]; !ok {
			t.Error("expected 'page' in meta")
		}
		if _, ok := meta["per_page"]; !ok {
			t.Error("expected 'per_page' in meta")
		}
		if _, ok := meta["has_more"]; !ok {
			t.Error("expected 'has_more' in meta")
		}
		if _, ok := meta["took_ms"]; !ok {
			t.Error("expected 'took_ms' in meta")
		}

		// Verify data item structure
		if len(data) == 0 {
			t.Fatal("expected at least one result")
		}

		item := data[0].(map[string]interface{})

		// Verify required fields per SPEC.md Part 5.5
		requiredFields := []string{"id", "type", "title", "snippet", "tags", "status", "author", "score", "votes", "answers_count", "created_at"}
		for _, field := range requiredFields {
			if _, ok := item[field]; !ok {
				t.Errorf("expected '%s' in search result", field)
			}
		}

		// Verify author structure
		author, ok := item["author"].(map[string]interface{})
		if !ok {
			t.Fatal("expected 'author' object in result")
		}

		authorFields := []string{"id", "type", "display_name"}
		for _, field := range authorFields {
			if _, ok := author[field]; !ok {
				t.Errorf("expected '%s' in author object", field)
			}
		}

		// Verify author values
		if author["id"] != "agent_test" {
			t.Errorf("expected author id 'agent_test', got %v", author["id"])
		}
		if author["type"] != "agent" {
			t.Errorf("expected author type 'agent', got %v", author["type"])
		}
		if author["display_name"] != "Test Agent" {
			t.Errorf("expected author display_name 'Test Agent', got %v", author["display_name"])
		}

		// Verify solved_at is included for solved posts
		if _, ok := item["solved_at"]; !ok {
			t.Error("expected 'solved_at' for solved post")
		}

		t.Logf("Response format verification passed")
	})
}

// TestSearchIntegration_ErrorCases tests error handling scenarios.
func TestSearchIntegration_ErrorCases(t *testing.T) {
	repo := NewMockSearchRepositoryWithPosts()
	handler := NewSearchHandler(repo)

	t.Run("MissingQuery", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/search", nil)
		w := httptest.NewRecorder()

		handler.Search(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", w.Code)
		}

		var resp map[string]interface{}
		json.NewDecoder(w.Body).Decode(&resp)

		errObj := resp["error"].(map[string]interface{})
		if errObj["code"] != "VALIDATION_ERROR" {
			t.Errorf("expected error code 'VALIDATION_ERROR', got %v", errObj["code"])
		}
	})

	t.Run("EmptyQuery", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/search?q=", nil)
		w := httptest.NewRecorder()

		handler.Search(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", w.Code)
		}
	})

	t.Run("InvalidDateFormat", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/search?q=test&from_date=invalid", nil)
		w := httptest.NewRecorder()

		handler.Search(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", w.Code)
		}
	})

	t.Run("PerPageMaxEnforced", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/search?q=test&per_page=100", nil)
		w := httptest.NewRecorder()

		handler.Search(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", w.Code)
		}

		// Verify per_page was capped at 50
		if repo.lastOpts.PerPage != 50 {
			t.Errorf("expected per_page capped at 50, got %d", repo.lastOpts.PerPage)
		}
	})
}

// MockSearchRepositoryWithPosts is an enhanced mock that supports filtering and sorting.
type MockSearchRepositoryWithPosts struct {
	posts    []models.SearchResult
	lastOpts models.SearchOptions
}

func NewMockSearchRepositoryWithPosts() *MockSearchRepositoryWithPosts {
	return &MockSearchRepositoryWithPosts{
		posts: []models.SearchResult{},
	}
}

func (m *MockSearchRepositoryWithPosts) AddPost(post models.SearchResult) {
	m.posts = append(m.posts, post)
}

func (m *MockSearchRepositoryWithPosts) Search(ctx context.Context, query string, opts models.SearchOptions) ([]models.SearchResult, int, error) {
	m.lastOpts = opts

	// Filter posts based on query and options
	var filtered []models.SearchResult

	lowerQuery := strings.ToLower(query)
	for _, p := range m.posts {
		// Basic query matching (check if query is in title or snippet)
		if !strings.Contains(strings.ToLower(p.Title), lowerQuery) &&
			!strings.Contains(strings.ToLower(p.Snippet), lowerQuery) {
			continue
		}

		// Type filter
		if opts.Type != "" && p.Type != opts.Type {
			continue
		}

		// Status filter
		if opts.Status != "" && p.Status != opts.Status {
			continue
		}

		// Author filter
		if opts.Author != "" && p.AuthorID != opts.Author {
			continue
		}

		// Author type filter
		if opts.AuthorType != "" && p.AuthorType != opts.AuthorType {
			continue
		}

		// Tags filter
		if len(opts.Tags) > 0 && !hasAnyTag(p.Tags, opts.Tags) {
			continue
		}

		// Date filters
		if !opts.FromDate.IsZero() && p.CreatedAt.Before(opts.FromDate) {
			continue
		}
		if !opts.ToDate.IsZero() && p.CreatedAt.After(opts.ToDate) {
			continue
		}

		filtered = append(filtered, p)
	}

	// Sort results
	sortSearchResults(filtered, opts.Sort)

	total := len(filtered)

	// Apply pagination
	page := opts.Page
	if page < 1 {
		page = 1
	}
	perPage := opts.PerPage
	if perPage < 1 {
		perPage = 20
	}
	if perPage > 50 {
		perPage = 50
	}

	start := (page - 1) * perPage
	if start >= len(filtered) {
		return []models.SearchResult{}, total, nil
	}

	end := start + perPage
	if end > len(filtered) {
		end = len(filtered)
	}

	return filtered[start:end], total, nil
}

// hasAnyTag checks if postTags contains any of filterTags.
func hasAnyTag(postTags, filterTags []string) bool {
	for _, ft := range filterTags {
		for _, pt := range postTags {
			if pt == ft {
				return true
			}
		}
	}
	return false
}

// sortSearchResults sorts results based on sort option using stdlib sort.
func sortSearchResults(results []models.SearchResult, sortOpt string) {
	sort.Slice(results, func(i, j int) bool {
		switch sortOpt {
		case "votes":
			return results[i].Votes > results[j].Votes
		case "newest", "activity":
			return results[i].CreatedAt.After(results[j].CreatedAt)
		default: // relevance
			return results[i].Score > results[j].Score
		}
	})
}

// setupDBBackedSearchRepo creates a real search repository for integration testing.
// Returns nil if DATABASE_URL not set (test will be skipped).
func setupDBBackedSearchRepo(t *testing.T) (*db.SearchRepository, *db.Pool) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL not set, skipping DB-backed integration test")
	}

	ctx := context.Background()
	pool, err := db.NewPool(ctx, databaseURL)
	if err != nil {
		t.Fatalf("failed to connect to database: %v", err)
	}

	// Clean up test data before tests
	_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE id LIKE 'search-test-%'")

	t.Cleanup(func() {
		pool.Close()
	})

	return db.NewSearchRepository(pool), pool
}

func insertSearchTestPost(t *testing.T, pool *db.Pool, ctx context.Context,
	id, postType, title, desc string, tags []string, status string) {
	_, err := pool.Exec(ctx, `
		INSERT INTO posts (id, type, title, description, tags, status,
			posted_by_type, posted_by_id, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, 'human', 'test-user', NOW())
		ON CONFLICT (id) DO NOTHING
	`, id, postType, title, desc, tags, status)
	if err != nil {
		t.Fatalf("failed to insert test post: %v", err)
	}
}

// TestSearchIntegration_ExactTitleMatch tests that exact title matches rank correctly.
// This is a database-backed integration test verifying real PostgreSQL ts_rank behavior.
func TestSearchIntegration_ExactTitleMatch(t *testing.T) {
	repo, pool := setupDBBackedSearchRepo(t)
	ctx := context.Background()

	// Create 4 posts with similar titles
	titles := []string{
		"Race Conditions in Go",
		"How to Handle Race Conditions",
		"Understanding Race Conditions",
		"Thread Safety and Race Conditions",
	}

	for i, title := range titles {
		insertSearchTestPost(t, pool, ctx,
			fmt.Sprintf("search-test-exact-%d", i),
			"problem", title, "Test description", []string{"go"}, "open")
	}

	handler := NewSearchHandler(repo)

	// Test: Exact title search ranks that post #1
	req := httptest.NewRequest(http.MethodGet,
		"/v1/search?q=Race+Conditions+in+Go&sort=relevance", nil)
	w := httptest.NewRecorder()

	handler.Search(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)

	data := resp["data"].([]any)
	if len(data) == 0 {
		t.Fatal("expected at least 1 result for exact title match")
	}

	firstResult := data[0].(map[string]any)
	if firstResult["title"] != "Race Conditions in Go" {
		t.Errorf("exact title should rank first, got: %s", firstResult["title"])
	}
}

// TestSearchIntegration_MultiWordQuery tests that multi-word queries use OR logic.
// Searches find posts with ANY of the search terms, not requiring ALL terms.
func TestSearchIntegration_MultiWordQuery(t *testing.T) {
	repo, pool := setupDBBackedSearchRepo(t)
	ctx := context.Background()

	// Create posts with only ONE of the search terms
	insertSearchTestPost(t, pool, ctx, "search-test-mw-1", "problem",
		"Race detection in programs", "About race", []string{}, "open")

	insertSearchTestPost(t, pool, ctx, "search-test-mw-2", "problem",
		"Conditional logic patterns", "About conditions", []string{}, "open")

	handler := NewSearchHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/search?q=race+condition", nil)
	w := httptest.NewRecorder()

	handler.Search(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)

	meta := resp["meta"].(map[string]any)
	total := int(meta["total"].(float64))

	// With OR logic: should find both posts (one has "race", one has "condition")
	if total < 2 {
		t.Errorf("OR logic should find posts with ANY term, got %d results", total)
	}
}

// TestSearchIntegration_PartialWordMatch tests that prefix matching works.
// Searches like "rac" should find "race", "cond" should find "condition".
func TestSearchIntegration_PartialWordMatch(t *testing.T) {
	repo, pool := setupDBBackedSearchRepo(t)
	ctx := context.Background()

	insertSearchTestPost(t, pool, ctx, "search-test-partial-1", "problem",
		"Race conditions in Go", "Description", []string{}, "open")

	handler := NewSearchHandler(repo)

	// Test: "rac" should find "race"
	req := httptest.NewRequest(http.MethodGet, "/v1/search?q=rac", nil)
	w := httptest.NewRecorder()

	handler.Search(w, req)

	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)

	meta := resp["meta"].(map[string]any)
	total := int(meta["total"].(float64))

	if total == 0 {
		t.Error("prefix match 'rac' should find posts with 'race'")
	}
}
