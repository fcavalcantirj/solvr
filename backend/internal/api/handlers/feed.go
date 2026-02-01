// Package handlers contains HTTP request handlers for the Solvr API.
package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"
)

// FeedAuthor contains author information for feed items.
type FeedAuthor struct {
	Type        string `json:"type"`
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	AvatarURL   string `json:"avatar_url,omitempty"`
}

// FeedItem represents a single item in the feed.
// Per SPEC.md Part 4.4 - Post cards in feed.
type FeedItem struct {
	// ID is the post UUID.
	ID string `json:"id"`

	// Type is the post type: problem, question, or idea.
	Type string `json:"type"`

	// Title is the post title.
	Title string `json:"title"`

	// Snippet is a short preview of the description.
	Snippet string `json:"snippet"`

	// Tags are the post tags.
	Tags []string `json:"tags,omitempty"`

	// Status is the current post status.
	Status string `json:"status"`

	// Author is the post author information.
	Author FeedAuthor `json:"author"`

	// VoteScore is upvotes minus downvotes.
	VoteScore int `json:"vote_score"`

	// AnswerCount is the number of answers (for questions) or approaches (for problems).
	AnswerCount int `json:"answer_count"`

	// ApproachCount is the number of approaches (for problems).
	ApproachCount int `json:"approach_count,omitempty"`

	// CreatedAt is when the post was created.
	CreatedAt time.Time `json:"created_at"`
}

// FeedMeta contains pagination metadata for feed responses.
type FeedMeta struct {
	Total   int  `json:"total"`
	Page    int  `json:"page"`
	PerPage int  `json:"per_page"`
	HasMore bool `json:"has_more"`
}

// FeedResponse is the response for feed endpoints.
type FeedResponse struct {
	Data []FeedItem `json:"data"`
	Meta FeedMeta   `json:"meta"`
}

// FeedRepositoryInterface defines the database operations for the feed.
type FeedRepositoryInterface interface {
	// GetRecentActivity returns recent posts and activity.
	// Per SPEC.md Part 5.6: GET /feed - Recent activity
	// Returns posts and answers ordered by created_at DESC.
	GetRecentActivity(ctx context.Context, page, perPage int) ([]FeedItem, int, error)

	// GetStuckProblems returns problems that have approaches with status='stuck'.
	// Per SPEC.md Part 5.6: GET /feed/stuck - Problems needing help
	GetStuckProblems(ctx context.Context, page, perPage int) ([]FeedItem, int, error)

	// GetUnansweredQuestions returns questions with zero answers.
	// Per SPEC.md Part 5.6: GET /feed/unanswered - Unanswered questions
	GetUnansweredQuestions(ctx context.Context, page, perPage int) ([]FeedItem, int, error)
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
		items = []FeedItem{}
	}

	response := FeedResponse{
		Data: items,
		Meta: FeedMeta{
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
		items = []FeedItem{}
	}

	response := FeedResponse{
		Data: items,
		Meta: FeedMeta{
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
		items = []FeedItem{}
	}

	response := FeedResponse{
		Data: items,
		Meta: FeedMeta{
			Total:   total,
			Page:    page,
			PerPage: perPage,
			HasMore: calculateHasMore(page, perPage, total),
		},
	}

	writeFeedJSON(w, http.StatusOK, response)
}
