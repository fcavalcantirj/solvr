package middleware

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestLoggingMiddleware verifies that request logging middleware works
func TestLoggingMiddleware(t *testing.T) {
	// Capture log output
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(nil)

	// Create test handler
	handler := Logging(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}))

	// Make request
	req := httptest.NewRequest(http.MethodGet, "/test/path", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	// Verify response came through
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	// Verify log was written
	if buf.Len() == 0 {
		t.Error("expected log output, got none")
	}

	// Verify log is JSON formatted
	logOutput := buf.String()
	// Skip timestamp prefix that log package adds
	jsonStart := bytes.IndexByte(buf.Bytes(), '{')
	if jsonStart == -1 {
		t.Fatal("expected JSON log output")
	}
	jsonBytes := buf.Bytes()[jsonStart:]

	var logEntry map[string]interface{}
	if err := json.Unmarshal(jsonBytes, &logEntry); err != nil {
		t.Fatalf("expected JSON log, got error: %v, log was: %s", err, logOutput)
	}

	// Verify required fields
	if logEntry["method"] != "GET" {
		t.Errorf("expected method 'GET', got '%v'", logEntry["method"])
	}
	if logEntry["path"] != "/test/path" {
		t.Errorf("expected path '/test/path', got '%v'", logEntry["path"])
	}
	if logEntry["status"] == nil {
		t.Error("expected status field in log")
	}
	if logEntry["duration_ms"] == nil {
		t.Error("expected duration_ms field in log")
	}
}

// TestLoggingMiddlewareWithRequestID verifies request ID is logged when present
func TestLoggingMiddlewareWithRequestID(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(nil)

	handler := Logging(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/posts", nil)
	req.Header.Set("X-Request-ID", "test-request-id-123")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	// Parse log
	jsonStart := bytes.IndexByte(buf.Bytes(), '{')
	if jsonStart == -1 {
		t.Fatal("expected JSON log output")
	}
	var logEntry map[string]interface{}
	json.Unmarshal(buf.Bytes()[jsonStart:], &logEntry)

	if logEntry["request_id"] != "test-request-id-123" {
		t.Errorf("expected request_id 'test-request-id-123', got '%v'", logEntry["request_id"])
	}
}

// TestLoggingMiddlewareErrorStatus verifies error status codes are logged
func TestLoggingMiddlewareErrorStatus(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(nil)

	handler := Logging(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))

	req := httptest.NewRequest(http.MethodGet, "/error", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	// Parse log
	jsonStart := bytes.IndexByte(buf.Bytes(), '{')
	var logEntry map[string]interface{}
	json.Unmarshal(buf.Bytes()[jsonStart:], &logEntry)

	// Status should be 500
	if status, ok := logEntry["status"].(float64); !ok || int(status) != 500 {
		t.Errorf("expected status 500, got '%v'", logEntry["status"])
	}
}

// TestLoggingMiddleware_NeverLogsAPIKey verifies that API keys are never logged
func TestLoggingMiddleware_NeverLogsAPIKey(t *testing.T) {
	tests := []struct {
		name      string
		apiKey    string
		headerKey string
	}{
		{
			name:      "solvr API key in Authorization header",
			apiKey:    "solvr_abc123def456ghi789jkl012mno345pqr678",
			headerKey: "Authorization",
		},
		{
			name:      "solvr API key with Bearer prefix",
			apiKey:    "Bearer solvr_test_key_super_secret_value_12345",
			headerKey: "Authorization",
		},
		{
			name:      "solvr API key in X-API-Key header",
			apiKey:    "solvr_my_very_secret_key_should_never_appear",
			headerKey: "X-API-Key",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			log.SetOutput(&buf)
			defer log.SetOutput(nil)

			handler := Logging(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))

			req := httptest.NewRequest(http.MethodGet, "/api/v1/search", nil)
			req.Header.Set(tc.headerKey, tc.apiKey)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			logOutput := buf.String()

			// Extract the actual key value (remove "Bearer " prefix if present)
			keyValue := tc.apiKey
			if len(keyValue) > 7 && keyValue[:7] == "Bearer " {
				keyValue = keyValue[7:]
			}

			// Verify the API key value never appears in logs
			if bytes.Contains(buf.Bytes(), []byte(keyValue)) {
				t.Errorf("API key '%s' was found in logs! Log output: %s", keyValue, logOutput)
			}

			// Verify logs still work (request was logged)
			if buf.Len() == 0 {
				t.Error("expected log output, got none")
			}
		})
	}
}

