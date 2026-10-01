package db

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
)

// The public post listing (spec.json idx 77 step 1: public post listing, measured on
// representative data). Measured on HEAD before this change (slice 13 spike: 200k posts, 60%
// by humans as in production, 1M replies, 20k users): every page cost a full aggregate of the
// live replies (~300 ms, 101k groups) whatever the page held, a newest page took 0.44 s and
// page 200 0.91 s; each human author was found by scanning users (u.id::text has no index),
// once per listed post; sort=hot computed its score in numeric arithmetic over every candidate
// (2.9 s). A page must read the rows it shows, and every sort must keep its definition.

// seedPostListing writes 20k users, 2k agents (a third owned by a human), 60k posts one minute
// apart (60% by humans; 5% family-scoped to the viewer, 3% deleted, 1% pending review) and 300k
// live replies (half on every 100th post, the rest spread; humans, agents and system; native
// replies, migrated approaches and comments, child replies). It returns the viewer's id.
func seedPostListing(ctx context.Context, t *testing.T, pool *Pool) string {
	t.Helper()
	for _, step := range []struct{ name, sql string }{
		{"users", `
			INSERT INTO users (id, username, display_name, email, referral_code, avatar_url)
			SELECT gen_random_uuid(), 'pl' || g, 'User ' || g, 'pl' || g || '@example.test',
			       'P' || lpad(g::text, 7, '0'), CASE WHEN g % 2 = 0 THEN 'https://img.test/u' || g END
			  FROM generate_series(1, 20000) g`},
		{"agents", `
			INSERT INTO agents (id, display_name, key_sha256, human_id)
			SELECT 'pl_agent_' || g, 'Agent ' || g, md5('k' || g) || md5('x' || g),
			       CASE WHEN g % 3 = 0 THEN (SELECT id FROM users WHERE username = 'pl' || g) END
			  FROM generate_series(1, 2000) g`},
		{"posts", `
			INSERT INTO posts (type, title, description, tags, posted_by_type, posted_by_id, status,
			                   upvotes, downvotes, view_count, created_at, deleted_at, visibility, owner_human_id)
			SELECT (ARRAY['problem','question','idea','post'])[1 + g % 4], 'Listed post ' || g,
			       'Description of listed post ' || g, ARRAY['pltag' || (g % 13), 'pltag' || (g % 7)],
			       CASE WHEN g % 5 < 3 THEN 'human' ELSE 'agent' END,
			       CASE WHEN g % 5 < 3 THEN u.id::text ELSE 'pl_agent_' || (1 + g % 2000) END,
			       CASE WHEN g % 97 = 0 THEN 'pending_review'
			            ELSE (ARRAY['open','solved','answered','active','in_progress'])[1 + g % 5] END,
			       (g * 7) % 23, (g * 3) % 5, (g * 11) % 1000, NOW() - g * INTERVAL '1 minute',
			       CASE WHEN g % 33 = 0 THEN NOW() END,
			       CASE WHEN g % 20 = 0 THEN 'family' ELSE 'public' END,
			       CASE WHEN g % 20 = 0 THEN (SELECT id FROM users WHERE username = 'pl1') END
			  FROM generate_series(1, 60000) g
			  JOIN users u ON u.username = 'pl' || (1 + (g * 7) % 20000)`},
		{"replies", `
			WITH listed AS (SELECT id, row_number() OVER (ORDER BY created_at DESC) rn FROM posts)
			INSERT INTO replies (post_id, author_type, author_id, body, legacy_type, legacy_id, created_at)
			SELECT p.id,
			       CASE WHEN g % 10 < 4 THEN 'human' WHEN g % 10 < 9 THEN 'agent' ELSE 'system' END,
			       CASE WHEN g % 10 < 4 THEN u.id::text WHEN g % 10 < 9 THEN 'pl_agent_' || (1 + g % 2000)
			            ELSE 'system' END,
			       'Reply body ' || g,
			       CASE WHEN g % 10 < 9 AND g % 20 IN (1, 2) THEN 'approach'
			            WHEN g % 10 < 9 AND g % 20 = 3 THEN 'comment' END,
			       CASE WHEN g % 10 < 9 AND g % 20 IN (1, 2, 3) THEN gen_random_uuid() END,
			       NOW() - g * INTERVAL '1 second'
			  FROM generate_series(1, 300000) g
			  JOIN listed p ON p.rn = CASE WHEN g % 2 = 0 THEN 100 * (1 + g % 600) ELSE 1 + g % 60000 END
			  JOIN users u ON u.username = 'pl' || (1 + (g * 13) % 20000)`},
		{"child replies", `
			INSERT INTO replies (post_id, parent_reply_id, author_type, author_id, body)
			SELECT r.post_id, r.id, 'agent', 'pl_agent_1', 'Child of ' || r.id
			  FROM replies r WHERE r.author_type = 'agent' AND r.legacy_type IS NULL
			   AND hashtext(r.id::text) % 25 = 0`},
		{"deleted replies", `UPDATE replies SET deleted_at = NOW() WHERE hashtext(id::text) % 40 = 0`},
		{"viewer votes", `
			INSERT INTO votes (target_type, target_id, voter_type, voter_id, direction, confirmed)
			SELECT 'post', p.id, 'human', u.id::text, CASE WHEN hashtext(p.id::text) % 2 = 0 THEN 'up' ELSE 'down' END, true
			  FROM posts p, users u WHERE u.username = 'pl1' AND p.title LIKE 'Listed post %3'`},
		{"analyze", `VACUUM ANALYZE users, agents, posts, replies, votes`},
	} {
		start := time.Now()
		_, err := pool.Exec(ctx, step.sql)
		require.NoError(t, err, "seed %s", step.name)
		t.Logf("seed %s: %s", step.name, time.Since(start).Round(time.Millisecond))
	}
	var viewer string
	require.NoError(t, pool.QueryRow(ctx, `SELECT id::text FROM users WHERE username = 'pl1'`).Scan(&viewer))
	return viewer
}

