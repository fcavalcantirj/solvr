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
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/go-chi/chi/v5"
)

// ProblemsRepositoryInterface defines the database operations for problems.
type ProblemsRepositoryInterface interface {
	// ListProblems returns problems matching the given options.
	ListProblems(ctx context.Context, opts models.PostListOptions) ([]models.PostWithAuthor, int, error)

	// FindProblemByID returns a single problem by ID.
	FindProblemByID(ctx context.Context, id string) (*models.PostWithAuthor, error)

	// CreateProblem creates a new problem and returns it.
	CreateProblem(ctx context.Context, post *models.Post) (*models.Post, error)

	// ListApproaches returns approaches for a problem.
	ListApproaches(ctx context.Context, problemID string, opts models.ApproachListOptions) ([]models.ApproachWithAuthor, int, error)

	// CreateApproach creates a new approach and returns it.
	CreateApproach(ctx context.Context, approach *models.Approach) (*models.Approach, error)

	// FindApproachByID returns a single approach by ID.
	FindApproachByID(ctx context.Context, id string) (*models.ApproachWithAuthor, error)

	// UpdateApproach updates an existing approach and returns it.
	UpdateApproach(ctx context.Context, approach *models.Approach) (*models.Approach, error)

	// AddProgressNote adds a progress note to an approach.
	AddProgressNote(ctx context.Context, note *models.ProgressNote) (*models.ProgressNote, error)

	// GetProgressNotes returns progress notes for an approach.
	GetProgressNotes(ctx context.Context, approachID string) ([]models.ProgressNote, error)

	// UpdateProblemStatus updates the status of a problem.
	UpdateProblemStatus(ctx context.Context, problemID string, status models.PostStatus) error
}

// ProblemsHandler handles problem-related HTTP requests.
type ProblemsHandler struct {
	repo      ProblemsRepositoryInterface
	postsRepo PostsRepositoryInterface // For listing problems (shares data with /v1/posts)
}

// NewProblemsHandler creates a new ProblemsHandler.
func NewProblemsHandler(repo ProblemsRepositoryInterface) *ProblemsHandler {
	return &ProblemsHandler{repo: repo}
}

// SetPostsRepository sets the posts repository for listing operations.
// This allows the problems handler to query the same data as /v1/posts?type=problem.
func (h *ProblemsHandler) SetPostsRepository(postsRepo PostsRepositoryInterface) {
	h.postsRepo = postsRepo
}

// findProblem finds a problem by ID using the shared postsRepo if available,
// otherwise falls back to the problems-specific repo.
// Per FIX-023: Posts created via POST /v1/posts are stored in the posts table,
// but handlers were looking in separate type-specific repositories. This method
// ensures problems can be found regardless of which endpoint created them.
func (h *ProblemsHandler) findProblem(ctx context.Context, id string) (*models.PostWithAuthor, error) {
	// First try postsRepo if available (this is where POST /v1/posts stores problems)
	if h.postsRepo != nil {
		problem, err := h.postsRepo.FindByID(ctx, id)
		if err == nil {
			// Verify it's actually a problem
			if problem.Type != models.PostTypeProblem {
				return nil, ErrProblemNotFound
			}
			// Check if deleted
			if problem.DeletedAt != nil {
				return nil, ErrProblemNotFound
			}
			return problem, nil
		}
		// If postsRepo returned an error other than "not found", fall through to try problemsRepo
	}

	// Fall back to problems-specific repo (for backwards compatibility)
	problem, err := h.repo.FindProblemByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// Verify it's actually a problem (the mock may not enforce this)
	if problem.Type != models.PostTypeProblem {
		return nil, ErrProblemNotFound
	}
	// Check if deleted
	if problem.DeletedAt != nil {
		return nil, ErrProblemNotFound
	}
	return problem, nil
}

// ProblemsListResponse is the response for listing problems.
type ProblemsListResponse struct {
	Data []models.PostWithAuthor `json:"data"`
	Meta ProblemsListMeta        `json:"meta"`
}

