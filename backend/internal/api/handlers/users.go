// Package handlers provides HTTP handlers for the Solvr API.
package handlers

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/fcavalcantirj/solvr/internal/auth"
	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
)

// UsersUserRepositoryInterface defines the user repository operations needed by UsersHandler.
type UsersUserRepositoryInterface interface {
	FindByID(ctx context.Context, id string) (*models.User, error)
	Update(ctx context.Context, user *models.User) (*models.User, error)
	GetUserStats(ctx context.Context, userID string) (*models.UserStats, error)
}

// UsersPostRepositoryInterface defines the post repository operations needed by UsersHandler.
type UsersPostRepositoryInterface interface {
	List(ctx context.Context, opts models.PostListOptions) ([]models.PostWithAuthor, int, error)
}

// UsersHandler handles user profile endpoints.
// Per BE-003: User profile endpoints for viewing and editing profiles.
type UsersHandler struct {
	userRepo UsersUserRepositoryInterface
	postRepo UsersPostRepositoryInterface
}

// NewUsersHandler creates a new UsersHandler instance.
func NewUsersHandler(userRepo UsersUserRepositoryInterface, postRepo UsersPostRepositoryInterface) *UsersHandler {
	return &UsersHandler{
		userRepo: userRepo,
		postRepo: postRepo,
	}
}

// PublicUserProfileResponse is the response for GET /v1/users/:id.
// Per BE-003: Public profile view (display_name, avatar, stats).
type PublicUserProfileResponse struct {
	ID          string           `json:"id"`
	Username    string           `json:"username"`
	DisplayName string           `json:"display_name"`
	AvatarURL   string           `json:"avatar_url,omitempty"`
	Bio         string           `json:"bio,omitempty"`
	Stats       models.UserStats `json:"stats"`
}

// UpdateProfileRequest is the request body for PATCH /v1/me.
type UpdateProfileRequest struct {
	DisplayName string `json:"display_name,omitempty"`
	Bio         string `json:"bio,omitempty"`
	AvatarURL   string `json:"avatar_url,omitempty"`
}

// GetUserProfile handles GET /v1/users/:id.
// Per BE-003: Public profile view - anyone can view any user's public profile.
func (h *UsersHandler) GetUserProfile(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := chi.URLParam(r, "id")

	if userID == "" {
		writeUsersError(w, http.StatusBadRequest, "BAD_REQUEST", "user ID is required")
		return
	}

	user, err := h.userRepo.FindByID(ctx, userID)
	if err != nil {
		if err == db.ErrNotFound {
			writeUsersError(w, http.StatusNotFound, "NOT_FOUND", "user not found")
			return
		}
		writeUsersError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to fetch user")
		return
	}

	if user == nil {
		writeUsersError(w, http.StatusNotFound, "NOT_FOUND", "user not found")
		return
	}

	// Get user stats
	stats, err := h.userRepo.GetUserStats(ctx, userID)
	if err != nil {
		// Continue with empty stats on error
		stats = &models.UserStats{}
	}

	response := PublicUserProfileResponse{
		ID:          user.ID,
		Username:    user.Username,
		DisplayName: user.DisplayName,
		AvatarURL:   user.AvatarURL,
		Bio:         user.Bio,
		Stats:       *stats,
	}

	writeUsersJSON(w, http.StatusOK, response)
}

