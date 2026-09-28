package db

import (
	"context"
	"fmt"
	"math"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/stretchr/testify/require"
)

// Task idx 76 step 3 (feature:briefing, platform sections): platform pulse, trending,
// rising posts, hardcore unsolved and recent victories read the canonical posts, replies
// and room outcomes. Every section counts or lists only public, published, approved
// posts; a contributor reply is a live human or agent reply. The approach success/failure
// workflow and the problem/idea-only fields (weight, evolved_into) are retired.

const cpBigLimit = 1000

var cpCounter atomic.Int64

// cpSeq returns a process-unique suffix for fixture identities created in a tight loop.
func cpSeq() string {
	return fmt.Sprintf("%d-%d", time.Now().UnixNano(), cpCounter.Add(1))
}

func cpIndex[T any](items []T, id func(T) string, want string) int {
	for i, it := range items {
		if id(it) == want {
			return i
		}
	}
	return -1
}

func cpSetPost(t *testing.T, pool *Pool, ctx context.Context, postID, set string, args ...any) {
	t.Helper()
	_, err := pool.Exec(ctx, `UPDATE posts SET `+set+` WHERE id = $1`, append([]any{postID}, args...)...)
	require.NoError(t, err)
}

func cpInsertRoom(t *testing.T, pool *Pool, ctx context.Context, age time.Duration) string {
	t.Helper()
	slug := fmt.Sprintf("cbpl-%d", time.Now().UnixNano())
	var id string
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO rooms (slug, display_name, created_at, updated_at, last_active_at)
		VALUES ($1, $2, $3, $3, $3) RETURNING id::text`,
		slug, "Canonical platform briefing room", time.Now().UTC().Add(-age)).Scan(&id))
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM rooms WHERE id = $1`, id) })
	return id
}

func TestCanonicalPlatformBriefing_Pulse(t *testing.T) {
	pool := setupTestDB(t)
	t.Cleanup(pool.Close)
	ctx := context.Background()
	repo := NewCanonicalPlatformBriefingRepository(pool)
	a := cbInsertAgent(t, pool, ctx, nil, nil)
	b := cbInsertAgent(t, pool, ctx, nil, nil)
	c := cbInsertAgent(t, pool, ctx, nil, nil)
	e := cbInsertAgent(t, pool, ctx, nil, nil)

	before, err := repo.GetPlatformPulse(ctx)
	require.NoError(t, err)

	byA := func(title string, s cbPostSeed) string {
		s.byID = a
		return cbInsertPost(t, pool, ctx, title, s)
	}
	openPost := byA("pulse open post", cbPostSeed{})
	openProblem := byA("pulse open problem", cbPostSeed{postType: "problem", status: "in_progress", age: 2 * time.Hour})
	openQuestion := byA("pulse open question", cbPostSeed{postType: "question", age: 30 * time.Hour})
	byA("pulse active idea", cbPostSeed{postType: "idea", status: "active", age: 30 * time.Hour})
	family := byA("pulse family post", cbPostSeed{visibility: "family"})
	byA("pulse draft post", cbPostSeed{publication: "draft"})
	byA("pulse pending post", cbPostSeed{moderation: "pending"})
	byA("pulse deleted post", cbPostSeed{deleted: true})
	byA("pulse closed post", cbPostSeed{status: "closed", age: time.Minute})
	byA("pulse solved problem", cbPostSeed{postType: "problem", status: "solved", age: time.Hour})
	byA("pulse solved long ago", cbPostSeed{postType: "problem", status: "solved", age: 8 * 24 * time.Hour})
	byA("pulse solved family", cbPostSeed{postType: "problem", status: "solved", visibility: "family", age: time.Hour})

	cbInsertReply(t, pool, ctx, openPost, "agent", b, 0, false)
	cbInsertReply(t, pool, ctx, openProblem, "agent", b, 0, false)
	cbInsertReply(t, pool, ctx, family, "agent", c, 0, false)
	cbInsertReply(t, pool, ctx, openPost, "agent", c, 0, true)
	cbInsertReply(t, pool, ctx, openPost, "system", fmt.Sprintf("cbpl-sys-%d", time.Now().UnixNano()), 0, false)
	cbInsertReply(t, pool, ctx, openQuestion, "agent", e, 8*24*time.Hour, false)

	after, err := repo.GetPlatformPulse(ctx)
	require.NoError(t, err)

	require.Equal(t, 4, after.OpenPosts-before.OpenPosts, "open public published approved posts of every type")
	require.Equal(t, 1, after.OpenProblems-before.OpenProblems, "problem-typed subset of the open posts")
	require.Equal(t, 1, after.OpenQuestions-before.OpenQuestions, "question-typed subset of the open posts")
	require.Equal(t, 1, after.ActiveIdeas-before.ActiveIdeas, "idea-typed subset of the open posts")
	require.Equal(t, 4, after.NewPostsLast24h-before.NewPostsLast24h, "public published approved posts created in 24h")
	require.Equal(t, 1, after.SolvedLast7d-before.SolvedLast7d, "public posts marked solved in the last 7 days")
	require.Equal(t, 2, after.ContributorsThisWeek-before.ContributorsThisWeek,
		"distinct authors of this week's public posts and live human/agent replies on them")
}

