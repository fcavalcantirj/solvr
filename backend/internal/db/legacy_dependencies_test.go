package db

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func depKeys(deps []LegacyDependency) []string {
	var keys []string
	for _, d := range deps {
		keys = append(keys, d.Key)
	}
	sort.Strings(keys)
	return keys
}

func writeSource(t *testing.T, root, rel, content string) {
	t.Helper()
	p := filepath.Join(root, rel)
	require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
	require.NoError(t, os.WriteFile(p, []byte(content), 0o644))
}

// The source scan finds application SQL on legacy tables or legacy post/target types,
// scheduled jobs and LISTEN consumers, and ignores tests and unrelated code.
func TestScanLegacySourceDependencies_FindsQueriesJobsAndConsumers(t *testing.T) {
	root := t.TempDir()
	writeSource(t, root, "internal/db/a.go", "package db\nconst q = `SELECT id FROM approaches WHERE deleted_at IS NULL`\n")
	writeSource(t, root, "internal/db/b.go", "package db\nconst q = `SELECT id FROM posts p WHERE p.type = 'problem'`\n")
	writeSource(t, root, "internal/db/c.go", "package db\nconst q = `LEFT JOIN answers a ON a.question_id = p.id`\n")
	writeSource(t, root, "internal/db/clean.go", "package db\nconst q = `SELECT id FROM replies WHERE post_id = $1`\n")
	writeSource(t, root, "internal/db/a_test.go", "package db\nconst q = `SELECT id FROM comments`\n")
	writeSource(t, root, "cmd/api/main.go", "package main\nfunc main() {\n\tj := jobs.NewStaleContentJob(r)\n\tk := jobs.NewCleanupJob(t)\n}\n")
	writeSource(t, root, "internal/db/listener.go", "package db\nfunc l() { conn.Exec(ctx, \"LISTEN \"+RoomEntryChannel) }\n")

	deps, err := ScanLegacySourceDependencies(root)
	require.NoError(t, err)

	assert.Equal(t, []string{
		"code:internal/db/a.go",
		"code:internal/db/b.go",
		"code:internal/db/c.go",
		"consumer:RoomEntryChannel",
		"job:CleanupJob",
		"job:StaleContentJob",
	}, depKeys(deps))
	for _, d := range deps {
		if d.Key == "code:internal/db/a.go" {
			assert.Contains(t, d.Detail, "approaches")
		}
		if d.Key == "code:internal/db/b.go" {
			assert.Contains(t, d.Detail, "problem")
		}
	}
}

// A legacy table stays visible to the scan when the SQL qualifies it with the public schema
// or quotes it; the dropped-table query probe found the unqualified-only regex missing
// `FROM public.approaches`.
func TestScanLegacySourceDependencies_FindsSchemaQualifiedAndQuotedTables(t *testing.T) {
	root := t.TempDir()
	writeSource(t, root, "internal/db/qualified.go", "package db\nconst q = `SELECT id FROM public.approaches WHERE id = $1`\n")
	writeSource(t, root, "internal/db/quoted.go", "package db\nconst q = `LEFT JOIN \"answers\" a ON a.question_id = p.id`\n")
	writeSource(t, root, "internal/db/both.go", "package db\nconst q = `INSERT INTO public.\"comments\" (id) VALUES ($1)`\n")
	writeSource(t, root, "internal/db/clean.go", "package db\nconst q = `SELECT id FROM public.replies r JOIN \"posts\" p ON p.id = r.post_id`\n")

	deps, err := ScanLegacySourceDependencies(root)
	require.NoError(t, err)

	assert.Equal(t, []string{
		"code:internal/db/both.go",
		"code:internal/db/qualified.go",
		"code:internal/db/quoted.go",
	}, depKeys(deps))
}

// Schema cleanup is complete only when every discovered dependency has an explicit
// disposition and every non-keep disposition is done; an owned object (an index on a
// legacy table) inherits its owner's disposition unless it has its own.
func TestLegacyCleanupBlockers_RequireAnExplicitDoneDisposition(t *testing.T) {
	registry := map[string]LegacyDependencyDisposition{
		"table:approaches":       {Action: LegacyActionRetire, Done: true, Note: "migrated"},
		"function:hybrid_search": {Action: LegacyActionRefactor, Done: false, Note: "pending"},
		"consumer:X":             {Action: LegacyActionKeep, Note: "unrelated"},
		"index:approaches.idx_e": {Action: LegacyActionRefactor, Done: false, Note: "needs a replies index"},
	}
	deps := []LegacyDependency{
		{Kind: "table", Key: "table:approaches"},
		{Kind: "index", Key: "index:approaches.idx_a", Owner: "table:approaches"},
		{Kind: "index", Key: "index:approaches.idx_e", Owner: "table:approaches"},
		{Kind: "function", Key: "function:hybrid_search"},
		{Kind: "consumer", Key: "consumer:X"},
		{Kind: "code", Key: "code:internal/db/new.go"},
	}

	blockers := LegacyCleanupBlockers(deps, registry)
	assert.Equal(t, []string{
		"code:internal/db/new.go (no disposition)",
		"function:hybrid_search (refactor pending)",
		"index:approaches.idx_e (refactor pending)",
	}, blockers)

	registry["function:hybrid_search"] = LegacyDependencyDisposition{Action: LegacyActionRefactor, Done: true, Note: "done"}
	registry["index:approaches.idx_e"] = LegacyDependencyDisposition{Action: LegacyActionRefactor, Done: true, Note: "done"}
	registry["code:internal/db/new.go"] = LegacyDependencyDisposition{Action: LegacyActionRetire, Done: true, Note: "done"}
	assert.Empty(t, LegacyCleanupBlockers(deps, registry))
}

