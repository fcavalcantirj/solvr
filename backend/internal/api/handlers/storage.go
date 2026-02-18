package handlers

import (
	"context"
	"encoding/json"
	"net/http"
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
	repo StorageRepositoryInterface
}

// NewStorageHandler creates a new StorageHandler.
func NewStorageHandler(repo StorageRepositoryInterface) *StorageHandler {
	return &StorageHandler{repo: repo}
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
