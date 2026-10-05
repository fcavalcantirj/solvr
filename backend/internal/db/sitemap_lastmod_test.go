package db

import (
	"context"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Task idx 83: a sitemap lastmod moves with material content changes only. A post's
// lastmod is its own last edit or its newest live reply, whichever is later; a view
// or a deleted reply moves nothing. /v1/sitemap/counts names each type's newest
// lastmod, so the sitemap index can say when each sub-sitemap last changed.
func TestSitemapLastmod_FollowsMaterialChanges(t *testing.T) {
	pool, _ := newMigratedScratchDatabase(t)
	ctx := context.Background()
	authorAgent(ctx, t, pool, "agent_lastmod")
	sitemap := NewSitemapRepository(pool)
	t0 := time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)
	t1 := t0.Add(48 * time.Hour)
	t2 := t1.Add(24 * time.Hour)

	var postID string
	require.NoError(t, pool.QueryRow(ctx, `INSERT INTO posts (type, title, description, tags, status,
		posted_by_type, posted_by_id, publication_state, moderation_state, visibility, created_at, updated_at)
		VALUES ('post', 'lastmod post', 'a body for the lastmod test', ARRAY['x'], 'open', 'agent',
		'agent_lastmod', 'published', 'approved', 'public', $1, $1) RETURNING id::text`, t0).Scan(&postID))

	lastmod := func() time.Time {
		t.Helper()
		page, err := sitemap.GetPaginatedSitemapURLs(ctx, models.SitemapURLsOptions{Type: "posts", Page: 1, PerPage: 10})
		require.NoError(t, err)
		all, err := sitemap.GetSitemapURLs(ctx)
		require.NoError(t, err)
		require.Len(t, page.Posts, 1)
		require.Len(t, all.Posts, 1)
		require.True(t, page.Posts[0].UpdatedAt.Equal(all.Posts[0].UpdatedAt), "the paged and flat listings agree")
		counts, err := sitemap.GetSitemapCounts(ctx)
		require.NoError(t, err)
		require.NotNil(t, counts.Lastmod.Posts)
		require.True(t, counts.Lastmod.Posts.Equal(page.Posts[0].UpdatedAt), "counts name the newest post lastmod")
		return page.Posts[0].UpdatedAt.UTC()
	}
	assert.Equal(t, t0, lastmod(), "an untouched post: its own time")

	_, err := pool.Exec(ctx, `INSERT INTO replies (post_id, author_type, author_id, body, created_at, updated_at)
		VALUES ($1, 'agent', 'agent_lastmod', 'a reply', $2, $2)`, postID, t1)
	require.NoError(t, err)
	assert.Equal(t, t1, lastmod(), "a new reply is a material change")

	_, err = pool.Exec(ctx, `INSERT INTO replies (post_id, author_type, author_id, body, created_at, updated_at, deleted_at)
		VALUES ($1, 'agent', 'agent_lastmod', 'a deleted reply', $2, $2, $2)`, postID, t2)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE posts SET view_count = view_count + 1 WHERE id = $1`, postID)
	require.NoError(t, err)
	assert.Equal(t, t1, lastmod(), "a deleted reply and a view move nothing")

	counts, err := sitemap.GetSitemapCounts(ctx)
	require.NoError(t, err)
	assert.Nil(t, counts.Lastmod.Rooms, "no indexable room, no rooms lastmod")
	assert.Nil(t, counts.Lastmod.Users, "no indexable person, no users lastmod")
}
