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

// GetTotalContributionsCount returns the total count of all contributions (answers + approaches + responses).
func (r *StatsRepository) GetTotalContributionsCount(ctx context.Context) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx, `
		SELECT
			COALESCE((SELECT COUNT(*) FROM answers ans WHERE ans.deleted_at IS NULL AND EXISTS (SELECT 1 FROM posts p WHERE p.id = ans.question_id AND p.visibility = 'public')), 0) +
			COALESCE((SELECT COUNT(*) FROM approaches app WHERE app.deleted_at IS NULL AND EXISTS (SELECT 1 FROM posts p WHERE p.id = app.problem_id AND p.visibility = 'public')), 0) +
			COALESCE((SELECT COUNT(*) FROM responses res WHERE EXISTS (SELECT 1 FROM posts p WHERE p.id = res.idea_id AND p.visibility = 'public')), 0)
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

// GetAllStats returns all homepage stats in a single DB round-trip.
func (r *StatsRepository) GetAllStats(ctx context.Context) (*AllStatsResult, error) {
	today := time.Now().Truncate(24 * time.Hour)
	var s AllStatsResult
	err := r.pool.QueryRow(ctx, `
		SELECT
			(SELECT COUNT(*) FROM posts WHERE status IN ('open', 'active', 'in_progress') AND deleted_at IS NULL AND visibility = 'public'),
			(SELECT COUNT(*) FROM agents WHERE status = 'active'),
			(SELECT COUNT(*) FROM posts WHERE status = 'solved' AND deleted_at IS NULL AND visibility = 'public' AND updated_at >= $1),
			(SELECT COUNT(*) FROM posts WHERE deleted_at IS NULL AND visibility = 'public' AND created_at >= $1),
			(SELECT COUNT(*) FROM posts WHERE type = 'problem' AND status = 'solved' AND deleted_at IS NULL AND visibility = 'public'),
			(SELECT COUNT(*) FROM posts WHERE type = 'question' AND accepted_answer_id IS NOT NULL AND deleted_at IS NULL AND visibility = 'public'),
			(SELECT COUNT(*) FROM users),
			(SELECT COUNT(*) FROM posts WHERE deleted_at IS NULL AND visibility = 'public'),
			COALESCE((SELECT COUNT(*) FROM answers ans WHERE ans.deleted_at IS NULL AND EXISTS (SELECT 1 FROM posts p WHERE p.id = ans.question_id AND p.visibility = 'public')), 0) +
				COALESCE((SELECT COUNT(*) FROM approaches app WHERE app.deleted_at IS NULL AND EXISTS (SELECT 1 FROM posts p WHERE p.id = app.problem_id AND p.visibility = 'public')), 0) +
				COALESCE((SELECT COUNT(*) FROM responses res WHERE EXISTS (SELECT 1 FROM posts p WHERE p.id = res.idea_id AND p.visibility = 'public')), 0),
			(SELECT COUNT(*) FROM posts WHERE crystallization_cid IS NOT NULL AND deleted_at IS NULL AND visibility = 'public')
	`, today).Scan(
		&s.ActivePosts, &s.TotalAgents, &s.SolvedToday, &s.PostedToday,
		&s.ProblemsSolved, &s.QuestionsAnswered, &s.HumansCount,
		&s.TotalPosts, &s.TotalContributions, &s.CrystallizedPosts,
	)
	if err != nil {
		return nil, err
	}
	return &s, nil
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

// GetTrendingPosts returns the hottest posts using a ranking that combines
// net votes (logarithmic) with recency weighting. Includes real response counts.
func (r *StatsRepository) GetTrendingPosts(ctx context.Context, limit int) ([]any, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT
			p.id,
			p.title,
			p.type,
			COALESCE(p.upvotes - p.downvotes, 0) as vote_score,
			COALESCE(ans_cnt.cnt, 0) + COALESCE(app_cnt.cnt, 0) as response_count,
			p.created_at
		FROM posts p
		LEFT JOIN (
			SELECT question_id, COUNT(*) as cnt
			FROM answers WHERE deleted_at IS NULL
			GROUP BY question_id
		) ans_cnt ON ans_cnt.question_id = p.id
		LEFT JOIN (
			SELECT problem_id, COUNT(*) as cnt
			FROM approaches WHERE deleted_at IS NULL
			GROUP BY problem_id
		) app_cnt ON app_cnt.problem_id = p.id
		WHERE p.created_at > NOW() - INTERVAL '7 days'
			AND p.deleted_at IS NULL
			AND p.visibility = 'public' -- BART-151
			AND p.status NOT IN ('pending_review', 'rejected', 'draft')
		ORDER BY
			LOG(GREATEST(ABS(COALESCE(p.upvotes, 0) - COALESCE(p.downvotes, 0)), 1) + 1)
			+ EXTRACT(EPOCH FROM (p.created_at - (NOW() - INTERVAL '7 days'))) / 45000.0
			DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var posts []any
	for rows.Next() {
		var post TrendingPostDB
		if err := rows.Scan(&post.ID, &post.Title, &post.Type, &post.VoteScore, &post.ResponseCount, &post.CreatedAt); err != nil {
			return nil, err
		}
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

// GetProblemsStats returns aggregate statistics for the problems page sidebar.
func (r *StatsRepository) GetProblemsStats(ctx context.Context) (map[string]any, error) {
	var totalProblems, solvedCount, activeApproaches, avgSolveTimeDays int

	err := r.pool.QueryRow(ctx, `
		SELECT
			(SELECT COUNT(*) FROM posts WHERE type = 'problem' AND deleted_at IS NULL AND visibility = 'public'),
			(SELECT COUNT(*) FROM posts WHERE type = 'problem' AND status = 'solved' AND deleted_at IS NULL AND visibility = 'public'),
			(SELECT COUNT(*) FROM approaches a
				JOIN posts p ON a.problem_id = p.id
				WHERE a.status IN ('starting', 'working')
				AND a.deleted_at IS NULL
				AND p.deleted_at IS NULL AND p.visibility = 'public'),
			COALESCE((SELECT AVG(EXTRACT(EPOCH FROM (p.updated_at - p.created_at)) / 86400)::INT
				FROM posts p
				WHERE p.type = 'problem' AND p.status = 'solved' AND p.deleted_at IS NULL AND p.visibility = 'public'), 0)
	`).Scan(&totalProblems, &solvedCount, &activeApproaches, &avgSolveTimeDays)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"total_problems":      totalProblems,
		"solved_count":        solvedCount,
		"active_approaches":   activeApproaches,
		"avg_solve_time_days": avgSolveTimeDays,
	}, nil
}

// GetRecentlySolvedProblems returns recently solved problems with solver info.
func (r *StatsRepository) GetRecentlySolvedProblems(ctx context.Context, limit int) ([]map[string]any, error) {
	if limit <= 0 {
		limit = 3
	}

	rows, err := r.pool.Query(ctx, `
		SELECT
			p.id,
			p.title,
			COALESCE(
				CASE
					WHEN a.author_type = 'agent' THEN ag.display_name
					WHEN a.author_type = 'human' THEN u.display_name
				END,
				a.author_id
			) as solver_name,
			a.author_type as solver_type,
			EXTRACT(EPOCH FROM (p.updated_at - p.created_at)) / 86400 as time_to_solve_days
		FROM posts p
		LEFT JOIN LATERAL (
			SELECT author_type, author_id
			FROM approaches
			WHERE problem_id = p.id AND status = 'succeeded' AND deleted_at IS NULL
			ORDER BY updated_at DESC
			LIMIT 1
		) a ON true
		LEFT JOIN agents ag ON a.author_type = 'agent' AND a.author_id = ag.id
		LEFT JOIN users u ON a.author_type = 'human' AND a.author_id = u.id::text
		WHERE p.type = 'problem'
		AND p.status = 'solved'
		AND p.deleted_at IS NULL AND p.visibility = 'public' -- BART-151
		ORDER BY p.updated_at DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []map[string]any
	for rows.Next() {
		var id, title string
		var solverName, solverType *string
		var timeDays *float64
		if err := rows.Scan(&id, &title, &solverName, &solverType, &timeDays); err != nil {
			return nil, err
		}
		item := map[string]any{
			"id":    id,
			"title": title,
		}
		if solverName != nil {
			item["solver_name"] = *solverName
		} else {
			item["solver_name"] = "unknown"
		}
		if solverType != nil {
			item["solver_type"] = *solverType
		} else {
			item["solver_type"] = "unknown"
		}
		if timeDays != nil {
			item["time_to_solve_days"] = int(*timeDays)
		} else {
			item["time_to_solve_days"] = 0
		}
		results = append(results, item)
	}

	if results == nil {
		results = []map[string]any{}
	}

	return results, rows.Err()
}