// ProblemsListMeta contains metadata for list responses.
type ProblemsListMeta struct {
	Total   int  `json:"total"`
	Page    int  `json:"page"`
	PerPage int  `json:"per_page"`
	HasMore bool `json:"has_more"`
}

// ProblemResponse is the response for a single problem.
type ProblemResponse struct {
	Data models.PostWithAuthor `json:"data"`
}

// ApproachesListResponse is the response for listing approaches.
type ApproachesListResponse struct {
	Data []models.ApproachWithAuthor `json:"data"`
	Meta ProblemsListMeta            `json:"meta"`
}

// CreateProblemRequest is the request body for creating a problem.
type CreateProblemRequest struct {
	Title           string   `json:"title"`
	Description     string   `json:"description"`
	Tags            []string `json:"tags,omitempty"`
	SuccessCriteria []string `json:"success_criteria,omitempty"`
	Weight          *int     `json:"weight,omitempty"`
}

// ProgressNoteRequest is the request body for adding a progress note.
type ProgressNoteRequest struct {
	Content string `json:"content"`
}

// VerifyApproachRequest is the request body for verifying an approach.
type VerifyApproachRequest struct {
	Verified bool `json:"verified"`
}

// List handles GET /v1/problems - list problems.
// Per FIX-020: Uses shared PostsRepository if set, to ensure consistency with /v1/posts?type=problem.
func (h *ProblemsHandler) List(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters
	opts := models.PostListOptions{
		Type:    models.PostTypeProblem, // Always filter by problem type
		Page:    parseProblemsIntParam(r.URL.Query().Get("page"), 1),
		PerPage: parseProblemsIntParam(r.URL.Query().Get("per_page"), 20),
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

	// Parse sort parameter
	if sortParam := r.URL.Query().Get("sort"); sortParam != "" {
		switch sortParam {
		case "newest", "votes", "approaches":
			opts.Sort = sortParam
		}
		// Invalid values are silently ignored (defaults to newest)
	}

	// Parse tags filter
	if tagsParam := r.URL.Query().Get("tags"); tagsParam != "" {
		opts.Tags = strings.Split(tagsParam, ",")
		for i, tag := range opts.Tags {
			opts.Tags[i] = strings.TrimSpace(tag)
		}
	}

	// Execute query - prefer postsRepo for consistent data with /v1/posts
	var problems []models.PostWithAuthor
	var total int
	var err error
	if h.postsRepo != nil {
		problems, total, err = h.postsRepo.List(r.Context(), opts)
	} else {
		problems, total, err = h.repo.ListProblems(r.Context(), opts)
	}
	if err != nil {
		writeProblemsError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to list problems")
		return
	}

	// Calculate has_more
	hasMore := (opts.Page * opts.PerPage) < total

	response := ProblemsListResponse{
		Data: problems,
		Meta: ProblemsListMeta{
			Total:   total,
			Page:    opts.Page,
			PerPage: opts.PerPage,
			HasMore: hasMore,
		},
	}

	writeProblemsJSON(w, http.StatusOK, response)
}

// Get handles GET /v1/problems/:id - get a single problem.
// Per FIX-023: Uses findProblem() to find problems from either postsRepo or problemsRepo.
func (h *ProblemsHandler) Get(w http.ResponseWriter, r *http.Request) {
	problemID := chi.URLParam(r, "id")
	if problemID == "" {
		writeProblemsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "problem ID is required")
		return
	}

	// FIX-023: Use findProblem() which checks postsRepo first, then falls back to problemsRepo
	problem, err := h.findProblem(r.Context(), problemID)
	if err != nil {
		if errors.Is(err, ErrProblemNotFound) {
			writeProblemsError(w, http.StatusNotFound, "NOT_FOUND", "problem not found")
			return
		}
		writeProblemsError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get problem")
		return
	}

	// Type and deletion checks are now done in findProblem()
	writeProblemsJSON(w, http.StatusOK, ProblemResponse{Data: *problem})
}

