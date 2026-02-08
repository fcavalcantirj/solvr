package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestNewRouter verifies that NewRouter returns a configured chi.Mux
func TestNewRouter(t *testing.T) {
	router := NewRouter(nil) // nil pool for basic tests
	if router == nil {
		t.Fatal("NewRouter returned nil")
	}
}

// TestRouterCORSHeaders verifies CORS middleware is configured
func TestRouterCORSHeaders(t *testing.T) {
	router := NewRouter(nil)

	// Make a preflight request
	req := httptest.NewRequest(http.MethodOptions, "/health", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	req.Header.Set("Access-Control-Request-Method", "GET")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// CORS should allow localhost:3000
	origin := w.Header().Get("Access-Control-Allow-Origin")
	if origin != "http://localhost:3000" {
		t.Errorf("expected Access-Control-Allow-Origin to be 'http://localhost:3000', got '%s'", origin)
	}
}

// TestRouterJSONContentType verifies JSON content-type middleware
func TestRouterJSONContentType(t *testing.T) {
	router := NewRouter(nil)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("expected Content-Type to be 'application/json', got '%s'", contentType)
	}
}

// TestRouterSecurityHeaders verifies security headers are set
func TestRouterSecurityHeaders(t *testing.T) {
	router := NewRouter(nil)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	tests := []struct {
		header   string
		expected string
	}{
		{"X-Content-Type-Options", "nosniff"},
		{"X-Frame-Options", "DENY"},
		{"X-XSS-Protection", "1; mode=block"},
	}

	for _, tt := range tests {
		got := w.Header().Get(tt.header)
		if got != tt.expected {
			t.Errorf("expected %s to be '%s', got '%s'", tt.header, tt.expected, got)
		}
	}
}

// TestRouterRequestID verifies request ID middleware adds X-Request-ID header
func TestRouterRequestID(t *testing.T) {
	router := NewRouter(nil)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	requestID := w.Header().Get("X-Request-ID")
	if requestID == "" {
		t.Error("expected X-Request-ID header to be set")
	}

	// Verify it looks like a UUID (36 characters with hyphens)
	if len(requestID) != 36 {
		t.Errorf("expected X-Request-ID to be UUID format (36 chars), got %d chars", len(requestID))
	}
}

// TestHealthEndpoint verifies GET /health returns correct response
func TestHealthEndpoint(t *testing.T) {
	router := NewRouter(nil)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Check required fields
	if response["status"] != "ok" {
		t.Errorf("expected status 'ok', got '%v'", response["status"])
	}
	if response["version"] == nil {
		t.Error("expected version field in response")
	}
	if response["timestamp"] == nil {
		t.Error("expected timestamp field in response")
	}
}

// TestHealthLiveEndpoint verifies GET /health/live returns correct response
func TestHealthLiveEndpoint(t *testing.T) {
	router := NewRouter(nil)

	req := httptest.NewRequest(http.MethodGet, "/health/live", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response["status"] != "alive" {
		t.Errorf("expected status 'alive', got '%v'", response["status"])
	}
}

// TestHealthReadyEndpointNoPool verifies GET /health/ready with no pool returns 503
func TestHealthReadyEndpointNoPool(t *testing.T) {
	router := NewRouter(nil) // No database pool

	req := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Should return 503 when no database pool is configured
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status 503 (no pool), got %d", w.Code)
	}
}

// TestNotFoundReturnsJSON verifies 404 responses are JSON formatted
func TestNotFoundReturnsJSON(t *testing.T) {
	router := NewRouter(nil)

	req := httptest.NewRequest(http.MethodGet, "/nonexistent-path", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("expected Content-Type 'application/json', got '%s'", contentType)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}

	// Should have error object
	if response["error"] == nil {
		t.Error("expected error object in response")
	}
}

// TestMethodNotAllowedReturnsJSON verifies 405 responses are JSON formatted
func TestMethodNotAllowedReturnsJSON(t *testing.T) {
	router := NewRouter(nil)

	// POST to /health should return 405
	req := httptest.NewRequest(http.MethodPost, "/health", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status 405, got %d", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("expected Content-Type 'application/json', got '%s'", contentType)
	}
}

// TestCORSProductionOrigin verifies CORS allows production domain solvr.dev
func TestCORSProductionOrigin(t *testing.T) {
	router := NewRouter(nil)

	// Make a preflight request from production origin
	req := httptest.NewRequest(http.MethodOptions, "/health", nil)
	req.Header.Set("Origin", "https://solvr.dev")
	req.Header.Set("Access-Control-Request-Method", "GET")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// CORS should allow solvr.dev
	origin := w.Header().Get("Access-Control-Allow-Origin")
	if origin != "https://solvr.dev" {
		t.Errorf("expected Access-Control-Allow-Origin to be 'https://solvr.dev', got '%s'", origin)
	}
}

