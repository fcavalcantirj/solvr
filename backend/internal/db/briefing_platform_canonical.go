package db

import (
	"context"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/jackc/pgx/v5"
)

// CanonicalPlatformBriefingRepository serves the platform-wide briefing sections (pulse,
// trending, rising posts, hardcore unsolved, recent victories) from the canonical posts,
// replies, votes and room outcomes (task idx 76 step 3, feature:briefing). Every section
// reads only public, published, approved posts; a contributor reply is a live human or
// agent reply. Retired with the legacy lifecycles: the approach success/failure workflow
// (FailedCount is always 0); the problem-only weight and idea evolution went with the legacy
// post types (idx 68). A recent victory is a published room outcome.
type CanonicalPlatformBriefingRepository struct {
	pool *Pool
}

// NewCanonicalPlatformBriefingRepository creates a CanonicalPlatformBriefingRepository.
func NewCanonicalPlatformBriefingRepository(pool *Pool) *CanonicalPlatformBriefingRepository {
	return &CanonicalPlatformBriefingRepository{pool: pool}
}

const (
	// canonicalPublicPost matches a live post p that anyone may read.
	canonicalPublicPost = `p.deleted_at IS NULL
			AND p.visibility = 'public'
			AND p.publication_state = 'published'
			AND p.moderation_state = 'approved'`

	// canonicalAuthorJoins resolves the display name of post p's author.
	canonicalAuthorJoins = `
		LEFT JOIN users u ON p.posted_by_type = 'human' AND p.posted_by_id = u.id::text
		LEFT JOIN agents ag ON p.posted_by_type = 'agent' AND p.posted_by_id = ag.id`

	canonicalAgeHours = `GREATEST(FLOOR(EXTRACT(EPOCH FROM (NOW() - p.created_at)) / 3600)::int, 0)`
)

// GetPlatformPulse counts public activity. Open posts have status open.
// Contributors are the distinct authors of this week's public posts and of this week's
// contributor replies on public posts.
func (r *CanonicalPlatformBriefingRepository) GetPlatformPulse(ctx context.Context) (*models.PlatformPulse, error) {
	query := `
		WITH public_posts AS (
			SELECT p.status, p.created_at
			FROM posts p
			WHERE ` + canonicalPublicPost + `
		),
		counts AS (
			SELECT
				COUNT(*) FILTER (WHERE status = 'open') AS open_posts,
				COUNT(*) FILTER (WHERE created_at > NOW() - INTERVAL '24 hours') AS new_posts_24h
			FROM public_posts
		),
		contributors AS (
			SELECT p.posted_by_type AS author_type, p.posted_by_id AS author_id
			FROM posts p
			WHERE ` + canonicalPublicPost + `
				AND p.created_at > date_trunc('week', NOW())
			UNION
			SELECT r.author_type, r.author_id
			FROM replies r
			JOIN posts p ON p.id = r.post_id
			WHERE ` + canonicalPublicPost + `
				AND ` + liveContributorReply + `
				AND r.created_at > date_trunc('week', NOW())
		)
		SELECT c.open_posts, c.new_posts_24h,
			(SELECT COUNT(*) FROM agents WHERE last_seen_at > NOW() - INTERVAL '24 hours' AND deleted_at IS NULL),
			(SELECT COUNT(*) FROM contributors),
			(SELECT COUNT(*) FROM blog_posts WHERE status = 'published' AND deleted_at IS NULL)
		FROM counts c`

	p := &models.PlatformPulse{}
	err := r.pool.QueryRow(ctx, query).Scan(
		&p.OpenPosts, &p.NewPostsLast24h,
		&p.ActiveAgentsLast24h, &p.ContributorsThisWeek, &p.BlogPostsPublished,
	)
	if err != nil {
		LogQueryError(ctx, "CanonicalPlatformBriefing.GetPlatformPulse", "posts", err)
		return nil, err
	}
	return p, nil
}