// referencePostList is the listing's definition, written plainly: every visible post with its
// author and its reply counts (each live reply in exactly one bucket, posts_reply_counts.go),
// filtered and ordered as the case says, one page of it. sort=hot is the trending score in its
// original numeric form.
const referencePostList = `
	SELECT p.id, p.type, p.title, p.description, p.tags, p.posted_by_type, p.posted_by_id, p.status,
	       p.upvotes, p.downvotes, p.view_count, p.success_criteria, p.weight, p.accepted_answer_id,
	       p.evolved_into, p.created_at, p.updated_at, p.deleted_at, p.crystallization_cid, p.crystallized_at,
	       COALESCE(p.original_language, ''), COALESCE(p.original_title, ''), COALESCE(p.original_description, ''),
	       COALESCE(u.display_name, ag.display_name, ''), COALESCE(u.avatar_url, ag.avatar_url, ''),
	       COALESCE(rc.ans, 0), COALESCE(rc.app, 0), COALESCE(rc.cmt, 0), COALESCE(ag.human_id::text, ''),
	       v.direction, p.visibility, p.publication_state, p.moderation_state, p.source_room_id::text
	  FROM posts p
	  LEFT JOIN users u ON p.posted_by_type = 'human' AND p.posted_by_id = u.id::text
	  LEFT JOIN agents ag ON p.posted_by_type = 'agent' AND p.posted_by_id = ag.id
	  LEFT JOIN (SELECT r.post_id,
	                    COUNT(*) FILTER (WHERE ` + replyAnswerBucket + `) AS ans,
	                    COUNT(*) FILTER (WHERE ` + replyApproachBucket + `) AS app,
	                    COUNT(*) FILTER (WHERE NOT ` + replyAnswerBucket + ` AND NOT (` + replyApproachBucket + `)) AS cmt
	               FROM replies r WHERE r.deleted_at IS NULL GROUP BY r.post_id) rc ON rc.post_id = p.id
	  LEFT JOIN votes v ON v.target_type = 'post' AND v.target_id = p.id AND v.voter_type = 'human' AND v.voter_id = $1
	 WHERE p.deleted_at IS NULL AND p.status NOT IN ('pending_review', 'rejected', 'draft')
	   AND (p.visibility = 'public' OR p.owner_human_id::text = $2)
	   AND ($3 = '' OR p.type = $3) AND (cardinality($4::text[]) = 0 OR p.tags && $4::text[])
	   AND ($5::boolean IS NULL OR (COALESCE(rc.ans, 0) > 0) = $5::boolean)
	 ORDER BY %s
	 LIMIT $6 OFFSET $7`

// postListCase is one listing request and how its page is defined.
type postListCase struct {
	name  string
	opts  models.PostListOptions
	order string // the reference ORDER BY
	// sortsByCounts: the order reads every post's reply counts, so the page statement may read
	// the replies once. filtersByAnswers: the total counts the posts with (or without) an
	// answer, so the count statement may read them once. Otherwise a statement reads only the
	// replies of the posts it shows.
	sortsByCounts, filtersByAnswers bool
}

