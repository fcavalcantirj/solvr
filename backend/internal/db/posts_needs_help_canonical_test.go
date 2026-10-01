package db

import (
	"context"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Task idx 76 step 3: the GET /v1/posts?needs_help=true filter (and the legacy /v1/feed/stuck
// route it serves) reads canonical replies, and the approach visibility check of the legacy
// approach routes lives with the legacy approach repository, so neither posts_list_filters.go
// nor visibility.go names a legacy table.
func TestLegacyNeedsHelpAndVisibility_ServedCanonically(t *testing.T) {
	src, err := ScanLegacySourceDependencies(backendRoot(t))
	require.NoError(t, err)
	for _, dep := range src {
		assert.NotEqual(t, "code:internal/db/posts_list_filters.go", dep.Key, "the needs-help filter names no legacy table or type")
		assert.NotEqual(t, "code:internal/db/visibility.go", dep.Key, "the shared visibility predicates name no legacy table or type")
	}
	for _, key := range []string{"code:internal/db/posts_list_filters.go", "code:internal/db/visibility.go"} {
		_, ok := LegacyDependencyDispositions[key]
		assert.False(t, ok, "%s no longer depends on a legacy table, so it carries no disposition", key)
	}
	assert.Equal(t, []string{"internal/db/approaches.go"},
		productionSourcesContaining(t, "func (r *ApproachesRepository) ApproachVisibleTo("),
		"the legacy approach routes' visibility check drops with the legacy approach repository")
	d, ok := LegacyDependencyDispositions["code:internal/db/approaches.go"]
	require.True(t, ok)
	assert.Equal(t, LegacyActionRetire, d.Action)
	assert.False(t, d.Done)
}

// Task idx 76 steps 3 and 5: a post needs help when it is in progress or carries a live stuck
// approach. After the contribution cutover the stuck approach is the reply migrated from it,
// whose status is kept in provenance, so the filter selects the same posts, and it keeps
// selecting them once the legacy tables are gone. An approach that was never migrated does
// not count, and neither does a native reply: replies have no status workflow.
func TestCanonicalNeedsHelp_KeepsTheLegacyFilterAcrossTheCutover(t *testing.T) {
	pool, dropLegacy := newMigratedScratchDatabase(t)
	ctx := context.Background()
	exec := func(sql string, args ...any) {
		t.Helper()
		_, err := pool.Exec(ctx, sql, args...)
		require.NoError(t, err, sql)
	}
	a := "agent_needs_help"
	insertRemapAgent(t, pool, ctx, a)
	tag := "needshelp"
	post := func(postType, status, title string) string {
		t.Helper()
		return insertTestPostWithAuthor(t, pool, ctx, postType, title, "needs help body", []string{tag}, status, "agent", a)
	}

	pInProgress := post("problem", "in_progress", "needs help: in progress, no approach")
	pStuck := post("problem", "open", "needs help: a stuck approach")
	insertApproach(t, pool, ctx, pStuck, a, "stuck", false)
	pMixed := post("problem", "open", "needs help: a working and a stuck approach")
	insertApproach(t, pool, ctx, pMixed, a, "working", false)
	insertApproach(t, pool, ctx, pMixed, a, "stuck", false)
	pWorking := post("problem", "open", "fine: a working approach")
	insertApproach(t, pool, ctx, pWorking, a, "working", false)
	pDeletedStuck := post("problem", "open", "fine: a deleted stuck approach")
	insertApproach(t, pool, ctx, pDeletedStuck, a, "stuck", true)
	pSolved := post("problem", "solved", "fine: solved, a failed approach")
	insertApproach(t, pool, ctx, pSolved, a, "failed", false)
	qOpen := post("question", "open", "fine: an open question")
	pFamily := post("problem", "open", "hidden: a family problem with a stuck approach")
	insertApproach(t, pool, ctx, pFamily, a, "stuck", false)
	exec(`UPDATE posts SET visibility = 'family' WHERE id = $1`, pFamily)
	pDeleted := post("problem", "in_progress", "hidden: a deleted problem in progress")
	exec(`UPDATE posts SET deleted_at = NOW() WHERE id = $1`, pDeleted)

	repo := NewPostRepository(pool)
	list := func() []string {
		t.Helper()
		posts, total, err := repo.List(ctx, models.PostListOptions{Tags: []string{tag}, NeedsHelp: true, Page: 1, PerPage: 50})
		require.NoError(t, err)
		ids := make([]string, 0, len(posts))
		for _, p := range posts {
			ids = append(ids, p.ID)
		}
		assert.Equal(t, len(ids), total)
		return ids
	}
	// The bare predicate, read without PostRepository.List (whose counts still join the legacy
	// tables until code:internal/db/posts.go is refactored).
	filter := func() []string {
		t.Helper()
		rows, err := pool.Query(ctx, `SELECT p.id::text FROM posts p
			WHERE p.deleted_at IS NULL AND `+publicOnlyVisibility("p")+` AND $1 = ANY(p.tags) AND `+needsHelpCondition, tag)
		require.NoError(t, err)
		defer rows.Close()
		var ids []string
		for rows.Next() {
			var id string
			require.NoError(t, rows.Scan(&id))
			ids = append(ids, id)
		}
		require.NoError(t, rows.Err())
		return ids
	}

	// Before the cutover no approach is a reply yet: only the in-progress post needs help.
	assert.ElementsMatch(t, []string{pInProgress}, list(), "an approach that was never migrated does not count")

	_, err := MigrateContributions(ctx, pool)
	require.NoError(t, err)
	_, err = RemapLegacyRelations(ctx, pool)
	require.NoError(t, err)

	want := []string{pInProgress, pStuck, pMixed}
	assert.ElementsMatch(t, want, list(), "in progress, or a live reply migrated from a stuck approach")
	assert.ElementsMatch(t, want, filter())

	dropLegacy()
	assert.ElementsMatch(t, want, filter(), "the needs-help filter needs no legacy table")

	// Native replies carry no status: the database refuses provenance on them (000118).
	_, err = pool.Exec(ctx, `INSERT INTO replies (post_id, author_type, author_id, body, provenance)
		VALUES ($1, 'agent', $2, 'native reply', '{"status": "stuck"}')`, pWorking, a)
	requireConstraintViolation(t, err, "replies_provenance_bounded")
	exec(`INSERT INTO replies (post_id, author_type, author_id, body) VALUES ($1, 'agent', $2, 'native reply')`, pWorking, a)
	exec(`INSERT INTO replies (post_id, author_type, author_id, body) VALUES ($1, 'agent', $2, 'native reply')`, qOpen, a)
	// A migrated stuck approach whose reply is deleted no longer counts.
	exec(`UPDATE replies SET deleted_at = NOW() WHERE post_id = $1 AND legacy_type = 'approach' AND provenance->>'status' = 'stuck'`, pMixed)
	assert.ElementsMatch(t, []string{pInProgress, pStuck}, filter())
}
