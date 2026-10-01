package db

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/stretchr/testify/require"
)

// Hybrid search reads the replies its results need (spec.json idx 77 step 1: search indexes from
// measured query plans). Measured before this change (slice 19 spike: 200k posts, 1M replies, a
// tenth of each embedded), the default hybrid search for a term in every post took 7.0 s: every
// post statement aggregated the reply counts of all 975k live replies to show those of 60 posts
// (2.0 s and 0.5 s), and hybrid_search_replies materialized every eligible reply (935k rows) to
// read it three times, so neither its keyword index nor its vector index could be used (4.6 s).
// After: 1.2 s (91 ms for a term in 1 in 2,000 replies, 4.9 s before).

// seedHybridReplyReads adds to seedSearchDocuments' 2,000 posts and 6,000 replies 54,000 replies
// that no query of these tests matches and that have no embedding, 27 per post; "zeromq" in 1
// in 200 of the other replies and in no post; and embeddings on every tenth of them: half point
// near fixedQueryEmbedder's vector, half are centred and fail the semantic cutoff.
func seedHybridReplyReads(ctx context.Context, t *testing.T, pool *Pool) {
	t.Helper()
	seedSearchDocuments(ctx, t, pool)
	for _, step := range []struct{ name, sql string }{
		{"filler", `WITH listed AS (SELECT id, row_number() OVER (ORDER BY created_at DESC) rn FROM posts)
			INSERT INTO replies (post_id, author_type, author_id, body, created_at)
			SELECT p.id, 'agent', 'sd_agent_' || (1 + g % 10), 'filler note number ' || g,
			       NOW() - g * INTERVAL '1 second'
			  FROM generate_series(1, 54000) g JOIN listed p ON p.rn = 1 + g % 2000`},
		{"rare", `UPDATE replies SET body = body || ' zeromq'
			WHERE body NOT LIKE 'filler note%' AND hashtext(id::text) % 200 = 3`},
		{"embeddings", `UPDATE replies r SET embedding = ARRAY(
				SELECT CASE WHEN hashtext(r.id::text) % 2 = 0
				            THEN ((i % 7) + 1) / 10.0 + (hashtext(r.id::text || i) % 1000) / 4000.0
				            ELSE (hashtext(r.id::text || i) % 1000) / 1000.0 END
				  FROM generate_series(0, 1023) i)::vector
			WHERE r.body NOT LIKE 'filler note%' AND hashtext(r.id::text) % 10 = 0`},
		{"deleted", `UPDATE replies SET deleted_at = NOW() WHERE hashtext(id::text) % 40 = 1`},
		{"analyze", `ANALYZE posts, replies`},
	} {
		_, err := pool.Exec(ctx, step.sql)
		require.NoError(t, err, "seed %s", step.name)
	}
}

// replyRowsTaken executes the statement under EXPLAIN ANALYZE and returns how many rows its
// scans of replies emitted, over all their loops: what the statement took from the table.
func replyRowsTaken(ctx context.Context, t *testing.T, pool *Pool, s capturedStatement) int {
	t.Helper()
	type node struct {
		RelationName string  `json:"Relation Name"`
		ActualRows   float64 `json:"Actual Rows"`
		ActualLoops  float64 `json:"Actual Loops"`
		Plans        []node  `json:"Plans"`
	}
	var raw []byte
	require.NoError(t, pool.QueryRow(ctx, "EXPLAIN (ANALYZE, FORMAT JSON) "+s.sql, s.args...).Scan(&raw))
	var doc []struct {
		Plan node `json:"Plan"`
	}
	require.NoError(t, json.Unmarshal(raw, &doc))
	taken := 0.0
	var walk func(n node)
	walk = func(n node) {
		if n.RelationName == "replies" {
			taken += n.ActualRows * n.ActualLoops
		}
		for _, c := range n.Plans {
			walk(c)
		}
	}
	walk(doc[0].Plan)
	return int(taken + 0.5)
}