// Every declared relationship (step 2) and worker/feature (step 3) of the task is part
// of the inventory, so none can be forgotten because no query happened to match it.
func TestDeclaredLegacyDependencies_CoverTheTaskLists(t *testing.T) {
	keys := depKeys(DeclaredLegacyDependencies())
	for _, k := range []string{
		"relation:votes", "relation:bookmarks", "relation:reports", "relation:notifications",
		"relation:accepted-answer-provenance", "relation:approach-relationships",
		"relation:progress-notes", "relation:verification-records", "relation:translations",
		"relation:archived-cids",
		"feature:briefing", "feature:badges", "feature:leaderboards", "feature:reputation",
		"feature:crystallization", "feature:forgetting", "feature:moderation",
		"feature:duplicate-detection", "feature:embedding-workers",
	} {
		assert.Contains(t, keys, k)
	}
}

func backendRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	require.NoError(t, err)
	_, err = os.Stat(filepath.Join(root, "go.mod"))
	require.NoError(t, err, "backend root must hold go.mod")
	return root
}

// assertRegistryCovers fails for a discovered key without a disposition and for a
// registry key of the same kinds that no longer matches anything (a stale entry).
func assertRegistryCovers(t *testing.T, deps []LegacyDependency, kinds map[string]bool) {
	t.Helper()
	found := map[string]bool{}
	for _, d := range deps {
		found[d.Key] = true
		if d.Owner != "" {
			continue // covered by its owner's disposition unless listed on its own
		}
		_, ok := LegacyDependencyDispositions[d.Key]
		assert.True(t, ok, "discovered legacy dependency without a disposition: %s (%s)", d.Key, d.Detail)
	}
	for key := range LegacyDependencyDispositions {
		if kinds[legacyKeyKind(key)] {
			assert.True(t, found[key], "disposition for a dependency that no longer exists: %s", key)
		}
	}
}

// Every application query, scheduled job and queue consumer in the real tree has a
// disposition, and the registry names none that is gone.
func TestLegacyDependencyRegistry_CoversTheSourceTree(t *testing.T) {
	deps, err := ScanLegacySourceDependencies(backendRoot(t))
	require.NoError(t, err)
	require.NotEmpty(t, deps)
	assertRegistryCovers(t, deps, map[string]bool{"code": true, "job": true, "consumer": true})
}

func TestLegacyDependencyRegistry_CoversTheDeclaredLists(t *testing.T) {
	assertRegistryCovers(t, DeclaredLegacyDependencies(), map[string]bool{"relation": true, "feature": true})
}

// Every note says what happens, so a disposition is a decision and not a placeholder.
func TestLegacyDependencyRegistry_EveryEntryIsADecision(t *testing.T) {
	valid := map[LegacyDependencyAction]bool{
		LegacyActionRemap: true, LegacyActionRefactor: true, LegacyActionRetire: true, LegacyActionKeep: true,
	}
	for key, d := range LegacyDependencyDispositions {
		assert.True(t, valid[d.Action], "%s: unknown action %q", key, d.Action)
		assert.GreaterOrEqual(t, len(d.Note), 20, "%s: note must explain the decision", key)
		assert.NotEmpty(t, legacyKeyKind(key), "%s: key must be kind:name", key)
	}
}

// The live catalog of a fully migrated database: every foreign key, view, function,
// trigger, index, check constraint and column tied to a legacy table or type is found and
// has a disposition, and the registry names no catalog object that is gone.
func TestDiscoverLegacySchemaDependencies_EveryCatalogObjectHasADisposition(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	ctx := context.Background()
	pool, err := NewPool(ctx, dbURL)
	require.NoError(t, err)
	defer pool.Close()

	deps, err := DiscoverLegacySchemaDependencies(ctx, pool)
	require.NoError(t, err)
	keys := depKeys(deps)

	// Known objects from the migrated schema must be found (the discovery is not vacuous).
	for _, k := range []string{
		"table:approaches", "table:answers", "table:responses", "table:comments",
		"table:approach_relationships", "table:progress_notes",
		"function:hybrid_search_approaches", "function:hybrid_search_answers",
		"check:votes.votes_target_type_check", "check:flags.flags_target_type_check",
		"check:posts.posts_type_check", "column:posts.accepted_answer_id",
		"index:approaches.idx_approaches_embedding", "fk:progress_notes.progress_notes_approach_id_fkey",
	} {
		assert.Contains(t, keys, k)
	}
	for _, d := range deps {
		if d.Key == "index:answers.answers_pkey" {
			assert.Equal(t, "table:answers", d.Owner)
		}
	}
	assertRegistryCovers(t, deps, map[string]bool{
		"table": true, "fk": true, "view": true, "function": true, "trigger": true,
		"index": true, "check": true, "column": true,
	})
}
