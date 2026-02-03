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

	// Make a preflight request
	req := httptest.NewRequest(http.MethodOptions, "/health", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	req.Header.Set("Access-Control-Request-Method", "GET")

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

	for _, header := range expectedHeaders {
		found := false
		// The header might be a comma-separated list
		if exposedHeaders != "" {
			for _, h := range []string{exposedHeaders} {
				if h == header || contains(exposedHeaders, header) {
					found = true
					break
				}
			}
		}
		if !found {
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
	router := NewRouter(nil)

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
	router := NewRouter(nil)

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
	router := NewRouter(nil)

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
	router := NewRouter(nil)

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
	router := NewRouter(nil)

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
	router := NewRouter(nil)

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
