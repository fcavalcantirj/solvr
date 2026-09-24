package db

import "context"

// KnowledgeTypes is the order GET /v1/overview publishes knowledge aggregates in.
var KnowledgeTypes = []string{"problem", "question", "idea", "post"}

// KnowledgeTypeTotals is the knowledge aggregate for one post type. Every figure counts
// what an anonymous GET /v1/posts lists: public, not deleted, not pending_review,
// rejected or draft. Reply figures read the canonical replies table.
type KnowledgeTypeTotals struct {
	Type              string
	Total             int
	ByStatus          map[string]int
	WithReplies       int // posts with at least one non-deleted reply
	WithAcceptedReply int // posts with an accepted reply (accepted_answer_id set)
	Replies           int // non-deleted replies on those posts
}

// GetKnowledgeTotals returns one entry per KnowledgeTypes type, zero-filled, in order.
// It is the single query behind the overview knowledge section and the legacy
// type-specific statistics adapters (task idx 72).
func (r *StatsRepository) GetKnowledgeTotals(ctx context.Context) ([]KnowledgeTypeTotals, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT p.type, p.status, COUNT(*),
			COUNT(*) FILTER (WHERE rc.n > 0),
			COUNT(*) FILTER (WHERE p.accepted_answer_id IS NOT NULL),
			COALESCE(SUM(rc.n), 0)
		FROM posts p
		LEFT JOIN (
			SELECT post_id, COUNT(*) AS n FROM replies WHERE deleted_at IS NULL GROUP BY post_id
		) rc ON rc.post_id = p.id
		WHERE p.deleted_at IS NULL
			AND p.visibility = 'public'
			AND p.status NOT IN ('pending_review', 'rejected', 'draft')
		GROUP BY p.type, p.status
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	byType := make(map[string]*KnowledgeTypeTotals, len(KnowledgeTypes))
	out := make([]KnowledgeTypeTotals, len(KnowledgeTypes))
	for i, t := range KnowledgeTypes {
		out[i] = KnowledgeTypeTotals{Type: t, ByStatus: map[string]int{}}
		byType[t] = &out[i]
	}
	for rows.Next() {
		var postType, status string
		var total, withReplies, withAccepted, replies int
		if err := rows.Scan(&postType, &status, &total, &withReplies, &withAccepted, &replies); err != nil {
			return nil, err
		}
		k, ok := byType[postType]
		if !ok {
			continue
		}
		k.Total += total
		k.ByStatus[status] = total
		k.WithReplies += withReplies
		k.WithAcceptedReply += withAccepted
		k.Replies += replies
	}
	return out, rows.Err()
}
