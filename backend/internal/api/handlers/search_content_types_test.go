package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// TestSearch_ContentTypesParam tests that content_types query param is parsed correctly.
func TestSearch_ContentTypesParam(t *testing.T) {
	repo := NewMockSearchRepository()
	repo.SetResults([]models.SearchResult{}, 0)

	handler := NewSearchHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/search?q=golang&content_types=posts,answers", nil)
	w := httptest.NewRecorder()

	handler.Search(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	// Verify the repository received the content_types filter
	if len(repo.searchOpts.ContentTypes) != 2 {
		t.Fatalf("expected 2 content types, got %d: %v", len(repo.searchOpts.ContentTypes), repo.searchOpts.ContentTypes)
	}

	if repo.searchOpts.ContentTypes[0] != "posts" {
		t.Errorf("expected first content type 'posts', got '%s'", repo.searchOpts.ContentTypes[0])
	}

	if repo.searchOpts.ContentTypes[1] != "answers" {
		t.Errorf("expected second content type 'answers', got '%s'", repo.searchOpts.ContentTypes[1])
	}
}

// TestSearch_ContentTypesDefault tests that default content_types includes all types.
func TestSearch_ContentTypesDefault(t *testing.T) {
	repo := NewMockSearchRepository()
	repo.SetResults([]models.SearchResult{}, 0)

	handler := NewSearchHandler(repo)

	// No content_types param — should default to all
	req := httptest.NewRequest(http.MethodGet, "/v1/search?q=test", nil)
	w := httptest.NewRecorder()

	handler.Search(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	// Default should be empty (meaning "all" — handler/repo treats empty as all)
	if len(repo.searchOpts.ContentTypes) != 0 {
		t.Errorf("expected empty content_types (default=all), got %v", repo.searchOpts.ContentTypes)
	}
}

// TestSearch_SourceFieldInResponse tests that search results include the source field.
func TestSearch_SourceFieldInResponse(t *testing.T) {
	repo := NewMockSearchRepository()
	now := time.Now()
	repo.SetResults([]models.SearchResult{
		{
			ID:         "post-123",
			Type:       "problem",
			Title:      "Race condition in PostgreSQL",
			Snippet:    "encountering a <mark>race condition</mark>",
			Tags:       []string{"postgresql"},
			Status:     "solved",
			AuthorID:   "agent_claude",
			AuthorType: "agent",
			AuthorName: "Claude",
			Score:      0.95,
			VoteScore:  42,
			CreatedAt:  now,
			Source:     "post",
		},
		{
			ID:         "answer-456",
			Type:       "answer",
			Title:      "How to fix race conditions",
			Snippet:    "Use transactions to fix <mark>race conditions</mark>",
			Tags:       nil,
			Status:     "",
			AuthorID:   "user-789",
			AuthorType: "human",
			AuthorName: "John",
			Score:      0.88,
			VoteScore:  15,
			CreatedAt:  now,
			Source:     "answer",
		},
		{
			ID:         "approach-789",
			Type:       "approach",
			Title:      "Transaction-based approach",
			Snippet:    "Wrapping all queries in <mark>transactions</mark>",
			Tags:       nil,
			Status:     "succeeded",
			AuthorID:   "agent_helper",
			AuthorType: "agent",
			AuthorName: "Helper",
			Score:      0.80,
			VoteScore:  8,
			CreatedAt:  now,
			Source:     "approach",
		},
	}, 3)

	handler := NewSearchHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/search?q=race+condition&content_types=posts,answers,approaches", nil)
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

	if len(data) != 3 {
		t.Fatalf("expected 3 results, got %d", len(data))
	}

	// Verify source field is present on each result
	expectedSources := []string{"post", "answer", "approach"}
	for i, item := range data {
		result := item.(map[string]interface{})
		source, ok := result["source"]
		if !ok {
			t.Errorf("result %d: expected 'source' field in response", i)
			continue
		}
		if source != expectedSources[i] {
			t.Errorf("result %d: expected source '%s', got '%v'", i, expectedSources[i], source)
		}
	}
}

// TestSearch_ContentTypesPostsOnly tests that content_types=posts returns only posts.
func TestSearch_ContentTypesPostsOnly(t *testing.T) {
	repo := NewMockSearchRepository()
	repo.SetResults([]models.SearchResult{}, 0)

	handler := NewSearchHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/search?q=test&content_types=posts", nil)
	w := httptest.NewRecorder()

	handler.Search(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	if len(repo.searchOpts.ContentTypes) != 1 {
		t.Fatalf("expected 1 content type, got %d", len(repo.searchOpts.ContentTypes))
	}

	if repo.searchOpts.ContentTypes[0] != "posts" {
		t.Errorf("expected content type 'posts', got '%s'", repo.searchOpts.ContentTypes[0])
	}
}

// TestSearch_ContentTypesAllThree tests that content_types=posts,answers,approaches is parsed.
func TestSearch_ContentTypesAllThree(t *testing.T) {
	repo := NewMockSearchRepository()
	repo.SetResults([]models.SearchResult{}, 0)

	handler := NewSearchHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/search?q=test&content_types=posts,answers,approaches", nil)
	w := httptest.NewRecorder()

	handler.Search(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	if len(repo.searchOpts.ContentTypes) != 3 {
		t.Fatalf("expected 3 content types, got %d", len(repo.searchOpts.ContentTypes))
	}

	expected := map[string]bool{"posts": true, "answers": true, "approaches": true}
	for _, ct := range repo.searchOpts.ContentTypes {
		if !expected[ct] {
			t.Errorf("unexpected content type '%s'", ct)
		}
	}
}
