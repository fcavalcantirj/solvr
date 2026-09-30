package db

import (
	"context"
	"fmt"
)

// UnpublishBySlug sets a published blog post back to draft: content moderation rejected it
// (anti-abuse W2). The author edits and publishes again.
func (r *BlogPostRepository) UnpublishBySlug(ctx context.Context, slug string) error {
	if _, err := r.pool.Exec(ctx, `
		UPDATE blog_posts SET status = 'draft', updated_at = NOW()
		WHERE slug = $1 AND status = 'published' AND deleted_at IS NULL`, slug); err != nil {
		LogQueryError(ctx, "UnpublishBySlug", "blog_posts", err)
		return fmt.Errorf("unpublish blog post: %w", err)
	}
	return nil
}