// TestCORSProductionWWWOrigin verifies CORS allows www.solvr.dev
func TestCORSProductionWWWOrigin(t *testing.T) {
	router := NewRouter(nil)

	// Make a preflight request from production www origin
	req := httptest.NewRequest(http.MethodOptions, "/health", nil)
	req.Header.Set("Origin", "https://www.solvr.dev")
	req.Header.Set("Access-Control-Request-Method", "GET")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// CORS should allow www.solvr.dev
	origin := w.Header().Get("Access-Control-Allow-Origin")
	if origin != "https://www.solvr.dev" {
		t.Errorf("expected Access-Control-Allow-Origin to be 'https://www.solvr.dev', got '%s'", origin)
	}
}

// TestCORSLocalhostDevelopment verifies CORS allows localhost:3000 in development
func TestCORSLocalhostDevelopment(t *testing.T) {
	router := NewRouter(nil)

	// Make a preflight request from localhost
	req := httptest.NewRequest(http.MethodOptions, "/health", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	req.Header.Set("Access-Control-Request-Method", "POST")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// CORS should allow localhost:3000
	origin := w.Header().Get("Access-Control-Allow-Origin")
	if origin != "http://localhost:3000" {
		t.Errorf("expected Access-Control-Allow-Origin to be 'http://localhost:3000', got '%s'", origin)
	}
}

// TestCORSCredentialsAllowed verifies Access-Control-Allow-Credentials is set to true
func TestCORSCredentialsAllowed(t *testing.T) {
	router := NewRouter(nil)

	// Make a preflight request
	req := httptest.NewRequest(http.MethodOptions, "/health", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	req.Header.Set("Access-Control-Request-Method", "GET")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Credentials should be allowed
	credentials := w.Header().Get("Access-Control-Allow-Credentials")
	if credentials != "true" {
		t.Errorf("expected Access-Control-Allow-Credentials to be 'true', got '%s'", credentials)
	}
}

// TestCORSExposedRateLimitHeaders verifies X-RateLimit-* headers are exposed
func TestCORSExposedRateLimitHeaders(t *testing.T) {
	router := NewRouter(nil)

	// Make an actual GET request (not preflight) - exposed headers are only sent in actual responses
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.Header.Set("Origin", "http://localhost:3000")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Check exposed headers include rate limit headers
	exposedHeaders := w.Header().Get("Access-Control-Expose-Headers")

	expectedHeaders := []string{
		"X-RateLimit-Limit",
		"X-RateLimit-Remaining",
		"X-RateLimit-Reset",
		"X-Request-ID",
	}

	// Headers are case-insensitive per HTTP spec, so use case-insensitive comparison
	exposedHeadersLower := strings.ToLower(exposedHeaders)
	for _, header := range expectedHeaders {
		if !strings.Contains(exposedHeadersLower, strings.ToLower(header)) {
			t.Errorf("expected Access-Control-Expose-Headers to include '%s', got '%s'", header, exposedHeaders)
		}
	}
}

// TestCORSAllowedMethods verifies allowed HTTP methods
func TestCORSAllowedMethods(t *testing.T) {
	router := NewRouter(nil)

	allowedMethods := []string{"GET", "POST", "PATCH", "DELETE"}

	for _, method := range allowedMethods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodOptions, "/health", nil)
			req.Header.Set("Origin", "http://localhost:3000")
			req.Header.Set("Access-Control-Request-Method", method)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			allowMethods := w.Header().Get("Access-Control-Allow-Methods")
			if !contains(allowMethods, method) {
				t.Errorf("expected Access-Control-Allow-Methods to include '%s', got '%s'", method, allowMethods)
			}
		})
	}
}

// TestCORSDisallowedOrigin verifies CORS rejects unknown origins
func TestCORSDisallowedOrigin(t *testing.T) {
	router := NewRouter(nil)

	// Make a preflight request from unknown origin
	req := httptest.NewRequest(http.MethodOptions, "/health", nil)
	req.Header.Set("Origin", "https://malicious-site.com")
	req.Header.Set("Access-Control-Request-Method", "GET")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// CORS should NOT allow this origin
	origin := w.Header().Get("Access-Control-Allow-Origin")
	if origin == "https://malicious-site.com" {
		t.Error("expected CORS to reject unknown origin 'https://malicious-site.com'")
	}
}

