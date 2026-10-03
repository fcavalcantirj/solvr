package db

import (
	"context"
	"fmt"
)

// legacyReusableOracle is the legacy overview reusable-posts read, HomepageRepository.
// ListReusablePosts as it was on e726cb7f, kept verbatim as a test oracle when the method was
// deleted with the legacy tables (idx 68): the cutover test compares the canonical list after
// the cutover with what this one listed before it. It reads approaches, answers and responses,
// so it only runs below the legacy archive.
type legacyReusableOracle struct{ *HomepageRepository }

func (r legacyReusableOracle) ListReusablePosts(ctx context.Context, limit int) ([]ReusablePost, error) {
	if limit <= 0 {
		limit = 6
	}

	rows, err := r.pool.Query(ctx, `
		SELECT p.id, p.type, p.title, p.status, COALESCE(p.tags, '{}'), c.contributions,
		       GREATEST(p.updated_at, COALESCE(p.created_at, p.updated_at)) AS last_activity
		  FROM posts p
		  JOIN LATERAL (
		      SELECT
		          (SELECT COUNT(*) FROM approaches a
		            WHERE a.problem_id = p.id AND a.deleted_at IS NULL)
		        + (SELECT COUNT(*) FROM answers an
		            WHERE an.question_id = p.id AND an.deleted_at IS NULL)
		        + (SELECT COUNT(*) FROM responses re WHERE re.idea_id = p.id)
		          AS contributions
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
