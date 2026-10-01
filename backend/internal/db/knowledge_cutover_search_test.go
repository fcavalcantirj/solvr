package db

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// Task idx 77 step 5: the cutover reconciles sampled search results as it reconciles the
// counters. With SearchSample it runs the most frequent recorded queries through the default
// search before converting anything and again at the end, and fails when a post found before
// is not found after while it is still publicly searchable, or when a legacy answer or
// approach the legacy contribution search matched is not found as a reply while public search
// reads it.

// searchSampleSeed names the question and the answer the failure test corrupts.
type searchSampleSeed struct{ answered, answer string }

func seedSearchSample(t *testing.T, pool *Pool) searchSampleSeed {
	t.Helper()
	ctx := context.Background()
	id := func(sql string, args ...any) string {
		t.Helper()
		var v string
		require.NoError(t, pool.QueryRow(ctx, sql, args...).Scan(&v), sql)
		return v
	}
	authorAgent(ctx, t, pool, "ks-agent")
	post := func(postType, status, title string) string {
		return id(`INSERT INTO posts (type, title, description, posted_by_type, posted_by_id, status)
			VALUES ($1, $2, 'body', 'agent', 'ks-agent', $3) RETURNING id::text`, postType, title, status)
	}
	answer := func(questionID, content string, deleted bool) string {
		return id(`INSERT INTO answers (question_id, author_type, author_id, content, deleted_at)
			VALUES ($1, 'agent', 'ks-agent', $2, CASE WHEN $3 THEN NOW() END) RETURNING id::text`, questionID, content, deleted)
	}
	var s searchSampleSeed
	solved := post("problem", "solved", "ksalpha goroutine leak")
	s.answered = post("question", "answered", "ksgamma cache invalidation")
	closed := post("question", "closed", "ksdelta an old question")
	draft := post("question", "draft", "ksepsilon a draft question")
	id(`INSERT INTO approaches (problem_id, author_type, author_id, angle, method, status)
		VALUES ($1, 'agent', 'ks-agent', 'ksbeta pin the runtime', 'bump it', 'succeeded') RETURNING id::text`, solved)
	id(`INSERT INTO approaches (problem_id, author_type, author_id, angle, deleted_at)
		VALUES ($1, 'agent', 'ks-agent', 'ksbeta a withdrawn approach', NOW()) RETURNING id::text`, solved)
	s.answer = answer(s.answered, "ksbeta version the keys", false)
	answer(closed, "ksbeta the answer nobody reads", false)
	answer(draft, "ksbeta the draft answer", false)
	answer(s.answered, "ksbeta a withdrawn answer", true)

	// Recorded searches, most frequent first. 'ksquote is rejected by the search before and
	// after the conversion; kszeta matches nothing; ksgamma is outside a sample of four.
	for q, n := range map[string]int{"ksbeta": 5, "ksalpha": 4, "'ksquote": 3, "kszeta": 2, "ksgamma": 1} {
		_, err := pool.Exec(ctx, `INSERT INTO search_queries (query, query_normalized, results_count, search_method, duration_ms)
			SELECT $1, $1, 0, 'fulltext_only', 1 FROM generate_series(1, $2)`, q, n)
		require.NoError(t, err)
	}
	return s
}

func TestKnowledgeCutover_SearchSampleFindsBeforeAndAfterTheMostFrequentQueries(t *testing.T) {
	pool, _ := newMigratedScratchDatabase(t)
	ctx := context.Background()
	seedSearchSample(t, pool)

	rep, err := RunKnowledgeCutover(ctx, pool, KnowledgeCutoverOptions{SearchSample: 4})
	require.NoError(t, err)
	got := rep.SearchSample
	require.NotNil(t, got)
	require.Equal(t, []string{"ksbeta", "ksalpha", "'ksquote", "kszeta"}, got.Queries, "most frequent first, ties by text")
	require.Equal(t, []string{"'ksquote"}, got.Errored, "a query the search rejects is left out, before and after")
	require.Empty(t, got.ErroredAfter)
	require.Equal(t, 1, got.PostsBefore, "ksalpha finds the solved problem by its own text")
	require.Equal(t, 3, got.PostsAfter, "ksbeta now finds the solved problem and the answered question through their replies")
	require.Empty(t, got.LostPosts)
	require.Empty(t, got.UnsearchablePosts)
	require.Equal(t, 4, got.Contributions, "the live approach and the three live answers match ksbeta")
	require.Equal(t, 2, got.ContributionsFound)
	require.Equal(t, 2, got.ContributionsHidden, "the answers on the closed and the draft question are not read by public search")
	require.Empty(t, got.MissingContributions)

	// The sample brackets the run: before anything is converted, and after everything is.
	require.Equal(t, "search_sample_before", rep.Steps[0].Name)
	require.Equal(t, "search_sample_after", rep.Steps[len(rep.Steps)-1].Name)
	require.Equal(t, 2, countRows(t, pool, ctx, `SELECT count(*) FROM cutover_ledger
		WHERE run_id = $1 AND step LIKE 'search_sample_%' AND finished_at IS NOT NULL AND error IS NULL`, rep.RunID))

	// A second run compares the converted model with itself.
	again, err := RunKnowledgeCutover(ctx, pool, KnowledgeCutoverOptions{SearchSample: 4})
	require.NoError(t, err)
	require.Equal(t, 3, again.SearchSample.PostsBefore)
	require.Equal(t, 3, again.SearchSample.PostsAfter)
	require.Equal(t, 2, again.SearchSample.ContributionsFound)
}

