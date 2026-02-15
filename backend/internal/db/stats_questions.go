// Package db provides database access for Solvr.
package db

import (
	"context"
)

// GetQuestionsStats returns aggregate statistics for questions.
func (r *StatsRepository) GetQuestionsStats(ctx context.Context) (map[string]any, error) {
	var totalQuestions, answeredCount int
	var responseRate, avgResponseTimeHours float64

	err := r.pool.QueryRow(ctx, `
		SELECT
			(SELECT COUNT(*) FROM posts WHERE type = 'question' AND deleted_at IS NULL),
			(SELECT COUNT(*) FROM posts WHERE type = 'question' AND accepted_answer_id IS NOT NULL AND deleted_at IS NULL),
			COALESCE((
				SELECT
					CASE WHEN COUNT(*) = 0 THEN 0
					ELSE (COUNT(*) FILTER (WHERE accepted_answer_id IS NOT NULL)::float / COUNT(*)::float) * 100
					END
				FROM posts WHERE type = 'question' AND deleted_at IS NULL
			), 0),
			COALESCE((
				SELECT AVG(EXTRACT(EPOCH FROM (a.created_at - p.created_at)) / 3600)
				FROM posts p
				JOIN LATERAL (
					SELECT created_at FROM answers
					WHERE question_id = p.id AND deleted_at IS NULL
					ORDER BY created_at ASC
					LIMIT 1
				) a ON true
				WHERE p.type = 'question' AND p.deleted_at IS NULL
			), 0)
	`).Scan(&totalQuestions, &answeredCount, &responseRate, &avgResponseTimeHours)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"total_questions":         totalQuestions,
		"answered_count":          answeredCount,
		"response_rate":           responseRate,
		"avg_response_time_hours": avgResponseTimeHours,
	}, nil
}

// GetRecentlyAnsweredQuestions returns recently answered questions with answerer info.
func (r *StatsRepository) GetRecentlyAnsweredQuestions(ctx context.Context, limit int) ([]map[string]any, error) {
	if limit <= 0 {
		limit = 3
	}

	rows, err := r.pool.Query(ctx, `
		SELECT
			p.id,
			p.title,
			COALESCE(
				CASE
					WHEN ans.author_type = 'agent' THEN ag.display_name
					WHEN ans.author_type = 'human' THEN u.display_name
				END,
				ans.author_id
			) as answerer_name,
			ans.author_type as answerer_type,
			EXTRACT(EPOCH FROM (ans.created_at - p.created_at)) / 3600 as time_to_answer_hours
		FROM posts p
		JOIN answers ans ON ans.id = p.accepted_answer_id
		LEFT JOIN agents ag ON ans.author_type = 'agent' AND ans.author_id = ag.id
		LEFT JOIN users u ON ans.author_type = 'human' AND ans.author_id = u.id::text
		WHERE p.type = 'question'
		AND p.accepted_answer_id IS NOT NULL
		AND p.deleted_at IS NULL
		ORDER BY p.updated_at DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []map[string]any
	for rows.Next() {
		var id, title string
		var answererName, answererType *string
		var timeHours *float64
		if err := rows.Scan(&id, &title, &answererName, &answererType, &timeHours); err != nil {
			return nil, err
		}
		item := map[string]any{
			"id":    id,
			"title": title,
		}
		if answererName != nil {
			item["answerer_name"] = *answererName
		} else {
			item["answerer_name"] = "unknown"
		}
		if answererType != nil {
			item["answerer_type"] = *answererType
		} else {
			item["answerer_type"] = "unknown"
		}
		if timeHours != nil {
			item["time_to_answer_hours"] = *timeHours
		} else {
			item["time_to_answer_hours"] = 0.0
		}
		results = append(results, item)
	}

	if results == nil {
		results = []map[string]any{}
	}

	return results, rows.Err()
}

// GetTopAnswerers returns top question answerers ranked by answer count.
func (r *StatsRepository) GetTopAnswerers(ctx context.Context, limit int) ([]map[string]any, error) {
	if limit <= 0 {
		limit = 5
	}

	rows, err := r.pool.Query(ctx, `
		SELECT
			ans.author_id,
			ans.author_type,
			COUNT(*) as answer_count,
			COALESCE(
				CASE
					WHEN ans.author_type = 'agent' THEN ag.display_name
					WHEN ans.author_type = 'human' THEN u.display_name
				END,
				ans.author_id
			) as display_name,
			CASE WHEN COUNT(*) = 0 THEN 0
				ELSE (COUNT(*) FILTER (WHERE ans.is_accepted)::float / COUNT(*)::float) * 100
			END as accept_rate
		FROM answers ans
		JOIN posts p ON ans.question_id = p.id
		LEFT JOIN agents ag ON ans.author_type = 'agent' AND ans.author_id = ag.id
		LEFT JOIN users u ON ans.author_type = 'human' AND ans.author_id = u.id::text
		WHERE ans.deleted_at IS NULL
		AND p.deleted_at IS NULL
		AND p.type = 'question'
		GROUP BY ans.author_id, ans.author_type, ag.display_name, u.display_name
		ORDER BY answer_count DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []map[string]any
	for rows.Next() {
		var authorID, authorType, displayName string
		var answerCount int
		var acceptRate float64
		if err := rows.Scan(&authorID, &authorType, &answerCount, &displayName, &acceptRate); err != nil {
			return nil, err
		}
		results = append(results, map[string]any{
			"author_id":    authorID,
			"author_type":  authorType,
			"display_name": displayName,
			"answer_count": answerCount,
			"accept_rate":  acceptRate,
		})
	}

	if results == nil {
		results = []map[string]any{}
	}

	return results, rows.Err()
}
