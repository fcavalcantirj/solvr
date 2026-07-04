package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestAgentEndpoints_APIKeyRotateRequiresAuth verifies that POST /v1/agents/:id/api-key
// is registered in the router AND requires authentication.
//
// This is the regression guard against the route silently going dead: an UNregistered
// route returns 404, while a wired + auth-gated route returns 401 for an unauthenticated
// request. Asserting 401 therefore proves BOTH that the route is mounted and that agent
// API-key rotation is gated (only a human owner may rotate — see AgentsHandler.RegenerateAPIKey).
//
// Uses setupTestRouter(t) (real DB pool, skips without DATABASE_URL) because the /v1 route
// tree is only mounted when the pool is non-nil; no DB row is touched because the auth
// middleware rejects the request before any handler runs.
func TestAgentEndpoints_APIKeyRotateRequiresAuth(t *testing.T) {
	router := setupTestRouter(t)

	req := httptest.NewRequest(http.MethodPost, "/v1/agents/some-agent/api-key", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 (route wired + auth-gated), got %d — a 404 means the route is not registered", w.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	errObj, ok := resp["error"].(map[string]interface{})
	if !ok {
		t.Fatal("expected error object in response")
	}
	if errObj["code"] != "UNAUTHORIZED" {
		t.Errorf("expected UNAUTHORIZED code, got %v", errObj["code"])
	}
}
