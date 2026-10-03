package db

import (
	"context"
	"errors"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// Moderation verdicts are canonical system replies (task idx 76 step 3). These tests run
// only when DATABASE_URL points at a local test database (setupTestDB skips otherwise).

func cleanupModerationWriterFixture(t *testing.T, pool *Pool, postID, userID string) {
	t.Helper()
	t.Cleanup(func() {
		ctx := context.Background()
		_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE id = $1", postID)
		_, _ = pool.Exec(ctx, "DELETE FROM users WHERE id = $1", userID)
	})
}

func TestModerationReplyWriter_RecordsVerdictAsSystemReply(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	ctx := context.Background()
	user := createCommentTestUser(t, pool)
	post := createCommentTestPost(t, pool, user.ID)
	cleanupModerationWriterFixture(t, pool, post.ID, user.ID)

	written, err := NewModerationReplyWriter(pool).Create(ctx, &models.Comment{
		TargetType: models.CommentTargetPost,
		TargetID:   post.ID,
		AuthorType: models.AuthorTypeSystem,
		AuthorID:   "solvr-moderator",
		Content:    "Post approved by Solvr moderation. Your post is now visible in the feed.",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if written.ID == "" || written.CreatedAt.IsZero() {
		t.Fatalf("written verdict has no identity: %+v", written)
	}

	var postID, authorType, authorID, body string
	var parent, legacyType *string
	err = pool.QueryRow(ctx, `
		SELECT post_id, parent_reply_id, author_type, author_id, body, legacy_type
		FROM replies WHERE id = $1 AND deleted_at IS NULL`, written.ID,
	).Scan(&postID, &parent, &authorType, &authorID, &body, &legacyType)
	if err != nil {
		t.Fatalf("verdict %s is not a reply: %v", written.ID, err)
	}
	if postID != post.ID || parent != nil || legacyType != nil {
		t.Errorf("reply post=%s parent=%v legacy_type=%v, want top-level native reply on %s", postID, parent, legacyType, post.ID)
	}
	if authorType != "system" || authorID != "solvr-moderator" {
		t.Errorf("reply author = %s/%s, want system/solvr-moderator", authorType, authorID)
	}
	if body != "Post approved by Solvr moderation. Your post is now visible in the feed." {
		t.Errorf("reply body = %q", body)
	}

	// No legacy comment can be written: the legacy archive migration (idx 68) moved the
	// comments table out of public.
	var commentsLive bool
	if err := pool.QueryRow(ctx, "SELECT to_regclass('public.comments') IS NOT NULL").Scan(&commentsLive); err != nil {
		t.Fatalf("look up the comments table: %v", err)
	}
	if commentsLive {
		t.Error("the legacy comments table is still live storage")
	}
}

func TestModerationReplyWriter_RejectsVerdictsNotOnAPost(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	ctx := context.Background()
	user := createCommentTestUser(t, pool)
	post := createCommentTestPost(t, pool, user.ID)
	cleanupModerationWriterFixture(t, pool, post.ID, user.ID)

	_, err := NewModerationReplyWriter(pool).Create(ctx, &models.Comment{
		TargetType: models.CommentTargetAnswer,
		TargetID:   post.ID,
		AuthorType: models.AuthorTypeSystem,
		AuthorID:   "solvr-moderator",
		Content:    "verdict on an answer",
	})
	if !errors.Is(err, ErrModerationNoteTarget) {
		t.Fatalf("err = %v, want ErrModerationNoteTarget", err)
	}

	_, err = NewModerationReplyWriter(pool).Create(ctx, &models.Comment{
		TargetType: models.CommentTargetPost,
		TargetID:   "00000000-0000-0000-0000-000000000000",
		AuthorType: models.AuthorTypeSystem,
		AuthorID:   "solvr-moderator",
		Content:    "verdict on a missing post",
	})
	if !errors.Is(err, ErrReplyPostNotFound) {
		t.Fatalf("err = %v, want ErrReplyPostNotFound", err)
	}

	var replies int
	if err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM replies WHERE post_id = $1", post.ID).Scan(&replies); err != nil {
		t.Fatalf("count replies: %v", err)
	}
	if replies != 0 {
		t.Errorf("a refused verdict still wrote %d replies", replies)
	}
}
