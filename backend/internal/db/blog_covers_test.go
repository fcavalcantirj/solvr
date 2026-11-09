package db

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// A blog post's generated cover (000148): one row per post, found by the post's slug for
// GET /v1/blog/{slug}/cover.jpg, listed for the backfill as the published posts that have
// none, and named on the post as its cover_image_url once stored.
func TestBlogCovers_SaveFindListAndName(t *testing.T) {
	pool, _ := newMigratedScratchDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	posts := NewBlogPostRepository(pool)
	covers := NewBlogCoverRepository(pool)
	mk := func(slug string, status models.BlogPostStatus) *models.BlogPost {
		p, err := posts.Create(ctx, &models.BlogPost{Slug: slug, Title: "Post " + slug, Body: "Body of " + slug, Excerpt: "E",
			Tags: []string{"esp32"}, PostedByType: models.AuthorTypeAgent, PostedByID: "agent_cover_test", Status: status})
		require.NoError(t, err)
		return p
	}
	a, b, draft := mk("cover-a", models.BlogPostStatusPublished), mk("cover-b", models.BlogPostStatusPublished), mk("cover-draft", models.BlogPostStatusDraft)

	missing, err := covers.ListPublishedWithoutCover(ctx, 0, 10)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{a.Slug, b.Slug}, slugsOf(missing), "drafts are not listed; %s is", draft.Slug)

	// The grace period: a post published less than 24 h ago is not listed yet.
	grace, err := covers.ListPublishedWithoutCover(ctx, 24*time.Hour, 10)
	require.NoError(t, err)
	assert.Empty(t, grace, "both were published just now")

	img := []byte{0xFF, 0xD8, 0xFF, 0xE0, 1, 2, 3}
	require.NoError(t, covers.Save(ctx, a.ID, &BlogCover{Image: img, ContentType: "image/jpeg", Model: "gpt-image-2", Prompt: "p"}))
	require.NoError(t, covers.NameCover(ctx, a.ID, "https://api.solvr.dev/v1/blog/cover-a/cover.jpg"))

	found, err := covers.FindBySlug(ctx, "cover-a")
	require.NoError(t, err)
	assert.Equal(t, img, found.Image)
	assert.Equal(t, "image/jpeg", found.ContentType)
	assert.Equal(t, "gpt-image-2", found.Model)
	assert.False(t, found.CreatedAt.IsZero())

	_, err = covers.FindBySlug(ctx, "cover-b")
	assert.ErrorIs(t, err, ErrBlogCoverNotFound)
	_, err = covers.FindBySlug(ctx, "no-such-post")
	assert.ErrorIs(t, err, ErrBlogCoverNotFound)

	named, err := posts.FindBySlug(ctx, "cover-a")
	require.NoError(t, err)
	assert.Equal(t, "https://api.solvr.dev/v1/blog/cover-a/cover.jpg", named.CoverImageURL)

	missing, err = covers.ListPublishedWithoutCover(ctx, 0, 10)
	require.NoError(t, err)
	assert.Equal(t, []string{b.Slug}, slugsOf(missing))
	assert.Equal(t, "Post cover-b", missing[0].Title)
	assert.Equal(t, []string{"esp32"}, missing[0].Tags)

	// Three failed tries take a post off the job's list; the operator route is not bound by it.
	for i := 0; i < MaxCoverAttempts; i++ {
		require.NoError(t, covers.IncrementCoverAttempts(ctx, b.ID))
	}
	missing, err = covers.ListPublishedWithoutCover(ctx, 0, 10)
	require.NoError(t, err)
	assert.Empty(t, missing)

	// The author removes the cover: the row is gone and the URL that named it is cleared.
	require.NoError(t, covers.Delete(ctx, a.ID, "https://api.solvr.dev/v1/blog/cover-a/cover.jpg"))
	_, err = covers.FindBySlug(ctx, "cover-a")
	assert.ErrorIs(t, err, ErrBlogCoverNotFound)
	cleared, err := posts.FindBySlug(ctx, "cover-a")
	require.NoError(t, err)
	assert.Equal(t, "", cleared.CoverImageURL)
	require.NoError(t, covers.Save(ctx, a.ID, &BlogCover{Image: img, ContentType: "image/jpeg", Model: "gpt-image-2", Prompt: "p"}))

	// Saving again replaces the cover (a regeneration).
	img2 := []byte{0xFF, 0xD8, 0xFF, 0xE1, 9}
	require.NoError(t, covers.Save(ctx, a.ID, &BlogCover{Image: img2, ContentType: "image/jpeg", Model: "gpt-image-2", Prompt: "p2"}))
	found, err = covers.FindBySlug(ctx, "cover-a")
	require.NoError(t, err)
	assert.Equal(t, img2, found.Image)

	// A deleted post's cover is gone with it.
	require.NoError(t, posts.Delete(ctx, "cover-a"))
	_, err = covers.FindBySlug(ctx, "cover-a")
	assert.ErrorIs(t, err, ErrBlogCoverNotFound)
}

func slugsOf(briefs []BlogCoverBrief) []string {
	out := make([]string, 0, len(briefs))
	for _, b := range briefs {
		out = append(out, b.Slug)
	}
	return out
}
