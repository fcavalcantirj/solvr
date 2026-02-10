// Package handlers contains HTTP request handlers for the Solvr API.
package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/go-chi/chi/v5"
)

// IdeasRepositoryInterface defines the database operations for ideas.
type IdeasRepositoryInterface interface {
	// ListIdeas returns ideas matching the given options.
	ListIdeas(ctx context.Context, opts models.PostListOptions) ([]models.PostWithAuthor, int, error)

	// FindIdeaByID returns a single idea by ID.
	FindIdeaByID(ctx context.Context, id string) (*models.PostWithAuthor, error)

	// CreateIdea creates a new idea and returns it.
	CreateIdea(ctx context.Context, post *models.Post) (*models.Post, error)

	// ListResponses returns responses for an idea.
	ListResponses(ctx context.Context, ideaID string, opts models.ResponseListOptions) ([]models.ResponseWithAuthor, int, error)

	// CreateResponse creates a new response and returns it.
	CreateResponse(ctx context.Context, response *models.Response) (*models.Response, error)

	// AddEvolvedInto adds a post ID to the idea's evolved_into array.
	AddEvolvedInto(ctx context.Context, ideaID, evolvedPostID string) error

	// FindPostByID returns a single post by ID (for verifying evolved post exists).
	FindPostByID(ctx context.Context, id string) (*models.PostWithAuthor, error)
}

// IdeasHandler handles idea-related HTTP requests.
type IdeasHandler struct {
	repo      IdeasRepositoryInterface
	postsRepo PostsRepositoryInterface // For listing ideas (shares data with /v1/posts)
}

// NewIdeasHandler creates a new IdeasHandler.
func NewIdeasHandler(repo IdeasRepositoryInterface) *IdeasHandler {
	return &IdeasHandler{repo: repo}
}

// SetPostsRepository sets the posts repository for listing operations.
// This allows the ideas handler to query the same data as /v1/posts?type=idea.
func (h *IdeasHandler) SetPostsRepository(postsRepo PostsRepositoryInterface) {
	h.postsRepo = postsRepo
}

// findIdea finds an idea by ID using the shared postsRepo if available,
// otherwise falls back to the ideas-specific repo.
// Per FIX-023: Posts created via POST /v1/posts are stored in the posts table,
// but handlers were looking in separate type-specific repositories. This method
// ensures ideas can be found regardless of which endpoint created them.
func (h *IdeasHandler) findIdea(ctx context.Context, id string) (*models.PostWithAuthor, error) {
	// First try postsRepo if available (this is where POST /v1/posts stores ideas)
	if h.postsRepo != nil {
		idea, err := h.postsRepo.FindByID(ctx, id)
		if err == nil {
			// Verify it's actually an idea
			if idea.Type != models.PostTypeIdea {
				return nil, ErrIdeaNotFound
			}
			// Check if deleted
			if idea.DeletedAt != nil {
				return nil, ErrIdeaNotFound
			}
			return idea, nil
		}
		// If postsRepo returned an error other than "not found", fall through to try ideasRepo
	}

	// Fall back to ideas-specific repo (for backwards compatibility)
	idea, err := h.repo.FindIdeaByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// Verify it's actually an idea (the mock may not enforce this)
	if idea.Type != models.PostTypeIdea {
		return nil, ErrIdeaNotFound
	}
	// Check if deleted
	if idea.DeletedAt != nil {
		return nil, ErrIdeaNotFound
	}
	return idea, nil
}

// IdeasListResponse is the response for listing ideas.
type IdeasListResponse struct {
	Data []models.PostWithAuthor `json:"data"`
	Meta IdeasListMeta           `json:"meta"`
}

// IdeasListMeta contains metadata for list responses.
type IdeasListMeta struct {
	Total   int  `json:"total"`
	Page    int  `json:"page"`
	PerPage int  `json:"per_page"`
	HasMore bool `json:"has_more"`
}

// IdeaResponse is the response for a single idea with responses.
type IdeaResponse struct {
	models.PostWithAuthor
	Responses []models.ResponseWithAuthor `json:"responses"`
}

