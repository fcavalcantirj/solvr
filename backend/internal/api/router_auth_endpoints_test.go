package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// TestAgentsRegisterEndpoint verifies POST /v1/agents/register endpoint is wired.
// Per API-CRITICAL requirement: Wire POST /v1/agents/register endpoint.
func TestAgentsRegisterEndpoint(t *testing.T) {
	router := setupTestRouter(t)

	// Create a valid registration request
	reqBody := fmt.Sprintf(`{"name":"test_agent_%s","description":"A test agent"}`, randomSuffix())
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
	router := setupTestRouter(t)

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

// TestGetClaimInfoEndpointExists verifies POST /v1/agents/claim/lookup endpoint exists.
// Per API-CRITICAL requirement: wire the claim info endpoint. It replaced GET /v1/claim/{token}
// (idx 75 step 5): a claim token is a credential and must not travel in a URL.
func TestGetClaimInfoEndpointExists(t *testing.T) {
	router := setupTestRouter(t)

	// POST /v1/agents/claim/lookup should not return 404
	req := httptest.NewRequest(http.MethodPost, "/v1/agents/claim/lookup", strings.NewReader(`{"token":"test_token_123"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Should NOT return 404 - endpoint should be wired
	if w.Code == http.StatusNotFound {
		t.Errorf("POST /v1/agents/claim/lookup returned 404 - endpoint not wired")
	}

	// LookupClaim returns 200 with token_valid: false for invalid tokens
	// or 500 if claim token repo not configured
	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200 or 500, got %d: %s", w.Code, w.Body.String())
	}
}

// TestConfirmClaimEndpointExists verifies the claim info endpoint is a POST and the retired
// path-token GET route is gone.
func TestConfirmClaimEndpointExists(t *testing.T) {
	router := setupTestRouter(t)

	// The lookup is public and wired as POST: a GET must not be answered as if it were wired.
	req := httptest.NewRequest(http.MethodGet, "/v1/agents/claim/lookup", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusMethodNotAllowed && w.Code != http.StatusNotFound {
		t.Errorf("GET /v1/agents/claim/lookup returned %d, want 405 (wired as POST only)", w.Code)
	}

	// GET /v1/claim/{token} put the credential in the path; it is retired.
	req = httptest.NewRequest(http.MethodGet, "/v1/claim/test_token_123", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("GET /v1/claim/{token} returned %d, want 404 (retired)", w.Code)
	}
}

// TestGitHubOAuthRedirectEndpoint verifies GET /v1/auth/github endpoint is wired.
// Per API-CRITICAL requirement: Wire OAuth endpoints /v1/auth/github/*.
func TestGitHubOAuthRedirectEndpoint(t *testing.T) {
	router := setupTestRouter(t)

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
	router := setupTestRouter(t)

	// GET /v1/auth/github/callback without code should return 400 (not 404)
	req := httptest.NewRequest(http.MethodGet, "/v1/auth/github/callback", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Should NOT return 404 - endpoint should be wired
	if w.Code == http.StatusNotFound {
		t.Errorf("GET /v1/auth/github/callback returned 404 - endpoint not wired")
	}

	// A callback without an authorization code ends on the frontend's error page (every
	// callback error redirects; Felipe, 2026-09-30).
	if w.Code != http.StatusFound {
		t.Fatalf("expected status 302 (missing code), got %d: %s", w.Code, w.Body.String())
	}
	loc, err := url.Parse(w.Header().Get("Location"))
	if err != nil || loc.Path != "/auth/callback" || loc.Query().Get("error") != "missing_code" {
		t.Errorf("expected a redirect to /auth/callback?error=missing_code, got %q", w.Header().Get("Location"))
	}
}

// TestGoogleOAuthRedirectEndpoint verifies GET /v1/auth/google endpoint is wired.
// Per API-CRITICAL requirement: Wire OAuth endpoints /v1/auth/google/*.
func TestGoogleOAuthRedirectEndpoint(t *testing.T) {
	router := setupTestRouter(t)

	// GET /v1/auth/google should redirect to Google OAuth
	req := httptest.NewRequest(http.MethodGet, "/v1/auth/google", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Should NOT return 404 - endpoint should be wired
	if w.Code == http.StatusNotFound {
		t.Errorf("GET /v1/auth/google returned 404 - endpoint not wired")
	}

	// Should return 302 Found (redirect)
	if w.Code != http.StatusFound {
		t.Errorf("expected status 302 (redirect), got %d: %s", w.Code, w.Body.String())
	}

	// Check redirect location contains Google OAuth URL
	location := w.Header().Get("Location")
	if location == "" {
		t.Error("expected Location header for redirect")
	}
	if !strings.Contains(location, "accounts.google.com/o/oauth2") {
		t.Errorf("expected redirect to Google OAuth, got Location: %s", location)
	}
}

// getKeys returns the keys of a map
func getKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// TestGoogleOAuthCallbackEndpoint verifies GET /v1/auth/google/callback endpoint is wired.
// Per API-CRITICAL requirement: Wire OAuth endpoints /v1/auth/google/*.
func TestGoogleOAuthCallbackEndpoint(t *testing.T) {
	router := setupTestRouter(t)

	// GET /v1/auth/google/callback without code should return 400 (not 404)
	req := httptest.NewRequest(http.MethodGet, "/v1/auth/google/callback", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Should NOT return 404 - endpoint should be wired
	if w.Code == http.StatusNotFound {
		t.Errorf("GET /v1/auth/google/callback returned 404 - endpoint not wired")
	}

	// A callback without an authorization code ends on the frontend's error page (every
	// callback error redirects; Felipe, 2026-09-30).
	if w.Code != http.StatusFound {
		t.Fatalf("expected status 302 (missing code), got %d: %s", w.Code, w.Body.String())
	}
	loc, err := url.Parse(w.Header().Get("Location"))
	if err != nil || loc.Path != "/auth/callback" || loc.Query().Get("error") != "missing_code" {
		t.Errorf("expected a redirect to /auth/callback?error=missing_code, got %q", w.Header().Get("Location"))
	}
}

// TestClaimEndpointWithAPIKeyAuth verifies POST /v1/agents/me/claim works with valid API key.
// Per FIX-002: Add API key auth middleware to /v1/agents/me/claim
func TestClaimEndpointWithAPIKeyAuth(t *testing.T) {
	router := setupTestRouter(t)

	// First, register an agent to get an API key
	reqBody := fmt.Sprintf(`{"name":"claim_test_%s","description":"Test agent for claim endpoint"}`, randomSuffix())
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

	// Now test POST /v1/agents/me/claim with valid API key
	claimReq := httptest.NewRequest(http.MethodPost, "/v1/agents/me/claim", nil)
	claimReq.Header.Set("Authorization", "Bearer "+apiKey)
	claimW := httptest.NewRecorder()
	router.ServeHTTP(claimW, claimReq)

	// Should return 201 Created with claim URL (or 200 if reusing existing token)
	if claimW.Code != http.StatusCreated && claimW.Code != http.StatusOK {
		t.Errorf("expected status 201 or 200, got %d: %s", claimW.Code, claimW.Body.String())
	}

	var claimResp map[string]interface{}
	if err := json.NewDecoder(claimW.Body).Decode(&claimResp); err != nil {
		t.Fatalf("failed to decode claim response: %v", err)
	}

	// Should have claim_url in response
	if claimResp["claim_url"] == nil {
		t.Error("expected claim_url in response")
	}

	// Should have token in response
	if claimResp["token"] == nil {
		t.Error("expected token in response")
	}
}

// TestClaimEndpointWithInvalidAPIKey verifies POST /v1/agents/me/claim rejects invalid API key.
// Per FIX-002: Add API key auth middleware to /v1/agents/me/claim
func TestClaimEndpointWithInvalidAPIKey(t *testing.T) {
	router := setupTestRouter(t)

	// Try to call claim endpoint with invalid API key
	claimReq := httptest.NewRequest(http.MethodPost, "/v1/agents/me/claim", nil)
	claimReq.Header.Set("Authorization", "Bearer solvr_invalid_api_key_12345678901234567890")
	claimW := httptest.NewRecorder()
	router.ServeHTTP(claimW, claimReq)

	// Should return 401 Unauthorized
	if claimW.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d: %s", claimW.Code, claimW.Body.String())
	}

	var errResp map[string]interface{}
	if err := json.NewDecoder(claimW.Body).Decode(&errResp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}

	// Should have error object
	if errResp["error"] == nil {
		t.Error("expected error object in response")
	}
}

// TestMeEndpointRequiresAuth verifies GET /v1/me requires authentication.
// Per FIX-005: Wire /v1/me endpoint
func TestMeEndpointRequiresAuth(t *testing.T) {
	router := setupTestRouter(t)

	// GET /v1/me without auth should return 401
	req := httptest.NewRequest(http.MethodGet, "/v1/me", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Should NOT return 404 - endpoint should be wired
	if w.Code == http.StatusNotFound {
		t.Errorf("GET /v1/me returned 404 - endpoint not wired (FIX-005)")
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

	// Should have error object
	if response["error"] == nil {
		t.Error("expected error object in response")
	}
}

// TestMeEndpointWithAPIKey verifies GET /v1/me returns agent info with API key auth.
// Per FIX-005: GET /v1/me with API key returns agent info
func TestMeEndpointWithAPIKey(t *testing.T) {
	router := setupTestRouter(t)

	// First, register an agent to get an API key
	reqBody := fmt.Sprintf(`{"name":"me_test_%s","description":"Test agent for /me endpoint"}`, randomSuffix())
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

	// Now test GET /v1/me with valid API key
	meReq := httptest.NewRequest(http.MethodGet, "/v1/me", nil)
	meReq.Header.Set("Authorization", "Bearer "+apiKey)
	meW := httptest.NewRecorder()
	router.ServeHTTP(meW, meReq)

	// Should return 200 OK
	if meW.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", meW.Code, meW.Body.String())
	}

	var meResp map[string]interface{}
	if err := json.NewDecoder(meW.Body).Decode(&meResp); err != nil {
		t.Fatalf("failed to decode /me response: %v", err)
	}

	// Should have data object
	data, ok := meResp["data"].(map[string]interface{})
	if !ok {
		t.Fatal("expected data in response")
	}

	// Should return agent data
	if data["type"] != "agent" {
		t.Errorf("expected type 'agent', got %v", data["type"])
	}

	// Should have agent ID (the registration handler prefixes with "agent_")
	agentID, _ := data["id"].(string)
	if !strings.Contains(agentID, "me_test_") {
		t.Errorf("expected id to contain 'me_test_', got %v", data["id"])
	}

	// Should have display_name
	if data["display_name"] == nil {
		t.Error("expected display_name in response")
	}
}

// =============================================================================
// Search API Public Access Tests (Router Level)
//
// These tests verify that the search endpoint is PUBLIC (no auth required).
// Per SPEC.md Part 5.6: "All content should be publicly discoverable and readable"
// =============================================================================

// TestSearchEndpointRequiresAuth verifies GET /v1/search is publicly accessible.
// UPDATED: Search is now public per SPEC.md Part 5.6
func TestSearchEndpointRequiresAuth(t *testing.T) {
	router := setupTestRouter(t)

	// GET /v1/search without auth should return 200 OK
	req := httptest.NewRequest(http.MethodGet, "/v1/search?q=test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Should NOT return 404 - endpoint should be wired
	if w.Code == http.StatusNotFound {
		t.Errorf("GET /v1/search returned 404 - endpoint not wired")
	}

	// Should return 200 OK (search is public)
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200 (public access), got %d: %s", w.Code, w.Body.String())
	}

	// Verify response is JSON
	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Should have data array (not error object)
	if response["data"] == nil {
		t.Error("expected data in response")
	}
}

// TestSearchEndpointWithAPIKey verifies GET /v1/search works with valid agent API key.
func TestSearchEndpointWithAPIKey(t *testing.T) {
	router := setupTestRouter(t)

	// First, register an agent to get an API key
	reqBody := fmt.Sprintf(`{"name":"search_test_%s","description":"Test agent for search endpoint"}`, randomSuffix())
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

	// Now test GET /v1/search with valid API key
	searchReq := httptest.NewRequest(http.MethodGet, "/v1/search?q=test", nil)
	searchReq.Header.Set("Authorization", "Bearer "+apiKey)
	searchW := httptest.NewRecorder()
	router.ServeHTTP(searchW, searchReq)

	// Should return 200 OK
	if searchW.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", searchW.Code, searchW.Body.String())
	}

	var searchResp map[string]interface{}
	if err := json.NewDecoder(searchW.Body).Decode(&searchResp); err != nil {
		t.Fatalf("failed to decode search response: %v", err)
	}

	// Should have data array
	if searchResp["data"] == nil {
		t.Error("expected data in response")
	}

	// Should have meta object
	if searchResp["meta"] == nil {
		t.Error("expected meta in response")
	}
}

// TestSearchEndpointWithInvalidAPIKey verifies GET /v1/search rejects an invalid API key.
// Search remains public when the Authorization header is omitted.
func TestSearchEndpointWithInvalidAPIKey(t *testing.T) {
	router := setupTestRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/v1/search?q=test", nil)
	req.Header.Set("Authorization", "Bearer solvr_invalid_api_key_12345678901234567890")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	errObj, ok := resp["error"].(map[string]interface{})
	if !ok || errObj["code"] != "INVALID_API_KEY" {
		t.Errorf("expected INVALID_API_KEY error, got %#v", resp)
	}
}

// TestSearchEndpointWithMalformedAuth verifies GET /v1/search rejects a malformed
// Authorization header instead of silently treating it as anonymous.
func TestSearchEndpointWithMalformedAuth(t *testing.T) {
	router := setupTestRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/v1/search?q=test", nil)
	req.Header.Set("Authorization", "NotBearer something")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d: %s", w.Code, w.Body.String())
	}
}
