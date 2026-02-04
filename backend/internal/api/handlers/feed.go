// Package handlers contains HTTP request handlers for the Solvr API.
package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// FeedRepositoryInterface defines the database operations for the feed.
type FeedRepositoryInterface interface {
	// GetRecentActivity returns recent posts and activity.
	// Per SPEC.md Part 5.6: GET /feed - Recent activity
	// Returns posts and answers ordered by created_at DESC.
	GetRecentActivity(ctx context.Context, page, perPage int) ([]models.FeedItem, int, error)

	// GetStuckProblems returns problems that have approaches with status='stuck'.
	// Per SPEC.md Part 5.6: GET /feed/stuck - Problems needing help
	GetStuckProblems(ctx context.Context, page, perPage int) ([]models.FeedItem, int, error)

	// GetUnansweredQuestions returns questions with zero answers.
	// Per SPEC.md Part 5.6: GET /feed/unanswered - Unanswered questions
	GetUnansweredQuestions(ctx context.Context, page, perPage int) ([]models.FeedItem, int, error)
}

// FeedHandler handles feed-related HTTP requests.
type FeedHandler struct {
	repo FeedRepositoryInterface
}

// NewFeedHandler creates a new FeedHandler.
func NewFeedHandler(repo FeedRepositoryInterface) *FeedHandler {
	return &FeedHandler{repo: repo}
}

// parseFeedPagination parses page and per_page query parameters with defaults.
func parseFeedPagination(r *http.Request) (page, perPage int) {
	page = 1
	perPage = 20

	if pageStr := r.URL.Query().Get("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	if perPageStr := r.URL.Query().Get("per_page"); perPageStr != "" {
		if pp, err := strconv.Atoi(perPageStr); err == nil && pp > 0 {
			perPage = pp
		}
	}

	// Cap per_page at 50 per SPEC.md
	if perPage > 50 {
		perPage = 50
	}

	return page, perPage
}

// calculateHasMore determines if there are more pages.
func calculateHasMore(page, perPage, total int) bool {
	return (page * perPage) < total
}

// writeFeedJSON writes a JSON response.
func writeFeedJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// writeFeedError writes an error JSON response.
func writeFeedError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]interface{}{
			"code":    code,
			"message": message,
		},
	})
}

// Feed handles GET /v1/feed - recent activity.
// Per SPEC.md Part 5.6: GET /feed -> Recent activity
// Returns recent posts and answers, union ordered by created_at DESC.
func (h *FeedHandler) Feed(w http.ResponseWriter, r *http.Request) {
	page, perPage := parseFeedPagination(r)

	items, total, err := h.repo.GetRecentActivity(r.Context(), page, perPage)
	if err != nil {
		writeFeedError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get feed")
		return
	}

	// Ensure items is not nil for JSON serialization
	if items == nil {
		items = []models.FeedItem{}
	}

	response := models.FeedResponse{
		Data: items,
		Meta: models.FeedMeta{
			Total:   total,
			Page:    page,
			PerPage: perPage,
			HasMore: calculateHasMore(page, perPage, total),
		},
	}

	writeFeedJSON(w, http.StatusOK, response)
}

// Stuck handles GET /v1/feed/stuck - problems needing help.
// Per SPEC.md Part 5.6: GET /feed/stuck -> Problems needing help
// Returns problems that have approaches with status='stuck'.
func (h *FeedHandler) Stuck(w http.ResponseWriter, r *http.Request) {
	page, perPage := parseFeedPagination(r)

	items, total, err := h.repo.GetStuckProblems(r.Context(), page, perPage)
	if err != nil {
		writeFeedError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get stuck problems")
		return
	}

	// Ensure items is not nil for JSON serialization
	if items == nil {
		items = []models.FeedItem{}
	}

	response := models.FeedResponse{
		Data: items,
		Meta: models.FeedMeta{
			Total:   total,
			Page:    page,
			PerPage: perPage,
			HasMore: calculateHasMore(page, perPage, total),
		},
	}

	writeFeedJSON(w, http.StatusOK, response)
}

// Unanswered handles GET /v1/feed/unanswered - unanswered questions.
// Per SPEC.md Part 5.6: GET /feed/unanswered -> Unanswered questions
// Returns questions with zero answers.
func (h *FeedHandler) Unanswered(w http.ResponseWriter, r *http.Request) {
	page, perPage := parseFeedPagination(r)

	items, total, err := h.repo.GetUnansweredQuestions(r.Context(), page, perPage)
	if err != nil {
		writeFeedError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get unanswered questions")
		return
	}

	// Ensure items is not nil for JSON serialization
	if items == nil {
		items = []models.FeedItem{}
	}

	response := models.FeedResponse{
		Data: items,
		Meta: models.FeedMeta{
			Total:   total,
			Page:    page,
			PerPage: perPage,
			HasMore: calculateHasMore(page, perPage, total),
		},
	}

	writeFeedJSON(w, http.StatusOK, response)
}
