// Package handlers contains HTTP request handlers for the Solvr API.
package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMCPHandler_Initialize(t *testing.T) {
	// Create handler with nil repos (we're not testing search/get)
	handler := NewMCPHandler(nil)

	// Create JSON-RPC request
	reqBody := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]interface{}{},
			"clientInfo": map[string]interface{}{
				"name":    "test-client",
				"version": "1.0.0",
			},
		},
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/v1/mcp", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.Handle(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	var resp jsonRPCResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.JSONRPC != "2.0" {
		t.Errorf("expected jsonrpc 2.0, got %s", resp.JSONRPC)
	}

	if resp.Error != nil {
		t.Errorf("unexpected error: %v", resp.Error)
	}

	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected result to be map, got %T", resp.Result)
	}

	// The MCP InitializeResult: protocolVersion, capabilities and serverInfo {name, version}.
	// A client built on the MCP SDK refuses an answer without serverInfo.
	if result["protocolVersion"] != "2024-11-05" {
		t.Errorf("expected protocolVersion 2024-11-05, got %v", result["protocolVersion"])
	}
	if capabilities, _ := result["capabilities"].(map[string]interface{}); capabilities == nil || capabilities["tools"] == nil {
		t.Errorf("expected capabilities.tools, got %v", result["capabilities"])
	}
	info, _ := result["serverInfo"].(map[string]interface{})
	if info == nil || info["name"] != "solvr" || info["version"] != MCPVersion {
		t.Errorf("expected serverInfo {name: solvr, version: %s}, got %v", MCPVersion, result["serverInfo"])
	}
	for _, flat := range []string{"name", "version"} {
		if _, ok := result[flat]; ok {
			t.Errorf("%s belongs inside serverInfo, not at the top of the result", flat)
		}
	}
}

// A JSON-RPC notification (a message without an id) is never answered: over HTTP the
// server accepts it with 202 and no body. MCP clients send notifications/initialized
// right after initialize.
func TestMCPHandler_NotificationsAreAcceptedWithoutAnAnswer(t *testing.T) {
	handler := NewMCPHandler(nil)
	for _, method := range []string{"notifications/initialized", "initialized", "notifications/cancelled", "unknown/notification"} {
		body := `{"jsonrpc":"2.0","method":"` + method + `"}`
		req := httptest.NewRequest(http.MethodPost, "/v1/mcp", bytes.NewReader([]byte(body)))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		handler.Handle(rr, req)

		if rr.Code != http.StatusAccepted {
			t.Errorf("%s: expected 202, got %d", method, rr.Code)
		}
		if rr.Body.Len() != 0 {
			t.Errorf("%s: expected no body, got %s", method, rr.Body.String())
		}
	}
}

func TestMCPHandler_ToolsList(t *testing.T) {
	handler := NewMCPHandler(nil)

	reqBody := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/list",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/v1/mcp", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.Handle(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	var resp jsonRPCResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Error != nil {
		t.Errorf("unexpected error: %v", resp.Error)
	}

	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected result to be map, got %T", resp.Result)
	}

	tools, ok := result["tools"].([]interface{})
	if !ok {
		t.Fatalf("expected tools to be array, got %T", result["tools"])
	}

	if len(tools) != 15 {
		t.Errorf("expected 15 tools, got %d", len(tools))
	}

	// Check tool names
	toolNames := make(map[string]bool)
	for _, tool := range tools {
		toolMap, ok := tool.(map[string]interface{})
		if !ok {
			continue
		}
		if name, ok := toolMap["name"].(string); ok {
			toolNames[name] = true
		}
	}

	expectedTools := []string{"solvr_search", "solvr_get", "solvr_post", "solvr_reply", "solvr_replies", "solvr_get_reply", "solvr_update_reply", "solvr_room_create", "solvr_room_join", "solvr_room_members", "solvr_room_add_member", "solvr_room_read", "solvr_room_send", "solvr_room_ticket", "solvr_room_watch"}
	for _, name := range expectedTools {
		if !toolNames[name] {
			t.Errorf("expected tool %s not found", name)
		}
	}
}

func TestMCPHandler_MethodNotAllowed(t *testing.T) {
	handler := NewMCPHandler(nil)

	req := httptest.NewRequest(http.MethodGet, "/v1/mcp", nil)
	rr := httptest.NewRecorder()

	handler.Handle(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200 (JSON-RPC error), got %d", rr.Code)
	}

	var resp jsonRPCResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Error == nil {
		t.Error("expected error response for GET method")
	}

	if resp.Error != nil && resp.Error.Code != -32600 {
		t.Errorf("expected error code -32600, got %d", resp.Error.Code)
	}
}

func TestMCPHandler_InvalidJSON(t *testing.T) {
	handler := NewMCPHandler(nil)

	req := httptest.NewRequest(http.MethodPost, "/v1/mcp", bytes.NewReader([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.Handle(rr, req)

	var resp jsonRPCResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Error == nil {
		t.Error("expected error response for invalid JSON")
	}

	if resp.Error != nil && resp.Error.Code != -32700 {
		t.Errorf("expected parse error code -32700, got %d", resp.Error.Code)
	}
}

func TestMCPHandler_UnknownMethod(t *testing.T) {
	handler := NewMCPHandler(nil)

	reqBody := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      3,
		"method":  "unknown/method",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/v1/mcp", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.Handle(rr, req)

	var resp jsonRPCResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Error == nil {
		t.Error("expected error response for unknown method")
	}

	if resp.Error != nil && resp.Error.Code != -32601 {
		t.Errorf("expected method not found code -32601, got %d", resp.Error.Code)
	}
}

func TestMCPHandler_Shutdown(t *testing.T) {
	handler := NewMCPHandler(nil)

	reqBody := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      4,
		"method":  "shutdown",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/v1/mcp", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.Handle(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	var resp jsonRPCResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Error != nil {
		t.Errorf("unexpected error: %v", resp.Error)
	}
}