// TestLoggingMiddleware_RedactsAuthorizationHeader verifies Authorization header is redacted
func TestLoggingMiddleware_RedactsAuthorizationHeader(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(nil)

	handler := Logging(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Test with JWT token
	req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	req.Header.Set("Authorization", "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.secret.signature")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	logOutput := buf.String()

	// JWT token should not appear in logs
	if bytes.Contains(buf.Bytes(), []byte("eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9")) {
		t.Errorf("JWT token was found in logs! Log output: %s", logOutput)
	}
}

// TestLoggingMiddleware_DoesNotLogQueryParamsWithSecrets verifies URL query params with secrets are redacted
func TestLoggingMiddleware_DoesNotLogQueryParamsWithSecrets(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		contains string // string that should NOT appear in logs
	}{
		{
			name:     "api_key query param",
			path:     "/callback?api_key=solvr_secret_key_12345&state=abc",
			contains: "solvr_secret_key_12345",
		},
		{
			name:     "token query param",
			path:     "/callback?token=super_secret_token_value&redirect=/",
			contains: "super_secret_token_value",
		},
		{
			name:     "access_token query param",
			path:     "/oauth/callback?access_token=ghp_xxxxxxxxxxxxxxxxxxxx&scope=user",
			contains: "ghp_xxxxxxxxxxxxxxxxxxxx",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			log.SetOutput(&buf)
			defer log.SetOutput(nil)

			handler := Logging(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))

			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			logOutput := buf.String()

			// Secret value should NOT appear in logs
			if bytes.Contains(buf.Bytes(), []byte(tc.contains)) {
				t.Errorf("Secret value '%s' was found in logs! Log output: %s", tc.contains, logOutput)
			}
		})
	}
}

// TestRedactSensitiveData verifies the redaction helper function
func TestRedactSensitiveData(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "solvr API key",
			input:    "solvr_abc123def456",
			expected: "solvr_***REDACTED***",
		},
		{
			name:     "Bearer token",
			input:    "Bearer eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxIn0.sig",
			expected: "Bearer ***REDACTED***",
		},
		{
			name:     "normal value unchanged",
			input:    "just-a-normal-value",
			expected: "just-a-normal-value",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := RedactSensitiveData(tc.input)
			if result != tc.expected {
				t.Errorf("expected '%s', got '%s'", tc.expected, result)
			}
		})
	}
}

// TestRedactURLPath verifies URL path redaction for sensitive query params
func TestRedactURLPath(t *testing.T) {
	tests := []struct {
		name            string
		input           string
		shouldNotContain []string // Values that should NOT appear in output
		shouldContain   []string // Values that SHOULD appear in output
	}{
		{
			name:             "api_key param redacted",
			input:            "/callback?api_key=solvr_secret&state=abc",
			shouldNotContain: []string{"solvr_secret"},
			shouldContain:    []string{"/callback", "api_key=", "state=abc"},
		},
		{
			name:             "token param redacted",
			input:            "/auth?token=secret_value&redirect=/home",
			shouldNotContain: []string{"secret_value"},
			shouldContain:    []string{"/auth", "token="},
		},
		{
			name:             "access_token param redacted",
			input:            "/oauth?access_token=ghp_xxx&scope=user",
			shouldNotContain: []string{"ghp_xxx"},
			shouldContain:    []string{"/oauth", "access_token=", "scope=user"},
		},
		{
			name:             "no sensitive params unchanged",
			input:            "/api/v1/search?q=test&page=1",
			shouldNotContain: []string{},
			shouldContain:    []string{"/api/v1/search", "q=test", "page=1"},
		},
		{
			name:             "path only unchanged",
			input:            "/api/v1/posts/123",
			shouldNotContain: []string{},
			shouldContain:    []string{"/api/v1/posts/123"},
		},
		{
			name:             "multiple sensitive params",
			input:            "/callback?token=abc&api_key=def&state=ghi",
			shouldNotContain: []string{"abc", "def"},
			shouldContain:    []string{"/callback", "token=", "api_key=", "state=ghi"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := RedactURLPath(tc.input)

			// Check that sensitive values are NOT in the output
			for _, forbidden := range tc.shouldNotContain {
				if bytes.Contains([]byte(result), []byte(forbidden)) {
					t.Errorf("output should NOT contain '%s', got '%s'", forbidden, result)
				}
			}

			// Check that expected strings ARE in the output
			for _, required := range tc.shouldContain {
				if !bytes.Contains([]byte(result), []byte(required)) {
					t.Errorf("output should contain '%s', got '%s'", required, result)
				}
			}
		})
	}
}
