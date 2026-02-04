// Package handlers contains HTTP request handlers for the Solvr API.
package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/fcavalcantirj/solvr/internal/auth"
	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/go-chi/chi/v5"
)

// PostsRepositoryInterface defines the database operations for posts.
type PostsRepositoryInterface interface {
	// List returns posts matching the given options.
	List(ctx context.Context, opts models.PostListOptions) ([]models.PostWithAuthor, int, error)

	// FindByID returns a single post by ID.
	FindByID(ctx context.Context, id string) (*models.PostWithAuthor, error)

	// Create creates a new post and returns it.
	Create(ctx context.Context, post *models.Post) (*models.Post, error)

	// Update updates an existing post and returns it.
	Update(ctx context.Context, post *models.Post) (*models.Post, error)

	// Delete soft-deletes a post by ID.
	Delete(ctx context.Context, id string) error

	// Vote records a vote on a post.
	Vote(ctx context.Context, postID, voterType, voterID, direction string) error
}

// PostsHandler handles post-related HTTP requests.
type PostsHandler struct {
	repo PostsRepositoryInterface
}

// NewPostsHandler creates a new PostsHandler.
func NewPostsHandler(repo PostsRepositoryInterface) *PostsHandler {
	return &PostsHandler{repo: repo}
}

// CreatePostRequest is the request body for creating a post.
type CreatePostRequest struct {
	Type            string   `json:"type"`
	Title           string   `json:"title"`
	Description     string   `json:"description"`
	Tags            []string `json:"tags,omitempty"`
	SuccessCriteria []string `json:"success_criteria,omitempty"` // For problems
	Weight          *int     `json:"weight,omitempty"`           // For problems
}

