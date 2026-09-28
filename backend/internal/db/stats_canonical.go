package db

import (
	"context"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/jackc/pgx/v5"
)

// CanonicalStatsRepository serves the public statistics (GET /v1/stats, /v1/stats/trending,
// /v1/stats/problems, /v1/stats/questions and the overview's community totals) from the
// canonical posts and replies (task idx 76 step 3). It overrides only the StatsRepository
// methods that read the legacy contribution tables; the post, agent, user, tag, idea and
// knowledge figures are the embedded repository's.
//
// A contribution is a live human or agent reply on a live public post: migrated answers,
// approaches, responses, comments and progress notes and native replies alike, never a system
// verdict. Retired with the approach status workflow: active_approaches is always 0. A solved
// problem's solver is the author of its latest reply migrated from a succeeded approach (the
// status is kept in provenance). A question's answers are its top-level contributor replies;
// the accepted one is the post's accepted reply. The problem and question figures are subsets
// of the posts.type column, passed as parameters: they read 0 once check:posts.posts_type_check
// narrows the column to 'post'.
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

	// canonicalSucceededApproachReply matches a reply r migrated from a succeeded approach.
	canonicalSucceededApproachReply = `r.deleted_at IS NULL AND r.legacy_type = 'approach' AND r.provenance->>'status' = 'succeeded'`

	// canonicalAnswerReply matches a reply r that answers its post: top-level, live, not a verdict.
	canonicalAnswerReply = `r.parent_reply_id IS NULL AND ` + liveContributorReply

	// canonicalReplyAuthorJoins and canonicalReplyAuthorName resolve reply r's author.
	canonicalReplyAuthorJoins = `
		LEFT JOIN agents ag ON r.author_type = 'agent' AND r.author_id = ag.id
		LEFT JOIN users u ON r.author_type = 'human' AND r.author_id = u.id::text`
	canonicalReplyAuthorName = `COALESCE(CASE WHEN r.author_type = 'agent' THEN ag.display_name
		WHEN r.author_type = 'human' THEN u.display_name END, r.author_id)`
)

