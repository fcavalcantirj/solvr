package db

import (
	"context"
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/stretchr/testify/require"
)

// Keyword search pages its results in two phases (search_keyword_page.go). Measured before this
// change (slice 20 spike: 50k posts, 250k replies, no embedder), the keyword search for a term in
// every post built every match's snippet, author and reply counts, and every matching reply's
// snippet and author, to return 20 of them.

// seedKeywordPage adds to seedSearchDocuments' 2,000 posts and 6,000 replies 20,000 replies that
// no query of these tests matches, "zeromq" in 1 in 200 of the other replies and in no post (posts
// found only through replies), 1 in 40 replies deleted, votes and activity times that differ per
// post, and a tenth of the posts in sd1's family.
func seedKeywordPage(ctx context.Context, t *testing.T, pool *Pool) {
	t.Helper()
	seedSearchDocuments(ctx, t, pool)
	for _, step := range []struct{ name, sql string }{
		{"filler", `WITH listed AS (SELECT id, row_number() OVER (ORDER BY created_at DESC) rn FROM posts)
			INSERT INTO replies (post_id, author_type, author_id, body, created_at)
			SELECT p.id, 'agent', 'sd_agent_' || (1 + g % 10), 'filler note number ' || g,
			       NOW() - g * INTERVAL '1 second'
			  FROM generate_series(1, 20000) g JOIN listed p ON p.rn = 1 + g % 2000`},
		{"rare", `UPDATE replies SET body = body || ' zeromq'
			WHERE body NOT LIKE 'filler note%' AND hashtext(id::text) % 200 = 3`},
		{"deleted", `UPDATE replies SET deleted_at = NOW() WHERE hashtext(id::text) % 40 = 1`},
		{"votes", `UPDATE posts SET upvotes = hashtext(id::text) % 7 + 7, updated_at = created_at + (hashtext(id::text) % 997) * INTERVAL '1 second'`},
		{"family", `UPDATE posts SET visibility = 'family', owner_human_id = (SELECT id FROM users WHERE username = 'sd1')
			WHERE hashtext(id::text) % 10 = 4`},
		{"analyze", `ANALYZE posts, replies`},
	} {
		_, err := pool.Exec(ctx, step.sql)
		require.NoError(t, err, "seed %s", step.name)
	}
}

// fullKeywordSearch is the keyword search's definition: every result built (searchKnowledge, or
// searchPosts for content_types=posts), stably sorted by score, then paged.
func fullKeywordSearch(ctx context.Context, t *testing.T, repo *SearchRepository, q string, opts models.SearchOptions) ([]models.SearchResult, int) {
	t.Helper()
	tsq := buildTsQuery(q)
	var (
		all []models.SearchResult
		err error
	)
	if len(opts.ContentTypes) == 0 {
		all, err = repo.searchKnowledge(ctx, nil, tsq, opts)
	} else {
		all, err = repo.searchPosts(ctx, tsq, opts)
	}
	require.NoError(t, err)
	sort.SliceStable(all, func(i, j int) bool { return all[i].Score > all[j].Score })
	limit, offset := searchPage(opts)
	if offset >= len(all) {
		return []models.SearchResult{}, len(all)
	}
	return all[offset:min(offset+limit, len(all))], len(all)
}

func TestSearch_KeywordPageEqualsTheFullSearch(t *testing.T) {
	scratch, _ := newMigratedScratchDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	seedKeywordPage(ctx, t, scratch)
	requireKeywordPageEqualsFullSearch(ctx, t, scratch)
}

// requireKeywordPageEqualsFullSearch compares Search's keyword page with fullKeywordSearch over the
// seed's queries, content types, sorts and pages.
func requireKeywordPageEqualsFullSearch(ctx context.Context, t *testing.T, pool *Pool) {
	t.Helper()
	repo := NewSearchRepository(pool)
	var viewer string
	require.NoError(t, pool.QueryRow(ctx, `SELECT id::text FROM users WHERE username = 'sd1'`).Scan(&viewer))

	cases := 0
	for _, q := range []string{"kubernetes", "postgres zeromq", "zeromq", "consensus lease", "nomatchanywhere"} {
		for _, ct := range [][]string{nil, {"posts"}} {
			for _, sortBy := range []string{"", "newest", "votes", "activity"} {
				for _, o := range []models.SearchOptions{
					{Page: 1, PerPage: 20},
					{Page: 3, PerPage: 7, ViewerHuman: viewer},
					{Page: 2, PerPage: 50, Tags: []string{"sdtag"}, FromDate: time.Now().Add(-20 * time.Hour)},
					{Page: 0, PerPage: 0},
					{Page: 400, PerPage: 20},
				} {
					o.ContentTypes, o.Sort = ct, sortBy
					want, wantTotal := fullKeywordSearch(ctx, t, repo, q, o)
					got, total, method, top, err := repo.Search(ctx, q, o)
					require.NoError(t, err)
					require.Equal(t, "fulltext_only", method)
					require.Nil(t, top)
					require.Equal(t, wantTotal, total, "q=%q ct=%v sort=%q opts=%+v", q, ct, sortBy, o)
					require.Equal(t, want, got, "q=%q ct=%v sort=%q opts=%+v", q, ct, sortBy, o)
					if len(want) > 0 {
						cases++
					}
				}
			}
		}
	}
	require.Greater(t, cases, 100, "most cases return a page to compare")
}

