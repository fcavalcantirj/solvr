// Package handlers contains HTTP request handlers for the Solvr API.
package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/fcavalcantirj/solvr/internal/api/response"
	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// Pagination constants per SPEC.md Part 5.6
const (
	DefaultPage    = 1
	DefaultPerPage = 20
	MaxPerPage     = 50
)

// parsePaginationParams validates and parses page and per_page query parameters.
// FIX-029: Returns error for invalid values instead of silently correcting.
// Per SPEC.md Part 5.6: page >= 1, per_page >= 1 and <= 50.
func parsePaginationParams(r *http.Request) (page, perPage int, err error) {
	page = DefaultPage
	perPage = DefaultPerPage

	// Parse page parameter
	if pageStr := r.URL.Query().Get("page"); pageStr != "" {
		parsedPage, parseErr := strconv.Atoi(pageStr)
		if parseErr != nil {
			return 0, 0, fmt.Errorf("page must be a valid integer")
		}
		if parsedPage < 1 {
			return 0, 0, fmt.Errorf("page must be >= 1")
		}
		page = parsedPage
	}

	// Parse per_page parameter
	if perPageStr := r.URL.Query().Get("per_page"); perPageStr != "" {
		parsedPerPage, parseErr := strconv.Atoi(perPageStr)
		if parseErr != nil {
			return 0, 0, fmt.Errorf("per_page must be a valid integer")
		}
		if parsedPerPage < 1 {
			return 0, 0, fmt.Errorf("per_page must be >= 1")
		}
		if parsedPerPage > MaxPerPage {
			return 0, 0, fmt.Errorf("per_page must be <= %d", MaxPerPage)
		}
		perPage = parsedPerPage
	}

	return page, perPage, nil
}

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

	// GetUserVote returns the user's current vote on a post, or nil if not voted.
	GetUserVote(ctx context.Context, postID, voterType, voterID string) (*string, error)
}

// PostsHandler handles post-related HTTP requests.
// EmbeddingServiceInterface defines the interface for generating text embeddings.
// Used by PostsHandler to generate embeddings on post creation.
type EmbeddingServiceInterface interface {
	GenerateEmbedding(ctx context.Context, text string) ([]float32, error)
	GenerateQueryEmbedding(ctx context.Context, text string) ([]float32, error)
}

// ModerationInput contains the post content to be moderated.
// Mirrors services.ModerationInput to avoid import cycle.
type ModerationInput struct {
	Title       string
	Description string
	Tags        []string
}

// ModerationResult contains the moderation decision.
// Mirrors services.ModerationResult to avoid import cycle.
type ModerationResult struct {
	Approved         bool
	LanguageDetected string
	RejectionReasons []string
	Confidence       float64
	Explanation      string
}

// RateLimitError is returned when a moderation API is rate limited.
type RateLimitError interface {
	error
	GetRetryAfter() time.Duration
}

// ContentModerationServiceInterface defines the interface for content moderation.
type ContentModerationServiceInterface interface {
	ModerateContent(ctx context.Context, input ModerationInput) (*ModerationResult, error)
}

// PostStatusUpdaterInterface updates post status without requiring full PostsRepositoryInterface.
type PostStatusUpdaterInterface interface {
	UpdateStatus(ctx context.Context, postID string, status models.PostStatus) error
}

// FlagCreatorInterface creates admin flags for moderation failures.
type FlagCreatorInterface interface {
	CreateFlag(ctx context.Context, flag *models.Flag) (*models.Flag, error)
}

// CommentCreatorInterface creates comments for moderation results.
type CommentCreatorInterface interface {
	Create(ctx context.Context, comment *models.Comment) (*models.Comment, error)
}

// NotificationServiceInterface sends notifications on moderation decisions.
type NotificationServiceInterface interface {
	NotifyOnModerationResult(ctx context.Context, postID, postTitle, postType, authorType, authorID string, approved bool, explanation string) error
}

// Default retry delays for content moderation (exponential backoff: 2s, 4s, 8s).
var defaultRetryDelays = []time.Duration{2 * time.Second, 4 * time.Second, 8 * time.Second}