// Create handles POST /v1/problems - create a new problem.
// Per SPEC.md Part 1.4 and FIX-016: Both humans (JWT) and AI agents (API key) can create problems.
func (h *ProblemsHandler) Create(w http.ResponseWriter, r *http.Request) {
	// Require authentication (JWT or API key)
	authInfo := GetAuthInfo(r)
	if authInfo == nil {
		writeProblemsError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}

	// Parse request body
	var req CreateProblemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeProblemsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid JSON body")
		return
	}

	// Validate title
	if req.Title == "" {
		writeProblemsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "title is required")
		return
	}
	if len(req.Title) < 10 {
		writeProblemsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "title must be at least 10 characters")
		return
	}
	if len(req.Title) > 200 {
		writeProblemsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "title must be at most 200 characters")
		return
	}

	// Validate description
	if req.Description == "" {
		writeProblemsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "description is required")
		return
	}
	if len(req.Description) < 50 {
		writeProblemsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "description must be at least 50 characters")
		return
	}

	// Validate tags
	if len(req.Tags) > models.MaxTagsPerPost {
		writeProblemsError(w, http.StatusBadRequest, "VALIDATION_ERROR", fmt.Sprintf("maximum %d tags allowed", models.MaxTagsPerPost))
		return
	}

	// Validate problem-specific fields
	if req.Weight != nil && (*req.Weight < 1 || *req.Weight > 5) {
		writeProblemsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "weight must be between 1 and 5")
		return
	}
	if len(req.SuccessCriteria) > 10 {
		writeProblemsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "maximum 10 success criteria allowed")
		return
	}

	// Create problem with author info from authentication
	post := &models.Post{
		Type:            models.PostTypeProblem,
		Title:           req.Title,
		Description:     req.Description,
		Tags:            req.Tags,
		PostedByType:    authInfo.AuthorType,
		PostedByID:      authInfo.AuthorID,
		Status:          models.PostStatusOpen,
		SuccessCriteria: req.SuccessCriteria,
		Weight:          req.Weight,
	}

	createdPost, err := h.repo.CreateProblem(r.Context(), post)
	if err != nil {
		writeProblemsError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to create problem")
		return
	}

	writeProblemsJSON(w, http.StatusCreated, map[string]interface{}{
		"data": createdPost,
	})
}

// ListApproaches handles GET /v1/problems/:id/approaches - list approaches for a problem.
// Per FIX-023: Uses findProblem() to find problems from either postsRepo or problemsRepo.
func (h *ProblemsHandler) ListApproaches(w http.ResponseWriter, r *http.Request) {
	problemID := chi.URLParam(r, "id")
	if problemID == "" {
		writeProblemsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "problem ID is required")
		return
	}

	// FIX-023: Use findProblem() which checks postsRepo first, then falls back to problemsRepo
	_, err := h.findProblem(r.Context(), problemID)
	if err != nil {
		if errors.Is(err, ErrProblemNotFound) {
			writeProblemsError(w, http.StatusNotFound, "NOT_FOUND", "problem not found")
			return
		}
		writeProblemsError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get problem")
		return
	}

	// Type and deletion checks are now done in findProblem()

	// Parse query parameters
	opts := models.ApproachListOptions{
		ProblemID: problemID,
		Page:      parseProblemsIntParam(r.URL.Query().Get("page"), 1),
		PerPage:   parseProblemsIntParam(r.URL.Query().Get("per_page"), 20),
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

	// Execute query
	approaches, total, err := h.repo.ListApproaches(r.Context(), problemID, opts)
	if err != nil {
		writeProblemsError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to list approaches")
		return
	}

	// Populate progress notes for each approach
	for i := range approaches {
		notes, err := h.repo.GetProgressNotes(r.Context(), approaches[i].ID)
		if err != nil {
			// Log error but don't fail - progress notes are optional
			continue
		}
		approaches[i].ProgressNotes = notes
	}

	// Calculate has_more
	hasMore := (opts.Page * opts.PerPage) < total

	response := ApproachesListResponse{
		Data: approaches,
		Meta: ProblemsListMeta{
			Total:   total,
			Page:    opts.Page,
			PerPage: opts.PerPage,
			HasMore: hasMore,
		},
	}

	writeProblemsJSON(w, http.StatusOK, response)
}

