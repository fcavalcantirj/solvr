package response

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
