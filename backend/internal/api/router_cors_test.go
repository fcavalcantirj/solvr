package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestRouterCORSHeaders verifies CORS middleware is configured
func TestRouterCORSHeaders(t *testing.T) {
	router := NewRouter(nil, nil, nil)

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

// TestCORSProductionOrigin verifies CORS allows production domain solvr.dev
func TestCORSProductionOrigin(t *testing.T) {
	router := NewRouter(nil, nil, nil)

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
	router := NewRouter(nil, nil, nil)

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
	router := NewRouter(nil, nil, nil)

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
	router := NewRouter(nil, nil, nil)

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
	router := NewRouter(nil, nil, nil)

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
	router := NewRouter(nil, nil, nil)

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
	router := NewRouter(nil, nil, nil)

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