// CreateApproach handles POST /v1/problems/:id/approaches - create a new approach.
// Per SPEC.md Part 1.4 and FIX-016: Both humans (JWT) and AI agents (API key) can start approaches.
// Per FIX-023: Uses findProblem() to find problems from either postsRepo or problemsRepo.
func (h *ProblemsHandler) CreateApproach(w http.ResponseWriter, r *http.Request) {
	// Require authentication (JWT or API key)
	authInfo := GetAuthInfo(r)
	if authInfo == nil {
		writeProblemsError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}

	problemID := chi.URLParam(r, "id")
	if problemID == "" {
		writeProblemsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "problem ID is required")
		return
	}

	// FIX-023: Use findProblem() which checks postsRepo first, then falls back to problemsRepo
	_, err := h.findProblem(r.Context(), problemID)
	if err != nil {
		if errors.Is(err, ErrProblemNotFound) {
			writeProblemsError(w, http.StatusNotFound, "NOT_FOUND", "problem not found")
			return
		}
		writeProblemsError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get problem")
		return
	}

	// Type and deletion checks are now done in findProblem()

	// Parse request body
	var req models.CreateApproachRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeProblemsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid JSON body")
		return
	}

	// Validate angle
	if req.Angle == "" {
		writeProblemsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "angle is required")
		return
	}
	if len(req.Angle) > 500 {
		writeProblemsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "angle must be at most 500 characters")
		return
	}

	// Validate method
	if len(req.Method) > 500 {
		writeProblemsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "method must be at most 500 characters")
		return
	}

	// Validate assumptions (max 10)
	if len(req.Assumptions) > 10 {
		writeProblemsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "maximum 10 assumptions allowed")
		return
	}

	// Create approach with author info from authentication
	approach := &models.Approach{
		ProblemID:   problemID,
		AuthorType:  authInfo.AuthorType,
		AuthorID:    authInfo.AuthorID,
		Angle:       req.Angle,
		Method:      req.Method,
		Assumptions: req.Assumptions,
		DiffersFrom: req.DiffersFrom,
		Status:      models.ApproachStatusStarting,
	}

	createdApproach, err := h.repo.CreateApproach(r.Context(), approach)
	if err != nil {
		writeProblemsError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to create approach")
		return
	}

	writeProblemsJSON(w, http.StatusCreated, map[string]interface{}{
		"data": createdApproach,
	})
}

// UpdateApproach handles PATCH /v1/approaches/:id - update an approach.
// Per FIX-016: Both humans (JWT) and AI agents (API key) can update their approaches.
func (h *ProblemsHandler) UpdateApproach(w http.ResponseWriter, r *http.Request) {
	// Require authentication (JWT or API key)
	authInfo := GetAuthInfo(r)
	if authInfo == nil {
		writeProblemsError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}

	approachID := chi.URLParam(r, "id")
	if approachID == "" {
		writeProblemsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "approach ID is required")
		return
	}

	// Get existing approach
	existingApproach, err := h.repo.FindApproachByID(r.Context(), approachID)
	if err != nil {
		if errors.Is(err, ErrApproachNotFound) {
			writeProblemsError(w, http.StatusNotFound, "NOT_FOUND", "approach not found")
			return
		}
		writeProblemsError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get approach")
		return
	}

	// Check ownership - only author can update (works for both humans and agents)
	if existingApproach.AuthorType != authInfo.AuthorType || existingApproach.AuthorID != authInfo.AuthorID {
		writeProblemsError(w, http.StatusForbidden, "FORBIDDEN", "you can only update your own approaches")
		return
	}

	// Parse request body
	var req models.UpdateApproachRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeProblemsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid JSON body")
		return
	}

	// Apply updates
	updatedApproach := existingApproach.Approach

	if req.Status != nil {
		newStatus := models.ApproachStatus(*req.Status)
		if !models.IsValidApproachStatus(newStatus) {
			writeProblemsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid status")
			return
		}
		updatedApproach.Status = newStatus
	}

	if req.Outcome != nil {
		if len(*req.Outcome) > 10000 {
			writeProblemsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "outcome must be at most 10000 characters")
			return
		}
		updatedApproach.Outcome = *req.Outcome
	}

	if req.Method != nil {
		if len(*req.Method) > 500 {
			writeProblemsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "method must be at most 500 characters")
			return
		}
		updatedApproach.Method = *req.Method
	}

	result, err := h.repo.UpdateApproach(r.Context(), &updatedApproach)
	if err != nil {
		writeProblemsError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to update approach")
		return
	}

	writeProblemsJSON(w, http.StatusOK, map[string]interface{}{
		"data": result,
	})
}

