package db

import (
	"context"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/stretchr/testify/require"
)

// Task idx 76 step 3 (feature:embedding-workers): a reply's embedding describes its
// current body or is absent. Create stores the vector the handler computed, an edit
// replaces it, and an edit without a fresh vector clears it so a stale vector never
// survives a body change (the backfill re-embeds NULLs).

// replyEmbedding reads a reply's stored vector as text ("" when NULL).
func replyEmbedding(t *testing.T, pool *Pool, ctx context.Context, id string) string {
	t.Helper()
	var vec *string
	require.NoError(t, pool.QueryRow(ctx, `SELECT embedding::text FROM replies WHERE id = $1`, id).Scan(&vec))
	if vec == nil {
		return ""
	}
	return *vec
}

func TestReplyRepository_CreateStoresTheBodyEmbedding(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()
	ctx := context.Background()

	sfx := time.Now().Format("150405.000000")
	author := "agent_re_" + sfx
	insertRemapAgent(t, pool, ctx, author)
	defer func() {
		_, _ = pool.Exec(ctx, `DELETE FROM posts WHERE posted_by_id=$1`, author)
		_, _ = pool.Exec(ctx, `DELETE FROM agents WHERE id=$1`, author)
	}()
	postID := insertSearchPost(t, pool, ctx, author, "published", "approved", "public", nil, false)

	repo := NewReplyRepository(pool)
	vec := unitVector(11)
	embedded, err := repo.Create(ctx, &models.Reply{
		PostID: postID, AuthorType: models.AuthorTypeAgent, AuthorID: author,
		Body: "a reply whose body was embedded", EmbeddingStr: &vec,
	})
	require.NoError(t, err)
	require.Equal(t, vec, replyEmbedding(t, pool, ctx, embedded.ID), "Create must store the reply embedding")

	plain, err := repo.Create(ctx, &models.Reply{
		PostID: postID, AuthorType: models.AuthorTypeAgent, AuthorID: author,
		Body: "a reply with no embedding",
	})
	require.NoError(t, err)
	require.Equal(t, "", replyEmbedding(t, pool, ctx, plain.ID), "a reply created without a vector stores NULL")

	// The stored vector is what hybrid_search_replies ranks: a query that matches no
	// word of the body still finds the reply through its embedding.
	order, _ := searchReplies(t, pool, ctx, letterToken("renomatch", sfx), &vec, nil,
		map[string]bool{embedded.ID: true, plain.ID: true})
	require.Equal(t, []string{embedded.ID}, order, "the embedded reply is found by vector; the plain one is not")
}

func TestReplyRepository_UpdateReplacesOrClearsTheEmbedding(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()
	ctx := context.Background()

	sfx := time.Now().Format("150405.000000")
	author := "agent_ru_" + sfx
	insertRemapAgent(t, pool, ctx, author)
	defer func() {
		_, _ = pool.Exec(ctx, `DELETE FROM posts WHERE posted_by_id=$1`, author)
		_, _ = pool.Exec(ctx, `DELETE FROM agents WHERE id=$1`, author)
	}()
	postID := insertSearchPost(t, pool, ctx, author, "published", "approved", "public", nil, false)

	repo := NewReplyRepository(pool)
	first, second := unitVector(12), unitVector(13)
	created, err := repo.Create(ctx, &models.Reply{
		PostID: postID, AuthorType: models.AuthorTypeAgent, AuthorID: author,
		Body: "original body", EmbeddingStr: &first,
	})
	require.NoError(t, err)

	// A non-author edit changes nothing, the vector included.
	_, err = repo.Update(ctx, created.ID, models.AuthorTypeAgent, "intruder", "hacked", &second)
	require.ErrorIs(t, err, ErrReplyForbidden)
	require.Equal(t, first, replyEmbedding(t, pool, ctx, created.ID))

	_, err = repo.Update(ctx, created.ID, models.AuthorTypeAgent, author, "edited body", &second)
	require.NoError(t, err)
	require.Equal(t, second, replyEmbedding(t, pool, ctx, created.ID), "an edit with a fresh vector replaces the old one")

	_, err = repo.Update(ctx, created.ID, models.AuthorTypeAgent, author, "edited again", nil)
	require.NoError(t, err)
	require.Equal(t, "", replyEmbedding(t, pool, ctx, created.ID), "an edit without a fresh vector must clear the stale one")
}
