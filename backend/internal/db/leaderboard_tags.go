package db

import (
	"context"
	"fmt"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// GetLeaderboardByTag fetches leaderboard entries filtered by a specific tag.
// Returns entries and total count.
func (r *LeaderboardRepository) GetLeaderboardByTag(ctx context.Context, tag string, opts models.LeaderboardOptions) ([]models.LeaderboardEntry, int, error) {
	// Calculate timeframe start date
	startDate := getTimeframeDate(opts.Timeframe)

	// Build the query based on options
	query := r.buildLeaderboardByTagQuery(tag, opts)

	// Execute query with appropriate parameters
	var rows interface{ Next() bool; Close(); Scan(...interface{}) error; Err() error }
	var err error

	if startDate != nil {
		rows, err = r.pool.Query(ctx, query, opts.Limit, opts.Offset, tag, startDate)
	} else {
		rows, err = r.pool.Query(ctx, query, opts.Limit, opts.Offset, tag)
	}

	if err != nil {
		LogQueryError(ctx, "GetLeaderboardByTag", "leaderboard", err)
		return nil, 0, err
	}
	defer rows.Close()

	// Scan results
	var entries []models.LeaderboardEntry
	var totalCount int

	for rows.Next() {
		var entry models.LeaderboardEntry
		err := rows.Scan(
			&entry.Rank,
			&entry.ID,
			&entry.Type,
			&entry.DisplayName,
			&entry.AvatarURL,
			&entry.Reputation,
			&entry.KeyStats.ProblemsSolved,
			&entry.KeyStats.AnswersAccepted,
			&entry.KeyStats.UpvotesReceived,
			&entry.KeyStats.TotalContributions,
			&totalCount,
		)
		if err != nil {
			LogQueryError(ctx, "GetLeaderboardByTag", "leaderboard", err)
			return nil, 0, err
		}
		entries = append(entries, entry)
	}

	if err := rows.Err(); err != nil {
		LogQueryError(ctx, "GetLeaderboardByTag", "leaderboard", err)
		return nil, 0, err
	}

	// If no entries, total count is 0
	if len(entries) == 0 {
		totalCount = 0
	}

	return entries, totalCount, nil
}

// buildLeaderboardByTagQuery constructs the SQL query for tag-specific leaderboard.
func (r *LeaderboardRepository) buildLeaderboardByTagQuery(tag string, opts models.LeaderboardOptions) string {
	var typeFilter string
	switch opts.Type {
	case "agents":
		typeFilter = "WHERE entity_type = 'agent'"
	case "users":
		typeFilter = "WHERE entity_type = 'user'"
	default:
		typeFilter = "" // "all" - no filter
	}

	// For timeframe filtering, we need to add a date filter
	var timeframeFilter string
	if opts.Timeframe == "monthly" || opts.Timeframe == "weekly" {
		timeframeFilter = "$4"
	} else {
		// For all_time, use a very old date (essentially no filter)
		timeframeFilter = "'1970-01-01'::timestamptz"
	}

	// Build the query with tag filtering
	// $1 = LIMIT, $2 = OFFSET, $3 = tag, $4 = startDate (if timeframe)
	query := fmt.Sprintf(`
		WITH leaderboard_data AS (
			-- Agents
			SELECT
				a.id,
				'agent' AS entity_type,
				a.display_name,
				COALESCE(a.avatar_url, '') AS avatar_url,
				CASE
					WHEN %s = '1970-01-01'::timestamptz THEN (
						-- For all_time with tag filter, recalculate from tagged activity
						COALESCE((
							SELECT COUNT(*)
							FROM posts p
							WHERE p.posted_by_id = a.id
								AND p.posted_by_type = 'agent'
								AND p.type = 'problem'
								AND p.status = 'solved'
								AND p.deleted_at IS NULL
								AND $3 = ANY(p.tags)
						), 0) * 100
						+
						COALESCE((
							SELECT COUNT(*)
							FROM answers ans
							JOIN posts q ON q.id = ans.question_id
							WHERE ans.author_id = a.id
								AND ans.author_type = 'agent'
								AND ans.is_accepted = true
								AND ans.deleted_at IS NULL
								AND $3 = ANY(q.tags)
						), 0) * 50
						+
						COALESCE((
							SELECT COUNT(*)
							FROM votes v
							WHERE v.confirmed = true
								AND v.direction = 'up'
								AND (
									(v.target_type = 'post' AND EXISTS (
										SELECT 1 FROM posts p
										WHERE p.id = v.target_id
											AND p.posted_by_type = 'agent'
											AND p.posted_by_id = a.id
											AND $3 = ANY(p.tags)
									))
									OR (v.target_type = 'answer' AND EXISTS (
										SELECT 1 FROM answers ans
										JOIN posts q ON q.id = ans.question_id
										WHERE ans.id = v.target_id
											AND ans.author_type = 'agent'
											AND ans.author_id = a.id
											AND $3 = ANY(q.tags)
									))
								)
						), 0) * 2
						-
						COALESCE((
							SELECT COUNT(*)
							FROM votes v
							WHERE v.confirmed = true
								AND v.direction = 'down'
								AND (
									(v.target_type = 'post' AND EXISTS (
										SELECT 1 FROM posts p
										WHERE p.id = v.target_id
											AND p.posted_by_type = 'agent'
											AND p.posted_by_id = a.id
											AND $3 = ANY(p.tags)
									))
									OR (v.target_type = 'answer' AND EXISTS (
										SELECT 1 FROM answers ans
										JOIN posts q ON q.id = ans.question_id
										WHERE ans.id = v.target_id
											AND ans.author_type = 'agent'
											AND ans.author_id = a.id
											AND $3 = ANY(q.tags)
									))
								)
						), 0)
					)
					ELSE (
						-- For timeframes, recalculate reputation from tagged activity
						COALESCE((
							SELECT COUNT(*)
							FROM posts p
							WHERE p.posted_by_id = a.id
								AND p.posted_by_type = 'agent'
								AND p.type = 'problem'
								AND p.status = 'solved'
								AND p.deleted_at IS NULL
								AND p.created_at >= %s
								AND $3 = ANY(p.tags)
						), 0) * 100
						+
						COALESCE((
							SELECT COUNT(*)
							FROM answers ans
							JOIN posts q ON q.id = ans.question_id
							WHERE ans.author_id = a.id
								AND ans.author_type = 'agent'
								AND ans.is_accepted = true
								AND ans.deleted_at IS NULL
								AND ans.created_at >= %s
								AND $3 = ANY(q.tags)
						), 0) * 50
						+
						COALESCE((
							SELECT COUNT(*)
							FROM votes v
							WHERE v.confirmed = true
								AND v.direction = 'up'
								AND v.created_at >= %s
								AND (
									(v.target_type = 'post' AND EXISTS (
										SELECT 1 FROM posts p
										WHERE p.id = v.target_id
											AND p.posted_by_type = 'agent'
											AND p.posted_by_id = a.id
											AND $3 = ANY(p.tags)
									))
									OR (v.target_type = 'answer' AND EXISTS (
										SELECT 1 FROM answers ans
										JOIN posts q ON q.id = ans.question_id
										WHERE ans.id = v.target_id
											AND ans.author_type = 'agent'
											AND ans.author_id = a.id
											AND $3 = ANY(q.tags)
									))
								)
						), 0) * 2
						-
						COALESCE((
							SELECT COUNT(*)
							FROM votes v
							WHERE v.confirmed = true
								AND v.direction = 'down'
								AND v.created_at >= %s
								AND (
									(v.target_type = 'post' AND EXISTS (
										SELECT 1 FROM posts p
										WHERE p.id = v.target_id
											AND p.posted_by_type = 'agent'
											AND p.posted_by_id = a.id
											AND $3 = ANY(p.tags)
									))
									OR (v.target_type = 'answer' AND EXISTS (
										SELECT 1 FROM answers ans
										JOIN posts q ON q.id = ans.question_id
										WHERE ans.id = v.target_id
											AND ans.author_type = 'agent'
											AND ans.author_id = a.id
											AND $3 = ANY(q.tags)
									))
								)
						), 0)
					)
				END AS reputation,
				a.created_at,
				-- Key stats (tagged activity only)
				COALESCE((
					SELECT COUNT(*)
					FROM posts p
					WHERE p.posted_by_id = a.id
						AND p.posted_by_type = 'agent'
						AND p.type = 'problem'
						AND p.status = 'solved'
						AND p.deleted_at IS NULL
						AND p.created_at >= %s
						AND $3 = ANY(p.tags)
				), 0) AS problems_solved,
				COALESCE((
					SELECT COUNT(*)
					FROM answers ans
					JOIN posts q ON q.id = ans.question_id
					WHERE ans.author_id = a.id
						AND ans.author_type = 'agent'
						AND ans.is_accepted = true
						AND ans.deleted_at IS NULL
						AND ans.created_at >= %s
						AND $3 = ANY(q.tags)
				), 0) AS answers_accepted,
				COALESCE((
					SELECT COUNT(*)
					FROM votes v
					WHERE v.confirmed = true
						AND v.direction = 'up'
						AND v.created_at >= %s
						AND (
							(v.target_type = 'post' AND EXISTS (
								SELECT 1 FROM posts p
								WHERE p.id = v.target_id
									AND p.posted_by_type = 'agent'
									AND p.posted_by_id = a.id
									AND $3 = ANY(p.tags)
							))
							OR (v.target_type = 'answer' AND EXISTS (
								SELECT 1 FROM answers ans
								JOIN posts q ON q.id = ans.question_id
								WHERE ans.id = v.target_id
									AND ans.author_type = 'agent'
									AND ans.author_id = a.id
									AND $3 = ANY(q.tags)
							))
						)
				), 0) AS upvotes_received
			FROM agents a
			WHERE a.status = 'active'

			UNION ALL

			-- Users
			SELECT
				u.id::text,
				'user' AS entity_type,
				u.display_name,
				COALESCE(u.avatar_url, '') AS avatar_url,
				-- Calculate user reputation from tagged activity
				(
					COALESCE((
						SELECT COUNT(*)
						FROM posts p
						WHERE p.posted_by_id = u.id::text
							AND p.posted_by_type = 'human'
							AND p.type = 'problem'
							AND p.status = 'solved'
							AND p.deleted_at IS NULL
							AND p.created_at >= %s
							AND $3 = ANY(p.tags)
					), 0) * 100
					+
					COALESCE((
						SELECT COUNT(*)
						FROM answers ans
						JOIN posts q ON q.id = ans.question_id
						WHERE ans.author_id = u.id::text
							AND ans.author_type = 'human'
							AND ans.is_accepted = true
							AND ans.deleted_at IS NULL
							AND ans.created_at >= %s
							AND $3 = ANY(q.tags)
					), 0) * 50
					+
					COALESCE((
						SELECT COUNT(*)
						FROM votes v
						WHERE v.confirmed = true
							AND v.direction = 'up'
							AND v.created_at >= %s
							AND (
								(v.target_type = 'post' AND EXISTS (
									SELECT 1 FROM posts p
									WHERE p.id = v.target_id
										AND p.posted_by_type = 'human'
										AND p.posted_by_id = u.id::text
										AND $3 = ANY(p.tags)
								))
								OR (v.target_type = 'answer' AND EXISTS (
									SELECT 1 FROM answers ans
									JOIN posts q ON q.id = ans.question_id
									WHERE ans.id = v.target_id
										AND ans.author_type = 'human'
										AND ans.author_id = u.id::text
										AND $3 = ANY(q.tags)
								))
							)
					), 0) * 2
					-
					COALESCE((
						SELECT COUNT(*)
						FROM votes v
						WHERE v.confirmed = true
							AND v.direction = 'down'
							AND v.created_at >= %s
							AND (
								(v.target_type = 'post' AND EXISTS (
									SELECT 1 FROM posts p
									WHERE p.id = v.target_id
										AND p.posted_by_type = 'human'
										AND p.posted_by_id = u.id::text
										AND $3 = ANY(p.tags)
								))
								OR (v.target_type = 'answer' AND EXISTS (
									SELECT 1 FROM answers ans
									JOIN posts q ON q.id = ans.question_id
									WHERE ans.id = v.target_id
										AND ans.author_type = 'human'
										AND ans.author_id = u.id::text
										AND $3 = ANY(q.tags)
								))
							)
					), 0)
				) AS reputation,
				u.created_at,
				-- Key stats (tagged activity only)
				COALESCE((
					SELECT COUNT(*)
					FROM posts p
					WHERE p.posted_by_id = u.id::text
						AND p.posted_by_type = 'human'
						AND p.type = 'problem'
						AND p.status = 'solved'
						AND p.deleted_at IS NULL
						AND p.created_at >= %s
						AND $3 = ANY(p.tags)
				), 0) AS problems_solved,
				COALESCE((
					SELECT COUNT(*)
					FROM answers ans
					JOIN posts q ON q.id = ans.question_id
					WHERE ans.author_id = u.id::text
						AND ans.author_type = 'human'
						AND ans.is_accepted = true
						AND ans.deleted_at IS NULL
						AND ans.created_at >= %s
						AND $3 = ANY(q.tags)
				), 0) AS answers_accepted,
				COALESCE((
					SELECT COUNT(*)
					FROM votes v
					WHERE v.confirmed = true
						AND v.direction = 'up'
						AND v.created_at >= %s
						AND (
							(v.target_type = 'post' AND EXISTS (
								SELECT 1 FROM posts p
								WHERE p.id = v.target_id
									AND p.posted_by_type = 'human'
									AND p.posted_by_id = u.id::text
									AND $3 = ANY(p.tags)
							))
							OR (v.target_type = 'answer' AND EXISTS (
								SELECT 1 FROM answers ans
								JOIN posts q ON q.id = ans.question_id
								WHERE ans.id = v.target_id
									AND ans.author_type = 'human'
									AND ans.author_id = u.id::text
									AND $3 = ANY(q.tags)
							))
						)
				), 0) AS upvotes_received
			FROM users u
		),
		ranked_leaderboard AS (
			SELECT
				ROW_NUMBER() OVER (ORDER BY reputation DESC, created_at ASC) AS rank,
				id,
				entity_type,
				display_name,
				avatar_url,
				reputation,
				problems_solved,
				answers_accepted,
				upvotes_received,
				(problems_solved + answers_accepted + upvotes_received) AS total_contributions
			FROM leaderboard_data
			WHERE reputation > 0
			%s
		),
		total_count AS (
			SELECT COUNT(*) AS cnt FROM ranked_leaderboard
		)
		SELECT
			rl.rank,
			rl.id,
			rl.entity_type,
			rl.display_name,
			rl.avatar_url,
			rl.reputation,
			rl.problems_solved,
			rl.answers_accepted,
			rl.upvotes_received,
			rl.total_contributions,
			tc.cnt AS total_count
		FROM ranked_leaderboard rl
		CROSS JOIN total_count tc
		ORDER BY rl.rank
		LIMIT $1 OFFSET $2
	`, timeframeFilter, timeframeFilter, timeframeFilter, timeframeFilter, timeframeFilter,
		timeframeFilter, timeframeFilter, timeframeFilter,
		timeframeFilter, timeframeFilter, timeframeFilter, timeframeFilter,
		timeframeFilter, timeframeFilter, timeframeFilter,
		typeFilter)

	return query
}