// AddProgressNote handles POST /v1/approaches/:id/progress - add a progress note.
// Per FIX-016: Both humans (JWT) and AI agents (API key) can add progress notes.
func (h *ProblemsHandler) AddProgressNote(w http.ResponseWriter, r *http.Request) {
	// Require authentication (JWT or API key)
	authInfo := GetAuthInfo(r)
	if authInfo == nil {
		writeProblemsError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}

	approachID := chi.URLParam(r, "id")
	if approachID == "" {
		writeProblemsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "approach ID is required")
		return
	}

	// Get existing approach
	existingApproach, err := h.repo.FindApproachByID(r.Context(), approachID)
	if err != nil {
		if errors.Is(err, ErrApproachNotFound) {
			writeProblemsError(w, http.StatusNotFound, "NOT_FOUND", "approach not found")
			return
		}
		writeProblemsError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get approach")
		return
	}

	// Check ownership - only author can add progress notes (works for both humans and agents)
	if existingApproach.AuthorType != authInfo.AuthorType || existingApproach.AuthorID != authInfo.AuthorID {
		writeProblemsError(w, http.StatusForbidden, "FORBIDDEN", "you can only add progress notes to your own approaches")
		return
	}

	// Parse request body
	var req ProgressNoteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeProblemsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid JSON body")
		return
	}

	// Validate content
	if req.Content == "" {
		writeProblemsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "content is required")
		return
	}

	// Create progress note
	note := &models.ProgressNote{
		ApproachID: approachID,
		Content:    req.Content,
	}

	createdNote, err := h.repo.AddProgressNote(r.Context(), note)
	if err != nil {
		writeProblemsError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to add progress note")
		return
	}

	writeProblemsJSON(w, http.StatusCreated, map[string]interface{}{
		"data": createdNote,
	})
}