func TestSearch_HybridTakesOnlyTheRepliesItsMatchesAndResultsNeed(t *testing.T) {
	scratch, _ := newMigratedScratchDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	seedHybridReplyReads(ctx, t, scratch)

	capture := &statementCapture{}
	traced, err := NewPool(ctx, scratch.Config().ConnString(), WithQueryTracer(capture))
	require.NoError(t, err)
	defer traced.Close()
	repo := NewSearchRepository(traced)
	repo.SetEmbeddingService(fixedQueryEmbedder{})

	var viewer string
	require.NoError(t, scratch.QueryRow(ctx, `SELECT id::text FROM users WHERE username = 'sd1'`).Scan(&viewer))
	var live, embedded, perPost int
	require.NoError(t, scratch.QueryRow(ctx, `
		SELECT COUNT(*), COUNT(*) FILTER (WHERE embedding IS NOT NULL),
		       (SELECT MAX(c) FROM (SELECT COUNT(*) c FROM replies WHERE deleted_at IS NULL GROUP BY post_id) x)
		  FROM replies WHERE deleted_at IS NULL`).Scan(&live, &embedded, &perPost))

	for _, c := range []struct {
		name string
		q    string
		opts models.SearchOptions
	}{
		{"knowledge", "kubernetes", models.SearchOptions{Page: 1, PerPage: 20}},
		{"knowledge viewer", "postgres zeromq", models.SearchOptions{Page: 2, PerPage: 20, ViewerHuman: viewer}},
		{"knowledge rare", "zeromq", models.SearchOptions{Page: 1, PerPage: 50}},
		{"posts", "kubernetes", models.SearchOptions{Page: 1, PerPage: 20, ContentTypes: []string{"posts"}}},
	} {
		t.Run(c.name, func(t *testing.T) {
			capture.take()
			got, _, method, _, err := repo.Search(ctx, c.q, c.opts)
			require.NoError(t, err)
			require.Equal(t, "hybrid_rrf", method)
			require.NotEmpty(t, got, "the seed fills this page")

			// Every result's counts are its post's live replies, bucketed as the listing does.
			for _, r := range got {
				var ans, app, cmt int
				require.NoError(t, scratch.QueryRow(ctx, `
					SELECT COUNT(*) FILTER (WHERE `+replyAnswerBucket+`),
					       COUNT(*) FILTER (WHERE `+replyApproachBucket+`),
					       COUNT(*) FILTER (WHERE NOT `+replyAnswerBucket+` AND NOT (`+replyApproachBucket+`))
					  FROM replies r WHERE r.post_id = $1 AND r.deleted_at IS NULL`, r.ID).Scan(&ans, &app, &cmt))
				require.Equal(t, []int{ans, app, cmt}, []int{r.AnswersCount, r.ApproachesCount, r.CommentsCount},
					"reply counts of post %s", r.ID)
			}

			// What one search may take from replies: the keyword matches and the embedded replies
			// (the candidates), each candidate pair's reply once more for its post (2 x 2 x
			// matchCount) and the matches' rows (matchCount), and the replies of the at most
			// matchCount posts each post statement returns (2 x matchCount x perPost). The
			// unmatched filler is not among them.
			var matches int
			require.NoError(t, scratch.QueryRow(ctx, `SELECT COUNT(*) FROM replies
				WHERE deleted_at IS NULL AND search_document @@ to_tsquery('english', $1)`, buildTsQuery(c.q)).Scan(&matches))
			n := hybridMatchCount(c.opts)
			bound := matches + embedded + 5*n + 2*n*perPost
			require.Less(t, bound, live/2, "the bound tells a read of the matches from a read of the table")
			taken := 0
			for _, s := range capture.take() {
				taken += replyRowsTaken(ctx, t, scratch, s)
			}
			t.Logf("took %d replies of %d live (%d keyword matches, %d embedded), bound %d", taken, live, matches, embedded, bound)
			require.LessOrEqual(t, taken, bound,
				"the search took %d replies of %d live; its matches and results need at most %d", taken, live, bound)
		})
	}
}

