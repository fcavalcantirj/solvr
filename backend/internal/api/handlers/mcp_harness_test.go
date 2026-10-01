package handlers

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	apimiddleware "github.com/fcavalcantirj/solvr/internal/api/middleware"
)

// The harness of the /v1/mcp tool tests: a dispatcher that records every request a tool sends
// to the API router and answers it, and a tools/call through the handler's own JSON-RPC entry.

// mcpSent is one request a tool dispatched to the API router.
type mcpSent struct {
	method, path, auth string
	query              map[string][]string
	header             http.Header
	body               string
	remoteAddr         string
	inProcess          bool
}

type mcpRecorder struct {
	mu    sync.Mutex
	sent  []mcpSent
	serve http.HandlerFunc
}

func (d *mcpRecorder) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	raw, _ := io.ReadAll(r.Body)
	d.mu.Lock()
	d.sent = append(d.sent, mcpSent{
		method: r.Method, path: r.URL.EscapedPath(), auth: r.Header.Get("Authorization"),
		query: r.URL.Query(), header: r.Header.Clone(), body: string(raw),
		remoteAddr: r.RemoteAddr, inProcess: apimiddleware.IsInProcessCall(r.Context()),
	})
	d.mu.Unlock()
	r.Body = io.NopCloser(bytes.NewReader(raw))
	d.serve(w, r)
}

func (d *mcpRecorder) requests() []mcpSent {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]mcpSent(nil), d.sent...)
}

// recordMCP returns an MCP handler whose tool calls reach serve, and the recorder.
func recordMCP(serve http.HandlerFunc) (*MCPHandler, *mcpRecorder) {
	rec := &mcpRecorder{serve: serve}
	return NewMCPHandler(rec), rec
}

// writeMCPJSON answers a dispatched request with a JSON body.
func writeMCPJSON(w http.ResponseWriter, status int, body interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

// callMCP calls a tool through POST /v1/mcp with the given Authorization header ("" = none)
// and returns its text and isError flag.
func callMCP(t *testing.T, h *MCPHandler, name string, args map[string]interface{}, auth string) (string, bool) {
	t.Helper()
	body, _ := json.Marshal(map[string]interface{}{"jsonrpc": "2.0", "id": 7, "method": "tools/call",
		"params": map[string]interface{}{"name": name, "arguments": args}})
	req := httptest.NewRequest(http.MethodPost, "/v1/mcp", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Request-ID", "mcp-call-1")
	req.RemoteAddr = "203.0.113.9:4567"
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	rr := httptest.NewRecorder()
	h.Handle(rr, req)

	var resp struct {
		Result struct {
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
			IsError bool `json:"isError"`
		} `json:"result"`
		Error *rpcError `json:"error"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("%s: decode the JSON-RPC answer: %v", name, err)
	}
	if resp.Error != nil {
		t.Fatalf("%s: JSON-RPC error %+v", name, resp.Error)
	}
	if len(resp.Result.Content) != 1 || resp.Result.Content[0].Type != "text" {
		t.Fatalf("%s: want one text content item, got %+v", name, resp.Result.Content)
	}
	return resp.Result.Content[0].Text, resp.Result.IsError
}
