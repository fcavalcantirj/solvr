package db

import (
	"context"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/stretchr/testify/require"
)

// The keyword documents search reads are stored (spec.json idx 77 step 1: search indexes from
// measured query plans). Measured before this change (slice 18 spike: 200k posts of ~580
// characters, 1M replies): a term in every post cost 24 s on the default hybrid path and 26 s
// on the keyword fallback, 23.4 s of it a scan recomputing to_tsvector of every matching post
// twice (filter and rank); ranking the same matches from a stored tsvector took 141 ms.

// fixedQueryEmbedder answers every query with fixedSearchVector, which the seed stores on a
// tenth of the posts, so the hybrid functions have semantic matches too.
type fixedQueryEmbedder struct{}

const fixedSearchVector = `(SELECT array_agg(((i % 7) + 1) / 10.0)::vector FROM generate_series(0, 1023) i)`

func (fixedQueryEmbedder) GenerateQueryEmbedding(context.Context, string) ([]float32, error) {
	v := make([]float32, 1024)
	for i := range v {
		v[i] = float32((i%7)+1) / 10
	}
	return v, nil
}

// seedSearchDocuments writes 2,000 published posts and 6,000 replies of varied words: every
// post says "kubernetes", every tenth "postgres"; two replies in five say "kubernetes".
func seedSearchDocuments(ctx context.Context, t *testing.T, pool *Pool) {
	t.Helper()
	words := `(string_to_array('error build deploy cache memory thread lock query index table schema
		migration request token session socket timeout retry queue worker container network route
		handler stream buffer module package runtime heap stack trace metric alert replica shard
		leader consensus lease secret policy audit signature release rollback canary flag', ' '))`
	text := func(seed string, n int) string {
		return `(SELECT string_agg(` + words + `[1 + ((` + seed + ` * 7919 + i * 104729 + i * i * 31) % 50)::int], ' ')
			FROM generate_series(1, ` + strconv.Itoa(n) + `) i)`
	}
	for _, step := range []struct{ name, sql string }{
		{"users", `INSERT INTO users (id, username, display_name, email, referral_code)
			SELECT gen_random_uuid(), 'sd' || g, 'User ' || g, 'sd' || g || '@example.test', 'D' || lpad(g::text, 7, '0')
			  FROM generate_series(1, 50) g`},
		{"agents", `INSERT INTO agents (id, display_name, key_sha256)
			SELECT 'sd_agent_' || g, 'Agent ' || g, md5('k' || g) || md5('x' || g) FROM generate_series(1, 10) g`},
		{"posts", `INSERT INTO posts (type, title, description, tags, posted_by_type, posted_by_id, status,
				publication_state, moderation_state, visibility, created_at)
			SELECT 'post', 'Post ' || g || ' ' || ` + text("g", 6) + `,
			       ` + text("g * 3", 30) + ` || ' kubernetes ' || CASE WHEN g % 10 = 0 THEN 'postgres ' ELSE '' END
			         || ` + text("g * 5", 30) + `,
			       ARRAY['sdtag'], CASE WHEN g % 2 = 0 THEN 'human' ELSE 'agent' END,
			       CASE WHEN g % 2 = 0 THEN u.id::text ELSE 'sd_agent_' || (1 + g % 10) END,
			       'open', 'published', 'approved', 'public', NOW() - g * INTERVAL '1 minute'
			  FROM generate_series(1, 2000) g JOIN users u ON u.username = 'sd' || (1 + g % 50)`},
		{"replies", `WITH listed AS (SELECT id, row_number() OVER (ORDER BY created_at DESC) rn FROM posts)
			INSERT INTO replies (post_id, author_type, author_id, body, legacy_type, legacy_id, created_at)
			SELECT p.id, CASE WHEN g % 10 < 9 THEN 'agent' ELSE 'system' END,
			       CASE WHEN g % 10 < 9 THEN 'sd_agent_' || (1 + g % 10) ELSE 'system' END,
			       ` + text("g * 11", 20) + ` || CASE WHEN g % 5 IN (0, 1) THEN ' kubernetes' ELSE '' END,
			       CASE WHEN g % 20 IN (1, 2) THEN 'approach' END,
			       CASE WHEN g % 20 IN (1, 2) THEN gen_random_uuid() END, NOW() - g * INTERVAL '1 second'
			  FROM generate_series(1, 6000) g JOIN listed p ON p.rn = 1 + g % 2000`},
		{"embeddings", `UPDATE posts SET embedding = ` + fixedSearchVector + ` WHERE hashtext(id::text) % 10 = 0`},
		{"analyze", `ANALYZE users, agents, posts, replies`},
	} {
		_, err := pool.Exec(ctx, step.sql)
		require.NoError(t, err, "seed %s", step.name)
	}
}