func TestKnowledgeCutover_WithoutASearchSampleNothingIsSampled(t *testing.T) {
	pool, _ := newMigratedScratchDatabase(t)
	ctx := context.Background()
	seedSearchSample(t, pool)

	rep, err := RunKnowledgeCutover(ctx, pool, KnowledgeCutoverOptions{})
	require.NoError(t, err)
	require.Nil(t, rep.SearchSample)
	for _, s := range rep.Steps {
		require.NotContains(t, s.Name, "search_sample")
	}
}

func TestKnowledgeCutover_ADryRunSamplesOnlyBefore(t *testing.T) {
	pool, _ := newMigratedScratchDatabase(t)
	ctx := context.Background()
	seedSearchSample(t, pool)

	rep, err := RunKnowledgeCutover(ctx, pool, KnowledgeCutoverOptions{DryRun: true, SearchSample: 2})
	require.NoError(t, err)
	require.Equal(t, []string{"ksbeta", "ksalpha"}, rep.SearchSample.Queries)
	require.Equal(t, 1, rep.SearchSample.PostsBefore)
	require.Equal(t, 4, rep.SearchSample.Contributions)
	require.Zero(t, rep.SearchSample.PostsAfter)
	require.Zero(t, rep.SearchSample.ContributionsFound)
	require.Equal(t, "search_sample_before", rep.Steps[0].Name)
	for _, s := range rep.Steps {
		require.NotEqual(t, "search_sample_after", s.Name)
	}
}

// A legacy answer that already has a reply is not converted again (the cutover is
// idempotent), so a reply that does not carry its contribution's text is a contribution the
// search lost: the run fails and names it.
func TestKnowledgeCutover_SearchSampleFailsWhenAMatchedContributionIsNotFoundAfter(t *testing.T) {
	pool, _ := newMigratedScratchDatabase(t)
	ctx := context.Background()
	s := seedSearchSample(t, pool)
	_, err := pool.Exec(ctx, `INSERT INTO replies (post_id, author_type, author_id, body, legacy_type, legacy_id)
		VALUES ($1, 'agent', 'ks-agent', 'a body without the answer', 'answer', $2)`, s.answered, s.answer)
	require.NoError(t, err)

	rep, err := RunKnowledgeCutover(ctx, pool, KnowledgeCutoverOptions{SearchSample: 4})
	require.Error(t, err)
	require.Contains(t, err.Error(), "search_sample_after")
	require.Equal(t, []SearchSampleMiss{{Query: "ksbeta", ID: s.answer, Kind: "answer"}}, rep.SearchSample.MissingContributions)
	require.Equal(t, 1, rep.SearchSample.ContributionsFound)
	require.Equal(t, 1, countRows(t, pool, ctx, `SELECT count(*) FROM cutover_ledger
		WHERE run_id = $1 AND step = 'search_sample_after' AND error LIKE '%1 matched contribution%'`, rep.RunID))
}

func TestCompareSearchSamples_ALostPostFailsOnlyWhileItIsStillSearchable(t *testing.T) {
	before := map[string][]string{"q1": {"p1", "p2", "p3"}, "q2": {"p4"}}
	after := map[string][]string{"q1": {"p3", "p1", "p9"}, "q2": {}}
	searchable := map[string]bool{"p1": true, "p3": true, "p4": true}

	lost, unsearchable := compareSampledPosts([]string{"q1", "q2"}, before, after, searchable)
	require.Equal(t, []SearchSampleMiss{{Query: "q2", ID: "p4", Kind: "post"}}, lost)
	require.Equal(t, []SearchSampleMiss{{Query: "q1", ID: "p2", Kind: "post"}}, unsearchable)
}

func TestSearchSampleReport_FailureNamesEveryUnexplainedDifference(t *testing.T) {
	require.NoError(t, (&SearchSampleReport{UnsearchablePosts: []SearchSampleMiss{{}}, ContributionsHidden: 3}).failure())
	require.EqualError(t, (&SearchSampleReport{
		LostPosts:            []SearchSampleMiss{{}, {}},
		MissingContributions: []SearchSampleMiss{{}},
		ErroredAfter:         []string{"x"},
	}).failure(), "2 post(s) found before are not found after while still searchable; "+
		"1 matched contribution(s) are not found as a reply; 1 query(ies) fail only after the conversion")
}

// The comparison reads every page: compared by its first page, a query whose new reply
// matches push old posts down would look like a loss (the rehearsal copy had 908 such).
func TestKnowledgeCutover_SearchSampleReadsEveryPage(t *testing.T) {
	pool, _ := newMigratedScratchDatabase(t)
	ctx := context.Background()
	authorAgent(ctx, t, pool, "ks-agent")
	_, err := pool.Exec(ctx, `INSERT INTO posts (type, title, description, posted_by_type, posted_by_id, status)
		SELECT 'idea', 'kspage idea ' || n, 'body', 'agent', 'ks-agent', 'open' FROM generate_series(1, 60) n`)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO search_queries (query, query_normalized, results_count, search_method, duration_ms)
		VALUES ('kspage', 'kspage', 60, 'fulltext_only', 1)`)
	require.NoError(t, err)

	rep, err := RunKnowledgeCutover(ctx, pool, KnowledgeCutoverOptions{SearchSample: 1})
	require.NoError(t, err)
	require.Equal(t, 60, rep.SearchSample.PostsBefore)
	require.Equal(t, 60, rep.SearchSample.PostsAfter)
}
