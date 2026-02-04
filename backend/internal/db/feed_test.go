package db

import (
	"context"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// TestFeedRepository_GetRecentActivity tests the GetRecentActivity method.
func TestFeedRepository_GetRecentActivity(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	ctx := context.Background()
	feedRepo := NewFeedRepository(pool)
	postRepo := NewPostRepository(pool)
	userRepo := NewUserRepository(pool)

	// Clean up any existing test data
	cleanupFeedTestData(t, pool)

	// Create a test user for author info
	testUser := createFeedTestUser(t, userRepo)

	// Create some test posts
	createFeedTestPost(t, postRepo, "Problem 1", models.PostTypeProblem, models.PostStatusOpen, models.AuthorTypeHuman, testUser.ID)
	createFeedTestPost(t, postRepo, "Question 1", models.PostTypeQuestion, models.PostStatusOpen, models.AuthorTypeHuman, testUser.ID)
	createFeedTestPost(t, postRepo, "Idea 1", models.PostTypeIdea, models.PostStatusActive, models.AuthorTypeHuman, testUser.ID)

	// Test GetRecentActivity
	items, total, err := feedRepo.GetRecentActivity(ctx, 1, 20)
	if err != nil {
		t.Fatalf("GetRecentActivity failed: %v", err)
	}

	if total != 3 {
		t.Errorf("expected total 3, got %d", total)
	}

	if len(items) != 3 {
		t.Errorf("expected 3 items, got %d", len(items))
	}

	// Verify all post types are present
	types := make(map[string]bool)
	for _, item := range items {
		types[item.Type] = true
	}

	if !types["problem"] {
		t.Error("expected problem type in feed")
	}
	if !types["question"] {
		t.Error("expected question type in feed")
	}
	if !types["idea"] {
		t.Error("expected idea type in feed")
	}

	// Clean up
	cleanupFeedTestData(t, pool)
}

// TestFeedRepository_GetRecentActivity_Pagination tests pagination for recent activity.
func TestFeedRepository_GetRecentActivity_Pagination(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	ctx := context.Background()
	feedRepo := NewFeedRepository(pool)
	postRepo := NewPostRepository(pool)
	userRepo := NewUserRepository(pool)

	// Clean up any existing test data
	cleanupFeedTestData(t, pool)

	// Create a test user
	testUser := createFeedTestUser(t, userRepo)

	// Create 15 posts
	for i := 0; i < 15; i++ {
		createFeedTestPost(t, postRepo, "Post "+string(rune('A'+i)), models.PostTypeQuestion, models.PostStatusOpen, models.AuthorTypeHuman, testUser.ID)
	}

	// Test first page (10 items)
	items1, total, err := feedRepo.GetRecentActivity(ctx, 1, 10)
	if err != nil {
		t.Fatalf("GetRecentActivity page 1 failed: %v", err)
	}

	if total != 15 {
		t.Errorf("expected total 15, got %d", total)
	}

	if len(items1) != 10 {
		t.Errorf("expected 10 items on page 1, got %d", len(items1))
	}

	// Test second page (5 items)
	items2, _, err := feedRepo.GetRecentActivity(ctx, 2, 10)
	if err != nil {
		t.Fatalf("GetRecentActivity page 2 failed: %v", err)
	}

	if len(items2) != 5 {
		t.Errorf("expected 5 items on page 2, got %d", len(items2))
	}

	// Clean up
	cleanupFeedTestData(t, pool)
}

// TestFeedRepository_GetStuckProblems tests GetStuckProblems method.
func TestFeedRepository_GetStuckProblems(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	ctx := context.Background()
	feedRepo := NewFeedRepository(pool)
	postRepo := NewPostRepository(pool)
	userRepo := NewUserRepository(pool)

	// Clean up any existing test data
	cleanupFeedTestData(t, pool)

	// Create a test user
	testUser := createFeedTestUser(t, userRepo)

	// Create some problems and questions
	createFeedTestPost(t, postRepo, "Stuck Problem", models.PostTypeProblem, models.PostStatusInProgress, models.AuthorTypeHuman, testUser.ID)
	createFeedTestPost(t, postRepo, "Open Problem", models.PostTypeProblem, models.PostStatusOpen, models.AuthorTypeHuman, testUser.ID)
	createFeedTestPost(t, postRepo, "Question", models.PostTypeQuestion, models.PostStatusOpen, models.AuthorTypeHuman, testUser.ID)

	// Test GetStuckProblems - returns problems with stuck approaches or in_progress status
	items, total, err := feedRepo.GetStuckProblems(ctx, 1, 20)
	if err != nil {
		t.Fatalf("GetStuckProblems failed: %v", err)
	}

	// We should get at least the in_progress problem
	if total < 1 {
		t.Errorf("expected at least 1 stuck problem, got %d", total)
	}

	// Verify all items are problems
	for _, item := range items {
		if item.Type != "problem" {
			t.Errorf("expected problem type, got %s", item.Type)
		}
	}

	// Clean up
	cleanupFeedTestData(t, pool)
}

// TestFeedRepository_GetUnansweredQuestions tests GetUnansweredQuestions method.
func TestFeedRepository_GetUnansweredQuestions(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	ctx := context.Background()
	feedRepo := NewFeedRepository(pool)
	postRepo := NewPostRepository(pool)
	userRepo := NewUserRepository(pool)

	// Clean up any existing test data
	cleanupFeedTestData(t, pool)

	// Create a test user
	testUser := createFeedTestUser(t, userRepo)

	// Create some questions and a problem
	createFeedTestPost(t, postRepo, "Unanswered Question 1", models.PostTypeQuestion, models.PostStatusOpen, models.AuthorTypeHuman, testUser.ID)
	createFeedTestPost(t, postRepo, "Unanswered Question 2", models.PostTypeQuestion, models.PostStatusOpen, models.AuthorTypeHuman, testUser.ID)
	createFeedTestPost(t, postRepo, "Problem", models.PostTypeProblem, models.PostStatusOpen, models.AuthorTypeHuman, testUser.ID)

	// Test GetUnansweredQuestions
	items, total, err := feedRepo.GetUnansweredQuestions(ctx, 1, 20)
	if err != nil {
		t.Fatalf("GetUnansweredQuestions failed: %v", err)
	}

	if total != 2 {
		t.Errorf("expected 2 unanswered questions, got %d", total)
	}

	// Verify all items are questions
	for _, item := range items {
		if item.Type != "question" {
			t.Errorf("expected question type, got %s", item.Type)
		}
	}

	// Clean up
	cleanupFeedTestData(t, pool)
}

// TestFeedRepository_GetUnansweredQuestions_Empty tests empty result.
func TestFeedRepository_GetUnansweredQuestions_Empty(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	ctx := context.Background()
	feedRepo := NewFeedRepository(pool)

	// Clean up any existing test data
	cleanupFeedTestData(t, pool)

	// Test with no data
	items, total, err := feedRepo.GetUnansweredQuestions(ctx, 1, 20)
	if err != nil {
		t.Fatalf("GetUnansweredQuestions failed: %v", err)
	}

	if total != 0 {
		t.Errorf("expected 0 questions, got %d", total)
	}

	if len(items) != 0 {
		t.Errorf("expected 0 items, got %d", len(items))
	}
}

// TestFeedRepository_GetRecentActivity_ExcludesDeleted tests that deleted posts are excluded.
func TestFeedRepository_GetRecentActivity_ExcludesDeleted(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	ctx := context.Background()
	feedRepo := NewFeedRepository(pool)
	postRepo := NewPostRepository(pool)
	userRepo := NewUserRepository(pool)

	// Clean up any existing test data
	cleanupFeedTestData(t, pool)

	// Create a test user
	testUser := createFeedTestUser(t, userRepo)

	// Create posts
	post1 := createFeedTestPost(t, postRepo, "Active Post", models.PostTypeQuestion, models.PostStatusOpen, models.AuthorTypeHuman, testUser.ID)
	post2 := createFeedTestPost(t, postRepo, "To Delete", models.PostTypeQuestion, models.PostStatusOpen, models.AuthorTypeHuman, testUser.ID)

	// Delete one post
	err := postRepo.Delete(ctx, post2.ID)
	if err != nil {
		t.Fatalf("failed to delete post: %v", err)
	}

	// Test GetRecentActivity
	items, total, err := feedRepo.GetRecentActivity(ctx, 1, 20)
	if err != nil {
		t.Fatalf("GetRecentActivity failed: %v", err)
	}

	if total != 1 {
		t.Errorf("expected 1 post (deleted should be excluded), got %d", total)
	}

	if len(items) != 1 {
		t.Errorf("expected 1 item, got %d", len(items))
	}

	if len(items) > 0 && items[0].ID != post1.ID {
		t.Errorf("expected active post, got %s", items[0].ID)
	}

	// Clean up
	cleanupFeedTestData(t, pool)
}

// Helper function to clean up test data
func cleanupFeedTestData(t *testing.T, pool *Pool) {
	ctx := context.Background()
	// Clean in order: answers -> approaches -> posts -> agents -> users
	_, _ = pool.pool.Exec(ctx, "DELETE FROM answers")
	_, _ = pool.pool.Exec(ctx, "DELETE FROM approaches")
	_, _ = pool.pool.Exec(ctx, "DELETE FROM posts")
	_, _ = pool.pool.Exec(ctx, "DELETE FROM agents")
	_, _ = pool.pool.Exec(ctx, "DELETE FROM users")
}

// Helper function to create a test user
func createFeedTestUser(t *testing.T, repo *UserRepository) *models.User {
	t.Helper()
	ctx := context.Background()

	user := &models.User{
		Username:       "testuser" + time.Now().Format("20060102150405.000000000"),
		DisplayName:    "Test User",
		Email:          "test" + time.Now().Format("20060102150405.000000000") + "@example.com",
		AuthProvider:   "github",
		AuthProviderID: "github_" + time.Now().Format("20060102150405.000000000"),
		Role:           "user",
	}

	created, err := repo.Create(ctx, user)
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	return created
}

// Helper function to create a test post
func createFeedTestPost(t *testing.T, repo *PostRepository, title string, postType models.PostType, status models.PostStatus, postedByType models.AuthorType, postedByID string) *models.Post {
	t.Helper()
	ctx := context.Background()

	post := &models.Post{
		Type:         postType,
		Title:        title,
		Description:  "Test description for " + title,
		Tags:         []string{"test", "go"},
		PostedByType: postedByType,
		PostedByID:   postedByID,
		Status:       status,
	}

	created, err := repo.Create(ctx, post)
	if err != nil {
		t.Fatalf("failed to create test post: %v", err)
	}

	return created
}
