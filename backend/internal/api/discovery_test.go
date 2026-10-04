package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// TestWellKnownAIAgentEndpoint verifies GET /.well-known/ai-agent.json
func TestWellKnownAIAgentEndpoint(t *testing.T) {
	router := NewRouter(nil, nil, nil)

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
	router := NewRouter(nil, nil, nil)

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
	router := NewRouter(nil, nil, nil)

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
	router := NewRouter(nil, nil, nil)

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
	router := NewRouter(nil, nil, nil)

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
	router := NewRouter(nil, nil, nil)

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
	router := NewRouter(nil, nil, nil)

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
	router := NewRouter(nil, nil, nil)

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
	router := NewRouter(nil, nil, nil)

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
	router := NewRouter(nil, nil, nil)

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
	router := NewRouter(nil, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/v1/openapi.yaml", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	body := w.Body.String()

	if !strings.Contains(body, "/search:") {
		t.Error("expected YAML to contain '/search:' path")
	}
}

// TestOpenAPISpec_IncludesPinEndpoints verifies pin/checkpoint/resurrection/identity paths are in OpenAPI spec
func TestOpenAPISpec_IncludesPinEndpoints(t *testing.T) {
	router := NewRouter(nil, nil, nil)

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

	expectedPaths := []string{
		"/pins",
		"/pins/{requestid}",
		"/agents/{id}/pins",
		"/agents/{id}/checkpoints",
		"/agents/me/checkpoints",
		"/agents/{id}/resurrection-bundle",
		"/agents/me/identity",
	}

	for _, path := range expectedPaths {
		if paths[path] == nil {
			t.Errorf("expected path '%s' in OpenAPI spec", path)
		}
	}
}

// TestOpenAPISpec_PinSchemas verifies pin-related schemas exist in OpenAPI components
func TestOpenAPISpec_PinSchemas(t *testing.T) {
	router := NewRouter(nil, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/v1/openapi.json", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	components, ok := response["components"].(map[string]interface{})
	if !ok {
		t.Fatal("expected 'components' object")
	}

	schemas, ok := components["schemas"].(map[string]interface{})
	if !ok {
		t.Fatal("expected 'schemas' object in components")
	}

	expectedSchemas := []string{
		"PinResponse",
		"PinInfo",
		"CreatePinRequest",
		"CreateCheckpointRequest",
		"ResurrectionBundleResponse",
	}

	for _, schema := range expectedSchemas {
		if schemas[schema] == nil {
			t.Errorf("expected schema '%s' in OpenAPI components", schema)
		}
	}
}

// TestRobotsTxtEndpoint verifies GET /robots.txt returns Disallow all
func TestRobotsTxtEndpoint(t *testing.T) {
	router := NewRouter(nil, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/robots.txt", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if !strings.Contains(contentType, "text/plain") {
		t.Errorf("expected Content-Type 'text/plain', got '%s'", contentType)
	}

	body := w.Body.String()
	if !strings.Contains(body, "User-agent: *") {
		t.Error("expected 'User-agent: *' in robots.txt")
	}
	if !strings.Contains(body, "Disallow: /") {
		t.Error("expected 'Disallow: /' in robots.txt")
	}
}

// TestDiscoveryEndpointsNoCORS verifies discovery endpoints work without CORS preflight
func TestDiscoveryEndpointsNoCORS(t *testing.T) {
	router := NewRouter(nil, nil, nil)

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

// TestWellKnownAIAgentMCPTools_MatchWhatV1MCPServes: the discovery document lists exactly the
// tools POST /v1/mcp tools/list serves (idx 52: solvr_answer became solvr_reply), so an agent
// that discovers Solvr is never told about a tool the server no longer offers.
func TestWellKnownAIAgentMCPTools_MatchWhatV1MCPServes(t *testing.T) {
	router := setupTestRouter(t)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/.well-known/ai-agent.json", nil))
	var discovery struct {
		MCP struct {
			Tools []string `json:"tools"`
		} `json:"mcp"`
	}
	if err := json.NewDecoder(w.Body).Decode(&discovery); err != nil {
		t.Fatalf("decode discovery: %v", err)
	}

	w = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/mcp",
		strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	var list struct {
		Result struct {
			Tools []struct {
				Name string `json:"name"`
			} `json:"tools"`
		} `json:"result"`
	}
	if err := json.NewDecoder(w.Body).Decode(&list); err != nil {
		t.Fatalf("decode tools/list: %v", err)
	}
	served := make([]string, 0, len(list.Result.Tools))
	for _, tool := range list.Result.Tools {
		served = append(served, tool.Name)
	}

	if len(served) == 0 {
		t.Fatal("POST /v1/mcp tools/list served no tools")
	}
	if strings.Join(discovery.MCP.Tools, ",") != strings.Join(served, ",") {
		t.Errorf("ai-agent.json mcp.tools = %v, /v1/mcp serves %v", discovery.MCP.Tools, served)
	}
}

// Every way to connect that ai-agent.json advertises exists: the MCP url is the route that
// serves MCP over HTTP, the docs are the site's API reference, and the CLI and SDK are Go
// paths in this repository. Nothing unpublished (an npm or PyPI name) is advertised.
func TestWellKnownAIAgent_AdvertisesOnlyWhatExists(t *testing.T) {
	router := NewRouter(nil, nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/.well-known/ai-agent.json", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var discovery struct {
		API  map[string]string    `json:"api"`
		MCP  struct{ URL string } `json:"mcp"`
		CLI  map[string]string    `json:"cli"`
		SDKs map[string]string    `json:"sdks"`
	}
	if err := json.NewDecoder(w.Body).Decode(&discovery); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if discovery.MCP.URL != "https://api.solvr.dev/v1/mcp" {
		t.Errorf("mcp.url = %q, want the route that serves MCP over HTTP", discovery.MCP.URL)
	}
	if discovery.API["docs"] != "https://solvr.dev/api-docs" {
		t.Errorf("api.docs = %q, want the site's API reference", discovery.API["docs"])
	}
	want := map[string]map[string]string{
		"cli":  {"go": "github.com/fcavalcantirj/solvr/cli/cmd/solvr"},
		"sdks": {"go": "github.com/fcavalcantirj/solvr/packages/sdk-go"},
	}
	for section, got := range map[string]map[string]string{"cli": discovery.CLI, "sdks": discovery.SDKs} {
		if len(got) != len(want[section]) || got["go"] != want[section]["go"] {
			t.Errorf("%s = %v, want %v", section, got, want[section])
		}
	}

	// The Go paths are real: the CLI's main package and the SDK's module live in this repo.
	if _, err := os.Stat("../../../cli/cmd/solvr/main.go"); err != nil {
		t.Errorf("cli/cmd/solvr/main.go: %v", err)
	}
	mod, err := os.ReadFile("../../../packages/sdk-go/go.mod")
	if err != nil || !strings.HasPrefix(string(mod), "module github.com/fcavalcantirj/solvr/packages/sdk-go\n") {
		t.Errorf("packages/sdk-go/go.mod does not declare the advertised module: %v", err)
	}
}
