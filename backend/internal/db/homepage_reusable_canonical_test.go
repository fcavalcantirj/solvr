package db

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Task idx 76 step 3: the overview's "Knowledge an agent can reuse" section is served from
// canonical posts and replies by CanonicalHomepageRepository. The legacy
// HomepageRepository.ListReusablePosts, which counts approaches, answers and responses, stays
// unwired until the tables go.
func TestLegacyReusablePosts_ServedCanonically(t *testing.T) {
	assert.ElementsMatch(t, []string{"internal/api/router_homepage.go"},
		productionSourcesContaining(t, "db.NewCanonicalHomepageRepository("),
		"the overview serves the canonical reusable posts")
	assert.ElementsMatch(t, []string{"internal/db/homepage_overview.go", "internal/db/homepage_reusable_canonical.go"},
		productionSourcesContaining(t, "NewHomepageRepository("),
		"only its definition and the canonical wrapper build the legacy homepage repository")

	src, err := ScanLegacySourceDependencies(backendRoot(t))
	require.NoError(t, err)
	for _, dep := range src {
		assert.NotEqual(t, "code:internal/db/homepage_reusable_canonical.go", dep.Key, "the canonical reusable posts name no legacy table or type")
	}
	d, ok := LegacyDependencyDispositions["code:internal/db/homepage_overview.go"]
	require.True(t, ok)
	assert.Equal(t, LegacyActionRefactor, d.Action)
	assert.False(t, d.Done, "homepage_overview.go still holds the unwired legacy ListReusablePosts and must lose it when the tables go")
	assert.Contains(t, d.Note, "homepage_reusable_canonical.go", "the note names the served reusable posts")
}

