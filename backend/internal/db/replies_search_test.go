package db

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// unitVector is a 1024-dim pgvector literal with a single 1 at position axis: two
// different axes are orthogonal (cosine distance 1), the same axis is identical (0).
func unitVector(axis int) string {
	parts := make([]string, 1024)
	for i := range parts {
		parts[i] = "0"
	}
	parts[axis] = "1"
	return "[" + strings.Join(parts, ",") + "]"
}

// letterToken turns the digits of s into letters so the token is one plain English lexeme.
func letterToken(prefix, s string) string {
	return prefix + strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return 'a' + (r - '0')
		}
		return -1
	}, s)
}

func insertSearchPost(t *testing.T, pool *Pool, ctx context.Context, author, pub, mod, visibility string, owner *string, deleted bool) string {
	t.Helper()
	var deletedAt any
	if deleted {
		deletedAt = time.Now().UTC()
	}
	var id string
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO posts (type, title, description, tags, status, posted_by_type, posted_by_id,
			publication_state, moderation_state, visibility, owner_human_id, deleted_at)
		VALUES ('post', 'rs post', 'rs body', ARRAY['rs'], 'open', 'agent', $1, $2, $3, $4, $5, $6)
		RETURNING id::text`, author, pub, mod, visibility, owner, deletedAt).Scan(&id))
	return id
}

func insertSearchReply(t *testing.T, pool *Pool, ctx context.Context, postID, author, body string, embedding *string, deleted bool) string {
	t.Helper()
	var deletedAt any
	if deleted {
		deletedAt = time.Now().UTC()
	}
	var id string
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO replies (post_id, author_type, author_id, body, embedding, deleted_at)
		VALUES ($1, 'agent', $2, $3, $4::vector, $5) RETURNING id::text`,
		postID, author, body, embedding, deletedAt).Scan(&id))
	return id
}

type replyHit struct {
	postID string
	score  float64
}

// searchReplies calls hybrid_search_replies and keeps only the replies in mine, in rank order.
func searchReplies(t *testing.T, pool *Pool, ctx context.Context, tsquery string, embedding *string, viewer *string, mine map[string]bool) ([]string, map[string]replyHit) {
	t.Helper()
	rows, err := pool.Query(ctx, `SELECT reply_id::text, post_id::text, rrf_score
		FROM hybrid_search_replies($1, $2::vector, 200, 1.0, 1.0, 60, $3::uuid)`, tsquery, embedding, viewer)
	require.NoError(t, err)
	defer rows.Close()
	var order []string
	hits := map[string]replyHit{}
	for rows.Next() {
		var id string
		var h replyHit
		require.NoError(t, rows.Scan(&id, &h.postID, &h.score))
		if mine[id] {
			order = append(order, id)
			hits[id] = h
		}
	}
	require.NoError(t, rows.Err())
	return order, hits
}

