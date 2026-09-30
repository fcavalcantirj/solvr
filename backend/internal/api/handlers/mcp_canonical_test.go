package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
)

// idx 52 step 2: the backend MCP server (/v1/mcp) is one of the two first-party MCP
// implementations. Like the npm mcp-server, it offers canonical posts with no type selection and
// a reply tool instead of the legacy answer/approach tool. Its write tools create nothing (the
// endpoint is unauthenticated); they must point to the canonical routes, never the retired ones.

// mcpRPC sends one JSON-RPC request to the handler and returns the decoded result object.
func mcpRPC(t *testing.T, method string, params map[string]interface{}) map[string]interface{} {
	t.Helper()
	body, _ := json.Marshal(map[string]interface{}{"jsonrpc": "2.0", "id": 1, "method": method, "params": params})
	req := httptest.NewRequest(http.MethodPost, "/v1/mcp", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	NewMCPHandler(nil, nil).Handle(rr, req)

	var resp struct {
		Result map[string]interface{} `json:"result"`
		Error  *rpcError              `json:"error"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode %s response: %v", method, err)
	}
	if resp.Error != nil {
		t.Fatalf("%s: unexpected JSON-RPC error %+v", method, resp.Error)
	}
	return resp.Result
}

// mcpToolText calls a tool and returns its text content and isError flag.
func mcpToolText(t *testing.T, name string, args map[string]interface{}) (string, bool) {
	t.Helper()
	result := mcpRPC(t, "tools/call", map[string]interface{}{"name": name, "arguments": args})
	content, _ := result["content"].([]interface{})
	if len(content) != 1 {
		t.Fatalf("%s: expected one content item, got %v", name, result["content"])
	}
	text, _ := content[0].(map[string]interface{})["text"].(string)
	isError, _ := result["isError"].(bool)
	return text, isError
}

// mcpToolSchemas returns each listed tool's input schema by name, and the names in list order.
func mcpToolSchemas(t *testing.T) (map[string]map[string]interface{}, []string) {
	t.Helper()
	tools, _ := mcpRPC(t, "tools/list", nil)["tools"].([]interface{})
	schemas := map[string]map[string]interface{}{}
	var names []string
	for _, raw := range tools {
		tool := raw.(map[string]interface{})
		name := tool["name"].(string)
		names = append(names, name)
		schemas[name] = tool["inputSchema"].(map[string]interface{})
	}
	return schemas, names
}

func schemaKeys(schema map[string]interface{}) []string {
	props, _ := schema["properties"].(map[string]interface{})
	keys := make([]string, 0, len(props))
	for k := range props {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func schemaRequired(schema map[string]interface{}) []string {
	raw, _ := schema["required"].([]interface{})
	out := make([]string, 0, len(raw))
	for _, r := range raw {
		out = append(out, r.(string))
	}
	sort.Strings(out)
	return out
}

func TestMCPHandler_ToolsAreSearchGetPostAndReply(t *testing.T) {
	_, names := mcpToolSchemas(t)
	want := []string{"solvr_search", "solvr_get", "solvr_post", "solvr_reply"}
	if strings.Join(names, ",") != strings.Join(want, ",") {
		t.Fatalf("tools = %v, want %v", names, want)
	}
	if strings.Join(MCPToolNames(), ",") != strings.Join(want, ",") {
		t.Fatalf("MCPToolNames() = %v, want the served list %v", MCPToolNames(), want)
	}
}

func TestMCPHandler_PostToolTakesNoType(t *testing.T) {
	schemas, _ := mcpToolSchemas(t)
	post := schemas["solvr_post"]
	if got := strings.Join(schemaKeys(post), ","); got != "description,tags,title" {
		t.Errorf("solvr_post properties = %s, want description,tags,title", got)
	}
	if got := strings.Join(schemaRequired(post), ","); got != "description,title" {
		t.Errorf("solvr_post required = %s, want description,title", got)
	}
}

func TestMCPHandler_GetToolHasNoIncludeOption(t *testing.T) {
	schemas, _ := mcpToolSchemas(t)
	if got := strings.Join(schemaKeys(schemas["solvr_get"]), ","); got != "id" {
		t.Errorf("solvr_get properties = %s, want id", got)
	}
}

func TestMCPHandler_ReplyToolSchema(t *testing.T) {
	schemas, _ := mcpToolSchemas(t)
	reply, ok := schemas["solvr_reply"]
	if !ok {
		t.Fatal("solvr_reply is not listed")
	}
	if got := strings.Join(schemaKeys(reply), ","); got != "body,parent_reply_id,post_id" {
		t.Errorf("solvr_reply properties = %s, want body,parent_reply_id,post_id", got)
	}
	if got := strings.Join(schemaRequired(reply), ","); got != "body,post_id" {
		t.Errorf("solvr_reply required = %s, want body,post_id", got)
	}
}

// Only the search filter (idx 53) may still name legacy post types; no write tool offers them.
func TestMCPHandler_NoWriteToolOffersALegacyTypeChoice(t *testing.T) {
	result := mcpRPC(t, "tools/list", nil)
	list, _ := json.Marshal(result)
	tools, _ := result["tools"].([]interface{})
	for _, tool := range tools {
		name := tool.(map[string]interface{})["name"].(string)
		if name == "solvr_search" {
			continue
		}
		raw, _ := json.Marshal(tool) // description and input schema
		for _, legacy := range []string{"problem", "question", "idea", "approach", "answer"} {
			if strings.Contains(string(raw), legacy) {
				t.Errorf("%s still mentions %q: %s", name, legacy, raw)
			}
		}
	}
	if strings.Contains(string(list), "solvr_answer") || strings.Contains(string(list), "approach_angle") {
		t.Errorf("tools/list still offers the legacy answer tool: %s", list)
	}
}

func TestMCPHandler_PostCallPointsToTheCanonicalRouteAndDropsAType(t *testing.T) {
	text, isError := mcpToolText(t, "solvr_post", map[string]interface{}{
		"type":        "problem",
		"title":       "Connection pool exhausted under load",
		"description": "Every request after the 20th waits for a free connection.",
	})
	if isError {
		t.Fatalf("solvr_post returned an error: %s", text)
	}
	for _, want := range []string{"POST /v1/posts", "Title: Connection pool exhausted under load",
		"Every request after the 20th waits"} {
		if !strings.Contains(text, want) {
			t.Errorf("solvr_post text lacks %q:\n%s", want, text)
		}
	}
	for _, gone := range []string{"Type:", "problem"} {
		if strings.Contains(text, gone) {
			t.Errorf("solvr_post text still carries the legacy type (%q):\n%s", gone, text)
		}
	}
}

func TestMCPHandler_ReplyCallPointsToTheCanonicalReplyRoute(t *testing.T) {
	text, isError := mcpToolText(t, "solvr_reply", map[string]interface{}{
		"post_id":         "post_123",
		"body":            "Raising max_conns to 50 fixed it.",
		"parent_reply_id": "reply_9",
	})
	if isError {
		t.Fatalf("solvr_reply returned an error: %s", text)
	}
	for _, want := range []string{"POST /v1/posts/post_123/replies", "In reply to: reply_9",
		"Raising max_conns to 50 fixed it."} {
		if !strings.Contains(text, want) {
			t.Errorf("solvr_reply text lacks %q:\n%s", want, text)
		}
	}

	top, _ := mcpToolText(t, "solvr_reply", map[string]interface{}{"post_id": "post_123", "body": "Top-level reply."})
	if strings.Contains(top, "In reply to") {
		t.Errorf("a top-level reply names a parent:\n%s", top)
	}
}

func TestMCPHandler_ReplyCallRequiresPostIDAndBody(t *testing.T) {
	for _, args := range []map[string]interface{}{{"body": "no post"}, {"post_id": "post_123"}} {
		text, isError := mcpToolText(t, "solvr_reply", args)
		if !isError {
			t.Errorf("solvr_reply %v: want isError, got %s", args, text)
		}
	}
}

// An agent configured against the old tool list gets an actionable error, never a success.
func TestMCPHandler_RetiredAnswerToolPointsToReply(t *testing.T) {
	text, isError := mcpToolText(t, "solvr_answer", map[string]interface{}{
		"post_id": "post_123", "content": "The answer", "approach_angle": "pooling",
	})
	if !isError {
		t.Fatalf("solvr_answer must be an error, got: %s", text)
	}
	for _, want := range []string{"solvr_answer was retired", "solvr_reply", "POST /v1/posts/{id}/replies"} {
		if !strings.Contains(text, want) {
			t.Errorf("retired solvr_answer text lacks %q:\n%s", want, text)
		}
	}
}
