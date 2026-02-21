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

// ApproachRelationshipsRepositoryInterface defines operations for approach version history.
type ApproachRelationshipsRepositoryInterface interface {
	GetVersionChain(ctx context.Context, approachID string, depth int) (*models.ApproachVersionHistory, error)
	CreateRelationship(ctx context.Context, rel *models.ApproachRelationship) (*models.ApproachRelationship, error)
}

// ProblemsHandler handles problem-related HTTP requests.
type ProblemsHandler struct {
	repo             ProblemsRepositoryInterface
	postsRepo        PostsRepositoryInterface // For listing problems (shares data with /v1/posts)
	relRepo          ApproachRelationshipsRepositoryInterface
	embeddingService EmbeddingServiceInterface
	logger           *slog.Logger
}

// NewProblemsHandler creates a new ProblemsHandler.
func NewProblemsHandler(repo ProblemsRepositoryInterface) *ProblemsHandler {
	return &ProblemsHandler{
		repo:   repo,
		logger: slog.New(slog.NewJSONHandler(os.Stderr, nil)),
	}
}

// SetEmbeddingService sets the embedding service for generating approach embeddings.
// When set, approach creation/update will generate and store embeddings for semantic search.
func (h *ProblemsHandler) SetEmbeddingService(svc EmbeddingServiceInterface) {
	h.embeddingService = svc
}

// SetPostsRepository sets the posts repository for listing operations.
// This allows the problems handler to query the same data as /v1/posts?type=problem.
func (h *ProblemsHandler) SetPostsRepository(postsRepo PostsRepositoryInterface) {
	h.postsRepo = postsRepo
}

// SetApproachRelationshipsRepository sets the repository for approach version history.
func (h *ProblemsHandler) SetApproachRelationshipsRepository(relRepo ApproachRelationshipsRepositoryInterface) {
	h.relRepo = relRepo
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
		if err.Error() == ErrProblemNotFound.Error() {
			return nil, ErrProblemNotFound
		}
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

// CreateProblemRequest is the request body for creating a problem.
type CreateProblemRequest struct {
	Title           string   `json:"title"`
	Description     string   `json:"description"`
	Tags            []string `json:"tags,omitempty"`
	SuccessCriteria []string `json:"success_criteria,omitempty"`
	Weight          *int     `json:"weight,omitempty"`
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
