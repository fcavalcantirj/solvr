package db

import (
	"context"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// TestPostList_CountsSystemComments verifies that system comments (moderation
// notices) are included in comments_count when listing posts. Before the fix,
// List() had `author_type != 'system'` in its subquery which excluded them.
func TestPostList_CountsSystemComments(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	ctx := context.Background()
	postRepo := NewPostRepository(pool)
	commentRepo := NewCommentsRepository(pool)

	user := createCommentTestUser(t, pool)
	post := createCommentTestPost(t, pool, user.ID)

	// Insert one system comment (moderation notice)
	_, err := commentRepo.Create(ctx, &models.Comment{
		TargetType: models.CommentTargetPost,
		TargetID:   post.ID,
		AuthorType: models.AuthorTypeSystem,
		AuthorID:   "system",
		Content:    "Post approved by moderation system",
	})
	if err != nil {
		t.Fatalf("failed to create system comment: %v", err)
	}

	// List() should count the system comment
	posts, _, err := postRepo.List(ctx, models.PostListOptions{
		Type:    models.PostTypeQuestion,
		Page:    1,
		PerPage: 50,
	})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	var found *models.PostWithAuthor
	for i := range posts {
		if posts[i].ID == post.ID {
			found = &posts[i]
			break
		}
	}
	if found == nil {
		t.Fatalf("post %s not found in List() results", post.ID)
	}

	if found.CommentsCount != 1 {
		t.Errorf("List() CommentsCount = %d, want 1 (system comment must be counted)", found.CommentsCount)
	}
}

// TestPostGet_CountsSystemComments verifies that FindByID also counts system
// comments consistently with List().
func TestPostGet_CountsSystemComments(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	ctx := context.Background()
	postRepo := NewPostRepository(pool)
	commentRepo := NewCommentsRepository(pool)

	user := createCommentTestUser(t, pool)
	post := createCommentTestPost(t, pool, user.ID)

	// Insert one system comment
	_, err := commentRepo.Create(ctx, &models.Comment{
		TargetType: models.CommentTargetPost,
		TargetID:   post.ID,
		AuthorType: models.AuthorTypeSystem,
		AuthorID:   "system",
		Content:    "Post approved by moderation system",
	})
	if err != nil {
		t.Fatalf("failed to create system comment: %v", err)
	}

	result, err := postRepo.FindByID(ctx, post.ID)
	if err != nil {
		t.Fatalf("FindByID failed: %v", err)
	}

	if result.CommentsCount != 1 {
		t.Errorf("FindByID() CommentsCount = %d, want 1 (system comment must be counted)", result.CommentsCount)
	}
}

// TestPostList_CountMatchesFeedCount verifies that List() produces the same
// count as the feed query when a post has both system and human comments.
func TestPostList_CountMatchesFeedCount(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	ctx := context.Background()
	postRepo := NewPostRepository(pool)
	commentRepo := NewCommentsRepository(pool)

	user := createCommentTestUser(t, pool)
	post := createCommentTestPost(t, pool, user.ID)

	// Insert one system comment + one human comment = 2 total
	_, err := commentRepo.Create(ctx, &models.Comment{
		TargetType: models.CommentTargetPost,
		TargetID:   post.ID,
		AuthorType: models.AuthorTypeSystem,
		AuthorID:   "system",
		Content:    "Moderation notice",
	})
	if err != nil {
		t.Fatalf("failed to create system comment: %v", err)
	}

	_, err = commentRepo.Create(ctx, &models.Comment{
		TargetType: models.CommentTargetPost,
		TargetID:   post.ID,
		AuthorType: models.AuthorTypeHuman,
		AuthorID:   user.ID,
		Content:    "Human reply",
	})
	if err != nil {
		t.Fatalf("failed to create human comment: %v", err)
	}

	// List() should return 2 (matches what feed counts)
	posts, _, err := postRepo.List(ctx, models.PostListOptions{
		Type:    models.PostTypeQuestion,
		Page:    1,
		PerPage: 50,
	})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	var found *models.PostWithAuthor
	for i := range posts {
		if posts[i].ID == post.ID {
			found = &posts[i]
			break
		}
	}
	if found == nil {
		t.Fatalf("post %s not found in List() results", post.ID)
	}

	if found.CommentsCount != 2 {
		t.Errorf("List() CommentsCount = %d, want 2 (1 system + 1 human)", found.CommentsCount)
	}
}
