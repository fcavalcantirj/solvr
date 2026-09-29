package db

import (
	"context"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Task idx 76 step 3: the sitemap decides post eligibility without comparing a legacy post
// type, so sitemap.go carries no disposition.
func TestLegacySitemap_ServedCanonically(t *testing.T) {
	src, err := ScanLegacySourceDependencies(backendRoot(t))
	require.NoError(t, err)
	for _, dep := range src {
		assert.NotEqual(t, "code:internal/db/sitemap.go", dep.Key, "the sitemap reads the canonical post states")
	}
	_, ok := LegacyDependencyDispositions["code:internal/db/sitemap.go"]
	assert.False(t, ok, "sitemap.go no longer depends on the legacy model, so it carries no disposition")
}

// sitemapPostIDs is what one sitemap listing says about posts: ids in order and their types.
func sitemapPostIDs(posts []models.SitemapPost) (ids []string, types map[string]string) {
	ids = []string{}
	types = map[string]string{}
	for _, p := range posts {
		ids = append(ids, p.ID)
		types[p.ID] = p.Type
	}
	return ids, types
}

// Task idx 76 step 3: GET /v1/sitemap/urls and /v1/sitemap/counts list a post when it is
// publicly eligible under the canonical rule (models.Post.PublicEligible: published,
// moderation-approved, public, not deleted), whatever its legacy type, votes or solved
// state; a draft, pending, rejected, archived, family or deleted post never. The legacy
// hidden statuses stay excluded too, so a row whose status alone says rejected is not
// listed. Everything keeps working once the legacy tables are gone and every post is the
// canonical type.
func TestCanonicalSitemap_ListsEveryPublicEligiblePostAcrossTheCutover(t *testing.T) {
	pool, dropLegacy := newMigratedScratchDatabase(t)
	ctx := context.Background()
	repo := NewSitemapRepository(pool)
	var owner string
	require.NoError(t, pool.QueryRow(ctx, `INSERT INTO users (username, display_name, email, referral_code)
		VALUES ('sitemapcanon', 'sitemapcanon', 'sitemapcanon@example.com', 'SMAPCANO') RETURNING id::text`).Scan(&owner))
	base := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	n := 0
	post := func(postType, status, pub, mod, visibility string, up, down int, deleted bool) string {
		t.Helper()
		n++
		var ownerID any
		if visibility == "family" {
			ownerID = owner
		}
		var deletedAt any
		if deleted {
			deletedAt = base
		}
		var id string
		require.NoError(t, pool.QueryRow(ctx, `INSERT INTO posts (type, title, description, tags, status,
			posted_by_type, posted_by_id, upvotes, downvotes, publication_state, moderation_state,
			visibility, owner_human_id, deleted_at, created_at, updated_at)
			VALUES ($1, $2, 'a post body for the sitemap canon test', ARRAY['sitemap'], $3, 'agent',
			'agent_sitemap_canon', $4, $5, $6, $7, $8, $9, $10, $11, $11) RETURNING id::text`,
			postType, "sitemap canon "+postType+" "+status, status, up, down, pub, mod, visibility,
			ownerID, deletedAt, base.Add(time.Duration(n)*time.Minute)).Scan(&id))
		return id
	}

	// Publicly eligible (listed), oldest first. The legacy rule listed only e2 and e4.
	e1 := post("problem", "open", "published", "approved", "public", 0, 0, false)   // unsolved, no votes
	e2 := post("problem", "solved", "published", "approved", "public", 0, 0, false) // solved
	e3 := post("idea", "active", "published", "approved", "public", 1, 0, false)    // net 1
	e4 := post("question", "open", "published", "approved", "public", 0, 0, false)
	e5 := post("post", "open", "published", "approved", "public", 0, 0, false) // canonical type
	// Never listed. x4 (closed) was listed by the legacy rule.
	post("question", "draft", "draft", "pending", "public", 3, 0, false)
	post("problem", "pending_review", "draft", "pending", "public", 3, 0, false)
	post("idea", "rejected", "draft", "rejected", "public", 5, 0, false)
	x4 := post("question", "closed", "archived", "approved", "public", 3, 0, false)
	post("problem", "solved", "published", "approved", "public", 3, 0, true) // deleted
	post("question", "open", "published", "approved", "family", 3, 0, false) // family
	// Rows whose legacy status and canonical states disagree: the canonical states decide,
	// and the legacy hidden statuses stay hidden. Publication alone never lists a post.
	post("question", "open", "draft", "pending", "public", 3, 0, false)
	post("question", "open", "published", "pending", "public", 3, 0, false)
	post("question", "rejected", "published", "approved", "public", 3, 0, false)

	want := []string{e5, e4, e3, e2, e1} // updated_at DESC
	wantTypes := map[string]string{e1: "problem", e2: "problem", e3: "idea", e4: "question", e5: "post"}
	check := func(stage string, types map[string]string) {
		t.Helper()
		urls, err := repo.GetSitemapURLs(ctx)
		require.NoError(t, err, stage)
		ids, gotTypes := sitemapPostIDs(urls.Posts)
		assert.Equal(t, want, ids, "%s: GetSitemapURLs lists the public eligible posts newest first", stage)
		assert.Equal(t, types, gotTypes, "%s: each entry carries its post's type", stage)
		assert.NotContains(t, ids, x4, "%s: an archived (closed) post is not listed", stage)

		counts, err := repo.GetSitemapCounts(ctx)
		require.NoError(t, err, stage)
		assert.Equal(t, len(want), counts.Posts, "%s: the posts count uses the same rule", stage)

		var paged []string
		for page := 1; page <= 4; page++ {
			res, err := repo.GetPaginatedSitemapURLs(ctx, models.SitemapURLsOptions{Type: "posts", Page: page, PerPage: 2})
			require.NoError(t, err, stage)
			ids, _ := sitemapPostIDs(res.Posts)
			switch page {
			case 3:
				assert.Equal(t, []string{e1}, ids, "%s: the last page holds the oldest eligible post", stage)
			case 4:
				assert.Empty(t, ids, "%s: past the end is empty", stage)
			default:
				assert.Len(t, ids, 2, "%s: page %d is full", stage, page)
			}
			paged = append(paged, ids...)
		}
		assert.Equal(t, want, paged, "%s: the paginated listing uses the same rule and order", stage)
	}

	check("before the cutover", wantTypes)
	dropLegacy()
	check("legacy tables dropped", wantTypes)

	// Schema cleanup narrows every post to the canonical type: the listing does not change.
	_, err := pool.Exec(ctx, `UPDATE posts SET type = 'post'`)
	require.NoError(t, err)
	allPost := map[string]string{}
	for _, id := range want {
		allPost[id] = "post"
	}
	check("every post the canonical type", allPost)
}