func TestCanonicalPlatformBriefing_Trending(t *testing.T) {
	pool := setupTestDB(t)
	t.Cleanup(pool.Close)
	ctx := context.Background()
	repo := NewCanonicalPlatformBriefingRepository(pool)
	me := cbInsertAgent(t, pool, ctx, nil, nil)
	other := cbInsertAgent(t, pool, ctx, nil, nil)
	byOther := func(title string, s cbPostSeed) string {
		s.byID = other
		return cbInsertPost(t, pool, ctx, title, s)
	}
	vote := func(postID string, n int, confirmed bool, ago time.Duration) {
		for i := 0; i < n; i++ {
			cbInsertVote(t, pool, ctx, "post", postID, "cbpl-voter-"+cpSeq(), "up", confirmed, ago)
		}
	}

	hot := byOther("trending hot", cbPostSeed{age: 3 * time.Hour})
	vote(hot, 2, true, time.Hour)
	_, err := pool.Exec(ctx, `INSERT INTO post_views (post_id, viewer_type, viewer_id) VALUES ($1, 'anonymous', 'cbpl-viewer')`, hot)
	require.NoError(t, err)
	cpSetPost(t, pool, ctx, hot, `upvotes = 7, downvotes = 2, view_count = 11`)
	warm := byOther("trending warm", cbPostSeed{age: 2 * time.Hour})
	vote(warm, 1, true, time.Hour)
	cold := byOther("trending cold", cbPostSeed{})
	vote(cold, 1, false, time.Hour)
	vote(cold, 1, true, 8*24*time.Hour)

	mine := cbInsertPost(t, pool, ctx, "trending mine", cbPostSeed{byID: me})
	vote(mine, 5, true, time.Hour)
	excluded := map[string]string{"my own post": mine}
	for name, s := range map[string]cbPostSeed{
		"family": {visibility: "family"}, "closed": {status: "closed"}, "draft": {publication: "draft"},
		"pending": {moderation: "pending"}, "deleted": {deleted: true},
	} {
		id := byOther("trending "+name, s)
		vote(id, 5, true, time.Hour)
		excluded[name] = id
	}

	got, err := repo.GetTrendingNow(ctx, me, cpBigLimit)
	require.NoError(t, err)
	idOf := func(p models.TrendingPost) string { return p.ID }
	iHot, iWarm, iCold := cpIndex(got, idOf, hot), cpIndex(got, idOf, warm), cpIndex(got, idOf, cold)
	require.True(t, iHot >= 0 && iWarm > iHot && iCold > iWarm,
		"ranked by confirmed post votes + views in 7 days: hot=%d warm=%d cold=%d", iHot, iWarm, iCold)
	for name, id := range excluded {
		require.Equal(t, -1, cpIndex(got, idOf, id), "%s must not trend", name)
	}
	h := got[iHot]
	require.Equal(t, "post", h.Type)
	require.Equal(t, 5, h.VoteScore)
	require.Equal(t, 11, h.ViewCount)
	require.Equal(t, "Canonical Briefing "+other[5:], h.AuthorName)
	require.Equal(t, "agent", h.AuthorType)
	require.Equal(t, 3, h.AgeHours)
	require.Equal(t, []string{"cbrf"}, h.Tags)

	limited, err := repo.GetTrendingNow(ctx, me, 1)
	require.NoError(t, err)
	require.Len(t, limited, 1)
}