// Task idx 76 steps 3 and 5: in a database holding only this fixture, the canonical reusable
// posts after the contribution cutover are the legacy list before it, in the same order with
// the same post fields, except where comments now count: a comment is a reply, so a post
// carrying only a comment becomes reusable and a comment adds one to its post's count. The list
// survives dropping the legacy tables unchanged. Native replies move it: a human or agent reply
// and a child reply count, a deleted reply and a system verdict do not.
func TestCanonicalReusablePosts_KeepTheLegacyListAcrossTheCutover(t *testing.T) {
	pool, dropLegacy := newMigratedScratchDatabase(t)
	ctx := context.Background()
	exec := func(sql string, args ...any) {
		t.Helper()
		_, err := pool.Exec(ctx, sql, args...)
		require.NoError(t, err, sql)
	}
	a := "agent_reuse_a"
	insertRemapAgent(t, pool, ctx, a)

	// Each post's last activity is set explicitly, one minute apart, newest last.
	base := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	minute := 0
	post := func(postType, title string) string {
		t.Helper()
		p := insertTestPostWithAuthor(t, pool, ctx, postType, title, "reusable body", []string{"reuse"}, "open", "agent", a)
		minute++
		at := base.Add(time.Duration(minute) * time.Minute)
		exec(`UPDATE posts SET created_at = $2, updated_at = $2 WHERE id = $1`, p, at)
		return p
	}
	answer := func(question string, deleted bool) string {
		t.Helper()
		var id string
		require.NoError(t, pool.QueryRow(ctx, `INSERT INTO answers (question_id, author_type, author_id, content, deleted_at)
			VALUES ($1, 'agent', $2, 'reusable answer', CASE WHEN $3 THEN NOW() END) RETURNING id::text`, question, a, deleted).Scan(&id))
		return id
	}

	pApproach := post("problem", "reuse problem with an approach")
	insertApproach(t, pool, ctx, pApproach, a, "working", false)
	pDeletedApproach := post("problem", "reuse problem with a deleted approach only")
	insertApproach(t, pool, ctx, pDeletedApproach, a, "failed", true)
	qAnswers := post("question", "reuse question with answers")
	firstAnswer := answer(qAnswers, false)
	answer(qAnswers, false)
	answer(qAnswers, true)
	insertComment(t, pool, ctx, "answer", firstAnswer, a, "a comment on an answer") // a child reply once migrated
	iResponse := post("idea", "reuse idea with a response")
	exec(`INSERT INTO responses (idea_id, author_type, author_id, content, response_type)
		VALUES ($1, 'agent', $2, 'reusable response', 'support')`, iResponse, a)
	qComment := post("question", "reuse question with a comment only")
	insertComment(t, pool, ctx, "post", qComment, a, "a comment on a question") // a top-level reply once migrated
	pMixed := post("problem", "reuse problem with an approach and a comment")
	approach := insertApproach(t, pool, ctx, pMixed, a, "succeeded", false)
	insertComment(t, pool, ctx, "approach", approach, a, "a comment on an approach")
	pFamily := post("problem", "reuse family problem")
	insertApproach(t, pool, ctx, pFamily, a, "working", false)
	exec(`UPDATE posts SET visibility = 'family' WHERE id = $1`, pFamily)
	pDraft := post("problem", "reuse draft problem")
	insertApproach(t, pool, ctx, pDraft, a, "working", false)
	exec(`UPDATE posts SET status = 'draft' WHERE id = $1`, pDraft)
	pDeleted := post("problem", "reuse deleted problem")
	insertApproach(t, pool, ctx, pDeleted, a, "working", false)
	exec(`UPDATE posts SET deleted_at = NOW() WHERE id = $1`, pDeleted)

	list := func(repo interface {
		ListReusablePosts(context.Context, int) ([]ReusablePost, error)
	}, limit int) []ReusablePost {
		t.Helper()
		posts, err := repo.ListReusablePosts(ctx, limit)
		require.NoError(t, err)
		for i := range posts {
			posts[i].LastActivityAt = posts[i].LastActivityAt.UTC()
		}
		return posts
	}
	ids := func(posts []ReusablePost) []string {
		out := make([]string, 0, len(posts))
		for _, p := range posts {
			out = append(out, p.ID)
		}
		return out
	}
	counts := func(posts []ReusablePost) map[string]int {
		out := map[string]int{}
		for _, p := range posts {
			out[p.ID] = p.ContributionCount
		}
		return out
	}

	legacy := list(NewHomepageRepository(pool), 50)
	require.Equal(t, []string{pMixed, iResponse, qAnswers, pApproach}, ids(legacy),
		"legacy: posts with a live approach, answer or response, newest activity first; no comment-only, family, draft or deleted post")
	require.Equal(t, map[string]int{pMixed: 1, iResponse: 1, qAnswers: 2, pApproach: 1}, counts(legacy),
		"legacy: live approaches, answers and responses; comments never count")

	_, err := MigrateContributions(ctx, pool)
	require.NoError(t, err)
	_, err = RemapLegacyRelations(ctx, pool)
	require.NoError(t, err)

	want := make([]ReusablePost, 0, len(legacy)+1)
	for _, p := range legacy {
		switch p.ID {
		case pMixed, qAnswers:
			p.ContributionCount++ // its comment is a reply now
		}
		want = append(want, p)
		if p.ID == pMixed {
			want = append(want, ReusablePost{ID: qComment, Type: "question", Title: "reuse question with a comment only",
				Status: "open", Tags: []string{"reuse"}, ContributionCount: 1, LastActivityAt: base.Add(5 * time.Minute)})
		}
	}

	home := NewCanonicalHomepageRepository(pool)
	got := list(home, 50)
	assert.Equal(t, want, got, "the legacy list as replies, plus the comments")

	dropLegacy()
	assert.Equal(t, got, list(home, 50), "the canonical reusable posts need no legacy table")
	assert.Equal(t, got[:2], list(home, 2), "the limit keeps the newest")
	assert.Len(t, list(home, 0), len(got), "no limit reads the default six")

	// Native replies after the cutover.
	pNative := post("post", "reuse native post")
	var nativeReply string
	require.NoError(t, pool.QueryRow(ctx, `INSERT INTO replies (post_id, author_type, author_id, body)
		VALUES ($1, 'agent', $2, 'native reply') RETURNING id::text`, pNative, a).Scan(&nativeReply))
	exec(`INSERT INTO replies (post_id, parent_reply_id, author_type, author_id, body) VALUES ($1, $2, 'agent', $3, 'native child')`, pNative, nativeReply, a)
	pVerdict := post("post", "reuse post with a moderation verdict only")
	exec(`INSERT INTO replies (post_id, author_type, author_id, body) VALUES ($1, 'system', 'moderation', 'approved')`, pVerdict)
	pGone := post("post", "reuse post with a deleted reply only")
	exec(`INSERT INTO replies (post_id, author_type, author_id, body, deleted_at) VALUES ($1, 'agent', $2, 'gone', NOW())`, pGone, a)
	exec(`INSERT INTO replies (post_id, author_type, author_id, body) VALUES ($1, 'human', 'some-human', 'a human reply')`, iResponse)
	exec(`INSERT INTO replies (post_id, author_type, author_id, body, deleted_at) VALUES ($1, 'agent', $2, 'gone too', NOW())`, pApproach, a)
	exec(`INSERT INTO replies (post_id, author_type, author_id, body) VALUES ($1, 'system', 'moderation', 'approved')`, qAnswers)

	live := list(home, 50)
	require.Equal(t, append([]string{pNative}, ids(got)...), ids(live),
		"the native post leads; a verdict-only or deleted-reply-only post is not reusable")
	wantCounts := counts(got)
	wantCounts[pNative] = 2
	wantCounts[iResponse]++
	assert.Equal(t, wantCounts, counts(live), "a human reply counts; a deleted reply or a verdict does not")
}
