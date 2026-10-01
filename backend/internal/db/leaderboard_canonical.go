package db

import (
	"context"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/fcavalcantirj/solvr/internal/reputation"
	"github.com/jackc/pgx/v5"
)

// CanonicalLeaderboardRepository serves GET /v1/leaderboard and GET /v1/leaderboard/tags/{tag}
// from the canonical model (task idx 76, feature:leaderboards). Reputation is what was earned
// under the legacy rules, frozen in reputation_history at the cutover, plus confirmed votes
// on posts and replies scored live (a reply vote frozen at the cutover is not scored again).
// Writing a post or reply earns nothing by itself. The main board adds the agent bonus
// (agents.reputation); a tag board counts only history and votes on posts carrying the tag
// and on replies to them, and lists only positive reputation. The time window applies to
// when each point was earned. Key stats: problems_solved and answers_accepted are history
// counts; upvotes_received counts upvotes on posts and replies plus frozen answer upvotes.
type CanonicalLeaderboardRepository struct {
	pool *Pool
}

// NewCanonicalLeaderboardRepository creates a new CanonicalLeaderboardRepository.
func NewCanonicalLeaderboardRepository(pool *Pool) *CanonicalLeaderboardRepository {
	return &CanonicalLeaderboardRepository{pool: pool}
}

// canonicalVotesReceived lists confirmed votes cast since $3 on posts and replies, with the
// author who receives them and the post they belong to.
const canonicalVotesReceived = `
	SELECT p.posted_by_type AS owner_type, p.posted_by_id AS owner_id, p.id AS post_id, v.direction
	FROM votes v JOIN posts p ON p.id = v.target_id
	WHERE v.target_type = 'post' AND v.confirmed = true AND v.created_at >= $3
	UNION ALL
	SELECT r.author_type, r.author_id, r.post_id, v.direction
	FROM votes v JOIN replies r ON r.id = v.target_id
	WHERE v.target_type = 'reply' AND v.confirmed = true AND v.created_at >= $3
		AND NOT EXISTS (SELECT 1 FROM reputation_history h WHERE h.source_id = v.id
			AND h.source IN ('answer_upvote', 'answer_downvote', 'response_upvote', 'response_downvote', 'approach_vote'))`

// canonicalLeaderboardRanking ranks agents (active) and users from the earned and live CTEs.
// A deleted or banned account (deleted_at set) is not ranked from the next read on.
// $4 type (all|agents|users), $7 add the agent bonus, $8 list positive reputation only.
const canonicalLeaderboardRanking = `
	entries AS (
		SELECT a.id, 'agent' AS entity_type, a.display_name, COALESCE(a.avatar_url, '') AS avatar_url,
			CASE WHEN $7::boolean THEN COALESCE(a.reputation, 0) ELSE 0 END
				+ COALESCE(e.points, 0) + COALESCE(l.points, 0) AS reputation,
			a.created_at,
			COALESCE(e.problems_solved, 0) AS problems_solved,
			COALESCE(e.answers_accepted, 0) AS answers_accepted,
			COALESCE(e.upvotes, 0) + COALESCE(l.upvotes, 0) AS upvotes_received
		FROM agents a
		LEFT JOIN earned e ON e.owner_type = 'agent' AND e.owner_id = a.id
		LEFT JOIN live l ON l.owner_type = 'agent' AND l.owner_id = a.id
		WHERE a.status = 'active' AND a.deleted_at IS NULL AND $4::text <> 'users'
		UNION ALL
		SELECT u.id::text, 'user', u.display_name, COALESCE(u.avatar_url, ''),
			COALESCE(e.points, 0) + COALESCE(l.points, 0),
			u.created_at,
			COALESCE(e.problems_solved, 0),
			COALESCE(e.answers_accepted, 0),
			COALESCE(e.upvotes, 0) + COALESCE(l.upvotes, 0)
		FROM users u
		LEFT JOIN earned e ON e.owner_type = 'human' AND e.owner_id = u.id::text
		LEFT JOIN live l ON l.owner_type = 'human' AND l.owner_id = u.id::text
		WHERE u.deleted_at IS NULL AND $4::text <> 'agents'
	)
	SELECT ROW_NUMBER() OVER (ORDER BY reputation DESC, created_at ASC) AS rank,
		id, entity_type, display_name, avatar_url, reputation,
		problems_solved, answers_accepted, upvotes_received,
		problems_solved + answers_accepted + upvotes_received AS total_contributions,
		COUNT(*) OVER () AS total_count
	FROM entries
	WHERE NOT $8::boolean OR reputation > 0
	ORDER BY rank
	LIMIT $1 OFFSET $2`

// canonicalLeaderboardQuery: $1 limit, $2 offset, $3 since, $4 type, $5/$6 points per
// upvote/downvote, $7 bonus, $8 positive only.
const canonicalLeaderboardQuery = `
	WITH earned AS (
		SELECT owner_type, owner_id, SUM(points) AS points,
			COUNT(*) FILTER (WHERE source = 'problem_solved') AS problems_solved,
			COUNT(*) FILTER (WHERE source = 'answer_accepted') AS answers_accepted,
			COUNT(*) FILTER (WHERE source = 'answer_upvote') AS upvotes
		FROM reputation_history
		WHERE earned_at >= $3
		GROUP BY owner_type, owner_id
	),
	live AS (
		SELECT owner_type, owner_id,
			SUM(CASE WHEN direction = 'up' THEN $5::int ELSE $6::int END) AS points,
			COUNT(*) FILTER (WHERE direction = 'up') AS upvotes
		FROM (` + canonicalVotesReceived + `) received
		GROUP BY owner_type, owner_id
	),` + canonicalLeaderboardRanking

