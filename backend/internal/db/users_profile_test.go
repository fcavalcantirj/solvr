// Package db provides database access for Solvr.
package db

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// TestUserProfile_GetUserStats_Integration tests user stats computation with real database.
// Per BE-003: Calculate user stats: posts_count, contributions_count, karma.
func TestUserProfile_GetUserStats_Integration(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}

	ctx := context.Background()
	pool, err := NewPool(ctx, databaseURL)
	if err != nil {
		t.Fatalf("failed to connect to database: %v", err)
	}
	defer pool.Close()

	userRepo := NewUserRepository(pool)
	postRepo := NewPostRepository(pool)

	// Create a test user
	suffix := time.Now().Format("20060102150405.000000000")
	user := &models.User{
		Username:       "profiletest" + suffix,
		DisplayName:    "Profile Test User",
		Email:          "profiletest" + suffix + "@example.com",
		AuthProvider:   models.AuthProviderGitHub,
		AuthProviderID: "github_profile_" + suffix,
		Role:           models.UserRoleUser,
	}

	createdUser, err := userRepo.Create(ctx, user)
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}
	t.Cleanup(func() {
		// Cleanup: delete user and posts
		pool.Exec(ctx, "DELETE FROM posts WHERE posted_by_id = $1", createdUser.ID)
		pool.Exec(ctx, "DELETE FROM users WHERE id = $1", createdUser.ID)
	})

	// Create some posts for the user
	for i := 0; i < 3; i++ {
		post := &models.Post{
			Type:         models.PostTypeQuestion,
			Title:        "Test question for profile stats",
			Description:  "This is a test question to verify user stats calculation works correctly.",
			Tags:         []string{"test"},
			PostedByType: models.AuthorTypeHuman,
			PostedByID:   createdUser.ID,
			Status:       models.PostStatusOpen,
		}
		_, err := postRepo.Create(ctx, post)
		if err != nil {
			t.Fatalf("failed to create test post %d: %v", i, err)
		}
	}

	// Get user stats
	stats, err := userRepo.GetUserStats(ctx, createdUser.ID)
	if err != nil {
		t.Fatalf("GetUserStats failed: %v", err)
	}

	// Verify posts_created count
	if stats.PostsCreated != 3 {
		t.Errorf("expected posts_created 3, got %d", stats.PostsCreated)
	}
}

// TestUserProfile_Update_Integration tests updating user profile with real database.
// Per BE-003: PATCH /v1/me - update own profile (display_name, bio).
func TestUserProfile_Update_Integration(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}

	ctx := context.Background()
	pool, err := NewPool(ctx, databaseURL)
	if err != nil {
		t.Fatalf("failed to connect to database: %v", err)
	}
	defer pool.Close()

	userRepo := NewUserRepository(pool)

	// Create a test user
	suffix := time.Now().Format("20060102150405.000000000")
	user := &models.User{
		Username:       "updatetest" + suffix,
		DisplayName:    "Update Test User",
		Email:          "updatetest" + suffix + "@example.com",
		AuthProvider:   models.AuthProviderGitHub,
		AuthProviderID: "github_update_" + suffix,
		Role:           models.UserRoleUser,
		Bio:            "Original bio",
	}

	createdUser, err := userRepo.Create(ctx, user)
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}
	t.Cleanup(func() {
		pool.Exec(ctx, "DELETE FROM users WHERE id = $1", createdUser.ID)
	})

	// Update the user profile
	createdUser.DisplayName = "Updated Display Name"
	createdUser.Bio = "Updated bio content"
	createdUser.AvatarURL = "https://example.com/new-avatar.png"

	updatedUser, err := userRepo.Update(ctx, createdUser)
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	// Verify the update
	if updatedUser.DisplayName != "Updated Display Name" {
		t.Errorf("expected display_name 'Updated Display Name', got %s", updatedUser.DisplayName)
	}
	if updatedUser.Bio != "Updated bio content" {
		t.Errorf("expected bio 'Updated bio content', got %s", updatedUser.Bio)
	}
	if updatedUser.AvatarURL != "https://example.com/new-avatar.png" {
		t.Errorf("expected avatar_url 'https://example.com/new-avatar.png', got %s", updatedUser.AvatarURL)
	}

	// Verify by fetching again
	fetchedUser, err := userRepo.FindByID(ctx, createdUser.ID)
	if err != nil {
		t.Fatalf("FindByID failed: %v", err)
	}
	if fetchedUser.DisplayName != "Updated Display Name" {
		t.Errorf("after fetch: expected display_name 'Updated Display Name', got %s", fetchedUser.DisplayName)
	}
}

