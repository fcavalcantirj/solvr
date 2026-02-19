package handlers

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/fcavalcantirj/solvr/internal/auth"
)

// StorageRepositoryInterface defines operations for storage quota tracking.
type StorageRepositoryInterface interface {
	// GetStorageUsage returns the current storage usage and quota for an owner.
	GetStorageUsage(ctx context.Context, ownerID, ownerType string) (used int64, quota int64, err error)
	// UpdateStorageUsed adjusts the storage_used_bytes by deltaBytes (positive or negative).
	UpdateStorageUsed(ctx context.Context, ownerID, ownerType string, deltaBytes int64) error
}

// StorageResponse is the response for GET /v1/me/storage.
type StorageResponse struct {
	Used       int64   `json:"used"`
	Quota      int64   `json:"quota"`
	Percentage float64 `json:"percentage"`
}

// StorageHandler handles storage quota endpoints.
type StorageHandler struct {
	repo        StorageRepositoryInterface
	agentFinder AgentFinderInterface
}

// NewStorageHandler creates a new StorageHandler.
func NewStorageHandler(repo StorageRepositoryInterface) *StorageHandler {
	return &StorageHandler{repo: repo}
}

// SetAgentFinderRepo sets the agent finder for cross-entity storage access.
func (h *StorageHandler) SetAgentFinderRepo(repo AgentFinderInterface) {
	h.agentFinder = repo
}

// GetStorage handles GET /v1/me/storage — returns storage usage for authenticated user/agent.
func (h *StorageHandler) GetStorage(w http.ResponseWriter, r *http.Request) {
	authInfo := GetAuthInfo(r)
	if authInfo == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]string{
				"code":    "UNAUTHORIZED",
				"message": "authentication required",
			},
		})
		return
	}

	used, quota, err := h.repo.GetStorageUsage(r.Context(), authInfo.AuthorID, string(authInfo.AuthorType))
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]string{
				"code":    "INTERNAL_ERROR",
				"message": "failed to get storage usage",
			},
		})
		return
	}

	var percentage float64
	if quota > 0 {
		percentage = float64(used) / float64(quota) * 100.0
	}

	resp := StorageResponse{
		Used:       used,
		Quota:      quota,
		Percentage: percentage,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{"data": resp})
}

// GetAgentStorage handles GET /v1/agents/{id}/storage — returns storage usage for an agent.
// Accessible by the agent itself (API key) or by the human who claimed the agent (JWT).
func (h *StorageHandler) GetAgentStorage(w http.ResponseWriter, r *http.Request, agentID string) {
	ctx := r.Context()

	// Check agent API key auth first (agent accessing own storage)
	authAgent := auth.AgentFromContext(ctx)
	if authAgent != nil {
		if authAgent.ID != agentID {
			writeStorageError(w, http.StatusForbidden, "FORBIDDEN", "agents can only access their own storage")
			return
		}
	} else {
		// Check human JWT auth
		claims := auth.ClaimsFromContext(ctx)
		if claims == nil {
			writeStorageError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
			return
		}

		// Look up the agent
		agent, err := h.agentFinder.FindByID(ctx, agentID)
		if err != nil {
			writeStorageError(w, http.StatusNotFound, "NOT_FOUND", "agent not found")
			return
		}

		// Verify the human is the owner
		if agent.HumanID == nil || *agent.HumanID != claims.UserID {
			writeStorageError(w, http.StatusForbidden, "FORBIDDEN", "you must be the claiming owner of this agent")
			return
		}
	}

	used, quota, err := h.repo.GetStorageUsage(ctx, agentID, "agent")
	if err != nil {
		writeStorageError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get storage usage")
		return
	}

	var percentage float64
	if quota > 0 {
		percentage = float64(used) / float64(quota) * 100.0
	}

	resp := StorageResponse{
		Used:       used,
		Quota:      quota,
		Percentage: percentage,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{"data": resp})
}

func writeStorageError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]string{
			"code":    code,
			"message": message,
		},
	})
}
