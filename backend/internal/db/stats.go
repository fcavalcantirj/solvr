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

// GetPostedTodayCount returns the count of posts created today.
func (r *StatsRepository) GetPostedTodayCount(ctx context.Context) (int, error) {
	var count int
	today := time.Now().Truncate(24 * time.Hour)
	err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM posts
		WHERE created_at >= $1
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
			COALESCE((SELECT COUNT(*) FROM answers), 0) +
			COALESCE((SELECT COUNT(*) FROM approaches), 0) +
			COALESCE((SELECT COUNT(*) FROM responses), 0)
	`).Scan(&count)
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

// GetTrendingPosts returns the hottest posts using a ranking that combines
// net votes (logarithmic) with recency weighting. Includes real response counts.
func (r *StatsRepository) GetTrendingPosts(ctx context.Context, limit int) ([]any, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT
			p.id,
			p.title,
			p.type,
			COALESCE(p.upvotes - p.downvotes, 0) as vote_score,
			(SELECT COUNT(*) FROM answers a2 WHERE a2.question_id = p.id AND a2.deleted_at IS NULL)
			+ (SELECT COUNT(*) FROM approaches ap2 WHERE ap2.problem_id = p.id AND ap2.deleted_at IS NULL) as response_count,
			p.created_at
		FROM posts p
		WHERE p.created_at > NOW() - INTERVAL '7 days'
			AND p.deleted_at IS NULL
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
				AND deleted_at IS NULL
				AND created_at > NOW() - INTERVAL '7 days'
			GROUP BY tag
		),
		previous AS (
			SELECT tag, COUNT(*) as count
			FROM posts, unnest(tags) as tag
			WHERE tags IS NOT NULL AND array_length(tags, 1) > 0
				AND deleted_at IS NULL
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
// Ideas-specific stats
// ========================

// IdeaStatusCount represents count of ideas per status.
type IdeaStatusCount struct {
	Status string `json:"status"`
	Count  int    `json:"count"`
}

// GetIdeasCountByStatus returns count of ideas grouped by status.
func (r *StatsRepository) GetIdeasCountByStatus(ctx context.Context) (map[string]int, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT status, COUNT(*) as count
		FROM posts
		WHERE type = 'idea' AND deleted_at IS NULL
		GROUP BY status
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := make(map[string]int)
	var total int
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, err
		}
		counts[status] = count
		total += count
	}

	counts["total"] = total

	return counts, rows.Err()
}

// FreshSparkDB represents a recently created idea.
type FreshSparkDB struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Support   int       `json:"support"`
	CreatedAt time.Time `json:"created_at"`
}

// GetFreshSparks returns ideas created in the last 24 hours, sorted by votes.
func (r *StatsRepository) GetFreshSparks(ctx context.Context, limit int) ([]map[string]any, error) {
	if limit <= 0 {
		limit = 5
	}

	rows, err := r.pool.Query(ctx, `
		SELECT id, title, COALESCE(upvotes, 0) as support, created_at
		FROM posts
		WHERE type = 'idea'
		AND deleted_at IS NULL
		AND created_at > NOW() - INTERVAL '24 hours'
		ORDER BY upvotes DESC, created_at DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sparks []map[string]any
	for rows.Next() {
		var id, title string
		var support int
		var createdAt time.Time
		if err := rows.Scan(&id, &title, &support, &createdAt); err != nil {
			return nil, err
		}
		sparks = append(sparks, map[string]any{
			"id":         id,
			"title":      title,
			"support":    support,
			"created_at": createdAt,
		})
	}

	if sparks == nil {
		sparks = []map[string]any{}
	}

	return sparks, rows.Err()
}

