package db

import (
	"context"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// === Update Tests ===

func TestPostRepository_Update_Success(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewPostRepository(pool)
	ctx := context.Background()
	authorAgent(ctx, t, pool, "test_agent_update")

	// First create a post
	post := &models.Post{
		Type:         models.PostTypePost,
		Title:        "Original Title",
		Description:  "Original description",
		Tags:         []string{"go"},
		PostedByType: models.AuthorTypeAgent,
		PostedByID:   "test_agent_update",
		Status:       models.PostStatusOpen,
	}

	createdPost, err := repo.Create(ctx, post)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE id = $1", createdPost.ID)
	}()

	// Update the post
	createdPost.Title = "Updated Title"
	createdPost.Description = "Updated description"
	createdPost.Tags = []string{"go", "updated"}
	createdPost.Status = models.PostStatusOpen

	updatedPost, err := repo.Update(ctx, createdPost)
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	// Verify updated fields
	if updatedPost.Title != "Updated Title" {
		t.Errorf("expected title 'Updated Title', got %s", updatedPost.Title)
	}

	if updatedPost.Description != "Updated description" {
		t.Errorf("expected description 'Updated description', got %s", updatedPost.Description)
	}

	if len(updatedPost.Tags) != 2 {
		t.Errorf("expected 2 tags, got %d", len(updatedPost.Tags))
	}

	if updatedPost.Status != models.PostStatusOpen {
		t.Errorf("expected status in_progress, got %s", updatedPost.Status)
	}

	// Verify updated_at changed
	if !updatedPost.UpdatedAt.After(createdPost.CreatedAt) {
		t.Error("expected updated_at to be after created_at")
	}

	// Verify by fetching again
	fetchedPost, err := repo.FindByID(ctx, createdPost.ID)
	if err != nil {
		t.Fatalf("FindByID() error = %v", err)
	}

	if fetchedPost.Title != "Updated Title" {
		t.Errorf("fetched post title mismatch: got %s", fetchedPost.Title)
	}
}

func TestPostRepository_Update_NotFound(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewPostRepository(pool)
	ctx := context.Background()

	// Try to update a non-existent post
	post := &models.Post{
		ID:           "non_existent_post_id_update_test",
		Type:         models.PostTypePost,
		Title:        "Non-existent",
		Description:  "This post does not exist",
		PostedByType: models.AuthorTypeAgent,
		PostedByID:   "test_agent",
		Status:       models.PostStatusOpen,
	}

	updatedPost, err := repo.Update(ctx, post)
	if err == nil {
		t.Fatal("expected error for non-existent post")
	}

	if err != ErrPostNotFound {
		t.Errorf("expected ErrPostNotFound, got %v", err)
	}

	if updatedPost != nil {
		t.Error("expected nil post for non-existent ID")
	}
}

func TestPostRepository_Update_Deleted(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewPostRepository(pool)
	ctx := context.Background()
	authorAgent(ctx, t, pool, "test_agent")

	var postID string

	// Insert a soft-deleted post
	err := pool.QueryRow(ctx, `
		INSERT INTO posts (type, title, description, posted_by_type, posted_by_id, status, deleted_at)
		VALUES ('post', 'Deleted Post', 'Description', 'agent', 'test_agent', 'open', NOW())
		RETURNING id::text
	`).Scan(&postID)
	if err != nil {
		t.Fatalf("failed to insert deleted post: %v", err)
	}

	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE id = $1", postID)
	}()

	// Try to update the deleted post
	post := &models.Post{
		ID:           postID,
		Type:         models.PostTypePost,
		Title:        "Updated Title",
		Description:  "Updated description",
		PostedByType: models.AuthorTypeAgent,
		PostedByID:   "test_agent",
		Status:       models.PostStatusOpen,
	}

	updatedPost, err := repo.Update(ctx, post)
	if err == nil {
		t.Fatal("expected error for deleted post")
	}

	if err != ErrPostNotFound {
		t.Errorf("expected ErrPostNotFound for deleted post, got %v", err)
	}

	if updatedPost != nil {
		t.Error("expected nil post for deleted post")
	}
}

func TestPostRepository_Update_PreservesImmutableFields(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewPostRepository(pool)
	ctx := context.Background()
	authorAgent(ctx, t, pool, "original_agent")

	// Create a post
	post := &models.Post{
		Type:         models.PostTypePost,
		Title:        "Original Title",
		Description:  "Original description",
		PostedByType: models.AuthorTypeAgent,
		PostedByID:   "original_agent",
		Status:       models.PostStatusOpen,
	}

	createdPost, err := repo.Create(ctx, post)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE id = $1", createdPost.ID)
	}()

	originalCreatedAt := createdPost.CreatedAt

	// Try to update immutable fields (type, posted_by_type, posted_by_id, created_at)
	// Update should only change mutable fields
	createdPost.Title = "Updated Title"

	updatedPost, err := repo.Update(ctx, createdPost)
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	// Verify created_at was not modified
	if !updatedPost.CreatedAt.Equal(originalCreatedAt) {
		t.Error("created_at should not be modified by Update")
	}

	// Verify type was preserved
	if updatedPost.Type != models.PostTypePost {
		t.Errorf("type should be preserved, got %s", updatedPost.Type)
	}

	// Verify posted_by fields were preserved
	if updatedPost.PostedByType != models.AuthorTypeAgent {
		t.Errorf("posted_by_type should be preserved, got %s", updatedPost.PostedByType)
	}

	if updatedPost.PostedByID != "original_agent" {
		t.Errorf("posted_by_id should be preserved, got %s", updatedPost.PostedByID)
	}
}

