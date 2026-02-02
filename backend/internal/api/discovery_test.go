package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestWellKnownAIAgentEndpoint verifies GET /.well-known/ai-agent.json
func TestWellKnownAIAgentEndpoint(t *testing.T) {
	router := NewRouter(nil)

	req := httptest.NewRequest(http.MethodGet, "/.well-known/ai-agent.json", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("expected Content-Type 'application/json', got '%s'", contentType)
	}
}

// TestWellKnownAIAgentContent verifies the content of /.well-known/ai-agent.json
func TestWellKnownAIAgentContent(t *testing.T) {
	router := NewRouter(nil)

	req := httptest.NewRequest(http.MethodGet, "/.well-known/ai-agent.json", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Check required fields per SPEC.md Part 18.3
	requiredFields := []string{"name", "description", "version", "api", "mcp", "cli", "sdks", "capabilities"}
	for _, field := range requiredFields {
		if response[field] == nil {
			t.Errorf("expected field '%s' in response", field)
		}
	}

	// Check name is "Solvr"
	if response["name"] != "Solvr" {
		t.Errorf("expected name to be 'Solvr', got '%v'", response["name"])
	}
}

// TestWellKnownAIAgentAPISection verifies the api section
func TestWellKnownAIAgentAPISection(t *testing.T) {
	router := NewRouter(nil)

	req := httptest.NewRequest(http.MethodGet, "/.well-known/ai-agent.json", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	api, ok := response["api"].(map[string]interface{})
	if !ok {
		t.Fatal("expected 'api' to be an object")
	}

	// Check api section has required fields
	apiFields := []string{"base_url", "openapi", "docs"}
	for _, field := range apiFields {
		if api[field] == nil {
			t.Errorf("expected api.%s in response", field)
		}
	}
}

// TestWellKnownAIAgentMCPSection verifies the mcp section
func TestWellKnownAIAgentMCPSection(t *testing.T) {
	router := NewRouter(nil)

	req := httptest.NewRequest(http.MethodGet, "/.well-known/ai-agent.json", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	mcp, ok := response["mcp"].(map[string]interface{})
	if !ok {
		t.Fatal("expected 'mcp' to be an object")
	}

	// Check mcp section has required fields
	if mcp["url"] == nil {
		t.Error("expected mcp.url in response")
	}
	if mcp["tools"] == nil {
		t.Error("expected mcp.tools in response")
	}
}

// TestWellKnownAIAgentCapabilities verifies the capabilities array
func TestWellKnownAIAgentCapabilities(t *testing.T) {
	router := NewRouter(nil)

	req := httptest.NewRequest(http.MethodGet, "/.well-known/ai-agent.json", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	capabilities, ok := response["capabilities"].([]interface{})
	if !ok {
		t.Fatal("expected 'capabilities' to be an array")
	}

	// Should include search, read, write, webhooks
	expectedCaps := []string{"search", "read", "write", "webhooks"}
	for _, expected := range expectedCaps {
		found := false
		for _, cap := range capabilities {
			if cap == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected capabilities to include '%s'", expected)
		}
	}
}

// TestOpenAPIJSONEndpoint verifies GET /v1/openapi.json
func TestOpenAPIJSONEndpoint(t *testing.T) {
	router := NewRouter(nil)

	req := httptest.NewRequest(http.MethodGet, "/v1/openapi.json", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("expected Content-Type 'application/json', got '%s'", contentType)
	}
}

// TestOpenAPIJSONContent verifies the content is valid OpenAPI 3.0
func TestOpenAPIJSONContent(t *testing.T) {
	router := NewRouter(nil)

	req := httptest.NewRequest(http.MethodGet, "/v1/openapi.json", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Check OpenAPI 3.0 required fields
	openapi, ok := response["openapi"].(string)
	if !ok {
		t.Fatal("expected 'openapi' field")
	}
	if !strings.HasPrefix(openapi, "3.") {
		t.Errorf("expected OpenAPI version 3.x, got '%s'", openapi)
	}

	// Check info object
	info, ok := response["info"].(map[string]interface{})
	if !ok {
		t.Fatal("expected 'info' object")
	}
	if info["title"] == nil {
		t.Error("expected info.title")
	}
	if info["version"] == nil {
		t.Error("expected info.version")
	}

	// Check paths object
	if response["paths"] == nil {
		t.Error("expected 'paths' object")
	}
}

// TestOpenAPIJSONHasSearchEndpoint verifies search endpoint is documented
func TestOpenAPIJSONHasSearchEndpoint(t *testing.T) {
	router := NewRouter(nil)

	req := httptest.NewRequest(http.MethodGet, "/v1/openapi.json", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	paths, ok := response["paths"].(map[string]interface{})
	if !ok {
		t.Fatal("expected 'paths' object")
	}

	// Check /search endpoint is documented
	if paths["/search"] == nil {
		t.Error("expected /search endpoint in OpenAPI spec")
	}
}

// TestOpenAPIYAMLEndpoint verifies GET /v1/openapi.yaml
func TestOpenAPIYAMLEndpoint(t *testing.T) {
	router := NewRouter(nil)

	req := httptest.NewRequest(http.MethodGet, "/v1/openapi.yaml", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	// Accept both text/yaml and application/x-yaml
	if contentType != "text/yaml" && contentType != "application/x-yaml" && contentType != "text/yaml; charset=utf-8" {
		t.Errorf("expected Content-Type to be YAML type, got '%s'", contentType)
	}
}

// TestOpenAPIYAMLContent verifies YAML content starts correctly
func TestOpenAPIYAMLContent(t *testing.T) {
	router := NewRouter(nil)

	req := httptest.NewRequest(http.MethodGet, "/v1/openapi.yaml", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	body := w.Body.String()

	// Check it starts with openapi version
	if !strings.Contains(body, "openapi:") {
		t.Error("expected YAML to contain 'openapi:' field")
	}
	if !strings.Contains(body, "3.") {
		t.Error("expected YAML to contain OpenAPI version 3.x")
	}
	if !strings.Contains(body, "Solvr") {
		t.Error("expected YAML to contain 'Solvr'")
	}
}

// TestOpenAPIYAMLHasSearchPath verifies /search is in YAML spec
func TestOpenAPIYAMLHasSearchPath(t *testing.T) {
	router := NewRouter(nil)

	req := httptest.NewRequest(http.MethodGet, "/v1/openapi.yaml", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	body := w.Body.String()

	if !strings.Contains(body, "/search:") {
		t.Error("expected YAML to contain '/search:' path")
	}
}

// TestDiscoveryEndpointsNoCORS verifies discovery endpoints work without CORS preflight
func TestDiscoveryEndpointsNoCORS(t *testing.T) {
	router := NewRouter(nil)

	endpoints := []string{
		"/.well-known/ai-agent.json",
		"/v1/openapi.json",
		"/v1/openapi.yaml",
	}

	for _, endpoint := range endpoints {
		t.Run(endpoint, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, endpoint, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("expected status 200 for %s, got %d", endpoint, w.Code)
			}
		})
	}
}
