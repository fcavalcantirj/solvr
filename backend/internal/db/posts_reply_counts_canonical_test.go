package db

import (
	"context"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Task idx 76 step 3: the posts list and detail read a post's contribution counts (and the
// has_answer filter and the answers/approaches/hot sorts built on them) from its replies, and
// the unwired approach-based crystallization lister lives with the legacy problem repository,
// so posts.go names no legacy table or type.
func TestLegacyPostCounts_ServedCanonically(t *testing.T) {
	src, err := ScanLegacySourceDependencies(backendRoot(t))
	require.NoError(t, err)
	for _, dep := range src {
		assert.NotEqual(t, "code:internal/db/posts.go", dep.Key, "post reads count replies, not legacy contributions")
	}
	_, ok := LegacyDependencyDispositions["code:internal/db/posts.go"]
	assert.False(t, ok, "posts.go no longer depends on a legacy table, so it carries no disposition")
	assert.Empty(t, productionSourcesContaining(t, "func (r *PostRepository) ListCrystallizationCandidates("),
		"the unwired approach-based crystallization lister was deleted with the legacy problem repository (idx 68)")
	assertLegacyDependencyGone(t, "code:internal/db/problems.go")
}

// postCounts is what the posts list and detail report for one post.
type postCounts struct{ Answers, Approaches, Comments, Replies int }

// Task idx 76 steps 3 and 5: after the contribution cutover a post's counts are read from its
// live replies. Every live reply lands in exactly one bucket, so answers + approaches +
// comments = reply_count = the post's reply list total: approaches are the migrated
// approaches; answers are top-level human/agent replies other than migrated approaches,
// comments and progress notes (migrated answers and responses, native replies); comments are
// the rest (migrated comments, progress notes, child replies, system replies). has_answer and
// the answers/approaches sorts follow the buckets, and all of it keeps working once the legacy
// tables are gone.
func TestCanonicalPostCounts_KeepTheLegacyCountsAcrossTheCutover(t *testing.T) {
	pool, dropLegacy := newMigratedScratchDatabase(t)
	ctx := context.Background()
	exec := func(sql string, args ...any) string {
		t.Helper()
		var id string
		require.NoError(t, pool.QueryRow(ctx, sql, args...).Scan(&id), sql)
		return id
	}
	a := "agent_post_counts"
	insertRemapAgent(t, pool, ctx, a)
	tag := "postcounts"
	post := func(postType, status, title string) string {
		t.Helper()
		return insertTestPostWithAuthor(t, pool, ctx, postType, title, "post counts body", []string{tag}, status, "agent", a)
	}
	answer := func(questionID string, deleted bool) string {
		t.Helper()
		return exec(`INSERT INTO answers (question_id, author_type, author_id, content, deleted_at)
			VALUES ($1, 'agent', $2, 'an answer', CASE WHEN $3 THEN NOW() END) RETURNING id::text`, questionID, a, deleted)
	}
	comment := func(targetType, targetID, authorType string) string {
		t.Helper()
		return exec(`INSERT INTO comments (target_type, target_id, author_type, author_id, content)
			VALUES ($1, $2, $3, $4, 'a comment') RETURNING id::text`, targetType, targetID, authorType, a)
	}
	response := func(ideaID string) {
		t.Helper()
		exec(`INSERT INTO responses (idea_id, author_type, author_id, content, response_type)
			VALUES ($1, 'agent', $2, 'a response', 'build') RETURNING id::text`, ideaID, a)
	}

	// q1: a live and a deleted answer, an agent and a system comment on the post, a comment
	// on the live answer. Legacy: answers 1, comments 2 (post comments only), replies 3.
	q1 := post("question", "open", "counts: answered question")
	an1 := answer(q1, false)
	answer(q1, true)
	comment("post", q1, "agent")
	comment("post", q1, "system")
	comment("answer", an1, "agent")
	// q2: only a comment on the post, so it stays unanswered. q3: nothing at all.
	q2 := post("question", "open", "counts: question with a comment only")
	comment("post", q2, "agent")
	q3 := post("question", "open", "counts: bare question")
	// p1: a live and a deleted approach, a progress note and a comment on the live approach,
	// a comment on the post. Legacy: approaches 1, comments 1, replies 2.
	p1 := post("problem", "open", "counts: problem with one approach")
	ap1 := insertApproach(t, pool, ctx, p1, a, "working", false)
	insertApproach(t, pool, ctx, p1, a, "failed", true)
	exec(`INSERT INTO progress_notes (approach_id, content) VALUES ($1, 'a progress note') RETURNING id::text`, ap1)
	comment("approach", ap1, "agent")
	comment("post", p1, "agent")
	// p2: three approaches, so it leads the approaches sort.
	p2 := post("problem", "open", "counts: problem with three approaches")
	for range 3 {
		insertApproach(t, pool, ctx, p2, a, "working", false)
	}
	// i1: two responses and a comment. Legacy: comments 1, replies 1 (responses were not counted).
	i1 := post("idea", "active", "counts: idea with responses")
	response(i1)
	response(i1)
	comment("post", i1, "agent")

	repo := NewPostRepository(pool)
	replies := NewReplyRepository(pool)
	list := func(opts models.PostListOptions) map[string]postCounts {
		t.Helper()
		opts.Tags, opts.Page, opts.PerPage = []string{tag}, 1, 50
		posts, total, err := repo.List(ctx, opts)
		require.NoError(t, err)
		assert.Equal(t, len(posts), total)
		out := make(map[string]postCounts, len(posts))
		for _, p := range posts {
			out[p.ID] = postCounts{p.AnswersCount, p.ApproachesCount, p.CommentsCount, p.ReplyCount}
		}
		return out
	}
	ids := func(opts models.PostListOptions) []string {
		t.Helper()
		opts.Tags, opts.Page, opts.PerPage = []string{tag}, 1, 50
		posts, _, err := repo.List(ctx, opts)
		require.NoError(t, err)
		out := make([]string, 0, len(posts))
		for _, p := range posts {
			out = append(out, p.ID)
		}
		return out
	}
	yes, no := true, false
	check := func(want map[string]postCounts) {
		t.Helper()
		assert.Equal(t, want, list(models.PostListOptions{}), "the posts list counts")
		for id, w := range want {
			got, err := repo.FindByID(ctx, id)
			require.NoError(t, err)
			assert.Equal(t, w, postCounts{got.AnswersCount, got.ApproachesCount, got.CommentsCount, got.ReplyCount}, "FindByID counts of %s", id)
			n, err := replies.CountByPost(ctx, id)
			require.NoError(t, err)
			assert.Equal(t, n, got.ReplyCount, "reply_count is the post's reply list total (%s)", id)
			assert.Equal(t, got.AnswersCount+got.ApproachesCount+got.CommentsCount, got.ReplyCount, "the buckets partition the replies (%s)", id)
		}
	}

	// Before the cutover no legacy row is a reply yet, so nothing counts.
	zero := map[string]postCounts{q1: {}, q2: {}, q3: {}, p1: {}, p2: {}, i1: {}}
	check(zero)

	// The shared-database fixtures' cutoverRepliesFor makes exactly the replies the real
	// cutover makes (same identity, parent, author and deletion).
	converted := func() []string {
		t.Helper()
		rows, err := pool.Query(ctx, `
			SELECT concat_ws('|', r.post_id, r.legacy_type, r.legacy_id, pr.legacy_type, pr.legacy_id,
				r.author_type, r.author_id, r.deleted_at IS NOT NULL)
			FROM replies r LEFT JOIN replies pr ON pr.id = r.parent_reply_id ORDER BY 1`)
		require.NoError(t, err)
		defer rows.Close()
		var out []string
		for rows.Next() {
			var s string
			require.NoError(t, rows.Scan(&s))
			out = append(out, s)
		}
		require.NoError(t, rows.Err())
		return out
	}
	cutoverRepliesFor(t, pool, ctx, q1, q2, q3, p1, p2, i1)
	fixture := converted()
	_, err := pool.Exec(ctx, `DELETE FROM replies`)
	require.NoError(t, err)

	_, err = MigrateContributions(ctx, pool)
	require.NoError(t, err)
	_, err = RemapLegacyRelations(ctx, pool)
	require.NoError(t, err)
	require.Len(t, fixture, 17)
	assert.Equal(t, fixture, converted(), "cutoverRepliesFor mirrors the cutover")

	migrated := map[string]postCounts{
		q1: {Answers: 1, Comments: 3, Replies: 4},    // + the comment on the answer
		q2: {Comments: 1, Replies: 1},                // a post comment is not an answer
		q3: {},                                       //
		p1: {Approaches: 1, Comments: 3, Replies: 4}, // + the note and the comment on the approach
		p2: {Approaches: 3, Replies: 3},              //
		i1: {Answers: 2, Comments: 1, Replies: 3},    // responses answer the idea
	}
	sorted := func() {
		t.Helper()
		// Every post is type post (idx 68), so the answer filters cover the problems and the idea too.
		assert.ElementsMatch(t, []string{q1, i1}, ids(models.PostListOptions{HasAnswer: &yes}))
		assert.ElementsMatch(t, []string{q2, q3, p1, p2}, ids(models.PostListOptions{HasAnswer: &no}))
		assert.Equal(t, []string{p2, p1}, ids(models.PostListOptions{Sort: "approaches"})[:2])
		assert.Equal(t, i1, ids(models.PostListOptions{Sort: "answers"})[0], "two answers lead the answers sort")
		assert.Len(t, ids(models.PostListOptions{Sort: "hot"}), 6, "the hot sort reads the same counts")
	}
	check(migrated)
	sorted()

	dropLegacy()
	check(migrated)
	sorted()

	// Native replies land in the same buckets: a top-level agent reply answers q3, its child
	// reply and a system verdict are comments, a deleted reply counts nowhere.
	n1 := exec(`INSERT INTO replies (post_id, author_type, author_id, body) VALUES ($1, 'agent', $2, 'native answer') RETURNING id::text`, q3, a)
	exec(`INSERT INTO replies (post_id, parent_reply_id, author_type, author_id, body) VALUES ($1, $2, 'agent', $3, 'native child') RETURNING id::text`, q3, n1, a)
	exec(`INSERT INTO replies (post_id, author_type, author_id, body) VALUES ($1, 'system', 'moderation', 'a verdict') RETURNING id::text`, q3)
	exec(`INSERT INTO replies (post_id, author_type, author_id, body, deleted_at) VALUES ($1, 'agent', $2, 'deleted', NOW()) RETURNING id::text`, q3, a)
	migrated[q3] = postCounts{Answers: 1, Comments: 2, Replies: 3}
	check(migrated)
	assert.ElementsMatch(t, []string{q1, q3, i1}, ids(models.PostListOptions{HasAnswer: &yes}))
	assert.ElementsMatch(t, []string{q2, p1, p2}, ids(models.PostListOptions{HasAnswer: &no}))
}
