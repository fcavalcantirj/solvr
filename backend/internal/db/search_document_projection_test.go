package db

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// A post's or reply's stored vector (posts.embedding, replies.embedding) is the semantic search
// document of its text, so it describes the text the row holds now or it is absent (idx 77,
// migration 000122). Measured on HEAD before it (idx 77 slice 6 spike, live API): a post PATCH
// that changed the title and description without a fresh vector (no embedder, or the embedder
// failed) kept the old text's vector, and the translation job's ApplyTranslation kept the
// original-language vector under the English text; only a reply edit cleared it. A missing
// vector is what the backfill (cmd/backfill-embeddings) rebuilds; a wrong one nothing finds.
// The keyword documents are expression indexes over the current text and cannot drift.

type searchDocument struct {
	kind string
	id   string
}

// storedVector reads a row's vector as text ("" when NULL).
func storedVector(ctx context.Context, t *testing.T, pool *Pool, table, id string) string {
	t.Helper()
	var vec *string
	require.NoError(t, pool.QueryRow(ctx, `SELECT embedding::text FROM `+table+` WHERE id = $1`, id).Scan(&vec))
	if vec == nil {
		return ""
	}
	return *vec
}

func readSearchDocumentDrift(ctx context.Context, t *testing.T, pool *Pool) []searchDocument {
	t.Helper()
	rows, err := pool.Query(ctx, `SELECT kind, id::text FROM search_document_drift()`)
	require.NoError(t, err)
	defer rows.Close()
	var out []searchDocument
	for rows.Next() {
		var d searchDocument
		require.NoError(t, rows.Scan(&d.kind, &d.id))
		out = append(out, d)
	}
	require.NoError(t, rows.Err())
	sort.Slice(out, func(i, j int) bool { return out[i].kind+out[i].id < out[j].kind+out[j].id })
	return out
}

func createVectorPost(ctx context.Context, t *testing.T, pool *Pool, author, title string, vec *string) string {
	t.Helper()
	post, err := NewPostRepository(pool).Create(ctx, &models.Post{
		Type: models.PostTypeQuestion, Title: title, Description: title + ", the description of the post",
		Tags: []string{"docs"}, PostedByType: models.AuthorTypeAgent, PostedByID: author,
		Status: models.PostStatusOpen, EmbeddingStr: vec,
	})
	require.NoError(t, err)
	return post.ID
}

