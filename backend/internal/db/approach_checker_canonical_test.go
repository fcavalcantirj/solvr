package db

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Task idx 76 steps 3 and 5: PATCH /v1/posts/{id} lets an owner mark a problem solved only
// when it has a succeeded approach. The router checks that with
// CanonicalApproachCheckerRepository, which reads the replies migrated from succeeded
// approaches; the legacy ApproachesRepository.HasSucceededApproach reads the approaches table
// and stays only on the legacy approach routes until the tables go.
func TestLegacyApproachChecker_ServedCanonically(t *testing.T) {
	assert.ElementsMatch(t, []string{"internal/api/router.go"},
		productionSourcesContaining(t, "db.NewCanonicalApproachCheckerRepository("),
		"PATCH /v1/posts checks a solved problem with the canonical checker")
	assert.Empty(t, productionSourcesContaining(t, "SetApproachChecker(db.NewApproachesRepository("),
		"no route checks the legacy approaches table before marking a problem solved")

	src, err := ScanLegacySourceDependencies(backendRoot(t))
	require.NoError(t, err)
	for _, dep := range src {
		assert.NotEqual(t, "code:internal/db/approach_checker_canonical.go", dep.Key, "the canonical checker names no legacy table or type")
	}
	d, ok := LegacyDependencyDispositions["code:internal/db/approaches.go"]
	require.True(t, ok)
	assert.Equal(t, LegacyActionRetire, d.Action)
	assert.False(t, d.Done, "approaches.go still serves the legacy approach routes")
	assert.Contains(t, d.Note, "approach_checker_canonical.go", "the note names the served solved-status check")
}

// Task idx 76 steps 3 and 5: in a database holding only this fixture, the canonical checker
// gives every problem the legacy answer once the contribution cutover has run, and still
// does after the legacy tables are dropped. Deleting the migrated reply withdraws the
// succeeded approach; a native reply (it has no status) never counts as one.
func TestCanonicalApproachChecker_KeepsTheLegacyAnswerAcrossTheCutover(t *testing.T) {
	pool, dropLegacy := newMigratedScratchDatabase(t)
	ctx := context.Background()
	exec := func(sql string, args ...any) {
		t.Helper()
		_, err := pool.Exec(ctx, sql, args...)
		require.NoError(t, err, sql)
	}
	a := "agent_solved_check"
	insertRemapAgent(t, pool, ctx, a)
	problem := func(title string) string {
		t.Helper()
		return insertTestPostWithAuthor(t, pool, ctx, "problem", title, "solved check body", []string{"solved"}, "open", "agent", a)
	}

	pSucceeded := problem("solved check: a succeeded approach")
	insertApproach(t, pool, ctx, pSucceeded, a, "succeeded", false)
	pMixed := problem("solved check: failed then succeeded")
	insertApproach(t, pool, ctx, pMixed, a, "failed", false)
	insertApproach(t, pool, ctx, pMixed, a, "succeeded", false)
	pDeleted := problem("solved check: a deleted succeeded approach")
	insertApproach(t, pool, ctx, pDeleted, a, "succeeded", true)
	pWorking := problem("solved check: working and stuck approaches")
	insertApproach(t, pool, ctx, pWorking, a, "working", false)
	insertApproach(t, pool, ctx, pWorking, a, "stuck", false)
	pFailed := problem("solved check: a failed approach")
	insertApproach(t, pool, ctx, pFailed, a, "failed", false)
	pNone := problem("solved check: no approach")
	problems := []string{pSucceeded, pMixed, pDeleted, pWorking, pFailed, pNone}

	answers := func(repo interface {
		HasSucceededApproach(context.Context, string) (bool, error)
	}) map[string]bool {
		t.Helper()
		out := map[string]bool{}
		for _, p := range problems {
			has, err := repo.HasSucceededApproach(ctx, p)
			require.NoError(t, err)
			out[p] = has
		}
		return out
	}

	want := map[string]bool{pSucceeded: true, pMixed: true, pDeleted: false, pWorking: false, pFailed: false, pNone: false}
	require.Equal(t, want, answers(NewApproachesRepository(pool)), "legacy: a live succeeded approach")

	checker := NewCanonicalApproachCheckerRepository(pool)
	assert.Equal(t, map[string]bool{pSucceeded: false, pMixed: false, pDeleted: false, pWorking: false, pFailed: false, pNone: false},
		answers(checker), "before the cutover no approach is a reply yet")

	_, err := MigrateContributions(ctx, pool)
	require.NoError(t, err)
	_, err = RemapLegacyRelations(ctx, pool)
	require.NoError(t, err)
	assert.Equal(t, want, answers(checker), "the legacy answer, read from the migrated replies")

	dropLegacy()
	assert.Equal(t, want, answers(checker), "the canonical checker needs no legacy table")

	exec(`INSERT INTO replies (post_id, author_type, author_id, body) VALUES ($1, 'agent', $2, 'a native reply that worked')`, pNone, a)
	exec(`UPDATE replies SET deleted_at = NOW() WHERE post_id = $1`, pSucceeded)
	want[pSucceeded] = false
	assert.Equal(t, want, answers(checker), "a deleted migrated reply withdraws it; a native reply is no succeeded approach")
}