// contains checks if a comma-separated string contains a value
func contains(s, substr string) bool {
	if s == "" {
		return false
	}
	// Simple substring check for comma-separated values
	return s == substr ||
		len(s) >= len(substr) && (s[:len(substr)] == substr && (len(s) == len(substr) || s[len(substr)] == ',') ||
		len(s) > len(substr)+1 && (s[len(s)-len(substr):] == substr && s[len(s)-len(substr)-1] == ',' ||
		len(s) > len(substr)+2 && (s[1:len(substr)+1] == substr || findInCSV(s, substr))))
}

// findInCSV checks if substr exists in a comma-separated string
func findInCSV(csv, target string) bool {
	start := 0
	for i := 0; i <= len(csv); i++ {
		if i == len(csv) || csv[i] == ',' {
			if i > start {
				// Trim spaces
				item := csv[start:i]
				for len(item) > 0 && item[0] == ' ' {
					item = item[1:]
				}
				for len(item) > 0 && item[len(item)-1] == ' ' {
					item = item[:len(item)-1]
				}
				if item == target {
					return true
				}
			}
			start = i + 1
		}
	}
	return false
}

// TestAgentsRegisterEndpoint verifies POST /v1/agents/register endpoint is wired.
// Per API-CRITICAL requirement: Wire POST /v1/agents/register endpoint.
func TestAgentsRegisterEndpoint(t *testing.T) {
	router := setupTestRouter(t)

	// Create a valid registration request
	reqBody := `{"name":"test_agent","description":"A test agent"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/agents/register", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Should return 201 Created with API key on success
	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Verify response contains api_key
	if response["api_key"] == nil {
		t.Error("expected api_key in response")
	}

	// Verify response contains agent
	if response["agent"] == nil {
		t.Error("expected agent in response")
	}

	// Verify success flag
	if response["success"] != true {
		t.Error("expected success=true in response")
	}
}

// TestClaimEndpointExists verifies POST /v1/agents/me/claim endpoint exists.
// Per API-CRITICAL requirement: Wire GET/POST /v1/agents/{id}/claim endpoints.
// The implementation uses /v1/agents/me/claim for agent self-claim.
func TestClaimEndpointExists(t *testing.T) {
	router := setupTestRouter(t)

	// POST /v1/agents/me/claim should return 401 without auth (not 404)
	req := httptest.NewRequest(http.MethodPost, "/v1/agents/me/claim", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Should return 401 Unauthorized (endpoint exists but requires auth), not 404
	if w.Code == http.StatusNotFound {
		t.Errorf("POST /v1/agents/me/claim returned 404 - endpoint not wired")
	}

	// Should be 401 Unauthorized because no API key auth
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d: %s", w.Code, w.Body.String())
	}
}

// TestGetClaimInfoEndpointExists verifies GET /v1/claim/{token} endpoint exists.
// Per API-CRITICAL requirement: Wire /v1/claim/{token} endpoints.
func TestGetClaimInfoEndpointExists(t *testing.T) {
	router := setupTestRouter(t)

	// GET /v1/claim/{token} should not return 404
	req := httptest.NewRequest(http.MethodGet, "/v1/claim/test_token_123", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Should NOT return 404 - endpoint should be wired
	if w.Code == http.StatusNotFound {
		t.Errorf("GET /v1/claim/{token} returned 404 - endpoint not wired")
	}

	// GetClaimInfo returns 200 with token_valid: false for invalid tokens
	// or 500 if claim token repo not configured
	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200 or 500, got %d: %s", w.Code, w.Body.String())
	}
}

// TestConfirmClaimEndpointExists verifies POST /v1/claim/{token} endpoint exists.
// Per API-CRITICAL requirement: Wire /v1/claim/{token} endpoints.
func TestConfirmClaimEndpointExists(t *testing.T) {
	router := setupTestRouter(t)

	// POST /v1/claim/{token} should return 401 without auth (not 404)
	req := httptest.NewRequest(http.MethodPost, "/v1/claim/test_token_123", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Should NOT return 404 - endpoint should be wired
	if w.Code == http.StatusNotFound {
		t.Errorf("POST /v1/claim/{token} returned 404 - endpoint not wired")
	}

	// ConfirmClaim requires JWT auth, so should return 401 or 500 (if repo not configured)
	if w.Code != http.StatusUnauthorized && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 401 or 500, got %d: %s", w.Code, w.Body.String())
	}
}

// TestGitHubOAuthRedirectEndpoint verifies GET /v1/auth/github endpoint is wired.
// Per API-CRITICAL requirement: Wire OAuth endpoints /v1/auth/github/*.
func TestGitHubOAuthRedirectEndpoint(t *testing.T) {
	router := setupTestRouter(t)

	// GET /v1/auth/github should redirect to GitHub OAuth
	req := httptest.NewRequest(http.MethodGet, "/v1/auth/github", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Should NOT return 404 - endpoint should be wired
	if w.Code == http.StatusNotFound {
		t.Errorf("GET /v1/auth/github returned 404 - endpoint not wired")
	}

	// Should return 302 Found (redirect)
	if w.Code != http.StatusFound {
		t.Errorf("expected status 302 (redirect), got %d: %s", w.Code, w.Body.String())
	}

	// Check redirect location contains GitHub OAuth URL
	location := w.Header().Get("Location")
	if location == "" {
		t.Error("expected Location header for redirect")
	}
	if !strings.Contains(location, "github.com/login/oauth/authorize") {
		t.Errorf("expected redirect to GitHub OAuth, got Location: %s", location)
	}
}

// TestGitHubOAuthCallbackEndpoint verifies GET /v1/auth/github/callback endpoint is wired.
// Per API-CRITICAL requirement: Wire OAuth endpoints /v1/auth/github/*.
func TestGitHubOAuthCallbackEndpoint(t *testing.T) {
	router := setupTestRouter(t)

	// GET /v1/auth/github/callback without code should return 400 (not 404)
	req := httptest.NewRequest(http.MethodGet, "/v1/auth/github/callback", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Should NOT return 404 - endpoint should be wired
	if w.Code == http.StatusNotFound {
		t.Errorf("GET /v1/auth/github/callback returned 404 - endpoint not wired")
	}

	// Should return 400 Bad Request (missing authorization code)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 (missing code), got %d: %s", w.Code, w.Body.String())
	}

	// Verify error response is JSON
	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Should have error object
	if response["error"] == nil {
		t.Error("expected error object in response")
	}
}

// TestGoogleOAuthRedirectEndpoint verifies GET /v1/auth/google endpoint is wired.
// Per API-CRITICAL requirement: Wire OAuth endpoints /v1/auth/google/*.
func TestGoogleOAuthRedirectEndpoint(t *testing.T) {
	router := setupTestRouter(t)

	// GET /v1/auth/google should redirect to Google OAuth
	req := httptest.NewRequest(http.MethodGet, "/v1/auth/google", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Should NOT return 404 - endpoint should be wired
	if w.Code == http.StatusNotFound {
		t.Errorf("GET /v1/auth/google returned 404 - endpoint not wired")
	}

	// Should return 302 Found (redirect)
	if w.Code != http.StatusFound {
		t.Errorf("expected status 302 (redirect), got %d: %s", w.Code, w.Body.String())
	}

	// Check redirect location contains Google OAuth URL
	location := w.Header().Get("Location")
	if location == "" {
		t.Error("expected Location header for redirect")
	}
	if !strings.Contains(location, "accounts.google.com/o/oauth2") {
		t.Errorf("expected redirect to Google OAuth, got Location: %s", location)
	}
}

// getKeys returns the keys of a map
func getKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// TestGoogleOAuthCallbackEndpoint verifies GET /v1/auth/google/callback endpoint is wired.
// Per API-CRITICAL requirement: Wire OAuth endpoints /v1/auth/google/*.
func TestGoogleOAuthCallbackEndpoint(t *testing.T) {
	router := setupTestRouter(t)

	// GET /v1/auth/google/callback without code should return 400 (not 404)
	req := httptest.NewRequest(http.MethodGet, "/v1/auth/google/callback", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Should NOT return 404 - endpoint should be wired
	if w.Code == http.StatusNotFound {
		t.Errorf("GET /v1/auth/google/callback returned 404 - endpoint not wired")
	}

	// Should return 400 Bad Request (missing authorization code)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 (missing code), got %d: %s", w.Code, w.Body.String())
	}

	// Verify error response is JSON
	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Should have error object
	if response["error"] == nil {
		t.Error("expected error object in response")
	}
}

// TestGetPostsEndpoint verifies GET /v1/posts endpoint is wired.
// Per API-CRITICAL requirement: Wire GET/POST /v1/posts endpoints.
func TestGetPostsEndpoint(t *testing.T) {
	router := setupTestRouter(t)

	// GET /v1/posts should return 200 with posts list
	req := httptest.NewRequest(http.MethodGet, "/v1/posts", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Should NOT return 404 - endpoint should be wired
	if w.Code == http.StatusNotFound {
		t.Errorf("GET /v1/posts returned 404 - endpoint not wired")
	}

	// Should return 200 OK
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify response is JSON with data and meta
	bodyBytes := w.Body.Bytes()
	t.Logf("Response body: %s", string(bodyBytes))

	var response map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Should have data key (can be null/empty array when no posts)
	if _, hasData := response["data"]; !hasData {
		t.Errorf("expected data key in response, got keys: %v", getKeys(response))
	}

	// Should have meta object
	if response["meta"] == nil {
		t.Error("expected meta in response")
	}

	// Meta should have pagination fields
	meta, ok := response["meta"].(map[string]interface{})
	if !ok {
		t.Error("meta should be an object")
	} else {
		if _, hasTotal := meta["total"]; !hasTotal {
			t.Error("meta should have total field")
		}
		if _, hasPage := meta["page"]; !hasPage {
			t.Error("meta should have page field")
		}
	}
}

// TestGetSinglePostEndpoint verifies GET /v1/posts/:id endpoint is wired.
// Per API-CRITICAL requirement: Wire GET/POST /v1/posts endpoints.
func TestGetSinglePostEndpoint(t *testing.T) {
	router := setupTestRouter(t)

	// GET /v1/posts/:id with non-existent ID should return 404 (not found post, not route)
	req := httptest.NewRequest(http.MethodGet, "/v1/posts/00000000-0000-0000-0000-000000000001", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Should return 404 NOT_FOUND (post not found), but with our standard error format
	// This confirms the route exists but the post doesn't
	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404 (post not found), got %d: %s", w.Code, w.Body.String())
	}

	// Verify error response is JSON with proper format
	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Should have error object with NOT_FOUND code
	errObj, ok := response["error"].(map[string]interface{})
	if !ok {
		t.Error("expected error object in response")
	} else {
		if errObj["code"] != "NOT_FOUND" {
			t.Errorf("expected error code NOT_FOUND, got %v", errObj["code"])
		}
	}
}

// TestCreatePostEndpointRequiresAuth verifies POST /v1/posts requires authentication.
// Per API-CRITICAL requirement: Wire GET/POST /v1/posts endpoints.
func TestCreatePostEndpointRequiresAuth(t *testing.T) {
	router := setupTestRouter(t)

	// POST /v1/posts without auth should return 401
	reqBody := `{"type":"question","title":"Test question title","description":"This is a test question description that is long enough"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/posts", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Should NOT return 404 - endpoint should be wired
	if w.Code == http.StatusNotFound {
		t.Errorf("POST /v1/posts returned 404 - endpoint not wired")
	}

	// Should return 401 Unauthorized
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401 (unauthorized), got %d: %s", w.Code, w.Body.String())
	}

	// Verify error response is JSON
	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Should have error object
	if response["error"] == nil {
		t.Error("expected error object in response")
	}
}

