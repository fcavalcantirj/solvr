// Package handlers provides HTTP handlers for the Solvr API.
package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/fcavalcantirj/solvr/internal/db"
)

// StatsRepositoryInterface defines the interface for stats data access.
type StatsRepositoryInterface interface {
	GetAllStats(ctx context.Context) (*db.AllStatsResult, error)
	GetActivePostsCount(ctx context.Context) (int, error)
	GetAgentsCount(ctx context.Context) (int, error)
	GetPostedTodayCount(ctx context.Context) (int, error)
	GetHumansCount(ctx context.Context) (int, error)
	GetTotalPostsCount(ctx context.Context) (int, error)
	GetTotalContributionsCount(ctx context.Context) (int, error)
	GetTrendingPosts(ctx context.Context, limit int) ([]any, error)
	GetTrendingTags(ctx context.Context, limit int) ([]any, error)
}

// StatsHandler handles statistics endpoints.
type StatsHandler struct {
	repo StatsRepositoryInterface
}

// NewStatsHandler creates a new StatsHandler.
func NewStatsHandler(repo StatsRepositoryInterface) *StatsHandler {
	return &StatsHandler{repo: repo}
}

// StatsResponse represents the response for GET /v1/stats
type StatsResponse struct {
	ActivePosts        int `json:"active_posts"`
	TotalAgents        int `json:"total_agents"`
	PostedToday        int `json:"posted_today"`
	HumansCount        int `json:"humans_count"`
	TotalPosts         int `json:"total_posts"`
	TotalContributions int `json:"total_contributions"`
	CrystallizedPosts  int `json:"crystallized_posts"`
}

// TrendingResponse represents the response for GET /v1/stats/trending
type TrendingResponse struct {
	Posts []TrendingPost `json:"posts"`
	Tags  []TrendingTag  `json:"tags"`
}

// TrendingPost represents a trending post for the sidebar
type TrendingPost struct {
	ID            string    `json:"id"`
	Title         string    `json:"title"`
	Type          string    `json:"type"`
	ResponseCount int       `json:"response_count"`
	VoteScore     int       `json:"vote_score"`
	CreatedAt     time.Time `json:"created_at,omitempty"`
}

// TrendingTag represents a trending tag
type TrendingTag struct {
	Name   string `json:"name"`
	Count  int    `json:"count"`
	Growth int    `json:"growth"` // percentage growth
}

// GetStats handles GET /v1/stats
func (h *StatsHandler) GetStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	s, err := h.repo.GetAllStats(ctx)
	if err != nil {
		writeStatsError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get stats")
		return
	}

	response := map[string]interface{}{
		"data": StatsResponse{
			ActivePosts:        s.ActivePosts,
			TotalAgents:        s.TotalAgents,
			PostedToday:        s.PostedToday,
			HumansCount:        s.HumansCount,
			TotalPosts:         s.TotalPosts,
			TotalContributions: s.TotalContributions,
			CrystallizedPosts:  s.CrystallizedPosts,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=30")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// GetTrending handles GET /v1/stats/trending
func (h *StatsHandler) GetTrending(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	posts, err := h.repo.GetTrendingPosts(ctx, 5)
	if err != nil {
		writeStatsError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get trending posts")
		return
	}

	tags, err := h.repo.GetTrendingTags(ctx, 10)
	if err != nil {
		writeStatsError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get trending tags")
		return
	}

	response := map[string]interface{}{
		"data": map[string]interface{}{
			"posts": posts,
			"tags":  tags,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=30")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

func writeStatsError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]string{
			"code":    code,
			"message": message,
		},
	})
}
