package db

import (
	"context"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Task idx 76 step 3: GET /v1/agents/{id}/activity is served from canonical posts and
// replies. The legacy AgentRepository.GetActivity, which reads answers, approaches and
// responses, stays unwired until the tables go.
func TestLegacyAgentActivity_ServedCanonically(t *testing.T) {
	assert.Empty(t, productionSourcesContaining(t, "AgentRepository.GetActivity("),
		"no production code calls the legacy agent activity")

	src, err := ScanLegacySourceDependencies(backendRoot(t))
	require.NoError(t, err)
	for _, dep := range src {
		assert.NotEqual(t, "code:internal/db/activity_canonical.go", dep.Key, "the canonical activity names no legacy table or type")
	}
	d, ok := LegacyDependencyDispositions["code:internal/db/agents.go"]
	require.True(t, ok)
	assert.Equal(t, LegacyActionRefactor, d.Action)
	assert.False(t, d.Done, "agents.go still holds the unwired legacy activity and stats and must lose them when the tables go")
	assert.Contains(t, d.Note, "activity_canonical.go", "the note names the served activity")
	assert.Contains(t, d.Note, "profile_stats_canonical.go", "the note names the served stats")
}

// Task idx 76 steps 3 and 5: in a database holding only this fixture, every item the legacy
// activity lists before the contribution cutover is listed by the canonical activity after it,
// as the reply the contribution became: same date, same post, same post title, type "reply",
// action "replied", its body's first 100 characters as title, "accepted" when its post names it
// as the accepted reply. Comments, which the legacy feed never listed, are replies now and are
// listed. The total counts exactly the listed items: the legacy total also counted the agent's
// family posts, which the feed never lists. The canonical activity survives dropping the legacy
// tables unchanged; native replies move it, and a deleted reply or another agent's does not.
func TestCanonicalAgentActivity_ListsPostsAndRepliesAcrossTheCutover(t *testing.T) {
	pool, dropLegacy := newMigratedScratchDatabase(t)
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

	a, b := "agent_activity_a", "agent_activity_b"
	insertRemapAgent(t, pool, ctx, a)
	insertRemapAgent(t, pool, ctx, b)
	human, err := NewUserRepository(pool).Create(ctx, &models.User{
		Username: "activityhuman", DisplayName: "Activity Human", Email: "activityhuman@example.com",
		AuthProvider: models.AuthProviderGitHub, AuthProviderID: "gh_activityhuman", Role: models.UserRoleUser,
	})
	require.NoError(t, err)
	h := human.ID

	post := func(postType, authorType, author string) string {
		return insertTestPostWithAuthor(t, pool, ctx, postType, "activity "+postType+" by "+authorType, "body", []string{"activity"}, "open", authorType, author)
	}
	family := func(p string) string {
		exec(`UPDATE posts SET visibility = 'family', owner_human_id = $2 WHERE id = $1`, p, h)
		return p
	}
	pA := post("problem", "agent", a)
	qA := post("question", "agent", a)
	family(post("idea", "agent", a)) // counted by the legacy total, never listed
	exec(`UPDATE posts SET deleted_at = NOW() WHERE id = $1`, post("problem", "agent", a))
	qH := post("question", "human", h)
	qHFamily := family(post("question", "human", h))
	iH := post("idea", "human", h)
	pH := post("problem", "human", h)

	answer := func(question, author string, deleted bool) string {
		return id(`INSERT INTO answers (question_id, author_type, author_id, content, deleted_at)
			VALUES ($1, 'agent', $2, $3, CASE WHEN $4 THEN NOW() END)`, question, author, "activity answer by "+author, deleted)
	}
	accepted := answer(qH, a, false)
	exec(`UPDATE answers SET is_accepted = true WHERE id = $1`, accepted)
	exec(`UPDATE posts SET accepted_answer_id = $2 WHERE id = $1`, qH, accepted)
	answer(qHFamily, a, false) // listed without its family post's title
	answer(qH, a, true)
	answer(qA, b, false)
	insertApproach(t, pool, ctx, pH, a, "succeeded", false)
	insertApproach(t, pool, ctx, pH, a, "working", true)
	exec(`INSERT INTO responses (idea_id, author_type, author_id, content, response_type)
		VALUES ($1, 'agent', $2, 'activity response', 'support')`, iH, a)
	topComment := insertComment(t, pool, ctx, "post", qH, a, "a comment on a question")          // a top-level reply once migrated
	childComment := insertComment(t, pool, ctx, "answer", accepted, a, "a comment on an answer") // a child reply once migrated

	activity := func(repo interface {
		GetActivity(context.Context, string, int, int) ([]models.ActivityItem, int, error)
	}, page, perPage int) ([]models.ActivityItem, int) {
		t.Helper()
		items, total, err := repo.GetActivity(ctx, a, page, perPage)
		require.NoError(t, err)
		return items, total
	}

	legacy, legacyTotal := activity(NewAgentRepository(pool), 1, 50)
	var legacyTypes []string
	for _, it := range legacy {
		legacyTypes = append(legacyTypes, it.Type)
	}
	sort.Strings(legacyTypes)
	require.Equal(t, []string{"answer", "answer", "approach", "post", "post", "response"}, legacyTypes,
		"legacy: the public posts and the live answers, approach and response; no comment")
	require.Equal(t, 7, legacyTotal, "legacy: the total also counts the family post the feed does not list")

	_, err = MigrateContributions(ctx, pool)
	require.NoError(t, err)
	_, err = RemapLegacyRelations(ctx, pool)
	require.NoError(t, err)

	// The reply a legacy row became.
	asReply := func(legacyType, legacyID string, from models.ActivityItem) models.ActivityItem {
		t.Helper()
		var replyID, title, postID string
		require.NoError(t, pool.QueryRow(ctx, `SELECT id::text, LEFT(body, 100), post_id::text FROM replies
			WHERE legacy_type = $1 AND legacy_id = $2`, legacyType, legacyID).Scan(&replyID, &title, &postID))
		from.ID, from.Type, from.Action, from.Title, from.PostType, from.Status = replyID, "reply", "replied", title, "", ""
		if legacyID == accepted {
			from.Status = "accepted"
		}
		if from.TargetID == "" {
			from.TargetID = postID
		}
		return from
	}
	var want []models.ActivityItem
	for _, it := range legacy {
		if it.Type != "post" {
			it = asReply(it.Type, it.ID, it)
		}
		want = append(want, it)
	}
	for _, c := range []string{topComment, childComment} {
		var item models.ActivityItem
		require.NoError(t, pool.QueryRow(ctx, `SELECT created_at FROM comments WHERE id = $1`, c).Scan(&item.CreatedAt))
		item.TargetTitle = "activity question by human"
		want = append(want, asReply("comment", c, item))
	}
	sort.SliceStable(want, func(i, j int) bool {
		if !want[i].CreatedAt.Equal(want[j].CreatedAt) {
			return want[i].CreatedAt.After(want[j].CreatedAt)
		}
		return want[i].ID < want[j].ID
	})

	agents := NewCanonicalReputationAgentRepository(pool)
	got, total := activity(agents, 1, 50)
	assert.Equal(t, want, got, "each legacy item as its reply, plus both comments")
	assert.Equal(t, len(want), total, "the total counts the listed items only")
	var familyTitles int
	for _, it := range got {
		if it.Type == "reply" && it.TargetID == qHFamily {
			familyTitles++
			assert.Empty(t, it.TargetTitle, "a family post's title is not shown")
		}
	}
	assert.Equal(t, 1, familyTitles)

	dropLegacy()
	afterDrop, afterDropTotal := activity(agents, 1, 50)
	assert.Equal(t, got, afterDrop, "the canonical activity needs no legacy table")
	assert.Equal(t, total, afterDropTotal)

	// Native replies after the cutover: a's reply on qA becomes its accepted reply; its child
	// reply and a reply on the family question are listed; a deleted reply and b's reply are not.
	long := strings.Repeat("x", 150)
	nA := id(`INSERT INTO replies (post_id, author_type, author_id, body) VALUES ($1, 'agent', $2, $3)`, qA, a, long)
	exec(`UPDATE posts SET accepted_answer_id = $2 WHERE id = $1`, qA, nA)
	nChild := id(`INSERT INTO replies (post_id, parent_reply_id, author_type, author_id, body) VALUES ($1, $2, 'agent', $3, 'native child')`, qA, nA, a)
	nFamily := id(`INSERT INTO replies (post_id, author_type, author_id, body) VALUES ($1, 'agent', $2, 'native on family')`, qHFamily, a)
	exec(`INSERT INTO replies (post_id, author_type, author_id, body, deleted_at) VALUES ($1, 'agent', $2, 'gone', NOW())`, pA, a)
	exec(`INSERT INTO replies (post_id, author_type, author_id, body) VALUES ($1, 'agent', $2, 'not mine')`, pA, b)

	live, liveTotal := activity(agents, 1, 50)
	require.Len(t, live, len(want)+3)
	assert.Equal(t, len(want)+3, liveTotal)
	undated := func(it models.ActivityItem) models.ActivityItem { it.CreatedAt = time.Time{}; return it }
	assert.Equal(t, models.ActivityItem{ID: nFamily, Type: "reply", Action: "replied", Title: "native on family",
		TargetID: qHFamily}, undated(live[0]), "newest first; no family post title")
	assert.Equal(t, models.ActivityItem{ID: nChild, Type: "reply", Action: "replied", Title: "native child",
		TargetID: qA, TargetTitle: "activity question by agent"}, undated(live[1]), "a child reply is activity")
	assert.Equal(t, models.ActivityItem{ID: nA, Type: "reply", Action: "replied", Title: long[:100], Status: "accepted",
		TargetID: qA, TargetTitle: "activity question by agent"}, undated(live[2]), "the accepted native reply")
	assert.Equal(t, got, live[3:], "the earlier items are unchanged")

	page2, page2Total := activity(agents, 2, 4)
	assert.Equal(t, live[4:8], page2)
	assert.Equal(t, liveTotal, page2Total)
	beyond, beyondTotal := activity(agents, 10, 4)
	assert.Empty(t, beyond)
	assert.Equal(t, liveTotal, beyondTotal, "a page past the end still reports the total")

	_, _, err = agents.GetActivity(ctx, "agent_activity_missing", 1, 20)
	assert.ErrorIs(t, err, ErrAgentNotFound, "an unknown agent is not found, as before")
}
