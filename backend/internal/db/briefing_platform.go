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

// GetRecentVictories returns recently solved problems (within 14 days) with solver info.
// Uses DISTINCT ON to pick the first succeeded approach per problem, then orders by most recent.
func (r *PlatformBriefingRepository) GetRecentVictories(ctx context.Context, limit int) ([]models.RecentVictory, error) {
	query := `
		SELECT * FROM (
			SELECT DISTINCT ON (p.id)
				p.id,
				p.title,
				COALESCE(u.display_name, ag.display_name, a.author_id, 'Unknown') AS solver_name,
				COALESCE(a.author_type, '') AS solver_type,
				COALESCE(a.author_id, '') AS solver_id,
				(SELECT COUNT(*) FROM approaches WHERE problem_id = p.id AND deleted_at IS NULL) AS total_approaches,
				GREATEST(FLOOR(EXTRACT(EPOCH FROM (p.updated_at - p.created_at)) / 86400)::int, 0) AS days_to_solve,
				TO_CHAR(p.updated_at, 'YYYY-MM-DD"T"HH24:MI:SS"Z"') AS solved_at,
				p.tags
			FROM posts p
			LEFT JOIN approaches a ON a.problem_id = p.id AND a.status = 'succeeded' AND a.deleted_at IS NULL
			LEFT JOIN users u ON a.author_type = 'human' AND a.author_id = u.id::text
			LEFT JOIN agents ag ON a.author_type = 'agent' AND a.author_id = ag.id
			WHERE p.type = 'problem'
				AND p.status = 'solved'
				AND p.updated_at > NOW() - INTERVAL '14 days'
				AND p.deleted_at IS NULL
			ORDER BY p.id, a.created_at ASC
		) sub
		ORDER BY solved_at DESC
		LIMIT $1`

	rows, err := r.pool.Query(ctx, query, limit)
	if err != nil {
		LogQueryError(ctx, "GetRecentVictories", "posts", err)
		return nil, err
	}
	defer rows.Close()

	var victories []models.RecentVictory
	for rows.Next() {
		var v models.RecentVictory
		var tags []string
		err := rows.Scan(
			&v.ID, &v.Title, &v.SolverName, &v.SolverType, &v.SolverID,
			&v.TotalApproaches, &v.DaysToSolve, &v.SolvedAt, &tags,
		)
		if err != nil {
			LogQueryError(ctx, "GetRecentVictories.Scan", "posts", err)
			return nil, err
		}
		if tags == nil {
			tags = []string{}
		}
		v.Tags = tags
		victories = append(victories, v)
	}

	if err := rows.Err(); err != nil {
		LogQueryError(ctx, "GetRecentVictories.Rows", "posts", err)
		return nil, err
	}

	if victories == nil {
		victories = []models.RecentVictory{}
	}
	return victories, nil
}
