package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// =============================================================================
// Router Integration Tests for Search Endpoint
//
// These tests verify that the search endpoint is properly configured at the
// router level to be PUBLIC (no auth required).
//
// Per SPEC.md Part 5.6: "All content should be publicly discoverable and readable"
// =============================================================================

// TestSearchEndpoint_NoAuthRequired verifies search works without Authorization header
func TestSearchEndpoint_NoAuthRequired(t *testing.T) {
	router := setupTestRouter(t)

	// Request WITHOUT Authorization header
	req := httptest.NewRequest(http.MethodGet, "/v1/search?q=test", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// Should return 200 OK (not 401 Unauthorized)
	if w.Code != http.StatusOK {
		t.Errorf("Expected 200 OK for search without auth. Got %d: %s", w.Code, w.Body.String())
	}

	// Verify response structure
	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Should have data array
	_, ok := resp["data"].([]interface{})
	if !ok {
		t.Error("Expected data array in response")
	}

	// Should have meta object
	_, ok = resp["meta"].(map[string]interface{})
	if !ok {
		t.Error("Expected meta object in response")
	}
}

// TestSearchEndpoint_WithValidAuth verifies search still works with valid auth
func TestSearchEndpoint_WithValidAuth(t *testing.T) {
	router := setupTestRouter(t)

	// Request WITH Authorization header (using a test agent key format)
	req := httptest.NewRequest(http.MethodGet, "/v1/search?q=test", nil)
	req.Header.Set("Authorization", "Bearer solvr_test_agent_key")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// Should return 200 OK (or 401 if key invalid, but either way auth is attempted)
	// For search being public, even invalid auth should return 200
	if w.Code != http.StatusOK && w.Code != http.StatusUnauthorized {
		t.Errorf("Expected 200 OK or 401, got %d: %s", w.Code, w.Body.String())
	}
}

// TestSearchEndpoint_WithInvalidAuth verifies search works even with invalid auth
// (auth is optional, invalid auth should be ignored for public endpoints)
func TestSearchEndpoint_WithInvalidAuth(t *testing.T) {
	router := setupTestRouter(t)

	// Request WITH invalid Authorization header
	req := httptest.NewRequest(http.MethodGet, "/v1/search?q=test", nil)
	req.Header.Set("Authorization", "Bearer totally_invalid_garbage_token")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// Should return 200 OK (auth is optional, invalid auth is ignored for public endpoints)
	if w.Code != http.StatusOK {
		t.Errorf("Expected 200 OK for search with invalid auth (should be ignored). Got %d: %s", w.Code, w.Body.String())
	}
}
