package middleware

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

// TestLoggingMiddleware_CapturesRequestBodyOnError verifies request body is logged for failed POST/PATCH requests
func TestLoggingMiddleware_CapturesRequestBodyOnError(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)

	handler := Logging(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":{"code":"VALIDATION_ERROR","message":"invalid body"}}`))
	}))

	// POST request with body
	body := `{"title":"Test Post","description":"Test description"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/posts", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	// Parse log
	jsonStart := bytes.IndexByte(buf.Bytes(), '{')
	if jsonStart == -1 {
		t.Fatal("expected JSON log output")
	}
	var logEntry map[string]interface{}
	if err := json.Unmarshal(buf.Bytes()[jsonStart:], &logEntry); err != nil {
		t.Fatalf("failed to parse log JSON: %v", err)
	}

	// Verify request_body field is included for failed POST
	if logEntry["request_body"] == nil {
		t.Error("expected 'request_body' field in log for failed POST request")
	}

	// Verify the body content is captured (don't compare exact JSON since key order varies)
	reqBody, ok := logEntry["request_body"].(string)
	if !ok {
		t.Errorf("expected 'request_body' to be a string, got %T", logEntry["request_body"])
	}
	if reqBody == "" {
		t.Error("expected non-empty request_body")
	}

	// Verify body contains expected fields (JSON may have different key order)
	if !bytes.Contains([]byte(reqBody), []byte(`"title":"Test Post"`)) {
		t.Errorf("expected request_body to contain title field, got '%s'", reqBody)
	}
	if !bytes.Contains([]byte(reqBody), []byte(`"description":"Test description"`)) {
		t.Errorf("expected request_body to contain description field, got '%s'", reqBody)
	}
}

// TestLoggingMiddleware_NoRequestBodyForSuccessfulPOST verifies body is NOT logged for successful requests
func TestLoggingMiddleware_NoRequestBodyForSuccessfulPOST(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)

	handler := Logging(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"data":{"id":"123"}}`))
	}))

	body := `{"title":"Test Post","description":"Test description"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/posts", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	// Parse log
	jsonStart := bytes.IndexByte(buf.Bytes(), '{')
	var logEntry map[string]interface{}
	json.Unmarshal(buf.Bytes()[jsonStart:], &logEntry)

	// Verify no request_body field for successful response
	if logEntry["request_body"] != nil {
		t.Errorf("expected no 'request_body' field for successful POST, got %v", logEntry["request_body"])
	}
}

// TestLoggingMiddleware_NoRequestBodyForGET verifies body is NOT logged for GET requests
func TestLoggingMiddleware_NoRequestBodyForGET(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)

	handler := Logging(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":{"code":"BAD_REQUEST","message":"invalid query"}}`))
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/posts", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	// Parse log
	jsonStart := bytes.IndexByte(buf.Bytes(), '{')
	var logEntry map[string]interface{}
	json.Unmarshal(buf.Bytes()[jsonStart:], &logEntry)

	// Verify no request_body field for GET request even on error
	if logEntry["request_body"] != nil {
		t.Errorf("expected no 'request_body' field for GET request, got %v", logEntry["request_body"])
	}
}

// TestLoggingMiddleware_RequestBodyRedactsSensitiveFields verifies sensitive fields are redacted in body
func TestLoggingMiddleware_RequestBodyRedactsSensitiveFields(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)

	handler := Logging(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":{"code":"VALIDATION_ERROR","message":"invalid"}}`))
	}))

	// POST request with sensitive fields
	body := `{"username":"testuser","password":"supersecretpassword123","api_key":"solvr_abc123","token":"mytoken456"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	// Parse log
	jsonStart := bytes.IndexByte(buf.Bytes(), '{')
	var logEntry map[string]interface{}
	json.Unmarshal(buf.Bytes()[jsonStart:], &logEntry)

	// Verify request_body exists
	reqBody, ok := logEntry["request_body"].(string)
	if !ok {
		t.Fatalf("expected 'request_body' to be a string, got %T", logEntry["request_body"])
	}

	// Verify sensitive values are NOT in the logged body
	if bytes.Contains([]byte(reqBody), []byte("supersecretpassword123")) {
		t.Error("password value should be redacted in request_body")
	}
	if bytes.Contains([]byte(reqBody), []byte("solvr_abc123")) {
		t.Error("api_key value should be redacted in request_body")
	}
	if bytes.Contains([]byte(reqBody), []byte("mytoken456")) {
		t.Error("token value should be redacted in request_body")
	}

	// Verify non-sensitive fields are still present
	if !bytes.Contains([]byte(reqBody), []byte("testuser")) {
		t.Error("username should be present in request_body")
	}

	// Verify REDACTED markers are present
	if !bytes.Contains([]byte(reqBody), []byte("***REDACTED***")) {
		t.Error("expected ***REDACTED*** markers in request_body")
	}
}

// TestLoggingMiddleware_RequestBodyTruncatedAt1KB verifies large bodies are truncated
func TestLoggingMiddleware_RequestBodyTruncatedAt1KB(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)

	handler := Logging(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":{"code":"BAD_REQUEST","message":"too large"}}`))
	}))

	// POST request with large body (2KB)
	largeBody := bytes.Repeat([]byte("x"), 2048)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/posts", bytes.NewBuffer(largeBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	// Parse log
	jsonStart := bytes.IndexByte(buf.Bytes(), '{')
	var logEntry map[string]interface{}
	json.Unmarshal(buf.Bytes()[jsonStart:], &logEntry)

	// Verify request_body exists and is truncated
	reqBody, ok := logEntry["request_body"].(string)
	if !ok {
		t.Fatalf("expected 'request_body' to be a string, got %T", logEntry["request_body"])
	}

	// Body should be <= 1KB + truncation indicator
	maxLen := 1024 + len("...[truncated]")
	if len(reqBody) > maxLen {
		t.Errorf("request_body should be truncated to ~1KB, got %d bytes", len(reqBody))
	}

	// Should have truncation indicator
	if !bytes.Contains([]byte(reqBody), []byte("[truncated]")) {
		t.Error("expected truncation indicator in large request_body")
	}
}

