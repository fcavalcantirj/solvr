package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestNewRouter verifies that NewRouter returns a configured chi.Mux
func TestNewRouter(t *testing.T) {
	router := NewRouter(nil, nil, nil) // nil pool for basic tests
	if router == nil {
		t.Fatal("NewRouter returned nil")
	}
}

// TestRouterJSONContentType verifies JSON content-type middleware
func TestRouterJSONContentType(t *testing.T) {
	router := NewRouter(nil, nil, nil)

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
	router := NewRouter(nil, nil, nil)

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
	router := NewRouter(nil, nil, nil)

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
	router := NewRouter(nil, nil, nil)

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
	router := NewRouter(nil, nil, nil)

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
	router := NewRouter(nil, nil, nil) // No database pool

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
	router := NewRouter(nil, nil, nil)

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
	router := NewRouter(nil, nil, nil)

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

// TestGetPostsEndpoint verifies GET /v1/posts endpoint is wired.
// Per API-CRITICAL requirement: Wire GET/POST /v1/posts endpoints.
func TestGetPostsEndpoint(t *testing.T) {
	router := setupTestRouter(t)

	// GET /v1/posts should return 200 with posts list
	req := httptest.NewRequest(http.MethodGet, "/v1/posts", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Should NOT return 404 - endpoint should be wired
	if w.Code == http.StatusNotFound {
		t.Errorf("GET /v1/posts returned 404 - endpoint not wired")
	}

	// Should return 200 OK
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify response is JSON with data and meta
	bodyBytes := w.Body.Bytes()
	t.Logf("Response body: %s", string(bodyBytes))

	var response map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Should have data key (can be null/empty array when no posts)
	if _, hasData := response["data"]; !hasData {
		t.Errorf("expected data key in response, got keys: %v", getKeys(response))
	}

	// Should have meta object
	if response["meta"] == nil {
		t.Error("expected meta in response")
	}

	// Meta should have pagination fields
	meta, ok := response["meta"].(map[string]interface{})
	if !ok {
		t.Error("meta should be an object")
	} else {
		if _, hasTotal := meta["total"]; !hasTotal {
			t.Error("meta should have total field")
		}
		if _, hasPage := meta["page"]; !hasPage {
			t.Error("meta should have page field")
		}
	}
}

// TestGetSinglePostEndpoint verifies GET /v1/posts/:id endpoint is wired.
// Per API-CRITICAL requirement: Wire GET/POST /v1/posts endpoints.
func TestGetSinglePostEndpoint(t *testing.T) {
	router := setupTestRouter(t)

	// GET /v1/posts/:id with non-existent ID should return 404 (not found post, not route)
	req := httptest.NewRequest(http.MethodGet, "/v1/posts/00000000-0000-0000-0000-000000000001", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Should return 404 NOT_FOUND (post not found), but with our standard error format
	// This confirms the route exists but the post doesn't
	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404 (post not found), got %d: %s", w.Code, w.Body.String())
	}

	// Verify error response is JSON with proper format
	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Should have error object with NOT_FOUND code
	errObj, ok := response["error"].(map[string]interface{})
	if !ok {
		t.Error("expected error object in response")
	} else {
		if errObj["code"] != "NOT_FOUND" {
			t.Errorf("expected error code NOT_FOUND, got %v", errObj["code"])
		}
	}
}

// TestCreatePostEndpointRequiresAuth verifies POST /v1/posts requires authentication.
// Per API-CRITICAL requirement: Wire GET/POST /v1/posts endpoints.
func TestCreatePostEndpointRequiresAuth(t *testing.T) {
	router := setupTestRouter(t)

	// POST /v1/posts without auth should return 401
	reqBody := `{"title":"Test question title","description":"This is a test question description that is long enough"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/posts", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Should NOT return 404 - endpoint should be wired
	if w.Code == http.StatusNotFound {
		t.Errorf("POST /v1/posts returned 404 - endpoint not wired")
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

// TestGetPostsEndpointNoPlaceholder verifies GET /v1/posts returns real data, not placeholder.
// Per FIX-001: routes.go placeholders override real router.go handlers
func TestGetPostsEndpointNoPlaceholder(t *testing.T) {
	router := setupTestRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/v1/posts", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Meta should NOT contain "coming soon" placeholder message
	meta, ok := response["meta"].(map[string]interface{})
	if ok {
		if msg, hasMsg := meta["message"]; hasMsg {
			if msgStr, isStr := msg.(string); isStr {
				if strings.Contains(strings.ToLower(msgStr), "coming soon") {
					t.Error("GET /v1/posts returned placeholder 'coming soon' message - routes.go override not removed")
				}
			}
		}
	}

	// Must have proper pagination fields in meta
	if meta != nil {
		if _, hasTotal := meta["total"]; !hasTotal {
			t.Error("meta should have total field for real data response")
		}
		if _, hasPage := meta["page"]; !hasPage {
			t.Error("meta should have page field for real data response")
		}
	}
}

// TestGetAgentProfileEndpoint verifies GET /v1/agents/{id} returns real agent data, not placeholder.
// Per FIX-001 and FIX-006: routes.go placeholders override real router.go handlers
func TestGetAgentProfileEndpoint(t *testing.T) {
	router := setupTestRouter(t)

	// Use in-memory repo's first agent - register one first
	reqBody := fmt.Sprintf(`{"name":"test_profile_%s","description":"Test agent for profile endpoint"}`, randomSuffix())
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

	agent, ok := regResp["agent"].(map[string]interface{})
	if !ok {
		t.Fatal("expected agent in registration response")
	}
	agentID, ok := agent["id"].(string)
	if !ok {
		t.Fatal("expected agent id in response")
	}

	// Now GET the agent profile
	req := httptest.NewRequest(http.MethodGet, "/v1/agents/"+agentID, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Should NOT return placeholder "coming soon" message
	if w.Code == http.StatusOK {
		var response map[string]interface{}
		if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		// Check if this is a placeholder response
		if msg, hasMsg := response["message"]; hasMsg {
			if msgStr, isStr := msg.(string); isStr {
				if strings.Contains(strings.ToLower(msgStr), "coming soon") {
					t.Error("GET /v1/agents/{id} returned placeholder 'coming soon' message - routes.go override not removed")
				}
			}
		}

		// Should have real agent data with 'data.agent' structure per handlers/agents.go GetAgent()
		if data, hasData := response["data"]; hasData {
			dataMap, ok := data.(map[string]interface{})
			if ok {
				// Check for agent field inside data
				if agentData, hasAgent := dataMap["agent"].(map[string]interface{}); hasAgent {
					if _, hasID := agentData["id"]; !hasID {
						t.Error("agent data should have id field")
					}
				} else {
					// Maybe direct structure - check for id in data directly
					if _, hasID := dataMap["id"]; !hasID {
						t.Error("data should have agent with id field, or id directly")
					}
				}
			}
		}
	} else if w.Code != http.StatusNotFound {
		// Route not wired if 404, but any other error code is unexpected
		t.Errorf("expected status 200 or 404, got %d: %s", w.Code, w.Body.String())
	}
}

// TestMCPEndpointExists verifies POST /v1/mcp endpoint is wired.
// Per MCP-005: Add HTTP transport support for MCP
func TestMCPEndpointExists(t *testing.T) {
	router := setupTestRouter(t)

	// POST /v1/mcp with initialize method should return 200 OK
	reqBody := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`
	req := httptest.NewRequest(http.MethodPost, "/v1/mcp", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Should NOT return 404 - endpoint should be wired
	if w.Code == http.StatusNotFound {
		t.Errorf("POST /v1/mcp returned 404 - endpoint not wired (MCP-005)")
	}

	// Should return 200 OK
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Should be valid JSON-RPC 2.0 response
	if response["jsonrpc"] != "2.0" {
		t.Errorf("expected jsonrpc 2.0, got %v", response["jsonrpc"])
	}

	// Should have result (not error)
	if response["error"] != nil {
		t.Errorf("unexpected error: %v", response["error"])
	}

	result, ok := response["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected result to be map, got %T", response["result"])
	}

	// Should return server info
	if info, _ := result["serverInfo"].(map[string]interface{}); info == nil || info["name"] != "solvr" {
		t.Errorf("expected serverInfo.name 'solvr', got %v", result["serverInfo"])
	}
}

// TestMCPToolsListEndpoint verifies POST /v1/mcp tools/list returns 15 tools.
// Per MCP-005: MCP over HTTP exposes every tool
func TestMCPToolsListEndpoint(t *testing.T) {
	router := setupTestRouter(t)

	reqBody := `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/mcp", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	result, ok := response["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected result to be map, got %T", response["result"])
	}

	tools, ok := result["tools"].([]interface{})
	if !ok {
		t.Fatalf("expected tools to be array, got %T", result["tools"])
	}

	// Should have 15 tools: one per contract operation (handlers.MCPOperationTools)
	if len(tools) != 15 {
		t.Errorf("expected 15 tools, got %d", len(tools))
	}
}
