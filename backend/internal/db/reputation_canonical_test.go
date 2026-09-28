package db

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/fcavalcantirj/solvr/internal/reputation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Task idx 76 steps 3-4 (feature:reputation): the router serves agent and user reputation
// (profiles, /v1/me, the agents and users lists) from the canonical repositories, which score
// it like the served leaderboard. The legacy formulas (the inline agent SQL and
// reputation.BuildReputationSQL) stay unwired until the legacy tables go.
func TestLegacyReputation_ServedByTheCanonicalRepositories(t *testing.T) {
	assert.Contains(t, productionSourcesContaining(t, "NewCanonicalReputationAgentRepository("), "internal/api/router.go")
	assert.Contains(t, productionSourcesContaining(t, "NewCanonicalReputationUserRepository("), "internal/api/router.go")
	assert.ElementsMatch(t, []string{"internal/db/leaderboard.go", "internal/db/users.go"},
		productionSourcesContaining(t, "reputation.BuildReputationSQL("),
		"only the unwired legacy leaderboard and legacy users list build the legacy formula")

	d := LegacyDependencyDispositions["feature:reputation"]
	assert.Equal(t, LegacyActionRefactor, d.Action)
	assert.True(t, d.Done, "feature:reputation is refactored and verified")
	b, ok := LegacyDependencyDispositions["code:internal/reputation/sql_builder.go"]
	require.True(t, ok)
	assert.Equal(t, LegacyActionRetire, b.Action)
	assert.False(t, b.Done, "sql_builder reads the legacy tables until they are dropped")

	assert.Equal(t, fmt.Sprintf("CASE WHEN v.direction = 'up' THEN %d ELSE %d END",
		reputation.PointsUpvoteReceived, reputation.PointsDownvoteReceived), canonicalVotePoints,
		"live votes score what the reputation constants say")
}

