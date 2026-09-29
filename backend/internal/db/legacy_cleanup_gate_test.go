package db

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func cleanupWhats(cleanups []LegacySchemaCleanup) []string {
	var out []string
	for _, c := range cleanups {
		out = append(out, c.String())
	}
	return out
}

// The scan reports only up-migration statements that remove legacy runtime storage: a legacy
// table dropped or renamed away, a column dropped from a legacy table, a column carrying a
// legacy identity dropped, a legacy function dropped, a named check narrowed so it no longer
// admits a legacy type it admitted before. The shapes the real history uses to widen checks,
// drop constraints on legacy tables or drop unrelated columns are not cleanup; neither are
// SQL comments or down migrations.
func TestScanLegacySchemaCleanup_FindsRemovalsNotMaintenance(t *testing.T) {
	dir := t.TempDir()
	writeSource(t, dir, "000001_create.up.sql", `CREATE TABLE approaches (id UUID PRIMARY KEY);
CREATE TABLE posts (
    id UUID PRIMARY KEY,
    accepted_answer_id UUID,
    CONSTRAINT posts_type_check CHECK (type IN ('problem', 'question', 'idea')),
    CONSTRAINT posts_posted_by_type_check CHECK (posted_by_type IN ('human', 'agent'))
);
`)
	writeSource(t, dir, "000002_widen.up.sql", `-- A later cleanup will DROP TABLE approaches; not this one.
ALTER TABLE posts DROP CONSTRAINT posts_type_check;
ALTER TABLE posts ADD CONSTRAINT posts_type_check
    CHECK (type IN ('problem', 'question', 'idea', 'post'));
ALTER TABLE approaches DROP CONSTRAINT approaches_status_check;
ALTER TABLE comments DROP CONSTRAINT IF EXISTS comments_author_type_check;
ALTER TABLE posts ALTER COLUMN accepted_answer_id DROP NOT NULL;
ALTER TABLE rooms DROP COLUMN IF EXISTS owner_id;
DROP FUNCTION IF EXISTS hybrid_search(text, vector(1024), int, float, float, int);
DROP TABLE IF EXISTS comments_archive;
CREATE OR REPLACE FUNCTION f() RETURNS int AS $$ SELECT 1; $$ LANGUAGE sql;
`)
	writeSource(t, dir, "000003_cleanup.up.sql", `DROP TABLE IF EXISTS public.approaches, "answers" CASCADE;
DROP FUNCTION IF EXISTS hybrid_search_answers(text, vector(1024), int, float, float, int), legacy_fn(int);
ALTER TABLE posts DROP COLUMN IF EXISTS accepted_answer_id;
ALTER TABLE posts DROP CONSTRAINT posts_type_check;
ALTER TABLE posts ADD CONSTRAINT posts_type_check CHECK (type = 'post');
ALTER TABLE comments RENAME TO comments_old;
ALTER TABLE ONLY progress_notes DROP note;
`)
	writeSource(t, dir, "000003_cleanup.down.sql", "DROP TABLE responses;\n")

	cleanups, err := ScanLegacySchemaCleanup(dir, []string{"legacy_fn"})
	require.NoError(t, err)

	assert.Equal(t, []string{
		"000003_cleanup.up.sql:1 drops table approaches",
		"000003_cleanup.up.sql:1 drops table answers",
		"000003_cleanup.up.sql:2 drops function hybrid_search_answers",
		"000003_cleanup.up.sql:2 drops function legacy_fn",
		"000003_cleanup.up.sql:3 drops column posts.accepted_answer_id",
		"000003_cleanup.up.sql:5 narrows posts_type_check (no longer admits 'idea', 'problem', 'question')",
		"000003_cleanup.up.sql:6 renames table comments",
		"000003_cleanup.up.sql:7 drops column progress_notes.note",
	}, cleanupWhats(cleanups))
}

