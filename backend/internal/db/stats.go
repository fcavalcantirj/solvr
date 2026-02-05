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
		AND updated_at >= $1
	`, today).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
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

// GetTrendingPosts returns the most active posts (by votes + responses).
func (r *StatsRepository) GetTrendingPosts(ctx context.Context, limit int) ([]any, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT 
			p.id, 
			p.title, 
			p.type,
			COALESCE(p.upvotes - p.downvotes, 0) as vote_score,
			p.created_at
		FROM posts p
		WHERE p.created_at > NOW() - INTERVAL '7 days'
		ORDER BY (COALESCE(p.upvotes, 0) + COALESCE(p.downvotes, 0)) DESC, p.created_at DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var posts []any
	for rows.Next() {
		var post TrendingPostDB
		if err := rows.Scan(&post.ID, &post.Title, &post.Type, &post.VoteScore, &post.CreatedAt); err != nil {
			return nil, err
		}
		// TODO: Add response count when we have that data
		post.ResponseCount = 0
		posts = append(posts, map[string]any{
			"id":             post.ID,
			"title":          post.Title,
			"type":           post.Type,
			"vote_score":     post.VoteScore,
			"response_count": post.ResponseCount,
			"created_at":     post.CreatedAt,
		})
	}

	if posts == nil {
		posts = []any{}
	}

	return posts, rows.Err()
}

// GetTrendingTags returns the most used tags.
// Note: This is a placeholder - tags table doesn't exist yet.
// Returns empty list until tags are implemented.
func (r *StatsRepository) GetTrendingTags(ctx context.Context, limit int) ([]any, error) {
	// TODO: Implement when tags table exists
	// For now, return empty list
	return []any{}, nil
}
