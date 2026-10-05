package db

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// SPEC.md 27.1, blog posts: the schema leaves excerpt, cover_image_url, meta_description, tags
// and the counters nullable. A row written by SQL or an older writer with NULL there is a post:
// every read answers it with empty text and zero counts, never "cannot scan NULL" (which the
// API turned into a 500 for GET /v1/blog/{slug}).
func TestBlogReads_AnswerARowWithNullColumns(t *testing.T) {
	pool, _ := newMigratedScratchDatabase(t)
	ctx := context.Background()
	repo := NewBlogPostRepository(pool)
	slug := fmt.Sprintf("null-columns-%d", time.Now().UnixNano())
	_, err := pool.Exec(ctx, `INSERT INTO blog_posts
		(slug, title, body, excerpt, tags, cover_image_url, meta_description, view_count, upvotes, downvotes,
		 read_time_minutes, posted_by_type, posted_by_id, status, published_at)
		VALUES ($1, 'A post with NULL columns', 'A body that is long enough to be a blog post body.',
		        NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, 'agent', 'agent_null_cols', 'published', NOW())`, slug)
	require.NoError(t, err)

	check := func(t *testing.T, p *models.BlogPostWithAuthor) {
		t.Helper()
		assert.Equal(t, slug, p.Slug)
		assert.Empty(t, p.Excerpt)
		assert.Empty(t, p.CoverImageURL)
		assert.Empty(t, p.MetaDescription)
		assert.Zero(t, p.ViewCount)
		assert.Zero(t, p.VoteScore)
		assert.Equal(t, 1, p.ReadTimeMinutes, "a NULL read time reads as the column's default")
	}

	post, err := repo.FindBySlug(ctx, slug)
	require.NoError(t, err, "the read by slug")
	check(t, post)

	post, err = repo.FindBySlugForViewer(ctx, slug, models.AuthorTypeHuman, "00000000-0000-0000-0000-000000000001")
	require.NoError(t, err, "the read by slug for a signed-in viewer")
	check(t, post)

	list, total, err := repo.List(ctx, models.BlogPostListOptions{Page: 1, PerPage: 10})
	require.NoError(t, err, "the list")
	require.Equal(t, 1, total)
	check(t, &list[0])

	featured, err := repo.GetFeatured(ctx)
	require.NoError(t, err, "the featured read")
	require.NotNil(t, featured)
	check(t, featured)

	// An update of such a row answers from its RETURNING columns, the counters still NULL.
	updated := post.BlogPost
	updated.Title = "A post with NULL columns, edited"
	got, err := repo.Update(ctx, &updated)
	require.NoError(t, err, "the update")
	assert.Equal(t, "A post with NULL columns, edited", got.Title)
	assert.Zero(t, got.ViewCount)
}
