package db

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// Shared fixtures, kept from the legacy comment repository tests retired with it (idx 68):
// the reply, post and moderation tests still build their author and post with them.

// createCommentTestUser creates a throwaway human account.
func createCommentTestUser(t *testing.T, pool *Pool) *models.User {
	t.Helper()
	ctx := context.Background()
	userRepo := NewUserRepository(pool)

	now := time.Now()
	ts := now.Format("150405.000000")
	username := "c" + now.Format("0405") + fmt.Sprintf("%06d", now.Nanosecond()/1000)[:5]
	user := &models.User{
		Username:       username,
		DisplayName:    "Comment Test User",
		Email:          "comment" + ts + "@example.com",
		AuthProvider:   "github",
		AuthProviderID: "gh_cmt_" + ts,
		Role:           "user",
	}

	created, err := userRepo.Create(ctx, user)
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}
	return created
}

// createCommentTestPost creates a test post for comment tests.
func createCommentTestPost(t *testing.T, pool *Pool, userID string) *models.Post {
	t.Helper()
	ctx := context.Background()
	postRepo := NewPostRepository(pool)

	post := &models.Post{
		Type:         models.PostTypePost,
		Title:        "Test question for comments " + time.Now().Format("150405"),
		Description:  "This is a test question used for comment testing",
		Tags:         []string{"test"},
		PostedByType: models.AuthorTypeHuman,
		PostedByID:   userID,
		Status:       models.PostStatusOpen,
	}

	created, err := postRepo.Create(ctx, post)
	if err != nil {
		t.Fatalf("failed to create test post: %v", err)
	}
	return created
}

// seedMigratedReply inserts a live top-level reply on postID shaped the way the contribution
// cutover wrote one from a legacy row: legacyType ("answer", "comment", ...) and a legacy id of
// its own. Tests that seeded legacy answers and comments, then copied them into replies, seed
// the reply directly since the legacy repositories were deleted (idx 68). It returns the id.
func seedMigratedReply(t *testing.T, pool *Pool, ctx context.Context, postID, legacyType, authorType, authorID, body string) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(ctx, `
		INSERT INTO replies (post_id, author_type, author_id, body, legacy_type, legacy_id)
		VALUES ($1, $2, $3, $4, $5, gen_random_uuid())
		RETURNING id::text`, postID, authorType, authorID, body, legacyType).Scan(&id); err != nil {
		t.Fatalf("seed %s reply: %v", legacyType, err)
	}
	return id
}
