// Package db provides database operations for the Solvr API.
package db

import (
	"context"
	"time"
)

// StatsRepository provides stats data from the database.
type StatsRepository struct {
	pool *Pool
}

// NewStatsRepository creates a new StatsRepository.
func NewStatsRepository(pool *Pool) *StatsRepository {
	return &StatsRepository{pool: pool}
}

// GetActivePostsCount returns the count of posts with status 'open' or 'active'.
func (r *StatsRepository) GetActivePostsCount(ctx context.Context) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM posts
		WHERE status IN ('open', 'active', 'in_progress')
		AND deleted_at IS NULL AND visibility = 'public' -- BART-151: public stats never count family posts
	`).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

// GetAgentsCount returns the total count of registered agents.
func (r *StatsRepository) GetAgentsCount(ctx context.Context) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM agents 
		WHERE status = 'active'
	`).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

// GetSolvedTodayCount returns the count of posts solved today.
func (r *StatsRepository) GetSolvedTodayCount(ctx context.Context) (int, error) {
	var count int
	today := time.Now().Truncate(24 * time.Hour)
	err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM posts
		WHERE status = 'solved'
		AND deleted_at IS NULL AND visibility = 'public' -- BART-151
		AND updated_at >= $1
	`, today).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

// GetPostedTodayCount returns the count of posts created today.
func (r *StatsRepository) GetPostedTodayCount(ctx context.Context) (int, error) {
	var count int
	today := time.Now().Truncate(24 * time.Hour)
	err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM posts
		WHERE deleted_at IS NULL AND visibility = 'public' -- BART-151
		AND created_at >= $1
	`, today).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

// GetProblemsSolvedCount returns the total count of solved problems.
func (r *StatsRepository) GetProblemsSolvedCount(ctx context.Context) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM posts
		WHERE type = 'problem' AND status = 'solved'
		AND deleted_at IS NULL AND visibility = 'public' -- BART-151
	`).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

// GetQuestionsAnsweredCount returns the count of questions with accepted answers.
func (r *StatsRepository) GetQuestionsAnsweredCount(ctx context.Context) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM posts
		WHERE type = 'question' AND accepted_answer_id IS NOT NULL
		AND deleted_at IS NULL AND visibility = 'public' -- BART-151
	`).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

// GetHumansCount returns the total count of human users.
func (r *StatsRepository) GetHumansCount(ctx context.Context) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM users
	`).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

// GetTotalPostsCount returns the total count of all posts.
func (r *StatsRepository) GetTotalPostsCount(ctx context.Context) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM posts
		WHERE deleted_at IS NULL AND visibility = 'public' -- BART-151
	`).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

// AllStatsResult holds all stats retrieved in a single query.
type AllStatsResult struct {
	ActivePosts        int
	TotalAgents        int
	SolvedToday        int
	PostedToday        int
	ProblemsSolved     int
	QuestionsAnswered  int
	HumansCount        int
	TotalPosts         int
	TotalContributions int
	CrystallizedPosts  int
}

// TrendingPostDB represents a trending post from the database.
type TrendingPostDB struct {
	ID            string
	Title         string
	Type          string
	ResponseCount int
	VoteScore     int
	CreatedAt     time.Time
}

// TrendingTagDB represents a trending tag from the database.
type TrendingTagDB struct {
	Name   string
	Count  int
	Growth int
}

// GetTrendingTags returns trending tags by comparing recent (7d) vs previous (7-14d) usage.
// Growth is calculated as percentage change between periods.
func (r *StatsRepository) GetTrendingTags(ctx context.Context, limit int) ([]any, error) {
	if limit <= 0 {
		limit = 10
	}

	rows, err := r.pool.Query(ctx, `
		WITH recent AS (
			SELECT tag, COUNT(*) as count
			FROM posts, unnest(tags) as tag
			WHERE tags IS NOT NULL AND array_length(tags, 1) > 0
				AND deleted_at IS NULL AND visibility = 'public' -- BART-151
				AND created_at > NOW() - INTERVAL '7 days'
			GROUP BY tag
		),
		previous AS (
			SELECT tag, COUNT(*) as count
			FROM posts, unnest(tags) as tag
			WHERE tags IS NOT NULL AND array_length(tags, 1) > 0
				AND deleted_at IS NULL AND visibility = 'public' -- BART-151
				AND created_at > NOW() - INTERVAL '14 days'
				AND created_at <= NOW() - INTERVAL '7 days'
			GROUP BY tag
		)
		SELECT
			COALESCE(r.tag, p.tag) as name,
			COALESCE(r.count, 0) as count,
			CASE
				WHEN COALESCE(p.count, 0) = 0 THEN
					CASE WHEN COALESCE(r.count, 0) > 0 THEN 100 ELSE 0 END
				ELSE ((COALESCE(r.count, 0) - p.count) * 100) / p.count
			END as growth
		FROM recent r
		FULL OUTER JOIN previous p ON r.tag = p.tag
		WHERE COALESCE(r.count, 0) > 0
		ORDER BY COALESCE(r.count, 0) DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tags []any
	for rows.Next() {
		var name string
		var count int
		var growth int
		if err := rows.Scan(&name, &count, &growth); err != nil {
			return nil, err
		}
		tags = append(tags, map[string]any{
			"name":   name,
			"count":  count,
			"growth": growth,
		})
	}

	if tags == nil {
		tags = []any{}
	}

	return tags, rows.Err()
}

// ========================
// Problems-specific stats
// ========================
