package db

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Task idx 76 steps 3 and 6: the /v1/stats/ideas sidebar reads only posts. It lives in
// stats_ideas.go and passes the idea type as a parameter, so stats.go names a legacy table or
// type only in the unwired methods that drop with the legacy tables (they compare the problem
// and question types, never the idea type).
func TestLegacyStats_IdeaSidebarNamesNoLegacyType(t *testing.T) {
	root := backendRoot(t)
	statsSrc, err := os.ReadFile(filepath.Join(root, "internal", "db", "stats.go"))
	require.NoError(t, err)
	var ideaTypes []string
	for _, m := range legacySQLTypeRe.FindAllStringSubmatch(string(statsSrc), -1) {
		if m[1] == "idea" {
			ideaTypes = append(ideaTypes, m[0])
		}
	}
	assert.Empty(t, ideaTypes, "stats.go compares no idea type: the idea sidebar moved to stats_ideas.go")

	for _, method := range []string{"GetIdeasCountByStatus", "GetFreshSparks", "GetReadyToDevelop",
		"GetTopSparklers", "GetIdeaPipelineStats", "GetRecentlyRealized"} {
		assert.Equal(t, []string{"internal/db/stats_ideas.go"},
			productionSourcesContaining(t, "func (r *StatsRepository) "+method+"("), method)
	}

	src, err := ScanLegacySourceDependencies(root)
	require.NoError(t, err)
	for _, dep := range src {
		assert.NotEqual(t, "code:internal/db/stats_ideas.go", dep.Key, "the idea sidebar names no legacy table or type")
	}
	_, ok := LegacyDependencyDispositions["code:internal/db/stats_ideas.go"]
	assert.False(t, ok, "no disposition for a file that is not a legacy dependency")
}

// ideasReader is what GET /v1/stats/ideas reads from the stats repository it is served by.
type ideasReader interface {
	GetIdeasCountByStatus(ctx context.Context) (map[string]int, error)
	GetFreshSparks(ctx context.Context, limit int) ([]map[string]any, error)
	GetReadyToDevelop(ctx context.Context, limit int) ([]map[string]any, error)
	GetTopSparklers(ctx context.Context, limit int) ([]map[string]any, error)
	GetIdeaPipelineStats(ctx context.Context) (map[string]any, error)
	GetRecentlyRealized(ctx context.Context, limit int) ([]map[string]any, error)
}

type ideasSnapshot struct {
	Counts   map[string]int
	Fresh    []map[string]any // id, title, support
	Ready    []map[string]any
	Top      []map[string]any
	Pipeline map[string]any
	Realized []map[string]any
}

func takeIdeasSnapshot(t *testing.T, ctx context.Context, r ideasReader) ideasSnapshot {
	t.Helper()
	var s ideasSnapshot
	var err error
	s.Counts, err = r.GetIdeasCountByStatus(ctx)
	require.NoError(t, err)
	fresh, err := r.GetFreshSparks(ctx, 100)
	require.NoError(t, err)
	s.Fresh = []map[string]any{}
	for _, m := range fresh {
		require.IsType(t, time.Time{}, m["created_at"])
		s.Fresh = append(s.Fresh, map[string]any{"id": m["id"], "title": m["title"], "support": m["support"]})
	}
	s.Ready, err = r.GetReadyToDevelop(ctx, 100)
	require.NoError(t, err)
	s.Top, err = r.GetTopSparklers(ctx, 100)
	require.NoError(t, err)
	s.Pipeline, err = r.GetIdeaPipelineStats(ctx)
	require.NoError(t, err)
	s.Realized, err = r.GetRecentlyRealized(ctx, 100)
	require.NoError(t, err)
	return s
}

