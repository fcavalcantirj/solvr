package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

// --- Mock IPFSHealthChecker ---

type MockIPFSHealthChecker struct {
	nodeInfo *IPFSNodeInfo
	err      error
}

func (m *MockIPFSHealthChecker) NodeInfo(ctx context.Context) (*IPFSNodeInfo, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.nodeInfo, nil
}

// --- Tests ---

func TestIPFSHealthHandler_Healthy(t *testing.T) {
	mock := &MockIPFSHealthChecker{
		nodeInfo: &IPFSNodeInfo{
			PeerID:        "12D3KooWJG6rZ1KWTQy1fPeaZuxhfukik3RmYTjyf76Yn6CwUP3A",
			AgentVersion:  "kubo/0.39.0/",
			ProtocolVersion: "ipfs/0.1.0",
		},
	}

	handler := NewIPFSHealthHandler(mock)

	req := httptest.NewRequest(http.MethodGet, "/v1/health/ipfs", nil)
	rr := httptest.NewRecorder()

	handler.Check(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	var resp IPFSHealthResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !resp.Connected {
		t.Error("expected connected=true")
	}
	if resp.PeerID != "12D3KooWJG6rZ1KWTQy1fPeaZuxhfukik3RmYTjyf76Yn6CwUP3A" {
		t.Errorf("expected peer ID '12D3KooWJG6rZ1KWTQy1fPeaZuxhfukik3RmYTjyf76Yn6CwUP3A', got '%s'", resp.PeerID)
	}
	if resp.Version != "kubo/0.39.0/" {
		t.Errorf("expected version 'kubo/0.39.0/', got '%s'", resp.Version)
	}
	if resp.Error != "" {
		t.Errorf("expected no error, got '%s'", resp.Error)
	}
}

func TestIPFSHealthHandler_Unhealthy(t *testing.T) {
	mock := &MockIPFSHealthChecker{
		err: errors.New("connection refused"),
	}

	handler := NewIPFSHealthHandler(mock)

	req := httptest.NewRequest(http.MethodGet, "/v1/health/ipfs", nil)
	rr := httptest.NewRecorder()

	handler.Check(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status 503, got %d", rr.Code)
	}

	var resp IPFSHealthResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Connected {
		t.Error("expected connected=false")
	}
	if resp.PeerID != "" {
		t.Errorf("expected empty peer ID, got '%s'", resp.PeerID)
	}
	if resp.Error != "connection refused" {
		t.Errorf("expected error 'connection refused', got '%s'", resp.Error)
	}
}

func TestIPFSHealthHandler_Timeout(t *testing.T) {
	mock := &MockIPFSHealthChecker{
		err: context.DeadlineExceeded,
	}

	handler := NewIPFSHealthHandler(mock)

	req := httptest.NewRequest(http.MethodGet, "/v1/health/ipfs", nil)
	rr := httptest.NewRecorder()

	handler.Check(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status 503, got %d", rr.Code)
	}

	var resp IPFSHealthResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Connected {
		t.Error("expected connected=false")
	}
	if resp.Error != "timeout" {
		t.Errorf("expected error 'timeout', got '%s'", resp.Error)
	}
}

func TestIPFSHealthHandler_NilNodeInfo(t *testing.T) {
	mock := &MockIPFSHealthChecker{
		nodeInfo: nil,
		err:      nil,
	}

	handler := NewIPFSHealthHandler(mock)

	req := httptest.NewRequest(http.MethodGet, "/v1/health/ipfs", nil)
	rr := httptest.NewRecorder()

	handler.Check(rr, req)

	// nil nodeInfo with no error is unexpected — should report not connected
	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status 503, got %d", rr.Code)
	}

	var resp IPFSHealthResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Connected {
		t.Error("expected connected=false")
	}
}

func TestIPFSHealthHandler_ResponseFormat(t *testing.T) {
	mock := &MockIPFSHealthChecker{
		nodeInfo: &IPFSNodeInfo{
			PeerID:          "12D3KooWTest",
			AgentVersion:    "kubo/0.39.0/",
			ProtocolVersion: "ipfs/0.1.0",
		},
	}

	handler := NewIPFSHealthHandler(mock)

	req := httptest.NewRequest(http.MethodGet, "/v1/health/ipfs", nil)
	rr := httptest.NewRecorder()

	handler.Check(rr, req)

	// Verify JSON contains expected fields
	var raw map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&raw); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Must have connected field
	if _, ok := raw["connected"]; !ok {
		t.Error("response missing 'connected' field")
	}
	// Must have peer_id field
	if _, ok := raw["peer_id"]; !ok {
		t.Error("response missing 'peer_id' field")
	}
	// Must have version field
	if _, ok := raw["version"]; !ok {
		t.Error("response missing 'version' field")
	}
}

func TestIPFSHealthHandler_ContextCancelled(t *testing.T) {
	mock := &MockIPFSHealthChecker{
		err: context.Canceled,
	}

	handler := NewIPFSHealthHandler(mock)

	req := httptest.NewRequest(http.MethodGet, "/v1/health/ipfs", nil)
	rr := httptest.NewRecorder()

	handler.Check(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status 503, got %d", rr.Code)
	}

	var resp IPFSHealthResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Connected {
		t.Error("expected connected=false")
	}
}
