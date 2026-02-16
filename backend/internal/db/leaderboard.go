package db

import (
	"context"
	"fmt"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// LeaderboardRepository handles database operations for leaderboard.
type LeaderboardRepository struct {
	pool *Pool
}

// NewLeaderboardRepository creates a new LeaderboardRepository.
func NewLeaderboardRepository(pool *Pool) *LeaderboardRepository {
	return &LeaderboardRepository{pool: pool}
}

// GetLeaderboard fetches leaderboard entries with the given options.
// Returns entries and total count.
func (r *LeaderboardRepository) GetLeaderboard(ctx context.Context, opts models.LeaderboardOptions) ([]models.LeaderboardEntry, int, error) {
	// Calculate timeframe start date
	startDate := getTimeframeDate(opts.Timeframe)

	// Build the query based on options
	// Always pass startDate - use epoch for all_time to include all activity
	if startDate == nil {
		epoch := time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC)
		startDate = &epoch
	}

	query := r.buildLeaderboardQuery(opts)

	// Execute query with startDate parameter
	rows, err := r.pool.Query(ctx, query, opts.Limit, opts.Offset, startDate)
	if err != nil {
		LogQueryError(ctx, "GetLeaderboard", "leaderboard", err)
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
			LogQueryError(ctx, "GetLeaderboard", "leaderboard", err)
			return nil, 0, err
		}
		entries = append(entries, entry)
	}

	if err := rows.Err(); err != nil {
		LogQueryError(ctx, "GetLeaderboard", "leaderboard", err)
		return nil, 0, err
	}

	// If no entries, total count is 0
	if len(entries) == 0 {
		totalCount = 0
	}

	return entries, totalCount, nil
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

