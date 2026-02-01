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
