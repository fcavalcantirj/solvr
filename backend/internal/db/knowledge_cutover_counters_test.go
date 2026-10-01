package db

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// idx 77 step 5: after the migration every stored counter is reconciled with the records it
// counts, not only the two the 2026-09-29 rehearsal found drifting. view_count (000120) and
// agents.reputation (000121) gained their drift and rebuild functions after the cutover
// sequence was written, and the cutover never ran them.

func TestKnowledgeCutover_ReconcilesEveryStoredCounter(t *testing.T) {
	pool, _ := newMigratedScratchDatabase(t)
	ctx := context.Background()
	s := seedCutoverLegacy(t, pool)
	exec := func(sql string, args ...any) {
		t.Helper()
		_, err := pool.Exec(ctx, sql, args...)
		require.NoError(t, err, sql)
	}
	// Two view rows (the trigger counts them: 2), then a stored count they do not back.
	exec(`INSERT INTO post_views (post_id, viewer_type, viewer_id) VALUES ($1, 'agent', 'v1'), ($1, 'agent', 'v2')`, s.question)
	exec(`UPDATE posts SET view_count = 7 WHERE id = $1`, s.question)
	// A stored reputation no grant backs (kc-agent has no claim, no model, no grant).
	exec(`UPDATE agents SET reputation = 999 WHERE id = 'kc-agent'`)

	dry, err := RunKnowledgeCutover(ctx, pool, KnowledgeCutoverOptions{DryRun: true})
	require.NoError(t, err)
	require.EqualValues(t, 1, dry.ViewDriftBefore)
	require.EqualValues(t, 1, dry.ReputationDriftBefore)
	require.Equal(t, 7, countRows(t, pool, ctx, `SELECT view_count FROM posts WHERE id = $1`, s.question), "a dry run repairs nothing")
	require.Equal(t, 999, countRows(t, pool, ctx, `SELECT reputation FROM agents WHERE id = 'kc-agent'`))

	rep, err := RunKnowledgeCutover(ctx, pool, KnowledgeCutoverOptions{})
	require.NoError(t, err)
	require.EqualValues(t, 1, rep.ViewDriftBefore)
	require.EqualValues(t, 1, rep.ViewCountsRebuilt)
	require.Zero(t, rep.ViewDriftAfter)
	require.EqualValues(t, 1, rep.ReputationDriftBefore)
	require.EqualValues(t, 1, rep.ReputationRebuilt)
	require.Zero(t, rep.ReputationDriftAfter)
	require.Equal(t, 2, countRows(t, pool, ctx, `SELECT view_count FROM posts WHERE id = $1`, s.question))
	require.Equal(t, 0, countRows(t, pool, ctx, `SELECT reputation FROM agents WHERE id = 'kc-agent'`))

	// Every stored counter is a ledgered step of the run, and none drifts afterwards.
	steps := map[string]bool{}
	for _, st := range rep.Steps {
		steps[st.Name] = true
	}
	for _, c := range cutoverCounters {
		require.True(t, steps[c.step], "the cutover reconciles %s", c.step)
		require.Equal(t, 0, countRows(t, pool, ctx, `SELECT count(*) FROM `+c.drift+`()`), c.drift)
	}

	again, err := RunKnowledgeCutover(ctx, pool, KnowledgeCutoverOptions{})
	require.NoError(t, err)
	require.Zero(t, again.ViewDriftBefore)
	require.Zero(t, again.ViewCountsRebuilt)
	require.Zero(t, again.ReputationDriftBefore)
	require.Zero(t, again.ReputationRebuilt)
}

// The search documents are the one projection the cutover cannot rebuild: a vector needs the
// embedding service (SearchDocumentJob, cmd/backfill-embeddings). The run reports how many
// live rows still lack one and never fails on them (251 on the post-purge production copy).
func TestKnowledgeCutover_ReportsSearchDocumentsWithoutAVector(t *testing.T) {
	pool, _ := newMigratedScratchDatabase(t)
	ctx := context.Background()
	seedCutoverLegacy(t, pool)

	rep, err := RunKnowledgeCutover(ctx, pool, KnowledgeCutoverOptions{})
	require.NoError(t, err)
	// The 3 legacy posts, plus the answer, the approach and the progress note converted to
	// replies; the moderator's comment becomes a system reply, which is never embedded.
	want := countRows(t, pool, ctx, `SELECT count(*) FROM search_document_drift()`)
	require.Equal(t, 6, want)
	require.EqualValues(t, want, rep.SearchDocumentsPending)

	dry, err := RunKnowledgeCutover(ctx, pool, KnowledgeCutoverOptions{DryRun: true})
	require.NoError(t, err)
	require.EqualValues(t, want, dry.SearchDocumentsPending)
}

// Every drift function in the schema is reconciled by the cutover, except the search
// documents', which it reports. A new stored counter with a drift function and no cutover step
// fails here.
func TestCutoverCounters_CoverEveryDriftFunctionInTheSchema(t *testing.T) {
	pool, _ := newMigratedScratchDatabase(t)
	ctx := context.Background()

	rows, err := pool.Query(ctx, `SELECT p.proname FROM pg_proc p JOIN pg_namespace n ON n.oid = p.pronamespace
		WHERE n.nspname = 'public' AND p.proname LIKE '%\_drift'`)
	require.NoError(t, err)
	var inSchema []string
	for rows.Next() {
		var name string
		require.NoError(t, rows.Scan(&name))
		inSchema = append(inSchema, name)
	}
	require.NoError(t, rows.Err())

	covered := []string{searchDocumentDrift}
	for _, c := range cutoverCounters {
		covered = append(covered, c.drift)
		var result string
		require.NoError(t, pool.QueryRow(ctx, `SELECT pg_get_function_result(p.oid) FROM pg_proc p
			JOIN pg_namespace n ON n.oid = p.pronamespace WHERE n.nspname = 'public' AND p.proname = $1`, c.rebuild).Scan(&result), c.rebuild)
		require.Equal(t, "integer", result, "%s returns the rows it repaired", c.rebuild)
	}
	require.ElementsMatch(t, inSchema, covered)
}
