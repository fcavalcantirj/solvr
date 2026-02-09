// Package handlers contains HTTP request handlers for the Solvr API.
package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/go-chi/chi/v5"
)

// ErrCommentNotFound is returned when a comment is not found.
var ErrCommentNotFound = errors.New("comment not found")

// CommentsRepositoryInterface defines the database operations for comments.
type CommentsRepositoryInterface interface {
	// List returns comments for a target.
	List(ctx context.Context, opts models.CommentListOptions) ([]models.CommentWithAuthor, int, error)

	// Create creates a new comment.
	Create(ctx context.Context, comment *models.Comment) (*models.Comment, error)

	// FindByID returns a single comment by ID.
	FindByID(ctx context.Context, id string) (*models.CommentWithAuthor, error)

	// Delete soft-deletes a comment by ID.
	Delete(ctx context.Context, id string) error

	// TargetExists checks if the target (approach, answer, response) exists.
	TargetExists(ctx context.Context, targetType models.CommentTargetType, targetID string) (bool, error)
}

// CommentsHandler handles comment-related HTTP requests.
type CommentsHandler struct {
	repo CommentsRepositoryInterface
}

// NewCommentsHandler creates a new CommentsHandler.
func NewCommentsHandler(repo CommentsRepositoryInterface) *CommentsHandler {
	return &CommentsHandler{repo: repo}
}

// CommentsListResponse is the response for listing comments.
type CommentsListResponse struct {
	Data []models.CommentWithAuthor `json:"data"`
	Meta CommentsListMeta           `json:"meta"`
}

// CommentsListMeta contains metadata for list responses.
type CommentsListMeta struct {
	Total   int  `json:"total"`
	Page    int  `json:"page"`
	PerPage int  `json:"per_page"`
	HasMore bool `json:"has_more"`
}

// List handles GET /v1/{target_type}/{id}/comments - list comments for a target.
func (h *CommentsHandler) List(w http.ResponseWriter, r *http.Request) {
	// Get target type and ID from URL
	targetTypeStr := chi.URLParam(r, "target_type")
	targetID := chi.URLParam(r, "id")

	// Validate target type
	targetType := models.CommentTargetType(targetTypeStr)
	if !models.IsValidCommentTargetType(targetType) {
		writeCommentsError(w, http.StatusBadRequest, "VALIDATION_ERROR",
			"invalid target type, must be one of: approach, answer, response")
		return
	}

	// Parse pagination params
	opts := models.CommentListOptions{
		TargetType: targetType,
		TargetID:   targetID,
		Page:       parseIntParam(r.URL.Query().Get("page"), 1),
		PerPage:    parseIntParam(r.URL.Query().Get("per_page"), 20),
	}

	if opts.Page < 1 {
		opts.Page = 1
	}
	if opts.PerPage < 1 {
		opts.PerPage = 20
	}
	if opts.PerPage > 50 {
		opts.PerPage = 50
	}

	// Query comments
	comments, total, err := h.repo.List(r.Context(), opts)
	if err != nil {
		writeCommentsError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to list comments")
		return
	}

	// Ensure data is never nil
	if comments == nil {
		comments = []models.CommentWithAuthor{}
	}

	hasMore := (opts.Page * opts.PerPage) < total

	response := CommentsListResponse{
		Data: comments,
		Meta: CommentsListMeta{
			Total:   total,
			Page:    opts.Page,
			PerPage: opts.PerPage,
			HasMore: hasMore,
		},
	}

	writeCommentsJSON(w, http.StatusOK, response)
}

// Create handles POST /v1/{target_type}/{id}/comments - create a comment.
// Per SPEC.md Part 1.4 and FIX-025: Both humans (JWT) and AI agents (API key) can comment.
func (h *CommentsHandler) Create(w http.ResponseWriter, r *http.Request) {
	// Require authentication (JWT or API key)
	authInfo := GetAuthInfo(r)
	if authInfo == nil {
		writeCommentsError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}

	// Get target type and ID from URL
	targetTypeStr := chi.URLParam(r, "target_type")
	targetID := chi.URLParam(r, "id")

	// Validate target type
	targetType := models.CommentTargetType(targetTypeStr)
	if !models.IsValidCommentTargetType(targetType) {
		writeCommentsError(w, http.StatusBadRequest, "VALIDATION_ERROR",
			"invalid target type, must be one of: approach, answer, response")
		return
	}

	// Parse request body
	var req models.CreateCommentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeCommentsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid JSON body")
		return
	}

	// Validate content
	content := strings.TrimSpace(req.Content)
	if content == "" {
		writeCommentsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "content is required")
		return
	}
	if len(content) > models.MaxCommentContentLength {
		writeCommentsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "content must be at most 2000 characters")
		return
	}

	// Check if target exists
	exists, err := h.repo.TargetExists(r.Context(), targetType, targetID)
	if err != nil {
		writeCommentsError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to verify target")
		return
	}
	if !exists {
		writeCommentsError(w, http.StatusNotFound, "NOT_FOUND", "target not found")
		return
	}

	// Create comment with author info from authentication
	comment := &models.Comment{
		TargetType: targetType,
		TargetID:   targetID,
		AuthorType: authInfo.AuthorType,
		AuthorID:   authInfo.AuthorID,
		Content:    content,
	}

	createdComment, err := h.repo.Create(r.Context(), comment)
	if err != nil {
		writeCommentsError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to create comment")
		return
	}

	writeCommentsJSON(w, http.StatusCreated, map[string]interface{}{
		"data": createdComment,
	})
}

// Delete handles DELETE /v1/comments/:id - soft delete a comment.
// Per SPEC.md Part 1.4 and FIX-025: Both humans (JWT) and AI agents (API key) can delete their comments.
func (h *CommentsHandler) Delete(w http.ResponseWriter, r *http.Request) {
	// Require authentication (JWT or API key)
	authInfo := GetAuthInfo(r)
	if authInfo == nil {
		writeCommentsError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}

	commentID := chi.URLParam(r, "id")
	if commentID == "" {
		writeCommentsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "comment ID is required")
		return
	}

	// Get existing comment
	comment, err := h.repo.FindByID(r.Context(), commentID)
	if err != nil {
		if errors.Is(err, ErrCommentNotFound) {
			writeCommentsError(w, http.StatusNotFound, "NOT_FOUND", "comment not found")
			return
		}
		writeCommentsError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get comment")
		return
	}

	// Check permission - owner or admin can delete (works for both humans and agents)
	isOwner := comment.AuthorType == authInfo.AuthorType && comment.AuthorID == authInfo.AuthorID
	isAdmin := authInfo.Role == "admin"

	if !isOwner && !isAdmin {
		writeCommentsError(w, http.StatusForbidden, "FORBIDDEN", "you can only delete your own comments")
		return
	}

	if err := h.repo.Delete(r.Context(), commentID); err != nil {
		writeCommentsError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to delete comment")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// writeCommentsJSON writes a JSON response.
func writeCommentsJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// writeCommentsError writes an error JSON response.
func writeCommentsError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]interface{}{
			"code":    code,
			"message": message,
		},
	})
}

