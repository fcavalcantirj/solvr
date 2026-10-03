package db

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
)

// CanonicalStatsRepository serves the public statistics (GET /v1/stats, /v1/stats/trending and
// the overview's community totals) from the canonical posts and replies (task idx 76 step 3).
// It overrides the StatsRepository methods that count contributions; the post, agent, user and
// tag figures are the embedded repository's.
//
// A contribution is a live human or agent reply on a live public post: migrated answers,
// approaches, responses, comments and progress notes and native replies alike, never a system
// verdict. The per-type and solved figures were retired with the legacy post types (idx 68).
type CanonicalStatsRepository struct {
	*StatsRepository
}

// NewCanonicalStatsRepository creates a CanonicalStatsRepository.
func NewCanonicalStatsRepository(pool *Pool) *CanonicalStatsRepository {
	return &CanonicalStatsRepository{StatsRepository: NewStatsRepository(pool)}
}

const (
	// canonicalPublicStatsPost matches a live public post p, the scope of every public statistic.
	canonicalPublicStatsPost = `p.deleted_at IS NULL AND p.visibility = 'public'`

	canonicalContributionCount = `(SELECT COUNT(*) FROM replies r JOIN posts p ON p.id = r.post_id
		WHERE ` + liveContributorReply + ` AND ` + canonicalPublicStatsPost + `)`
)

// GetTotalContributionsCount counts live human and agent replies on live public posts.
func (r *CanonicalStatsRepository) GetTotalContributionsCount(ctx context.Context) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx, `SELECT `+canonicalContributionCount).Scan(&count)
	return count, err
}

// GetAllStats returns the homepage stats in one round-trip; TotalContributions counts replies.
func (r *CanonicalStatsRepository) GetAllStats(ctx context.Context) (*AllStatsResult, error) {
	today := time.Now().Truncate(24 * time.Hour)
	var s AllStatsResult
	err := r.pool.QueryRow(ctx, `
		SELECT
			(SELECT COUNT(*) FROM posts p WHERE status = 'open' AND `+canonicalPublicStatsPost+`),
			(SELECT COUNT(*) FROM agents WHERE status = 'active'),
			(SELECT COUNT(*) FROM posts p WHERE `+canonicalPublicStatsPost+` AND created_at >= $1),
			(SELECT COUNT(*) FROM users),
			(SELECT COUNT(*) FROM posts p WHERE `+canonicalPublicStatsPost+`),
			`+canonicalContributionCount+`,
			(SELECT COUNT(*) FROM posts p WHERE crystallization_cid IS NOT NULL AND `+canonicalPublicStatsPost+`)
	`, today).Scan(
		&s.ActivePosts, &s.TotalAgents, &s.PostedToday, &s.HumansCount,
		&s.TotalPosts, &s.TotalContributions, &s.CrystallizedPosts,
	)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// GetTrendingPosts ranks this week's public posts like the legacy repository does;
// response_count is the post's live contributor replies.
func (r *CanonicalStatsRepository) GetTrendingPosts(ctx context.Context, limit int) ([]any, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT p.id, p.title, p.type,
			COALESCE(p.upvotes - p.downvotes, 0) AS vote_score,
			COALESCE(rc.n, 0) AS response_count,
			p.created_at
		FROM posts p
		LEFT JOIN (
			SELECT r.post_id, COUNT(*) AS n FROM replies r WHERE `+liveContributorReply+` GROUP BY r.post_id
		) rc ON rc.post_id = p.id
		WHERE p.created_at > NOW() - INTERVAL '7 days'
			AND `+canonicalPublicStatsPost+`
			AND p.status NOT IN ('pending_review', 'rejected', 'draft')
		ORDER BY
			LOG(GREATEST(ABS(COALESCE(p.upvotes, 0) - COALESCE(p.downvotes, 0)), 1) + 1)
			+ EXTRACT(EPOCH FROM (p.created_at - (NOW() - INTERVAL '7 days'))) / 45000.0
			DESC
		LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	posts := []any{}
	for _, m := range collectStatsRows(rows, func(row pgx.Rows) (map[string]any, error) {
		var p TrendingPostDB
		err := row.Scan(&p.ID, &p.Title, &p.Type, &p.VoteScore, &p.ResponseCount, &p.CreatedAt)
		return map[string]any{"id": p.ID, "title": p.Title, "type": p.Type, "vote_score": p.VoteScore,
			"response_count": p.ResponseCount, "created_at": p.CreatedAt}, err
	}, &err) {
		posts = append(posts, m)
	}
	return posts, err
}

// collectStatsRows scans every row with scan and closes rows. It returns an empty slice,
// never nil, and stores the first scan or iteration error in *errp.
func collectStatsRows(rows pgx.Rows, scan func(pgx.Rows) (map[string]any, error), errp *error) []map[string]any {
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		m, err := scan(rows)
		if err != nil {
			*errp = err
			return nil
		}
		out = append(out, m)
	}
	*errp = rows.Err()
	return out
}

func orUnknown(s *string) string {
	if s == nil {
		return "unknown"
	}
	return *s
}

func orZero(f *float64) float64 {
	if f == nil {
		return 0
	}
	return *f
}