// referenceHybridReplies is hybrid_search_replies' definition: the live replies of published,
// approved, visible posts; the 2 x match_count best by keyword rank and the 2 x match_count
// nearest under the 0.85 cutoff, ties broken by reply id; fused by reciprocal rank.
const referenceHybridReplies = `
	WITH eligible AS MATERIALIZED (
		SELECT r.id, r.post_id, r.search_document, r.embedding
		  FROM replies r JOIN posts p ON p.id = r.post_id
		 WHERE r.deleted_at IS NULL AND p.deleted_at IS NULL
		   AND p.publication_state = 'published' AND p.moderation_state = 'approved'
		   AND (p.visibility = 'public' OR ($4::uuid IS NOT NULL AND p.owner_human_id = $4::uuid))),
	full_text AS (
		SELECT id, ROW_NUMBER() OVER (ORDER BY ts_rank_cd(search_document, to_tsquery('english', $1)) DESC, id) rank_ix
		  FROM eligible WHERE search_document @@ to_tsquery('english', $1)
		 ORDER BY rank_ix LIMIT $3 * 2),
	semantic AS (
		SELECT id, ROW_NUMBER() OVER (ORDER BY embedding <=> $2::vector, id) rank_ix
		  FROM eligible WHERE embedding IS NOT NULL AND embedding <=> $2::vector < 0.85
		 ORDER BY rank_ix LIMIT $3 * 2)
	SELECT e.id::text, e.post_id::text,
	       COALESCE(1.0 / (60 + ft.rank_ix), 0.0) * 2.0::float8
	       + COALESCE(1.0 / (60 + s.rank_ix), 0.0) * 1.0::float8 AS rrf_score
	  FROM full_text ft FULL OUTER JOIN semantic s ON ft.id = s.id
	  JOIN eligible e ON e.id = COALESCE(ft.id, s.id)
	 ORDER BY rrf_score DESC, e.id LIMIT $3`

func TestHybridSearchReplies_EqualsItsDefinition(t *testing.T) {
	scratch, _ := newMigratedScratchDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	seedHybridReplyReads(ctx, t, scratch)
	// A family post with replies, visible to its owner only.
	var viewer string
	require.NoError(t, scratch.QueryRow(ctx, `SELECT id::text FROM users WHERE username = 'sd1'`).Scan(&viewer))
	_, err := scratch.Exec(ctx, `UPDATE posts SET visibility = 'family', owner_human_id = $1
		WHERE hashtext(id::text) % 50 = 0`, viewer)
	require.NoError(t, err)

	type row struct {
		ID, PostID string
		Score      float64
	}
	read := func(tx Tx, sql string, args ...any) []row {
		rows, err := tx.Query(ctx, sql, args...)
		require.NoError(t, err)
		var out []row
		for rows.Next() {
			var r row
			require.NoError(t, rows.Scan(&r.ID, &r.PostID, &r.Score))
			out = append(out, r)
		}
		require.NoError(t, rows.Err())
		return out
	}
	vec := `(SELECT array_agg(((i % 7) + 1) / 10.0)::vector FROM generate_series(0, 1023) i)`
	for _, q := range []string{"kubernetes", "postgres zeromq", "zeromq", "filler"} {
		for _, n := range []int{20, 60, 150} {
			for _, v := range []any{nil, viewer} {
				// The definition is exact; so is the function once the planner may not answer
				// the nearest-neighbour order from the approximate vector index.
				tx, err := scratch.BeginTx(ctx)
				require.NoError(t, err)
				_, err = tx.Exec(ctx, `SET LOCAL enable_indexscan = off`)
				require.NoError(t, err)
				var qv string
				require.NoError(t, tx.QueryRow(ctx, `SELECT `+vec+`::text`).Scan(&qv))
				want := read(tx, referenceHybridReplies, buildTsQuery(q), qv, n, v)
				got := read(tx, `SELECT reply_id::text, post_id::text, rrf_score
					FROM hybrid_search_replies($1, $2::vector, $3, 2.0, 1.0, 60, $4::uuid)`, buildTsQuery(q), qv, n, v)
				require.NoError(t, tx.Rollback(ctx))
				require.NotEmpty(t, want, "q=%s n=%d viewer=%v finds replies", q, n, v)
				require.Equal(t, want, got, "q=%s n=%d viewer=%v", q, n, v)
			}
		}
	}
}