func TestCanonicalPlatformBriefing_RisingPosts(t *testing.T) {
	pool := setupTestDB(t)
	t.Cleanup(pool.Close)
	ctx := context.Background()
	repo := NewCanonicalPlatformBriefingRepository(pool)
	other := cbInsertAgent(t, pool, ctx, nil, nil)
	byOther := func(title string, s cbPostSeed) string {
		s.byID = other
		return cbInsertPost(t, pool, ctx, title, s)
	}
	replies := func(postID string, n int) {
		for i := 0; i < n; i++ {
			cbInsertReply(t, pool, ctx, postID, "agent", fmt.Sprintf("cbpl-replier-%d", i), time.Hour, false)
		}
	}

	busy := byOther("rising busy post", cbPostSeed{age: 5 * time.Hour})
	replies(busy, 3)
	cbInsertReply(t, pool, ctx, busy, "system", "moderation", time.Hour, false)
	cbInsertReply(t, pool, ctx, busy, "human", "cbpl-gone", time.Hour, true)
	idea := byOther("rising idea", cbPostSeed{postType: "idea", age: 4 * time.Hour})
	replies(idea, 1)
	cpSetPost(t, pool, ctx, idea, `evolved_into = ARRAY[$2::uuid]`, busy)
	upvoted := byOther("rising upvoted only", cbPostSeed{})
	cpSetPost(t, pool, ctx, upvoted, `upvotes = 5`)

	quiet := byOther("rising nothing", cbPostSeed{})
	systemOnly := byOther("rising system only", cbPostSeed{})
	cbInsertReply(t, pool, ctx, systemOnly, "system", "moderation", time.Hour, false)
	excluded := map[string]string{"no engagement": quiet, "system verdict only": systemOnly}
	for name, s := range map[string]cbPostSeed{
		"closed": {status: "closed"}, "dormant": {postType: "idea", status: "dormant"},
		"family": {visibility: "family"}, "draft": {publication: "draft"}, "pending": {moderation: "pending"},
	} {
		id := byOther("rising "+name, s)
		replies(id, 4)
		excluded[name] = id
	}

	got, err := repo.GetRisingIdeas(ctx, cpBigLimit)
	require.NoError(t, err)
	idOf := func(r models.RisingIdea) string { return r.ID }
	iBusy, iIdea, iUp := cpIndex(got, idOf, busy), cpIndex(got, idOf, idea), cpIndex(got, idOf, upvoted)
	require.True(t, iBusy >= 0 && iIdea > iBusy && iUp > iIdea,
		"any post type, ranked by contributor replies then upvotes: busy=%d idea=%d upvoted=%d", iBusy, iIdea, iUp)
	for name, id := range excluded {
		require.Equal(t, -1, cpIndex(got, idOf, id), "%s must not be rising", name)
	}
	require.Equal(t, 3, got[iBusy].ResponseCount, "live human/agent replies only")
	require.Equal(t, 5, got[iBusy].AgeHours)
	require.Equal(t, 1, got[iIdea].ResponseCount)
	require.Equal(t, 0, got[iIdea].EvolvedCount, "idea evolution is retired")
	require.Equal(t, 0, got[iUp].ResponseCount)
	require.Equal(t, 5, got[iUp].Upvotes)

	limited, err := repo.GetRisingIdeas(ctx, 1)
	require.NoError(t, err)
	require.Len(t, limited, 1)
}