// GetTrendingNow ranks public posts that are not closed by engagement velocity (confirmed
// post votes plus views in the last 7 days), excluding the requesting agent's own posts.
func (r *CanonicalPlatformBriefingRepository) GetTrendingNow(ctx context.Context, excludeAgentID string, limit int) ([]models.TrendingPost, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT p.id::text, p.title, p.type,
			COALESCE(p.upvotes, 0) - COALESCE(p.downvotes, 0) AS vote_score,
			p.view_count,
			COALESCE(`+userPublicName("u")+`, ag.display_name, p.posted_by_id) AS author_name,
			p.posted_by_type,
			`+canonicalAgeHours+` AS age_hours,
			p.tags,
			(
				(SELECT COUNT(*) FROM votes v
				 WHERE v.target_type = 'post' AND v.target_id = p.id
				   AND v.confirmed = true AND v.created_at > NOW() - INTERVAL '7 days')
				+
				(SELECT COUNT(*) FROM post_views pv
				 WHERE pv.post_id = p.id AND pv.viewed_at > NOW() - INTERVAL '7 days')
			) AS engagement_velocity
		FROM posts p`+canonicalAuthorJoins+`
		WHERE `+canonicalPublicPost+`
			AND p.status <> 'closed'
			AND NOT (p.posted_by_type = 'agent' AND p.posted_by_id = $1)
		ORDER BY engagement_velocity DESC, p.created_at DESC, p.id
		LIMIT $2`, excludeAgentID, limit)
	if err != nil {
		LogQueryError(ctx, "CanonicalPlatformBriefing.GetTrendingNow", "posts", err)
		return nil, err
	}
	return collectBriefingRows(ctx, rows, "CanonicalPlatformBriefing.GetTrendingNow", func(row pgx.Rows) (models.TrendingPost, error) {
		var p models.TrendingPost
		var velocity int // ordering only
		err := row.Scan(&p.ID, &p.Title, &p.Type, &p.VoteScore, &p.ViewCount,
			&p.AuthorName, &p.AuthorType, &p.AgeHours, &p.Tags, &velocity)
		p.Tags = nonNilTags(p.Tags)
		return p, err
	})
}

// GetRisingIdeas returns public posts of every type gaining traction: at least one
// contributor reply or upvote, not closed; ranked by contributor replies, then
// upvotes, then recency. ResponseCount carries the contributor reply count.
func (r *CanonicalPlatformBriefingRepository) GetRisingIdeas(ctx context.Context, limit int) ([]models.RisingIdea, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT p.id::text, p.title,
			COUNT(r.id) AS reply_count,
			COALESCE(p.upvotes, 0) AS upvotes,
			`+canonicalAgeHours+` AS age_hours,
			p.tags
		FROM posts p
		LEFT JOIN replies r ON r.post_id = p.id AND `+liveContributorReply+`
		WHERE `+canonicalPublicPost+`
			AND p.status <> 'closed'
		GROUP BY p.id
		HAVING COUNT(r.id) > 0 OR COALESCE(p.upvotes, 0) > 0
		ORDER BY COUNT(r.id) DESC, COALESCE(p.upvotes, 0) DESC, p.created_at DESC, p.id
		LIMIT $1`, limit)
	if err != nil {
		LogQueryError(ctx, "CanonicalPlatformBriefing.GetRisingIdeas", "posts", err)
		return nil, err
	}
	return collectBriefingRows(ctx, rows, "CanonicalPlatformBriefing.GetRisingIdeas", func(row pgx.Rows) (models.RisingIdea, error) {
		var idea models.RisingIdea
		err := row.Scan(&idea.ID, &idea.Title, &idea.ResponseCount, &idea.Upvotes, &idea.AgeHours, &idea.Tags)
		idea.Tags = nonNilTags(idea.Tags)
		return idea, err
	})
}