// TestGetPostsEndpointNoPlaceholder verifies GET /v1/posts returns real data, not placeholder.
// Per FIX-001: routes.go placeholders override real router.go handlers
func TestGetPostsEndpointNoPlaceholder(t *testing.T) {
	router := setupTestRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/v1/posts", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Meta should NOT contain "coming soon" placeholder message
	meta, ok := response["meta"].(map[string]interface{})
	if ok {
		if msg, hasMsg := meta["message"]; hasMsg {
			if msgStr, isStr := msg.(string); isStr {
				if strings.Contains(strings.ToLower(msgStr), "coming soon") {
					t.Error("GET /v1/posts returned placeholder 'coming soon' message - routes.go override not removed")
				}
			}
		}
	}

	// Must have proper pagination fields in meta
	if meta != nil {
		if _, hasTotal := meta["total"]; !hasTotal {
			t.Error("meta should have total field for real data response")
		}
		if _, hasPage := meta["page"]; !hasPage {
			t.Error("meta should have page field for real data response")
		}
	}
}

// TestGetAgentProfileEndpoint verifies GET /v1/agents/{id} returns real agent data, not placeholder.
// Per FIX-001 and FIX-006: routes.go placeholders override real router.go handlers
func TestGetAgentProfileEndpoint(t *testing.T) {
	router := setupTestRouter(t)

	// Use in-memory repo's first agent - register one first
	reqBody := `{"name":"test_profile_agent","description":"Test agent for profile endpoint"}`
	regReq := httptest.NewRequest(http.MethodPost, "/v1/agents/register", strings.NewReader(reqBody))
	regReq.Header.Set("Content-Type", "application/json")
	regW := httptest.NewRecorder()
	router.ServeHTTP(regW, regReq)

	if regW.Code != http.StatusCreated {
		t.Fatalf("failed to register test agent: %d - %s", regW.Code, regW.Body.String())
	}

	var regResp map[string]interface{}
	if err := json.NewDecoder(regW.Body).Decode(&regResp); err != nil {
		t.Fatalf("failed to decode registration response: %v", err)
	}

	agent, ok := regResp["agent"].(map[string]interface{})
	if !ok {
		t.Fatal("expected agent in registration response")
	}
	agentID, ok := agent["id"].(string)
	if !ok {
		t.Fatal("expected agent id in response")
	}

	// Now GET the agent profile
	req := httptest.NewRequest(http.MethodGet, "/v1/agents/"+agentID, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Should NOT return placeholder "coming soon" message
	if w.Code == http.StatusOK {
		var response map[string]interface{}
		if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		// Check if this is a placeholder response
		if msg, hasMsg := response["message"]; hasMsg {
			if msgStr, isStr := msg.(string); isStr {
				if strings.Contains(strings.ToLower(msgStr), "coming soon") {
					t.Error("GET /v1/agents/{id} returned placeholder 'coming soon' message - routes.go override not removed")
				}
			}
		}

		// Should have real agent data with 'data.agent' structure per handlers/agents.go GetAgent()
		if data, hasData := response["data"]; hasData {
			dataMap, ok := data.(map[string]interface{})
			if ok {
				// Check for agent field inside data
				if agentData, hasAgent := dataMap["agent"].(map[string]interface{}); hasAgent {
					if _, hasID := agentData["id"]; !hasID {
						t.Error("agent data should have id field")
					}
				} else {
					// Maybe direct structure - check for id in data directly
					if _, hasID := dataMap["id"]; !hasID {
						t.Error("data should have agent with id field, or id directly")
					}
				}
			}
		}
	} else if w.Code != http.StatusNotFound {
		// Route not wired if 404, but any other error code is unexpected
		t.Errorf("expected status 200 or 404, got %d: %s", w.Code, w.Body.String())
	}
}

// TestClaimEndpointWithAPIKeyAuth verifies POST /v1/agents/me/claim works with valid API key.
// Per FIX-002: Add API key auth middleware to /v1/agents/me/claim
func TestClaimEndpointWithAPIKeyAuth(t *testing.T) {
	router := setupTestRouter(t)

	// First, register an agent to get an API key
	reqBody := `{"name":"claim_test_agent","description":"Test agent for claim endpoint"}`
	regReq := httptest.NewRequest(http.MethodPost, "/v1/agents/register", strings.NewReader(reqBody))
	regReq.Header.Set("Content-Type", "application/json")
	regW := httptest.NewRecorder()
	router.ServeHTTP(regW, regReq)

	if regW.Code != http.StatusCreated {
		t.Fatalf("failed to register test agent: %d - %s", regW.Code, regW.Body.String())
	}

	var regResp map[string]interface{}
	if err := json.NewDecoder(regW.Body).Decode(&regResp); err != nil {
		t.Fatalf("failed to decode registration response: %v", err)
	}

	apiKey, ok := regResp["api_key"].(string)
	if !ok || apiKey == "" {
		t.Fatal("expected api_key in registration response")
	}

	// Now test POST /v1/agents/me/claim with valid API key
	claimReq := httptest.NewRequest(http.MethodPost, "/v1/agents/me/claim", nil)
	claimReq.Header.Set("Authorization", "Bearer "+apiKey)
	claimW := httptest.NewRecorder()
	router.ServeHTTP(claimW, claimReq)

	// Should return 201 Created with claim URL (or 200 if reusing existing token)
	if claimW.Code != http.StatusCreated && claimW.Code != http.StatusOK {
		t.Errorf("expected status 201 or 200, got %d: %s", claimW.Code, claimW.Body.String())
	}

	var claimResp map[string]interface{}
	if err := json.NewDecoder(claimW.Body).Decode(&claimResp); err != nil {
		t.Fatalf("failed to decode claim response: %v", err)
	}

	// Should have claim_url in response
	if claimResp["claim_url"] == nil {
		t.Error("expected claim_url in response")
	}

	// Should have token in response
	if claimResp["token"] == nil {
		t.Error("expected token in response")
	}
}

// TestClaimEndpointWithInvalidAPIKey verifies POST /v1/agents/me/claim rejects invalid API key.
// Per FIX-002: Add API key auth middleware to /v1/agents/me/claim
func TestClaimEndpointWithInvalidAPIKey(t *testing.T) {
	router := setupTestRouter(t)

	// Try to call claim endpoint with invalid API key
	claimReq := httptest.NewRequest(http.MethodPost, "/v1/agents/me/claim", nil)
	claimReq.Header.Set("Authorization", "Bearer solvr_invalid_api_key_12345678901234567890")
	claimW := httptest.NewRecorder()
	router.ServeHTTP(claimW, claimReq)

	// Should return 401 Unauthorized
	if claimW.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d: %s", claimW.Code, claimW.Body.String())
	}

	var errResp map[string]interface{}
	if err := json.NewDecoder(claimW.Body).Decode(&errResp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}

	// Should have error object
	if errResp["error"] == nil {
		t.Error("expected error object in response")
	}
}