// buildLeaderboardQuery constructs the SQL query based on options.
func (r *LeaderboardRepository) buildLeaderboardQuery(opts models.LeaderboardOptions) string {
	var typeFilter string
	switch opts.Type {
	case "agents":
		typeFilter = "WHERE entity_type = 'agent'"
	case "users":
		typeFilter = "WHERE entity_type = 'user'"
	default:
		typeFilter = "" // "all" - no filter
	}

	// Build the query
	// Always recalculate reputation from activity for consistency.
	// The timeframe filter is always $3 (startDate parameter).
	query := fmt.Sprintf(`
		WITH leaderboard_data AS (
			-- Agents
			SELECT
				a.id,
				'agent' AS entity_type,
				a.display_name,
				COALESCE(a.avatar_url, '') AS avatar_url,
				-- Reputation formula matching agents.GetAgentStats() for consistency
				-- Formula: bonus + problems_solved*100 + problems_contributed*25 +
				--          answers_accepted*50 + answers_given*10 + ideas_posted*15 +
				--          responses_given*5 + upvotes*2 - downvotes*1
				(
					-- Bonus from agents.reputation column
					COALESCE(a.reputation, 0)
					+
					-- Problems SOLVED: 100 points each
					COALESCE((
						SELECT COUNT(*)
						FROM posts p
						WHERE p.posted_by_id = a.id
							AND p.posted_by_type = 'agent'
							AND p.type = 'problem'
							AND p.status = 'solved'
							AND p.deleted_at IS NULL
							AND p.created_at >= $3
					), 0) * 100
					+
					-- Problems CONTRIBUTED (all problems): 25 points each
					COALESCE((
						SELECT COUNT(*)
						FROM posts p
						WHERE p.posted_by_id = a.id
							AND p.posted_by_type = 'agent'
							AND p.type = 'problem'
							AND p.deleted_at IS NULL
							AND p.created_at >= $3
					), 0) * 25
					+
					-- Answers ACCEPTED: 50 points each
					COALESCE((
						SELECT COUNT(*)
						FROM answers ans
						WHERE ans.author_id = a.id
							AND ans.author_type = 'agent'
							AND ans.is_accepted = true
							AND ans.deleted_at IS NULL
							AND ans.created_at >= $3
					), 0) * 50
					+
					-- Answers GIVEN (all answers): 10 points each
					COALESCE((
						SELECT COUNT(*)
						FROM answers ans
						WHERE ans.author_id = a.id
							AND ans.author_type = 'agent'
							AND ans.deleted_at IS NULL
							AND ans.created_at >= $3
					), 0) * 10
					+
					-- Ideas POSTED: 15 points each
					COALESCE((
						SELECT COUNT(*)
						FROM posts p
						WHERE p.posted_by_id = a.id
							AND p.posted_by_type = 'agent'
							AND p.type = 'idea'
							AND p.deleted_at IS NULL
							AND p.created_at >= $3
					), 0) * 15
					+
					-- Responses GIVEN: 5 points each
					COALESCE((
						SELECT COUNT(*)
						FROM responses r
						WHERE r.author_id = a.id
							AND r.author_type = 'agent'
							AND r.created_at >= $3
					), 0) * 5
					+
					-- Upvotes RECEIVED: 2 points each
					COALESCE((
						SELECT COUNT(*)
						FROM votes v
						WHERE v.confirmed = true
							AND v.direction = 'up'
							AND v.created_at >= $3
							AND (
								(v.target_type = 'post' AND EXISTS (
									SELECT 1 FROM posts p WHERE p.id = v.target_id AND p.posted_by_type = 'agent' AND p.posted_by_id = a.id
								))
								OR (v.target_type = 'answer' AND EXISTS (
									SELECT 1 FROM answers ans WHERE ans.id = v.target_id AND ans.author_type = 'agent' AND ans.author_id = a.id
								))
								OR (v.target_type = 'response' AND EXISTS (
									SELECT 1 FROM responses r WHERE r.id = v.target_id AND r.author_type = 'agent' AND r.author_id = a.id
								))
							)
					), 0) * 2
					-
					-- Downvotes RECEIVED: -1 point each
					COALESCE((
						SELECT COUNT(*)
						FROM votes v
						WHERE v.confirmed = true
							AND v.direction = 'down'
							AND v.created_at >= $3
							AND (
								(v.target_type = 'post' AND EXISTS (
									SELECT 1 FROM posts p WHERE p.id = v.target_id AND p.posted_by_type = 'agent' AND p.posted_by_id = a.id
								))
								OR (v.target_type = 'answer' AND EXISTS (
									SELECT 1 FROM answers ans WHERE ans.id = v.target_id AND ans.author_type = 'agent' AND ans.author_id = a.id
								))
								OR (v.target_type = 'response' AND EXISTS (
									SELECT 1 FROM responses r WHERE r.id = v.target_id AND r.author_type = 'agent' AND r.author_id = a.id
								))
							)
					), 0)
				) AS reputation,
				a.created_at,
				-- Key stats
				COALESCE((
					SELECT COUNT(*)
					FROM posts p
					WHERE p.posted_by_id = a.id
						AND p.posted_by_type = 'agent'
						AND p.type = 'problem'
						AND p.status = 'solved'
						AND p.deleted_at IS NULL
						AND p.created_at >= $3
				), 0) AS problems_solved,
				COALESCE((
					SELECT COUNT(*)
					FROM answers ans
					WHERE ans.author_id = a.id
						AND ans.author_type = 'agent'
						AND ans.is_accepted = true
						AND ans.deleted_at IS NULL
						AND ans.created_at >= $3
				), 0) AS answers_accepted,
				COALESCE((
					SELECT COUNT(*)
					FROM votes v
					WHERE v.confirmed = true
						AND v.direction = 'up'
						AND v.created_at >= $3
						AND (
							(v.target_type = 'post' AND EXISTS (
								SELECT 1 FROM posts p WHERE p.id = v.target_id AND p.posted_by_type = 'agent' AND p.posted_by_id = a.id
							))
							OR (v.target_type = 'answer' AND EXISTS (
								SELECT 1 FROM answers ans WHERE ans.id = v.target_id AND ans.author_type = 'agent' AND ans.author_id = a.id
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
				-- Calculate user reputation from activity
				(
					COALESCE((
						SELECT COUNT(*)
						FROM posts p
						WHERE p.posted_by_id = u.id::text
							AND p.posted_by_type = 'human'
							AND p.type = 'problem'
							AND p.status = 'solved'
							AND p.deleted_at IS NULL
							AND p.created_at >= $3
					), 0) * 100
					+
					COALESCE((
						SELECT COUNT(*)
						FROM answers ans
						WHERE ans.author_id = u.id::text
							AND ans.author_type = 'human'
							AND ans.is_accepted = true
							AND ans.deleted_at IS NULL
							AND ans.created_at >= $3
					), 0) * 50
					+
					COALESCE((
						SELECT COUNT(*)
						FROM votes v
						WHERE v.confirmed = true
							AND v.direction = 'up'
							AND v.created_at >= $3
							AND (
								(v.target_type = 'post' AND EXISTS (
									SELECT 1 FROM posts p WHERE p.id = v.target_id AND p.posted_by_type = 'human' AND p.posted_by_id = u.id::text
								))
								OR (v.target_type = 'answer' AND EXISTS (
									SELECT 1 FROM answers ans WHERE ans.id = v.target_id AND ans.author_type = 'human' AND ans.author_id = u.id::text
								))
							)
					), 0) * 2
					-
					COALESCE((
						SELECT COUNT(*)
						FROM votes v
						WHERE v.confirmed = true
							AND v.direction = 'down'
							AND v.created_at >= $3
							AND (
								(v.target_type = 'post' AND EXISTS (
									SELECT 1 FROM posts p WHERE p.id = v.target_id AND p.posted_by_type = 'human' AND p.posted_by_id = u.id::text
								))
								OR (v.target_type = 'answer' AND EXISTS (
									SELECT 1 FROM answers ans WHERE ans.id = v.target_id AND ans.author_type = 'human' AND ans.author_id = u.id::text
								))
							)
					), 0)
				) AS reputation,
				u.created_at,
				-- Key stats
				COALESCE((
					SELECT COUNT(*)
					FROM posts p
					WHERE p.posted_by_id = u.id::text
						AND p.posted_by_type = 'human'
						AND p.type = 'problem'
						AND p.status = 'solved'
						AND p.deleted_at IS NULL
						AND p.created_at >= $3
				), 0) AS problems_solved,
				COALESCE((
					SELECT COUNT(*)
					FROM answers ans
					WHERE ans.author_id = u.id::text
						AND ans.author_type = 'human'
						AND ans.is_accepted = true
						AND ans.deleted_at IS NULL
						AND ans.created_at >= $3
				), 0) AS answers_accepted,
				COALESCE((
					SELECT COUNT(*)
					FROM votes v
					WHERE v.confirmed = true
						AND v.direction = 'up'
						AND v.created_at >= $3
						AND (
							(v.target_type = 'post' AND EXISTS (
								SELECT 1 FROM posts p WHERE p.id = v.target_id AND p.posted_by_type = 'human' AND p.posted_by_id = u.id::text
							))
							OR (v.target_type = 'answer' AND EXISTS (
								SELECT 1 FROM answers ans WHERE ans.id = v.target_id AND ans.author_type = 'human' AND ans.author_id = u.id::text
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
	`, typeFilter)

	return query
}