// TestPostListByAuthor_Integration tests listing posts by author with real database.
// Per BE-003: GET /v1/me/posts - list own posts.
func TestPostListByAuthor_Integration(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}

	ctx := context.Background()
	pool, err := NewPool(ctx, databaseURL)
	if err != nil {
		t.Fatalf("failed to connect to database: %v", err)
	}
	defer pool.Close()

	userRepo := NewUserRepository(pool)
	postRepo := NewPostRepository(pool)

	// Create two test users
	suffix := time.Now().Format("20060102150405.000000000")
	user1 := &models.User{
		Username:       "listtest1" + suffix,
		DisplayName:    "List Test User 1",
		Email:          "listtest1" + suffix + "@example.com",
		AuthProvider:   models.AuthProviderGitHub,
		AuthProviderID: "github_list1_" + suffix,
		Role:           models.UserRoleUser,
	}
	user2 := &models.User{
		Username:       "listtest2" + suffix,
		DisplayName:    "List Test User 2",
		Email:          "listtest2" + suffix + "@example.com",
		AuthProvider:   models.AuthProviderGitHub,
		AuthProviderID: "github_list2_" + suffix,
		Role:           models.UserRoleUser,
	}

	createdUser1, err := userRepo.Create(ctx, user1)
	if err != nil {
		t.Fatalf("failed to create test user 1: %v", err)
	}
	createdUser2, err := userRepo.Create(ctx, user2)
	if err != nil {
		t.Fatalf("failed to create test user 2: %v", err)
	}
	t.Cleanup(func() {
		pool.Exec(ctx, "DELETE FROM posts WHERE posted_by_id IN ($1, $2)", createdUser1.ID, createdUser2.ID)
		pool.Exec(ctx, "DELETE FROM users WHERE id IN ($1, $2)", createdUser1.ID, createdUser2.ID)
	})

	// Create posts for user1 (3 posts)
	for i := 0; i < 3; i++ {
		post := &models.Post{
			Type:         models.PostTypeQuestion,
			Title:        "Test question from user 1",
			Description:  "This is a test question from user 1 to verify author filtering.",
			Tags:         []string{"test"},
			PostedByType: models.AuthorTypeHuman,
			PostedByID:   createdUser1.ID,
			Status:       models.PostStatusOpen,
		}
		_, err := postRepo.Create(ctx, post)
		if err != nil {
			t.Fatalf("failed to create post for user 1: %v", err)
		}
	}

	// Create posts for user2 (2 posts)
	for i := 0; i < 2; i++ {
		post := &models.Post{
			Type:         models.PostTypeProblem,
			Title:        "Test problem from user 2",
			Description:  "This is a test problem from user 2 to verify author filtering works.",
			Tags:         []string{"test"},
			PostedByType: models.AuthorTypeHuman,
			PostedByID:   createdUser2.ID,
			Status:       models.PostStatusOpen,
		}
		_, err := postRepo.Create(ctx, post)
		if err != nil {
			t.Fatalf("failed to create post for user 2: %v", err)
		}
	}

	// List posts by user1
	opts := models.PostListOptions{
		AuthorType: models.AuthorTypeHuman,
		AuthorID:   createdUser1.ID,
		Page:       1,
		PerPage:    10,
	}
	posts, total, err := postRepo.List(ctx, opts)
	if err != nil {
		t.Fatalf("List posts for user1 failed: %v", err)
	}

	if total != 3 {
		t.Errorf("expected total 3 posts for user1, got %d", total)
	}
	if len(posts) != 3 {
		t.Errorf("expected 3 posts for user1, got %d", len(posts))
	}

	// Verify all posts belong to user1
	for _, p := range posts {
		if p.PostedByID != createdUser1.ID {
			t.Errorf("expected all posts to belong to user1, got post from %s", p.PostedByID)
		}
	}

	// List posts by user2
	opts.AuthorID = createdUser2.ID
	posts, total, err = postRepo.List(ctx, opts)
	if err != nil {
		t.Fatalf("List posts for user2 failed: %v", err)
	}

	if total != 2 {
		t.Errorf("expected total 2 posts for user2, got %d", total)
	}
	if len(posts) != 2 {
		t.Errorf("expected 2 posts for user2, got %d", len(posts))
	}
}

// TestUserProfile_FindByID_NotFound_Integration tests FindByID with non-existent user.
// Per BE-003: GET /v1/users/:id should return 404 for non-existent users.
func TestUserProfile_FindByID_NotFound_Integration(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}

	ctx := context.Background()
	pool, err := NewPool(ctx, databaseURL)
	if err != nil {
		t.Fatalf("failed to connect to database: %v", err)
	}
	defer pool.Close()

	userRepo := NewUserRepository(pool)

	// Try to find a non-existent user
	_, err = userRepo.FindByID(ctx, "00000000-0000-0000-0000-000000000000")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}