func TestCanonicalPlatformBriefing_HardcoreUnsolved(t *testing.T) {
	pool := setupTestDB(t)
	t.Cleanup(pool.Close)
	ctx := context.Background()
	repo := NewCanonicalPlatformBriefingRepository(pool)
	other := cbInsertAgent(t, pool, ctx, nil, nil)
	byOther := func(title string, s cbPostSeed) string {
		s.byID = other
		return cbInsertPost(t, pool, ctx, title, s)
	}
	replies := func(postID, authorType string, n int) {
		for i := 0; i < n; i++ {
			cbInsertReply(t, pool, ctx, postID, authorType, fmt.Sprintf("cbpl-hard-%d", i), time.Hour, false)
		}
	}

	contested := byOther("hardcore contested", cbPostSeed{age: 2 * time.Hour})
	replies(contested, "agent", 2)
	replies(contested, "human", 1)
	old := byOther("hardcore old and liked", cbPostSeed{age: 40 * 24 * time.Hour})
	cpSetPost(t, pool, ctx, old, `upvotes = 3, downvotes = 1`)

	twoReplies := byOther("hardcore two replies", cbPostSeed{})
	replies(twoReplies, "agent", 2)
	replies(twoReplies, "system", 3)
	oldUnliked := byOther("hardcore old unliked", cbPostSeed{age: 40 * 24 * time.Hour})
	heavy := byOther("hardcore heavy problem", cbPostSeed{postType: "problem"})
	cpSetPost(t, pool, ctx, heavy, `weight = 5`)
	excluded := map[string]string{"two contributor replies": twoReplies, "old without score": oldUnliked,
		"weight is retired": heavy}
	for name, s := range map[string]cbPostSeed{
		"solved": {postType: "problem", status: "solved"}, "answered": {postType: "question", status: "answered"},
		"closed": {status: "closed"}, "evolved": {postType: "idea", status: "evolved"},
		"family": {visibility: "family"}, "draft": {publication: "draft"}, "pending": {moderation: "pending"},
	} {
		id := byOther("hardcore "+name, s)
		replies(id, "agent", 5)
		excluded[name] = id
	}

	got, err := repo.GetHardcoreUnsolved(ctx, cpBigLimit)
	require.NoError(t, err)
	idOf := func(h models.HardcoreUnsolved) string { return h.ID }
	iOld, iContested := cpIndex(got, idOf, old), cpIndex(got, idOf, contested)
	require.True(t, iOld >= 0 && iContested > iOld, "ordered by difficulty: old=%d contested=%d", iOld, iContested)
	for name, id := range excluded {
		require.Equal(t, -1, cpIndex(got, idOf, id), "%s must not be hardcore", name)
	}
	c := got[iContested]
	require.Equal(t, 3, c.TotalApproaches, "live human/agent replies")
	require.Equal(t, 0, c.FailedCount, "the approach failure workflow is retired")
	require.Equal(t, 1, c.Weight, "the problem-only weight is retired")
	require.Equal(t, 0, c.AgeDays)
	require.InDelta(t, 4*math.Log(2+2.0/24), c.DifficultyScore, 0.01)
	o := got[iOld]
	require.Equal(t, 0, o.TotalApproaches)
	require.Equal(t, 40, o.AgeDays)
	require.InDelta(t, math.Log(42)*2, o.DifficultyScore, 0.01)
	require.Equal(t, []string{"cbrf"}, o.Tags)

	limited, err := repo.GetHardcoreUnsolved(ctx, 1)
	require.NoError(t, err)
	require.Len(t, limited, 1)
}

