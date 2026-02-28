package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/fcavalcantirj/solvr/internal/api/response"
	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/go-chi/chi/v5"
)

// BlogPostRepositoryInterface defines the database operations for blog posts.
type BlogPostRepositoryInterface interface {
	List(ctx context.Context, opts models.BlogPostListOptions) ([]models.BlogPostWithAuthor, int, error)
	FindBySlug(ctx context.Context, slug string) (*models.BlogPostWithAuthor, error)
	FindBySlugForViewer(ctx context.Context, slug string, viewerType models.AuthorType, viewerID string) (*models.BlogPostWithAuthor, error)
	Create(ctx context.Context, post *models.BlogPost) (*models.BlogPost, error)
	Update(ctx context.Context, post *models.BlogPost) (*models.BlogPost, error)
	Delete(ctx context.Context, slug string) error
	Vote(ctx context.Context, blogPostID, voterType, voterID, direction string) error
	IncrementViewCount(ctx context.Context, slug string) error
	ListTags(ctx context.Context) ([]models.BlogTag, error)
	GetFeatured(ctx context.Context) (*models.BlogPostWithAuthor, error)
	SlugExists(ctx context.Context, slug string) (bool, error)
}

// BlogHandler handles blog-related HTTP requests.
type BlogHandler struct {
	repo              BlogPostRepositoryInterface
	logger            *slog.Logger
	contentModService ContentModerationServiceInterface
}

// NewBlogHandler creates a new BlogHandler.
func NewBlogHandler(repo BlogPostRepositoryInterface) *BlogHandler {
	return &BlogHandler{
		repo:   repo,
		logger: slog.New(slog.NewJSONHandler(os.Stderr, nil)),
	}
}

// SetContentModerationService sets the content moderation service.
func (h *BlogHandler) SetContentModerationService(svc ContentModerationServiceInterface) {
	h.contentModService = svc
}

// CreateBlogPostRequest is the request body for creating a blog post.
type CreateBlogPostRequest struct {
	Title           string   `json:"title"`
	Slug            string   `json:"slug,omitempty"`
	Body            string   `json:"body"`
	Excerpt         string   `json:"excerpt,omitempty"`
	Tags            []string `json:"tags,omitempty"`
	CoverImageURL   string   `json:"cover_image_url,omitempty"`
	Status          string   `json:"status,omitempty"`
	MetaDescription string   `json:"meta_description,omitempty"`
}

// UpdateBlogPostRequest is the request body for updating a blog post.
type UpdateBlogPostRequest struct {
	Title           *string  `json:"title,omitempty"`
	Body            *string  `json:"body,omitempty"`
	Excerpt         *string  `json:"excerpt,omitempty"`
	Tags            []string `json:"tags,omitempty"`
	CoverImageURL   *string  `json:"cover_image_url,omitempty"`
	Status          *string  `json:"status,omitempty"`
	MetaDescription *string  `json:"meta_description,omitempty"`
}

// BlogListResponse is the response for listing blog posts.
type BlogListResponse struct {
	Data []models.BlogPostWithAuthor `json:"data"`
	Meta BlogListMeta                `json:"meta"`
}

// BlogListMeta contains pagination metadata for blog list responses.
type BlogListMeta struct {
	Total   int  `json:"total"`
	Page    int  `json:"page"`
	PerPage int  `json:"per_page"`
	HasMore bool `json:"has_more"`
}

// BlogPostResponse is the response for a single blog post.
type BlogPostResponse struct {
	Data interface{} `json:"data"`
}

