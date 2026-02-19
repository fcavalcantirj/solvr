package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// TestSearch_MetaMethodHybrid tests that meta.method is 'hybrid' when embedding service is available.
func TestSearch_MetaMethodHybrid(t *testing.T) {
	repo := NewMockSearchRepository()
	repo.SetResults([]models.SearchResult{
		{
			ID:         "post-1",
			Type:       "problem",
			Title:      "Test Problem",
			Snippet:    "test snippet",
			Tags:       []string{"go"},
			Status:     "open",
			AuthorID:   "agent_test",
			AuthorType: "agent",
			AuthorName: "Test Agent",
			Score:      0.9,
			VoteScore:  5,
			CreatedAt:  time.Now(),
			Source:     "post",
		},
	}, 1)
	repo.SetMethod("hybrid")

	handler := NewSearchHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/search?q=test", nil)
	w := httptest.NewRecorder()

	handler.Search(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	meta, ok := resp["meta"].(map[string]interface{})
	if !ok {
		t.Fatal("expected meta object in response")
	}

	method, ok := meta["method"]
	if !ok {
		t.Fatal("expected 'method' field in meta")
	}

	if method != "hybrid" {
		t.Errorf("expected method 'hybrid', got '%v'", method)
	}
}

// TestSearch_MetaMethodFulltext tests that meta.method is 'fulltext' when no embedding service.
func TestSearch_MetaMethodFulltext(t *testing.T) {
	repo := NewMockSearchRepository()
	repo.SetResults([]models.SearchResult{}, 0)
	repo.SetMethod("fulltext")

	handler := NewSearchHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/search?q=test", nil)
	w := httptest.NewRecorder()

	handler.Search(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	meta, ok := resp["meta"].(map[string]interface{})
	if !ok {
		t.Fatal("expected meta object in response")
	}

	method, ok := meta["method"]
	if !ok {
		t.Fatal("expected 'method' field in meta")
	}

	if method != "fulltext" {
		t.Errorf("expected method 'fulltext', got '%v'", method)
	}
}

// TestSearch_MetaMethodDefaultsToFulltext tests that method defaults to 'fulltext' when not set.
func TestSearch_MetaMethodDefaultsToFulltext(t *testing.T) {
	repo := NewMockSearchRepository()
	repo.SetResults([]models.SearchResult{}, 0)
	// Don't call SetMethod — should default to "fulltext"

	handler := NewSearchHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/search?q=test", nil)
	w := httptest.NewRecorder()

	handler.Search(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	meta, ok := resp["meta"].(map[string]interface{})
	if !ok {
		t.Fatal("expected meta object in response")
	}

	method, ok := meta["method"]
	if !ok {
		t.Fatal("expected 'method' field in meta")
	}

	if method != "fulltext" {
		t.Errorf("expected default method 'fulltext', got '%v'", method)
	}
}

// TestSearch_MetaMethodWithResults tests method is returned alongside results and total.
func TestSearch_MetaMethodWithResults(t *testing.T) {
	repo := NewMockSearchRepository()
	now := time.Now()
	repo.SetResults([]models.SearchResult{
		{
			ID:         "post-1",
			Type:       "problem",
			Title:      "Race condition in Go",
			Snippet:    "<mark>race condition</mark> in concurrent code",
			Tags:       []string{"go", "concurrency"},
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
			ID:         "post-2",
			Type:       "question",
			Title:      "How to handle race conditions?",
			Snippet:    "Best practices for <mark>race conditions</mark>",
			Tags:       []string{"go"},
			Status:     "open",
			AuthorID:   "user-1",
			AuthorType: "human",
			AuthorName: "Dev",
			Score:      0.80,
			VoteScore:  10,
			CreatedAt:  now,
			Source:     "post",
		},
	}, 2)
	repo.SetMethod("hybrid")

	handler := NewSearchHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/search?q=race+condition", nil)
	w := httptest.NewRecorder()

	handler.Search(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Verify data is present
	data, ok := resp["data"].([]interface{})
	if !ok {
		t.Fatal("expected data array in response")
	}
	if len(data) != 2 {
		t.Errorf("expected 2 results, got %d", len(data))
	}

	// Verify meta has both total and method
	meta, ok := resp["meta"].(map[string]interface{})
	if !ok {
		t.Fatal("expected meta object in response")
	}

	if meta["total"].(float64) != 2 {
		t.Errorf("expected total 2, got %v", meta["total"])
	}

	if meta["method"] != "hybrid" {
		t.Errorf("expected method 'hybrid', got '%v'", meta["method"])
	}
}