// === Delete Tests ===

func TestPostRepository_Delete_Success(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewPostRepository(pool)
	ctx := context.Background()
	authorAgent(ctx, t, pool, "test_agent_delete")

	// Create a post
	post := &models.Post{
		Type:         models.PostTypePost,
		Title:        "Post to Delete",
		Description:  "This post will be soft deleted",
		PostedByType: models.AuthorTypeAgent,
		PostedByID:   "test_agent_delete",
		Status:       models.PostStatusOpen,
	}

	createdPost, err := repo.Create(ctx, post)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	defer func() {
		// Clean up even if test fails
		_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE id = $1", createdPost.ID)
	}()

	// Verify post exists before delete
	_, err = repo.FindByID(ctx, createdPost.ID)
	if err != nil {
		t.Fatalf("Post should exist before delete: %v", err)
	}

	// Delete the post (soft delete)
	err = repo.Delete(ctx, createdPost.ID)
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	// Verify post is not found via FindByID (soft deleted)
	_, err = repo.FindByID(ctx, createdPost.ID)
	if err == nil {
		t.Fatal("expected error after delete")
	}

	if err != ErrPostNotFound {
		t.Errorf("expected ErrPostNotFound after delete, got %v", err)
	}

	// Verify post still exists in database but with deleted_at set
	var deletedAt *time.Time
	err = pool.QueryRow(ctx, "SELECT deleted_at FROM posts WHERE id = $1", createdPost.ID).Scan(&deletedAt)
	if err != nil {
		t.Fatalf("failed to query deleted post: %v", err)
	}

	if deletedAt == nil {
		t.Error("expected deleted_at to be set")
	}
}

func TestPostRepository_Delete_NotFound(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewPostRepository(pool)
	ctx := context.Background()

	// Try to delete a non-existent post
	err := repo.Delete(ctx, "non_existent_post_id_delete_test")
	if err == nil {
		t.Fatal("expected error for non-existent post")
	}

	if err != ErrPostNotFound {
		t.Errorf("expected ErrPostNotFound, got %v", err)
	}
}

func TestPostRepository_Delete_AlreadyDeleted(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewPostRepository(pool)
	ctx := context.Background()
	authorAgent(ctx, t, pool, "test_agent")

	var postID string

	// Insert a soft-deleted post
	err := pool.QueryRow(ctx, `
		INSERT INTO posts (type, title, description, posted_by_type, posted_by_id, status, deleted_at)
		VALUES ('post', 'Already Deleted', 'Description', 'agent', 'test_agent', 'open', NOW())
		RETURNING id::text
	`).Scan(&postID)
	if err != nil {
		t.Fatalf("failed to insert deleted post: %v", err)
	}

	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE id = $1", postID)
	}()

	// Try to delete an already deleted post
	err = repo.Delete(ctx, postID)
	if err == nil {
		t.Fatal("expected error for already deleted post")
	}

	if err != ErrPostNotFound {
		t.Errorf("expected ErrPostNotFound for already deleted post, got %v", err)
	}
}

func TestPostRepository_Delete_ExcludedFromList(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewPostRepository(pool)
	ctx := context.Background()
	authorAgent(ctx, t, pool, "test_agent_delete_list")

	// Create a post
	post := &models.Post{
		Type:         models.PostTypePost,
		Title:        "Post to Delete for List Test",
		Description:  "This post will be deleted and should not appear in list",
		Tags:         []string{"delete_test_unique_tag"},
		PostedByType: models.AuthorTypeAgent,
		PostedByID:   "test_agent_delete_list",
		Status:       models.PostStatusOpen,
	}

	createdPost, err := repo.Create(ctx, post)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE id = $1", createdPost.ID)
	}()

	// Verify post appears in list before delete
	opts := models.PostListOptions{
		Tags:    []string{"delete_test_unique_tag"},
		Page:    1,
		PerPage: 100,
	}

	posts, _, err := repo.List(ctx, opts)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	foundBeforeDelete := false
	for _, p := range posts {
		if p.ID == createdPost.ID {
			foundBeforeDelete = true
			break
		}
	}

	if !foundBeforeDelete {
		t.Error("post should appear in list before delete")
	}

	// Delete the post
	err = repo.Delete(ctx, createdPost.ID)
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	// Verify post does not appear in list after delete
	posts, _, err = repo.List(ctx, opts)
	if err != nil {
		t.Fatalf("List() after delete error = %v", err)
	}

	for _, p := range posts {
		if p.ID == createdPost.ID {
			t.Error("deleted post should not appear in list")
		}
	}
}
