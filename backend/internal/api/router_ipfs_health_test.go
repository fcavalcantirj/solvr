package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/api/handlers"
)

// TestIPFSHealthEndpoint verifies GET /v1/health/ipfs returns IPFS health status.
// Without a real IPFS node, the endpoint should return 503 (service unavailable).
func TestIPFSHealthEndpoint(t *testing.T) {
	router := NewRouter(nil)

	req := httptest.NewRequest(http.MethodGet, "/v1/health/ipfs", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Without a running IPFS node at localhost:5001, expect 503
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status 503, got %d", w.Code)
	}

	var resp handlers.IPFSHealthResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Connected {
		t.Error("expected connected=false without running IPFS node")
	}
	if resp.Error == "" {
		t.Error("expected error message when IPFS node is not available")
	}
}

// TestIPFSHealthEndpoint_ResponseFormat verifies the response has required JSON fields.
func TestIPFSHealthEndpoint_ResponseFormat(t *testing.T) {
	router := NewRouter(nil)

	req := httptest.NewRequest(http.MethodGet, "/v1/health/ipfs", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var raw map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&raw); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Must have connected field
	if _, ok := raw["connected"]; !ok {
		t.Error("response missing 'connected' field")
	}
}
