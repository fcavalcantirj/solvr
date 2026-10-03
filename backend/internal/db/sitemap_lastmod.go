package db

import (
	"context"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// sitemapPostLastmod is a post's sitemap lastmod (task idx 83): its own last edit or
// its newest live reply, whichever is later. Views, votes and other counters never
// move posts.updated_at, so they never move this. Written against an unaliased posts.
const sitemapPostLastmod = `GREATEST(updated_at, COALESCE(
		(SELECT MAX(r.updated_at) FROM replies r WHERE r.post_id = posts.id AND r.deleted_at IS NULL),
		updated_at))`

// sitemapLastmods reads each listed type's newest lastmod, under the same eligibility
// rules as the listings, for the sitemap index.
func (r *SitemapRepository) sitemapLastmods(ctx context.Context) (models.SitemapLastmods, error) {
	var l models.SitemapLastmods
	queries := []struct {
		dest **time.Time
		sql  string
	}{
		{&l.Posts, `SELECT MAX(` + sitemapPostLastmod + `) FROM posts WHERE ` + sitemapPostEligible},
		{&l.Agents, `SELECT MAX(COALESCE(updated_at, created_at)) FROM agents WHERE ` + sitemapAgentEligible},
		{&l.BlogPosts, `SELECT MAX(updated_at) FROM blog_posts WHERE deleted_at IS NULL AND status = 'published'`},
		{&l.Rooms, `SELECT MAX(last_active_at) FROM rooms WHERE ` + roomIndexablePredicate},
	}
	for _, q := range queries {
		var t *time.Time
		if err := r.pool.QueryRow(ctx, q.sql).Scan(&t); err != nil {
			return l, err
		}
		*q.dest = t
	}
	return l, nil
}