// TestMeEndpointRequiresAuth verifies GET /v1/me requires authentication.
// Per FIX-005: Wire /v1/me endpoint
func TestMeEndpointRequiresAuth(t *testing.T) {
	router := setupTestRouter(t)

	// GET /v1/me without auth should return 401
	req := httptest.NewRequest(http.MethodGet, "/v1/me", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Should NOT return 404 - endpoint should be wired
	if w.Code == http.StatusNotFound {
		t.Errorf("GET /v1/me returned 404 - endpoint not wired (FIX-005)")
	}

	// Should return 401 Unauthorized
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401 (unauthorized), got %d: %s", w.Code, w.Body.String())
	}

	// Verify error response is JSON
	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Should have error object
	if response["error"] == nil {
		t.Error("expected error object in response")
	}
}

// TestMeEndpointWithAPIKey verifies GET /v1/me returns agent info with API key auth.
// Per FIX-005: GET /v1/me with API key returns agent info
func TestMeEndpointWithAPIKey(t *testing.T) {
	router := setupTestRouter(t)

	// First, register an agent to get an API key
	reqBody := `{"name":"me_test_agent","description":"Test agent for /me endpoint"}`
	regReq := httptest.NewRequest(http.MethodPost, "/v1/agents/register", strings.NewReader(reqBody))
	regReq.Header.Set("Content-Type", "application/json")
	regW := httptest.NewRecorder()
	router.ServeHTTP(regW, regReq)

	if regW.Code != http.StatusCreated {
		t.Fatalf("failed to register test agent: %d - %s", regW.Code, regW.Body.String())
	}

	var regResp map[string]interface{}
	if err := json.NewDecoder(regW.Body).Decode(&regResp); err != nil {
		t.Fatalf("failed to decode registration response: %v", err)
	}

	apiKey, ok := regResp["api_key"].(string)
	if !ok || apiKey == "" {
		t.Fatal("expected api_key in registration response")
	}

	// Now test GET /v1/me with valid API key
	meReq := httptest.NewRequest(http.MethodGet, "/v1/me", nil)
	meReq.Header.Set("Authorization", "Bearer "+apiKey)
	meW := httptest.NewRecorder()
	router.ServeHTTP(meW, meReq)

	// Should return 200 OK
	if meW.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", meW.Code, meW.Body.String())
	}

	var meResp map[string]interface{}
	if err := json.NewDecoder(meW.Body).Decode(&meResp); err != nil {
		t.Fatalf("failed to decode /me response: %v", err)
	}

	// Should have data object
	data, ok := meResp["data"].(map[string]interface{})
	if !ok {
		t.Fatal("expected data in response")
	}

	// Should return agent data
	if data["type"] != "agent" {
		t.Errorf("expected type 'agent', got %v", data["type"])
	}

	// Should have agent ID (the registration handler prefixes with "agent_")
	agentID, _ := data["id"].(string)
	if agentID != "me_test_agent" && agentID != "agent_me_test_agent" {
		t.Errorf("expected id to contain 'me_test_agent', got %v", data["id"])
	}

	// Should have display_name
	if data["display_name"] == nil {
		t.Error("expected display_name in response")
	}
}

