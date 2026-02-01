// Package handlers provides HTTP handlers for the Solvr API.
package handlers

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/fcavalcantirj/solvr/internal/auth"
	"github.com/fcavalcantirj/solvr/internal/models"
)

// MeUserRepositoryInterface defines the interface for user repository operations
// needed by the Me handler.
type MeUserRepositoryInterface interface {
	FindByID(ctx context.Context, id string) (*models.User, error)
	GetUserStats(ctx context.Context, userID string) (*models.UserStats, error)
}

// MeHandler handles the GET /v1/auth/me endpoint.
type MeHandler struct {
	config   *OAuthConfig
	userRepo MeUserRepositoryInterface
}

// NewMeHandler creates a new MeHandler instance.
func NewMeHandler(config *OAuthConfig, userRepo MeUserRepositoryInterface) *MeHandler {
	return &MeHandler{
		config:   config,
		userRepo: userRepo,
	}
}

// MeResponse represents the response for GET /v1/auth/me.
// Per SPEC.md Part 5.2: GET /auth/me -> Current user info.
type MeResponse struct {
	ID          string            `json:"id"`
	Username    string            `json:"username"`
	DisplayName string            `json:"display_name"`
	Email       string            `json:"email"`
	AvatarURL   string            `json:"avatar_url,omitempty"`
	Bio         string            `json:"bio,omitempty"`
	Role        string            `json:"role"`
	Stats       models.UserStats  `json:"stats"`
}

// Me handles GET /v1/auth/me
// Requires valid JWT, returns current user profile with stats.
// Per SPEC.md Part 5.2: GET /auth/me -> Current user info.
func (h *MeHandler) Me(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Check for valid JWT authentication (claims should be set by middleware)
	claims := auth.ClaimsFromContext(ctx)
	if claims == nil {
		writeMeUnauthorized(w, "UNAUTHORIZED", "Authentication required")
		return
	}

	// Look up user by ID from claims
	user, err := h.userRepo.FindByID(ctx, claims.UserID)
	if err != nil {
		writeMeInternalError(w, "Failed to fetch user")
		return
	}

	// User not found
	if user == nil {
		writeMeNotFound(w, "User not found")
		return
	}

	// Get user stats
	stats, err := h.userRepo.GetUserStats(ctx, claims.UserID)
	if err != nil {
		// Log error but continue with empty stats
		stats = &models.UserStats{}
	}

	// Build response
	response := MeResponse{
		ID:          user.ID,
		Username:    user.Username,
		DisplayName: user.DisplayName,
		Email:       user.Email,
		AvatarURL:   user.AvatarURL,
		Bio:         user.Bio,
		Role:        user.Role,
		Stats:       *stats,
	}

	writeMeJSON(w, http.StatusOK, response)
}

// Helper functions for writing responses

func writeMeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{"data": data})
}

func writeMeUnauthorized(w http.ResponseWriter, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]string{
			"code":    code,
			"message": message,
		},
	})
}

func writeMeNotFound(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotFound)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]string{
			"code":    "NOT_FOUND",
			"message": message,
		},
	})
}

func writeMeInternalError(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusInternalServerError)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]string{
			"code":    "INTERNAL_ERROR",
			"message": message,
		},
	})
}
