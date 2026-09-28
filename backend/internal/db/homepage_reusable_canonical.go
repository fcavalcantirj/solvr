package db

import (
	"context"
	"fmt"
)

// CanonicalHomepageRepository serves the homepage overview from the canonical posts and replies
// (task idx 76 step 3). It overrides only ListReusablePosts, the one HomepageRepository read
// that counts the legacy contribution tables; the room, activity, search and usage reads are
// the embedded repository's.
type CanonicalHomepageRepository struct {
	*HomepageRepository
}

// NewCanonicalHomepageRepository creates a CanonicalHomepageRepository.
func NewCanonicalHomepageRepository(pool *Pool) *CanonicalHomepageRepository {
	return &CanonicalHomepageRepository{HomepageRepository: NewHomepageRepository(pool)}
}

// ListReusablePosts returns public posts that already carry at least one live human or agent
// reply — migrated answers, approaches, responses, comments and progress notes and native
// replies alike, child replies included, never a system verdict. Ordered by the post's own last
// activity, as before.
func (r *CanonicalHomepageRepository) ListReusablePosts(ctx context.Context, limit int) ([]ReusablePost, error) {
	if limit <= 0 {
		limit = 6
	}

	rows, err := r.pool.Query(ctx, `
		SELECT p.id, p.type, p.title, p.status, COALESCE(p.tags, '{}'), c.contributions,
		       GREATEST(p.updated_at, COALESCE(p.created_at, p.updated_at)) AS last_activity
		  FROM posts p
		  JOIN LATERAL (
		      SELECT COUNT(*) AS contributions FROM replies r
		       WHERE r.post_id = p.id AND `+liveContributorReply+`
		  ) c ON TRUE
		 WHERE p.deleted_at IS NULL
		   AND p.visibility = 'public'
		   AND p.status NOT IN ('draft', 'pending_review', 'rejected')
		   AND c.contributions > 0
		 ORDER BY last_activity DESC
		 LIMIT $1
	`, limit)
	if err != nil {
		LogQueryError(ctx, "ListReusablePosts", "posts", err)
		return nil, fmt.Errorf("list reusable posts: %w", err)
	}
	defer rows.Close()

	out := make([]ReusablePost, 0, limit)
	for rows.Next() {
		var p ReusablePost
		if err := rows.Scan(
			&p.ID, &p.Type, &p.Title, &p.Status, &p.Tags, &p.ContributionCount, &p.LastActivityAt,
		); err != nil {
			return nil, fmt.Errorf("scan reusable post: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
