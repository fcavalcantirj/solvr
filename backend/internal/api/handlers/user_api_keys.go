// Package handlers provides HTTP handlers for the Solvr API.
package handlers

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/fcavalcantirj/solvr/internal/auth"
	"github.com/fcavalcantirj/solvr/internal/models"
)

// UserAPIKeyRepositoryInterface defines the interface for user API key repository operations.
type UserAPIKeyRepositoryInterface interface {
	Create(ctx context.Context, key *models.UserAPIKey) (*models.UserAPIKey, error)
	FindByUserID(ctx context.Context, userID string) ([]*models.UserAPIKey, error)
	FindByID(ctx context.Context, id string) (*models.UserAPIKey, error)
	Revoke(ctx context.Context, id, userID string) error
	UpdateLastUsed(ctx context.Context, id string) error
}

// UserAPIKeysHandler handles user API key management endpoints.
// Per prd-v2.json API-KEYS requirements.
type UserAPIKeysHandler struct {
	repo UserAPIKeyRepositoryInterface
}

// NewUserAPIKeysHandler creates a new UserAPIKeysHandler instance.
func NewUserAPIKeysHandler(repo UserAPIKeyRepositoryInterface) *UserAPIKeysHandler {
	return &UserAPIKeysHandler{
		repo: repo,
	}
}

// APIKeyResponse represents a single API key in the list response.
// Key value is masked - only shows preview.
type APIKeyResponse struct {
	ID         string  `json:"id"`
	UserID     string  `json:"user_id"`
	Name       string  `json:"name"`
	KeyPreview string  `json:"key_preview"`
	LastUsedAt *string `json:"last_used_at,omitempty"`
	CreatedAt  string  `json:"created_at"`
	UpdatedAt  string  `json:"updated_at"`
}

// ListAPIKeys handles GET /v1/users/me/api-keys
// Returns all active API keys for the authenticated user.
// Keys are masked - only showing a preview, not the actual key.
// Per prd-v2.json: "Return all API keys for authenticated user, Include name, created_at, last_used_at, Mask actual key value (show prefix only)"
func (h *UserAPIKeysHandler) ListAPIKeys(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Check for valid JWT authentication
	claims := auth.ClaimsFromContext(ctx)
	if claims == nil {
		writeAPIKeyUnauthorized(w, "UNAUTHORIZED", "Authentication required")
		return
	}

	// Get all active keys for the user
	keys, err := h.repo.FindByUserID(ctx, claims.UserID)
	if err != nil {
		writeAPIKeyInternalError(w, "Failed to fetch API keys")
		return
	}

	// Convert to response format with masked keys
	response := make([]APIKeyResponse, len(keys))
	for i, key := range keys {
		response[i] = h.toAPIKeyResponse(key)
	}

	writeAPIKeyJSON(w, http.StatusOK, response)
}

// toAPIKeyResponse converts a UserAPIKey to the API response format.
// The actual key hash is never exposed - only a preview placeholder.
func (h *UserAPIKeysHandler) toAPIKeyResponse(key *models.UserAPIKey) APIKeyResponse {
	resp := APIKeyResponse{
		ID:         key.ID,
		UserID:     key.UserID,
		Name:       key.Name,
		KeyPreview: "solvr_...****", // Keys are hashed, we can't show actual value
		CreatedAt:  key.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:  key.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}

	if key.LastUsedAt != nil {
		formatted := key.LastUsedAt.Format("2006-01-02T15:04:05Z07:00")
		resp.LastUsedAt = &formatted
	}

	return resp
}

// Helper functions for writing responses

func writeAPIKeyJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{"data": data})
}

func writeAPIKeyUnauthorized(w http.ResponseWriter, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]string{
			"code":    code,
			"message": message,
		},
	})
}

func writeAPIKeyInternalError(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusInternalServerError)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]string{
			"code":    "INTERNAL_ERROR",
			"message": message,
		},
	})
}
