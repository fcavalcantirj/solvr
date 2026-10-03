package db

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Task idx 76 step 3: the resurrection bundle's knowledge reads posts and replies. No
// resurrection query names a legacy table or compares a legacy post type, so resurrection.go
// carries no disposition.
func TestLegacyResurrection_ServedCanonically(t *testing.T) {
	src, err := ScanLegacySourceDependencies(backendRoot(t))
	require.NoError(t, err)
	for _, dep := range src {
		assert.NotEqual(t, "code:internal/db/resurrection.go", dep.Key, "the resurrection bundle reads posts and replies")
	}
	_, ok := LegacyDependencyDispositions["code:internal/db/resurrection.go"]
	assert.False(t, ok, "resurrection.go no longer depends on the legacy model, so it carries no disposition")
}

// resurrectionPost is what the bundle says about one of the agent's posts.
type resurrectionPost struct {
	ID, Status  string
	Up, Down    int
	Tags        []string
	CreatedAtOK bool
}

// resurrectionAttempt is what the bundle says about one approach.
type resurrectionAttempt struct {
	ID, PostID, Angle, Method, Status, CreatedAt string
}

// Task idx 76 step 3: GET /v1/agents/{id}/resurrection-bundle serves its knowledge from the
// canonical model. ideas lists the agent's live public posts of every type (Problems, Ideas and
// Questions are one Post model) by net votes; problems lists those still open (draft or open;
// in_progress and active are retired, idx 68, and read open after the archive) newest first; approaches lists the agent's live replies migrated from
// approaches, newest first, with the reply id, its post id and the angle, method and status the
// cutover kept in provenance. Native replies are never approaches. All of it keeps working
// once the legacy tables are gone.
func TestCanonicalResurrection_ServesEveryPostTypeAndMigratedApproachesAcrossTheCutover(t *testing.T) {
	pool, dropLegacy := newMigratedScratchDatabase(t)
	ctx := context.Background()
	exec := func(sql string, args ...any) string {
		t.Helper()
		var id string
		require.NoError(t, pool.QueryRow(ctx, sql, args...).Scan(&id), sql)
		return id
	}
	a, b := "agent_resur_canon", "agent_resur_other"
	insertRemapAgent(t, pool, ctx, a)
	insertRemapAgent(t, pool, ctx, b)
	base := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	created := map[string]time.Time{}
	n := 0
	post := func(author, postType, status string, up, down int) string {
		t.Helper()
		n++
		id := insertTestPostWithAuthor(t, pool, ctx, postType, "resurrection canon "+postType+" "+status,
			"a post body for the resurrection canon test", []string{"resur", postType}, status, "agent", author)
		at := base.Add(time.Duration(n) * time.Minute)
		_, err := pool.Exec(ctx, `UPDATE posts SET upvotes = $1, downvotes = $2, created_at = $3 WHERE id = $4`,
			up, down, at, id)
		require.NoError(t, err)
		created[id] = at
		return id
	}

	pIdea := post(a, "idea", "active", 5, 1)            // net 4, open
	pSolved := post(a, "problem", "solved", 3, 0)       // net 3, resolved
	pProblem := post(a, "problem", "open", 2, 0)        // net 2, open
	pWorking := post(a, "problem", "in_progress", 1, 0) // net 1, open
	pQuestion := post(a, "question", "open", 1, 0)      // net 1, open
	pPost := post(a, "post", "open", 0, 0)              // net 0, open
	pDraft := post(a, "question", "draft", 0, 0)        // net 0, open (a draft)
	pEvolved := post(a, "idea", "evolved", 0, 0)        // net 0, resolved
	pAnswered := post(a, "question", "answered", 0, 1)  // net -1, resolved
	pDeleted := post(a, "idea", "active", 9, 0)
	pFamily := post(a, "problem", "open", 9, 0)
	_, err := pool.Exec(ctx, `UPDATE posts SET deleted_at = NOW() WHERE id = $1`, pDeleted)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE posts SET visibility = 'family' WHERE id = $1`, pFamily)
	require.NoError(t, err)
	post(b, "problem", "open", 9, 0)

	approach := func(author, postID, angle string, method any, status string, at time.Time, deleted bool) string {
		t.Helper()
		return exec(`INSERT INTO approaches (problem_id, author_type, author_id, angle, method, status, created_at, deleted_at)
			VALUES ($1, 'agent', $2, $3, $4, $5, $6, CASE WHEN $7 THEN NOW() END) RETURNING id::text`,
			postID, author, angle, method, status, at, deleted)
	}
	ap1 := approach(a, pProblem, "the first angle", "the first method", "succeeded", base.Add(time.Hour), false)
	ap2 := approach(a, pSolved, "the second angle", nil, "failed", base.Add(2*time.Hour), false)
	approach(a, pProblem, "a deleted angle", "gone", "working", base.Add(3*time.Hour), true)
	approach(b, pProblem, "another agent's angle", "theirs", "working", base.Add(4*time.Hour), false)
	exec(`INSERT INTO answers (question_id, author_type, author_id, content) VALUES ($1, 'agent', $2, 'an answer')
		RETURNING id::text`, pQuestion, a)

	repo := NewResurrectionRepository(pool)
	ideas := func(limit int) []string {
		t.Helper()
		got, err := repo.GetAgentIdeas(ctx, a, limit)
		require.NoError(t, err)
		ids := make([]string, 0, len(got))
		for _, p := range got {
			ids = append(ids, p.ID)
		}
		return ids
	}
	problems := func() []string {
		t.Helper()
		got, err := repo.GetAgentOpenProblems(ctx, a)
		require.NoError(t, err)
		ids := make([]string, 0, len(got))
		for _, p := range got {
			assert.True(t, created[p.ID].Equal(p.CreatedAt), "problem %s created_at", p.ID)
			ids = append(ids, p.ID)
		}
		return ids
	}
	approaches := func(limit int) []resurrectionAttempt {
		t.Helper()
		got, err := repo.GetAgentApproaches(ctx, a, limit)
		require.NoError(t, err)
		require.NotNil(t, got, "an empty section is an empty list, not null")
		out := make([]resurrectionAttempt, 0, len(got))
		for _, ap := range got {
			out = append(out, resurrectionAttempt{ap.ID, ap.ProblemID, ap.Angle, ap.Method, ap.Status,
				ap.CreatedAt.UTC().Format(time.RFC3339)})
		}
		return out
	}

	wantIdeas := []string{pIdea, pSolved, pProblem, pQuestion, pWorking, pEvolved, pDraft, pPost, pAnswered}
	wantProblems := []string{pDraft, pPost, pQuestion, pProblem}
	checkPosts := func() {
		t.Helper()
		assert.Equal(t, wantIdeas, ideas(50), "every live public post of the agent, by net votes then newest")
		assert.Equal(t, wantIdeas[:2], ideas(2), "the ideas limit")
		assert.Equal(t, wantProblems, problems(), "the agent's live public posts still open, newest first")
		got, err := repo.GetAgentIdeas(ctx, a, 1)
		require.NoError(t, err)
		require.Len(t, got, 1)
		assert.Equal(t, resurrectionPost{pIdea, "active", 5, 1, []string{"resur", "idea"}, true},
			resurrectionPost{got[0].ID, got[0].Status, got[0].Upvotes, got[0].Downvotes, got[0].Tags,
				created[pIdea].Equal(got[0].CreatedAt)})
	}

	// Before the cutover no approach is a reply yet: the posts are served, approaches are not.
	checkPosts()
	assert.Empty(t, approaches(50))

	_, err = MigrateContributions(ctx, pool)
	require.NoError(t, err)
	_, err = RemapLegacyRelations(ctx, pool)
	require.NoError(t, err)
	replyOf := func(legacyID string) string {
		t.Helper()
		return exec(`SELECT id::text FROM replies WHERE legacy_type = 'approach' AND legacy_id = $1`, legacyID)
	}
	wantApproaches := []resurrectionAttempt{
		{replyOf(ap2), pSolved, "the second angle", "", "failed", base.Add(2 * time.Hour).Format(time.RFC3339)},
		{replyOf(ap1), pProblem, "the first angle", "the first method", "succeeded", base.Add(time.Hour).Format(time.RFC3339)},
	}
	check := func() {
		t.Helper()
		checkPosts()
		assert.Equal(t, wantApproaches, approaches(50), "the agent's live migrated approaches, newest first")
		assert.Equal(t, wantApproaches[:1], approaches(1), "the approaches limit")
	}
	check()

	dropLegacy()
	check()

	// Native replies after the drop are not approaches, top-level or not.
	native := exec(`INSERT INTO replies (post_id, author_type, author_id, body) VALUES ($1, 'agent', $2, 'a native reply')
		RETURNING id::text`, pPost, a)
	exec(`INSERT INTO replies (post_id, parent_reply_id, author_type, author_id, body)
		VALUES ($1, $2, 'agent', $3, 'a native child') RETURNING id::text`, pPost, native, a)
	check()
}