// List handles GET /v1/blog — list blog posts.
func (h *BlogHandler) List(w http.ResponseWriter, r *http.Request) {
	page, perPage, err := parsePaginationParams(r)
	if err != nil {
		response.WriteValidationError(w, err.Error(), nil)
		return
	}

	opts := models.BlogPostListOptions{
		Page:    page,
		PerPage: perPage,
	}

	// Parse tags filter
	if tagsParam := r.URL.Query().Get("tags"); tagsParam != "" {
		opts.Tags = strings.Split(tagsParam, ",")
		for i, tag := range opts.Tags {
			opts.Tags[i] = strings.TrimSpace(tag)
		}
	}

	// Parse sort
	if sortParam := r.URL.Query().Get("sort"); sortParam != "" {
		opts.Sort = sortParam
	}

	// Pass viewer info for user_vote lookup
	if authInfo := GetAuthInfo(r); authInfo != nil {
		opts.ViewerType = authInfo.AuthorType
		opts.ViewerID = authInfo.AuthorID
	}

	posts, total, err := h.repo.List(r.Context(), opts)
	if err != nil {
		ctx := response.LogContext{
			Operation: "List",
			Resource:  "blog_posts",
			RequestID: r.Header.Get("X-Request-ID"),
		}
		response.WriteInternalErrorWithLog(w, "failed to list blog posts", err, ctx, h.logger)
		return
	}

	hasMore := (opts.Page * opts.PerPage) < total

	resp := BlogListResponse{
		Data: posts,
		Meta: BlogListMeta{
			Total:   total,
			Page:    opts.Page,
			PerPage: opts.PerPage,
			HasMore: hasMore,
		},
	}

	writeBlogJSON(w, http.StatusOK, resp)
}

// GetBySlug handles GET /v1/blog/{slug} — get a single blog post.
func (h *BlogHandler) GetBySlug(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	if slug == "" {
		writeBlogError(w, http.StatusBadRequest, "VALIDATION_ERROR", "slug is required")
		return
	}

	var post *models.BlogPostWithAuthor
	var err error

	if authInfo := GetAuthInfo(r); authInfo != nil {
		post, err = h.repo.FindBySlugForViewer(r.Context(), slug, authInfo.AuthorType, authInfo.AuthorID)
	} else {
		post, err = h.repo.FindBySlug(r.Context(), slug)
	}

	if err != nil {
		if errors.Is(err, db.ErrBlogPostNotFound) {
			writeBlogError(w, http.StatusNotFound, "NOT_FOUND", "blog post not found")
			return
		}
		ctx := response.LogContext{
			Operation: "FindBySlug",
			Resource:  "blog_post",
			RequestID: r.Header.Get("X-Request-ID"),
			Extra:     map[string]string{"slug": slug},
		}
		response.WriteInternalErrorWithLog(w, "failed to get blog post", err, ctx, h.logger)
		return
	}

	writeBlogJSON(w, http.StatusOK, BlogPostResponse{Data: post})
}

