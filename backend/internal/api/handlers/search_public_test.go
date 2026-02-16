package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// =============================================================================
// Search API Public Access Tests
//
// These tests document that the search endpoint is PUBLIC and does NOT require
// authentication. Per SPEC.md Part 5.6: "All content should be publicly
// discoverable and readable".
//
// Authentication is only required for mutations (POST/PATCH/DELETE), not for
// browsing content or searching.
// =============================================================================

// TestSearch_NoAuthRequired verifies search works without any authentication.
// NOTE: This test currently tests the handler directly (which has no auth).
// The actual auth is enforced at the router level via middleware.
// This test documents the handler behavior - it doesn't reject requests.
func TestSearch_NoAuthRequired(t *testing.T) {
	repo := NewMockSearchRepository()
	repo.SetResults([]models.SearchResult{
		{
			ID:         "test-id",
			Type:       "problem",
			Title:      "Test Problem",
			Snippet:    "Test snippet",
			Tags:       []string{"go"},
			Status:     "open",
			Score:      0.95,
			Votes:      5,
			CreatedAt:  time.Now(),
			AuthorID:   "agent_test",
			AuthorType: "agent",
			AuthorName: "Test Agent",
		},
	}, 1)
	handler := NewSearchHandler(repo)

	// Request with NO Authorization header
	req := httptest.NewRequest(http.MethodGet, "/v1/search?q=test", nil)
	w := httptest.NewRecorder()

	handler.Search(w, req)

	// Handler returns 200 OK - search is public, no auth required
	if w.Code != http.StatusOK {
		t.Errorf("Search handler should return 200. Expected 200, got %d", w.Code)
	}

	// Verify response structure
	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Should have data array
	data, ok := resp["data"].([]interface{})
	if !ok {
		t.Fatal("Expected data array in response")
	}
	if len(data) != 1 {
		t.Errorf("Expected 1 result, got %d", len(data))
	}

	// Should have meta object
	_, ok = resp["meta"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected meta object in response")
	}
}

// TestSearch_WithAuthHeader verifies search works when valid auth is provided.
func TestSearch_WithAuthHeader(t *testing.T) {
	repo := NewMockSearchRepository()
	repo.SetResults([]models.SearchResult{
		{
			ID:    "test-id",
			Type:  "solution",
			Title: "Test Solution",
		},
	}, 1)
	handler := NewSearchHandler(repo)

	// Request WITH Authorization header
	req := httptest.NewRequest(http.MethodGet, "/v1/search?q=test", nil)
	req.Header.Set("Authorization", "Bearer solvr_test_agent_key")
	w := httptest.NewRecorder()

	handler.Search(w, req)

	// Should return 200 OK
	if w.Code != http.StatusOK {
		t.Errorf("Search should work with auth header. Expected 200, got %d", w.Code)
	}
}

// TestSearch_WithInvalidAuthHeader documents that the handler itself doesn't validate auth.
// Auth validation happens in the middleware at the router level.
func TestSearch_WithInvalidAuthHeader(t *testing.T) {
	repo := NewMockSearchRepository()
	repo.SetResults([]models.SearchResult{}, 0)
	handler := NewSearchHandler(repo)

	// Request with INVALID Authorization header
	req := httptest.NewRequest(http.MethodGet, "/v1/search?q=test", nil)
	req.Header.Set("Authorization", "Bearer totally_invalid_garbage_token")
	w := httptest.NewRecorder()

	handler.Search(w, req)

	// Handler returns 200 - search is public, ignores invalid auth
	if w.Code != http.StatusOK {
		t.Errorf("Search handler returns 200 (search is public). Expected 200, got %d", w.Code)
	}
}

// TestSearch_WithJWTAuthHeader verifies search works with JWT auth.
func TestSearch_WithJWTAuthHeader(t *testing.T) {
	repo := NewMockSearchRepository()
	repo.SetResults([]models.SearchResult{}, 0)
	handler := NewSearchHandler(repo)

	// Request with JWT-style Authorization header
	req := httptest.NewRequest(http.MethodGet, "/v1/search?q=test", nil)
	req.Header.Set("Authorization", "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.fake")
	w := httptest.NewRecorder()

	handler.Search(w, req)

	// Should return 200 OK
	if w.Code != http.StatusOK {
		t.Errorf("Search should work with JWT header. Expected 200, got %d", w.Code)
	}
}

// TestSearch_PublicReadContract documents that search is public and works with or without auth.
// NOTE: Handler tests show handler behavior. Router integration tests verify no auth required.
func TestSearch_PublicReadContract(t *testing.T) {
	repo := NewMockSearchRepository()
	repo.SetResults([]models.SearchResult{
		{
			ID:         "problem-1",
			Type:       "problem",
			Title:      "Memory Leak Issue",
			Snippet:    "Found a <mark>memory</mark> leak in the application",
			Tags:       []string{"go", "memory", "debugging"},
			Status:     "open",
			Score:      0.85,
			Votes:      10,
			CreatedAt:  time.Now(),
			AuthorID:   "agent_debugger",
			AuthorType: "agent",
			AuthorName: "Debugger Agent",
		},
	}, 1)
	handler := NewSearchHandler(repo)

	testCases := []struct {
		name        string
		authHeader  string
		shouldWork  bool
		description string
	}{
		{
			name:        "no auth",
			authHeader:  "",
			shouldWork:  true,
			description: "Search is public - works without any auth",
		},
		{
			name:        "agent api key",
			authHeader:  "Bearer solvr_abc123",
			shouldWork:  true,
			description: "Search is public - works with agent key (optional)",
		},
		{
			name:        "user api key",
			authHeader:  "Bearer solvr_sk_xyz789",
			shouldWork:  true,
			description: "Search is public - works with user key (optional)",
		},
		{
			name:        "jwt token",
			authHeader:  "Bearer eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJ1c2VyIn0.sig",
			shouldWork:  true,
			description: "Search is public - works with JWT (optional)",
		},
		{
			name:        "invalid token",
			authHeader:  "Bearer invalid",
			shouldWork:  true,
			description: "Search is public - ignores invalid auth",
		},
		{
			name:        "malformed header",
			authHeader:  "NotBearer something",
			shouldWork:  true,
			description: "Search is public - ignores malformed auth",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/v1/search?q=memory+leak", nil)
			if tc.authHeader != "" {
				req.Header.Set("Authorization", tc.authHeader)
			}
			w := httptest.NewRecorder()

			handler.Search(w, req)

			if tc.shouldWork && w.Code != http.StatusOK {
				t.Errorf("%s: Expected 200, got %d", tc.description, w.Code)
			}
		})
	}
}
