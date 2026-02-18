package db

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// createCommentTestUser creates a test user for comment tests.
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
		Type:         models.PostTypeQuestion,
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

func TestCommentsRepository_Create(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	ctx := context.Background()
	repo := NewCommentsRepository(pool)
	user := createCommentTestUser(t, pool)
	post := createCommentTestPost(t, pool, user.ID)

	comment := &models.Comment{
		TargetType: models.CommentTargetPost,
		TargetID:   post.ID,
		AuthorType: models.AuthorTypeHuman,
		AuthorID:   user.ID,
		Content:    "This is a test comment on a post",
	}

	created, err := repo.Create(ctx, comment)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if created.ID == "" {
		t.Error("expected non-empty ID")
	}
	if created.Content != "This is a test comment on a post" {
		t.Errorf("expected content 'This is a test comment on a post', got %q", created.Content)
	}
	if created.CreatedAt.IsZero() {
		t.Error("expected non-zero CreatedAt")
	}
	if created.DeletedAt != nil {
		t.Error("expected nil DeletedAt for new comment")
	}
}

func TestCommentsRepository_FindByID(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	ctx := context.Background()
	repo := NewCommentsRepository(pool)
	user := createCommentTestUser(t, pool)
	post := createCommentTestPost(t, pool, user.ID)

	// Create a comment
	comment := &models.Comment{
		TargetType: models.CommentTargetPost,
		TargetID:   post.ID,
		AuthorType: models.AuthorTypeHuman,
		AuthorID:   user.ID,
		Content:    "Comment for FindByID test",
	}
	created, err := repo.Create(ctx, comment)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Find it
	found, err := repo.FindByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("FindByID failed: %v", err)
	}

	if found.ID != created.ID {
		t.Errorf("expected ID %q, got %q", created.ID, found.ID)
	}
	if found.Content != "Comment for FindByID test" {
		t.Errorf("expected content 'Comment for FindByID test', got %q", found.Content)
	}
	// Should have author info populated
	if found.Author.ID == "" {
		t.Error("expected non-empty author ID")
	}
	if found.Author.DisplayName == "" {
		t.Error("expected non-empty author display name")
	}
}

func TestCommentsRepository_FindByID_NotFound(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	ctx := context.Background()
	repo := NewCommentsRepository(pool)

	_, err := repo.FindByID(ctx, "00000000-0000-0000-0000-000000000000")
	if err == nil {
		t.Fatal("expected error for non-existent comment")
	}
}

func TestCommentsRepository_List(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	ctx := context.Background()
	repo := NewCommentsRepository(pool)
	user := createCommentTestUser(t, pool)
	post := createCommentTestPost(t, pool, user.ID)

	// Create multiple comments
	for i := 0; i < 3; i++ {
		comment := &models.Comment{
			TargetType: models.CommentTargetPost,
			TargetID:   post.ID,
			AuthorType: models.AuthorTypeHuman,
			AuthorID:   user.ID,
			Content:    "List test comment",
		}
		if _, err := repo.Create(ctx, comment); err != nil {
			t.Fatalf("Create failed: %v", err)
		}
	}

	// List comments for this post
	opts := models.CommentListOptions{
		TargetType: models.CommentTargetPost,
		TargetID:   post.ID,
		Page:       1,
		PerPage:    10,
	}
	comments, total, err := repo.List(ctx, opts)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if total < 3 {
		t.Errorf("expected total >= 3, got %d", total)
	}
	if len(comments) < 3 {
		t.Errorf("expected at least 3 comments, got %d", len(comments))
	}

	// Each comment should have author info
	for _, c := range comments {
		if c.Author.ID == "" {
			t.Error("expected non-empty author ID in list")
		}
	}
}

func TestCommentsRepository_List_Pagination(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	ctx := context.Background()
	repo := NewCommentsRepository(pool)
	user := createCommentTestUser(t, pool)
	post := createCommentTestPost(t, pool, user.ID)

	// Create 5 comments
	for i := 0; i < 5; i++ {
		comment := &models.Comment{
			TargetType: models.CommentTargetPost,
			TargetID:   post.ID,
			AuthorType: models.AuthorTypeHuman,
			AuthorID:   user.ID,
			Content:    "Pagination test comment",
		}
		if _, err := repo.Create(ctx, comment); err != nil {
			t.Fatalf("Create failed: %v", err)
		}
	}

	// Get page 1 with perPage=2
	opts := models.CommentListOptions{
		TargetType: models.CommentTargetPost,
		TargetID:   post.ID,
		Page:       1,
		PerPage:    2,
	}
	comments, total, err := repo.List(ctx, opts)
	if err != nil {
		t.Fatalf("List page 1 failed: %v", err)
	}

	if len(comments) != 2 {
		t.Errorf("expected 2 comments on page 1, got %d", len(comments))
	}
	if total < 5 {
		t.Errorf("expected total >= 5, got %d", total)
	}
}

func TestCommentsRepository_Delete(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	ctx := context.Background()
	repo := NewCommentsRepository(pool)
	user := createCommentTestUser(t, pool)
	post := createCommentTestPost(t, pool, user.ID)

	// Create and then delete
	comment := &models.Comment{
		TargetType: models.CommentTargetPost,
		TargetID:   post.ID,
		AuthorType: models.AuthorTypeHuman,
		AuthorID:   user.ID,
		Content:    "Comment to be deleted",
	}
	created, err := repo.Create(ctx, comment)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	err = repo.Delete(ctx, created.ID)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Should not be found after soft delete
	_, err = repo.FindByID(ctx, created.ID)
	if err == nil {
		t.Error("expected error after deleting comment, got nil")
	}
}

func TestCommentsRepository_TargetExists(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	ctx := context.Background()
	repo := NewCommentsRepository(pool)
	user := createCommentTestUser(t, pool)
	post := createCommentTestPost(t, pool, user.ID)

	// Post should exist
	exists, err := repo.TargetExists(ctx, models.CommentTargetPost, post.ID)
	if err != nil {
		t.Fatalf("TargetExists failed: %v", err)
	}
	if !exists {
		t.Error("expected post to exist as target")
	}

	// Non-existent post should not exist
	exists, err = repo.TargetExists(ctx, models.CommentTargetPost, "00000000-0000-0000-0000-000000000000")
	if err != nil {
		t.Fatalf("TargetExists for non-existent failed: %v", err)
	}
	if exists {
		t.Error("expected non-existent post to not exist as target")
	}
}