// referenceKnowledgeScores is the keyword search's definition, from the text itself: every
// searchable public post found by its own text or a live non-system reply's (on a published,
// approved post), scored by the better of the two ranks.
const referenceKnowledgeScores = `
	WITH own AS (
		SELECT p.id, ts_rank(to_tsvector('english', p.title || ' ' || p.description), to_tsquery('english', $1)) s
		  FROM posts p WHERE ` + searchablePostRule + ` AND p.visibility = 'public'
		   AND to_tsvector('english', p.title || ' ' || p.description) @@ to_tsquery('english', $1)),
	rm AS (
		SELECT r.post_id id, MAX(ts_rank(to_tsvector('english', r.body), to_tsquery('english', $1))) s
		  FROM replies r JOIN posts p ON p.id = r.post_id
		 WHERE r.deleted_at IS NULL AND r.author_type <> 'system' AND ` + replyMatchPostRule + `
		   AND p.visibility = 'public' AND to_tsvector('english', r.body) @@ to_tsquery('english', $1)
		 GROUP BY r.post_id)
	SELECT GREATEST(own.s, rm.s)::float8 FROM own FULL JOIN rm ON rm.id = own.id ORDER BY 1 DESC`

func TestSearch_KeywordMatchesReadStoredDocumentsAndKeepTheirRanks(t *testing.T) {
	scratch, _ := newMigratedScratchDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	seedSearchDocuments(ctx, t, scratch)

	// The stored document is the text's: on insert, and on every edit of the text.
	_, err := scratch.Exec(ctx, `UPDATE posts SET title = title || ' zeromq' WHERE hashtext(id::text) % 7 = 0`)
	require.NoError(t, err)
	_, err = scratch.Exec(ctx, `UPDATE replies SET body = body || ' zeromq' WHERE hashtext(id::text) % 7 = 0`)
	require.NoError(t, err)
	var drift int
	require.NoError(t, scratch.QueryRow(ctx, `
		SELECT (SELECT COUNT(*) FROM posts WHERE search_document IS DISTINCT FROM to_tsvector('english', title || ' ' || description))
		     + (SELECT COUNT(*) FROM replies WHERE search_document IS DISTINCT FROM to_tsvector('english', body))`).Scan(&drift))
	require.Zero(t, drift, "every stored search document equals the document of its row's text")

	capture := &statementCapture{}
	traced, err := NewPool(ctx, scratch.Config().ConnString(), WithQueryTracer(capture))
	require.NoError(t, err)
	defer traced.Close()

	var viewer string
	require.NoError(t, scratch.QueryRow(ctx, `SELECT id::text FROM users WHERE username = 'sd1'`).Scan(&viewer))
	for _, hybrid := range []bool{false, true} {
		repo := NewSearchRepository(traced)
		if hybrid {
			repo.SetEmbeddingService(fixedQueryEmbedder{})
		}
		for _, c := range []struct {
			name string
			q    string
			opts models.SearchOptions
		}{
			{"knowledge", "kubernetes", models.SearchOptions{Page: 1, PerPage: 20}},
			{"knowledge viewer", "postgres zeromq", models.SearchOptions{Page: 2, PerPage: 20, ViewerHuman: viewer}},
			{"posts", "kubernetes", models.SearchOptions{Page: 1, PerPage: 20, ContentTypes: []string{"posts"}}},
			{"answers", "kubernetes", models.SearchOptions{Page: 1, PerPage: 20, ContentTypes: []string{"answers"}}},
			{"approaches", "kubernetes", models.SearchOptions{Page: 1, PerPage: 20, ContentTypes: []string{"approaches"}}},
		} {
			name := c.name
			if hybrid {
				name = "hybrid " + name
			}
			t.Run(name, func(t *testing.T) {
				capture.take()
				got, total, method, _, err := repo.Search(ctx, c.q, c.opts)
				require.NoError(t, err)
				require.NotEmpty(t, got, "the seed fills this page")
				if hybrid {
					require.Equal(t, "hybrid_rrf", method)
				}

				if !hybrid && c.opts.ContentTypes == nil && c.opts.ViewerHuman == "" {
					var want []float64
					rows, err := scratch.Query(ctx, referenceKnowledgeScores, buildTsQuery(c.q))
					require.NoError(t, err)
					for rows.Next() {
						var s float64
						require.NoError(t, rows.Scan(&s))
						want = append(want, s)
					}
					require.NoError(t, rows.Err())
					require.Equal(t, len(want), total, "the total counts every post the text matches")
					scores := make([]float64, len(got))
					for i, r := range got {
						scores[i] = r.Score
					}
					require.True(t, sort.IsSorted(sort.Reverse(sort.Float64Slice(scores))), "best first")
					require.Equal(t, want[:len(got)], scores, "the page holds the best ranks of the text")
				}

				for i, s := range capture.take() {
					rows, err := scratch.Query(ctx, "EXPLAIN (VERBOSE) "+s.sql, s.args...)
					require.NoError(t, err)
					parses := 0
					for rows.Next() {
						var line string
						require.NoError(t, rows.Scan(&line))
						if strings.Contains(line, "to_tsvector(") {
							parses++
						}
					}
					require.NoError(t, rows.Err())
					require.Zero(t, parses, "statement %d parses documents instead of reading the stored ones (%d plan lines)", i, parses)
				}
			})
		}
	}
}