// Without the registry's function names a function that does not carry a legacy table name
// is not recognized, so the gate must pass them in.
func TestScanLegacySchemaCleanup_FunctionNamesComeFromTheCaller(t *testing.T) {
	dir := t.TempDir()
	writeSource(t, dir, "000001_drop.up.sql", "DROP FUNCTION legacy_fn(int);\n")

	cleanups, err := ScanLegacySchemaCleanup(dir, nil)
	require.NoError(t, err)
	assert.Empty(t, cleanups)
}

// Step 6 where schema cleanup is written: a migration may remove legacy storage only once
// every dependency has an explicit disposition and every non-keep disposition is done. A
// pending registry entry blocks even when no scan discovers its object (the catalog is not
// read without a database).
func TestLegacySchemaCleanupGate_BlocksCleanupWhileAnyDispositionIsPending(t *testing.T) {
	quiet := t.TempDir()
	writeSource(t, quiet, "000001_widen.up.sql", "ALTER TABLE votes ADD CONSTRAINT votes_target_type_check CHECK (target_type IN ('post', 'answer', 'reply'));\n")
	cleanup := t.TempDir()
	writeSource(t, cleanup, "000001_drop.up.sql", "DROP TABLE approaches;\n")

	registry := map[string]LegacyDependencyDisposition{
		"table:approaches":         {Action: LegacyActionRetire, Done: false, Note: "drop after verified migration"},
		"code:internal/db/keep.go": {Action: LegacyActionKeep, Note: "unrelated"},
	}
	deps := []LegacyDependency{{Kind: "code", Key: "code:internal/db/keep.go"}}

	require.NoError(t, LegacySchemaCleanupGate(quiet, deps, registry), "no cleanup, nothing to gate")

	err := LegacySchemaCleanupGate(cleanup, deps, registry)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "000001_drop.up.sql:1 drops table approaches")
	assert.Contains(t, err.Error(), "table:approaches (retire pending)")
	assert.NotContains(t, err.Error(), "keep.go")

	withUnknown := append(deps, LegacyDependency{Kind: "code", Key: "code:internal/db/new.go"})
	registry["table:approaches"] = LegacyDependencyDisposition{Action: LegacyActionRetire, Done: true, Note: "migrated and verified"}
	err = LegacySchemaCleanupGate(cleanup, withUnknown, registry)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "code:internal/db/new.go (no disposition)")

	require.NoError(t, LegacySchemaCleanupGate(cleanup, deps, registry), "every disposition done or keep")
}

// The real migrations against the real inventory: today's history must not contain cleanup
// while dispositions are pending, and a cleanup migration appended to that history is
// blocked by exactly the real blockers until each of them is done.
func TestLegacySchemaCleanupGate_TheRealMigrations(t *testing.T) {
	root := backendRoot(t)
	deps, err := ScanLegacySourceDependencies(root)
	require.NoError(t, err)
	deps = append(deps, DeclaredLegacyDependencies()...)
	migrations := filepath.Join(root, "migrations")

	require.NoError(t, LegacySchemaCleanupGate(migrations, deps, LegacyDependencyDispositions))

	blockers := LegacyCleanupBlockers(append(deps, registryDependencies(LegacyDependencyDispositions)...), LegacyDependencyDispositions)
	t.Logf("%d legacy cleanup blockers:\n  %s", len(blockers), strings.Join(blockers, "\n  "))

	appended := t.TempDir()
	files, err := filepath.Glob(filepath.Join(migrations, "*.sql"))
	require.NoError(t, err)
	require.NotEmpty(t, files)
	for _, f := range files {
		src, err := os.ReadFile(f)
		require.NoError(t, err)
		writeSource(t, appended, filepath.Base(f), string(src))
	}
	writeSource(t, appended, "999999_drop_legacy.up.sql", "DROP TABLE IF EXISTS approaches;\n")

	err = LegacySchemaCleanupGate(appended, deps, LegacyDependencyDispositions)
	if len(blockers) == 0 {
		require.NoError(t, err)
		return
	}
	require.Error(t, err)
	assert.Contains(t, err.Error(), "999999_drop_legacy.up.sql:1 drops table approaches")
	for _, b := range blockers {
		assert.Contains(t, err.Error(), b)
	}
}
