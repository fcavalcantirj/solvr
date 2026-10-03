package db

import (
	"context"
	"maps"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Task idx 76 step 5: search moves with the schema. The post results' counts and the
// content_types=answers / content_types=approaches sources read replies, so no search code
// names a legacy table and search.go carries no disposition.
func TestLegacySearch_ServedCanonically(t *testing.T) {
	src, err := ScanLegacySourceDependencies(backendRoot(t))
	require.NoError(t, err)
	for _, dep := range src {
		assert.NotEqual(t, "code:internal/db/search.go", dep.Key, "search reads replies, not legacy contributions")
		assert.NotEqual(t, "code:internal/db/search_replies.go", dep.Key, "the reply sources name no legacy table")
	}
	_, ok := LegacyDependencyDispositions["code:internal/db/search.go"]
	assert.False(t, ok, "search.go no longer depends on a legacy table, so it carries no disposition")
}

// searchHit is what one search result says about itself.
type searchHit struct {
	Source, Type, Status, Author string
	Votes                        int
}

// searchCounts is what a post result reports about its replies.
type searchCounts struct{ Answers, Approaches, Comments int }

// Task idx 76 step 5: after the contribution cutover, search finds the migrated contributions
// as replies. content_types=answers returns the replies in the answer bucket of the post counts
// (migrated answers and responses, native top-level human/agent replies); content_types=
// approaches returns the migrated approaches; post results count their replies in the same
// buckets as the posts list. A reply is found only when its post passes the rule the post
// results of the same search use (not deleted, not a draft, pending or rejected, public or the
// viewer's family), and all of it keeps working once the legacy tables are gone.
func TestCanonicalSearch_FindsMigratedContributionsAcrossTheCutover(t *testing.T) {
	pool, dropLegacy := newMigratedScratchDatabase(t)
	ctx := context.Background()
	exec := func(sql string, args ...any) string {
		t.Helper()
		var id string
		require.NoError(t, pool.QueryRow(ctx, sql, args...).Scan(&id), sql)
		return id
	}
	a := "agent_search_canon"
	insertRemapAgent(t, pool, ctx, a)
	owner := exec(`INSERT INTO users (username, display_name, email, referral_code)
		VALUES ('searchcanon', 'searchcanon', 'searchcanon@example.com', 'SRCHCANO') RETURNING id::text`)
	kw := "quokkasearch"
	tags := []string{"searchcanon"}
	post := func(postType, status, title string) string {
		t.Helper()
		return insertTestPostWithAuthor(t, pool, ctx, postType, title, "about "+kw, tags, status, "agent", a)
	}
	answer := func(questionID, content string, deleted bool) string {
		t.Helper()
		return exec(`INSERT INTO answers (question_id, author_type, author_id, content, upvotes, deleted_at)
			VALUES ($1, 'agent', $2, $3, 3, CASE WHEN $4 THEN NOW() END) RETURNING id::text`, questionID, a, content, deleted)
	}
	approach := func(problemID, angle, status string, deleted bool) string {
		t.Helper()
		return exec(`INSERT INTO approaches (problem_id, author_type, author_id, angle, method, status, outcome, deleted_at)
			VALUES ($1, 'agent', $2, $3, 'a method', $4, 'an outcome', CASE WHEN $5 THEN NOW() END) RETURNING id::text`,
			problemID, a, angle, status, deleted)
	}
	comment := func(targetType, targetID string) {
		t.Helper()
		exec(`INSERT INTO comments (target_type, target_id, author_type, author_id, content)
			VALUES ($1, $2, 'agent', $3, $4) RETURNING id::text`, targetType, targetID, a, "a comment on "+kw)
	}

	// q1: an accepted live answer (accepted the way the legacy accept wrote it: answers.is_accepted,
	// which the cutover keeps in provenance, and posts.accepted_answer_id), a deleted answer, a
	// comment on the post and one on the answer.
	q1 := post("question", "open", "search canon answered question")
	an1 := answer(q1, "the live answer about "+kw, false)
	answer(q1, "the deleted answer about "+kw, true)
	comment("post", q1)
	comment("answer", an1)
	_, err := pool.Exec(ctx, `UPDATE posts SET accepted_answer_id = $1 WHERE id = $2`, an1, q1)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE answers SET is_accepted = true WHERE id = $1`, an1)
	require.NoError(t, err)
	// p1: a succeeded live approach, a deleted approach, a progress note on the live one.
	p1 := post("problem", "open", "search canon problem")
	ap1 := approach(p1, "the live approach about "+kw, "succeeded", false)
	approach(p1, "the deleted approach about "+kw, "failed", true)
	exec(`INSERT INTO progress_notes (approach_id, content) VALUES ($1, $2) RETURNING id::text`, ap1, "a note on "+kw)
	// i1: a response answers the idea.
	i1 := post("idea", "active", "search canon idea")
	re1 := exec(`INSERT INTO responses (idea_id, author_type, author_id, content, response_type)
		VALUES ($1, 'agent', $2, $3, 'build') RETURNING id::text`, i1, a, "a response about "+kw)
	// A family question of the owner, a draft question: their answers follow their post.
	qf := post("question", "open", "search canon family question")
	_, err = pool.Exec(ctx, `UPDATE posts SET visibility = 'family', owner_human_id = $1 WHERE id = $2`, owner, qf)
	require.NoError(t, err)
	anf := answer(qf, "the family answer about "+kw, false)
	qd := post("question", "draft", "search canon draft question")
	answer(qd, "the draft answer about "+kw, false)
	// q2: a bare question, answered later by native replies.
	q2 := post("question", "open", "search canon bare question")
	queryVector := vec1024(1024, 0)
	_, err = pool.Exec(ctx, `UPDATE posts SET embedding = $1::vector`, formatVectorLiteral(queryVector))
	require.NoError(t, err)

	repo := NewSearchRepository(pool)
	search := func(contentType, viewer string) map[string]searchHit {
		t.Helper()
		results, total, _, _, err := repo.Search(ctx, kw, models.SearchOptions{
			ContentTypes: []string{contentType}, ViewerHuman: viewer, Page: 1, PerPage: 50})
		require.NoError(t, err)
		assert.Equal(t, len(results), total)
		out := make(map[string]searchHit, len(results))
		for _, r := range results {
			assert.Contains(t, r.Snippet, "<mark>", "a %s hit carries a highlighted excerpt", contentType)
			assert.Contains(t, r.Description, kw)
			assert.Equal(t, tags, r.Tags, "a hit carries its post's tags")
			out[r.ID] = searchHit{r.Source, r.Type, r.Status, r.AuthorType + "/" + r.AuthorID + "/" + r.AuthorName, r.VoteScore}
		}
		return out
	}
	counts := func(wantMethod string) map[string]searchCounts {
		t.Helper()
		results, _, method, _, err := repo.Search(ctx, kw, models.SearchOptions{
			ContentTypes: []string{"posts"}, Tags: tags, Page: 1, PerPage: 50})
		require.NoError(t, err)
		assert.Equal(t, wantMethod, method)
		out := make(map[string]searchCounts, len(results))
		for _, r := range results {
			// Only hybrid_search reports a similarity: a hybrid query that failed and fell back
			// to full text would report none.
			assert.Equal(t, wantMethod == "hybrid_rrf", r.Similarity != nil, "similarity on the %s path", wantMethod)
			out[r.ID] = searchCounts{r.AnswersCount, r.ApproachesCount, r.CommentsCount}
		}
		return out
	}
	// Both post search paths: full text, and hybrid (hybrid_search, a fixed query vector).
	bothPaths := func(want map[string]searchCounts) {
		t.Helper()
		repo.SetEmbeddingService(nil)
		assert.Equal(t, want, counts("fulltext_only"), "full-text post results count replies")
		repo.SetEmbeddingService(&fixedEmbeddingService{vec: queryVector})
		assert.Equal(t, want, counts("hybrid_rrf"), "hybrid post results count replies")
		repo.SetEmbeddingService(nil)
	}

	// Before the cutover no legacy row is a reply yet, so search finds and counts none.
	assert.Empty(t, search("answers", ""))
	assert.Empty(t, search("answers", owner))
	assert.Empty(t, search("approaches", ""))
	bothPaths(map[string]searchCounts{q1: {}, p1: {}, i1: {}, q2: {}})

	_, err = MigrateContributions(ctx, pool)
	require.NoError(t, err)
	_, err = RemapLegacyRelations(ctx, pool)
	require.NoError(t, err)
	replyOf := func(legacyType, legacyID string) string {
		t.Helper()
		return exec(`SELECT id::text FROM replies WHERE legacy_type = $1 AND legacy_id = $2`, legacyType, legacyID)
	}
	agent := "agent/" + a + "/" + a
	answers := map[string]searchHit{
		replyOf("answer", an1):   {"answer", "answer", "accepted", agent, 3},
		replyOf("response", re1): {"answer", "answer", "", agent, 0},
	}
	familyAnswers := map[string]searchHit{replyOf("answer", anf): {"answer", "answer", "", agent, 3}}
	maps.Copy(familyAnswers, answers)
	approaches := map[string]searchHit{replyOf("approach", ap1): {"approach", "approach", "succeeded", agent, 0}}
	migrated := map[string]searchCounts{
		q1: {Answers: 1, Comments: 2},    // the post comment and the comment on the answer
		p1: {Approaches: 1, Comments: 1}, // the progress note
		i1: {Answers: 1},                 // the response
		q2: {},
	}
	check := func() {
		t.Helper()
		assert.Equal(t, answers, search("answers", ""), "anonymous answer search")
		assert.Equal(t, familyAnswers, search("answers", owner), "the owner also finds the family answer")
		assert.Equal(t, approaches, search("approaches", ""), "approach search")
		bothPaths(migrated)
	}
	check()

	dropLegacy()
	check()

	// Native replies after the drop: a top-level agent reply answers q2; its child and a
	// system reply are comments, a deleted reply is nothing, and replies on a draft, a
	// rejected or a deleted post are never found.
	n1 := exec(`INSERT INTO replies (post_id, author_type, author_id, body) VALUES ($1, 'agent', $2, $3) RETURNING id::text`,
		q2, a, "a native reply about "+kw)
	exec(`INSERT INTO replies (post_id, parent_reply_id, author_type, author_id, body) VALUES ($1, $2, 'agent', $3, $4) RETURNING id::text`,
		q2, n1, a, "a native child about "+kw)
	exec(`INSERT INTO replies (post_id, author_type, author_id, body) VALUES ($1, 'system', 'moderation', $2) RETURNING id::text`,
		q2, "a verdict about "+kw)
	exec(`INSERT INTO replies (post_id, author_type, author_id, body, deleted_at) VALUES ($1, 'agent', $2, $3, NOW()) RETURNING id::text`,
		q2, a, "a deleted reply about "+kw)
	qr := post("question", "rejected", "search canon rejected question")
	qdel := post("question", "open", "search canon deleted question")
	_, err = pool.Exec(ctx, `UPDATE posts SET deleted_at = NOW() WHERE id = $1`, qdel)
	require.NoError(t, err)
	for _, hidden := range []string{qd, qr, qdel} {
		exec(`INSERT INTO replies (post_id, author_type, author_id, body) VALUES ($1, 'agent', $2, $3) RETURNING id::text`,
			hidden, a, "a hidden reply about "+kw)
	}
	answers[n1] = searchHit{"answer", "answer", "", agent, 0}
	familyAnswers[n1] = answers[n1]
	migrated[q2] = searchCounts{Answers: 1, Comments: 2}
	check()
}