func postListCases(viewer string) []postListCase {
	yes, no := true, false
	const newest = "p.created_at DESC"
	const hot = `(LOG(GREATEST(ABS(COALESCE(p.upvotes,0) - COALESCE(p.downvotes,0)) + COALESCE(rc.cmt,0) * 2
		+ COALESCE(rc.ans,0) * 3 + COALESCE(rc.app,0) * 3 + COALESCE(p.view_count,0) * 0.01, 1) + 1)
		+ EXTRACT(EPOCH FROM (p.created_at - (NOW() - INTERVAL '7 days'))) / 45000.0) DESC`
	return []postListCase{
		{name: "newest", opts: models.PostListOptions{Page: 1, PerPage: 20}, order: newest},
		{name: "newest page 40", opts: models.PostListOptions{Page: 40, PerPage: 20, Sort: "new"}, order: newest},
		{name: "votes", opts: models.PostListOptions{Page: 2, PerPage: 20, Sort: "votes"},
			order: "(p.upvotes - p.downvotes) DESC, p.created_at DESC"},
		{name: "type and tag", opts: models.PostListOptions{Page: 1, PerPage: 50, Type: "problem", Tags: []string{"pltag3"}},
			order: newest},
		{name: "viewer with family and votes", order: newest, opts: models.PostListOptions{Page: 1, PerPage: 100,
			ViewerType: models.AuthorTypeHuman, ViewerID: viewer, ViewerHuman: viewer}},
		{name: "hot", opts: models.PostListOptions{Page: 1, PerPage: 20, Sort: "hot"}, order: hot, sortsByCounts: true},
		{name: "answers", opts: models.PostListOptions{Page: 1, PerPage: 20, Sort: "answers"},
			order: "COALESCE(rc.ans, 0) DESC, p.created_at DESC", sortsByCounts: true},
		{name: "approaches", opts: models.PostListOptions{Page: 3, PerPage: 20, Sort: "approaches"},
			order: "COALESCE(rc.app, 0) DESC, p.created_at DESC", sortsByCounts: true},
		{name: "unanswered", opts: models.PostListOptions{Page: 1, PerPage: 20, HasAnswer: &no}, order: newest, filtersByAnswers: true},
		{name: "answered", opts: models.PostListOptions{Page: 5, PerPage: 20, HasAnswer: &yes}, order: newest, filtersByAnswers: true},
	}
}

// expectedPostList runs the reference definition for a case and returns its page.
func expectedPostList(ctx context.Context, t *testing.T, pool *Pool, c postListCase) []models.PostWithAuthor {
	t.Helper()
	viewerID, viewerHuman := "", ""
	if c.opts.ViewerType == models.AuthorTypeHuman {
		viewerID, viewerHuman = c.opts.ViewerID, c.opts.ViewerHuman
	}
	tags := c.opts.Tags
	if tags == nil {
		tags = []string{}
	}
	rows, err := pool.Query(ctx, fmt.Sprintf(referencePostList, c.order), viewerID, viewerHuman, string(c.opts.Type),
		tags, c.opts.HasAnswer, c.opts.PerPage, (c.opts.Page-1)*c.opts.PerPage)
	require.NoError(t, err, "reference %s", c.name)
	defer rows.Close()
	repo := NewPostRepository(pool)
	want := []models.PostWithAuthor{}
	for rows.Next() {
		p, err := repo.scanPostWithAuthorRows(rows)
		require.NoError(t, err)
		want = append(want, *p)
	}
	require.NoError(t, rows.Err())
	return want
}

// statementCapture records the statements a call sends, with their arguments.
type statementCapture struct {
	mu    sync.Mutex
	stmts []capturedStatement
}

type capturedStatement struct {
	sql  string
	args []any
}

func (c *statementCapture) TraceQueryStart(ctx context.Context, _ *pgx.Conn, d pgx.TraceQueryStartData) context.Context {
	c.mu.Lock()
	c.stmts = append(c.stmts, capturedStatement{d.SQL, d.Args})
	c.mu.Unlock()
	return ctx
}

func (c *statementCapture) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}

func (c *statementCapture) take() []capturedStatement {
	c.mu.Lock()
	defer c.mu.Unlock()
	s := c.stmts
	c.stmts = nil
	return s
}

// planNode is the part of an EXPLAIN (ANALYZE, BUFFERS, FORMAT JSON) node this test reads.
type planNode struct {
	NodeType     string     `json:"Node Type"`
	RelationName string     `json:"Relation Name"`
	SharedHit    int        `json:"Shared Hit Blocks"`
	SharedRead   int        `json:"Shared Read Blocks"`
	Plans        []planNode `json:"Plans"`
}

// tableReads executes the statement under EXPLAIN ANALYZE and returns, per table, the buffers
// its scan nodes touched (index pages included) and whether any node scanned it sequentially.
func tableReads(ctx context.Context, t *testing.T, pool *Pool, s capturedStatement) (map[string]int, map[string]bool) {
	t.Helper()
	var raw []byte
	require.NoError(t, pool.QueryRow(ctx, "EXPLAIN (ANALYZE, BUFFERS, FORMAT JSON) "+s.sql, s.args...).Scan(&raw))
	var doc []struct {
		Plan planNode `json:"Plan"`
	}
	require.NoError(t, json.Unmarshal(raw, &doc))
	buffers, seq := map[string]int{}, map[string]bool{}
	var walk func(n planNode)
	walk = func(n planNode) {
		if n.RelationName != "" {
			buffers[n.RelationName] += n.SharedHit + n.SharedRead
			if strings.Contains(n.NodeType, "Seq Scan") {
				seq[n.RelationName] = true
			}
		}
		for _, c := range n.Plans {
			walk(c)
		}
	}
	walk(doc[0].Plan)
	return buffers, seq
}