// hybrid_search_replies is the one replies search function that replaces
// hybrid_search_answers and hybrid_search_approaches (task idx 76 step 5): it fuses full
// text over the reply body with vector similarity over the reply embedding, and only
// returns live replies under posts the viewer may read under the canonical rule
// (published, moderation-approved, public or the viewer's family, not deleted).
func TestHybridSearchReplies_FusesTextAndVectorOverEligibleReplies(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()
	ctx := context.Background()

	sfx := time.Now().Format("150405.000000")
	token := letterToken("rsq", sfx)
	author := "agent_rs_" + sfx
	insertRemapAgent(t, pool, ctx, author)

	var owner, stranger string
	for i, u := range []*string{&owner, &stranger} {
		name := letterToken("rsu", sfx) + string(rune('a'+i))
		require.NoError(t, pool.QueryRow(ctx, `INSERT INTO users (username, display_name, email, referral_code)
			VALUES ($1::text, $1::text, $1::text || '@example.com', upper(substr(md5($1::text), 1, 8))) RETURNING id::text`, name).Scan(u))
	}
	defer func() {
		_, _ = pool.Exec(ctx, `DELETE FROM posts WHERE posted_by_id=$1`, author)
		_, _ = pool.Exec(ctx, `DELETE FROM agents WHERE id=$1`, author)
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE id IN ($1::uuid, $2::uuid)`, owner, stranger)
	}()

	near, far := unitVector(7), unitVector(8)
	pub := insertSearchPost(t, pool, ctx, author, "published", "approved", "public", nil, false)
	fam := insertSearchPost(t, pool, ctx, author, "published", "approved", "family", &owner, false)
	draft := insertSearchPost(t, pool, ctx, author, "draft", "pending", "public", nil, false)
	pending := insertSearchPost(t, pool, ctx, author, "published", "pending", "public", nil, false)
	rejected := insertSearchPost(t, pool, ctx, author, "draft", "rejected", "public", nil, false)
	archived := insertSearchPost(t, pool, ctx, author, "archived", "approved", "public", nil, false)
	gone := insertSearchPost(t, pool, ctx, author, "published", "approved", "public", nil, true)

	text := "a failed attempt mentioning " + token + " in the body"
	both := insertSearchReply(t, pool, ctx, pub, author, text, &near, false)
	textOnly := insertSearchReply(t, pool, ctx, pub, author, text, nil, false)
	vecOnly := insertSearchReply(t, pool, ctx, pub, author, "unrelated words", &near, false)
	farVec := insertSearchReply(t, pool, ctx, pub, author, "other words", &far, false)
	deletedReply := insertSearchReply(t, pool, ctx, pub, author, text, &near, true)
	family := insertSearchReply(t, pool, ctx, fam, author, text, &near, false)
	hidden := map[string]string{}
	for name, post := range map[string]string{"draft": draft, "pending": pending, "rejected": rejected, "archived": archived, "deleted post": gone} {
		hidden[insertSearchReply(t, pool, ctx, post, author, text, &near, false)] = name
	}

	mine := map[string]bool{both: true, textOnly: true, vecOnly: true, farVec: true, deletedReply: true, family: true}
	for id := range hidden {
		mine[id] = true
	}

	// Anonymous: public live replies only; text and vector matches are fused.
	order, hits := searchReplies(t, pool, ctx, token, &near, nil, mine)
	assert.ElementsMatch(t, []string{both, textOnly, vecOnly}, order)
	require.NotEmpty(t, order)
	assert.Equal(t, both, order[0], "a reply matching text and vector ranks first")
	assert.Greater(t, hits[both].score, hits[textOnly].score)
	assert.Greater(t, hits[both].score, hits[vecOnly].score)
	for _, id := range order {
		assert.Equal(t, pub, hits[id].postID, "reply %s resolves to its post", id)
		assert.Greater(t, hits[id].score, 0.0, "reply %s is ranked by what it matched", id)
	}

	// The owner's family sees its family-scoped reply; a stranger does not.
	order, hits = searchReplies(t, pool, ctx, token, &near, &owner, mine)
	assert.ElementsMatch(t, []string{both, textOnly, vecOnly, family}, order)
	assert.Equal(t, fam, hits[family].postID)
	order, _ = searchReplies(t, pool, ctx, token, &near, &stranger, mine)
	assert.ElementsMatch(t, []string{both, textOnly, vecOnly}, order)

	// Without a query embedding the function is full text only.
	order, _ = searchReplies(t, pool, ctx, token, nil, nil, mine)
	assert.ElementsMatch(t, []string{both, textOnly}, order)

	for id, name := range hidden {
		assert.NotContains(t, order, id, "reply under a %s post", name)
	}
}

// The cutover copies each legacy answer/approach embedding onto the reply migrated from
// it, so migrated content stays semantically searchable through hybrid_search_replies
// without re-embedding; it never overwrites a reply's own embedding and a second run
// changes nothing (task idx 76 step 5).
func TestRemapLegacyRelations_CopiesLegacyEmbeddingsOntoReplies(t *testing.T) {
	pool, _ := newPreArchiveScratchDatabase(t) // the cutover tool runs below the legacy archive
	defer pool.Close()
	ctx := context.Background()

	sfx := time.Now().Format("150405.000000")
	agent := "agent_lre_" + sfx
	insertRemapAgent(t, pool, ctx, agent)
	defer func() {
		for _, q := range []string{
			`DELETE FROM approaches WHERE author_id=$1`, `DELETE FROM answers WHERE author_id=$1`,
			`DELETE FROM posts WHERE posted_by_id=$1`, `DELETE FROM agents WHERE id=$1`,
		} {
			_, _ = pool.Exec(ctx, q, agent)
		}
	}()

	probID := insertTestPostWithAuthor(t, pool, ctx, "problem", "lre problem "+sfx, "body", []string{"lre"}, "open", "agent", agent)
	qID := insertTestPostWithAuthor(t, pool, ctx, "question", "lre question "+sfx, "body", []string{"lre"}, "open", "agent", agent)
	_, err := pool.Exec(ctx, `UPDATE posts SET publication_state='published', moderation_state='approved' WHERE id IN ($1,$2)`, probID, qID)
	require.NoError(t, err)

	apprVec, ansVec, ownVec := unitVector(11), unitVector(12), unitVector(13)
	appr := insertApproach(t, pool, ctx, probID, agent, "failed", false)
	_, err = pool.Exec(ctx, `UPDATE approaches SET embedding=$2::vector WHERE id=$1`, appr, apprVec)
	require.NoError(t, err)
	var ans, ansNoVec, ansOwn string
	for _, a := range []struct {
		id  *string
		vec *string
	}{{&ans, &ansVec}, {&ansNoVec, nil}, {&ansOwn, &ansVec}} {
		require.NoError(t, pool.QueryRow(ctx, `INSERT INTO answers (question_id, author_type, author_id, content, embedding)
			VALUES ($1,'agent',$2,'an answer',$3::vector) RETURNING id`, qID, agent, a.vec).Scan(a.id))
	}

	_, err = MigrateContributions(ctx, pool)
	require.NoError(t, err)
	rOwn, _, _, _ := replyFor(t, pool, ctx, "answer", ansOwn)
	_, err = pool.Exec(ctx, `UPDATE replies SET embedding=$2::vector WHERE id=$1`, rOwn, ownVec)
	require.NoError(t, err)

	report, err := RemapLegacyRelations(ctx, pool)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, report.Embeddings, int64(2))

	embeddingOf := func(replyID string) *string {
		var v *string
		require.NoError(t, pool.QueryRow(ctx, `SELECT embedding::text FROM replies WHERE id=$1`, replyID).Scan(&v))
		return v
	}
	rAppr, _, _, _ := replyFor(t, pool, ctx, "approach", appr)
	rAns, _, _, _ := replyFor(t, pool, ctx, "answer", ans)
	rNoVec, _, _, _ := replyFor(t, pool, ctx, "answer", ansNoVec)
	require.NotNil(t, embeddingOf(rAppr))
	assert.Equal(t, apprVec, *embeddingOf(rAppr))
	require.NotNil(t, embeddingOf(rAns))
	assert.Equal(t, ansVec, *embeddingOf(rAns))
	assert.Nil(t, embeddingOf(rNoVec), "an answer without an embedding stays without one")
	assert.Equal(t, ownVec, *embeddingOf(rOwn), "a reply's own embedding is never overwritten")

	// The migrated approach is now found by its legacy embedding alone.
	order, hits := searchReplies(t, pool, ctx, "zzznomatchzzz", &apprVec, nil, map[string]bool{rAppr: true})
	assert.Equal(t, []string{rAppr}, order)
	assert.Equal(t, probID, hits[rAppr].postID)

	again, err := RemapLegacyRelations(ctx, pool)
	require.NoError(t, err)
	assert.Zero(t, again.Embeddings, "second run copies nothing")
}

// The legacy search functions and vector indexes left public with the legacy archive
// migration only after their replacement (replies embedding, HNSW index,
// hybrid_search_replies) exists in the schema, and the registry no longer lists them.
func TestLegacySearchObjects_LeavePublicAfterTheirReplacement(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()
	ctx := context.Background()

	var fn, idx, legacy int
	require.NoError(t, pool.QueryRow(ctx, `SELECT COUNT(*) FROM pg_proc WHERE proname='hybrid_search_replies'`).Scan(&fn))
	require.NoError(t, pool.QueryRow(ctx, `SELECT COUNT(*) FROM pg_indexes
		WHERE tablename='replies' AND indexname='idx_replies_embedding' AND indexdef LIKE '%hnsw%'`).Scan(&idx))
	require.Equal(t, 1, fn, "hybrid_search_replies must exist")
	require.Equal(t, 1, idx, "replies must have an HNSW embedding index")
	require.NoError(t, pool.QueryRow(ctx, `SELECT
		(SELECT COUNT(*) FROM pg_proc WHERE pronamespace = 'public'::regnamespace
			AND proname IN ('hybrid_search_answers', 'hybrid_search_approaches')) +
		(SELECT COUNT(*) FROM pg_indexes WHERE schemaname = 'public'
			AND indexname IN ('idx_answers_embedding', 'idx_approaches_embedding'))`).Scan(&legacy))
	assert.Zero(t, legacy, "the legacy search functions and indexes left public")

	for _, key := range []string{
		"function:hybrid_search_answers", "function:hybrid_search_approaches",
		"index:answers.idx_answers_embedding", "index:approaches.idx_approaches_embedding",
	} {
		assert.NotContains(t, LegacyDependencyDispositions, key)
	}
}
