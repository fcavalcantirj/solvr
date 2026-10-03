package db

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// wantAnchor is what a reply anchor must say about the contribution it came from.
type wantAnchor struct {
	Reply, AuthorID, Author, AuthorType, LegacyType, LegacyStatus string
	CreatedAt                                                     time.Time
}

// Task idx 53 steps 1-3 (and the result half of step 5): after the knowledge cutover, the
// DEFAULT search — no type filter, no content_types — finds knowledge by text that used to
// live in a problem, question, idea, answer, response, or succeeded or failed approach. Every
// result is a canonical post, once; a contribution match travels with its post as a reply
// anchor with a link, a highlighted excerpt, and its original author, time and origin. Replies
// on draft, rejected, deleted or (for others) family posts, deleted replies and system replies
// are never found: not as a result, not in the total, not in top_similarity. Both the full-text
// and the hybrid path (fixed query vector) are checked.
func TestCanonicalSearch_DefaultResolvesKnowledgeToPostsAndReplyAnchors(t *testing.T) {
	pool, archiveLegacy := newPreArchiveScratchDatabase(t)
	ctx := context.Background()
	scan := func(sql string, args ...any) string {
		t.Helper()
		var id string
		require.NoError(t, pool.QueryRow(ctx, sql, args...).Scan(&id), sql)
		return id
	}
	poster, contributor := "agent_ks_poster", "agent_ks_contributor"
	insertRemapAgent(t, pool, ctx, poster)
	insertRemapAgent(t, pool, ctx, contributor)
	human := scan(`INSERT INTO users (username, display_name, email, referral_code)
		VALUES ('ksanswerer', 'KS Answerer', 'ksanswerer@example.com', 'KSANSWR1') RETURNING id::text`)
	owner := scan(`INSERT INTO users (username, display_name, email, referral_code)
		VALUES ('ksowner', 'KS Owner', 'ksowner@example.com', 'KSOWNER1') RETURNING id::text`)
	post := func(postType, status, title string) string {
		t.Helper()
		return insertTestPostWithAuthor(t, pool, ctx, postType, title, "the body of "+title, nil, status, "agent", poster)
	}
	at := func(day int) time.Time { return time.Date(2025, 1, day, 3, 4, 5, 0, time.UTC) }
	answer := func(questionID, authorType, authorID, content string, day int, deleted bool) string {
		t.Helper()
		return scan(`INSERT INTO answers (question_id, author_type, author_id, content, created_at, deleted_at)
			VALUES ($1, $2, $3, $4, $5, CASE WHEN $6 THEN NOW() END) RETURNING id::text`,
			questionID, authorType, authorID, content, at(day), deleted)
	}
	approach := func(problemID, angle, status string, day int) string {
		t.Helper()
		return scan(`INSERT INTO approaches (problem_id, author_type, author_id, angle, method, status, outcome, created_at)
			VALUES ($1, 'agent', $2, $3, 'a method', $4, 'an outcome', $5) RETURNING id::text`,
			problemID, contributor, angle, status, at(day))
	}

	// Visible knowledge, one distinct word per piece of text.
	problem := post("problem", "open", "kwproblem crashes on start")
	succeeded := approach(problem, "kwsucceeded pin the version", "succeeded", 2)
	failed := approach(problem, "kwfailed clear the cache", "failed", 3)
	question := post("question", "open", "kwquestion how to cache")
	answered := answer(question, "human", human, "kwanswer use a shared cache", 4, false)
	idea := post("idea", "active", "kwidea a cache dashboard")
	response := scan(`INSERT INTO responses (idea_id, author_type, author_id, content, response_type, created_at)
		VALUES ($1, 'agent', $2, 'kwresponse I would build it', 'build', $3) RETURNING id::text`, idea, contributor, at(5))
	semanticPost := post("question", "open", "an unrelated question")
	semantic := answer(semanticPost, "agent", contributor, "an unrelated remark", 6, false)

	// Hidden knowledge: every hidden reply says kwhidden.
	var hiddenAnswers []string
	for _, status := range []string{"draft", "rejected", "pending_review"} {
		hiddenAnswers = append(hiddenAnswers, answer(post("question", status, "a "+status+" question"),
			"agent", contributor, "kwhidden on a "+status+" post", 7, false))
	}
	deletedPost := post("question", "open", "a deleted question")
	hiddenAnswers = append(hiddenAnswers, answer(deletedPost, "agent", contributor, "kwhidden on a deleted post", 7, false))
	_, err := pool.Exec(ctx, `UPDATE posts SET deleted_at = NOW() WHERE id = $1`, deletedPost)
	require.NoError(t, err)
	hiddenAnswers = append(hiddenAnswers, answer(question, "agent", contributor, "kwhidden deleted answer", 7, true))
	familyPost := post("question", "open", "a family question")
	_, err = pool.Exec(ctx, `UPDATE posts SET visibility = 'family', owner_human_id = $1 WHERE id = $2`, owner, familyPost)
	require.NoError(t, err)
	family := answer(familyPost, "agent", contributor, "kwfamily the family answer", 8, false)

	_, err = RunKnowledgeCutover(ctx, pool, KnowledgeCutoverOptions{})
	require.NoError(t, err)
	archiveLegacy() // production order: seed, cutover, archive; search runs at head
	replyOf := func(legacyType, legacyID string) string {
		t.Helper()
		return scan(`SELECT id::text FROM replies WHERE legacy_type = $1 AND legacy_id = $2`, legacyType, legacyID)
	}
	systemReply := scan(`INSERT INTO replies (post_id, author_type, author_id, body)
		VALUES ($1, 'system', 'moderation', 'kwhidden a moderation verdict') RETURNING id::text`, question)

	// Embeddings: the query vector is all ones. Every hidden reply IS the query vector
	// (similarity 1), so a leak would top the results and top_similarity; the one visible
	// embedded reply has similarity 768/sqrt(1024*768) = 0.866 and no query word.
	queryVector := vec1024(1024, 0)
	hiddenReplies := []string{systemReply, replyOf("answer", family)}
	for _, a := range hiddenAnswers {
		hiddenReplies = append(hiddenReplies, replyOf("answer", a))
	}
	_, err = pool.Exec(ctx, `UPDATE replies SET embedding = $1::vector WHERE id = ANY($2::uuid[])`,
		formatVectorLiteral(queryVector), hiddenReplies)
	require.NoError(t, err)
	semanticReply := replyOf("answer", semantic)
	_, err = pool.Exec(ctx, `UPDATE replies SET embedding = $1::vector WHERE id = $2`,
		formatVectorLiteral(vec1024(768, 0)), semanticReply)
	require.NoError(t, err)
	const semanticSimilarity = 0.8660254

	contributorAnchor := func(reply, legacyType, legacyStatus string, day int) wantAnchor {
		return wantAnchor{Reply: reply, AuthorID: contributor, Author: contributor, AuthorType: "agent",
			LegacyType: legacyType, LegacyStatus: legacyStatus, CreatedAt: at(day)}
	}
	semanticAnchor := contributorAnchor(semanticReply, "answer", "", 6)
	familyAnchor := contributorAnchor(replyOf("answer", family), "answer", "", 8)
	answerAnchor := wantAnchor{Reply: replyOf("answer", answered), AuthorID: human, Author: "KS Answerer",
		AuthorType: "human", LegacyType: "answer", CreatedAt: at(4)}

	repo := NewSearchRepository(pool)
	check := func(path, query, viewer string, want map[string][]wantAnchor) {
		t.Helper()
		// In hybrid search the embedded replies the viewer may read match every query by
		// meaning: the visible one always, the family one for its owner.
		wantTop := semanticSimilarity
		if path == "hybrid_rrf" {
			add := func(postID string, a wantAnchor) {
				for _, have := range want[postID] {
					if have.Reply == a.Reply {
						return
					}
				}
				want[postID] = append(want[postID], a)
			}
			add(semanticPost, semanticAnchor)
			if viewer == owner {
				add(familyPost, familyAnchor)
				wantTop = 1
			}
		}
		results, total, method, top, err := repo.Search(ctx, query, models.SearchOptions{
			ViewerHuman: viewer, Page: 1, PerPage: 50})
		require.NoError(t, err)
		assert.Equal(t, path, method, query)
		assert.Equal(t, len(want), total, "%s %q: total counts posts, once each", path, query)
		got := make(map[string][]wantAnchor, len(results))
		for _, r := range results {
			assert.NotContains(t, got, r.ID, "%s %q: post %s listed twice", path, query, r.ID)
			assert.Equal(t, "post", r.Source)
			got[r.ID] = []wantAnchor{}
			for _, m := range r.MatchedReplies {
				assert.Equal(t, r.ID, m.PostID)
				assert.Equal(t, "/posts/"+r.ID+"#"+m.ID, m.URL)
				assert.NotEmpty(t, m.Snippet)
				if path == "fulltext_only" {
					assert.Contains(t, m.Snippet, "<mark>", "%s %q: a word match is highlighted", path, query)
				}
				got[r.ID] = append(got[r.ID], wantAnchor{Reply: m.ID, AuthorID: m.Author.ID,
					Author: m.Author.DisplayName, AuthorType: m.Author.Type, LegacyType: deref(m.LegacyType),
					LegacyStatus: deref(m.LegacyStatus), CreatedAt: m.CreatedAt.UTC()})
			}
		}
		if path == "hybrid_rrf" {
			require.NotNil(t, top, "%s %q: the embedded visible reply is always a semantic match", path, query)
			assert.InDelta(t, wantTop, *top, 1e-4, "%s %q: no hidden reply reaches top_similarity", path, query)
		} else {
			assert.Nil(t, top)
		}
		for _, m := range []map[string][]wantAnchor{want, got} {
			for id := range m {
				sort.Slice(m[id], func(i, j int) bool { return m[id][i].Reply < m[id][j].Reply })
			}
		}
		assert.Equal(t, want, got, "%s %q", path, query)
	}

	for _, path := range []string{"fulltext_only", "hybrid_rrf"} {
		if path == "hybrid_rrf" {
			repo.SetEmbeddingService(&fixedEmbeddingService{vec: queryVector})
		}
		check(path, "kwproblem", "", map[string][]wantAnchor{problem: {}})
		check(path, "kwquestion", "", map[string][]wantAnchor{question: {}})
		check(path, "kwidea", "", map[string][]wantAnchor{idea: {}})
		check(path, "kwsucceeded", "", map[string][]wantAnchor{
			problem: {contributorAnchor(replyOf("approach", succeeded), "approach", "succeeded", 2)}})
		check(path, "kwfailed", "", map[string][]wantAnchor{
			problem: {contributorAnchor(replyOf("approach", failed), "approach", "failed", 3)}})
		check(path, "kwanswer", "", map[string][]wantAnchor{question: {answerAnchor}})
		check(path, "kwresponse", "", map[string][]wantAnchor{
			idea: {contributorAnchor(replyOf("response", response), "response", "", 5)}})
		// A post matched by its own text and a reply's is one result with the anchor.
		check(path, "kwquestion kwanswer", "", map[string][]wantAnchor{question: {answerAnchor}})
		// Two replies of one post: one result, two anchors.
		check(path, "kwsucceeded kwfailed", "", map[string][]wantAnchor{problem: {
			contributorAnchor(replyOf("approach", succeeded), "approach", "succeeded", 2),
			contributorAnchor(replyOf("approach", failed), "approach", "failed", 3)}})
		check(path, "kwhidden", "", map[string][]wantAnchor{})
		check(path, "kwhidden", owner, map[string][]wantAnchor{})
		check(path, "kwfamily", "", map[string][]wantAnchor{})
		check(path, "kwfamily", human, map[string][]wantAnchor{})
		check(path, "kwfamily", owner, map[string][]wantAnchor{familyPost: {familyAnchor}})
		check(path, "kwsemanticonly", "", map[string][]wantAnchor{})
	}
}
