// Package db provides database access for the Solvr API.
package db

import (
	"context"
	"math"

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