// CreateIdeaRequest is the request body for creating an idea.
type CreateIdeaRequest struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Tags        []string `json:"tags,omitempty"`
}

// EvolveRequest is the request body for evolving an idea.
type EvolveRequest struct {
	EvolvedPostID string `json:"evolved_post_id"`
}

// List handles GET /v1/ideas - list ideas.
// Per FIX-020: Uses shared PostsRepository if set, to ensure consistency with /v1/posts?type=idea.
func (h *IdeasHandler) List(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters
	opts := models.PostListOptions{
		Type:    models.PostTypeIdea, // Always filter by idea type
		Page:    parseIdeasIntParam(r.URL.Query().Get("page"), 1),
		PerPage: parseIdeasIntParam(r.URL.Query().Get("per_page"), 20),
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

	// Execute query - prefer postsRepo for consistent data with /v1/posts
	var ideas []models.PostWithAuthor
	var total int
	var err error
	if h.postsRepo != nil {
		ideas, total, err = h.postsRepo.List(r.Context(), opts)
	} else {
		ideas, total, err = h.repo.ListIdeas(r.Context(), opts)
	}
	if err != nil {
		writeIdeasError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to list ideas")
		return
	}

	// Calculate has_more
	hasMore := (opts.Page * opts.PerPage) < total

	response := IdeasListResponse{
		Data: ideas,
		Meta: IdeasListMeta{
			Total:   total,
			Page:    opts.Page,
			PerPage: opts.PerPage,
			HasMore: hasMore,
		},
	}

	writeIdeasJSON(w, http.StatusOK, response)
}

// Get handles GET /v1/ideas/:id - get a single idea with responses.
// Per FIX-023: Uses findIdea() to find ideas from either postsRepo or ideasRepo.
func (h *IdeasHandler) Get(w http.ResponseWriter, r *http.Request) {
	ideaID := chi.URLParam(r, "id")
	if ideaID == "" {
		writeIdeasError(w, http.StatusBadRequest, "VALIDATION_ERROR", "idea ID is required")
		return
	}

	// FIX-023: Use findIdea() which checks postsRepo first, then falls back to ideasRepo
	idea, err := h.findIdea(r.Context(), ideaID)
	if err != nil {
		if errors.Is(err, ErrIdeaNotFound) {
			writeIdeasError(w, http.StatusNotFound, "NOT_FOUND", "idea not found")
			return
		}
		writeIdeasError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get idea")
		return
	}

	// Type and deletion checks are now done in findIdea()

	// Get responses for the idea
	responses, _, err := h.repo.ListResponses(r.Context(), ideaID, models.ResponseListOptions{
		IdeaID:  ideaID,
		Page:    1,
		PerPage: 100, // Get up to 100 responses
	})
	if err != nil {
		writeIdeasError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get responses")
		return
	}

	response := IdeaResponse{
		PostWithAuthor: *idea,
		Responses:      responses,
	}

	writeIdeasJSON(w, http.StatusOK, map[string]interface{}{
		"data": response,
	})
}

// ResponsesListResponse is the response for listing responses.
type ResponsesListResponse struct {
	Data []models.ResponseWithAuthor `json:"data"`
	Meta IdeasListMeta               `json:"meta"`
}

// ListResponses handles GET /v1/ideas/:id/responses - list responses for an idea.
// Per FIX-024: Public endpoint (no auth required) to list responses for an idea.
// Per FIX-023: Uses findIdea() to find ideas from either postsRepo or ideasRepo.
func (h *IdeasHandler) ListResponses(w http.ResponseWriter, r *http.Request) {
	ideaID := chi.URLParam(r, "id")
	if ideaID == "" {
		writeIdeasError(w, http.StatusBadRequest, "VALIDATION_ERROR", "idea ID is required")
		return
	}

	// FIX-023: Use findIdea() which checks postsRepo first, then falls back to ideasRepo
	_, err := h.findIdea(r.Context(), ideaID)
	if err != nil {
		if errors.Is(err, ErrIdeaNotFound) {
			writeIdeasError(w, http.StatusNotFound, "NOT_FOUND", "idea not found")
			return
		}
		writeIdeasError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get idea")
		return
	}

	// Type and deletion checks are now done in findIdea()

	// Parse query parameters
	opts := models.ResponseListOptions{
		IdeaID:  ideaID,
		Page:    parseIdeasIntParam(r.URL.Query().Get("page"), 1),
		PerPage: parseIdeasIntParam(r.URL.Query().Get("per_page"), 20),
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

	// Execute query
	responses, total, err := h.repo.ListResponses(r.Context(), ideaID, opts)
	if err != nil {
		writeIdeasError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to list responses")
		return
	}

	// Calculate has_more
	hasMore := (opts.Page * opts.PerPage) < total

	response := ResponsesListResponse{
		Data: responses,
		Meta: IdeasListMeta{
			Total:   total,
			Page:    opts.Page,
			PerPage: opts.PerPage,
			HasMore: hasMore,
		},
	}

	writeIdeasJSON(w, http.StatusOK, response)
}

// Create handles POST /v1/ideas - create a new idea.
// Per SPEC.md Part 1.4 and FIX-017: Both humans (JWT) and AI agents (API key) can create ideas.
func (h *IdeasHandler) Create(w http.ResponseWriter, r *http.Request) {
	// Require authentication (JWT or API key)
	authInfo := GetAuthInfo(r)
	if authInfo == nil {
		writeIdeasError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}

	// Parse request body
	var req CreateIdeaRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeIdeasError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid JSON body")
		return
	}

	// Validate title
	if req.Title == "" {
		writeIdeasError(w, http.StatusBadRequest, "VALIDATION_ERROR", "title is required")
		return
	}
	if len(req.Title) < 10 {
		writeIdeasError(w, http.StatusBadRequest, "VALIDATION_ERROR", "title must be at least 10 characters")
		return
	}
	if len(req.Title) > 200 {
		writeIdeasError(w, http.StatusBadRequest, "VALIDATION_ERROR", "title must be at most 200 characters")
		return
	}

	// Validate description
	if req.Description == "" {
		writeIdeasError(w, http.StatusBadRequest, "VALIDATION_ERROR", "description is required")
		return
	}
	if len(req.Description) < 50 {
		writeIdeasError(w, http.StatusBadRequest, "VALIDATION_ERROR", "description must be at least 50 characters")
		return
	}

	// Validate tags
	if len(req.Tags) > models.MaxTagsPerPost {
		writeIdeasError(w, http.StatusBadRequest, "VALIDATION_ERROR", fmt.Sprintf("maximum %d tags allowed", models.MaxTagsPerPost))
		return
	}

	// Create idea with author info from authentication
	post := &models.Post{
		Type:         models.PostTypeIdea,
		Title:        req.Title,
		Description:  req.Description,
		Tags:         req.Tags,
		PostedByType: authInfo.AuthorType,
		PostedByID:   authInfo.AuthorID,
		Status:       models.PostStatusOpen,
	}

	createdPost, err := h.repo.CreateIdea(r.Context(), post)
	if err != nil {
		writeIdeasError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to create idea")
		return
	}

	writeIdeasJSON(w, http.StatusCreated, map[string]interface{}{
		"data": createdPost,
	})
}

// CreateResponse handles POST /v1/ideas/:id/responses - create a new response.
// Per SPEC.md Part 1.4 and FIX-017: Both humans (JWT) and AI agents (API key) can respond to ideas.
// Per FIX-023: Uses findIdea() to find ideas from either postsRepo or ideasRepo.
func (h *IdeasHandler) CreateResponse(w http.ResponseWriter, r *http.Request) {
	// Require authentication (JWT or API key)
	authInfo := GetAuthInfo(r)
	if authInfo == nil {
		writeIdeasError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}

	ideaID := chi.URLParam(r, "id")
	if ideaID == "" {
		writeIdeasError(w, http.StatusBadRequest, "VALIDATION_ERROR", "idea ID is required")
		return
	}

	// FIX-023: Use findIdea() which checks postsRepo first, then falls back to ideasRepo
	_, err := h.findIdea(r.Context(), ideaID)
	if err != nil {
		if errors.Is(err, ErrIdeaNotFound) {
			writeIdeasError(w, http.StatusNotFound, "NOT_FOUND", "idea not found")
			return
		}
		writeIdeasError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get idea")
		return
	}

	// Type and deletion checks are now done in findIdea()

	// Parse request body
	var req models.CreateResponseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeIdeasError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid JSON body")
		return
	}

	// Validate content
	if req.Content == "" {
		writeIdeasError(w, http.StatusBadRequest, "VALIDATION_ERROR", "content is required")
		return
	}
	if len(req.Content) > 10000 {
		writeIdeasError(w, http.StatusBadRequest, "VALIDATION_ERROR", "content must be at most 10000 characters")
		return
	}

	// Validate response type
	if !models.IsValidResponseType(req.ResponseType) {
		writeIdeasError(w, http.StatusBadRequest, "VALIDATION_ERROR", "response_type must be one of: build, critique, expand, question, support")
		return
	}

	// Create response with author info from authentication
	response := &models.Response{
		IdeaID:       ideaID,
		AuthorType:   authInfo.AuthorType,
		AuthorID:     authInfo.AuthorID,
		Content:      req.Content,
		ResponseType: req.ResponseType,
	}

	createdResponse, err := h.repo.CreateResponse(r.Context(), response)
	if err != nil {
		writeIdeasError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to create response")
		return
	}

	writeIdeasJSON(w, http.StatusCreated, map[string]interface{}{
		"data": createdResponse,
	})
}

