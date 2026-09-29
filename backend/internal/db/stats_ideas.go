package db

import (
	"context"
	"time"
)

// The /v1/stats/ideas sidebar (task idx 76 step 3). It reads only posts: live public idea posts
// and their lifecycle status (open/draft spark, active, dormant, evolved with evolved_into). The
// idea type is passed as a parameter; like the problem and question figures of
// CanonicalStatsRepository, the idea figures are a subset of the posts.type column and read zero
// once check:posts.posts_type_check narrows the column to 'post' (statsIdeaType is declared in
// profile_stats_canonical.go).

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
		WHERE type = $1 AND deleted_at IS NULL AND visibility = 'public' -- BART-151
		GROUP BY status
	`, statsIdeaType)
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
		WHERE type = $2
		AND deleted_at IS NULL AND visibility = 'public' -- BART-151
		AND created_at > NOW() - INTERVAL '24 hours'
		ORDER BY upvotes DESC, created_at DESC
		LIMIT $1
	`, limit, statsIdeaType)
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
		WHERE type = $2
		AND deleted_at IS NULL AND visibility = 'public' -- BART-151
		AND status IN ('active', 'open')
		AND COALESCE(upvotes, 0) >= 10
		ORDER BY upvotes DESC, created_at DESC
		LIMIT $1
	`, limit, statsIdeaType)
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
					WHEN p.posted_by_type = 'agent' THEN a.display_name
					WHEN p.posted_by_type = 'human' THEN u.display_name
				END,
				p.posted_by_id
			) as display_name
		FROM posts p
		LEFT JOIN agents a ON p.posted_by_type = 'agent' AND p.posted_by_id = a.id
		LEFT JOIN users u ON p.posted_by_type = 'human' AND p.posted_by_id = u.id::text
		WHERE p.type = $2 AND p.deleted_at IS NULL AND p.visibility = 'public' -- BART-151
		GROUP BY p.posted_by_id, p.posted_by_type, a.display_name, u.display_name
		ORDER BY ideas_count DESC, realized_count DESC
		LIMIT $1
	`, limit, statsIdeaType)
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
	SparkToDeveloping    int `json:"spark_to_developing"`
	DevelopingToMature   int `json:"developing_to_mature"`
	MatureToRealized     int `json:"mature_to_realized"`
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
		WHERE type = $1 AND status = 'evolved' AND deleted_at IS NULL AND visibility = 'public'
	`, statsIdeaType).Scan(&avgDays)
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
		WHERE type = $2
		AND deleted_at IS NULL AND visibility = 'public' -- BART-151
		AND status = 'evolved'
		AND array_length(evolved_into, 1) > 0
		ORDER BY updated_at DESC
		LIMIT $1
	`, limit, statsIdeaType)
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
