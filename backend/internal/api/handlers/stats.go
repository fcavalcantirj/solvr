// Package handlers provides HTTP handlers for the Solvr API.
package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// StatsRepositoryInterface defines the interface for stats data access.
type StatsRepositoryInterface interface {
	GetActivePostsCount(ctx context.Context) (int, error)
	GetAgentsCount(ctx context.Context) (int, error)
	GetSolvedTodayCount(ctx context.Context) (int, error)
	GetPostedTodayCount(ctx context.Context) (int, error)
	GetProblemsSolvedCount(ctx context.Context) (int, error)
	GetQuestionsAnsweredCount(ctx context.Context) (int, error)
	GetHumansCount(ctx context.Context) (int, error)
	GetTotalPostsCount(ctx context.Context) (int, error)
	GetTotalContributionsCount(ctx context.Context) (int, error)
	GetTrendingPosts(ctx context.Context, limit int) ([]any, error)
	GetTrendingTags(ctx context.Context, limit int) ([]any, error)
	// Problems-specific stats
	GetProblemsStats(ctx context.Context) (map[string]any, error)
	GetRecentlySolvedProblems(ctx context.Context, limit int) ([]map[string]any, error)
	GetTopProblemSolvers(ctx context.Context, limit int) ([]map[string]any, error)
	// Ideas-specific stats
	GetIdeasCountByStatus(ctx context.Context) (map[string]int, error)
	GetFreshSparks(ctx context.Context, limit int) ([]map[string]any, error)
	GetReadyToDevelop(ctx context.Context, limit int) ([]map[string]any, error)
	GetTopSparklers(ctx context.Context, limit int) ([]map[string]any, error)
	GetIdeaPipelineStats(ctx context.Context) (map[string]any, error)
	GetRecentlyRealized(ctx context.Context, limit int) ([]map[string]any, error)
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
	SolvedToday        int `json:"solved_today"`
	PostedToday        int `json:"posted_today"`
	ProblemsSolved     int `json:"problems_solved"`
	QuestionsAnswered  int `json:"questions_answered"`
	HumansCount        int `json:"humans_count"`
	TotalPosts         int `json:"total_posts"`
	TotalContributions int `json:"total_contributions"`
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

	activePosts, err := h.repo.GetActivePostsCount(ctx)
	if err != nil {
		writeStatsError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get active posts count")
		return
	}

	totalAgents, err := h.repo.GetAgentsCount(ctx)
	if err != nil {
		writeStatsError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get agents count")
		return
	}

	solvedToday, err := h.repo.GetSolvedTodayCount(ctx)
	if err != nil {
		writeStatsError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get solved today count")
		return
	}

	postedToday, err := h.repo.GetPostedTodayCount(ctx)
	if err != nil {
		writeStatsError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get posted today count")
		return
	}

	problemsSolved, err := h.repo.GetProblemsSolvedCount(ctx)
	if err != nil {
		writeStatsError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get problems solved count")
		return
	}

	questionsAnswered, err := h.repo.GetQuestionsAnsweredCount(ctx)
	if err != nil {
		writeStatsError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get questions answered count")
		return
	}

	humansCount, err := h.repo.GetHumansCount(ctx)
	if err != nil {
		writeStatsError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get humans count")
		return
	}

	totalPosts, err := h.repo.GetTotalPostsCount(ctx)
	if err != nil {
		writeStatsError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get total posts count")
		return
	}

	totalContributions, err := h.repo.GetTotalContributionsCount(ctx)
	if err != nil {
		writeStatsError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get total contributions count")
		return
	}

	response := map[string]interface{}{
		"data": StatsResponse{
			ActivePosts:        activePosts,
			TotalAgents:        totalAgents,
			SolvedToday:        solvedToday,
			PostedToday:        postedToday,
			ProblemsSolved:     problemsSolved,
			QuestionsAnswered:  questionsAnswered,
			HumansCount:        humansCount,
			TotalPosts:         totalPosts,
			TotalContributions: totalContributions,
		},
	}

	w.Header().Set("Content-Type", "application/json")
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
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// GetProblemsStats handles GET /v1/stats/problems
// Returns statistics for the Problems page sidebar
func (h *StatsHandler) GetProblemsStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	stats, err := h.repo.GetProblemsStats(ctx)
	if err != nil {
		writeStatsError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get problems stats")
		return
	}

	recentlySolved, err := h.repo.GetRecentlySolvedProblems(ctx, 3)
	if err != nil {
		writeStatsError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get recently solved problems")
		return
	}

	topSolvers, err := h.repo.GetTopProblemSolvers(ctx, 5)
	if err != nil {
		writeStatsError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get top problem solvers")
		return
	}

	stats["recently_solved"] = recentlySolved
	stats["top_solvers"] = topSolvers

	response := map[string]interface{}{
		"data": stats,
	}

	w.Header().Set("Content-Type", "application/json")
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

// GetIdeasStats handles GET /v1/stats/ideas
// Returns comprehensive statistics for the Ideas page sidebar
func (h *StatsHandler) GetIdeasStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get counts by status
	countsByStatus, err := h.repo.GetIdeasCountByStatus(ctx)
	if err != nil {
		writeStatsError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get ideas counts")
		return
	}

	// Get fresh sparks (recent ideas)
	freshSparks, err := h.repo.GetFreshSparks(ctx, 5)
	if err != nil {
		writeStatsError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get fresh sparks")
		return
	}

	// Get ready to develop ideas
	readyToDevelop, err := h.repo.GetReadyToDevelop(ctx, 5)
	if err != nil {
		writeStatsError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get ready to develop ideas")
		return
	}

	// Get top sparklers (contributors)
	topSparklers, err := h.repo.GetTopSparklers(ctx, 5)
	if err != nil {
		writeStatsError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get top sparklers")
		return
	}

	// Get trending tags
	trendingTags, err := h.repo.GetTrendingTags(ctx, 10)
	if err != nil {
		writeStatsError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get trending tags")
		return
	}

	// Get pipeline stats
	pipelineStats, err := h.repo.GetIdeaPipelineStats(ctx)
	if err != nil {
		writeStatsError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get pipeline stats")
		return
	}

	// Get recently realized ideas
	recentlyRealized, err := h.repo.GetRecentlyRealized(ctx, 3)
	if err != nil {
		writeStatsError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get recently realized ideas")
		return
	}

	response := map[string]interface{}{
		"data": map[string]interface{}{
			"counts_by_status":  countsByStatus,
			"fresh_sparks":      freshSparks,
			"ready_to_develop":  readyToDevelop,
			"top_sparklers":     topSparklers,
			"trending_tags":     trendingTags,
			"pipeline_stats":    pipelineStats,
			"recently_realized": recentlyRealized,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}
