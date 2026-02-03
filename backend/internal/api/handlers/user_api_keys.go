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

// CreateAPIKeyRequest represents the request body for creating an API key.
type CreateAPIKeyRequest struct {
	Name string `json:"name"`
}

// CreateAPIKeyResponse represents the response when a new API key is created.
// Contains the full key which is shown only once.
type CreateAPIKeyResponse struct {
	ID        string `json:"id"`
	UserID    string `json:"user_id"`
	Name      string `json:"name"`
	Key       string `json:"key"` // Full key, shown only once!
	CreatedAt string `json:"created_at"`
}

// CreateAPIKey handles POST /v1/users/me/api-keys
// Creates a new API key for the authenticated user.
// Per prd-v2.json: "Accept name for the key, Generate secure random key (solvr_sk_xxx),
// Return full key ONCE (never stored in plain text), Store hashed version"
func (h *UserAPIKeysHandler) CreateAPIKey(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Check for valid JWT authentication
	claims := auth.ClaimsFromContext(ctx)
	if claims == nil {
		writeAPIKeyUnauthorized(w, "UNAUTHORIZED", "Authentication required")
		return
	}

	// Parse request body
	var req CreateAPIKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIKeyValidationError(w, "Invalid request body")
		return
	}

	// Validate name
	if err := h.validateKeyName(req.Name); err != nil {
		writeAPIKeyValidationError(w, err.Error())
		return
	}

	// Generate a new API key with solvr_sk_ prefix (sk = secret key)
	plainKey := generateUserAPIKey()

	// Hash the key for storage (never store plain text)
	keyHash, err := auth.HashAPIKey(plainKey)
	if err != nil {
		writeAPIKeyInternalError(w, "Failed to create API key")
		return
	}

	// Create the key in database
	key := &models.UserAPIKey{
		UserID:  claims.UserID,
		Name:    req.Name,
		KeyHash: keyHash,
	}

	created, err := h.repo.Create(ctx, key)
	if err != nil {
		writeAPIKeyInternalError(w, "Failed to create API key")
		return
	}

	// Return the full key (only time it's shown)
	response := CreateAPIKeyResponse{
		ID:        created.ID,
		UserID:    created.UserID,
		Name:      created.Name,
		Key:       plainKey,
		CreatedAt: created.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}

	writeAPIKeyJSON(w, http.StatusCreated, response)
}

// validateKeyName validates the API key name.
func (h *UserAPIKeysHandler) validateKeyName(name string) error {
	if name == "" {
		return &validationError{"Name is required"}
	}
	if len(name) > 100 {
		return &validationError{"Name must be 100 characters or less"}
	}
	return nil
}

// validationError is a simple error type for validation errors.
type validationError struct {
	msg string
}

func (e *validationError) Error() string {
	return e.msg
}

// generateUserAPIKey creates a new API key with the solvr_sk_ prefix.
// Uses the auth package's key generation but with a different prefix for user keys.
func generateUserAPIKey() string {
	// Generate the base key (which has solvr_ prefix)
	baseKey := auth.GenerateAPIKey()
	// Replace solvr_ with solvr_sk_ for user secret keys
	return "solvr_sk_" + baseKey[6:] // Remove "solvr_" and add "solvr_sk_"
}

func writeAPIKeyValidationError(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]string{
			"code":    "VALIDATION_ERROR",
			"message": message,
		},
	})
}