func TestSearchDocuments_ATextChangeWithoutAFreshVectorClearsTheStaleOne(t *testing.T) {
	pool, _ := newMigratedScratchDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	author := authorAgent(ctx, t, pool, "docs_"+uuid.NewString()[:8])
	posts := NewPostRepository(pool)
	v1, v2, v3 := unitVector(1), unitVector(2), unitVector(3)

	// The API PATCH path: UpdateIfUnmodified with no vector (COALESCE keeps the stored one).
	edited := createVectorPost(ctx, t, pool, author, "Original text about pgx pool exhaustion", &v1)
	edit := func(mutate func(p *models.Post), vec *string) {
		t.Helper()
		current, err := posts.FindByID(ctx, edited)
		require.NoError(t, err)
		p := current.Post
		mutate(&p)
		p.EmbeddingStr = vec
		_, err = posts.UpdateIfUnmodified(ctx, &p, nil)
		require.NoError(t, err)
	}
	edit(func(p *models.Post) { p.Tags = []string{"docs", "pgx"} }, nil)
	require.Equal(t, v1, storedVector(ctx, t, pool, "posts", edited), "a tags-only edit keeps the vector of the unchanged text")
	edit(func(p *models.Post) { p.Title = "Rewritten text about Redis cache stampede" }, nil)
	require.Equal(t, "", storedVector(ctx, t, pool, "posts", edited), "a title edit without a fresh vector clears the old text's vector")
	edit(func(p *models.Post) { p.Description = "A rewritten description, embedded this time" }, &v2)
	require.Equal(t, v2, storedVector(ctx, t, pool, "posts", edited), "an edit with a fresh vector stores it")
	edit(func(p *models.Post) { p.Description = "Rewritten again while the embedder failed" }, nil)
	require.Equal(t, "", storedVector(ctx, t, pool, "posts", edited), "a description edit without a fresh vector clears it")

	// A status move does not touch the text, so the vector stays.
	status := createVectorPost(ctx, t, pool, author, "A post whose status moves", &v3)
	require.NoError(t, posts.UpdateStatus(ctx, status, models.PostStatusSolved))
	require.Equal(t, v3, storedVector(ctx, t, pool, "posts", status))

	// The translation job rewrites the text in English; the vector was the original language's.
	translated := createVectorPost(ctx, t, pool, author, "Pergunta original sobre pool de conexoes", &v1)
	require.NoError(t, posts.ApplyTranslation(ctx, translated, "Translated question about connection pools",
		"The translated English description"))
	require.Equal(t, "", storedVector(ctx, t, pool, "posts", translated), "a translation clears the original-language vector")

	// Replies: the repository already writes the body and its vector together; any other
	// writer that changes the body without a vector clears it too.
	replies := NewReplyRepository(pool)
	reply, err := replies.Create(ctx, &models.Reply{PostID: status, AuthorType: models.AuthorTypeAgent, AuthorID: author,
		Body: "A reply body about pgx pool sizing", EmbeddingStr: &v1})
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE replies SET body = 'A reply body rewritten by another writer' WHERE id = $1`, reply.ID)
	require.NoError(t, err)
	require.Equal(t, "", storedVector(ctx, t, pool, "replies", reply.ID), "a body change without a vector clears the reply's")
	_, err = replies.Update(ctx, reply.ID, models.AuthorTypeAgent, author, "A reply body embedded again", &v2, nil)
	require.NoError(t, err)
	require.Equal(t, v2, storedVector(ctx, t, pool, "replies", reply.ID))
	_, err = pool.Exec(ctx, `UPDATE replies SET updated_at = NOW() WHERE id = $1`, reply.ID)
	require.NoError(t, err)
	require.Equal(t, v2, storedVector(ctx, t, pool, "replies", reply.ID), "a write that keeps the body keeps the vector")
	_, err = pool.Exec(ctx, `UPDATE replies SET body = body WHERE id = $1`, reply.ID)
	require.NoError(t, err)
	require.Equal(t, v2, storedVector(ctx, t, pool, "replies", reply.ID), "rewriting the same body keeps the vector")
}

func TestSearchDocuments_DriftListsTheLiveRowsTheBackfillMustEmbed(t *testing.T) {
	pool, _ := newMigratedScratchDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	author := authorAgent(ctx, t, pool, "drift_"+uuid.NewString()[:8])
	vec := unitVector(4)

	require.Empty(t, readSearchDocumentDrift(ctx, t, pool))
	embedded := createVectorPost(ctx, t, pool, author, "An embedded post", &vec)
	bare := createVectorPost(ctx, t, pool, author, "A post the embedder never reached", nil)
	gone := createVectorPost(ctx, t, pool, author, "A deleted post without a vector", nil)
	require.NoError(t, NewPostRepository(pool).Delete(ctx, gone))

	bareReply := insertSearchReply(t, pool, ctx, embedded, author, "a reply without a vector", nil, false)
	insertSearchReply(t, pool, ctx, embedded, author, "an embedded reply", &vec, false)
	insertSearchReply(t, pool, ctx, embedded, author, "a deleted reply without a vector", nil, true)
	_, err := pool.Exec(ctx, `INSERT INTO replies (post_id, author_type, author_id, body)
		VALUES ($1, 'system', 'moderation', 'a system verdict is not knowledge')`, embedded)
	require.NoError(t, err)

	want := []searchDocument{{"post", bare}, {"reply", bareReply}}
	sort.Slice(want, func(i, j int) bool { return want[i].kind+want[i].id < want[j].kind+want[j].id })
	require.Equal(t, want, readSearchDocumentDrift(ctx, t, pool))

	// A text edit without a vector puts the row on the list; embedding it takes it off.
	_, err = pool.Exec(ctx, `UPDATE posts SET title = 'An embedded post, retitled' WHERE id = $1`, embedded)
	require.NoError(t, err)
	require.Contains(t, readSearchDocumentDrift(ctx, t, pool), searchDocument{"post", embedded})
	_, err = pool.Exec(ctx, `UPDATE posts SET embedding = $2::vector WHERE id = ANY($1::uuid[])`, []string{embedded, bare}, vec)
	require.NoError(t, err)
	require.Equal(t, []searchDocument{{"reply", bareReply}}, readSearchDocumentDrift(ctx, t, pool))
}

// Migration 000122 clears the vector of every post the translation job rewrote: it was
// computed from the original-language text at creation, before the translation. It moves
// no other vector and no updated_at (the If-Match validator).
func TestSearchDocuments_TheMigrationClearsTheVectorsOfTranslatedText(t *testing.T) {
	pool, _ := newMigratedScratchDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	migration := func(direction string) {
		t.Helper()
		sql, err := os.ReadFile(filepath.Join(backendRoot(t), "migrations", "000122_search_document_projection."+direction+".sql"))
		require.NoError(t, err)
		_, err = pool.Exec(ctx, string(sql))
		require.NoError(t, err, "000122 %s", direction)
	}
	migration("down")
	author := authorAgent(ctx, t, pool, "mig_"+uuid.NewString()[:8])
	vec := unitVector(5)
	post := func(title, originalTitle string) string {
		t.Helper()
		id := createVectorPost(ctx, t, pool, author, title, &vec)
		if originalTitle != "" {
			_, err := pool.Exec(ctx, `UPDATE posts SET original_title = $2, original_description = $3 WHERE id = $1`,
				id, originalTitle, originalTitle+", the description of the post")
			require.NoError(t, err)
		}
		return id
	}
	translated := post("Translated question about pools", "Pergunta sobre pools")
	sameText := post("A title the translator returned unchanged", "A title the translator returned unchanged")
	plain := post("A post that was never translated", "")
	stamps := func() string {
		t.Helper()
		var s string
		require.NoError(t, pool.QueryRow(ctx, `SELECT string_agg(id::text || ':' || updated_at::text, ',' ORDER BY id) FROM posts`).Scan(&s))
		return s
	}
	before := stamps()
	migration("up")

	require.Equal(t, "", storedVector(ctx, t, pool, "posts", translated), "the vector of the original-language text is cleared")
	require.Equal(t, vec, storedVector(ctx, t, pool, "posts", sameText), "a translation that changed nothing keeps its vector")
	require.Equal(t, vec, storedVector(ctx, t, pool, "posts", plain))
	require.Equal(t, before, stamps(), "the migration moves no updated_at")
	require.Equal(t, []searchDocument{{"post", translated}}, readSearchDocumentDrift(ctx, t, pool))

	migration("down")
	_, err := pool.Exec(ctx, `UPDATE posts SET title = 'Edited after the down migration' WHERE id = $1`, plain)
	require.NoError(t, err)
	require.Equal(t, vec, storedVector(ctx, t, pool, "posts", plain), "down removes the trigger (the pre-122 behavior)")
	migration("up")
	require.Equal(t, vec, storedVector(ctx, t, pool, "posts", plain), "up does not guess at untranslated rows")
}
