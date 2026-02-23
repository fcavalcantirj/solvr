package db

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
)

func createBookmarkTestUser(t *testing.T, repo *UserRepository) *models.User {
	t.Helper()
	ctx := context.Background()

	now := time.Now()
	ts := now.Format("150405.000000")
	user := &models.User{
		Username:       "bm" + now.Format("0405") + fmt.Sprintf("%06d", now.Nanosecond()/1000)[:4],
		DisplayName:    "Bookmark Test User",
		Email:          "bookmark_" + ts + "@example.com",
		AuthProvider:   "github",
		AuthProviderID: "github_bookmark_" + ts,
		Role:           "user",
	}

	created, err := repo.Create(ctx, user)
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}
	return created
}

func TestBookmarkRepository_AddBookmark(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	ctx := context.Background()
	bookmarkRepo := NewBookmarkRepository(pool)
	postRepo := NewPostRepository(pool)
	userRepo := NewUserRepository(pool)

	// Create test user
	testUser := createBookmarkTestUser(t, userRepo)

	// Create a test post
	post := &models.Post{
		Type:         models.PostTypeQuestion,
		Title:        "Test question for bookmarks",
		Description:  "This is a test question to be bookmarked by users",
		Tags:         []string{"test"},
		PostedByType: models.AuthorTypeHuman,
		PostedByID:   testUser.ID,
		Status:       models.PostStatusOpen,
	}
	createdPost, err := postRepo.Create(ctx, post)
	if err != nil {
		t.Fatalf("failed to create test post: %v", err)
	}

	// Test adding a bookmark
	bookmark, err := bookmarkRepo.Add(ctx, "human", testUser.ID, createdPost.ID)
	if err != nil {
		t.Fatalf("failed to add bookmark: %v", err)
	}

	if bookmark.PostID != createdPost.ID {
		t.Errorf("expected post ID %s, got %s", createdPost.ID, bookmark.PostID)
	}
	if bookmark.UserType != "human" {
		t.Errorf("expected user type human, got %s", bookmark.UserType)
	}
	if bookmark.UserID != testUser.ID {
		t.Errorf("expected user ID %s, got %s", testUser.ID, bookmark.UserID)
	}
}

func TestBookmarkRepository_AddBookmark_Duplicate(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	ctx := context.Background()
	bookmarkRepo := NewBookmarkRepository(pool)
	postRepo := NewPostRepository(pool)
	userRepo := NewUserRepository(pool)

	// Create test user
	testUser := createBookmarkTestUser(t, userRepo)

	// Create a test post
	post := &models.Post{
		Type:         models.PostTypeQuestion,
		Title:        "Test question for duplicate bookmark",
		Description:  "This is a test question to test duplicate bookmarks",
		Tags:         []string{"test"},
		PostedByType: models.AuthorTypeHuman,
		PostedByID:   testUser.ID,
		Status:       models.PostStatusOpen,
	}
	createdPost, err := postRepo.Create(ctx, post)
	if err != nil {
		t.Fatalf("failed to create test post: %v", err)
	}

	// Add bookmark first time
	_, err = bookmarkRepo.Add(ctx, "human", testUser.ID, createdPost.ID)
	if err != nil {
		t.Fatalf("failed to add bookmark first time: %v", err)
	}

	// Add bookmark second time - should return error
	_, err = bookmarkRepo.Add(ctx, "human", testUser.ID, createdPost.ID)
	if err != ErrBookmarkExists {
		t.Errorf("expected ErrBookmarkExists, got %v", err)
	}
}