// TestMCPEndpointExists verifies POST /v1/mcp endpoint is wired.
// Per MCP-005: Add HTTP transport support for MCP
func TestMCPEndpointExists(t *testing.T) {
	router := setupTestRouter(t)

	// POST /v1/mcp with initialize method should return 200 OK
	reqBody := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`
	req := httptest.NewRequest(http.MethodPost, "/v1/mcp", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Should NOT return 404 - endpoint should be wired
	if w.Code == http.StatusNotFound {
		t.Errorf("POST /v1/mcp returned 404 - endpoint not wired (MCP-005)")
	}

	// Should return 200 OK
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Should be valid JSON-RPC 2.0 response
	if response["jsonrpc"] != "2.0" {
		t.Errorf("expected jsonrpc 2.0, got %v", response["jsonrpc"])
	}

	// Should have result (not error)
	if response["error"] != nil {
		t.Errorf("unexpected error: %v", response["error"])
	}

	result, ok := response["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected result to be map, got %T", response["result"])
	}

	// Should return server info
	if result["name"] != "solvr" {
		t.Errorf("expected server name 'solvr', got %v", result["name"])
	}
}

// TestMCPToolsListEndpoint verifies POST /v1/mcp tools/list returns 4 tools.
// Per MCP-005: MCP over HTTP should expose all 4 tools
func TestMCPToolsListEndpoint(t *testing.T) {
	router := setupTestRouter(t)

	reqBody := `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/mcp", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	result, ok := response["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected result to be map, got %T", response["result"])
	}

	tools, ok := result["tools"].([]interface{})
	if !ok {
		t.Fatalf("expected tools to be array, got %T", result["tools"])
	}

	// Should have 4 tools: solvr_search, solvr_get, solvr_post, solvr_answer
	if len(tools) != 4 {
		t.Errorf("expected 4 tools, got %d", len(tools))
	}
}

