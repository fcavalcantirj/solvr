// Package handlers provides HTTP handlers for the Solvr API.
package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

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

// UsersAgentRepositoryInterface defines the agent repository operations needed by UsersHandler.
// Per prd-v4: GET /v1/users/{id}/agents endpoint to list agents by human_id.
type UsersAgentRepositoryInterface interface {
	FindByHumanID(ctx context.Context, humanID string) ([]*models.Agent, error)
}

// UsersUserListRepositoryInterface defines the user list repository operations.
// Per prd-v4: GET /v1/users endpoint to list all users with pagination.
type UsersUserListRepositoryInterface interface {
	List(ctx context.Context, opts models.PublicUserListOptions) ([]models.UserListItem, int, error)
}

// UsersHandler handles user profile endpoints.
// Per BE-003: User profile endpoints for viewing and editing profiles.
// Per prd-v4: GET /v1/users/{id}/agents to list agents by human_id.
// Per prd-v4: GET /v1/users to list all users.
// Per prd-v4: GET /v1/users/{id}/contributions to list user contributions.
type UsersHandler struct {
	userRepo       UsersUserRepositoryInterface
	postRepo       UsersPostRepositoryInterface
	agentRepo      UsersAgentRepositoryInterface
	userListRepo   UsersUserListRepositoryInterface
	answersRepo    ContribAnswersRepositoryInterface
	approachesRepo ContribApproachesRepositoryInterface
	responsesRepo  ContribResponsesRepositoryInterface
}

// NewUsersHandler creates a new UsersHandler instance.
func NewUsersHandler(userRepo UsersUserRepositoryInterface, postRepo UsersPostRepositoryInterface) *UsersHandler {
	return &UsersHandler{
		userRepo: userRepo,
		postRepo: postRepo,
	}
}

// SetAgentRepository sets the agent repository for listing user's agents.
// Per prd-v4: GET /v1/users/{id}/agents endpoint.
func (h *UsersHandler) SetAgentRepository(repo UsersAgentRepositoryInterface) {
	h.agentRepo = repo
}

