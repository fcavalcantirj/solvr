package db

import (
	"context"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// SPEC.md 27.2: GET /v1/posts?indexable=true lists exactly the posts the post sitemap lists.
// The post archive pages and the profile pages link what it lists, so a post the sitemap
// names and the list leaves out would stay without an inbound link, and a post the list
// names and the sitemap leaves out would be linked into a noindex page.

// The rule is written once. The sitemap reads it against an unaliased posts table, the list
// against "p": the two texts differ by that alias and nothing else.
func TestPostIndexablePredicate_IsOneRuleForTheSitemapAndTheList(t *testing.T) {
	assert.Equal(t, sitemapPostEligible, postIndexablePredicate(""))
	assert.Equal(t, sitemapPostEligible, strings.ReplaceAll(postIndexablePredicate("p"), "p.", ""))
	for _, column := range []string{"deleted_at", "visibility", "publication_state", "moderation_state", "status"} {
		assert.Contains(t, postIndexablePredicate("p"), "p."+column, "the list's form qualifies %s", column)
		assert.NotContains(t, sitemapPostEligible, "."+column, "the sitemap's form is unaliased")
	}
}

// indexableListIDs reads every page of the indexable list at perPage and returns the ids in
// the order served, with the total each page reported.
func indexableListIDs(ctx context.Context, t *testing.T, repo *PostRepository, opts models.PostListOptions, perPage int) (ids []string, total int) {
	t.Helper()
	opts.Indexable = true
	opts.PerPage = perPage
	ids = []string{}
	for page := 1; page <= 50; page++ {
		opts.Page = page
		posts, pageTotal, err := repo.List(ctx, opts)
		require.NoError(t, err)
		if page == 1 {
			total = pageTotal
		}
		assert.Equal(t, total, pageTotal, "page %d reports the same total", page)
		if len(posts) == 0 {
			return ids, total
		}
		for _, p := range posts {
			ids = append(ids, p.ID)
		}
	}
	t.Fatal("the indexable list never ended")
	return nil, 0
}

func sortedCopy(ids []string) []string {
	out := append([]string{}, ids...)
	sort.Strings(out)
	return out
}

func TestPostList_Indexable_ListsExactlyWhatTheSitemapLists(t *testing.T) {
	pool, _ := newMigratedScratchDatabase(t)
	ctx := context.Background()
	agent := authorAgent(ctx, t, pool, "agent_archive_a")
	other := authorAgent(ctx, t, pool, "agent_archive_b")
	owner := authorHuman(ctx, t, pool, "Archive Owner")
	posts := NewPostRepository(pool)
	sitemap := NewSitemapRepository(pool)

	base := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	n := 0
	// post inserts one post; each call is a minute newer than the one before unless at is given.
	post := func(author, status, pub, mod, visibility string, deleted bool, at ...time.Time) string {
		t.Helper()
		n++
		created := base.Add(time.Duration(n) * time.Minute)
		if len(at) > 0 {
			created = at[0]
		}
		var ownerID, deletedAt any
		if visibility == "family" {
			ownerID = owner
		}
		if deleted {
			deletedAt = created
		}
		var id string
		require.NoError(t, pool.QueryRow(ctx, `INSERT INTO posts (type, title, description, tags, status,
			posted_by_type, posted_by_id, publication_state, moderation_state, visibility,
			owner_human_id, deleted_at, created_at, updated_at)
			VALUES ('post', $1, 'a post body for the archive list test', ARRAY['archive'], $2, 'agent',
			$3, $4, $5, $6, $7, $8, $9, $9) RETURNING id::text`,
			"archive list "+status+" "+pub+" "+mod+" "+visibility, status, author, pub, mod, visibility,
			ownerID, deletedAt, created).Scan(&id))
		return id
	}

	// Listed by the sitemap, oldest first.
	e1 := post(agent, "open", "published", "approved", "public", false)
	e2 := post(agent, "closed", "published", "approved", "public", false)
	e3 := post(other, "stale", "published", "approved", "public", false)
	e4 := post(agent, "open", "published", "approved", "public", false)
	// Never listed by the sitemap: each fails one part of the rule.
	post(agent, "draft", "draft", "pending", "public", false)
	post(agent, "pending_review", "draft", "pending", "public", false)
	post(agent, "rejected", "draft", "rejected", "public", false)
	archived := post(agent, "closed", "archived", "approved", "public", false)
	post(agent, "open", "published", "approved", "public", true)
	family := post(agent, "open", "published", "approved", "family", false)
	post(agent, "open", "draft", "pending", "public", false)
	awaitingModeration := post(agent, "open", "published", "pending", "public", false)
	hiddenStatusOnly := post(agent, "rejected", "published", "approved", "public", false)
	post(agent, "open", "published", "rejected", "public", false)

	urls, err := sitemap.GetSitemapURLs(ctx)
	require.NoError(t, err)
	listed, _ := sitemapPostIDs(urls.Posts)
	require.ElementsMatch(t, []string{e1, e2, e3, e4}, listed, "the fixture is what the sitemap lists")
	counts, err := sitemap.GetSitemapCounts(ctx)
	require.NoError(t, err)

	t.Run("the list and the sitemap name the same posts, newest first", func(t *testing.T) {
		ids, total := indexableListIDs(ctx, t, posts, models.PostListOptions{Sort: "new"}, 50)
		assert.Equal(t, []string{e4, e3, e2, e1}, ids)
		assert.Equal(t, sortedCopy(listed), sortedCopy(ids), "post by post, the sitemap's listing")
		assert.Equal(t, counts.Posts, total, "meta.total is the sitemap's count")
	})

	t.Run("without the filter the list is wider than the sitemap", func(t *testing.T) {
		plain, _, err := posts.List(ctx, models.PostListOptions{Sort: "new", Page: 1, PerPage: 50})
		require.NoError(t, err)
		var ids []string
		for _, p := range plain {
			ids = append(ids, p.ID)
		}
		assert.Contains(t, ids, awaitingModeration, "the plain list shows a post awaiting moderation")
		assert.Contains(t, ids, archived, "the plain list shows an archived post")
		assert.NotContains(t, listed, awaitingModeration)
		assert.NotContains(t, listed, archived)
	})

	t.Run("it only narrows: an author's self-view gets no hidden post", func(t *testing.T) {
		own := models.PostListOptions{Sort: "new", AuthorType: models.AuthorTypeAgent, AuthorID: agent, IncludeHidden: true}
		ids, total := indexableListIDs(ctx, t, posts, own, 50)
		assert.Equal(t, []string{e4, e2, e1}, ids)
		assert.Equal(t, 3, total)
		assert.NotContains(t, ids, hiddenStatusOnly)

		// The same self-view without the filter does show the hidden posts.
		own.Page, own.PerPage = 1, 50
		all, _, err := posts.List(ctx, own)
		require.NoError(t, err)
		var allIDs []string
		for _, p := range all {
			allIDs = append(allIDs, p.ID)
		}
		assert.Contains(t, allIDs, hiddenStatusOnly)
	})

	t.Run("it only narrows: a family member gets no family post", func(t *testing.T) {
		ids, _ := indexableListIDs(ctx, t, posts, models.PostListOptions{Sort: "new", ViewerHuman: owner}, 50)
		assert.Equal(t, []string{e4, e3, e2, e1}, ids)

		seen, _, err := posts.List(ctx, models.PostListOptions{Sort: "new", ViewerHuman: owner, Page: 1, PerPage: 50})
		require.NoError(t, err)
		var seenIDs []string
		for _, p := range seen {
			seenIDs = append(seenIDs, p.ID)
		}
		assert.Contains(t, seenIDs, family, "without the filter the family reads its own post")
	})

	t.Run("it combines with the author filter", func(t *testing.T) {
		ids, total := indexableListIDs(ctx, t, posts,
			models.PostListOptions{Sort: "new", AuthorType: models.AuthorTypeAgent, AuthorID: other}, 50)
		assert.Equal(t, []string{e3}, ids)
		assert.Equal(t, 1, total)
	})

	t.Run("numbered pages hold each post exactly once", func(t *testing.T) {
		ids, total := indexableListIDs(ctx, t, posts, models.PostListOptions{Sort: "new"}, 3)
		assert.Equal(t, []string{e4, e3, e2, e1}, ids)
		assert.Equal(t, 4, total)
	})

	// Posts written in one statement share a timestamp. Ordered by time alone, the database
	// may serve a tie in a different order on each page read, so a post can show on two
	// pages and another on none. The order ends on the id.
	t.Run("posts that share a timestamp are still each on one page, in id order", func(t *testing.T) {
		same := base.Add(2 * time.Hour)
		var tied []string
		for i := 0; i < 7; i++ {
			tied = append(tied, post(agent, "open", "published", "approved", "public", false, same))
		}
		sort.Sort(sort.Reverse(sort.StringSlice(tied)))
		want := append(append([]string{}, tied...), e4, e3, e2, e1)

		for _, perPage := range []int{2, 3, 50} {
			ids, total := indexableListIDs(ctx, t, posts, models.PostListOptions{Sort: "new"}, perPage)
			assert.Equal(t, want, ids, "per_page %d", perPage)
			assert.Equal(t, len(want), total)
		}

		urls, err := sitemap.GetSitemapURLs(ctx)
		require.NoError(t, err)
		listed, _ := sitemapPostIDs(urls.Posts)
		assert.Equal(t, sortedCopy(listed), sortedCopy(want))
	})
}