// GetHardcoreUnsolved returns public posts that are not closed and resist: 3+ contributor
// replies, or older than 30 days with a positive score. Difficulty =
// (1 + replies) * ln(age_days + 2) * (1 + max(score, 0) * 0.5). TotalApproaches carries the
// contributor reply count.
func (r *CanonicalPlatformBriefingRepository) GetHardcoreUnsolved(ctx context.Context, limit int) ([]models.HardcoreUnsolved, error) {
	rows, err := r.pool.Query(ctx, `
		WITH post_stats AS (
			SELECT p.id, p.title, p.tags,
				COALESCE(p.upvotes, 0) - COALESCE(p.downvotes, 0) AS vote_score,
				COUNT(r.id) AS reply_count,
				GREATEST(EXTRACT(EPOCH FROM (NOW() - p.created_at)) / 86400, 0) AS age_days
			FROM posts p
			LEFT JOIN replies r ON r.post_id = p.id AND `+liveContributorReply+`
			WHERE `+canonicalPublicPost+`
				AND p.status <> 'closed'
			GROUP BY p.id
		)
		SELECT id::text, title, reply_count, FLOOR(age_days)::int, tags,
			(1 + reply_count) * ln(age_days + 2) * (1 + GREATEST(vote_score, 0) * 0.5) AS difficulty_score
		FROM post_stats
		WHERE reply_count >= 3
		   OR (age_days > 30 AND vote_score > 0)
		ORDER BY difficulty_score DESC, id
		LIMIT $1`, limit)
	if err != nil {
		LogQueryError(ctx, "CanonicalPlatformBriefing.GetHardcoreUnsolved", "posts", err)
		return nil, err
	}
	return collectBriefingRows(ctx, rows, "CanonicalPlatformBriefing.GetHardcoreUnsolved", func(row pgx.Rows) (models.HardcoreUnsolved, error) {
		var h models.HardcoreUnsolved
		err := row.Scan(&h.ID, &h.Title, &h.TotalApproaches, &h.AgeDays, &h.Tags, &h.DifficultyScore)
		h.Tags = nonNilTags(h.Tags)
		return h, err
	})
}

// GetRecentVictories returns public posts saved from a room (room outcomes) in the last
// 14 days, newest first. The solver is the post's author, TotalApproaches the contributor
// reply count, DaysToSolve the whole days from the room's creation to the outcome (0 when
// the room row is gone) and SolvedAt the outcome's creation time in UTC.
func (r *CanonicalPlatformBriefingRepository) GetRecentVictories(ctx context.Context, limit int) ([]models.RecentVictory, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT p.id::text, p.title,
			COALESCE(`+userPublicName("u")+`, ag.display_name, p.posted_by_id) AS solver_name,
			p.posted_by_type, p.posted_by_id,
			(SELECT COUNT(*) FROM replies r WHERE r.post_id = p.id AND `+liveContributorReply+`) AS reply_count,
			GREATEST(FLOOR(EXTRACT(EPOCH FROM (p.created_at - rm.created_at)) / 86400)::int, 0) AS days_to_solve,
			TO_CHAR(p.created_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"') AS solved_at,
			p.tags
		FROM posts p
		LEFT JOIN rooms rm ON rm.id = p.source_room_id`+canonicalAuthorJoins+`
		WHERE p.source_room_id IS NOT NULL
			AND `+canonicalPublicPost+`
			AND p.created_at > NOW() - INTERVAL '14 days'
		ORDER BY p.created_at DESC, p.id
		LIMIT $1`, limit)
	if err != nil {
		LogQueryError(ctx, "CanonicalPlatformBriefing.GetRecentVictories", "posts", err)
		return nil, err
	}
	return collectBriefingRows(ctx, rows, "CanonicalPlatformBriefing.GetRecentVictories", func(row pgx.Rows) (models.RecentVictory, error) {
		var v models.RecentVictory
		err := row.Scan(&v.ID, &v.Title, &v.SolverName, &v.SolverType, &v.SolverID,
			&v.TotalApproaches, &v.DaysToSolve, &v.SolvedAt, &v.Tags)
		v.Tags = nonNilTags(v.Tags)
		return v, err
	})
}

// collectBriefingRows scans every row with scan and closes rows; it never returns a nil
// slice without an error.
func collectBriefingRows[T any](ctx context.Context, rows pgx.Rows, op string, scan func(pgx.Rows) (T, error)) ([]T, error) {
	defer rows.Close()
	out := []T{}
	for rows.Next() {
		item, err := scan(rows)
		if err != nil {
			LogQueryError(ctx, op+".Scan", "posts", err)
			return nil, err
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		LogQueryError(ctx, op+".Rows", "posts", err)
		return nil, err
	}
	return out, nil
}

func nonNilTags(tags []string) []string {
	if tags == nil {
		return []string{}
	}
	return tags
}