// SetUserListRepository sets the user list repository for listing users.
// Per prd-v4: GET /v1/users endpoint.
func (h *UsersHandler) SetUserListRepository(repo UsersUserListRepositoryInterface) {
	h.userListRepo = repo
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

	// Validate UUID format to prevent DB errors (e.g. /v1/users/me matching {id})
	if _, err := uuid.Parse(userID); err != nil {
		writeUsersError(w, http.StatusBadRequest, "BAD_REQUEST", "invalid user ID format")
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

// UserAgentsResponse is the response for GET /v1/users/{id}/agents.
// Per prd-v4: Return agent list with basic info (exclude api_key_hash).
type UserAgentsResponse struct {
	Data []models.Agent `json:"data"`
	Meta struct {
		Total   int `json:"total"`
		Page    int `json:"page"`
		PerPage int `json:"per_page"`
	} `json:"meta"`
}

// UsersListResponse is the response for GET /v1/users.
// Per prd-v4: Return user list with public info, reputation, agents_count.
type UsersListResponse struct {
	Data []models.UserListItem `json:"data"`
	Meta struct {
		Total  int `json:"total"`
		Limit  int `json:"limit"`
		Offset int `json:"offset"`
	} `json:"meta"`
}

// GetUserAgents handles GET /v1/users/{id}/agents.
// Per prd-v4: List agents claimed by a user.
func (h *UsersHandler) GetUserAgents(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := chi.URLParam(r, "id")

	if userID == "" {
		writeUsersError(w, http.StatusBadRequest, "BAD_REQUEST", "user ID is required")
		return
	}

	// Validate UUID format to prevent DB errors
	if _, err := uuid.Parse(userID); err != nil {
		writeUsersError(w, http.StatusBadRequest, "BAD_REQUEST", "invalid user ID format")
		return
	}

	if h.agentRepo == nil {
		writeUsersError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "agent repository not configured")
		return
	}

	agents, err := h.agentRepo.FindByHumanID(ctx, userID)
	if err != nil {
		writeUsersError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to fetch agents")
		return
	}

	// Convert to response format (ensure api_key_hash is not exposed)
	responseAgents := make([]models.Agent, 0, len(agents))
	for _, agent := range agents {
		// Create a copy without api_key_hash
		responseAgent := *agent
		responseAgent.APIKeyHash = "" // Ensure not exposed
		responseAgents = append(responseAgents, responseAgent)
	}

	// Build response
	resp := UserAgentsResponse{}
	resp.Data = responseAgents
	resp.Meta.Total = len(responseAgents)
	resp.Meta.Page = 1
	resp.Meta.PerPage = len(responseAgents)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

// ListUsers handles GET /v1/users.
// Per prd-v4: List all users with pagination and sorting.
// Response includes: id, username, display_name, avatar_url, reputation, agents_count, created_at.
// Does NOT expose email or auth_provider_id.
func (h *UsersHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if h.userListRepo == nil {
		writeUsersError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "user list repository not configured")
		return
	}

	// Parse query params
	opts := models.PublicUserListOptions{
		Limit:  20, // default
		Offset: 0,
		Sort:   models.PublicUserSortNewest, // default
	}

	// Parse limit (default 20, max 100)
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if limit, err := parsePositiveInt(limitStr); err == nil {
			opts.Limit = limit
		}
	}
	if opts.Limit > 100 {
		opts.Limit = 100
	}
	if opts.Limit < 1 {
		opts.Limit = 20
	}

	// Parse offset
	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if offset, err := parsePositiveInt(offsetStr); err == nil {
			opts.Offset = offset
		}
	}

	// Parse sort (newest/reputation/agents)
	if sort := r.URL.Query().Get("sort"); sort != "" {
		switch sort {
		case models.PublicUserSortNewest, models.PublicUserSortReputation, models.PublicUserSortAgents:
			opts.Sort = sort
		}
	}

	users, total, err := h.userListRepo.List(ctx, opts)
	if err != nil {
		writeUsersError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to list users")
		return
	}

	// Ensure we return empty array, not null
	if users == nil {
		users = []models.UserListItem{}
	}

	// Build response
	resp := UsersListResponse{}
	resp.Data = users
	resp.Meta.Total = total
	resp.Meta.Limit = opts.Limit
	resp.Meta.Offset = opts.Offset

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

// parsePositiveInt parses a string as a positive integer.
func parsePositiveInt(s string) (int, error) {
	var n int
	_, err := fmt.Sscanf(s, "%d", &n)
	if err != nil || n < 0 {
		return 0, fmt.Errorf("invalid positive integer: %s", s)
	}
	return n, nil
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
	authInfo := GetAuthInfo(r)
	if authInfo == nil {
		writeUsersError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}

	// Parse pagination params (use existing function from posts.go)
	page, perPage, err := parsePaginationParams(r)
	if err != nil {
		writeUsersError(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}

	// List posts by author
	opts := models.PostListOptions{
		AuthorType: authInfo.AuthorType,
		AuthorID:   authInfo.AuthorID,
		Page:       page,
		PerPage:    perPage,
	}

	posts, total, err := h.postRepo.List(r.Context(), opts)
	if err != nil {
		writeUsersError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to list posts")
		return
	}

	writeUsersListJSON(w, http.StatusOK, posts, total, page, perPage)
}

// GetMyContributions handles GET /v1/me/contributions.
// Per prd-v4: Returns answers, approaches, and responses for the authenticated user.
// Uses the same logic as GetUserContributions but with the authenticated user's identity.
func (h *UsersHandler) GetMyContributions(w http.ResponseWriter, r *http.Request) {
	authInfo := GetAuthInfo(r)
	if authInfo == nil {
		writeUsersError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}

	// Parse pagination params
	page, perPage, err := parsePaginationParams(r)
	if err != nil {
		writeUsersError(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}

	// Parse type filter
	typeFilter := r.URL.Query().Get("type")

	items, total := h.fetchContributions(r.Context(), string(authInfo.AuthorType), authInfo.AuthorID, typeFilter, page, perPage)

	resp := ContributionsResponse{
		Data: items,
		Meta: ContributionsMeta{
			Total:   total,
			Page:    page,
			PerPage: perPage,
			HasMore: total > page*perPage,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
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
