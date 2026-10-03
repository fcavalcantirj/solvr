package db

import (
	"context"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Task idx 76 step 3: the stats counts on profiles (GET /v1/agents/{id}, /v1/users/{id},
// /v1/me, the resurrection bundle) are served from canonical posts, replies and votes. The
// legacy GetAgentStats/GetUserStats, which read answers and responses, were deleted with the
// legacy tables (idx 68).
func TestLegacyProfileStats_ServedCanonically(t *testing.T) {
	assert.Empty(t, productionSourcesContaining(t, "AgentRepository.GetAgentStats("),
		"no production code calls the legacy agent stats")
	assert.Empty(t, productionSourcesContaining(t, "UserRepository.GetUserStats("),
		"no production code calls the legacy user stats")

	src, err := ScanLegacySourceDependencies(backendRoot(t))
	require.NoError(t, err)
	for _, dep := range src {
		assert.NotEqual(t, "code:internal/db/profile_stats_canonical.go", dep.Key, "the canonical profile stats name no legacy table or type")
		assert.NotEqual(t, "code:internal/db/reputation_canonical.go", dep.Key, "the canonical profile repositories name no legacy table or type")
	}
	for _, key := range []string{"code:internal/db/agents.go", "code:internal/db/users.go"} {
		assertLegacyDependencyGone(t, key) // their legacy stats were deleted (idx 68)
	}
}

// Task idx 76 steps 3 and 5, idx 68: in a database holding only this fixture, the canonical
// profile stats after the contribution cutover count the owner's live posts, every live reply
// (migrated answers, responses, approaches and comments alike) and confirmed upvotes on their
// posts and replies (approach upvotes included). They survive dropping the legacy tables
// unchanged, and native replies and votes move them. The per-type counters were retired.
func TestCanonicalProfileStats_KeepsLegacyCountsAcrossTheCutover(t *testing.T) {
	pool, archiveLegacy := newPreArchiveScratchDatabase(t)
	ctx := context.Background()
	exec := func(sql string, args ...any) {
		t.Helper()
		_, err := pool.Exec(ctx, sql, args...)
		require.NoError(t, err, sql)
	}
	id := func(sql string, args ...any) string {
		t.Helper()
		var out string
		require.NoError(t, pool.QueryRow(ctx, sql+` RETURNING id::text`, args...).Scan(&out), sql)
		return out
	}

	a, b := "agent_pstats_a", "agent_pstats_b"
	insertRemapAgent(t, pool, ctx, a)
	insertRemapAgent(t, pool, ctx, b)
	human, err := NewUserRepository(pool).Create(ctx, &models.User{
		Username: "pstatshuman", DisplayName: "Profile Stats Human", Email: "pstatshuman@example.com",
		AuthProvider: models.AuthProviderGitHub, AuthProviderID: "gh_pstatshuman", Role: models.UserRoleUser,
	})
	require.NoError(t, err)
	h := human.ID

	post := func(postType, status, authorType, author string) string {
		return insertTestPostWithAuthor(t, pool, ctx, postType, "pstats "+postType, "body", []string{"pstats"}, status, authorType, author)
	}
	pASolved := post("problem", "solved", "agent", a)
	pAOpen := post("problem", "open", "agent", a)
	qA := post("question", "open", "agent", a)
	iA := post("idea", "open", "agent", a)
	pADeleted := post("problem", "solved", "agent", a)
	exec(`UPDATE posts SET deleted_at = NOW() WHERE id = $1`, pADeleted)
	qH := post("question", "open", "human", h)
	iH := post("idea", "open", "human", h)
	pH := post("problem", "open", "human", h)

	answer := func(question, authorType, author string, accepted, deleted bool) string {
		return id(`INSERT INTO answers (question_id, author_type, author_id, content, is_accepted, deleted_at)
			VALUES ($1, $2, $3, 'pstats answer', $4, CASE WHEN $5 THEN NOW() END)`, question, authorType, author, accepted, deleted)
	}
	ansAAccepted := answer(qH, "agent", a, true, false)
	answer(qA, "agent", a, false, false)
	answer(qH, "agent", a, false, true)
	ansH := answer(qA, "human", h, true, false)
	answer(qH, "agent", b, false, false)
	exec(`UPDATE posts SET accepted_answer_id = $2 WHERE id = $1`, qH, ansAAccepted)
	exec(`UPDATE posts SET accepted_answer_id = $2 WHERE id = $1`, qA, ansH)
	response := func(idea, authorType, author string) string {
		return id(`INSERT INTO responses (idea_id, author_type, author_id, content, response_type)
			VALUES ($1, $2, $3, 'pstats response', 'support')`, idea, authorType, author)
	}
	rA := response(iH, "agent", a)
	rH := response(iA, "human", h)
	apA := insertApproach(t, pool, ctx, pH, a, "succeeded", false)
	apH := id(`INSERT INTO approaches (problem_id, author_type, author_id, angle, status)
		VALUES ($1, 'human', $2, 'pstats human angle', 'working')`, pAOpen, h)
	insertComment(t, pool, ctx, "post", qH, a, "a comment on a question") // a top-level reply once migrated
	exec(`INSERT INTO comments (target_type, target_id, author_type, author_id, content) VALUES
		('post', $1, 'human', $3, 'a comment on an idea'),
		('answer', $2, 'human', $3, 'a comment on an answer')`, iA, ansAAccepted, h) // top-level and child replies
	exec(`INSERT INTO replies (post_id, author_type, author_id, body) VALUES ($1, 'system', 'solvr-moderation', 'verdict')`, qA)

	voters := 0
	vote := func(targetType, target, direction string, confirmed bool) {
		voters++
		exec(`INSERT INTO votes (target_type, target_id, voter_type, voter_id, direction, confirmed)
			VALUES ($1, $2, 'agent', $3, $4, $5)`, targetType, target, "pstats_voter_"+string(rune('a'+voters)), direction, confirmed)
	}
	vote("post", pASolved, "up", true)
	vote("answer", ansAAccepted, "up", true)
	vote("response", rA, "up", true)
	vote("approach", apA, "up", true)
	vote("post", pAOpen, "up", false)
	vote("answer", ansH, "up", true)
	vote("post", qH, "up", true)
	vote("response", rH, "down", true)
	vote("approach", apH, "up", true)

	agentStats := func(repo interface {
		GetAgentStats(context.Context, string) (*models.AgentStats, error)
	}, agent string) models.AgentStats {
		t.Helper()
		s, err := repo.GetAgentStats(ctx, agent)
		require.NoError(t, err)
		return *s
	}
	userStats := func(repo interface {
		GetUserStats(context.Context, string) (*models.UserStats, error)
	}) models.UserStats {
		t.Helper()
		s, err := repo.GetUserStats(ctx, h)
		require.NoError(t, err)
		return *s
	}

	_, err = MigrateContributions(ctx, pool)
	require.NoError(t, err)
	_, err = RemapLegacyRelations(ctx, pool)
	require.NoError(t, err)

	agents, users := NewCanonicalReputationAgentRepository(pool), NewCanonicalReputationUserRepository(pool)
	withoutReputation := func(s models.AgentStats) models.AgentStats { s.Reputation = 0; return s }
	canonicalA, canonicalB, canonicalH := agentStats(agents, a), agentStats(agents, b), userStats(users)
	// a: four live posts (not the deleted one); five live replies (two answers, the response, the
	// approach, the comment; not the deleted answer); four confirmed upvotes (a post, an answer,
	// the response and the approach, now replies; not the unconfirmed vote).
	wantA := models.AgentStats{PostsCreated: 4, Contributions: 5, UpvotesReceived: 4}
	assert.Equal(t, wantA, withoutReputation(canonicalA))
	assert.Equal(t, models.AgentStats{Contributions: 1}, withoutReputation(canonicalB), "b's one answer is a reply")
	// h: three posts; five replies (the answer, the response, the approach and both comments, the
	// one on the answer as a child reply); three upvotes (the answer, the question, the approach).
	wantH := models.UserStats{PostsCreated: 3, Contributions: 5, UpvotesReceived: 3}
	gotH := canonicalH
	gotH.Reputation = 0
	assert.Equal(t, wantH, gotH, "every live reply of h is a contribution")
	missing, err := agents.GetAgentStats(ctx, "agent_pstats_missing")
	require.NoError(t, err)
	assert.Equal(t, &models.AgentStats{}, missing, "an unknown agent has zero stats, as before")

	archiveLegacy()
	assert.Equal(t, canonicalA, agentStats(agents, a), "the canonical agent stats need no legacy table")
	assert.Equal(t, canonicalB, agentStats(agents, b))
	assert.Equal(t, canonicalH, userStats(users), "the canonical user stats need no legacy table")

	// Native replies after the cutover: a's reply and its child reply are contributions, its upvote
	// counts and the downvote does not; a deleted reply counts for nothing; h's reply on an idea
	// is a contribution.
	nA := id(`INSERT INTO replies (post_id, author_type, author_id, body) VALUES ($1, 'agent', $2, 'native answer')`, qA, a)
	exec(`INSERT INTO replies (post_id, parent_reply_id, author_type, author_id, body) VALUES ($1, $2, 'agent', $3, 'native child')`, qA, nA, a)
	exec(`INSERT INTO replies (post_id, author_type, author_id, body, deleted_at) VALUES ($1, 'agent', $2, 'gone', NOW())`, qH, a)
	exec(`INSERT INTO replies (post_id, author_type, author_id, body) VALUES ($1, 'human', $2, 'native idea reply')`, iA, h)
	vote("reply", nA, "up", true)
	vote("reply", nA, "down", true)

	liveA := withoutReputation(agentStats(agents, a))
	wantA.Contributions, wantA.UpvotesReceived = 7, 5
	assert.Equal(t, wantA, liveA)
	liveH := userStats(users)
	liveH.Reputation = 0
	wantH.Contributions = 6
	assert.Equal(t, wantH, liveH, "h's native reply on an idea is a contribution")
}