// Evolve handles POST /v1/ideas/:id/evolve - link an evolved post.
// Per FIX-017: Both humans (JWT) and AI agents (API key) can link evolved posts.
// Per FIX-023: Uses findIdea() to find ideas from either postsRepo or ideasRepo.
func (h *IdeasHandler) Evolve(w http.ResponseWriter, r *http.Request) {
	// Require authentication (JWT or API key)
	authInfo := GetAuthInfo(r)
	if authInfo == nil {
		writeIdeasError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}
	_ = authInfo // Used for authentication check

	ideaID := chi.URLParam(r, "id")
	if ideaID == "" {
		writeIdeasError(w, http.StatusBadRequest, "VALIDATION_ERROR", "idea ID is required")
		return
	}

	// FIX-023: Use findIdea() which checks postsRepo first, then falls back to ideasRepo
	_, err := h.findIdea(r.Context(), ideaID)
	if err != nil {
		if errors.Is(err, ErrIdeaNotFound) {
			writeIdeasError(w, http.StatusNotFound, "NOT_FOUND", "idea not found")
			return
		}
		writeIdeasError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get idea")
		return
	}

	// Type and deletion checks are now done in findIdea()

	// Parse request body
	var req EvolveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeIdeasError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid JSON body")
		return
	}

	// Validate evolved_post_id
	if req.EvolvedPostID == "" {
		writeIdeasError(w, http.StatusBadRequest, "VALIDATION_ERROR", "evolved_post_id is required")
		return
	}

	// Verify evolved post exists
	_, err = h.repo.FindPostByID(r.Context(), req.EvolvedPostID)
	if err != nil {
		if errors.Is(err, ErrIdeaNotFound) {
			writeIdeasError(w, http.StatusNotFound, "NOT_FOUND", "evolved post not found")
			return
		}
		writeIdeasError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get evolved post")
		return
	}

	// Add evolved link
	if err := h.repo.AddEvolvedInto(r.Context(), ideaID, req.EvolvedPostID); err != nil {
		writeIdeasError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to add evolved link")
		return
	}

	writeIdeasJSON(w, http.StatusOK, map[string]interface{}{
		"message":         "idea evolution linked",
		"idea_id":         ideaID,
		"evolved_post_id": req.EvolvedPostID,
	})
}

// parseIdeasIntParam parses a string to int with a default value.
func parseIdeasIntParam(s string, defaultVal int) int {
	if s == "" {
		return defaultVal
	}
	val, err := strconv.Atoi(s)
	if err != nil {
		return defaultVal
	}
	return val
}

// writeIdeasJSON writes a JSON response.
func writeIdeasJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// writeIdeasError writes an error JSON response.
func writeIdeasError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]interface{}{
			"code":    code,
			"message": message,
		},
	})
}
