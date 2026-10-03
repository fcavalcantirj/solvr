package db

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Task idx 76 step 3: the public statistics that read the legacy contribution tables (GET
// /v1/stats, /v1/stats/trending, /v1/stats/problems, /v1/stats/questions and the overview's
// community totals) are served by CanonicalStatsRepository. The legacy StatsRepository methods
// that read answers, approaches and responses were deleted with the legacy tables (idx 68).
func TestLegacyStats_ServedByTheCanonicalRepository(t *testing.T) {
	assert.ElementsMatch(t, []string{"internal/api/router.go", "internal/api/router_homepage.go"},
		productionSourcesContaining(t, "db.NewCanonicalStatsRepository("),
		"the stats routes and the overview serve the canonical statistics")
	assert.ElementsMatch(t, []string{"internal/db/stats.go", "internal/db/stats_canonical.go"},
		productionSourcesContaining(t, "NewStatsRepository("),
		"only its definition and the canonical wrapper build the legacy stats repository")

	src, err := ScanLegacySourceDependencies(backendRoot(t))
	require.NoError(t, err)
	for _, dep := range src {
		assert.NotEqual(t, "code:internal/db/stats_canonical.go", dep.Key, "the canonical statistics name no legacy table or type")
	}
	assertLegacyDependencyGone(t, "code:internal/db/stats_questions.go") // deleted with the legacy tables (idx 68)
	s, ok := LegacyDependencyDispositions["code:internal/db/stats.go"]
	require.True(t, ok)
	assert.Equal(t, LegacyActionRefactor, s.Action)
	assert.False(t, s.Done, "its solved and answered counters and the idea sidebar still name legacy types")
}

// statsReader is what the served stats routes read that the legacy tables fed.
type statsReader interface {
	GetAllStats(ctx context.Context) (*AllStatsResult, error)
	GetTotalContributionsCount(ctx context.Context) (int, error)
	GetTrendingPosts(ctx context.Context, limit int) ([]any, error)
	GetProblemsStats(ctx context.Context) (map[string]any, error)
	GetRecentlySolvedProblems(ctx context.Context, limit int) ([]map[string]any, error)
	GetTopProblemSolvers(ctx context.Context, limit int) ([]map[string]any, error)
	GetQuestionsStats(ctx context.Context) (map[string]any, error)
	GetRecentlyAnsweredQuestions(ctx context.Context, limit int) ([]map[string]any, error)
	GetTopAnswerers(ctx context.Context, limit int) ([]map[string]any, error)
}

type trendingStat struct {
	Type      string
	VoteScore int
	Responses int
}

type answererStat struct {
	Name       string
	Count      int
	AcceptRate float64
}

type statsSnapshot struct {
	All              AllStatsResult
	Contributions    int
	Trending         map[string]trendingStat // post id -> figures
	Problems         map[string]any
	RecentlySolved   []map[string]any
	TopSolvers       map[string]int // author id -> solved problems
	Questions        map[string]any
	RecentlyAnswered []map[string]any
	TopAnswerers     map[string]answererStat // author id -> figures
}

func roundStat(v any) any {
	if f, ok := v.(float64); ok {
		return math.Round(f*1000) / 1000
	}
	return v
}

func roundedStats(m map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = roundStat(v)
	}
	return out
}