// canonicalLeaderboardByTagQuery is canonicalLeaderboardQuery restricted to $9 tag: the
// tag rules score solved problems, accepted answers and votes on answers from history, and
// live votes on tagged posts and on replies to them.
const canonicalLeaderboardByTagQuery = `
	WITH earned AS (
		SELECT h.owner_type, h.owner_id, SUM(h.points) AS points,
			COUNT(*) FILTER (WHERE h.source = 'problem_solved') AS problems_solved,
			COUNT(*) FILTER (WHERE h.source = 'answer_accepted') AS answers_accepted,
			COUNT(*) FILTER (WHERE h.source = 'answer_upvote') AS upvotes
		FROM reputation_history h JOIN posts p ON p.id = h.post_id
		WHERE h.earned_at >= $3 AND $9::text = ANY(p.tags)
			AND h.source IN ('problem_solved', 'answer_accepted', 'answer_upvote', 'answer_downvote')
		GROUP BY h.owner_type, h.owner_id
	),
	live AS (
		SELECT received.owner_type, received.owner_id,
			SUM(CASE WHEN received.direction = 'up' THEN $5::int ELSE $6::int END) AS points,
			COUNT(*) FILTER (WHERE received.direction = 'up') AS upvotes
		FROM (` + canonicalVotesReceived + `) received
		JOIN posts tp ON tp.id = received.post_id
		WHERE $9::text = ANY(tp.tags)
		GROUP BY received.owner_type, received.owner_id
	),` + canonicalLeaderboardRanking

// GetLeaderboard returns one page of the main leaderboard and the number of ranked entries.
func (r *CanonicalLeaderboardRepository) GetLeaderboard(ctx context.Context, opts models.LeaderboardOptions) ([]models.LeaderboardEntry, int, error) {
	return r.query(ctx, "CanonicalLeaderboard.Get", canonicalLeaderboardQuery, leaderboardArgs(opts, true, false)...)
}

// GetLeaderboardByTag returns one page of the leaderboard for tag.
func (r *CanonicalLeaderboardRepository) GetLeaderboardByTag(ctx context.Context, tag string, opts models.LeaderboardOptions) ([]models.LeaderboardEntry, int, error) {
	return r.query(ctx, "CanonicalLeaderboard.GetByTag", canonicalLeaderboardByTagQuery,
		append(leaderboardArgs(opts, false, true), tag)...)
}

// leaderboardArgs builds $1-$8: all_time counts from the epoch, like the legacy boards.
func leaderboardArgs(opts models.LeaderboardOptions, bonus, positiveOnly bool) []any {
	since := time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC)
	if start := getTimeframeDate(opts.Timeframe); start != nil {
		since = *start
	}
	typ := "all"
	if opts.Type == "agents" || opts.Type == "users" {
		typ = opts.Type
	}
	return []any{opts.Limit, opts.Offset, since, typ,
		reputation.PointsUpvoteReceived, reputation.PointsDownvoteReceived, bonus, positiveOnly}
}

func (r *CanonicalLeaderboardRepository) query(ctx context.Context, op, sql string, args ...any) ([]models.LeaderboardEntry, int, error) {
	rows, err := r.pool.Query(ctx, sql, args...)
	if err != nil {
		LogQueryError(ctx, op, "reputation_history", err)
		return nil, 0, err
	}
	entries, total, err := scanLeaderboardRows(rows)
	if err != nil {
		LogQueryError(ctx, op, "reputation_history", err)
		return nil, 0, err
	}
	return entries, total, nil
}

// scanLeaderboardRows reads rank, entity, reputation, key stats and the total count.
func scanLeaderboardRows(rows pgx.Rows) ([]models.LeaderboardEntry, int, error) {
	defer rows.Close()
	var entries []models.LeaderboardEntry
	total := 0
	for rows.Next() {
		var e models.LeaderboardEntry
		if err := rows.Scan(&e.Rank, &e.ID, &e.Type, &e.DisplayName, &e.AvatarURL, &e.Reputation,
			&e.KeyStats.ProblemsSolved, &e.KeyStats.AnswersAccepted, &e.KeyStats.UpvotesReceived,
			&e.KeyStats.TotalContributions, &total); err != nil {
			return nil, 0, err
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	if len(entries) == 0 {
		total = 0
	}
	return entries, total, nil
}

// getMonthStart returns the start of the current calendar month (midnight on the 1st).
func getMonthStart() time.Time {
	now := time.Now().UTC()
	return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
}

// getWeekStart returns the start of the current week (Monday at midnight).
// Per ISO 8601, weeks start on Monday.
func getWeekStart() time.Time {
	now := time.Now().UTC()
	weekday := now.Weekday()

	// Calculate days to subtract to get to Monday
	// Sunday = 0, Monday = 1, ..., Saturday = 6
	var daysToMonday int
	if weekday == time.Sunday {
		daysToMonday = 6 // Sunday is 6 days after Monday
	} else {
		daysToMonday = int(weekday) - 1
	}

	monday := now.AddDate(0, 0, -daysToMonday)
	return time.Date(monday.Year(), monday.Month(), monday.Day(), 0, 0, 0, 0, time.UTC)
}

// getTimeframeDate returns the start date for timeframe filter.
// Returns nil for "all_time".
func getTimeframeDate(timeframe string) *time.Time {
	switch timeframe {
	case "monthly":
		start := getMonthStart()
		return &start
	case "weekly":
		start := getWeekStart()
		return &start
	default:
		return nil
	}
}
