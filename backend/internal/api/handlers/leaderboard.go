package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// LeaderboardRepositoryInterface defines the database operations for leaderboard.
type LeaderboardRepositoryInterface interface {
	GetLeaderboard(ctx context.Context, opts models.LeaderboardOptions) ([]models.LeaderboardEntry, int, error)
	GetLeaderboardByTag(ctx context.Context, tag string, opts models.LeaderboardOptions) ([]models.LeaderboardEntry, int, error)
}

// LeaderboardHandler handles leaderboard HTTP requests.
type LeaderboardHandler struct {
	repo LeaderboardRepositoryInterface
}

// NewLeaderboardHandler creates a new LeaderboardHandler.
func NewLeaderboardHandler(repo LeaderboardRepositoryInterface) *LeaderboardHandler {
	return &LeaderboardHandler{
		repo: repo,
	}
}

// LeaderboardResponse is the API response for leaderboard endpoints.
type LeaderboardResponse struct {
	Data []models.LeaderboardEntry `json:"data"`
	Meta LeaderboardMeta           `json:"meta"`
}

// LeaderboardMeta contains pagination metadata.
type LeaderboardMeta struct {
	Total   int  `json:"total"`
	Page    int  `json:"page"`
	PerPage int  `json:"per_page"`
	HasMore bool `json:"has_more"`
}

// GetLeaderboard handles GET /v1/leaderboard.
// Query params: type (all|agents|users), timeframe (all_time|monthly|weekly), limit, offset
func (h *LeaderboardHandler) GetLeaderboard(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters
	opts := models.LeaderboardOptions{
		Type:      r.URL.Query().Get("type"),
		Timeframe: r.URL.Query().Get("timeframe"),
		Limit:     50, // Default limit
		Offset:    0,  // Default offset
	}

	// Default values
	if opts.Type == "" {
		opts.Type = "all"
	}
	if opts.Timeframe == "" {
		opts.Timeframe = "all_time"
	}

	// Parse limit
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil {
			if limit > 0 && limit <= 100 {
				opts.Limit = limit
			}
		}
	}

	// Parse offset
	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if offset, err := strconv.Atoi(offsetStr); err == nil && offset >= 0 {
			opts.Offset = offset
		}
	}

	// Fetch leaderboard data
	entries, total, err := h.repo.GetLeaderboard(r.Context(), opts)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to fetch leaderboard")
		return
	}

	// Build response
	page := (opts.Offset / opts.Limit) + 1
	hasMore := (opts.Offset + len(entries)) < total

	response := LeaderboardResponse{
		Data: entries,
		Meta: LeaderboardMeta{
			Total:   total,
			Page:    page,
			PerPage: opts.Limit,
			HasMore: hasMore,
		},
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// GetLeaderboardByTag handles GET /v1/leaderboard/tags/{tag}.
// Query params: type (all|agents|users), timeframe (all_time|monthly|weekly), limit, offset
func (h *LeaderboardHandler) GetLeaderboardByTag(w http.ResponseWriter, r *http.Request) {
	// Extract tag from URL path parameter
	tag := r.PathValue("tag")
	if tag == "" {
		writeError(w, http.StatusBadRequest, "INVALID_TAG", "tag parameter is required")
		return
	}

	// Parse query parameters (identical to GetLeaderboard)
	opts := models.LeaderboardOptions{
		Type:      r.URL.Query().Get("type"),
		Timeframe: r.URL.Query().Get("timeframe"),
		Limit:     50, // Default limit
		Offset:    0,  // Default offset
	}

	// Default values
	if opts.Type == "" {
		opts.Type = "all"
	}
	if opts.Timeframe == "" {
		opts.Timeframe = "all_time"
	}

	// Parse limit
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil {
			if limit > 0 && limit <= 100 {
				opts.Limit = limit
			}
		}
	}

	// Parse offset
	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if offset, err := strconv.Atoi(offsetStr); err == nil && offset >= 0 {
			opts.Offset = offset
		}
	}

	// Call repository
	entries, total, err := h.repo.GetLeaderboardByTag(r.Context(), tag, opts)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to fetch leaderboard")
		return
	}

	// Build response (identical to GetLeaderboard)
	page := (opts.Offset / opts.Limit) + 1
	hasMore := (opts.Offset + len(entries)) < total

	response := LeaderboardResponse{
		Data: entries,
		Meta: LeaderboardMeta{
			Total:   total,
			Page:    page,
			PerPage: opts.Limit,
			HasMore: hasMore,
		},
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// writeError writes a JSON error response.
func writeError(w http.ResponseWriter, status int, code, message string) {
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]string{
			"code":    code,
			"message": message,
		},
	})
}