// TestLoggingMiddleware_CapturesRequestBodyOnPATCH verifies body is logged for failed PATCH requests
func TestLoggingMiddleware_CapturesRequestBodyOnPATCH(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)

	handler := Logging(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":{"code":"INTERNAL_ERROR","message":"db error"}}`))
	}))

	body := `{"title":"Updated Title"}`
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/posts/123", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	// Parse log
	jsonStart := bytes.IndexByte(buf.Bytes(), '{')
	var logEntry map[string]interface{}
	json.Unmarshal(buf.Bytes()[jsonStart:], &logEntry)

	// Verify request_body field is included for failed PATCH
	if logEntry["request_body"] == nil {
		t.Error("expected 'request_body' field in log for failed PATCH request")
	}

	reqBody, _ := logEntry["request_body"].(string)
	// Verify body contains expected field (JSON may have different key order, but single key is stable)
	if !bytes.Contains([]byte(reqBody), []byte(`"title":"Updated Title"`)) {
		t.Errorf("expected request_body to contain title field, got '%s'", reqBody)
	}
}

// TestRedactRequestBody verifies the body redaction function
func TestRedactRequestBody(t *testing.T) {
	tests := []struct {
		name             string
		input            string
		shouldNotContain []string
		shouldContain    []string
	}{
		{
			name:             "password field redacted",
			input:            `{"username":"user","password":"secret123"}`,
			shouldNotContain: []string{"secret123"},
			shouldContain:    []string{"username", "user", "password", "REDACTED"},
		},
		{
			name:             "api_key field redacted",
			input:            `{"name":"test","api_key":"solvr_abcdef"}`,
			shouldNotContain: []string{"solvr_abcdef"},
			shouldContain:    []string{"name", "test", "api_key", "REDACTED"},
		},
		{
			name:             "token field redacted",
			input:            `{"token":"mytoken","data":"value"}`,
			shouldNotContain: []string{"mytoken"},
			shouldContain:    []string{"token", "data", "value", "REDACTED"},
		},
		{
			name:             "multiple sensitive fields",
			input:            `{"password":"pass1","api_key":"key1","token":"tok1"}`,
			shouldNotContain: []string{"pass1", "key1", "tok1"},
			shouldContain:    []string{"REDACTED"},
		},
		{
			name:             "no sensitive fields unchanged",
			input:            `{"title":"Test","description":"Hello"}`,
			shouldNotContain: []string{},
			shouldContain:    []string{"title", "Test", "description", "Hello"},
		},
		{
			name:             "nested password field",
			input:            `{"user":{"password":"nested_secret"}}`,
			shouldNotContain: []string{"nested_secret"},
			shouldContain:    []string{"password", "REDACTED"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := RedactRequestBody(tc.input)

			for _, forbidden := range tc.shouldNotContain {
				if bytes.Contains([]byte(result), []byte(forbidden)) {
					t.Errorf("output should NOT contain '%s', got '%s'", forbidden, result)
				}
			}

			for _, required := range tc.shouldContain {
				if !bytes.Contains([]byte(result), []byte(required)) {
					t.Errorf("output should contain '%s', got '%s'", required, result)
				}
			}
		})
	}
}