// Task idx 76 steps 3-4 (feature:reputation): after the contribution cutover, an agent's and a
// user's reputation on their profile (GetAgentStats, GetUserStats) and in the agents and users
// lists equals what the served leaderboard shows, and equals what the legacy leaderboard showed
// before the cutover: earned reputation is kept as history, migrated rows award nothing new.
// Live votes on posts and replies then move every surface by the same points. The agents and
// users lists sort by that reputation. Only Reputation changes in the stats: the other counts
// are still the legacy repository's.
func TestCanonicalReputation_ProfilesAndListsAgreeWithTheServedLeaderboard(t *testing.T) {
	pool := setupTestDB(t)
	t.Cleanup(pool.Close) // registered first, so it runs after the fixture cleanups
	ctx := context.Background()

	sfx := time.Now().Format("150405.000000")
	holder, newcomer, rich := "agent_rph_"+sfx, "agent_rpn_"+sfx, "agent_rpr_"+sfx
	voter := "agent_rpv_" + sfx
	for _, id := range []string{holder, newcomer, rich} {
		insertRemapAgent(t, pool, ctx, id)
	}
	legacyAgents := NewAgentRepository(pool)
	require.NoError(t, legacyAgents.AddReputation(ctx, holder, 7))
	n := time.Now().UnixNano() % 1000000000
	user, err := NewUserRepository(pool).Create(ctx, &models.User{
		Username: fmt.Sprintf("rpc%d", n), DisplayName: "Reputation Cutover User",
		Email: fmt.Sprintf("rpc%d@example.com", n), AuthProvider: models.AuthProviderGitHub,
		AuthProviderID: fmt.Sprintf("gh_rpc%d", n), Role: models.UserRoleUser,
	})
	require.NoError(t, err)
	owners := []string{holder, newcomer, rich, user.ID}
	t.Cleanup(func() {
		c := context.Background()
		pool.Exec(c, `DELETE FROM reputation_history WHERE owner_id = ANY($1)`, owners) //nolint:errcheck
		pool.Exec(c, `DELETE FROM votes WHERE voter_id LIKE $1`, voter+"%")             //nolint:errcheck
		pool.Exec(c, `DELETE FROM comments WHERE author_id = ANY($1)`, owners)          //nolint:errcheck
		pool.Exec(c, `DELETE FROM responses WHERE author_id = ANY($1)`, owners)         //nolint:errcheck
		pool.Exec(c, `DELETE FROM approaches WHERE author_id = ANY($1)`, owners)        //nolint:errcheck
		pool.Exec(c, `DELETE FROM answers WHERE author_id = ANY($1)`, owners)           //nolint:errcheck
		pool.Exec(c, `DELETE FROM replies WHERE author_id = ANY($1)`, owners)           //nolint:errcheck
		pool.Exec(c, `DELETE FROM posts WHERE posted_by_id = ANY($1)`, owners)          //nolint:errcheck
		pool.Exec(c, `DELETE FROM agents WHERE id = ANY($1)`, owners[:3])               //nolint:errcheck
		pool.Exec(c, `DELETE FROM users WHERE id = $1`, user.ID)                        //nolint:errcheck
	})

	post := func(postType, status, authorType, author string) string {
		return insertTestPostWithAuthor(t, pool, ctx, postType, "rpc "+postType+" "+sfx, "body", []string{"rpc"}, status, authorType, author)
	}
	solved := post("problem", "solved", "agent", holder)
	idea := post("idea", "open", "agent", holder)
	question := post("question", "open", "human", user.ID)
	userSolved := post("problem", "solved", "human", user.ID)
	answer := func(authorType, author string, accepted bool) string {
		var id string
		require.NoError(t, pool.QueryRow(ctx, `
			INSERT INTO answers (question_id, author_type, author_id, content, is_accepted)
			VALUES ($1, $2, $3, 'rpc answer', $4) RETURNING id::text`, question, authorType, author, accepted).Scan(&id))
		return id
	}
	accepted := answer("agent", holder, true)
	userAnswer := answer("human", user.ID, false)
	_, err = pool.Exec(ctx, `UPDATE posts SET accepted_answer_id = $1 WHERE id = $2`, accepted, question)
	require.NoError(t, err)
	var response string
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO responses (idea_id, author_type, author_id, content, response_type)
		VALUES ($1, 'agent', $2, 'rpc response', 'support') RETURNING id::text`, idea, holder).Scan(&response))
	insertComment(t, pool, ctx, "post", solved, holder, "rpc agent comment")
	_, err = pool.Exec(ctx, `INSERT INTO comments (target_type, target_id, author_type, author_id, content)
		VALUES ('post', $1, 'human', $2, 'rpc human comment')`, userSolved, user.ID)
	require.NoError(t, err)
	approach := insertApproach(t, pool, ctx, solved, newcomer, "succeeded", false)
	votes := 0
	vote := func(targetType, targetID, direction string) {
		votes++
		_, err := pool.Exec(ctx, `
			INSERT INTO votes (target_type, target_id, voter_type, voter_id, direction, confirmed)
			VALUES ($1, $2, 'agent', $3, $4, true)`, targetType, targetID, fmt.Sprintf("%s_%d", voter, votes), direction)
		require.NoError(t, err)
	}
	vote("post", solved, "up")
	vote("answer", accepted, "up")
	vote("response", response, "down")
	vote("answer", userAnswer, "up")
	vote("approach", approach, "up")

	served := NewCanonicalLeaderboardRepository(pool)
	boardOf := func(fetch leaderboardFetch) map[string]int {
		out := map[string]int{}
		for id, e := range leaderboardEntriesFor(t, fetch, models.LeaderboardOptions{Type: "all", Timeframe: "all_time"}, owners...) {
			out[id] = e.Reputation
		}
		return out
	}
	legacyBoard := NewLeaderboardRepository(pool)
	before := boardOf(func(o models.LeaderboardOptions) ([]models.LeaderboardEntry, int, error) {
		return legacyBoard.GetLeaderboard(ctx, o)
	})
	// Scored by the legacy leaderboard: holder = 7 (bonus) + 125 (solved) + 15 (idea) + 60
	// (accepted answer) + 5 (response) + 2 (comment) + votes (+2 +2 -1) = 217; the user = 125
	// + 10 (answer) + 2 (comment) + 2 (vote) = 139; the newcomer's approach and the vote on it
	// score nothing.
	require.Equal(t, map[string]int{holder: 217, user.ID: 139, newcomer: 0, rich: 0}, before)
	legacyStats, err := legacyAgents.GetAgentStats(ctx, holder)
	require.NoError(t, err)
	t.Logf("legacy GetAgentStats(holder).Reputation before the cutover = %d (the legacy agent formula scores no comment)", legacyStats.Reputation)

	_, err = MigrateContributions(ctx, pool)
	require.NoError(t, err)
	_, err = RemapLegacyRelations(ctx, pool)
	require.NoError(t, err)
	// An agent whose reputation is only history: the legacy formulas cannot see it.
	_, err = pool.Exec(ctx, `
		INSERT INTO reputation_history (source, source_id, owner_type, owner_id, points, earned_at)
		VALUES ('problem_solved', gen_random_uuid(), 'agent', $1, 1000, NOW())`, rich)
	require.NoError(t, err)

	agents, users := NewCanonicalReputationAgentRepository(pool), NewCanonicalReputationUserRepository(pool)
	agentList := func() []models.AgentWithPostCount {
		list, total, err := agents.List(ctx, models.AgentListOptions{Query: sfx, Status: "all", Sort: "reputation", PerPage: 100})
		require.NoError(t, err)
		require.Equal(t, 3, total)
		return list
	}
	userListed := func() int {
		for offset := 0; ; offset += 100 {
			list, total, err := users.List(ctx, models.PublicUserListOptions{Limit: 100, Offset: offset, Sort: models.PublicUserSortReputation})
			require.NoError(t, err)
			for _, u := range list {
				if u.ID == user.ID {
					return u.Reputation
				}
			}
			require.Less(t, offset+100, total, "user not listed")
		}
	}
	// surfaces returns, per owner, each reputation surface's value.
	surfaces := func() map[string]map[string]int {
		board := boardOf(func(o models.LeaderboardOptions) ([]models.LeaderboardEntry, int, error) {
			return served.GetLeaderboard(ctx, o)
		})
		out := map[string]map[string]int{}
		listed := agentList()
		for _, id := range owners[:3] {
			stats, err := agents.GetAgentStats(ctx, id)
			require.NoError(t, err)
			out[id] = map[string]int{"leaderboard": board[id], "stats": stats.Reputation}
			for _, a := range listed {
				if a.ID == id {
					out[id]["list"] = a.Reputation
				}
			}
		}
		stats, err := users.GetUserStats(ctx, user.ID)
		require.NoError(t, err)
		out[user.ID] = map[string]int{"leaderboard": board[user.ID], "stats": stats.Reputation, "list": userListed()}
		return out
	}
	every := func(rep int) map[string]int { return map[string]int{"leaderboard": rep, "stats": rep, "list": rep} }

	assert.Equal(t, map[string]map[string]int{
		holder: every(217), user.ID: every(139), newcomer: every(0), rich: every(1000),
	}, surfaces(), "after the cutover every surface shows the reputation the legacy leaderboard showed before it")
	listed := agentList()
	assert.Equal(t, []string{rich, holder, newcomer}, []string{listed[0].ID, listed[1].ID, listed[2].ID},
		"the agents list sorts by canonical reputation")

	canonicalStats, err := agents.GetAgentStats(ctx, holder)
	require.NoError(t, err)
	legacyNow, err := legacyAgents.GetAgentStats(ctx, holder)
	require.NoError(t, err)
	canonicalStats.Reputation, legacyNow.Reputation = 0, 0
	assert.Equal(t, legacyNow, canonicalStats, "only Reputation differs from the legacy stats")
	missing, err := agents.GetAgentStats(ctx, "agent_rp_missing_"+sfx)
	require.NoError(t, err)
	assert.Equal(t, &models.AgentStats{}, missing, "an unknown agent has zero stats, as before")

	// Live votes after the cutover: on a migrated reply, on a native post and on a native reply.
	replies, posts := NewReplyRepository(pool), NewPostRepository(pool)
	acceptedReply, _, _, _ := replyFor(t, pool, ctx, "answer", accepted)
	require.NoError(t, replies.Vote(ctx, acceptedReply, "agent", voter+"_live1", "up"))
	native := post("post", "open", "agent", newcomer)
	require.NoError(t, posts.Vote(ctx, native, "agent", voter+"_live2", "up"))
	nativeReply, err := replies.Create(ctx, &models.Reply{PostID: solved, AuthorType: models.AuthorTypeAgent, AuthorID: newcomer, Body: "rpc native reply"})
	require.NoError(t, err)
	require.NoError(t, replies.Vote(ctx, nativeReply.ID, "agent", voter+"_live3", "down"))
	live := map[string]map[string]int{holder: every(219), user.ID: every(139), newcomer: every(1), rich: every(1000)}
	assert.Equal(t, live, surfaces(), "live votes move every surface by the same points")

	_, err = RemapLegacyRelations(ctx, pool)
	require.NoError(t, err)
	assert.Equal(t, live, surfaces(), "a second cutover run changes no reputation")
}
