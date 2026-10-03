package db

import (
	"context"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// KnowledgeTypes is the order GET /v1/overview publishes knowledge aggregates in: one entry,
// "post", since the legacy post types were retired (idx 68).
var KnowledgeTypes = []string{string(models.PostTypePost)}

// KnowledgeTypeTotals is the knowledge aggregate for one post type. Every figure counts
// what an anonymous GET /v1/posts lists: public, not deleted, not pending_review,
// rejected or draft. Reply figures read the canonical replies table.
type KnowledgeTypeTotals struct {
	Type        string
	Total       int
	ByStatus    map[string]int
	WithReplies int // posts with at least one non-deleted reply
	Replies     int // non-deleted replies on those posts
}

// GetKnowledgeTotals returns one entry per KnowledgeTypes type, zero-filled, in order. Every
// listed post counts under "post", whatever type a row still stores before the legacy archive
// migration relabels it. It is the single query behind the overview knowledge section.
func (r *StatsRepository) GetKnowledgeTotals(ctx context.Context) ([]KnowledgeTypeTotals, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT p.status, COUNT(*),
			COUNT(*) FILTER (WHERE rc.n > 0),
			COALESCE(SUM(rc.n), 0)
		FROM posts p
		LEFT JOIN (
			SELECT post_id, COUNT(*) AS n FROM replies WHERE deleted_at IS NULL GROUP BY post_id
		) rc ON rc.post_id = p.id
		WHERE p.deleted_at IS NULL
			AND p.visibility = 'public'
			AND p.status NOT IN ('pending_review', 'rejected', 'draft')
		GROUP BY p.status
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	k := KnowledgeTypeTotals{Type: string(models.PostTypePost), ByStatus: map[string]int{}}
	for rows.Next() {
		var status string
		var total, withReplies, replies int
		if err := rows.Scan(&status, &total, &withReplies, &replies); err != nil {
			return nil, err
		}
		k.Total += total
		k.ByStatus[status] = total
		k.WithReplies += withReplies
		k.Replies += replies
	}
	return []KnowledgeTypeTotals{k}, rows.Err()
}
