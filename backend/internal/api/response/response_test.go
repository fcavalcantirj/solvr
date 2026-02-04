package response

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestWriteJSON verifies WriteJSON outputs correct format
func TestWriteJSON(t *testing.T) {
	w := httptest.NewRecorder()

	data := map[string]string{"message": "hello"}
	WriteJSON(w, http.StatusOK, data)

	// Check status code
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	// Check content type
	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("expected Content-Type 'application/json', got '%s'", contentType)
	}

	// Check response body format
	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Should wrap in data envelope
	dataEnvelope, ok := response["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected 'data' envelope, got: %+v", response)
	}
	if dataEnvelope["message"] != "hello" {
		t.Errorf("expected message 'hello', got '%v'", dataEnvelope["message"])
	}
}

// TestWriteJSONWithMeta verifies WriteJSONWithMeta includes metadata
func TestWriteJSONWithMeta(t *testing.T) {
	w := httptest.NewRecorder()

	data := []string{"a", "b", "c"}
	meta := Meta{Total: 100, Page: 1, PerPage: 20, HasMore: true}
	WriteJSONWithMeta(w, http.StatusOK, data, meta)

	var response map[string]interface{}
	json.NewDecoder(w.Body).Decode(&response)

	// Check data
	if response["data"] == nil {
		t.Error("expected 'data' field")
	}

	// Check meta
	metaObj, ok := response["meta"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected 'meta' object, got: %+v", response["meta"])
	}
	if metaObj["total"].(float64) != 100 {
		t.Errorf("expected total 100, got %v", metaObj["total"])
	}
	if metaObj["page"].(float64) != 1 {
		t.Errorf("expected page 1, got %v", metaObj["page"])
	}
	if metaObj["per_page"].(float64) != 20 {
		t.Errorf("expected per_page 20, got %v", metaObj["per_page"])
	}
	if metaObj["has_more"].(bool) != true {
		t.Error("expected has_more true")
	}
}

// TestWriteError verifies WriteError outputs correct format
func TestWriteError(t *testing.T) {
	w := httptest.NewRecorder()

	WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid input")

	// Check status code
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}

	// Check response body format
	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Should have error envelope
	errorEnvelope, ok := response["error"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected 'error' envelope, got: %+v", response)
	}
	if errorEnvelope["code"] != "VALIDATION_ERROR" {
		t.Errorf("expected code 'VALIDATION_ERROR', got '%v'", errorEnvelope["code"])
	}
	if errorEnvelope["message"] != "invalid input" {
		t.Errorf("expected message 'invalid input', got '%v'", errorEnvelope["message"])
	}
}

// TestWriteErrorWithDetails verifies WriteErrorWithDetails includes details
func TestWriteErrorWithDetails(t *testing.T) {
	w := httptest.NewRecorder()

	details := map[string]string{"field": "email", "reason": "invalid format"}
	WriteErrorWithDetails(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid input", details)

	var response map[string]interface{}
	json.NewDecoder(w.Body).Decode(&response)

	errorEnvelope := response["error"].(map[string]interface{})
	detailsObj := errorEnvelope["details"].(map[string]interface{})

	if detailsObj["field"] != "email" {
		t.Errorf("expected field 'email', got '%v'", detailsObj["field"])
	}
}

// TestWriteCreated verifies WriteCreated uses 201 status
func TestWriteCreated(t *testing.T) {
	w := httptest.NewRecorder()

	data := map[string]string{"id": "123"}
	WriteCreated(w, data)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d", w.Code)
	}
}

// TestWriteNoContent verifies WriteNoContent returns 204 with no body
func TestWriteNoContent(t *testing.T) {
	w := httptest.NewRecorder()

	WriteNoContent(w)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected status 204, got %d", w.Code)
	}

	if w.Body.Len() != 0 {
		t.Errorf("expected empty body, got %d bytes", w.Body.Len())
	}
}