// Posts that tie on every key a sort names still page as the full search does. The seed's posts
// share their hour as created_at and updated_at (about sixty a group), so every sort meets ties;
// the seed above meets them only when two posts happen to share updated_at (sort=activity).
func TestSearch_KeywordPageEqualsTheFullSearchAmongTies(t *testing.T) {
	scratch, _ := newMigratedScratchDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	seedKeywordPage(ctx, t, scratch)
	_, err := scratch.Exec(ctx, `UPDATE posts SET created_at = date_trunc('hour', created_at), updated_at = date_trunc('hour', created_at)`)
	require.NoError(t, err)
	requireKeywordPageEqualsFullSearch(ctx, t, scratch)
}

// Posts that tie on every key of a sort page in id order, so consecutive pages neither repeat nor
// skip a post. Twelve posts share their text, created_at, updated_at and votes, and are written in
// descending id order: their physical order is the reverse of the order the search gives.
func TestSearch_TiedPostsPageInIDOrder(t *testing.T) {
	scratch, _ := newMigratedScratchDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	_, err := scratch.Exec(ctx, `
		WITH a AS (INSERT INTO agents (id, display_name, key_sha256) VALUES ('tied_agent', 'Tied', md5('t') || md5('d')) RETURNING id)
		INSERT INTO posts (id, type, title, description, tags, posted_by_type, posted_by_id, status,
		                   publication_state, moderation_state, visibility, created_at, updated_at)
		SELECT ('00000000-0000-4000-8000-' || lpad(g::text, 12, '0'))::uuid, 'post', 'Tied post', 'quokkapage lattice notes',
		       ARRAY['tiedtag'], 'agent', a.id, 'open', 'published', 'approved', 'public', NOW(), NOW()
		  FROM a, generate_series(12, 1, -1) g`)
	require.NoError(t, err)
	// Analyzed, the posts are read in their physical order (a sequential scan). Unanalyzed, the
	// planner reads them through idx_posts_not_deleted, in id order, and a sort without the id
	// tie-break would pass this test by accident.
	_, err = scratch.Exec(ctx, `ANALYZE posts`)
	require.NoError(t, err)
	ids := make([]string, 12)
	for i := range ids {
		ids[i] = fmt.Sprintf("00000000-0000-4000-8000-%012d", i+1)
	}
	var physical []string
	require.NoError(t, scratch.QueryRow(ctx, `SELECT array_agg(id::text ORDER BY ctid) FROM posts`).Scan(&physical))
	require.Equal(t, []string{ids[11], ids[10], ids[9]}, physical[:3], "the posts are stored in descending id order")

	repo := NewSearchRepository(scratch)
	for _, ct := range [][]string{nil, {"posts"}} {
		for _, sortBy := range []string{"", "newest", "votes", "activity"} {
			var paged []string
			for page := 1; page <= 3; page++ {
				o := models.SearchOptions{Page: page, PerPage: 5, ContentTypes: ct, Sort: sortBy}
				want, wantTotal := fullKeywordSearch(ctx, t, repo, "quokkapage", o)
				got, total, _, _, err := repo.Search(ctx, "quokkapage", o)
				require.NoError(t, err)
				require.Equal(t, 12, total, "ct=%v sort=%q page=%d", ct, sortBy, page)
				require.Equal(t, wantTotal, total, "ct=%v sort=%q page=%d", ct, sortBy, page)
				require.Equal(t, want, got, "ct=%v sort=%q page=%d", ct, sortBy, page)
				for _, r := range got {
					paged = append(paged, r.ID)
				}
			}
			require.Equal(t, ids, paged, "ct=%v sort=%q: tied posts page in id order, none twice and none missed", ct, sortBy)
		}
	}
}

