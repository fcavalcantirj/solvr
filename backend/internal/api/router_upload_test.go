package api

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
)

// =============================================================================
// IPFS Upload (POST /v1/add) Route Tests
//
// These tests verify that the upload endpoint is wired into the router
// with proper authentication via UnifiedAuthMiddleware.
// Per prd-v6-ipfs-expanded.json: "Implement POST /v1/add endpoint"
// =============================================================================

// TestAddEndpointRequiresAuth verifies POST /v1/add requires authentication.
func TestAddEndpointRequiresAuth(t *testing.T) {
	router := setupTestRouter(t)

	// Build multipart request
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, err := writer.CreateFormFile("file", "test.txt")
	if err != nil {
		t.Fatalf("failed to create form file: %v", err)
	}
	part.Write([]byte("test content"))
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/v1/add", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Should NOT return 404 - endpoint should be wired
	if w.Code == http.StatusNotFound {
		t.Errorf("POST /v1/add returned 404 - endpoint not wired")
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
	if response["error"] == nil {
		t.Error("expected error object in response")
	}
}

// TestAddEndpointWithAgentAPIKey verifies POST /v1/add works with agent API key auth.
func TestAddEndpointWithAgentAPIKey(t *testing.T) {
	router := setupTestRouter(t)

	// Register an agent to get an API key
	regReq := httptest.NewRequest(http.MethodPost, "/v1/agents/register",
		bytes.NewReader([]byte(`{"name":"upload_test_agent","description":"Test agent for upload API"}`)))
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

	// POST /v1/add with valid API key and multipart file
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, err := writer.CreateFormFile("file", "test-upload.txt")
	if err != nil {
		t.Fatalf("failed to create form file: %v", err)
	}
	part.Write([]byte("agent upload content"))
	writer.Close()

	addReq := httptest.NewRequest(http.MethodPost, "/v1/add", &buf)
	addReq.Header.Set("Content-Type", writer.FormDataContentType())
	addReq.Header.Set("Authorization", "Bearer "+apiKey)

	addW := httptest.NewRecorder()
	router.ServeHTTP(addW, addReq)

	// Should NOT return 401 or 404 - auth should pass and endpoint wired
	if addW.Code == http.StatusUnauthorized {
		t.Errorf("POST /v1/add with valid API key returned 401 - auth not working")
	}
	if addW.Code == http.StatusNotFound {
		t.Errorf("POST /v1/add returned 404 - endpoint not wired")
	}

	// Should return 200 OK (IPFS success) or 500 (IPFS not reachable in test env)
	if addW.Code != http.StatusOK && addW.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200 or 500, got %d: %s", addW.Code, addW.Body.String())
	}
}