// =============================================================================
// Search API Auth Tests (Router Level)
//
// These tests verify that the search endpoint REQUIRES authentication.
// Auth is enforced at the router level via UnifiedAuthMiddleware.
// =============================================================================

// TestSearchEndpointRequiresAuth verifies GET /v1/search requires authentication.
// Per user decision: search should not be public - it requires an agent key,
// user API key, or JWT token.
func TestSearchEndpointRequiresAuth(t *testing.T) {
	router := setupTestRouter(t)

	// GET /v1/search without auth should return 401
	req := httptest.NewRequest(http.MethodGet, "/v1/search?q=test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Should NOT return 404 - endpoint should be wired
	if w.Code == http.StatusNotFound {
		t.Errorf("GET /v1/search returned 404 - endpoint not wired")
	}

	// Should return 401 Unauthorized (not 200)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401 (unauthorized), got %d: %s", w.Code, w.Body.String())
	}

	// Verify error response is JSON
	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Should have error object
	if response["error"] == nil {
		t.Error("expected error object in response")
	}
}

// TestSearchEndpointWithAPIKey verifies GET /v1/search works with valid agent API key.
func TestSearchEndpointWithAPIKey(t *testing.T) {
	router := setupTestRouter(t)

	// First, register an agent to get an API key
	reqBody := `{"name":"search_test_agent","description":"Test agent for search endpoint"}`
	regReq := httptest.NewRequest(http.MethodPost, "/v1/agents/register", strings.NewReader(reqBody))
	regReq.Header.Set("Content-Type", "application/json")
	regW := httptest.NewRecorder()
	router.ServeHTTP(regW, regReq)

	if regW.Code != http.StatusCreated {
		t.Fatalf("failed to register test agent: %d - %s", regW.Code, regW.Body.String())
	}

	var regResp map[string]interface{}
	if err := json.NewDecoder(regW.Body).Decode(&regResp); err != nil {
		t.Fatalf("failed to decode registration response: %v", err)
	}

	apiKey, ok := regResp["api_key"].(string)
	if !ok || apiKey == "" {
		t.Fatal("expected api_key in registration response")
	}

	// Now test GET /v1/search with valid API key
	searchReq := httptest.NewRequest(http.MethodGet, "/v1/search?q=test", nil)
	searchReq.Header.Set("Authorization", "Bearer "+apiKey)
	searchW := httptest.NewRecorder()
	router.ServeHTTP(searchW, searchReq)

	// Should return 200 OK
	if searchW.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", searchW.Code, searchW.Body.String())
	}

	var searchResp map[string]interface{}
	if err := json.NewDecoder(searchW.Body).Decode(&searchResp); err != nil {
		t.Fatalf("failed to decode search response: %v", err)
	}

	// Should have data array
	if searchResp["data"] == nil {
		t.Error("expected data in response")
	}

	// Should have meta object
	if searchResp["meta"] == nil {
		t.Error("expected meta in response")
	}
}

// TestSearchEndpointWithInvalidAPIKey verifies GET /v1/search rejects invalid API key.
func TestSearchEndpointWithInvalidAPIKey(t *testing.T) {
	router := setupTestRouter(t)

	// GET /v1/search with invalid API key should return 401
	req := httptest.NewRequest(http.MethodGet, "/v1/search?q=test", nil)
	req.Header.Set("Authorization", "Bearer solvr_invalid_api_key_12345678901234567890")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Should return 401 Unauthorized
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d: %s", w.Code, w.Body.String())
	}

	var errResp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&errResp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}

	// Should have error object
	if errResp["error"] == nil {
		t.Error("expected error object in response")
	}
}

// TestSearchEndpointWithMalformedAuth verifies GET /v1/search rejects malformed auth.
func TestSearchEndpointWithMalformedAuth(t *testing.T) {
	router := setupTestRouter(t)

	// GET /v1/search with malformed auth header should return 401
	req := httptest.NewRequest(http.MethodGet, "/v1/search?q=test", nil)
	req.Header.Set("Authorization", "NotBearer something")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Should return 401 Unauthorized
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d: %s", w.Code, w.Body.String())
	}
}