func TestSearch_KeywordPageBuildsOnlyItsPage(t *testing.T) {
	scratch, _ := newMigratedScratchDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	seedKeywordPage(ctx, t, scratch)
	capture := &statementCapture{}
	traced, err := NewPool(ctx, scratch.Config().ConnString(), WithQueryTracer(capture))
	require.NoError(t, err)
	defer traced.Close()
	repo := NewSearchRepository(traced)

	var live, perPost int
	require.NoError(t, scratch.QueryRow(ctx, `
		SELECT COUNT(*), (SELECT MAX(c) FROM (SELECT COUNT(*) c FROM replies WHERE deleted_at IS NULL GROUP BY post_id) x)
		  FROM replies WHERE deleted_at IS NULL`).Scan(&live, &perPost))

	for _, c := range []struct {
		name string
		q    string
		opts models.SearchOptions
	}{
		{"knowledge", "kubernetes", models.SearchOptions{Page: 1, PerPage: 20}},
		{"knowledge rare", "zeromq", models.SearchOptions{Page: 2, PerPage: 5}},
		{"posts", "kubernetes", models.SearchOptions{Page: 2, PerPage: 20, ContentTypes: []string{"posts"}}},
	} {
		t.Run(c.name, func(t *testing.T) {
			var matches int
			require.NoError(t, scratch.QueryRow(ctx, `SELECT COUNT(*) FROM replies
				WHERE deleted_at IS NULL AND search_document @@ to_tsquery('english', $1)`, buildTsQuery(c.q)).Scan(&matches))
			capture.take()
			got, total, _, _, err := repo.Search(ctx, c.q, c.opts)
			require.NoError(t, err)
			require.NotEmpty(t, got)
			require.Greater(t, total, len(got), "more posts match than one page holds")

			// The search reads the matching replies to rank them (and at most again for the page's
			// anchors), and the replies of its page's posts for their counts: not every live reply.
			limit, _ := searchPage(c.opts)
			bound := 2*matches + 2*limit*perPost
			require.Less(t, bound, live/2, "the bound is far below reading every live reply")
			taken := 0
			for _, s := range capture.take() {
				taken += replyRowsTaken(ctx, t, scratch, s)
			}
			t.Logf("%s: took %d replies of %d live (bound %d, %d matching)", c.name, taken, live, bound, matches)
			require.LessOrEqual(t, taken, bound)
		})
	}
}

// Equal scores among posts found by their own text keep the full search's own order: for the
// default sort, the better own rank first, whatever the posts' ages. Two posts tie through
// identical replies that outrank both; the older post's own text holds the term twice, the
// newer's once (slice 20 mutation m2, ordering such ties by created_at, passed the seed above).
func TestSearch_KeywordPageTiesKeepTheOwnRankOrder(t *testing.T) {
	scratch, _ := newMigratedScratchDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	var older, newer string
	require.NoError(t, scratch.QueryRow(ctx, `
		WITH a AS (INSERT INTO agents (id, display_name, key_sha256) VALUES ('tie_agent', 'Tie', md5('t') || md5('u')) RETURNING id),
		     p AS (INSERT INTO posts (type, title, description, tags, posted_by_type, posted_by_id, status,
		                              publication_state, moderation_state, visibility, created_at)
		           SELECT 'post', 'Tie ' || g, d, ARRAY['tietag'], 'agent', a.id, 'open', 'published', 'approved', 'public',
		                  NOW() - (3 - g) * INTERVAL '1 hour'
		             FROM a, (VALUES (1, 'quokkatie quokkatie lattice notes'), (2, 'quokkatie lattice notes')) v(g, d)
		           RETURNING id, title),
		     r AS (INSERT INTO replies (post_id, author_type, author_id, body)
		           SELECT p.id, 'agent', 'tie_agent', 'quokkatie quokkatie quokkatie quokkatie quokkatie' FROM p)
		SELECT (SELECT id::text FROM p WHERE title = 'Tie 1'), (SELECT id::text FROM p WHERE title = 'Tie 2')`).
		Scan(&older, &newer))

	var ownOlder, ownNewer float64
	require.NoError(t, scratch.QueryRow(ctx, `
		SELECT MAX(ts_rank(search_document, to_tsquery('english', 'quokkatie'))) FILTER (WHERE id = $1),
		       MAX(ts_rank(search_document, to_tsquery('english', 'quokkatie'))) FILTER (WHERE id = $2)
		  FROM posts`, older, newer).Scan(&ownOlder, &ownNewer))
	require.Greater(t, ownOlder, ownNewer, "the older post ranks better by its own text")

	repo := NewSearchRepository(scratch)
	opts := models.SearchOptions{Page: 1, PerPage: 20}
	got, total, _, _, err := repo.Search(ctx, "quokkatie", opts)
	require.NoError(t, err)
	require.Equal(t, 2, total)
	require.Len(t, got, 2)
	require.Equal(t, got[0].Score, got[1].Score, "both posts score through their identical replies")
	require.Greater(t, got[0].Score, ownOlder, "the replies outrank both posts' own text")
	require.Equal(t, []string{older, newer}, []string{got[0].ID, got[1].ID})
	want, wantTotal := fullKeywordSearch(ctx, t, repo, "quokkatie", opts)
	require.Equal(t, wantTotal, total)
	require.Equal(t, want, got)
}