func takeStatsSnapshot(t *testing.T, ctx context.Context, r statsReader) statsSnapshot {
	t.Helper()
	var s statsSnapshot
	all, err := r.GetAllStats(ctx)
	require.NoError(t, err)
	s.All = *all
	s.Contributions, err = r.GetTotalContributionsCount(ctx)
	require.NoError(t, err)

	trending, err := r.GetTrendingPosts(ctx, 100)
	require.NoError(t, err)
	s.Trending = map[string]trendingStat{}
	for _, item := range trending {
		m := item.(map[string]any)
		s.Trending[m["id"].(string)] = trendingStat{Type: m["type"].(string), VoteScore: m["vote_score"].(int), Responses: m["response_count"].(int)}
	}

	s.Problems, err = r.GetProblemsStats(ctx)
	require.NoError(t, err)
	s.Problems = roundedStats(s.Problems)
	solved, err := r.GetRecentlySolvedProblems(ctx, 100)
	require.NoError(t, err)
	for _, m := range solved {
		s.RecentlySolved = append(s.RecentlySolved, roundedStats(m))
	}
	solvers, err := r.GetTopProblemSolvers(ctx, 100)
	require.NoError(t, err)
	s.TopSolvers = map[string]int{}
	for _, m := range solvers {
		s.TopSolvers[m["author_id"].(string)] = m["solved_count"].(int)
	}

	s.Questions, err = r.GetQuestionsStats(ctx)
	require.NoError(t, err)
	s.Questions = roundedStats(s.Questions)
	answered, err := r.GetRecentlyAnsweredQuestions(ctx, 100)
	require.NoError(t, err)
	for _, m := range answered {
		s.RecentlyAnswered = append(s.RecentlyAnswered, roundedStats(m))
	}
	answerers, err := r.GetTopAnswerers(ctx, 100)
	require.NoError(t, err)
	s.TopAnswerers = map[string]answererStat{}
	for _, m := range answerers {
		s.TopAnswerers[m["author_id"].(string)] = answererStat{
			Name: m["display_name"].(string), Count: m["answer_count"].(int), AcceptRate: roundStat(m["accept_rate"]).(float64),
		}
	}
	return s
}