func TestBookmarkRepository_RemoveBookmark(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	ctx := context.Background()
	bookmarkRepo := NewBookmarkRepository(pool)
	postRepo := NewPostRepository(pool)
	userRepo := NewUserRepository(pool)

	// Create test user
	testUser := createBookmarkTestUser(t, userRepo)

	// Create a test post
	post := &models.Post{
		Type:         models.PostTypeQuestion,
		Title:        "Test question for removing bookmark",
		Description:  "This is a test question for removing bookmarks",
		Tags:         []string{"test"},
		PostedByType: models.AuthorTypeHuman,
		PostedByID:   testUser.ID,
		Status:       models.PostStatusOpen,
	}
	createdPost, err := postRepo.Create(ctx, post)
	if err != nil {
		t.Fatalf("failed to create test post: %v", err)
	}

	// Add bookmark
	_, err = bookmarkRepo.Add(ctx, "human", testUser.ID, createdPost.ID)
	if err != nil {
		t.Fatalf("failed to add bookmark: %v", err)
	}

	// Remove bookmark
	err = bookmarkRepo.Remove(ctx, "human", testUser.ID, createdPost.ID)
	if err != nil {
		t.Fatalf("failed to remove bookmark: %v", err)
	}

	// Check it's removed
	bookmarks, _, err := bookmarkRepo.ListByUser(ctx, "human", testUser.ID, 1, 10)
	if err != nil {
		t.Fatalf("failed to list bookmarks: %v", err)
	}
	if len(bookmarks) != 0 {
		t.Errorf("expected 0 bookmarks after removal, got %d", len(bookmarks))
	}
}

func TestBookmarkRepository_ListByUser(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	ctx := context.Background()
	bookmarkRepo := NewBookmarkRepository(pool)
	postRepo := NewPostRepository(pool)
	userRepo := NewUserRepository(pool)

	// Create test user
	testUser := createBookmarkTestUser(t, userRepo)

	// Create multiple test posts
	for i := 0; i < 3; i++ {
		post := &models.Post{
			Type:         models.PostTypeQuestion,
			Title:        "Test question for listing bookmarks " + string(rune('A'+i)),
			Description:  "This is test question number for listing bookmarks",
			Tags:         []string{"test"},
			PostedByType: models.AuthorTypeHuman,
			PostedByID:   testUser.ID,
			Status:       models.PostStatusOpen,
		}
		createdPost, err := postRepo.Create(ctx, post)
		if err != nil {
			t.Fatalf("failed to create test post %d: %v", i, err)
		}

		_, err = bookmarkRepo.Add(ctx, "human", testUser.ID, createdPost.ID)
		if err != nil {
			t.Fatalf("failed to add bookmark %d: %v", i, err)
		}
	}

	// List bookmarks
	bookmarks, total, err := bookmarkRepo.ListByUser(ctx, "human", testUser.ID, 1, 10)
	if err != nil {
		t.Fatalf("failed to list bookmarks: %v", err)
	}

	if len(bookmarks) != 3 {
		t.Errorf("expected 3 bookmarks, got %d", len(bookmarks))
	}
	if total != 3 {
		t.Errorf("expected total 3, got %d", total)
	}
}

func TestBookmarkRepository_IsBookmarked(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	ctx := context.Background()
	bookmarkRepo := NewBookmarkRepository(pool)
	postRepo := NewPostRepository(pool)
	userRepo := NewUserRepository(pool)

	// Create test user
	testUser := createBookmarkTestUser(t, userRepo)

	// Create a test post
	post := &models.Post{
		Type:         models.PostTypeQuestion,
		Title:        "Test question for checking bookmark",
		Description:  "This is a test question to check if bookmarked",
		Tags:         []string{"test"},
		PostedByType: models.AuthorTypeHuman,
		PostedByID:   testUser.ID,
		Status:       models.PostStatusOpen,
	}
	createdPost, err := postRepo.Create(ctx, post)
	if err != nil {
		t.Fatalf("failed to create test post: %v", err)
	}

	// Check before bookmarking
	isBookmarked, err := bookmarkRepo.IsBookmarked(ctx, "human", testUser.ID, createdPost.ID)
	if err != nil {
		t.Fatalf("failed to check bookmark: %v", err)
	}
	if isBookmarked {
		t.Error("expected not bookmarked before adding")
	}

	// Add bookmark
	_, err = bookmarkRepo.Add(ctx, "human", testUser.ID, createdPost.ID)
	if err != nil {
		t.Fatalf("failed to add bookmark: %v", err)
	}

	// Check after bookmarking
	isBookmarked, err = bookmarkRepo.IsBookmarked(ctx, "human", testUser.ID, createdPost.ID)
	if err != nil {
		t.Fatalf("failed to check bookmark after adding: %v", err)
	}
	if !isBookmarked {
		t.Error("expected bookmarked after adding")
	}
}