// Task idx 76 steps 3 and 5: the idea sidebar, as the router serves it (CanonicalStatsRepository),
// gives the same figures before the contribution cutover, after it and once the legacy tables
// are dropped: it reads only live public idea posts and their lifecycle status. The idea figures
// are a subset of the posts.type column, so once check:posts.posts_type_check narrows the column
// to 'post' they read zero and empty lists, without an error.
func TestCanonicalIdeaStats_ReadOnlyPostsAcrossTheCutover(t *testing.T) {
	pool, dropLegacy := newMigratedScratchDatabase(t)
	ctx := context.Background()
	t0 := time.Now().UTC().Truncate(time.Second)
	hoursAgo := func(h float64) time.Time { return t0.Add(-time.Duration(h * float64(time.Hour))) }
	exec := func(sql string, args ...any) {
		t.Helper()
		_, err := pool.Exec(ctx, sql, args...)
		require.NoError(t, err, sql)
	}

	ia, ib := "agent_ideas_a", "agent_ideas_b"
	insertRemapAgent(t, pool, ctx, ia)
	insertRemapAgent(t, pool, ctx, ib)
	human, err := NewUserRepository(pool).Create(ctx, &models.User{
		Username: "ideashuman", DisplayName: "Ideas Human", Email: "ideashuman@example.com",
		AuthProvider: models.AuthProviderGitHub, AuthProviderID: "gh_ideashuman", Role: models.UserRoleUser,
	})
	require.NoError(t, err)
	hu := human.ID

	post := func(postType, title, status, authorType, author string, created, updated time.Time, up, down int) string {
		id := insertTestPostWithAuthor(t, pool, ctx, postType, title, "body", []string{"ideas"}, status, authorType, author)
		exec(`UPDATE posts SET created_at = $2, updated_at = $3, upvotes = $4, downvotes = $5 WHERE id = $1`,
			id, created, updated, up, down)
		return id
	}
	target := post("problem", "built from an idea", "open", "agent", ia, hoursAgo(1), hoursAgo(1), 30, 0)
	fresh := post("idea", "fresh spark", "open", "agent", ia, hoursAgo(2), hoursAgo(2), 12, 4)
	older := post("idea", "older active", "active", "human", hu, hoursAgo(48), hoursAgo(48), 10, 0)
	quiet := post("idea", "quiet active", "active", "agent", ib, hoursAgo(3), hoursAgo(3), 2, 0)
	evolved := post("idea", "realized idea", "evolved", "agent", ia, hoursAgo(240), hoursAgo(144), 5, 0)
	exec(`UPDATE posts SET evolved_into = ARRAY[$2::uuid] WHERE id = $1`, evolved, target)
	post("idea", "dormant idea", "dormant", "agent", ib, hoursAgo(720), hoursAgo(720), 0, 0)
	family := post("idea", "family idea", "open", "agent", ia, hoursAgo(1), hoursAgo(1), 50, 0)
	exec(`UPDATE posts SET visibility = 'family' WHERE id = $1`, family)
	deleted := post("idea", "deleted idea", "open", "agent", ib, hoursAgo(1), hoursAgo(1), 40, 0)
	exec(`UPDATE posts SET deleted_at = $2 WHERE id = $1`, deleted, hoursAgo(1))
	exec(`INSERT INTO responses (idea_id, author_type, author_id, content, response_type)
		VALUES ($1, 'agent', $2, 'ideas response', 'support')`, fresh, ib)

	want := ideasSnapshot{
		Counts: map[string]int{"open": 1, "active": 2, "evolved": 1, "dormant": 1, "total": 5},
		Fresh: []map[string]any{
			{"id": fresh, "title": "fresh spark", "support": 12},
			{"id": quiet, "title": "quiet active", "support": 2},
		},
		Ready: []map[string]any{
			{"id": fresh, "title": "fresh spark", "support": 12, "validation_score": 75},
			{"id": older, "title": "older active", "support": 10, "validation_score": 100},
		},
		Top: []map[string]any{
			{"id": ia, "name": ia, "type": "agent", "ideas_count": 2, "realized_count": 1},
			{"id": ib, "name": ib, "type": "agent", "ideas_count": 2, "realized_count": 0},
			{"id": hu, "name": "Ideas Human", "type": "human", "ideas_count": 1, "realized_count": 0},
		},
		// spark = open 1, developing = active 2, mature = dormant 1, realized = evolved 1.
		Pipeline: map[string]any{
			"spark_to_developing": 66, "developing_to_mature": 33, "mature_to_realized": 50, "avg_days_to_realization": 4,
		},
		Realized: []map[string]any{{"id": evolved, "title": "realized idea", "evolved_into": target}},
	}

	served := NewCanonicalStatsRepository(pool)
	require.Equal(t, want, takeIdeasSnapshot(t, ctx, served), "before the cutover")

	_, err = MigrateContributions(ctx, pool)
	require.NoError(t, err)
	_, err = RemapLegacyRelations(ctx, pool)
	require.NoError(t, err)
	assert.Equal(t, want, takeIdeasSnapshot(t, ctx, served), "after the cutover")

	dropLegacy()
	assert.Equal(t, want, takeIdeasSnapshot(t, ctx, served), "the idea sidebar needs no legacy table")

	exec(`UPDATE posts SET type = 'post'`)
	assert.Equal(t, ideasSnapshot{
		Counts: map[string]int{"total": 0},
		Fresh:  []map[string]any{},
		Ready:  []map[string]any{},
		Top:    []map[string]any{},
		Pipeline: map[string]any{
			"spark_to_developing": 0, "developing_to_mature": 0, "mature_to_realized": 0, "avg_days_to_realization": 0,
		},
		Realized: []map[string]any{},
	}, takeIdeasSnapshot(t, ctx, served), "idea figures are a type-column subset: zero once it narrows")
}