// VerifyApproach handles POST /v1/approaches/:id/verify - verify an approach solution.
// Per FIX-016: Both humans (JWT) and AI agents (API key) who own the problem can verify.
func (h *ProblemsHandler) VerifyApproach(w http.ResponseWriter, r *http.Request) {
	// Require authentication (JWT or API key)
	authInfo := GetAuthInfo(r)
	if authInfo == nil {
		writeProblemsError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}

	approachID := chi.URLParam(r, "id")
	if approachID == "" {
		writeProblemsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "approach ID is required")
		return
	}

	// Get existing approach
	approach, err := h.repo.FindApproachByID(r.Context(), approachID)
	if err != nil {
		if errors.Is(err, ErrApproachNotFound) {
			writeProblemsError(w, http.StatusNotFound, "NOT_FOUND", "approach not found")
			return
		}
		writeProblemsError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get approach")
		return
	}

	// Get the problem to verify ownership
	// FIX-024: Use findProblem() which checks postsRepo first, then falls back to problemsRepo
	problem, err := h.findProblem(r.Context(), approach.ProblemID)
	if err != nil {
		if errors.Is(err, ErrProblemNotFound) {
			writeProblemsError(w, http.StatusNotFound, "NOT_FOUND", "problem not found")
			return
		}
		writeProblemsError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get problem")
		return
	}

	// Check ownership - only problem owner can verify (works for both humans and agents)
	if problem.PostedByType != authInfo.AuthorType || problem.PostedByID != authInfo.AuthorID {
		writeProblemsError(w, http.StatusForbidden, "FORBIDDEN", "only the problem owner can verify approaches")
		return
	}

	// Parse request body
	var req VerifyApproachRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeProblemsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid JSON body")
		return
	}

	// If verified and approach succeeded, update problem status to solved
	if req.Verified && approach.Status == models.ApproachStatusSucceeded {
		// FIX-025: Try postsRepo first (where most problems are stored), then fall back to problemsRepo
		var updateErr error
		if h.postsRepo != nil {
			// Update via postsRepo.Update (requires full post object)
			updatedPost := models.Post{
				ID:           problem.ID,
				Type:         problem.Type,
				Title:        problem.Title,
				Description:  problem.Description,
				Tags:         problem.Tags,
				PostedByType: problem.PostedByType,
				PostedByID:   problem.PostedByID,
				Status:       models.PostStatusSolved,
				Upvotes:      problem.Upvotes,
				Downvotes:    problem.Downvotes,
				ViewCount:    problem.ViewCount,
				CreatedAt:    problem.CreatedAt,
			}
			_, updateErr = h.postsRepo.Update(r.Context(), &updatedPost)
		}
		if updateErr != nil || h.postsRepo == nil {
			// Fall back to problemsRepo
			updateErr = h.repo.UpdateProblemStatus(r.Context(), problem.ID, models.PostStatusSolved)
		}
		if updateErr != nil {
			writeProblemsError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to update problem status")
			return
		}
	}

	writeProblemsJSON(w, http.StatusOK, map[string]interface{}{
		"message":  "approach verified",
		"verified": req.Verified,
	})
}

// parseProblemsIntParam parses a string to int with a default value.
func parseProblemsIntParam(s string, defaultVal int) int {
	if s == "" {
		return defaultVal
	}
	val, err := strconv.Atoi(s)
	if err != nil {
		return defaultVal
	}
	return val
}

// ProblemExportResponse is the response for GET /v1/problems/:id/export.
type ProblemExportResponse struct {
	Markdown      string `json:"markdown"`
	TokenEstimate int    `json:"token_estimate"`
}

// Export handles GET /v1/problems/:id/export - export problem as LLM-friendly markdown.
// This is a public endpoint (same auth as viewing the problem).
func (h *ProblemsHandler) Export(w http.ResponseWriter, r *http.Request) {
	problemID := chi.URLParam(r, "id")
	if problemID == "" {
		writeProblemsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "problem ID is required")
		return
	}

	// Get problem
	problem, err := h.findProblem(r.Context(), problemID)
	if err != nil {
		if errors.Is(err, ErrProblemNotFound) {
			writeProblemsError(w, http.StatusNotFound, "NOT_FOUND", "problem not found")
			return
		}
		writeProblemsError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get problem")
		return
	}

	// Get ALL approaches (high limit to fetch all)
	opts := models.ApproachListOptions{
		ProblemID: problemID,
		Page:      1,
		PerPage:   1000, // High limit to get all approaches
	}
	approaches, _, err := h.repo.ListApproaches(r.Context(), problemID, opts)
	if err != nil {
		writeProblemsError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get approaches")
		return
	}

	// Get progress notes for each approach
	for i := range approaches {
		notes, err := h.repo.GetProgressNotes(r.Context(), approaches[i].ID)
		if err != nil {
			continue // Non-fatal, just skip notes
		}
		approaches[i].ProgressNotes = notes
	}

	// Generate markdown
	markdown := generateProblemExportMarkdown(problem, approaches)
	tokenEstimate := len(markdown) / 4 // Rough estimate: ~4 chars per token

	writeProblemsJSON(w, http.StatusOK, ProblemExportResponse{
		Markdown:      markdown,
		TokenEstimate: tokenEstimate,
	})
}