// Create handles POST /v1/blog — create a new blog post.
func (h *BlogHandler) Create(w http.ResponseWriter, r *http.Request) {
	authInfo := GetAuthInfo(r)
	if authInfo == nil {
		writeBlogError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}

	var req CreateBlogPostRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeBlogError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid JSON body")
		return
	}

	// Validate title
	if req.Title == "" {
		writeBlogError(w, http.StatusBadRequest, "VALIDATION_ERROR", "title is required")
		return
	}
	if len(req.Title) < 10 {
		writeBlogError(w, http.StatusBadRequest, "VALIDATION_ERROR", "title must be at least 10 characters")
		return
	}
	if len(req.Title) > 300 {
		writeBlogError(w, http.StatusBadRequest, "VALIDATION_ERROR", "title must be at most 300 characters")
		return
	}

	// Validate body
	if req.Body == "" {
		writeBlogError(w, http.StatusBadRequest, "VALIDATION_ERROR", "body is required")
		return
	}
	if len(req.Body) < 50 {
		writeBlogError(w, http.StatusBadRequest, "VALIDATION_ERROR", "body must be at least 50 characters")
		return
	}

	// Validate tags
	if len(req.Tags) > 10 {
		writeBlogError(w, http.StatusBadRequest, "VALIDATION_ERROR", "maximum 10 tags allowed")
		return
	}

	// Auto-generate slug if empty
	slug := req.Slug
	if slug == "" {
		slug = models.GenerateSlug(req.Title)
	}

	// Validate slug format
	if !validateSlug(slug) {
		writeBlogError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid slug format")
		return
	}

	// Determine status
	status := models.BlogPostStatusDraft
	if req.Status != "" {
		if !models.IsValidBlogPostStatus(req.Status) {
			writeBlogError(w, http.StatusBadRequest, "VALIDATION_ERROR", "status must be one of: draft, published, archived")
			return
		}
		status = models.BlogPostStatus(req.Status)
	}

	// Auto-calculate read time and excerpt
	readTime := models.CalculateReadTime(req.Body)
	excerpt := req.Excerpt
	if excerpt == "" {
		excerpt = models.GenerateExcerpt(req.Body, 500)
	}

	post := &models.BlogPost{
		Slug:            slug,
		Title:           req.Title,
		Body:            req.Body,
		Excerpt:         excerpt,
		Tags:            req.Tags,
		CoverImageURL:   req.CoverImageURL,
		PostedByType:    authInfo.AuthorType,
		PostedByID:      authInfo.AuthorID,
		Status:          status,
		ReadTimeMinutes: readTime,
		MetaDescription: req.MetaDescription,
	}

	createdPost, err := h.repo.Create(r.Context(), post)
	if err != nil {
		if errors.Is(err, db.ErrDuplicateSlug) {
			writeBlogError(w, http.StatusConflict, "DUPLICATE_CONTENT", "a blog post with this slug already exists")
			return
		}
		ctx := response.LogContext{
			Operation: "Create",
			Resource:  "blog_post",
			RequestID: r.Header.Get("X-Request-ID"),
			Extra: map[string]string{
				"authorType": string(authInfo.AuthorType),
				"authorID":   authInfo.AuthorID,
			},
		}
		response.WriteInternalErrorWithLog(w, "failed to create blog post", err, ctx, h.logger)
		return
	}

	writeBlogJSON(w, http.StatusCreated, BlogPostResponse{Data: createdPost})
}

// Update handles PATCH /v1/blog/{slug} — update a blog post.
func (h *BlogHandler) Update(w http.ResponseWriter, r *http.Request) {
	authInfo := GetAuthInfo(r)
	if authInfo == nil {
		writeBlogError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}

	slug := chi.URLParam(r, "slug")
	if slug == "" {
		writeBlogError(w, http.StatusBadRequest, "VALIDATION_ERROR", "slug is required")
		return
	}

	// Fetch existing post
	existing, err := h.repo.FindBySlug(r.Context(), slug)
	if err != nil {
		if errors.Is(err, db.ErrBlogPostNotFound) {
			writeBlogError(w, http.StatusNotFound, "NOT_FOUND", "blog post not found")
			return
		}
		ctx := response.LogContext{
			Operation: "FindBySlug",
			Resource:  "blog_post",
			RequestID: r.Header.Get("X-Request-ID"),
			Extra:     map[string]string{"slug": slug},
		}
		response.WriteInternalErrorWithLog(w, "failed to get blog post", err, ctx, h.logger)
		return
	}

	// Verify ownership
	if existing.PostedByType != authInfo.AuthorType || existing.PostedByID != authInfo.AuthorID {
		writeBlogError(w, http.StatusForbidden, "FORBIDDEN", "you can only update your own blog posts")
		return
	}

	// Parse update request
	var req UpdateBlogPostRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeBlogError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid JSON body")
		return
	}

	// Apply partial updates to existing post
	updatedPost := existing.BlogPost

	if req.Title != nil {
		if len(*req.Title) < 10 {
			writeBlogError(w, http.StatusBadRequest, "VALIDATION_ERROR", "title must be at least 10 characters")
			return
		}
		if len(*req.Title) > 300 {
			writeBlogError(w, http.StatusBadRequest, "VALIDATION_ERROR", "title must be at most 300 characters")
			return
		}
		updatedPost.Title = *req.Title
	}

	if req.Body != nil {
		if len(*req.Body) < 50 {
			writeBlogError(w, http.StatusBadRequest, "VALIDATION_ERROR", "body must be at least 50 characters")
			return
		}
		updatedPost.Body = *req.Body
		updatedPost.ReadTimeMinutes = models.CalculateReadTime(*req.Body)
	}

	if req.Excerpt != nil {
		updatedPost.Excerpt = *req.Excerpt
	}

	if req.Tags != nil {
		if len(req.Tags) > 10 {
			writeBlogError(w, http.StatusBadRequest, "VALIDATION_ERROR", "maximum 10 tags allowed")
			return
		}
		updatedPost.Tags = req.Tags
	}

	if req.CoverImageURL != nil {
		updatedPost.CoverImageURL = *req.CoverImageURL
	}

	if req.MetaDescription != nil {
		updatedPost.MetaDescription = *req.MetaDescription
	}

	if req.Status != nil {
		if !models.IsValidBlogPostStatus(*req.Status) {
			writeBlogError(w, http.StatusBadRequest, "VALIDATION_ERROR", "status must be one of: draft, published, archived")
			return
		}
		newStatus := models.BlogPostStatus(*req.Status)
		// Set published_at on transition to published
		if newStatus == models.BlogPostStatusPublished && existing.Status != models.BlogPostStatusPublished {
			now := time.Now()
			updatedPost.PublishedAt = &now
		}
		updatedPost.Status = newStatus
	}

	result, err := h.repo.Update(r.Context(), &updatedPost)
	if err != nil {
		ctx := response.LogContext{
			Operation: "Update",
			Resource:  "blog_post",
			RequestID: r.Header.Get("X-Request-ID"),
			Extra:     map[string]string{"slug": slug},
		}
		response.WriteInternalErrorWithLog(w, "failed to update blog post", err, ctx, h.logger)
		return
	}

	writeBlogJSON(w, http.StatusOK, BlogPostResponse{Data: result})
}

