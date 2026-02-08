// Package api provides HTTP routing and handlers for the Solvr API.
package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/auth"
)

// TestUserEndpoints_NotificationsRequiresAuth tests that GET /v1/notifications requires auth.
func TestUserEndpoints_NotificationsRequiresAuth(t *testing.T) {
	r := setupTestRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/v1/notifications", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	// Should return 401 without auth
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
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

// TestUserEndpoints_NotificationsMarkReadRequiresAuth tests POST /v1/notifications/:id/read requires auth.
func TestUserEndpoints_NotificationsMarkReadRequiresAuth(t *testing.T) {
	r := setupTestRouter(t)

	req := httptest.NewRequest(http.MethodPost, "/v1/notifications/test-123/read", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	// Should return 401 without auth
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

// TestUserEndpoints_NotificationsMarkAllReadRequiresAuth tests POST /v1/notifications/read-all requires auth.
func TestUserEndpoints_NotificationsMarkAllReadRequiresAuth(t *testing.T) {
	r := setupTestRouter(t)

	req := httptest.NewRequest(http.MethodPost, "/v1/notifications/read-all", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	// Should return 401 without auth
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

// TestUserEndpoints_APIKeysListRequiresAuth tests GET /v1/users/me/api-keys requires auth.
func TestUserEndpoints_APIKeysListRequiresAuth(t *testing.T) {
	r := setupTestRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/v1/users/me/api-keys", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	// Should return 401 without auth
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
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

// TestUserEndpoints_APIKeysCreateRequiresAuth tests POST /v1/users/me/api-keys requires auth.
func TestUserEndpoints_APIKeysCreateRequiresAuth(t *testing.T) {
	r := setupTestRouter(t)

	body := strings.NewReader(`{"name": "test-key"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/users/me/api-keys", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	// Should return 401 without auth
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

// TestUserEndpoints_APIKeysRevokeRequiresAuth tests DELETE /v1/users/me/api-keys/:id requires auth.
func TestUserEndpoints_APIKeysRevokeRequiresAuth(t *testing.T) {
	r := setupTestRouter(t)

	req := httptest.NewRequest(http.MethodDelete, "/v1/users/me/api-keys/key-123", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	// Should return 401 without auth
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

// TestUserEndpoints_APIKeysRegenerateRequiresAuth tests POST /v1/users/me/api-keys/:id/regenerate requires auth.
func TestUserEndpoints_APIKeysRegenerateRequiresAuth(t *testing.T) {
	r := setupTestRouter(t)

	req := httptest.NewRequest(http.MethodPost, "/v1/users/me/api-keys/key-123/regenerate", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	// Should return 401 without auth
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

// TestUserEndpoints_MoltbookAuthAcceptsPost tests POST /v1/auth/moltbook is wired.
func TestUserEndpoints_MoltbookAuthAcceptsPost(t *testing.T) {
	r := setupTestRouter(t)

	// POST without body should return validation error, not 404
	req := httptest.NewRequest(http.MethodPost, "/v1/auth/moltbook", nil)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	// Should not be 404 - endpoint should exist
	if w.Code == http.StatusNotFound {
		t.Error("POST /v1/auth/moltbook returned 404 - endpoint not wired")
	}

	// Should return 400 for empty body
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for empty body, got %d", w.Code)
	}
}

// TestUserEndpoints_MoltbookAuthValidation tests POST /v1/auth/moltbook validates input.
func TestUserEndpoints_MoltbookAuthValidation(t *testing.T) {
	r := setupTestRouter(t)

	// POST with empty identity_token should return validation error
	body := strings.NewReader(`{"identity_token": ""}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/auth/moltbook", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

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
		t.Errorf("expected VALIDATION_ERROR code, got %v", errObj["code"])
	}
}

// TestUserEndpoints_NotificationsWithJWT tests GET /v1/notifications with JWT auth returns list.
func TestUserEndpoints_NotificationsWithJWT(t *testing.T) {
	r := setupTestRouter(t)

	// Create a valid JWT token for testing
	jwt := createUserEndpointsTestJWT("test-user-123")

	req := httptest.NewRequest(http.MethodGet, "/v1/notifications", nil)
	req.Header.Set("Authorization", "Bearer "+jwt)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	// Should return 200 with JWT auth
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d; body: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Should have data array
	if _, ok := resp["data"]; !ok {
		t.Error("expected 'data' field in response")
	}

	// Should have meta with pagination
	meta, ok := resp["meta"].(map[string]interface{})
	if !ok {
		t.Error("expected 'meta' field in response")
	}
	if _, ok := meta["total"]; !ok {
		t.Error("expected 'total' in meta")
	}
}

// TestUserEndpoints_APIKeysListWithJWT tests GET /v1/users/me/api-keys with JWT returns list.
func TestUserEndpoints_APIKeysListWithJWT(t *testing.T) {
	r := setupTestRouter(t)

	// Create a valid JWT token for testing
	jwt := createUserEndpointsTestJWT("test-user-123")

	req := httptest.NewRequest(http.MethodGet, "/v1/users/me/api-keys", nil)
	req.Header.Set("Authorization", "Bearer "+jwt)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	// Should return 200 with JWT auth
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d; body: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Should have data array
	if _, ok := resp["data"]; !ok {
		t.Error("expected 'data' field in response")
	}
}

// createUserEndpointsTestJWT creates a test JWT token.
// Uses the same secret as in router.go for testing.
func createUserEndpointsTestJWT(userID string) string {
	secret := "test-jwt-secret-32-chars-long!!"

	// Use the auth package's GenerateJWT function
	token, err := auth.GenerateJWT(secret, userID, "test@example.com", "user", time.Hour)
	if err != nil {
		panic("failed to generate test JWT: " + err.Error())
	}

	return token
}