// generateProblemExportMarkdown creates LLM-friendly markdown export.
func generateProblemExportMarkdown(problem *models.PostWithAuthor, approaches []models.ApproachWithAuthor) string {
	var sb strings.Builder

	// Header
	sb.WriteString(fmt.Sprintf("# Problem: %s\n", problem.Title))
	sb.WriteString(fmt.Sprintf("**Status:** %s | **Posted:** %s | **By:** %s\n",
		strings.ToUpper(string(problem.Status)),
		problem.CreatedAt.Format("2006-01-02"),
		problem.Author.DisplayName,
	))
	if len(problem.Tags) > 0 {
		sb.WriteString(fmt.Sprintf("**Tags:** %s\n", strings.Join(problem.Tags, ", ")))
	}
	sb.WriteString(fmt.Sprintf("**URL:** https://solvr.dev/problems/%s\n\n", problem.ID))

	// Description
	sb.WriteString("## Description\n")
	sb.WriteString(problem.Description)
	sb.WriteString("\n\n---\n\n")

	// Approaches
	sb.WriteString(fmt.Sprintf("## Approaches (%d)\n\n", len(approaches)))

	// Count stats
	succeeded, failed, inProgress := 0, 0, 0
	var lastActivity time.Time

	for i, approach := range approaches {
		sb.WriteString(fmt.Sprintf("### Approach %d: %s\n", i+1, approach.Angle))
		sb.WriteString(fmt.Sprintf("**Status:** %s | **By:** %s | **Created:** %s\n\n",
			strings.ToUpper(string(approach.Status)),
			approach.Author.DisplayName,
			approach.CreatedAt.Format("2006-01-02"),
		))

		if approach.Method != "" {
			sb.WriteString("**Method:**\n")
			sb.WriteString(approach.Method)
			sb.WriteString("\n\n")
		}

		if len(approach.Assumptions) > 0 {
			sb.WriteString("**Assumptions:**\n")
			for _, a := range approach.Assumptions {
				sb.WriteString(fmt.Sprintf("- %s\n", a))
			}
			sb.WriteString("\n")
		}

		if approach.Outcome != "" {
			sb.WriteString("**Outcome:**\n")
			sb.WriteString(approach.Outcome)
			sb.WriteString("\n\n")
		}

		if len(approach.ProgressNotes) > 0 {
			sb.WriteString("**Progress Notes:**\n\n")
			for j, note := range approach.ProgressNotes {
				sb.WriteString(fmt.Sprintf("#### Note %d (%s)\n", j+1, note.CreatedAt.Format("2006-01-02")))
				sb.WriteString(note.Content)
				sb.WriteString("\n\n")
				if note.CreatedAt.After(lastActivity) {
					lastActivity = note.CreatedAt
				}
			}
		}

		// Track stats
		switch approach.Status {
		case models.ApproachStatusSucceeded:
			succeeded++
		case models.ApproachStatusFailed:
			failed++
		default:
			inProgress++
		}
		if approach.UpdatedAt.After(lastActivity) {
			lastActivity = approach.UpdatedAt
		}

		sb.WriteString("---\n\n")
	}

	// Summary
	sb.WriteString("## Summary\n")
	sb.WriteString(fmt.Sprintf("- Total approaches: %d\n", len(approaches)))
	sb.WriteString(fmt.Sprintf("- Succeeded: %d | Failed: %d | In Progress: %d\n", succeeded, failed, inProgress))
	if !lastActivity.IsZero() {
		sb.WriteString(fmt.Sprintf("- Last activity: %s\n", lastActivity.Format("2006-01-02")))
	}

	return sb.String()
}

// writeProblemsJSON writes a JSON response.
func writeProblemsJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// writeProblemsError writes an error JSON response.
func writeProblemsError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]interface{}{
			"code":    code,
			"message": message,
		},
	})
}
