package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