var (
	statsProblemType  = string(models.PostTypeProblem)
	statsQuestionType = string(models.PostTypeQuestion)
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
			(SELECT COUNT(*) FROM posts p WHERE status IN ('open', 'active', 'in_progress') AND `+canonicalPublicStatsPost+`),
			(SELECT COUNT(*) FROM agents WHERE status = 'active'),
			(SELECT COUNT(*) FROM posts p WHERE status = 'solved' AND `+canonicalPublicStatsPost+` AND updated_at >= $1),
			(SELECT COUNT(*) FROM posts p WHERE `+canonicalPublicStatsPost+` AND created_at >= $1),
			(SELECT COUNT(*) FROM posts p WHERE p.type = $2 AND status = 'solved' AND `+canonicalPublicStatsPost+`),
			(SELECT COUNT(*) FROM posts p WHERE p.type = $3 AND accepted_answer_id IS NOT NULL AND `+canonicalPublicStatsPost+`),
			(SELECT COUNT(*) FROM users),
			(SELECT COUNT(*) FROM posts p WHERE `+canonicalPublicStatsPost+`),
			`+canonicalContributionCount+`,
			(SELECT COUNT(*) FROM posts p WHERE crystallization_cid IS NOT NULL AND `+canonicalPublicStatsPost+`)
	`, today, statsProblemType, statsQuestionType).Scan(
		&s.ActivePosts, &s.TotalAgents, &s.SolvedToday, &s.PostedToday,
		&s.ProblemsSolved, &s.QuestionsAnswered, &s.HumansCount,
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

// GetProblemsStats returns the problems sidebar aggregates. active_approaches is retired
// with the approach status workflow and is always 0.
func (r *CanonicalStatsRepository) GetProblemsStats(ctx context.Context) (map[string]any, error) {
	var total, solved, avgSolveDays int
	err := r.pool.QueryRow(ctx, `
		SELECT
			(SELECT COUNT(*) FROM posts p WHERE p.type = $1 AND `+canonicalPublicStatsPost+`),
			(SELECT COUNT(*) FROM posts p WHERE p.type = $1 AND p.status = 'solved' AND `+canonicalPublicStatsPost+`),
			COALESCE((SELECT AVG(EXTRACT(EPOCH FROM (p.updated_at - p.created_at)) / 86400)::INT
				FROM posts p WHERE p.type = $1 AND p.status = 'solved' AND `+canonicalPublicStatsPost+`), 0)
	`, statsProblemType).Scan(&total, &solved, &avgSolveDays)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"total_problems":      total,
		"solved_count":        solved,
		"active_approaches":   0,
		"avg_solve_time_days": avgSolveDays,
	}, nil
}

// GetRecentlySolvedProblems returns the latest solved public problems; the solver is the
// author of the problem's latest reply migrated from a succeeded approach, else "unknown".
func (r *CanonicalStatsRepository) GetRecentlySolvedProblems(ctx context.Context, limit int) ([]map[string]any, error) {
	if limit <= 0 {
		limit = 3
	}
	rows, err := r.pool.Query(ctx, `
		SELECT p.id, p.title, `+canonicalReplyAuthorName+`, r.author_type,
			EXTRACT(EPOCH FROM (p.updated_at - p.created_at)) / 86400
		FROM posts p
		LEFT JOIN LATERAL (
			SELECT r.author_type, r.author_id FROM replies r
			WHERE r.post_id = p.id AND `+canonicalSucceededApproachReply+`
			ORDER BY r.updated_at DESC
			LIMIT 1
		) r ON true`+canonicalReplyAuthorJoins+`
		WHERE p.type = $2 AND p.status = 'solved' AND `+canonicalPublicStatsPost+`
		ORDER BY p.updated_at DESC
		LIMIT $1`, limit, statsProblemType)
	if err != nil {
		return nil, err
	}
	out := collectStatsRows(rows, func(row pgx.Rows) (map[string]any, error) {
		var id, title string
		var name, authorType *string
		var days *float64
		err := row.Scan(&id, &title, &name, &authorType, &days)
		return map[string]any{"id": id, "title": title, "solver_name": orUnknown(name),
			"solver_type": orUnknown(authorType), "time_to_solve_days": int(orZero(days))}, err
	}, &err)
	return out, err
}

// GetTopProblemSolvers ranks authors by the distinct live public problems where a reply of
// theirs migrated from a succeeded approach.
func (r *CanonicalStatsRepository) GetTopProblemSolvers(ctx context.Context, limit int) ([]map[string]any, error) {
	if limit <= 0 {
		limit = 5
	}
	rows, err := r.pool.Query(ctx, `
		SELECT r.author_id, r.author_type, COUNT(DISTINCT r.post_id) AS solved_count, `+canonicalReplyAuthorName+`
		FROM replies r
		JOIN posts p ON p.id = r.post_id`+canonicalReplyAuthorJoins+`
		WHERE `+canonicalSucceededApproachReply+` AND `+canonicalPublicStatsPost+`
		GROUP BY r.author_id, r.author_type, ag.display_name, u.display_name
		ORDER BY solved_count DESC
		LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	out := collectStatsRows(rows, func(row pgx.Rows) (map[string]any, error) {
		var authorID, authorType, name string
		var solved int
		err := row.Scan(&authorID, &authorType, &solved, &name)
		return map[string]any{"author_id": authorID, "author_type": authorType,
			"display_name": name, "solved_count": solved}, err
	}, &err)
	return out, err
}

