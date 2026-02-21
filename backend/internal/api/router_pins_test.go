package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// =============================================================================
// IPFS Pinning API Route Tests
//
// These tests verify that the pinning API endpoints are wired into the router
// with proper authentication via UnifiedAuthMiddleware.
// Per prd-v6-ipfs-expanded.json: "Add routes for pinning API to router.go"
// =============================================================================

// TestPinsCreateEndpointRequiresAuth verifies POST /v1/pins requires authentication.
func TestPinsCreateEndpointRequiresAuth(t *testing.T) {
	router := setupTestRouter(t)

	// POST /v1/pins without auth should return 401
	reqBody := `{"cid":"QmYwAPJzv5CZsnA625s3Xf2nemtYgPpHdWEz79ojWnPbdG","name":"test"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/pins", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Should NOT return 404 - endpoint should be wired
	if w.Code == http.StatusNotFound {
		t.Errorf("POST /v1/pins returned 404 - endpoint not wired")
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

// TestPinsListEndpointRequiresAuth verifies GET /v1/pins requires authentication.
func TestPinsListEndpointRequiresAuth(t *testing.T) {
	router := setupTestRouter(t)

	// GET /v1/pins without auth should return 401
	req := httptest.NewRequest(http.MethodGet, "/v1/pins", nil)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Should NOT return 404 - endpoint should be wired
	if w.Code == http.StatusNotFound {
		t.Errorf("GET /v1/pins returned 404 - endpoint not wired")
	}

	// Should return 401 Unauthorized
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401 (unauthorized), got %d: %s", w.Code, w.Body.String())
	}
}

// TestPinsGetByRequestIDEndpointRequiresAuth verifies GET /v1/pins/:requestid requires auth.
func TestPinsGetByRequestIDEndpointRequiresAuth(t *testing.T) {
	router := setupTestRouter(t)

	// GET /v1/pins/:requestid without auth should return 401
	req := httptest.NewRequest(http.MethodGet, "/v1/pins/00000000-0000-0000-0000-000000000001", nil)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Should NOT return 404 - endpoint should be wired
	if w.Code == http.StatusNotFound {
		t.Errorf("GET /v1/pins/:requestid returned 404 - endpoint not wired")
	}

	// Should return 401 Unauthorized
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401 (unauthorized), got %d: %s", w.Code, w.Body.String())
	}
}

// TestPinsDeleteEndpointRequiresAuth verifies DELETE /v1/pins/:requestid requires auth.
func TestPinsDeleteEndpointRequiresAuth(t *testing.T) {
	router := setupTestRouter(t)

	// DELETE /v1/pins/:requestid without auth should return 401
	req := httptest.NewRequest(http.MethodDelete, "/v1/pins/00000000-0000-0000-0000-000000000001", nil)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Should NOT return 404 - endpoint should be wired
	if w.Code == http.StatusNotFound {
		t.Errorf("DELETE /v1/pins/:requestid returned 404 - endpoint not wired")
	}

	// Should return 401 Unauthorized
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401 (unauthorized), got %d: %s", w.Code, w.Body.String())
	}
}

// TestPinsCreateWithAgentAPIKey verifies POST /v1/pins works with agent API key auth.
func TestPinsCreateWithAgentAPIKey(t *testing.T) {
	router := setupTestRouter(t)

	// First, register an agent to get an API key
	reqBody := fmt.Sprintf(`{"name":"pins_test_%s","description":"Test agent for pinning API"}`, randomSuffix())
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

	// POST /v1/pins with valid API key and valid CID
	// The IPFS service may not be running, so we test that auth passes
	// by verifying we get a non-401 response
	pinBody := `{"cid":"QmYwAPJzv5CZsnA625s3Xf2nemtYgPpHdWEz79ojWnPbdG","name":"test-pin"}`
	pinReq := httptest.NewRequest(http.MethodPost, "/v1/pins", strings.NewReader(pinBody))
	pinReq.Header.Set("Content-Type", "application/json")
	pinReq.Header.Set("Authorization", "Bearer "+apiKey)

	pinW := httptest.NewRecorder()
	router.ServeHTTP(pinW, pinReq)

	// Should NOT return 401 or 404 - auth should pass and endpoint should be wired
	if pinW.Code == http.StatusUnauthorized {
		t.Errorf("POST /v1/pins with valid API key returned 401 - auth not working for pins")
	}
	if pinW.Code == http.StatusNotFound {
		t.Errorf("POST /v1/pins returned 404 - endpoint not wired")
	}

	// Should return 202 Accepted (pin created and queued) or 500 if IPFS unavailable
	if pinW.Code != http.StatusAccepted && pinW.Code != http.StatusInternalServerError {
		t.Errorf("expected status 202 or 500, got %d: %s", pinW.Code, pinW.Body.String())
	}
}

// TestPinsListWithAgentAPIKey verifies GET /v1/pins works with agent API key auth.
func TestPinsListWithAgentAPIKey(t *testing.T) {
	router := setupTestRouter(t)

	// Register an agent first
	reqBody := fmt.Sprintf(`{"name":"pins_list_%s","description":"Test agent for listing pins"}`, randomSuffix())
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

	// GET /v1/pins with valid API key
	listReq := httptest.NewRequest(http.MethodGet, "/v1/pins", nil)
	listReq.Header.Set("Authorization", "Bearer "+apiKey)

	listW := httptest.NewRecorder()
	router.ServeHTTP(listW, listReq)

	// Should NOT return 401 or 404
	if listW.Code == http.StatusUnauthorized {
		t.Errorf("GET /v1/pins with valid API key returned 401 - auth not working")
	}
	if listW.Code == http.StatusNotFound {
		t.Errorf("GET /v1/pins returned 404 - endpoint not wired")
	}

	// Should return 200 OK with empty list or 500 if DB issue
	if listW.Code != http.StatusOK && listW.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200 or 500, got %d: %s", listW.Code, listW.Body.String())
	}

	if listW.Code == http.StatusOK {
		var resp map[string]interface{}
		if err := json.NewDecoder(listW.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		// Should have count and results per Pinning Service API format
		if _, hasCount := resp["count"]; !hasCount {
			t.Error("expected count in response")
		}
		if _, hasResults := resp["results"]; !hasResults {
			t.Error("expected results in response")
		}
	}
}