// UpdateProfile handles PATCH /v1/me.
// Per BE-003: Update own profile (display_name, bio).
// Only authenticated humans can update their profile.
func (h *UsersHandler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Check for agent authentication - agents cannot update profile
	agent := auth.AgentFromContext(ctx)
	if agent != nil {
		writeUsersError(w, http.StatusForbidden, "FORBIDDEN", "agents cannot update user profile")
		return
	}

	// Check for user authentication (JWT)
	claims := auth.ClaimsFromContext(ctx)
	if claims == nil {
		writeUsersError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}

	// Parse request body
	var req UpdateProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeUsersError(w, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
		return
	}

	// Get current user
	user, err := h.userRepo.FindByID(ctx, claims.UserID)
	if err != nil {
		if err == db.ErrNotFound {
			writeUsersError(w, http.StatusNotFound, "NOT_FOUND", "user not found")
			return
		}
		writeUsersError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to fetch user")
		return
	}

	if user == nil {
		writeUsersError(w, http.StatusNotFound, "NOT_FOUND", "user not found")
		return
	}

	// Update only provided fields
	if req.DisplayName != "" {
		user.DisplayName = req.DisplayName
	}
	if req.Bio != "" {
		user.Bio = req.Bio
	}
	if req.AvatarURL != "" {
		user.AvatarURL = req.AvatarURL
	}

	// Save updates
	updated, err := h.userRepo.Update(ctx, user)
	if err != nil {
		writeUsersError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to update profile")
		return
	}

	// Get updated stats
	stats, _ := h.userRepo.GetUserStats(ctx, claims.UserID)
	if stats == nil {
		stats = &models.UserStats{}
	}

	response := PublicUserProfileResponse{
		ID:          updated.ID,
		Username:    updated.Username,
		DisplayName: updated.DisplayName,
		AvatarURL:   updated.AvatarURL,
		Bio:         updated.Bio,
		Stats:       *stats,
	}

	writeUsersJSON(w, http.StatusOK, response)
}

// GetMyPosts handles GET /v1/me/posts.
// Per BE-003: List own posts.
func (h *UsersHandler) GetMyPosts(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Check for agent authentication
	agent := auth.AgentFromContext(ctx)
	var authorType models.AuthorType
	var authorID string

	if agent != nil {
		authorType = models.AuthorTypeAgent
		authorID = agent.ID
	} else {
		// Check for user authentication (JWT)
		claims := auth.ClaimsFromContext(ctx)
		if claims == nil {
			writeUsersError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
			return
		}
		authorType = models.AuthorTypeHuman
		authorID = claims.UserID
	}

	// Parse pagination params (use existing function from posts.go)
	page, perPage, err := parsePaginationParams(r)
	if err != nil {
		writeUsersError(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}

	// List posts by author
	opts := models.PostListOptions{
		AuthorType: authorType,
		AuthorID:   authorID,
		Page:       page,
		PerPage:    perPage,
	}

	posts, total, err := h.postRepo.List(ctx, opts)
	if err != nil {
		writeUsersError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to list posts")
		return
	}

	writeUsersListJSON(w, http.StatusOK, posts, total, page, perPage)
}

// GetMyContributions handles GET /v1/me/contributions.
// Per BE-003: List own answers/approaches/responses.
// For now, this returns posts (later can add answers, approaches, responses).
func (h *UsersHandler) GetMyContributions(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Check for agent authentication
	agent := auth.AgentFromContext(ctx)
	var authorType models.AuthorType
	var authorID string

	if agent != nil {
		authorType = models.AuthorTypeAgent
		authorID = agent.ID
	} else {
		// Check for user authentication (JWT)
		claims := auth.ClaimsFromContext(ctx)
		if claims == nil {
			writeUsersError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
			return
		}
		authorType = models.AuthorTypeHuman
		authorID = claims.UserID
	}

	// Parse pagination params (use existing function from posts.go)
	page, perPage, err := parsePaginationParams(r)
	if err != nil {
		writeUsersError(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}

	// For now, return posts as contributions (can extend to answers/approaches later)
	opts := models.PostListOptions{
		AuthorType: authorType,
		AuthorID:   authorID,
		Page:       page,
		PerPage:    perPage,
	}

	posts, total, err := h.postRepo.List(ctx, opts)
	if err != nil {
		writeUsersError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to list contributions")
		return
	}

	writeUsersListJSON(w, http.StatusOK, posts, total, page, perPage)
}

// Helper functions

func writeUsersJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{"data": data})
}

func writeUsersListJSON(w http.ResponseWriter, status int, data interface{}, total, page, perPage int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"data": data,
		"meta": map[string]int{
			"total":    total,
			"page":     page,
			"per_page": perPage,
		},
	})
}

func writeUsersError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]string{
			"code":    code,
			"message": message,
		},
	})
}
