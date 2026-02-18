package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
)

// IPFSNodeInfo holds identity information returned by a kubo IPFS node.
type IPFSNodeInfo struct {
	PeerID          string `json:"peer_id"`
	AgentVersion    string `json:"agent_version"`
	ProtocolVersion string `json:"protocol_version"`
}

// IPFSHealthChecker is the interface for checking IPFS node health.
type IPFSHealthChecker interface {
	NodeInfo(ctx context.Context) (*IPFSNodeInfo, error)
}

// IPFSHealthResponse is the response for the IPFS health check endpoint.
type IPFSHealthResponse struct {
	Connected bool   `json:"connected"`
	PeerID    string `json:"peer_id"`
	Version   string `json:"version"`
	Error     string `json:"error,omitempty"`
}

// IPFSHealthHandler handles IPFS health check requests.
type IPFSHealthHandler struct {
	checker IPFSHealthChecker
}

// NewIPFSHealthHandler creates a new IPFSHealthHandler.
func NewIPFSHealthHandler(checker IPFSHealthChecker) *IPFSHealthHandler {
	return &IPFSHealthHandler{checker: checker}
}

// Check handles GET /v1/health/ipfs — returns IPFS node connectivity status.
func (h *IPFSHealthHandler) Check(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	info, err := h.checker.NodeInfo(ctx)
	if err != nil {
		errMsg := err.Error()
		if errors.Is(err, context.DeadlineExceeded) {
			errMsg = "timeout"
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(IPFSHealthResponse{
			Connected: false,
			Error:     errMsg,
		})
		return
	}

	if info == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(IPFSHealthResponse{
			Connected: false,
			Error:     "no node info returned",
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(IPFSHealthResponse{
		Connected: true,
		PeerID:    info.PeerID,
		Version:   info.AgentVersion,
	})
}