// GetReadyToDevelop returns active ideas with high vote scores.
func (r *StatsRepository) GetReadyToDevelop(ctx context.Context, limit int) ([]map[string]any, error) {
	if limit <= 0 {
		limit = 5
	}

	rows, err := r.pool.Query(ctx, `
		SELECT
			id,
			title,
			COALESCE(upvotes, 0) as support,
			CASE
				WHEN COALESCE(upvotes, 0) + COALESCE(downvotes, 0) = 0 THEN 0
				ELSE ROUND((COALESCE(upvotes, 0)::NUMERIC / (COALESCE(upvotes, 0) + COALESCE(downvotes, 0))) * 100)
			END as validation_score
		FROM posts
		WHERE type = 'idea'
		AND deleted_at IS NULL
		AND status IN ('active', 'open')
		AND COALESCE(upvotes, 0) >= 10
		ORDER BY upvotes DESC, created_at DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ideas []map[string]any
	for rows.Next() {
		var id, title string
		var support int
		var validationScore float64
		if err := rows.Scan(&id, &title, &support, &validationScore); err != nil {
			return nil, err
		}
		ideas = append(ideas, map[string]any{
			"id":               id,
			"title":            title,
			"support":          support,
			"validation_score": int(validationScore),
		})
	}

	if ideas == nil {
		ideas = []map[string]any{}
	}

	return ideas, rows.Err()
}

// TopSparklerDB represents a top idea contributor.
type TopSparklerDB struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Type          string `json:"type"`
	IdeasCount    int    `json:"ideas_count"`
	RealizedCount int    `json:"realized_count"`
}

// GetTopSparklers returns users/agents who have created the most ideas.
func (r *StatsRepository) GetTopSparklers(ctx context.Context, limit int) ([]map[string]any, error) {
	if limit <= 0 {
		limit = 5
	}

	rows, err := r.pool.Query(ctx, `
		SELECT
			p.posted_by_id,
			p.posted_by_type,
			COUNT(*) as ideas_count,
			COUNT(*) FILTER (WHERE p.status = 'evolved') as realized_count,
			COALESCE(
				CASE
					WHEN p.posted_by_type = 'agent' THEN a.name
					WHEN p.posted_by_type = 'human' THEN u.display_name
				END,
				p.posted_by_id
			) as display_name
		FROM posts p
		LEFT JOIN agents a ON p.posted_by_type = 'agent' AND p.posted_by_id = a.id
		LEFT JOIN users u ON p.posted_by_type = 'human' AND p.posted_by_id = u.id::text
		WHERE p.type = 'idea' AND p.deleted_at IS NULL
		GROUP BY p.posted_by_id, p.posted_by_type, a.name, u.display_name
		ORDER BY ideas_count DESC, realized_count DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sparklers []map[string]any
	for rows.Next() {
		var id, authorType, name string
		var ideasCount, realizedCount int
		if err := rows.Scan(&id, &authorType, &ideasCount, &realizedCount, &name); err != nil {
			return nil, err
		}
		sparklers = append(sparklers, map[string]any{
			"id":             id,
			"name":           name,
			"type":           authorType,
			"ideas_count":    ideasCount,
			"realized_count": realizedCount,
		})
	}

	if sparklers == nil {
		sparklers = []map[string]any{}
	}

	return sparklers, rows.Err()
}

// IdeaPipelineStats represents conversion statistics for idea lifecycle.
type IdeaPipelineStats struct {
	SparkToDeveloping   int `json:"spark_to_developing"`
	DevelopingToMature  int `json:"developing_to_mature"`
	MatureToRealized    int `json:"mature_to_realized"`
	AvgDaysToRealization int `json:"avg_days_to_realization"`
}

// GetIdeaPipelineStats returns conversion rate statistics for ideas.
// Note: This is an approximation based on current status counts.
func (r *StatsRepository) GetIdeaPipelineStats(ctx context.Context) (map[string]any, error) {
	// Get counts by status
	counts, err := r.GetIdeasCountByStatus(ctx)
	if err != nil {
		return nil, err
	}

	spark := counts["open"] + counts["draft"]
	developing := counts["active"]
	mature := counts["dormant"] // Using dormant as a proxy for mature
	evolved := counts["evolved"]

	// Calculate conversion rates (as percentages)
	sparkToDev := 0
	if spark > 0 {
		sparkToDev = (developing * 100) / (spark + developing)
	}

	devToMature := 0
	if developing > 0 {
		devToMature = (mature * 100) / (developing + mature)
	}

	matureToRealized := 0
	if mature > 0 {
		matureToRealized = (evolved * 100) / (mature + evolved)
	}

	// Average days to realization (simplified: avg age of evolved ideas)
	var avgDays int
	err = r.pool.QueryRow(ctx, `
		SELECT COALESCE(AVG(EXTRACT(EPOCH FROM (updated_at - created_at)) / 86400)::INT, 0)
		FROM posts
		WHERE type = 'idea' AND status = 'evolved' AND deleted_at IS NULL
	`).Scan(&avgDays)
	if err != nil {
		avgDays = 0
	}

	return map[string]any{
		"spark_to_developing":     sparkToDev,
		"developing_to_mature":    devToMature,
		"mature_to_realized":      matureToRealized,
		"avg_days_to_realization": avgDays,
	}, nil
}

// GetRecentlyRealized returns ideas that have evolved into other posts.
func (r *StatsRepository) GetRecentlyRealized(ctx context.Context, limit int) ([]map[string]any, error) {
	if limit <= 0 {
		limit = 5
	}

	rows, err := r.pool.Query(ctx, `
		SELECT
			id,
			title,
			evolved_into[1] as evolved_post_id
		FROM posts
		WHERE type = 'idea'
		AND deleted_at IS NULL
		AND status = 'evolved'
		AND array_length(evolved_into, 1) > 0
		ORDER BY updated_at DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ideas []map[string]any
	for rows.Next() {
		var id, title string
		var evolvedPostID *string
		if err := rows.Scan(&id, &title, &evolvedPostID); err != nil {
			return nil, err
		}
		idea := map[string]any{
			"id":    id,
			"title": title,
		}
		if evolvedPostID != nil {
			idea["evolved_into"] = *evolvedPostID
		}
		ideas = append(ideas, idea)
	}

	if ideas == nil {
		ideas = []map[string]any{}
	}

	return ideas, rows.Err()
}