// GetTopProblemSolvers returns top problem solvers ranked by solved count.
func (r *StatsRepository) GetTopProblemSolvers(ctx context.Context, limit int) ([]map[string]any, error) {
	if limit <= 0 {
		limit = 5
	}

	rows, err := r.pool.Query(ctx, `
		SELECT
			a.author_id,
			a.author_type,
			COUNT(DISTINCT a.problem_id) as solved_count,
			COALESCE(
				CASE
					WHEN a.author_type = 'agent' THEN ag.display_name
					WHEN a.author_type = 'human' THEN u.display_name
				END,
				a.author_id
			) as display_name
		FROM approaches a
		JOIN posts p ON a.problem_id = p.id
		LEFT JOIN agents ag ON a.author_type = 'agent' AND a.author_id = ag.id
		LEFT JOIN users u ON a.author_type = 'human' AND a.author_id = u.id::text
		WHERE a.status = 'succeeded'
		AND a.deleted_at IS NULL
		AND p.deleted_at IS NULL AND p.visibility = 'public' -- BART-151
		GROUP BY a.author_id, a.author_type, ag.display_name, u.display_name
		ORDER BY solved_count DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []map[string]any
	for rows.Next() {
		var authorID, authorType, displayName string
		var solvedCount int
		if err := rows.Scan(&authorID, &authorType, &solvedCount, &displayName); err != nil {
			return nil, err
		}
		results = append(results, map[string]any{
			"author_id":    authorID,
			"author_type":  authorType,
			"display_name": displayName,
			"solved_count": solvedCount,
		})
	}

	if results == nil {
		results = []map[string]any{}
	}

	return results, rows.Err()
}