// Delete handles DELETE /v1/blog/{slug} — soft delete a blog post.
func (h *BlogHandler) Delete(w http.ResponseWriter, r *http.Request) {
	authInfo := GetAuthInfo(r)
	if authInfo == nil {
		writeBlogError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}

	slug := chi.URLParam(r, "slug")
	if slug == "" {
		writeBlogError(w, http.StatusBadRequest, "VALIDATION_ERROR", "slug is required")
		return
	}

	// Fetch existing to verify ownership
	existing, err := h.repo.FindBySlug(r.Context(), slug)
	if err != nil {
		if errors.Is(err, db.ErrBlogPostNotFound) {
			writeBlogError(w, http.StatusNotFound, "NOT_FOUND", "blog post not found")
			return
		}
		ctx := response.LogContext{
			Operation: "FindBySlug",
			Resource:  "blog_post",
			RequestID: r.Header.Get("X-Request-ID"),
			Extra:     map[string]string{"slug": slug},
		}
		response.WriteInternalErrorWithLog(w, "failed to get blog post", err, ctx, h.logger)
		return
	}

	// Verify ownership
	if existing.PostedByType != authInfo.AuthorType || existing.PostedByID != authInfo.AuthorID {
		writeBlogError(w, http.StatusForbidden, "FORBIDDEN", "you can only delete your own blog posts")
		return
	}

	if err := h.repo.Delete(r.Context(), slug); err != nil {
		ctx := response.LogContext{
			Operation: "Delete",
			Resource:  "blog_post",
			RequestID: r.Header.Get("X-Request-ID"),
			Extra:     map[string]string{"slug": slug},
		}
		response.WriteInternalErrorWithLog(w, "failed to delete blog post", err, ctx, h.logger)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// Vote handles POST /v1/blog/{slug}/vote — vote on a blog post.
func (h *BlogHandler) Vote(w http.ResponseWriter, r *http.Request) {
	authInfo := GetAuthInfo(r)
	if authInfo == nil {
		writeBlogError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}

	slug := chi.URLParam(r, "slug")
	if slug == "" {
		writeBlogError(w, http.StatusBadRequest, "VALIDATION_ERROR", "slug is required")
		return
	}

	var req VoteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeBlogError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid JSON body")
		return
	}

	if req.Direction != "up" && req.Direction != "down" {
		writeBlogError(w, http.StatusBadRequest, "VALIDATION_ERROR", "direction must be 'up' or 'down'")
		return
	}

	// Fetch post to get ID and check self-vote
	post, err := h.repo.FindBySlug(r.Context(), slug)
	if err != nil {
		if errors.Is(err, db.ErrBlogPostNotFound) {
			writeBlogError(w, http.StatusNotFound, "NOT_FOUND", "blog post not found")
			return
		}
		ctx := response.LogContext{
			Operation: "FindBySlug",
			Resource:  "blog_post",
			RequestID: r.Header.Get("X-Request-ID"),
			Extra:     map[string]string{"slug": slug},
		}
		response.WriteInternalErrorWithLog(w, "failed to get blog post", err, ctx, h.logger)
		return
	}

	// Prevent self-vote
	if post.PostedByType == authInfo.AuthorType && post.PostedByID == authInfo.AuthorID {
		writeBlogError(w, http.StatusForbidden, "FORBIDDEN", "cannot vote on your own blog post")
		return
	}

	if err := h.repo.Vote(r.Context(), post.ID, string(authInfo.AuthorType), authInfo.AuthorID, req.Direction); err != nil {
		ctx := response.LogContext{
			Operation: "Vote",
			Resource:  "blog_post",
			RequestID: r.Header.Get("X-Request-ID"),
			Extra: map[string]string{
				"slug":      slug,
				"direction": req.Direction,
			},
		}
		response.WriteInternalErrorWithLog(w, "failed to vote on blog post", err, ctx, h.logger)
		return
	}

	writeBlogJSON(w, http.StatusOK, map[string]interface{}{
		"data": map[string]string{
			"status":    "ok",
			"direction": req.Direction,
		},
	})
}

// RecordView handles POST /v1/blog/{slug}/view — increment view count.
func (h *BlogHandler) RecordView(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	if slug == "" {
		writeBlogError(w, http.StatusBadRequest, "VALIDATION_ERROR", "slug is required")
		return
	}

	if err := h.repo.IncrementViewCount(r.Context(), slug); err != nil {
		if errors.Is(err, db.ErrBlogPostNotFound) {
			writeBlogError(w, http.StatusNotFound, "NOT_FOUND", "blog post not found")
			return
		}
		ctx := response.LogContext{
			Operation: "IncrementViewCount",
			Resource:  "blog_post",
			RequestID: r.Header.Get("X-Request-ID"),
			Extra:     map[string]string{"slug": slug},
		}
		response.WriteInternalErrorWithLog(w, "failed to record view", err, ctx, h.logger)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// GetFeatured handles GET /v1/blog/featured — get the featured blog post.
func (h *BlogHandler) GetFeatured(w http.ResponseWriter, r *http.Request) {
	post, err := h.repo.GetFeatured(r.Context())
	if err != nil {
		if errors.Is(err, db.ErrBlogPostNotFound) {
			writeBlogError(w, http.StatusNotFound, "NOT_FOUND", "no featured blog post found")
			return
		}
		ctx := response.LogContext{
			Operation: "GetFeatured",
			Resource:  "blog_post",
			RequestID: r.Header.Get("X-Request-ID"),
		}
		response.WriteInternalErrorWithLog(w, "failed to get featured blog post", err, ctx, h.logger)
		return
	}

	writeBlogJSON(w, http.StatusOK, BlogPostResponse{Data: post})
}

// ListTags handles GET /v1/blog/tags — list all tags with counts.
func (h *BlogHandler) ListTags(w http.ResponseWriter, r *http.Request) {
	tags, err := h.repo.ListTags(r.Context())
	if err != nil {
		ctx := response.LogContext{
			Operation: "ListTags",
			Resource:  "blog_post",
			RequestID: r.Header.Get("X-Request-ID"),
		}
		response.WriteInternalErrorWithLog(w, "failed to list blog tags", err, ctx, h.logger)
		return
	}

	writeBlogJSON(w, http.StatusOK, map[string]interface{}{
		"data": tags,
	})
}

// Ensure interface compliance.
var _ BlogPostRepositoryInterface = (*db.BlogPostRepository)(nil)