type PostsHandler struct {
	repo              PostsRepositoryInterface
	logger            *slog.Logger
	embeddingService  EmbeddingServiceInterface
	contentModService ContentModerationServiceInterface
	statusUpdater     PostStatusUpdaterInterface
	flagCreator       FlagCreatorInterface
	commentRepo       CommentCreatorInterface
	notifService      NotificationServiceInterface
	retryDelays       []time.Duration
}

// NewPostsHandler creates a new PostsHandler.
func NewPostsHandler(repo PostsRepositoryInterface) *PostsHandler {
	return &PostsHandler{
		repo:        repo,
		logger:      slog.New(slog.NewJSONHandler(os.Stderr, nil)),
		retryDelays: defaultRetryDelays,
	}
}

// SetLogger sets a custom logger for the handler.
// This is useful for testing or custom logging configurations.
func (h *PostsHandler) SetLogger(logger *slog.Logger) {
	h.logger = logger
}

// SetEmbeddingService sets the embedding service for generating post embeddings.
// When set, post creation will generate and store embeddings for semantic search.
func (h *PostsHandler) SetEmbeddingService(svc EmbeddingServiceInterface) {
	h.embeddingService = svc
}

// SetContentModerationService sets the content moderation service.
// When set, post creation triggers async moderation via Groq.
func (h *PostsHandler) SetContentModerationService(svc ContentModerationServiceInterface) {
	h.contentModService = svc
}

// SetPostStatusUpdater sets the post status updater for async moderation.
func (h *PostsHandler) SetPostStatusUpdater(updater PostStatusUpdaterInterface) {
	h.statusUpdater = updater
}

// SetFlagCreator sets the flag creator for moderation failure reporting.
func (h *PostsHandler) SetFlagCreator(creator FlagCreatorInterface) {
	h.flagCreator = creator
}

// SetCommentRepo sets the comment repository for creating moderation comments.
func (h *PostsHandler) SetCommentRepo(repo CommentCreatorInterface) {
	h.commentRepo = repo
}

// SetNotificationService sets the notification service for moderation notifications.
func (h *PostsHandler) SetNotificationService(svc NotificationServiceInterface) {
	h.notifService = svc
}

