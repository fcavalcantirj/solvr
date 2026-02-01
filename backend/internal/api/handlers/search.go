// Package handlers contains HTTP request handlers for the Solvr API.
package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// SearchOptions represents all available search filters and options.
type SearchOptions struct {
	Type       string    // Filter by post type (problem, question, idea)
	Tags       []string  // Filter by tags
	Status     string    // Filter by status
	Author     string    // Filter by author_id
	AuthorType string    // Filter by author_type (human, agent)
	FromDate   time.Time // Filter posts created after this date
	ToDate     time.Time // Filter posts created before this date
	Sort       string    // Sort order (relevance, newest, votes, activity)
	Page       int       // Page number (1-indexed)
	PerPage    int       // Results per page
}

// SearchRepositoryInterface defines the database operations for search.
type SearchRepositoryInterface interface {
	// Search performs a full-text search with the given query and options.
	// Returns results, total count, and any error.
	Search(ctx context.Context, query string, opts SearchOptions) ([]models.SearchResult, int, error)
}

// SearchHandler handles search-related HTTP requests.
type SearchHandler struct {
	repo SearchRepositoryInterface
}

// NewSearchHandler creates a new SearchHandler.
func NewSearchHandler(repo SearchRepositoryInterface) *SearchHandler {
	return &SearchHandler{repo: repo}
}

// SearchResponse is the response structure for search results.
type SearchResponse struct {
	Data []models.SearchResultResponse `json:"data"`
	Meta SearchResponseMeta            `json:"meta"`
}

// SearchResponseMeta contains metadata about the search response.
type SearchResponseMeta struct {
	Query   string `json:"query"`
	Total   int    `json:"total"`
	Page    int    `json:"page"`
	PerPage int    `json:"per_page"`
	HasMore bool   `json:"has_more"`
	TookMs  int64  `json:"took_ms"`
}

// Search handles GET /v1/search - search the knowledge base.
// Query params per SPEC.md Part 5.5:
//   - q: search query (required)
//   - type: filter by post type (problem|question|idea)
//   - tags: comma-separated tags
//   - status: filter by status
//   - author: filter by author_id
//   - author_type: filter by author_type (human|agent)
//   - from_date: filter posts after this date (ISO format)
//   - to_date: filter posts before this date (ISO format)
//   - sort: relevance|newest|votes|activity (default: relevance)
//   - page: page number (default: 1)
//   - per_page: results per page (default: 20, max: 50)
func (h *SearchHandler) Search(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	// Parse and validate query parameter
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if query == "" {
		writeSearchError(w, http.StatusBadRequest, "VALIDATION_ERROR", "search query 'q' is required")
		return
	}

	// Parse filters
	opts := SearchOptions{
		Type:       r.URL.Query().Get("type"),
		Status:     r.URL.Query().Get("status"),
		Author:     r.URL.Query().Get("author"),
		AuthorType: r.URL.Query().Get("author_type"),
		Sort:       r.URL.Query().Get("sort"),
	}

	// Parse tags (comma-separated)
	if tagsParam := r.URL.Query().Get("tags"); tagsParam != "" {
		opts.Tags = strings.Split(tagsParam, ",")
		// Trim whitespace from each tag
		for i, tag := range opts.Tags {
			opts.Tags[i] = strings.TrimSpace(tag)
		}
	}

	// Parse date filters
	if fromDate := r.URL.Query().Get("from_date"); fromDate != "" {
		parsed, err := time.Parse("2006-01-02", fromDate)
		if err != nil {
			writeSearchError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid from_date format, use YYYY-MM-DD")
			return
		}
		opts.FromDate = parsed
	}

	if toDate := r.URL.Query().Get("to_date"); toDate != "" {
		parsed, err := time.Parse("2006-01-02", toDate)
		if err != nil {
			writeSearchError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid to_date format, use YYYY-MM-DD")
			return
		}
		opts.ToDate = parsed
	}

	// Set default sort
	if opts.Sort == "" {
		opts.Sort = "relevance"
	}

	// Parse pagination
	opts.Page = parseIntParam(r.URL.Query().Get("page"), 1)
	if opts.Page < 1 {
		opts.Page = 1
	}

	opts.PerPage = parseIntParam(r.URL.Query().Get("per_page"), 20)
	if opts.PerPage < 1 {
		opts.PerPage = 20
	}
	if opts.PerPage > 50 {
		opts.PerPage = 50 // Cap at 50 per SPEC.md
	}

	// Execute search
	results, total, err := h.repo.Search(r.Context(), query, opts)
	if err != nil {
		writeSearchError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "search failed")
		return
	}

	// Convert to response format
	responseData := make([]models.SearchResultResponse, len(results))
	for i, result := range results {
		responseData[i] = result.ToResponse()
	}

	// Calculate has_more
	hasMore := (opts.Page * opts.PerPage) < total

	// Calculate took_ms
	tookMs := time.Since(start).Milliseconds()

	// Build response
	response := SearchResponse{
		Data: responseData,
		Meta: SearchResponseMeta{
			Query:   query,
			Total:   total,
			Page:    opts.Page,
			PerPage: opts.PerPage,
			HasMore: hasMore,
			TookMs:  tookMs,
		},
	}

	writeSearchJSON(w, http.StatusOK, response)
}

// parseIntParam parses a string to int with a default value.
func parseIntParam(s string, defaultVal int) int {
	if s == "" {
		return defaultVal
	}
	val, err := strconv.Atoi(s)
	if err != nil {
		return defaultVal
	}
	return val
}

// writeSearchJSON writes a JSON response.
func writeSearchJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// writeSearchError writes an error JSON response.
func writeSearchError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]interface{}{
			"code":    code,
			"message": message,
		},
	})
}
