package db

import (
	"context"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// legacyActivityOracle is the legacy agent activity feed, AgentRepository.GetActivity as it was
// on e726cb7f, kept verbatim as a test oracle when the method was deleted with the legacy tables
// (idx 68): the cutover test compares the canonical feed after the cutover with what this one
// listed before it. It reads answers, approaches and responses, so it only runs below the
// legacy archive.
type legacyActivityOracle struct{ *AgentRepository }

func (r legacyActivityOracle) GetActivity(ctx context.Context, agentID string, page, perPage int) ([]models.ActivityItem, int, error) {
	// First verify agent exists
	_, err := r.FindByID(ctx, agentID)
	if err != nil {
		return nil, 0, err
	}

	// Calculate offset
	offset := (page - 1) * perPage

	// Query to get activity items - combines posts, answers, approaches
	// Uses UNION ALL to combine results from different tables
	query := `
		WITH activity AS (
			-- Posts created by agent
			SELECT
				p.id::text,
				'post' as type,
				'created' as action,
				p.title,
				p.type as post_type,
				p.status,
				p.created_at,
				'' as target_id,
				'' as target_title
			FROM posts p
			WHERE p.posted_by_type = 'agent' AND p.posted_by_id = $1 AND p.deleted_at IS NULL
			  AND p.visibility = 'public' -- BART-151: public activity feed

			UNION ALL

			-- Answers by agent
			SELECT
				a.id::text,
				'answer' as type,
				'answered' as action,
				LEFT(a.content, 100) as title,
				'' as post_type,
				CASE WHEN a.is_accepted THEN 'accepted' ELSE 'pending' END as status,
				a.created_at,
				p.id::text as target_id,
				CASE WHEN p.visibility = 'public' THEN p.title ELSE '' END as target_title
			FROM answers a
			JOIN posts p ON a.question_id = p.id
			WHERE a.author_type = 'agent' AND a.author_id = $1 AND a.deleted_at IS NULL

			UNION ALL

			-- Approaches by agent
			SELECT
				ap.id::text,
				'approach' as type,
				'started_approach' as action,
				ap.angle as title,
				'' as post_type,
				ap.status,
				ap.created_at,
				p.id::text as target_id,
				CASE WHEN p.visibility = 'public' THEN p.title ELSE '' END as target_title
			FROM approaches ap
			JOIN posts p ON ap.problem_id = p.id
			WHERE ap.author_type = 'agent' AND ap.author_id = $1 AND ap.deleted_at IS NULL

			UNION ALL

			-- Responses to ideas by agent
			SELECT
				r.id::text,
				'response' as type,
				'responded' as action,
				LEFT(r.content, 100) as title,
				'' as post_type,
				'' as status,
				r.created_at,
				p.id::text as target_id,
				CASE WHEN p.visibility = 'public' THEN p.title ELSE '' END as target_title
			FROM responses r
			JOIN posts p ON r.idea_id = p.id
			WHERE r.author_type = 'agent' AND r.author_id = $1
		)
		SELECT id, type, action, title, post_type, status, created_at, target_id, target_title
		FROM activity
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.pool.Query(ctx, query, agentID, perPage, offset)
	if err != nil {
		LogQueryError(ctx, "GetActivity", "activity", err)
		return nil, 0, err
	}
	defer rows.Close()

	var items []models.ActivityItem
	for rows.Next() {
		var item models.ActivityItem
		err := rows.Scan(
			&item.ID,
			&item.Type,
			&item.Action,
			&item.Title,
			&item.PostType,
			&item.Status,
			&item.CreatedAt,
			&item.TargetID,
			&item.TargetTitle,
		)
		if err != nil {
			LogQueryError(ctx, "GetActivity.Scan", "activity", err)
			return nil, 0, err
		}
		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		LogQueryError(ctx, "GetActivity.Rows", "activity", err)
		return nil, 0, err
	}

	// Count total items
	countQuery := `
		SELECT COUNT(*) FROM (
			SELECT 1 FROM posts WHERE posted_by_type = 'agent' AND posted_by_id = $1 AND deleted_at IS NULL
			UNION ALL
			SELECT 1 FROM answers WHERE author_type = 'agent' AND author_id = $1 AND deleted_at IS NULL
			UNION ALL
			SELECT 1 FROM approaches WHERE author_type = 'agent' AND author_id = $1 AND deleted_at IS NULL
			UNION ALL
			SELECT 1 FROM responses WHERE author_type = 'agent' AND author_id = $1
		) as counts
	`

	var total int
	err = r.pool.QueryRow(ctx, countQuery, agentID).Scan(&total)
	if err != nil {
		LogQueryError(ctx, "GetActivity.Count", "activity", err)
		return nil, 0, err
	}

	return items, total, nil
}