// TestWriteInternalErrorWithLog verifies that internal errors are logged with context
func TestWriteInternalErrorWithLog(t *testing.T) {
	// Capture log output
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	w := httptest.NewRecorder()
	err := errors.New("database connection timeout")
	ctx := LogContext{
		Operation: "FindByID",
		Resource:  "post",
		RequestID: "req-123",
		Extra: map[string]string{
			"postID": "post-456",
		},
	}

	WriteInternalErrorWithLog(w, "failed to get post", err, ctx, logger)

	// Check HTTP response
	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", w.Code)
	}

	var response map[string]interface{}
	json.NewDecoder(w.Body).Decode(&response)
	errorEnvelope := response["error"].(map[string]interface{})
	if errorEnvelope["code"] != "INTERNAL_ERROR" {
		t.Errorf("expected code 'INTERNAL_ERROR', got '%v'", errorEnvelope["code"])
	}

	// Check log output
	logOutput := buf.String()
	if logOutput == "" {
		t.Error("expected log output, got none")
	}

	// Verify log contains error details
	if !strings.Contains(logOutput, "failed to get post") {
		t.Errorf("expected log to contain message, got: %s", logOutput)
	}
	if !strings.Contains(logOutput, "database connection timeout") {
		t.Errorf("expected log to contain error, got: %s", logOutput)
	}
	if !strings.Contains(logOutput, "FindByID") {
		t.Errorf("expected log to contain operation, got: %s", logOutput)
	}
	if !strings.Contains(logOutput, "post") {
		t.Errorf("expected log to contain resource, got: %s", logOutput)
	}
	if !strings.Contains(logOutput, "req-123") {
		t.Errorf("expected log to contain request_id, got: %s", logOutput)
	}
	if !strings.Contains(logOutput, "post-456") {
		t.Errorf("expected log to contain postID, got: %s", logOutput)
	}
}

// TestWriteInternalErrorWithLog_MinimalContext verifies logging works with minimal context
func TestWriteInternalErrorWithLog_MinimalContext(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	w := httptest.NewRecorder()
	err := errors.New("unexpected error")
	ctx := LogContext{} // No context provided

	WriteInternalErrorWithLog(w, "something went wrong", err, ctx, logger)

	// Check HTTP response
	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", w.Code)
	}

	// Check log output
	logOutput := buf.String()
	if !strings.Contains(logOutput, "something went wrong") {
		t.Errorf("expected log to contain message, got: %s", logOutput)
	}
	if !strings.Contains(logOutput, "unexpected error") {
		t.Errorf("expected log to contain error, got: %s", logOutput)
	}
}

// TestWriteInternalErrorWithLog_NilLogger verifies graceful handling when logger is nil
func TestWriteInternalErrorWithLog_NilLogger(t *testing.T) {
	w := httptest.NewRecorder()
	err := errors.New("some error")
	ctx := LogContext{}

	// Should not panic with nil logger
	WriteInternalErrorWithLog(w, "error occurred", err, ctx, nil)

	// Check HTTP response is still written
	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", w.Code)
	}
}

// TestLogContext verifies LogContext can hold various extra fields
func TestLogContext_ExtraFields(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	w := httptest.NewRecorder()
	err := errors.New("validation failed")
	ctx := LogContext{
		Operation: "Create",
		Resource:  "post",
		RequestID: "req-789",
		Extra: map[string]string{
			"userID":  "user-123",
			"agentID": "agent-456",
			"type":    "problem",
		},
	}

	WriteInternalErrorWithLog(w, "failed to create post", err, ctx, logger)

	logOutput := buf.String()
	if !strings.Contains(logOutput, "user-123") {
		t.Errorf("expected log to contain userID, got: %s", logOutput)
	}
	if !strings.Contains(logOutput, "agent-456") {
		t.Errorf("expected log to contain agentID, got: %s", logOutput)
	}
	if !strings.Contains(logOutput, "problem") {
		t.Errorf("expected log to contain type, got: %s", logOutput)
	}
}
