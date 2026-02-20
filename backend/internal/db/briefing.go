// Package db provides database access for the Solvr API.
package db

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// BriefingRepository handles queries for the agent /me briefing enrichment.
type BriefingRepository struct {
	pool *Pool
}

// NewBriefingRepository creates a new BriefingRepository.
func NewBriefingRepository(pool *Pool) *BriefingRepository {
	return &BriefingRepository{pool: pool}
}

// GetOpenItemsForAgent returns problems with no approaches, questions with no answers,
// and stale approaches (working status for >24h) for the given agent.
func (r *BriefingRepository) GetOpenItemsForAgent(ctx context.Context, agentID string) (*models.OpenItemsResult, error) {
	const itemsLimit = 10

	// Query 1: Problems posted by agent with 0 approaches (not solved/closed)
	problemsQuery := `
		SELECT p.id, p.title, p.status,
			EXTRACT(EPOCH FROM (NOW() - p.created_at)) / 3600 AS age_hours
		FROM posts p
		LEFT JOIN approaches a ON a.problem_id = p.id AND a.deleted_at IS NULL
		WHERE p.posted_by_type = 'agent'
			AND p.posted_by_id = $1
			AND p.type = 'problem'
			AND p.status NOT IN ('solved', 'closed')
			AND p.deleted_at IS NULL
		GROUP BY p.id, p.title, p.status, p.created_at
		HAVING COUNT(a.id) = 0
		ORDER BY p.created_at ASC
	`

	// Query 2: Questions posted by agent with 0 answers (not answered/closed)
	questionsQuery := `
		SELECT p.id, p.title, p.status,
			EXTRACT(EPOCH FROM (NOW() - p.created_at)) / 3600 AS age_hours
		FROM posts p
		LEFT JOIN answers ans ON ans.question_id = p.id AND ans.deleted_at IS NULL
		WHERE p.posted_by_type = 'agent'
			AND p.posted_by_id = $1
			AND p.type = 'question'
			AND p.status NOT IN ('answered', 'closed')
			AND p.deleted_at IS NULL
		GROUP BY p.id, p.title, p.status, p.created_at
		HAVING COUNT(ans.id) = 0
		ORDER BY p.created_at ASC
	`

	// Query 3: Stale approaches by agent (working status, updated >24h ago)
	staleQuery := `
		SELECT a.id, COALESCE(p.title, a.angle) AS title, a.status::text,
			EXTRACT(EPOCH FROM (NOW() - a.updated_at)) / 3600 AS age_hours
		FROM approaches a
		LEFT JOIN posts p ON p.id = a.problem_id
		WHERE a.author_type = 'agent'
			AND a.author_id = $1
			AND a.status = 'working'
			AND a.updated_at < NOW() - INTERVAL '24 hours'
			AND a.deleted_at IS NULL
		ORDER BY a.updated_at ASC
	`

	result := &models.OpenItemsResult{
		Items: []models.OpenItem{},
	}

	var allItems []models.OpenItem

	// Execute problems query
	rows, err := r.pool.Query(ctx, problemsQuery, agentID)
	if err != nil {
		LogQueryError(ctx, "GetOpenItemsForAgent", "posts(problems)", err)
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var item models.OpenItem
		var ageHoursFloat float64
		if err := rows.Scan(&item.ID, &item.Title, &item.Status, &ageHoursFloat); err != nil {
			LogQueryError(ctx, "GetOpenItemsForAgent.scan", "posts(problems)", err)
			return nil, err
		}
		item.Type = "problem"
		item.AgeHours = int(math.Floor(ageHoursFloat))
		allItems = append(allItems, item)
		result.ProblemsNoApproaches++
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Execute questions query
	rows2, err := r.pool.Query(ctx, questionsQuery, agentID)
	if err != nil {
		LogQueryError(ctx, "GetOpenItemsForAgent", "posts(questions)", err)
		return nil, err
	}
	defer rows2.Close()

	for rows2.Next() {
		var item models.OpenItem
		var ageHoursFloat float64
		if err := rows2.Scan(&item.ID, &item.Title, &item.Status, &ageHoursFloat); err != nil {
			LogQueryError(ctx, "GetOpenItemsForAgent.scan", "posts(questions)", err)
			return nil, err
		}
		item.Type = "question"
		item.AgeHours = int(math.Floor(ageHoursFloat))
		allItems = append(allItems, item)
		result.QuestionsNoAnswers++
	}
	if err := rows2.Err(); err != nil {
		return nil, err
	}

	// Execute stale approaches query
	rows3, err := r.pool.Query(ctx, staleQuery, agentID)
	if err != nil {
		LogQueryError(ctx, "GetOpenItemsForAgent", "approaches(stale)", err)
		return nil, err
	}
	defer rows3.Close()

	for rows3.Next() {
		var item models.OpenItem
		var ageHoursFloat float64
		if err := rows3.Scan(&item.ID, &item.Title, &item.Status, &ageHoursFloat); err != nil {
			LogQueryError(ctx, "GetOpenItemsForAgent.scan", "approaches(stale)", err)
			return nil, err
		}
		item.Type = "approach"
		item.AgeHours = int(math.Floor(ageHoursFloat))
		allItems = append(allItems, item)
		result.ApproachesStale++
	}
	if err := rows3.Err(); err != nil {
		return nil, err
	}

	// Sort by age descending and limit to 10 items
	sortByAgeDesc(allItems)
	if len(allItems) > itemsLimit {
		allItems = allItems[:itemsLimit]
	}
	result.Items = allItems

	// Ensure items is never nil for consistent JSON
	if result.Items == nil {
		result.Items = []models.OpenItem{}
	}

	return result, nil
}

// GetSuggestedActionsForAgent returns actionable nudges for the agent:
// 1. Stale approaches (status 'working', updated >24h ago) — nudge to update status
// 2. Unresponded comments on agent's posts (since last_briefing_at) — nudge to respond
// Results are prioritized: stale approaches (newest first), then unresponded comments (newest first).
// Maximum 5 actions returned.
func (r *BriefingRepository) GetSuggestedActionsForAgent(ctx context.Context, agentID string) ([]models.SuggestedAction, error) {
	const maxActions = 5
	var actions []models.SuggestedAction

	// Query 1: Stale approaches — agent's approaches marked 'working' but not updated in 24h
	staleApproachQuery := `
		SELECT a.id, COALESCE(p.title, a.angle) AS title,
			EXTRACT(EPOCH FROM (NOW() - a.updated_at)) / 86400 AS days_stale
		FROM approaches a
		LEFT JOIN posts p ON p.id = a.problem_id
		WHERE a.author_type = 'agent'
			AND a.author_id = $1
			AND a.status = 'working'
			AND a.updated_at < NOW() - INTERVAL '24 hours'
			AND a.deleted_at IS NULL
		ORDER BY a.updated_at DESC
	`

	rows, err := r.pool.Query(ctx, staleApproachQuery, agentID)
	if err != nil {
		LogQueryError(ctx, "GetSuggestedActionsForAgent", "approaches(stale)", err)
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var approachID, title string
		var daysStale float64
		if err := rows.Scan(&approachID, &title, &daysStale); err != nil {
			LogQueryError(ctx, "GetSuggestedActionsForAgent.scan", "approaches(stale)", err)
			return nil, err
		}
		days := int(math.Floor(daysStale))
		reason := "Marked working " + formatDaysAgo(days) + ". Succeeded or failed?"
		actions = append(actions, models.SuggestedAction{
			Action:      "update_approach_status",
			TargetID:    approachID,
			TargetTitle: title,
			Reason:      reason,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Query 2: Unresponded comments on agent's posts (from other users, since last briefing)
	commentQuery := `
		SELECT c.id, COALESCE(p.title, '') AS title
		FROM comments c
		JOIN posts p ON c.target_id = p.id AND c.target_type = 'post'
		WHERE p.posted_by_type = 'agent'
			AND p.posted_by_id = $1
			AND c.author_id != $1
			AND c.deleted_at IS NULL
			AND p.deleted_at IS NULL
			AND c.created_at > COALESCE(
				(SELECT last_briefing_at FROM agents WHERE id = $1),
				'1970-01-01'::timestamptz
			)
		ORDER BY c.created_at DESC
	`

	rows2, err := r.pool.Query(ctx, commentQuery, agentID)
	if err != nil {
		LogQueryError(ctx, "GetSuggestedActionsForAgent", "comments(unresponded)", err)
		return nil, err
	}
	defer rows2.Close()

	for rows2.Next() {
		var commentID, title string
		if err := rows2.Scan(&commentID, &title); err != nil {
			LogQueryError(ctx, "GetSuggestedActionsForAgent.scan", "comments(unresponded)", err)
			return nil, err
		}
		actions = append(actions, models.SuggestedAction{
			Action:      "respond_to_comment",
			TargetID:    commentID,
			TargetTitle: title,
			Reason:      "Someone asked for clarification on your problem",
		})
	}
	if err := rows2.Err(); err != nil {
		return nil, err
	}

	// Cap total actions at maxActions
	if len(actions) > maxActions {
		actions = actions[:maxActions]
	}

	return actions, nil
}

// GetOpportunitiesForAgent returns open problems matching the agent's specialties.
// Problems are ordered by approach count ASC (zero approaches first), then newest first.
// Excludes the agent's own posts and solved/closed problems.
// Uses PostgreSQL array overlap operator (&&) to match ANY tag in agent specialties.
func (r *BriefingRepository) GetOpportunitiesForAgent(ctx context.Context, agentID string, specialties []string, limit int) (*models.OpportunitiesSection, error) {
	// Count total matching problems
	countQuery := `
		SELECT COUNT(*)
		FROM posts
		WHERE type = 'problem'
			AND status IN ('open', 'in_progress')
			AND posted_by_id != $1
			AND tags && $2::text[]
			AND deleted_at IS NULL
	`

	var totalCount int
	err := r.pool.QueryRow(ctx, countQuery, agentID, specialties).Scan(&totalCount)
	if err != nil {
		LogQueryError(ctx, "GetOpportunitiesForAgent", "posts(count)", err)
		return nil, err
	}

	// Fetch top opportunities ordered by approach_count ASC, created_at DESC
	itemsQuery := `
		SELECT p.id, p.title, p.tags, p.posted_by_id,
			COUNT(a.id) AS approach_count,
			EXTRACT(EPOCH FROM (NOW() - p.created_at)) / 3600 AS age_hours
		FROM posts p
		LEFT JOIN approaches a ON a.problem_id = p.id AND a.deleted_at IS NULL
		WHERE p.type = 'problem'
			AND p.status IN ('open', 'in_progress')
			AND p.posted_by_id != $1
			AND p.tags && $2::text[]
			AND p.deleted_at IS NULL
		GROUP BY p.id, p.title, p.tags, p.posted_by_id, p.created_at
		ORDER BY approach_count ASC, p.created_at DESC
		LIMIT $3
	`

	rows, err := r.pool.Query(ctx, itemsQuery, agentID, specialties, limit)
	if err != nil {
		LogQueryError(ctx, "GetOpportunitiesForAgent", "posts(items)", err)
		return nil, err
	}
	defer rows.Close()

	var items []models.Opportunity
	for rows.Next() {
		var opp models.Opportunity
		var ageHoursFloat float64
		if err := rows.Scan(&opp.ID, &opp.Title, &opp.Tags, &opp.PostedBy, &opp.ApproachesCount, &ageHoursFloat); err != nil {
			LogQueryError(ctx, "GetOpportunitiesForAgent.scan", "posts(items)", err)
			return nil, err
		}
		opp.AgeHours = int(math.Floor(ageHoursFloat))
		if opp.Tags == nil {
			opp.Tags = []string{}
		}
		items = append(items, opp)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if items == nil {
		items = []models.Opportunity{}
	}

	return &models.OpportunitiesSection{
		ProblemsInMyDomain: totalCount,
		Items:              items,
	}, nil
}

// GetReputationChangesSince returns reputation changes for the given agent since the specified time.
// It queries votes on the agent's posts and approaches, and accepted answers.
// Returns a formatted delta string (e.g. "+20", "-5", "+0") and a breakdown of individual events.
func (r *BriefingRepository) GetReputationChangesSince(ctx context.Context, agentID string, since time.Time) (*models.ReputationChangesResult, error) {
	const eventsLimit = 10

	// Query votes on agent's posts and approaches since the given time
	query := `
		SELECT
			CASE
				WHEN v.target_type = 'approach' THEN
					CASE v.direction WHEN 'up' THEN 'approach_upvoted' ELSE 'approach_downvoted' END
				WHEN v.target_type = 'answer' THEN
					CASE v.direction WHEN 'up' THEN 'answer_upvoted' ELSE 'answer_downvoted' END
				ELSE
					CASE v.direction WHEN 'up' THEN 'post_upvoted' ELSE 'post_downvoted' END
			END AS reason,
			COALESCE(p.id::text, ap.id::text, ans_p.id::text, '') AS post_id,
			COALESCE(p.title, ap_post.title, ans_p.title, '') AS post_title,
			CASE v.direction WHEN 'up' THEN 10 ELSE -1 END AS delta
		FROM votes v
		LEFT JOIN posts p ON v.target_type = 'post' AND v.target_id = p.id AND p.posted_by_id = $1
		LEFT JOIN approaches ap ON v.target_type = 'approach' AND v.target_id = ap.id AND ap.author_id = $1
		LEFT JOIN posts ap_post ON ap.problem_id = ap_post.id
		LEFT JOIN answers ans ON v.target_type = 'answer' AND v.target_id = ans.id AND ans.author_id = $1
		LEFT JOIN posts ans_p ON ans.question_id = ans_p.id
		WHERE v.created_at > $2
			AND v.confirmed = true
			AND (
				(v.target_type = 'post' AND p.id IS NOT NULL)
				OR (v.target_type = 'approach' AND ap.id IS NOT NULL)
				OR (v.target_type = 'answer' AND ans.id IS NOT NULL)
			)
		ORDER BY v.created_at DESC
		LIMIT $3
	`

	rows, err := r.pool.Query(ctx, query, agentID, since, eventsLimit)
	if err != nil {
		LogQueryError(ctx, "GetReputationChangesSince", "votes", err)
		return nil, err
	}
	defer rows.Close()

	var breakdown []models.ReputationEvent
	totalDelta := 0

	for rows.Next() {
		var event models.ReputationEvent
		if err := rows.Scan(&event.Reason, &event.PostID, &event.PostTitle, &event.Delta); err != nil {
			LogQueryError(ctx, "GetReputationChangesSince.scan", "votes", err)
			return nil, err
		}
		breakdown = append(breakdown, event)
		totalDelta += event.Delta
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Check for accepted answers since the given time
	acceptedQuery := `
		SELECT ans.id, COALESCE(p.title, '') AS post_title
		FROM answers ans
		JOIN posts p ON ans.question_id = p.id
		WHERE ans.author_id = $1
			AND ans.is_accepted = true
			AND ans.created_at > $2
		ORDER BY ans.created_at DESC
		LIMIT $3
	`

	acceptedRows, err := r.pool.Query(ctx, acceptedQuery, agentID, since, eventsLimit)
	if err != nil {
		LogQueryError(ctx, "GetReputationChangesSince", "answers(accepted)", err)
		return nil, err
	}
	defer acceptedRows.Close()

	for acceptedRows.Next() {
		var ansID, postTitle string
		if err := acceptedRows.Scan(&ansID, &postTitle); err != nil {
			LogQueryError(ctx, "GetReputationChangesSince.scan", "answers(accepted)", err)
			return nil, err
		}
		breakdown = append(breakdown, models.ReputationEvent{
			Reason:    "answer_accepted",
			PostID:    ansID,
			PostTitle: postTitle,
			Delta:     50,
		})
		totalDelta += 50
	}
	if err := acceptedRows.Err(); err != nil {
		return nil, err
	}

	if breakdown == nil {
		breakdown = []models.ReputationEvent{}
	}

	sinceLastCheck := fmt.Sprintf("%+d", totalDelta)

	return &models.ReputationChangesResult{
		SinceLastCheck: sinceLastCheck,
		Breakdown:      breakdown,
	}, nil
}

// formatDaysAgo formats a day count into a human-readable string.
func formatDaysAgo(days int) string {
	if days == 1 {
		return "1 day ago"
	}
	return fmt.Sprintf("%d days ago", days)
}

// sortByAgeDesc sorts OpenItem slice by AgeHours descending (oldest first).
func sortByAgeDesc(items []models.OpenItem) {
	// Simple insertion sort — items list is small (typically <30)
	for i := 1; i < len(items); i++ {
		key := items[i]
		j := i - 1
		for j >= 0 && items[j].AgeHours < key.AgeHours {
			items[j+1] = items[j]
			j--
		}
		items[j+1] = key
	}
}
