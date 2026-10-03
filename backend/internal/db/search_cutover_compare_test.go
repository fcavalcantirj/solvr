package db

import (
	"context"
	"sort"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// cmpProbe is one representative query: its text and, on the hybrid path, the axis of the
// query vector (nothingAxis matches no stored embedding, so a keyword probe stays keyword).
type cmpProbe struct {
	query string
	axis  int
}

const nothingAxis = 1000

// cmpRun is what one default search returned: each post with its anchors (reply ids,
// sorted), the post order, the total and top_similarity.
type cmpRun struct {
	hits  map[string][]string
	order []string
	total int
	top   *float64
}

// cmpAxis is a unit vector on one axis. Distinct axes are orthogonal (cosine distance 1), so
// a query vector finds exactly the rows embedded on its axis, with similarity 1.
func cmpAxis(i int) []float32 {
	v := make([]float32, 1024)
	v[i] = 1
	return v
}

// Task idx 53 step 4: a fixed set of representative queries runs through the default search
// on one legacy fixture BEFORE and AFTER the knowledge cutover (RunKnowledgeCutover, the
// production sequence), on the full-text and the hybrid path. The comparison proves:
//   - no post found before is lost after; a query that matches no contribution returns the
//     same posts, order, total and top_similarity;
//   - every contribution the legacy contribution search matched (answers by content,
//     approaches by angle, method, outcome and solution) is found after as a reply anchor on
//     its post, unless its post is not publicly readable (draft, or closed/archived — the
//     canonical read rule the reply search uses on both paths);
//   - responses and comments, never searchable before, are found after;
//   - the legacy embeddings copied onto the migrated replies make them semantic matches, and
//     a post's own embedding matches exactly as before;
//   - each result's reply counts are its live replies (a deleted answer is not counted).
func TestCanonicalSearch_RepresentativeQueriesBeforeAndAfterCutover(t *testing.T) {
	pool, _ := newPreArchiveScratchDatabase(t)
	ctx := context.Background()
	scan := func(sql string, args ...any) string {
		t.Helper()
		var id string
		require.NoError(t, pool.QueryRow(ctx, sql, args...).Scan(&id), sql)
		return id
	}
	exec := func(sql string, args ...any) {
		t.Helper()
		_, err := pool.Exec(ctx, sql, args...)
		require.NoError(t, err, sql)
	}
	poster, contributor := "agent_cmp_poster", "agent_cmp_contributor"
	insertRemapAgent(t, pool, ctx, poster)
	insertRemapAgent(t, pool, ctx, contributor)
	human := scan(`INSERT INTO users (username, display_name, email, referral_code)
		VALUES ('cmphuman', 'Cmp Human', 'cmphuman@example.com', 'CMPHUMN1') RETURNING id::text`)
	post := func(postType, status, title, desc string) string {
		t.Helper()
		return insertTestPostWithAuthor(t, pool, ctx, postType, title, desc, nil, status, "agent", poster)
	}
	approach := func(problemID, angle, method, status, outcome string) string {
		t.Helper()
		return scan(`INSERT INTO approaches (problem_id, author_type, author_id, angle, method, status, outcome)
			VALUES ($1, 'agent', $2, $3, $4, $5, $6) RETURNING id::text`, problemID, contributor, angle, method, status, outcome)
	}
	answer := func(questionID, content string, deleted bool) string {
		t.Helper()
		return scan(`INSERT INTO answers (question_id, author_type, author_id, content, deleted_at)
			VALUES ($1, 'human', $2, $3, CASE WHEN $4 THEN NOW() END) RETURNING id::text`, questionID, human, content, deleted)
	}
	embed := func(table, id string, axis int) {
		t.Helper()
		exec(`UPDATE `+table+` SET embedding = $1::vector WHERE id = $2`, formatVectorLiteral(cmpAxis(axis)), id)
	}

	solved := post("problem", "solved", "cmpalpha goroutine leak", "workers keep cmpshared state alive")
	answered := post("question", "answered", "cmpbeta cache invalidation", "when should a cmpshared cache expire")
	active := post("idea", "active", "cmpgamma status board", "a board for every running agent")
	semantic := post("problem", "open", "an embedded problem", "described by its vector")
	embed("posts", semantic, 10)
	closed := post("question", "closed", "cmpnu an old question", "nobody asks this anymore")
	draft := post("question", "draft", "cmpxi a draft question", "not published yet")

	succeeded := approach(solved, "cmpdelta pin the runtime", "cmpshared bump the version", "succeeded", "cmpepsilon the leak stopped")
	embed("approaches", succeeded, 11)
	failed := approach(solved, "cmpzeta retry the loop", "restart it", "failed", "it came back")
	humanAnswer := answer(answered, "cmpeta version the keys", false)
	embed("answers", humanAnswer, 12)
	response := scan(`INSERT INTO responses (idea_id, author_type, author_id, content, response_type)
		VALUES ($1, 'agent', $2, 'cmptheta I would build it', 'build') RETURNING id::text`, active, contributor)
	comment := scan(`INSERT INTO comments (target_type, target_id, author_type, author_id, content)
		VALUES ('post', $1, 'agent', $2, 'cmpiota a fair question') RETURNING id::text`, answered, contributor)
	closedAnswer := answer(closed, "cmpkappa the answer nobody reads", false)
	draftAnswer := answer(draft, "cmplambda the draft answer", false)
	answer(answered, "cmpmu a withdrawn answer", true)

	// legacyContributionHits is the pre-cutover contribution search: the live answers and
	// approaches whose text matched, by legacy table and id.
	legacyContributionHits := func(query string) map[string]string {
		t.Helper()
		ts := buildTsQuery(query)
		hits := map[string]string{}
		for legacyType, sql := range map[string]string{
			"answer": `SELECT id::text FROM answers WHERE deleted_at IS NULL
				AND to_tsvector('english', content) @@ to_tsquery('english', $1)`,
			"approach": `SELECT id::text FROM approaches WHERE deleted_at IS NULL
				AND to_tsvector('english', COALESCE(angle, '') || ' ' || COALESCE(method, '') || ' ' ||
					COALESCE(outcome, '') || ' ' || COALESCE(solution, '')) @@ to_tsquery('english', $1)`,
		} {
			rows, err := pool.Query(ctx, sql, ts)
			require.NoError(t, err)
			for rows.Next() {
				var id string
				require.NoError(t, rows.Scan(&id))
				hits[id] = legacyType
			}
			require.NoError(t, rows.Err())
			rows.Close()
		}
		return hits
	}

	fulltext := NewSearchRepository(pool)
	search := func(path string, p cmpProbe) cmpRun {
		t.Helper()
		repo := fulltext
		if path == "hybrid_rrf" {
			repo = NewSearchRepository(pool)
			repo.SetEmbeddingService(&fixedEmbeddingService{vec: cmpAxis(p.axis)})
		}
		results, total, method, top, err := repo.Search(ctx, p.query, models.SearchOptions{Page: 1, PerPage: 50})
		require.NoError(t, err)
		require.Equal(t, path, method, p.query)
		run := cmpRun{hits: map[string][]string{}, total: total, top: top}
		for _, r := range results {
			run.order = append(run.order, r.ID)
			run.hits[r.ID] = []string{}
			for _, m := range r.MatchedReplies {
				run.hits[r.ID] = append(run.hits[r.ID], m.ID)
			}
			sort.Strings(run.hits[r.ID])
		}
		return run
	}

	keyword := func(words ...string) []cmpProbe {
		probes := make([]cmpProbe, len(words))
		for i, w := range words {
			probes[i] = cmpProbe{query: w, axis: nothingAxis}
		}
		return probes
	}
	probes := keyword("cmpalpha", "cmpbeta", "cmpgamma", "cmpnu", "cmpxi", "cmpshared", "cmpdelta",
		"cmpepsilon", "cmpzeta", "cmpeta", "cmptheta", "cmpiota", "cmpkappa", "cmplambda", "cmpmu")
	semanticProbes := []cmpProbe{{"cmpnothing", 10}, {"cmpnothing", 11}, {"cmpnothing", 12}}
	paths := []string{"fulltext_only", "hybrid_rrf"}

	before := map[string]map[cmpProbe]cmpRun{}
	legacyBefore := map[string]map[string]string{}
	for _, path := range paths {
		before[path] = map[cmpProbe]cmpRun{}
		for _, p := range probes {
			before[path][p] = search(path, p)
		}
	}
	for _, p := range semanticProbes {
		before["hybrid_rrf"][p] = search("hybrid_rrf", p)
	}
	for _, p := range probes {
		legacyBefore[p.query] = legacyContributionHits(p.query)
	}

	_, err := RunKnowledgeCutover(ctx, pool, KnowledgeCutoverOptions{})
	require.NoError(t, err)
	replyOf := func(legacyType, legacyID string) string {
		t.Helper()
		return scan(`SELECT id::text FROM replies WHERE legacy_type = $1 AND legacy_id = $2`, legacyType, legacyID)
	}

	// What the default search finds before and after, per probe (the same on both paths).
	none := map[string][]string{}
	wantBefore := map[string]map[string][]string{
		"cmpalpha": {solved: {}}, "cmpbeta": {answered: {}}, "cmpgamma": {active: {}}, "cmpnu": {closed: {}},
		"cmpshared": {solved: {}, answered: {}},
	}
	wantAfter := map[string]map[string][]string{
		"cmpalpha": {solved: {}}, "cmpbeta": {answered: {}}, "cmpgamma": {active: {}}, "cmpnu": {closed: {}},
		"cmpshared":  {solved: {replyOf("approach", succeeded)}, answered: {}},
		"cmpdelta":   {solved: {replyOf("approach", succeeded)}},
		"cmpepsilon": {solved: {replyOf("approach", succeeded)}},
		"cmpzeta":    {solved: {replyOf("approach", failed)}},
		"cmpeta":     {answered: {replyOf("answer", humanAnswer)}},
		"cmptheta":   {active: {replyOf("response", response)}},
		"cmpiota":    {answered: {replyOf("comment", comment)}},
	}
	// Legacy contributions the canonical search deliberately does not surface: their post is
	// not publicly readable (closed = archived, or a draft).
	notSurfaced := map[string]bool{closedAnswer: true, draftAnswer: true}

	for _, path := range paths {
		for _, p := range probes {
			b, a := before[path][p], search(path, p)
			wb, wa := wantBefore[p.query], wantAfter[p.query]
			if wb == nil {
				wb = none
			}
			if wa == nil {
				wa = none
			}
			assert.Equal(t, wb, b.hits, "%s %q before the cutover", path, p.query)
			assert.Equal(t, wa, a.hits, "%s %q after the cutover", path, p.query)
			assert.Equal(t, len(wa), a.total, "%s %q: total counts posts once", path, p.query)
			for id := range b.hits {
				assert.Contains(t, a.hits, id, "%s %q: post %s found before is lost after", path, p.query, id)
			}

			// Every contribution the legacy search matched is an anchor after, unless its
			// post is not publicly readable.
			anchors := map[string]bool{}
			for _, ids := range a.hits {
				for _, id := range ids {
					anchors[id] = true
				}
			}
			for legacyID, legacyType := range legacyBefore[p.query] {
				reply := replyOf(legacyType, legacyID)
				if notSurfaced[legacyID] {
					assert.False(t, anchors[reply], "%s %q: %s on a hidden post surfaced", path, p.query, legacyType)
				} else {
					assert.True(t, anchors[reply], "%s %q: legacy %s %s is not an anchor after", path, p.query, legacyType, legacyID)
				}
			}

			// A query no contribution matches returns what it returned before.
			noAnchors := true
			for _, ids := range a.hits {
				noAnchors = noAnchors && len(ids) == 0
			}
			if noAnchors {
				assert.Equal(t, b.order, a.order, "%s %q: same posts in the same order", path, p.query)
				assert.Equal(t, b.total, a.total, "%s %q: same total", path, p.query)
				assert.Equal(t, b.top, a.top, "%s %q: same top_similarity", path, p.query)
			}
		}
	}

	// Semantic probes: the post's own embedding matches as before; the embeddings copied from
	// the legacy approach and answer make their replies semantic matches after the cutover.
	for _, c := range []struct {
		probe         cmpProbe
		before, after map[string][]string
	}{
		{semanticProbes[0], map[string][]string{semantic: {}}, map[string][]string{semantic: {}}},
		{semanticProbes[1], none, map[string][]string{solved: {replyOf("approach", succeeded)}}},
		{semanticProbes[2], none, map[string][]string{answered: {replyOf("answer", humanAnswer)}}},
	} {
		b, a := before["hybrid_rrf"][c.probe], search("hybrid_rrf", c.probe)
		assert.Equal(t, c.before, b.hits, "axis %d before the cutover", c.probe.axis)
		assert.Equal(t, c.after, a.hits, "axis %d after the cutover", c.probe.axis)
		require.NotNil(t, a.top, "axis %d: a semantic match has a similarity", c.probe.axis)
		assert.InDelta(t, 1.0, *a.top, 1e-6, "axis %d", c.probe.axis)
	}
	assert.Nil(t, before["hybrid_rrf"][semanticProbes[1]].top, "no legacy contribution was a semantic match before")

	// Counts: each result's reply counts partition its live replies; the withdrawn
	// (deleted) answer is not counted.
	results, _, _, _, err := fulltext.Search(ctx, "cmpalpha cmpbeta cmpgamma", models.SearchOptions{Page: 1, PerPage: 50})
	require.NoError(t, err)
	require.Len(t, results, 3)
	counts := map[string][3]int{}
	for _, r := range results {
		counts[r.ID] = [3]int{r.AnswersCount, r.ApproachesCount, r.CommentsCount}
		var live int
		require.NoError(t, pool.QueryRow(ctx, `SELECT COUNT(*) FROM replies WHERE post_id = $1 AND deleted_at IS NULL`, r.ID).Scan(&live))
		assert.Equal(t, live, r.AnswersCount+r.ApproachesCount+r.CommentsCount, "post %s", r.ID)
	}
	assert.Equal(t, map[string][3]int{solved: {0, 2, 0}, answered: {1, 0, 1}, active: {1, 0, 0}}, counts)
}