func TestCanonicalPlatformBriefing_RecentVictoriesAreRoomOutcomes(t *testing.T) {
	pool := setupTestDB(t)
	t.Cleanup(pool.Close)
	ctx := context.Background()
	repo := NewCanonicalPlatformBriefingRepository(pool)
	solver := cbInsertAgent(t, pool, ctx, nil, nil)
	room := cpInsertRoom(t, pool, ctx, 3*24*time.Hour)
	outcome := func(title, roomID string, s cbPostSeed) string {
		if s.byID == "" {
			s.byID = solver
		}
		id := cbInsertPost(t, pool, ctx, title, s)
		cpSetPost(t, pool, ctx, id, `source_room_id = $2::uuid`, roomID)
		return id
	}

	agentWin := outcome("victory by an agent", room, cbPostSeed{age: time.Hour, tags: []string{"cbpl", "win"}})
	cbInsertReply(t, pool, ctx, agentWin, "agent", "cbpl-r1", time.Minute, false)
	cbInsertReply(t, pool, ctx, agentWin, "human", "cbpl-r2", time.Minute, false)
	cbInsertReply(t, pool, ctx, agentWin, "system", "moderation", time.Minute, false)
	ghost := fmt.Sprintf("cbpl-ghost-%d", time.Now().UnixNano())
	humanWin := outcome("victory by an unresolved human", room, cbPostSeed{byType: "human", byID: ghost, age: 10 * time.Minute})
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM posts WHERE posted_by_id = $1`, ghost) })
	orphanWin := outcome("victory from a vanished room", "00000000-0000-4000-8000-00000000cb01", cbPostSeed{age: 2 * time.Hour})

	excluded := map[string]string{
		"family":      outcome("victory family", room, cbPostSeed{visibility: "family"}),
		"draft":       outcome("victory draft", room, cbPostSeed{publication: "draft"}),
		"pending":     outcome("victory pending", room, cbPostSeed{moderation: "pending"}),
		"deleted":     outcome("victory deleted", room, cbPostSeed{deleted: true}),
		"15 days old": outcome("victory too old", room, cbPostSeed{age: 15 * 24 * time.Hour}),
		"not a room outcome": cbInsertPost(t, pool, ctx, "solved problem without a room",
			cbPostSeed{postType: "problem", status: "solved", byID: solver}),
	}

	got, err := repo.GetRecentVictories(ctx, cpBigLimit)
	require.NoError(t, err)
	idOf := func(v models.RecentVictory) string { return v.ID }
	iHuman, iAgent, iOrphan := cpIndex(got, idOf, humanWin), cpIndex(got, idOf, agentWin), cpIndex(got, idOf, orphanWin)
	require.True(t, iHuman >= 0 && iAgent > iHuman && iOrphan > iAgent,
		"newest outcome first: human=%d agent=%d orphan=%d", iHuman, iAgent, iOrphan)
	for name, id := range excluded {
		require.Equal(t, -1, cpIndex(got, idOf, id), "%s must not be a victory", name)
	}

	a := got[iAgent]
	require.Equal(t, "victory by an agent", a.Title)
	require.Equal(t, "Canonical Briefing "+solver[5:], a.SolverName)
	require.Equal(t, "agent", a.SolverType)
	require.Equal(t, solver, a.SolverID)
	require.Equal(t, 2, a.TotalApproaches, "live human/agent replies on the outcome post")
	require.Equal(t, 2, a.DaysToSolve, "room opened 3 days ago, outcome saved 1 hour ago")
	var created time.Time
	require.NoError(t, pool.QueryRow(ctx, `SELECT created_at FROM posts WHERE id = $1`, agentWin).Scan(&created))
	require.Equal(t, created.UTC().Format("2006-01-02T15:04:05Z"), a.SolvedAt)
	require.Equal(t, []string{"cbpl", "win"}, a.Tags)

	h := got[iHuman]
	require.Equal(t, ghost, h.SolverName, "unresolved author falls back to its id")
	require.Equal(t, "human", h.SolverType)
	require.Equal(t, 0, got[iOrphan].DaysToSolve, "unknown room start")

	limited, err := repo.GetRecentVictories(ctx, 1)
	require.NoError(t, err)
	require.Len(t, limited, 1)
}
