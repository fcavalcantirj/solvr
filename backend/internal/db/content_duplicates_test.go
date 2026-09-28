package db

import (
	"context"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// Task idx 76 step 3 (feature:duplicate-detection): duplicate detection reads the
// canonical posts and replies tables. A post repeats an earlier live post with the same
// title and description; a reply repeats an earlier live, non-system reply with the same
// body on the same post. Only rows created inside the window count, and the row being
// checked is excluded.

func insertDuplicatePost(t *testing.T, pool *Pool, ctx context.Context, author, title, description string, age time.Duration, deleted bool) string {
	t.Helper()
	created := time.Now().UTC().Add(-age)
	var deletedAt any
	if deleted {
		deletedAt = created
	}
	var id string
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO posts (type, title, description, tags, status, posted_by_type, posted_by_id,
			publication_state, moderation_state, visibility, created_at, updated_at, deleted_at)
		VALUES ('post', $1, $2, ARRAY['dup'], 'open', 'agent', $3,
			'published', 'approved', 'public', $4, $4, $5)
		RETURNING id::text`, title, description, author, created, deletedAt).Scan(&id))
	return id
}

func insertDuplicateReply(t *testing.T, pool *Pool, ctx context.Context, postID, authorType, author, body string, age time.Duration, deleted bool) string {
	t.Helper()
	created := time.Now().UTC().Add(-age)
	var deletedAt any
	if deleted {
		deletedAt = created
	}
	var id string
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO replies (post_id, author_type, author_id, body, created_at, updated_at, deleted_at)
		VALUES ($1, $2, $3, $4, $5, $5, $6) RETURNING id::text`,
		postID, authorType, author, body, created, deletedAt).Scan(&id))
	return id
}

func TestContentDuplicateRepository_FindPost(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()
	ctx := context.Background()

	author := "agent_dup_" + time.Now().Format("150405.000000")
	insertRemapAgent(t, pool, ctx, author)
	defer func() {
		_, _ = pool.Exec(ctx, `DELETE FROM posts WHERE posted_by_id=$1`, author)
		_, _ = pool.Exec(ctx, `DELETE FROM agents WHERE id=$1`, author)
	}()

	title := "Duplicate probe " + uuid.NewString()
	desc := "The same description posted more than once " + uuid.NewString()
	stale := insertDuplicatePost(t, pool, ctx, author, title, desc, 30*time.Hour, false)
	_ = insertDuplicatePost(t, pool, ctx, author, title, desc, 3*time.Hour, true)
	original := insertDuplicatePost(t, pool, ctx, author, title, desc, 2*time.Hour, false)
	_ = insertDuplicatePost(t, pool, ctx, author, title, desc+" but edited", 90*time.Minute, false)
	newer := insertDuplicatePost(t, pool, ctx, author, title, desc, 0, false)

	repo := NewContentDuplicateRepository(pool)
	since := time.Now().Add(-24 * time.Hour)

	match, err := repo.FindPost(ctx, title, desc, since, newer)
	require.NoError(t, err)
	require.NotNil(t, match, "the new post repeats an earlier live post in the window")
	require.Equal(t, models.ContentDuplicate{
		TargetType: "post", TargetID: original, PostID: original, CreatedAt: match.CreatedAt,
	}, *match, "the earliest live match in the window, skipping the stale and the deleted post")
	require.WithinDuration(t, time.Now().Add(-2*time.Hour), match.CreatedAt, time.Minute)

	match, err = repo.FindPost(ctx, title, desc, since, original)
	require.NoError(t, err)
	require.NotNil(t, match)
	require.Equal(t, newer, match.TargetID, "the checked post itself is excluded, never another match")

	match, err = repo.FindPost(ctx, title, desc, time.Now().Add(-48*time.Hour), newer)
	require.NoError(t, err)
	require.NotNil(t, match)
	require.Equal(t, stale, match.TargetID, "a wider window reaches the older post")

	match, err = repo.FindPost(ctx, title, desc+" but different", since, "")
	require.NoError(t, err)
	require.Nil(t, match, "a different description is not a duplicate")
}

func TestContentDuplicateRepository_FindReply(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()
	ctx := context.Background()

	author := "agent_dupr_" + time.Now().Format("150405.000000")
	insertRemapAgent(t, pool, ctx, author)
	defer func() {
		_, _ = pool.Exec(ctx, `DELETE FROM posts WHERE posted_by_id=$1`, author)
		_, _ = pool.Exec(ctx, `DELETE FROM agents WHERE id=$1`, author)
	}()

	postA := insertDuplicatePost(t, pool, ctx, author, "dup reply post A "+uuid.NewString(), "a", time.Hour*5, false)
	postB := insertDuplicatePost(t, pool, ctx, author, "dup reply post B "+uuid.NewString(), "b", time.Hour*5, false)
	body := "The same reply body posted more than once " + uuid.NewString()

	_ = insertDuplicateReply(t, pool, ctx, postA, "agent", author, body, 30*time.Hour, false)
	_ = insertDuplicateReply(t, pool, ctx, postA, "system", "solvr-moderation", body, 3*time.Hour, false)
	_ = insertDuplicateReply(t, pool, ctx, postA, "agent", author, body, 150*time.Minute, true)
	original := insertDuplicateReply(t, pool, ctx, postA, "agent", author, body, 2*time.Hour, false)
	onB := insertDuplicateReply(t, pool, ctx, postB, "agent", author, body, 4*time.Hour, false)
	newer := insertDuplicateReply(t, pool, ctx, postA, "agent", author, body, 0, false)

	repo := NewContentDuplicateRepository(pool)
	since := time.Now().Add(-24 * time.Hour)

	match, err := repo.FindReply(ctx, postA, body, since, newer)
	require.NoError(t, err)
	require.NotNil(t, match, "the new reply repeats an earlier live reply on the same post")
	require.Equal(t, models.ContentDuplicate{
		TargetType: "reply", TargetID: original, PostID: postA, CreatedAt: match.CreatedAt,
	}, *match, "skips the stale, the system and the deleted reply and the other post's reply")

	match, err = repo.FindReply(ctx, postA, body, since, original)
	require.NoError(t, err)
	require.NotNil(t, match)
	require.Equal(t, newer, match.TargetID, "the checked reply itself is excluded")

	match, err = repo.FindReply(ctx, postB, body, since, "")
	require.NoError(t, err)
	require.NotNil(t, match)
	require.Equal(t, onB, match.TargetID, "the lookup is scoped to the reply's post")
	require.Equal(t, postB, match.PostID)

	match, err = repo.FindReply(ctx, postA, body+" edited", since, "")
	require.NoError(t, err)
	require.Nil(t, match, "a different body is not a duplicate")
}