// replyCountsFloor is the cheapest read of every post's reply counts: one aggregate of the live
// replies into the three buckets, no posts, no authors, no sort. It is the yardstick for the
// requests that need every post's counts.
func replyCountsFloor(ctx context.Context, t *testing.T, pool *Pool) time.Duration {
	t.Helper()
	best := time.Duration(1<<63 - 1)
	for i := 0; i < 3; i++ {
		start := time.Now()
		var n int
		require.NoError(t, pool.QueryRow(ctx, `
			SELECT COUNT(*) FROM (SELECT r.post_id,
			       COUNT(*) FILTER (WHERE `+replyAnswerBucket+`) AS ans,
			       COUNT(*) FILTER (WHERE `+replyApproachBucket+`) AS app,
			       COUNT(*) FILTER (WHERE NOT `+replyAnswerBucket+` AND NOT (`+replyApproachBucket+`)) AS cmt
			  FROM replies r WHERE r.deleted_at IS NULL GROUP BY r.post_id) c`).Scan(&n))
		best = min(best, time.Since(start))
	}
	return best
}

// postListCountsBound is how many reply-count aggregates one page that sorts or filters by
// reply counts may cost. Measured here before this change (two-bucket yardstick, 48 ms):
// sort=hot 906 ms (numeric arithmetic, 19x), has_answer 184-271 ms (two full aggregates);
// after it (this yardstick, 49 ms): hot 134, answers 132, approaches 102, has_answer 38-53 ms.
// What the plans must not read is asserted from the plans; this bound catches CPU, as numeric.
const postListCountsBound = 4

func TestPostList_AtGrowthVolumeEveryPageMatchesItsDefinitionAndReadsOnlyWhatItNeeds(t *testing.T) {
	scratch, _ := newMigratedScratchDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	viewer := seedPostListing(ctx, t, scratch)

	capture := &statementCapture{}
	traced, err := NewPool(ctx, scratch.Config().ConnString(), WithQueryTracer(capture))
	require.NoError(t, err)
	defer traced.Close()
	repo := NewPostRepository(traced)

	pages := map[string]int{}
	for _, table := range []string{"replies", "users"} {
		var n int
		require.NoError(t, scratch.QueryRow(ctx, `SELECT relpages FROM pg_class WHERE oid = $1::regclass`, table).Scan(&n))
		require.Greater(t, n, 200, "the %s table spans enough pages to tell a page read from a scan", table)
		pages[table] = n
	}
	floor := replyCountsFloor(ctx, t, scratch)

	for _, c := range postListCases(viewer) {
		t.Run(c.name, func(t *testing.T) {
			want := expectedPostList(ctx, t, scratch, c)
			require.NotEmpty(t, want, "the seed fills this page")
			capture.take()
			start := time.Now()
			got, total, err := repo.List(ctx, c.opts)
			took := time.Since(start)
			require.NoError(t, err)
			require.Equal(t, want, got, "the page must equal the definition")
			require.Greater(t, total, len(got), "the total counts every matching post")
			t.Logf("%s: %s (reply counts floor %s)", c.name, took.Round(time.Millisecond), floor.Round(time.Millisecond))

			for i, s := range capture.take() {
				buffers, seq := tableReads(ctx, t, scratch, s)
				t.Logf("  statement %d buffers %v seq %v", i, buffers, seq)
				require.False(t, seq["users"], "statement %d scans users to find authors", i)
				total := strings.HasPrefix(strings.TrimSpace(s.sql), "SELECT COUNT(*)")
				if (total && !c.filtersByAnswers) || (!total && !c.sortsByCounts) {
					require.False(t, seq["replies"], "statement %d scans replies it does not need", i)
					require.Less(t, buffers["replies"], pages["replies"]/4,
						"statement %d reads %d reply pages; the table has %d", i, buffers["replies"], pages["replies"])
				}
			}
			if c.sortsByCounts || c.filtersByAnswers {
				require.LessOrEqual(t, took, postListCountsBound*floor, fmt.Sprintf(
					"a %s page must cost at most %d reply-count aggregates: took %s, one aggregate %s",
					c.name, postListCountsBound, took.Round(time.Millisecond), floor.Round(time.Millisecond)))
			}
		})
	}
}