// UpdatePostRequest is the request body for updating a post.
type UpdatePostRequest struct {
	Title       *string  `json:"title,omitempty"`
	Description *string  `json:"description,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	Status      *string  `json:"status,omitempty"`
}

// VoteRequest is the request body for voting.
type VoteRequest struct {
	Direction string `json:"direction"` // "up" or "down"
}

// PostsListResponse is the response for listing posts.
type PostsListResponse struct {
	Data []models.PostWithAuthor `json:"data"`
	Meta PostsListMeta           `json:"meta"`
}

// PostsListMeta contains metadata for list responses.
type PostsListMeta struct {
	Total   int  `json:"total"`
	Page    int  `json:"page"`
	PerPage int  `json:"per_page"`
	HasMore bool `json:"has_more"`
}

// PostResponse is the response for a single post.
type PostResponse struct {
	Data models.PostWithAuthor `json:"data"`
}

// authInfo holds authentication information from either JWT claims or API key.
// Per SPEC.md Part 1.4: Both humans and AI agents can perform all actions.
type authInfo struct {
	authorType models.AuthorType
	authorID   string
	role       string // Only for humans (JWT), empty for agents
}

// getAuthInfo extracts authentication information from the request context.
// Supports both JWT authentication (humans) and API key authentication (agents).
// Returns nil if not authenticated.
func getAuthInfo(r *http.Request) *authInfo {
	// First try JWT claims (human authentication)
	claims := auth.ClaimsFromContext(r.Context())
	if claims != nil {
		return &authInfo{
			authorType: models.AuthorTypeHuman,
			authorID:   claims.UserID,
			role:       claims.Role,
		}
	}

	// Then try agent authentication (API key)
	agent := auth.AgentFromContext(r.Context())
	if agent != nil {
		return &authInfo{
			authorType: models.AuthorTypeAgent,
			authorID:   agent.ID,
			role:       "", // Agents don't have roles (yet)
		}
	}

	return nil
}

// List handles GET /v1/posts - list posts.
func (h *PostsHandler) List(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters
	opts := models.PostListOptions{
		Page:    parseIntParam(r.URL.Query().Get("page"), 1),
		PerPage: parseIntParam(r.URL.Query().Get("per_page"), 20),
	}

	if opts.Page < 1 {
		opts.Page = 1
	}
	if opts.PerPage < 1 {
		opts.PerPage = 20
	}
	if opts.PerPage > 50 {
		opts.PerPage = 50 // Cap at 50 per SPEC.md
	}

	// Parse type filter
	if typeParam := r.URL.Query().Get("type"); typeParam != "" {
		opts.Type = models.PostType(typeParam)
	}

	// Parse status filter
	if statusParam := r.URL.Query().Get("status"); statusParam != "" {
		opts.Status = models.PostStatus(statusParam)
	}

	// Parse tags filter
	if tagsParam := r.URL.Query().Get("tags"); tagsParam != "" {
		opts.Tags = strings.Split(tagsParam, ",")
		for i, tag := range opts.Tags {
			opts.Tags[i] = strings.TrimSpace(tag)
		}
	}

	// Execute query
	posts, total, err := h.repo.List(r.Context(), opts)
	if err != nil {
		writePostsError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to list posts")
		return
	}

	// Calculate has_more
	hasMore := (opts.Page * opts.PerPage) < total

	response := PostsListResponse{
		Data: posts,
		Meta: PostsListMeta{
			Total:   total,
			Page:    opts.Page,
			PerPage: opts.PerPage,
			HasMore: hasMore,
		},
	}

	writePostsJSON(w, http.StatusOK, response)
}

// Get handles GET /v1/posts/:id - get a single post.
func (h *PostsHandler) Get(w http.ResponseWriter, r *http.Request) {
	postID := chi.URLParam(r, "id")
	if postID == "" {
		writePostsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "post ID is required")
		return
	}

	post, err := h.repo.FindByID(r.Context(), postID)
	if err != nil {
		if errors.Is(err, db.ErrPostNotFound) {
			writePostsError(w, http.StatusNotFound, "NOT_FOUND", "post not found")
			return
		}
		writePostsError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get post")
		return
	}

	// Check if deleted
	if post.DeletedAt != nil {
		writePostsError(w, http.StatusNotFound, "NOT_FOUND", "post not found")
		return
	}

	writePostsJSON(w, http.StatusOK, PostResponse{Data: *post})
}

// Create handles POST /v1/posts - create a new post.
// Per SPEC.md Part 1.4 and FIX-003: Both humans (JWT) and AI agents (API key) can create posts.
func (h *PostsHandler) Create(w http.ResponseWriter, r *http.Request) {
	// Require authentication (JWT or API key)
	authInfo := getAuthInfo(r)
	if authInfo == nil {
		writePostsError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}

	// Parse request body
	var req CreatePostRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writePostsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid JSON body")
		return
	}

	// Validate type
	postType := models.PostType(req.Type)
	if !models.IsValidPostType(postType) {
		writePostsError(w, http.StatusBadRequest, "INVALID_TYPE", "type must be one of: problem, question, idea")
		return
	}

	// Validate title
	if req.Title == "" {
		writePostsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "title is required")
		return
	}
	if len(req.Title) < 10 {
		writePostsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "title must be at least 10 characters")
		return
	}
	if len(req.Title) > 200 {
		writePostsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "title must be at most 200 characters")
		return
	}

	// Validate description
	if req.Description == "" {
		writePostsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "description is required")
		return
	}
	if len(req.Description) < 50 {
		writePostsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "description must be at least 50 characters")
		return
	}

	// Validate tags (max 5)
	if len(req.Tags) > 5 {
		writePostsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "maximum 5 tags allowed")
		return
	}

	// Validate problem-specific fields
	if postType == models.PostTypeProblem {
		if req.Weight != nil && (*req.Weight < 1 || *req.Weight > 5) {
			writePostsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "weight must be between 1 and 5")
			return
		}
		if len(req.SuccessCriteria) > 10 {
			writePostsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "maximum 10 success criteria allowed")
			return
		}
	}

	// Create post with author info from authentication
	post := &models.Post{
		Type:            postType,
		Title:           req.Title,
		Description:     req.Description,
		Tags:            req.Tags,
		PostedByType:    authInfo.authorType,
		PostedByID:      authInfo.authorID,
		Status:          models.PostStatusOpen,
		SuccessCriteria: req.SuccessCriteria,
		Weight:          req.Weight,
	}

	createdPost, err := h.repo.Create(r.Context(), post)
	if err != nil {
		writePostsError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to create post")
		return
	}

	writePostsJSON(w, http.StatusCreated, map[string]interface{}{
		"data": createdPost,
	})
}

// Update handles PATCH /v1/posts/:id - update a post.
// Per SPEC.md Part 15.2 and FIX-003: Users can edit their own content (humans and agents).
func (h *PostsHandler) Update(w http.ResponseWriter, r *http.Request) {
	// Require authentication (JWT or API key)
	authInfo := getAuthInfo(r)
	if authInfo == nil {
		writePostsError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}

	postID := chi.URLParam(r, "id")
	if postID == "" {
		writePostsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "post ID is required")
		return
	}

	// Get existing post
	existingPost, err := h.repo.FindByID(r.Context(), postID)
	if err != nil {
		if errors.Is(err, db.ErrPostNotFound) {
			writePostsError(w, http.StatusNotFound, "NOT_FOUND", "post not found")
			return
		}
		writePostsError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get post")
		return
	}

	// Check ownership - only owner can update (works for both humans and agents)
	isOwner := existingPost.PostedByType == authInfo.authorType && existingPost.PostedByID == authInfo.authorID
	if !isOwner {
		writePostsError(w, http.StatusForbidden, "FORBIDDEN", "you can only update your own posts")
		return
	}

	// Parse request body
	var req UpdatePostRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writePostsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid JSON body")
		return
	}

	// Apply updates
	updatedPost := existingPost.Post

	if req.Title != nil {
		if len(*req.Title) < 10 {
			writePostsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "title must be at least 10 characters")
			return
		}
		if len(*req.Title) > 200 {
			writePostsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "title must be at most 200 characters")
			return
		}
		updatedPost.Title = *req.Title
	}

	if req.Description != nil {
		if len(*req.Description) < 50 {
			writePostsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "description must be at least 50 characters")
			return
		}
		updatedPost.Description = *req.Description
	}

	if req.Tags != nil {
		if len(req.Tags) > 5 {
			writePostsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "maximum 5 tags allowed")
			return
		}
		updatedPost.Tags = req.Tags
	}

	if req.Status != nil {
		newStatus := models.PostStatus(*req.Status)
		if !models.IsValidPostStatus(newStatus, updatedPost.Type) {
			writePostsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid status for this post type")
			return
		}
		updatedPost.Status = newStatus
	}

	result, err := h.repo.Update(r.Context(), &updatedPost)
	if err != nil {
		writePostsError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to update post")
		return
	}

	writePostsJSON(w, http.StatusOK, map[string]interface{}{
		"data": result,
	})
}

// Delete handles DELETE /v1/posts/:id - soft delete a post.
// Per SPEC.md Part 15.1 and FIX-003: Users can delete their own content, admins can delete any.
func (h *PostsHandler) Delete(w http.ResponseWriter, r *http.Request) {
	// Require authentication (JWT or API key)
	authInfo := getAuthInfo(r)
	if authInfo == nil {
		writePostsError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}

	postID := chi.URLParam(r, "id")
	if postID == "" {
		writePostsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "post ID is required")
		return
	}

	// Get existing post
	existingPost, err := h.repo.FindByID(r.Context(), postID)
	if err != nil {
		if errors.Is(err, db.ErrPostNotFound) {
			writePostsError(w, http.StatusNotFound, "NOT_FOUND", "post not found")
			return
		}
		writePostsError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get post")
		return
	}

	// Check permission - owner or admin can delete (works for both humans and agents)
	isOwner := existingPost.PostedByType == authInfo.authorType && existingPost.PostedByID == authInfo.authorID
	isAdmin := authInfo.role == "admin"

	if !isOwner && !isAdmin {
		writePostsError(w, http.StatusForbidden, "FORBIDDEN", "you can only delete your own posts")
		return
	}

	if err := h.repo.Delete(r.Context(), postID); err != nil {
		writePostsError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to delete post")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// Vote handles POST /v1/posts/:id/vote - vote on a post.
// Per SPEC.md Part 2.9 and FIX-003: Both humans and agents can vote, but not on own content.
func (h *PostsHandler) Vote(w http.ResponseWriter, r *http.Request) {
	// Require authentication (JWT or API key)
	authInfo := getAuthInfo(r)
	if authInfo == nil {
		writePostsError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}

	postID := chi.URLParam(r, "id")
	if postID == "" {
		writePostsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "post ID is required")
		return
	}

	// Parse request body
	var req VoteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writePostsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid JSON body")
		return
	}

	// Validate direction
	if req.Direction != "up" && req.Direction != "down" {
		writePostsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "direction must be 'up' or 'down'")
		return
	}

	// Get post to check it exists
	post, err := h.repo.FindByID(r.Context(), postID)
	if err != nil {
		if errors.Is(err, db.ErrPostNotFound) {
			writePostsError(w, http.StatusNotFound, "NOT_FOUND", "post not found")
			return
		}
		writePostsError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get post")
		return
	}

	// Cannot vote on own content (applies to both humans and agents)
	if post.PostedByType == authInfo.authorType && post.PostedByID == authInfo.authorID {
		writePostsError(w, http.StatusForbidden, "FORBIDDEN", "cannot vote on your own content")
		return
	}

	// Record vote with the appropriate voter type
	err = h.repo.Vote(r.Context(), postID, string(authInfo.authorType), authInfo.authorID, req.Direction)
	if err != nil {
		if errors.Is(err, ErrDuplicateVote) {
			writePostsError(w, http.StatusConflict, "DUPLICATE_VOTE", "you have already voted on this post")
			return
		}
		writePostsError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to record vote")
		return
	}

	writePostsJSON(w, http.StatusOK, map[string]interface{}{
		"message": "vote recorded",
	})
}

// writePostsJSON writes a JSON response.
func writePostsJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// writePostsError writes an error JSON response.
func writePostsError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]interface{}{
			"code":    code,
			"message": message,
		},
	})
}