// SetRetryDelays overrides retry delays (useful for testing).
func (h *PostsHandler) SetRetryDelays(delays []time.Duration) {
	h.retryDelays = delays
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

// List handles GET /v1/posts - list posts.
func (h *PostsHandler) List(w http.ResponseWriter, r *http.Request) {
	// FIX-029: Validate pagination parameters
	page, perPage, err := parsePaginationParams(r)
	if err != nil {
		response.WriteValidationError(w, err.Error(), nil)
		return
	}

	opts := models.PostListOptions{
		Page:    page,
		PerPage: perPage,
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

	// FE-024: Parse author filter for user profile pages
	if authorType := r.URL.Query().Get("author_type"); authorType != "" {
		opts.AuthorType = models.AuthorType(authorType)
	}
	if authorID := r.URL.Query().Get("author_id"); authorID != "" {
		opts.AuthorID = authorID
	}

	// Allow authenticated users to see their own hidden posts (pending_review, rejected, draft)
	if opts.AuthorID != "" {
		if authInfo := GetAuthInfo(r); authInfo != nil {
			if authInfo.AuthorID == opts.AuthorID && string(authInfo.AuthorType) == string(opts.AuthorType) {
				opts.IncludeHidden = true
			}
		}
	}

	// Execute query
	posts, total, err := h.repo.List(r.Context(), opts)
	if err != nil {
		ctx := response.LogContext{
			Operation: "List",
			Resource:  "posts",
			RequestID: r.Header.Get("X-Request-ID"),
		}
		response.WriteInternalErrorWithLog(w, "failed to list posts", err, ctx, h.logger)
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
		ctx := response.LogContext{
			Operation: "FindByID",
			Resource:  "post",
			RequestID: r.Header.Get("X-Request-ID"),
			Extra:     map[string]string{"postID": postID},
		}
		response.WriteInternalErrorWithLog(w, "failed to get post", err, ctx, h.logger)
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
	authInfo := GetAuthInfo(r)
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

	// Validate tags
	if len(req.Tags) > models.MaxTagsPerPost {
		writePostsError(w, http.StatusBadRequest, "VALIDATION_ERROR", fmt.Sprintf("maximum %d tags allowed", models.MaxTagsPerPost))
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
		PostedByType:    authInfo.AuthorType,
		PostedByID:      authInfo.AuthorID,
		Status:          models.PostStatusPendingReview,
		SuccessCriteria: req.SuccessCriteria,
		Weight:          req.Weight,
	}

	// Synchronous embedding adds ~50-100ms latency but ensures post is immediately searchable
	if h.embeddingService != nil {
		embedCtx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		text := post.Title + " " + post.Description
		embedding, embedErr := h.embeddingService.GenerateEmbedding(embedCtx, text)
		if embedErr != nil {
			h.logger.Warn("failed to generate embedding for post", "error", embedErr)
		} else {
			vecStr := float32SliceToVectorString(embedding)
			post.EmbeddingStr = &vecStr
		}
	}

	createdPost, err := h.repo.Create(r.Context(), post)
	if err != nil {
		ctx := response.LogContext{
			Operation: "Create",
			Resource:  "post",
			RequestID: r.Header.Get("X-Request-ID"),
			Extra: map[string]string{
				"type":       string(postType),
				"authorType": string(authInfo.AuthorType),
				"authorID":   authInfo.AuthorID,
			},
		}
		response.WriteInternalErrorWithLog(w, "failed to create post", err, ctx, h.logger)
		return
	}

	// Trigger async content moderation if service is configured
	if h.contentModService != nil {
		go h.moderatePostAsync(createdPost.ID, post.Title, post.Description, post.Tags, string(post.Type), string(authInfo.AuthorType), authInfo.AuthorID)
	}

	writePostsJSON(w, http.StatusCreated, map[string]interface{}{
		"data": createdPost,
	})
}

// Update handles PATCH /v1/posts/:id - update a post.
// Per SPEC.md Part 15.2 and FIX-003: Users can edit their own content (humans and agents).
func (h *PostsHandler) Update(w http.ResponseWriter, r *http.Request) {
	// Require authentication (JWT or API key)
	authInfo := GetAuthInfo(r)
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
		ctx := response.LogContext{
			Operation: "FindByID",
			Resource:  "post",
			RequestID: r.Header.Get("X-Request-ID"),
			Extra:     map[string]string{"postID": postID, "caller": "Update"},
		}
		response.WriteInternalErrorWithLog(w, "failed to get post", err, ctx, h.logger)
		return
	}

	// Check ownership - only owner can update (works for both humans and agents)
	isOwner := existingPost.PostedByType == authInfo.AuthorType && existingPost.PostedByID == authInfo.AuthorID
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
		if len(req.Tags) > models.MaxTagsPerPost {
			writePostsError(w, http.StatusBadRequest, "VALIDATION_ERROR", fmt.Sprintf("maximum %d tags allowed", models.MaxTagsPerPost))
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

	// Regenerate embedding if title or description changed
	contentChanged := req.Title != nil || req.Description != nil
	if contentChanged && h.embeddingService != nil {
		embedCtx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		text := updatedPost.Title + " " + updatedPost.Description
		embedding, embedErr := h.embeddingService.GenerateEmbedding(embedCtx, text)
		if embedErr != nil {
			h.logger.Warn("failed to regenerate embedding for post", "error", embedErr, "postID", postID)
		} else {
			vecStr := float32SliceToVectorString(embedding)
			updatedPost.EmbeddingStr = &vecStr
		}
	}

	result, err := h.repo.Update(r.Context(), &updatedPost)
	if err != nil {
		ctx := response.LogContext{
			Operation: "Update",
			Resource:  "post",
			RequestID: r.Header.Get("X-Request-ID"),
			Extra:     map[string]string{"postID": postID},
		}
		response.WriteInternalErrorWithLog(w, "failed to update post", err, ctx, h.logger)
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
	authInfo := GetAuthInfo(r)
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
		ctx := response.LogContext{
			Operation: "FindByID",
			Resource:  "post",
			RequestID: r.Header.Get("X-Request-ID"),
			Extra:     map[string]string{"postID": postID, "caller": "Delete"},
		}
		response.WriteInternalErrorWithLog(w, "failed to get post", err, ctx, h.logger)
		return
	}

	// Check permission - owner or admin can delete (works for both humans and agents)
	isOwner := existingPost.PostedByType == authInfo.AuthorType && existingPost.PostedByID == authInfo.AuthorID
	isAdmin := authInfo.Role == "admin"

	if !isOwner && !isAdmin {
		writePostsError(w, http.StatusForbidden, "FORBIDDEN", "you can only delete your own posts")
		return
	}

	if err := h.repo.Delete(r.Context(), postID); err != nil {
		ctx := response.LogContext{
			Operation: "Delete",
			Resource:  "post",
			RequestID: r.Header.Get("X-Request-ID"),
			Extra:     map[string]string{"postID": postID},
		}
		response.WriteInternalErrorWithLog(w, "failed to delete post", err, ctx, h.logger)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// Vote handles POST /v1/posts/:id/vote - vote on a post.
// Per SPEC.md Part 2.9 and FIX-003: Both humans and agents can vote, but not on own content.
func (h *PostsHandler) Vote(w http.ResponseWriter, r *http.Request) {
	// Require authentication (JWT or API key)
	authInfo := GetAuthInfo(r)
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
		ctx := response.LogContext{
			Operation: "FindByID",
			Resource:  "post",
			RequestID: r.Header.Get("X-Request-ID"),
			Extra:     map[string]string{"postID": postID, "caller": "Vote"},
		}
		response.WriteInternalErrorWithLog(w, "failed to get post", err, ctx, h.logger)
		return
	}

	// Cannot vote on own content (applies to both humans and agents)
	if post.PostedByType == authInfo.AuthorType && post.PostedByID == authInfo.AuthorID {
		writePostsError(w, http.StatusForbidden, "FORBIDDEN", "cannot vote on your own content")
		return
	}

	// Record vote with the appropriate voter type
	err = h.repo.Vote(r.Context(), postID, string(authInfo.AuthorType), authInfo.AuthorID, req.Direction)
	if err != nil {
		if errors.Is(err, ErrDuplicateVote) {
			writePostsError(w, http.StatusConflict, "DUPLICATE_VOTE", "you have already voted on this post")
			return
		}
		ctx := response.LogContext{
			Operation: "Vote",
			Resource:  "post",
			RequestID: r.Header.Get("X-Request-ID"),
			Extra: map[string]string{
				"postID":    postID,
				"direction": req.Direction,
				"voterType": string(authInfo.AuthorType),
				"voterID":   authInfo.AuthorID,
			},
		}
		response.WriteInternalErrorWithLog(w, "failed to record vote", err, ctx, h.logger)
		return
	}

	// Re-fetch post to get updated vote counts
	updatedPost, fetchErr := h.repo.FindByID(r.Context(), postID)
	if fetchErr != nil {
		// Vote was recorded but re-fetch failed — return success with zeroed scores
		writePostsJSON(w, http.StatusOK, map[string]interface{}{
			"data": map[string]interface{}{
				"vote_score": 0,
				"upvotes":    0,
				"downvotes":  0,
				"user_vote":  req.Direction,
			},
		})
		return
	}

	writePostsJSON(w, http.StatusOK, map[string]interface{}{
		"data": map[string]interface{}{
			"vote_score": updatedPost.VoteScore,
			"upvotes":    updatedPost.Upvotes,
			"downvotes":  updatedPost.Downvotes,
			"user_vote":  req.Direction,
		},
	})
}

// GetMyVote handles GET /v1/posts/:id/my-vote - get current user's vote on a post.
func (h *PostsHandler) GetMyVote(w http.ResponseWriter, r *http.Request) {
	// Require authentication
	authInfo := GetAuthInfo(r)
	if authInfo == nil {
		writePostsError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}

	postID := chi.URLParam(r, "id")
	if postID == "" {
		writePostsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "post ID is required")
		return
	}

	vote, err := h.repo.GetUserVote(r.Context(), postID, string(authInfo.AuthorType), authInfo.AuthorID)
	if err != nil {
		if errors.Is(err, db.ErrPostNotFound) {
			writePostsError(w, http.StatusNotFound, "NOT_FOUND", "post not found")
			return
		}
		ctx := response.LogContext{
			Operation: "GetUserVote",
			Resource:  "post",
			RequestID: r.Header.Get("X-Request-ID"),
			Extra:     map[string]string{"postID": postID, "caller": "GetMyVote"},
		}
		response.WriteInternalErrorWithLog(w, "failed to get user vote", err, ctx, h.logger)
		return
	}

	writePostsJSON(w, http.StatusOK, map[string]interface{}{
		"data": map[string]interface{}{
			"vote": vote,
		},
	})
}

// writePostsJSON writes a JSON response.
func writePostsJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// float32SliceToVectorString converts a float32 slice to PostgreSQL vector literal format.
// Example: [0.1, 0.2, 0.3] -> "[0.1,0.2,0.3]"
func float32SliceToVectorString(v []float32) string {
	if len(v) == 0 {
		return "[]"
	}
	s := "["
	for i, f := range v {
		if i > 0 {
			s += ","
		}
		s += fmt.Sprintf("%g", f)
	}
	s += "]"
	return s
}

// moderatePostAsync runs content moderation asynchronously with retry logic.
// Uses context.Background() with 30s timeout (not request context).
func (h *PostsHandler) moderatePostAsync(postID, title, description string, tags []string, postType, authorType, authorID string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	input := ModerationInput{
		Title:       title,
		Description: description,
		Tags:        tags,
	}

	maxAttempts := len(h.retryDelays)
	attempt := 0

	for attempt < maxAttempts {
		result, err := h.contentModService.ModerateContent(ctx, input)
		if err != nil {
			// Rate limit errors: sleep and retry without counting as attempt
			var rateLimitErr RateLimitError
			if errors.As(err, &rateLimitErr) {
				retryAfter := rateLimitErr.GetRetryAfter()
				h.logger.Warn("moderation rate limited, retrying", "postID", postID, "retryAfter", retryAfter)
				time.Sleep(retryAfter)
				continue
			}

			// Other errors: count as attempt and use exponential backoff
			attempt++
			h.logger.Warn("moderation attempt failed", "postID", postID, "attempt", attempt, "error", err)
			if attempt < maxAttempts {
				time.Sleep(h.retryDelays[attempt-1])
				continue
			}

			// All retries exhausted
			h.logger.Error("moderation failed after all retries", "postID", postID, "attempts", attempt)
			if h.flagCreator != nil {
				parsedID, parseErr := uuid.Parse(postID)
				if parseErr != nil {
					h.logger.Error("invalid post ID for flag creation", "postID", postID, "error", parseErr)
					return
				}
				flag := &models.Flag{
					TargetType:   "post",
					TargetID:     parsedID,
					ReporterType: "system",
					ReporterID:   "content-moderation",
					Reason:       "moderation_failed",
					Details:      fmt.Sprintf("Content moderation failed after %d attempts: %v", attempt, err),
					Status:       "pending",
				}
				if _, flagErr := h.flagCreator.CreateFlag(ctx, flag); flagErr != nil {
					h.logger.Error("failed to create moderation failure flag", "postID", postID, "error", flagErr)
				}
			}
			return
		}

		// Moderation succeeded - update status
		if h.statusUpdater == nil {
			h.logger.Error("no status updater configured", "postID", postID)
			return
		}

		var newStatus models.PostStatus
		if result.Approved {
			newStatus = models.PostStatusOpen
		} else {
			newStatus = models.PostStatusRejected
		}

		if err := h.statusUpdater.UpdateStatus(ctx, postID, newStatus); err != nil {
			h.logger.Error("failed to update post status after moderation", "postID", postID, "status", newStatus, "error", err)
		}

		// Create system comment explaining the moderation decision
		if h.commentRepo != nil {
			var commentContent string
			if result.Approved {
				commentContent = "Post approved by Solvr moderation. Your post is now visible in the feed."
			} else {
				commentContent = fmt.Sprintf("Post rejected by Solvr moderation.\n\nReason: %s\n\nYou can edit your post and resubmit for review.", result.Explanation)
			}

			comment := &models.Comment{
				TargetType: models.CommentTargetPost,
				TargetID:   postID,
				AuthorType: models.AuthorTypeSystem,
				AuthorID:   "solvr-moderator",
				Content:    commentContent,
			}
			if _, commentErr := h.commentRepo.Create(ctx, comment); commentErr != nil {
				h.logger.Error("failed to create moderation comment", "postID", postID, "error", commentErr)
			}
		}

		// Send notification to post author about moderation result
		if h.notifService != nil {
			if notifErr := h.notifService.NotifyOnModerationResult(ctx, postID, title, postType, authorType, authorID, result.Approved, result.Explanation); notifErr != nil {
				h.logger.Error("failed to send moderation notification", "postID", postID, "error", notifErr)
			}
		}
		return
	}
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