// Task idx 76 steps 3 and 5: in a database holding only this fixture, the legacy statistics
// before the contribution cutover and the canonical statistics after it agree wherever the
// canonical model keeps the concept, and differ exactly where it deliberately does not: every
// live human or agent reply on a live public post is a contribution (comments and progress notes
// included, system verdicts and replies on deleted posts not); the approach status workflow is
// retired (no active approaches); a question's answers are its top-level replies. The canonical
// figures then survive dropping the legacy tables unchanged, and native replies move them.
func TestCanonicalStats_KeepsLegacyFiguresAcrossTheCutover(t *testing.T) {
	pool, dropLegacy := newMigratedScratchDatabase(t)
	ctx := context.Background()
	t0 := time.Now().UTC().Truncate(time.Second)
	hoursAgo := func(h float64) time.Time { return t0.Add(-time.Duration(h * float64(time.Hour))) }
	exec := func(sql string, args ...any) {
		t.Helper()
		_, err := pool.Exec(ctx, sql, args...)
		require.NoError(t, err, sql)
	}

	sa, sb, sc := "agent_stats_a", "agent_stats_b", "agent_stats_c"
	for _, id := range []string{sa, sb, sc} {
		insertRemapAgent(t, pool, ctx, id)
	}
	human, err := NewUserRepository(pool).Create(ctx, &models.User{
		Username: "statshuman", DisplayName: "Stats Human", Email: "statshuman@example.com",
		AuthProvider: models.AuthProviderGitHub, AuthProviderID: "gh_statshuman", Role: models.UserRoleUser,
	})
	require.NoError(t, err)
	h := human.ID

	post := func(postType, status, authorType, author string, created, updated time.Time) string {
		id := insertTestPostWithAuthor(t, pool, ctx, postType, "stats "+postType+" "+status, "body", []string{"stats"}, status, authorType, author)
		exec(`UPDATE posts SET created_at = $2, updated_at = $3 WHERE id = $1`, id, created, updated)
		return id
	}
	p1 := post("problem", "solved", "agent", sb, hoursAgo(120), t0)
	p2 := post("problem", "solved", "agent", sb, hoursAgo(72), hoursAgo(1))
	p3 := post("problem", "solved", "human", h, hoursAgo(24), hoursAgo(2))
	p4 := post("problem", "open", "agent", sb, hoursAgo(20), hoursAgo(20))
	q1 := post("question", "open", "human", h, hoursAgo(10), hoursAgo(3))
	q2 := post("question", "open", "agent", sa, hoursAgo(6), hoursAgo(6))
	i1 := post("idea", "open", "agent", sa, hoursAgo(30), hoursAgo(30))
	famP := post("problem", "solved", "agent", sa, hoursAgo(40), hoursAgo(4))
	famQ := post("question", "open", "agent", sa, hoursAgo(40), hoursAgo(4))
	exec(`UPDATE posts SET visibility = 'family' WHERE id IN ($1, $2)`, famP, famQ)
	delP := post("problem", "solved", "agent", sb, hoursAgo(50), hoursAgo(5))
	exec(`UPDATE posts SET deleted_at = $2 WHERE id = $1`, delP, hoursAgo(5))

	approach := func(problem, author, status string, deleted bool, updated time.Time) string {
		id := insertApproach(t, pool, ctx, problem, author, status, deleted)
		exec(`UPDATE approaches SET updated_at = $2 WHERE id = $1`, id, updated)
		return id
	}
	approach(p1, sa, "succeeded", false, hoursAgo(2))
	succeededC := approach(p1, sc, "succeeded", false, hoursAgo(1)) // the latest success: p1's solver
	approach(p1, sb, "failed", false, hoursAgo(3))
	approach(p1, sb, "working", false, hoursAgo(3))
	approach(p1, sb, "starting", true, hoursAgo(3))
	approach(p2, sa, "succeeded", false, hoursAgo(1))
	approach(p4, sb, "starting", false, hoursAgo(19))
	approach(famP, sa, "succeeded", false, hoursAgo(4))
	approach(delP, sb, "succeeded", false, hoursAgo(5))
	exec(`INSERT INTO progress_notes (approach_id, content) VALUES ($1, 'stats progress note')`, succeededC)
	exec(`INSERT INTO replies (post_id, author_type, author_id, body) VALUES ($1, 'system', 'solvr-moderation', 'verdict')`, p1)

	answer := func(question, authorType, author string, accepted, deleted bool, created time.Time) string {
		var deletedAt any
		if deleted {
			deletedAt = created
		}
		var id string
		require.NoError(t, pool.QueryRow(ctx, `
			INSERT INTO answers (question_id, author_type, author_id, content, is_accepted, created_at, deleted_at)
			VALUES ($1, $2, $3, 'stats answer', $4, $5, $6) RETURNING id::text`,
			question, authorType, author, accepted, created, deletedAt).Scan(&id))
		return id
	}
	acceptedH := answer(q1, "human", h, true, false, hoursAgo(6)) // 4h after q1
	answer(q1, "agent", sa, false, false, hoursAgo(8))            // q1's first answer, 2h after it
	answer(q1, "agent", sb, false, true, hoursAgo(9.5))
	exec(`UPDATE posts SET accepted_answer_id = $2 WHERE id = $1`, q1, acceptedH)
	famAnswer := answer(famQ, "agent", sa, true, false, hoursAgo(39))
	exec(`UPDATE posts SET accepted_answer_id = $2 WHERE id = $1`, famQ, famAnswer)
	comment := func(targetType, target, author string, created time.Time) {
		id := insertComment(t, pool, ctx, targetType, target, author, "stats comment")
		exec(`UPDATE comments SET created_at = $2 WHERE id = $1`, id, created)
	}
	comment("post", q1, sc, hoursAgo(9))          // q1's first reply, 1h after it
	comment("answer", acceptedH, sb, hoursAgo(5)) // a child reply once migrated
	exec(`INSERT INTO responses (idea_id, author_type, author_id, content, response_type)
		VALUES ($1, 'agent', $2, 'stats response', 'support')`, i1, sa)

	canonical := NewCanonicalStatsRepository(pool)
	// The legacy StatsRepository (deleted, idx 68) counted live answers (2), live approaches (7,
	// the one on the deleted post included) and responses (1) on public posts as contributions;
	// comments and progress notes never counted. The other platform figures come from posts, which
	// the canonical repository reads the same way before the cutover.
	pre := takeStatsSnapshot(t, ctx, canonical)
	require.Equal(t, 3, pre.All.ProblemsSolved)
	require.Equal(t, 1, pre.All.QuestionsAnswered)
	wantSolved := []map[string]any{
		{"id": p1, "title": "stats problem solved", "solver_name": sc, "solver_type": "agent", "time_to_solve_days": 5},
		{"id": p2, "title": "stats problem solved", "solver_name": sa, "solver_type": "agent", "time_to_solve_days": 2},
		{"id": p3, "title": "stats problem solved", "solver_name": "unknown", "solver_type": "unknown", "time_to_solve_days": 0},
	}
	wantAnswered := []map[string]any{
		{"id": q1, "title": "stats question open", "answerer_name": "Stats Human", "answerer_type": "human", "time_to_answer_hours": 4.0},
	}

	_, err = MigrateContributions(ctx, pool)
	require.NoError(t, err)
	_, err = RemapLegacyRelations(ctx, pool)
	require.NoError(t, err)

	after := takeStatsSnapshot(t, ctx, canonical)
	// Canonical contributions: p1's 4 live approaches + the progress note, p2 1, p4 1, q1's 2 live
	// answers + both comments, i1's response. Not the system verdict, not the deleted rows, not
	// the family posts, not the approach on the deleted post.
	wantAll := pre.All
	wantAll.TotalContributions = 12
	assert.Equal(t, wantAll, after.All, "only the contribution total changes")
	assert.Equal(t, 12, after.Contributions)
	wantTrending := map[string]trendingStat{
		p1: {"problem", 0, 5}, p2: {"problem", 0, 1}, p3: {"problem", 0, 0}, p4: {"problem", 0, 1},
		q1: {"question", 0, 4}, q2: {"question", 0, 0}, i1: {"idea", 0, 1},
	}
	assert.Equal(t, wantTrending, after.Trending, "every live contributor reply is a response")
	assert.Equal(t, map[string]any{
		"total_problems": 4, "solved_count": 3, "active_approaches": 0, "avg_solve_time_days": 3,
	}, after.Problems, "the approach status workflow is retired")
	assert.Equal(t, wantSolved, after.RecentlySolved, "the solver is kept across the cutover")
	assert.Equal(t, map[string]int{sa: 2, sc: 1}, after.TopSolvers, "solvers are kept across the cutover")
	assert.Equal(t, map[string]any{
		"total_questions": 2, "answered_count": 1, "response_rate": 50.0, "avg_response_time_hours": 1.0,
	}, after.Questions, "the first reply to q1 is the comment, 1h after it")
	assert.Equal(t, wantAnswered, after.RecentlyAnswered, "the accepted reply keeps its author and time")
	wantAnswerers := map[string]answererStat{h: {"Stats Human", 1, 100}, sa: {sa, 1, 0}, sc: {sc, 1, 0}}
	assert.Equal(t, wantAnswerers, after.TopAnswerers, "the comment on q1 is a top-level reply; the one on the answer is not")

	dropLegacy()
	assert.Equal(t, after, takeStatsSnapshot(t, ctx, canonical), "the canonical statistics need no legacy table")

	var native string
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO replies (post_id, author_type, author_id, body, created_at, updated_at)
		VALUES ($1, 'agent', $2, 'native answer', $3, $3) RETURNING id::text`, q2, sb, hoursAgo(3)).Scan(&native))
	exec(`INSERT INTO replies (post_id, parent_reply_id, author_type, author_id, body) VALUES ($1, $2, 'agent', $3, 'native child')`, q2, native, sa)
	exec(`INSERT INTO replies (post_id, author_type, author_id, body) VALUES ($1, 'system', 'solvr-moderation', 'verdict')`, q2)
	exec(`INSERT INTO replies (post_id, author_type, author_id, body, deleted_at) VALUES ($1, 'agent', $2, 'gone', NOW())`, q2, sc)

	live := takeStatsSnapshot(t, ctx, canonical)
	assert.Equal(t, 14, live.All.TotalContributions, "the native reply and its child count")
	assert.Equal(t, 14, live.Contributions)
	wantTrending[q2] = trendingStat{"question", 0, 2}
	assert.Equal(t, wantTrending, live.Trending)
	assert.InDelta(t, 2.0, live.Questions["avg_response_time_hours"], 0.001, "q1 after 1h, q2 after 3h")
	wantAnswerers[sb] = answererStat{sb, 1, 0}
	assert.Equal(t, wantAnswerers, live.TopAnswerers, "the native top-level reply is an answer; its child is not")
}
