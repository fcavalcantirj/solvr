package db

import (
	"context"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// PlatformBriefingRepository provides queries for platform-wide briefing sections.
type PlatformBriefingRepository struct {
	pool *Pool
}

// NewPlatformBriefingRepository creates a new PlatformBriefingRepository.
func NewPlatformBriefingRepository(pool *Pool) *PlatformBriefingRepository {
	return &PlatformBriefingRepository{pool: pool}
}

// GetPlatformPulse returns global Solvr activity statistics using a single CTE query.
func (r *PlatformBriefingRepository) GetPlatformPulse(ctx context.Context) (*models.PlatformPulse, error) {
	query := `
		WITH open_problems AS (
			SELECT COUNT(*) AS cnt FROM posts
			WHERE type = 'problem' AND status IN ('open', 'in_progress') AND deleted_at IS NULL
		),
		open_questions AS (
			SELECT COUNT(*) AS cnt FROM posts
			WHERE type = 'question' AND status = 'open' AND deleted_at IS NULL
		),
		active_ideas AS (
			SELECT COUNT(*) AS cnt FROM posts
			WHERE type = 'idea' AND status IN ('open', 'active') AND deleted_at IS NULL
		),
		new_posts_24h AS (
			SELECT COUNT(*) AS cnt FROM posts
			WHERE created_at > NOW() - INTERVAL '24 hours' AND deleted_at IS NULL
		),
		solved_7d AS (
			SELECT COUNT(*) AS cnt FROM posts
			WHERE type = 'problem' AND status = 'solved'
			  AND updated_at > NOW() - INTERVAL '7 days' AND deleted_at IS NULL
		),
		active_agents_24h AS (
			SELECT COUNT(*) AS cnt FROM agents
			WHERE last_seen_at > NOW() - INTERVAL '24 hours'
			  AND (deleted_at IS NULL)
		),
		contributors_week AS (
			SELECT COUNT(DISTINCT posted_by_id) AS cnt FROM posts
			WHERE created_at > date_trunc('week', NOW()) AND deleted_at IS NULL
		)
		SELECT
			(SELECT cnt FROM open_problems),
			(SELECT cnt FROM open_questions),
			(SELECT cnt FROM active_ideas),
			(SELECT cnt FROM new_posts_24h),
			(SELECT cnt FROM solved_7d),
			(SELECT cnt FROM active_agents_24h),
			(SELECT cnt FROM contributors_week)`

	p := &models.PlatformPulse{}
	err := r.pool.QueryRow(ctx, query).Scan(
		&p.OpenProblems,
		&p.OpenQuestions,
		&p.ActiveIdeas,
		&p.NewPostsLast24h,
		&p.SolvedLast7d,
		&p.ActiveAgentsLast24h,
		&p.ContributorsThisWeek,
	)
	if err != nil {
		LogQueryError(ctx, "GetPlatformPulse", "posts", err)
		return nil, err
	}
	return p, nil
}
