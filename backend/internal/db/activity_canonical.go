package db

import (
	"context"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// Canonical agent activity (task idx 76 step 3): GET /v1/agents/{id}/activity lists the agent's
// live public posts and every live reply it wrote, newest first, never the legacy contribution
// tables. A reply is one item type whatever it replaced (an answer, approach, response, comment
// or progress note before the move to replies, or a native reply, threaded or not):
// type "reply", action "replied", the first 100 characters of its body as title, status
// "accepted" when its post names it as the accepted reply and empty otherwise (replies carry no
// status workflow), its post's id as target, and its post's title only when the post is public.
// The total counts exactly the items the feed lists.

// canonicalAgentActivity lists the activity of agent $1.
const canonicalAgentActivity = `
	WITH activity AS (
		SELECT p.id::text AS id, 'post' AS type, 'created' AS action, p.title, p.type AS post_type,
			p.status, p.created_at, '' AS target_id, '' AS target_title
		FROM posts p
		WHERE p.posted_by_type = 'agent' AND p.posted_by_id = $1 AND p.deleted_at IS NULL
			AND p.visibility = 'public'
		UNION ALL
		SELECT r.id::text, 'reply', 'replied', LEFT(r.body, 100), '',
			CASE WHEN r.provenance->>'is_accepted' = 'true' THEN 'accepted' ELSE '' END, r.created_at,
			p.id::text, CASE WHEN p.visibility = 'public' THEN p.title ELSE '' END
		FROM replies r JOIN posts p ON p.id = r.post_id
		WHERE r.author_type = 'agent' AND r.author_id = $1 AND r.deleted_at IS NULL
	)`

// GetActivity returns a page of the agent's canonical activity and its total. It returns
// ErrAgentNotFound for an unknown or deleted agent.
func (r *CanonicalReputationAgentRepository) GetActivity(ctx context.Context, agentID string, page, perPage int) ([]models.ActivityItem, int, error) {
	if _, err := r.FindByID(ctx, agentID); err != nil {
		return nil, 0, err
	}

	rows, err := r.pool.Query(ctx, canonicalAgentActivity+`
		SELECT id, type, action, title, post_type, status, created_at, target_id, target_title
		FROM activity ORDER BY created_at DESC, id LIMIT $2 OFFSET $3`,
		agentID, perPage, (page-1)*perPage)
	if err != nil {
		LogQueryError(ctx, "CanonicalActivity.GetActivity", "replies", err)
		return nil, 0, err
	}
	defer rows.Close()

	var items []models.ActivityItem
	for rows.Next() {
		var it models.ActivityItem
		if err := rows.Scan(&it.ID, &it.Type, &it.Action, &it.Title, &it.PostType, &it.Status,
			&it.CreatedAt, &it.TargetID, &it.TargetTitle); err != nil {
			LogQueryError(ctx, "CanonicalActivity.GetActivity.Scan", "replies", err)
			return nil, 0, err
		}
		items = append(items, it)
	}
	if err := rows.Err(); err != nil {
		LogQueryError(ctx, "CanonicalActivity.GetActivity.Rows", "replies", err)
		return nil, 0, err
	}

	var total int
	if err := r.pool.QueryRow(ctx, canonicalAgentActivity+` SELECT COUNT(*)::int FROM activity`, agentID).Scan(&total); err != nil {
		LogQueryError(ctx, "CanonicalActivity.GetActivity.Count", "replies", err)
		return nil, 0, err
	}
	return items, total, nil
}