// GetQuestionsStats returns the questions sidebar aggregates; the response time is the
// average time to a question's first top-level contributor reply.
func (r *CanonicalStatsRepository) GetQuestionsStats(ctx context.Context) (map[string]any, error) {
	var total, answered int
	var responseRate, avgHours float64
	err := r.pool.QueryRow(ctx, `
		SELECT
			(SELECT COUNT(*) FROM posts p WHERE p.type = $1 AND `+canonicalPublicStatsPost+`),
			(SELECT COUNT(*) FROM posts p WHERE p.type = $1 AND p.accepted_answer_id IS NOT NULL AND `+canonicalPublicStatsPost+`),
			COALESCE((SELECT CASE WHEN COUNT(*) = 0 THEN 0
					ELSE (COUNT(*) FILTER (WHERE p.accepted_answer_id IS NOT NULL)::float / COUNT(*)::float) * 100 END
				FROM posts p WHERE p.type = $1 AND `+canonicalPublicStatsPost+`), 0),
			COALESCE((SELECT AVG(EXTRACT(EPOCH FROM (fr.created_at - p.created_at)) / 3600)
				FROM posts p
				JOIN LATERAL (
					SELECT r.created_at FROM replies r
					WHERE r.post_id = p.id AND `+canonicalAnswerReply+`
					ORDER BY r.created_at ASC
					LIMIT 1
				) fr ON true
				WHERE p.type = $1 AND `+canonicalPublicStatsPost+`), 0)
	`, statsQuestionType).Scan(&total, &answered, &responseRate, &avgHours)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"total_questions":         total,
		"answered_count":          answered,
		"response_rate":           responseRate,
		"avg_response_time_hours": avgHours,
	}, nil
}

// GetRecentlyAnsweredQuestions returns the latest public questions with an accepted reply,
// named by that reply's author.
func (r *CanonicalStatsRepository) GetRecentlyAnsweredQuestions(ctx context.Context, limit int) ([]map[string]any, error) {
	if limit <= 0 {
		limit = 3
	}
	rows, err := r.pool.Query(ctx, `
		SELECT p.id, p.title, `+canonicalReplyAuthorName+`, r.author_type,
			EXTRACT(EPOCH FROM (r.created_at - p.created_at)) / 3600
		FROM posts p
		JOIN replies r ON r.id = p.accepted_answer_id`+canonicalReplyAuthorJoins+`
		WHERE p.type = $2 AND `+canonicalPublicStatsPost+`
		ORDER BY p.updated_at DESC
		LIMIT $1`, limit, statsQuestionType)
	if err != nil {
		return nil, err
	}
	out := collectStatsRows(rows, func(row pgx.Rows) (map[string]any, error) {
		var id, title string
		var name, authorType *string
		var hours *float64
		err := row.Scan(&id, &title, &name, &authorType, &hours)
		return map[string]any{"id": id, "title": title, "answerer_name": orUnknown(name),
			"answerer_type": orUnknown(authorType), "time_to_answer_hours": orZero(hours)}, err
	}, &err)
	return out, err
}

// GetTopAnswerers ranks authors by their top-level contributor replies on live public
// questions; accept_rate is the share that is the question's accepted reply.
func (r *CanonicalStatsRepository) GetTopAnswerers(ctx context.Context, limit int) ([]map[string]any, error) {
	if limit <= 0 {
		limit = 5
	}
	rows, err := r.pool.Query(ctx, `
		SELECT r.author_id, r.author_type, COUNT(*) AS answer_count, `+canonicalReplyAuthorName+`,
			(COUNT(*) FILTER (WHERE r.id = p.accepted_answer_id)::float / COUNT(*)::float) * 100 AS accept_rate
		FROM replies r
		JOIN posts p ON p.id = r.post_id`+canonicalReplyAuthorJoins+`
		WHERE `+canonicalAnswerReply+` AND p.type = $2 AND `+canonicalPublicStatsPost+`
		GROUP BY r.author_id, r.author_type, ag.display_name, u.display_name
		ORDER BY answer_count DESC
		LIMIT $1`, limit, statsQuestionType)
	if err != nil {
		return nil, err
	}
	out := collectStatsRows(rows, func(row pgx.Rows) (map[string]any, error) {
		var authorID, authorType, name string
		var count int
		var rate float64
		err := row.Scan(&authorID, &authorType, &count, &name, &rate)
		return map[string]any{"author_id": authorID, "author_type": authorType, "display_name": name,
			"answer_count": count, "accept_rate": rate}, err
	}, &err)
	return out, err
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
