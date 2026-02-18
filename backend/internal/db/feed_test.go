package db

import (
	"context"
	"fmt"
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


// Helper function to create a test user
func createFeedTestUser(t *testing.T, repo *UserRepository) *models.User {
	t.Helper()
	ctx := context.Background()

	// Use shorter timestamp format to stay within username length limit (30 chars)
	timestamp := time.Now().Format("150405.000")
	user := &models.User{
		Username:       "testuser" + timestamp,
		DisplayName:    "Test User",
		Email:          "test" + timestamp + "@example.com",
		AuthProvider:   "github",
		AuthProviderID: "github_" + timestamp,
		Role:           "user",
	}

	created, err := repo.Create(ctx, user)
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	return created
}

// TestFeedRepository_GetRecentActivity_CommentCounts tests that comment counts are correct for all post types.
// This test verifies that:
// - Questions show answer_count (from answers table)
// - Problems show approach_count (from approaches table)
// - Ideas show answer_count (from responses table, mapped to answer_count field)
func TestFeedRepository_GetRecentActivity_CommentCounts(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	ctx := context.Background()
	feedRepo := NewFeedRepository(pool)
	postRepo := NewPostRepository(pool)
	userRepo := NewUserRepository(pool)
	answersRepo := NewAnswersRepository(pool)
	approachesRepo := NewApproachesRepository(pool)
	responsesRepo := NewResponsesRepository(pool)

	// Clean up any existing test data
	cleanupFeedTestData(t, pool)

	// Create a test user
	testUser := createFeedTestUser(t, userRepo)

	// Create a question with 2 answers
	question := createFeedTestPost(t, postRepo, "Test Question", models.PostTypeQuestion, models.PostStatusOpen, models.AuthorTypeHuman, testUser.ID)
	for i := 0; i < 2; i++ {
		_, err := answersRepo.CreateAnswer(ctx, &models.Answer{
			QuestionID: question.ID,
			AuthorType: models.AuthorTypeHuman,
			AuthorID:   testUser.ID,
			Content:    fmt.Sprintf("Answer %d", i+1),
		})
		if err != nil {
			t.Fatalf("failed to create answer: %v", err)
		}
	}

	// Create a problem with 3 approaches
	problem := createFeedTestPost(t, postRepo, "Test Problem", models.PostTypeProblem, models.PostStatusOpen, models.AuthorTypeHuman, testUser.ID)
	for i := 0; i < 3; i++ {
		_, err := approachesRepo.CreateApproach(ctx, &models.Approach{
			ProblemID:  problem.ID,
			AuthorType: models.AuthorTypeHuman,
			AuthorID:   testUser.ID,
			Angle:      fmt.Sprintf("Approach %d", i+1),
			Status:     models.ApproachStatusWorking,
		})
		if err != nil {
			t.Fatalf("failed to create approach: %v", err)
		}
	}

	// Create an idea with 4 responses
	idea := createFeedTestPost(t, postRepo, "Test Idea", models.PostTypeIdea, models.PostStatusActive, models.AuthorTypeHuman, testUser.ID)
	for i := 0; i < 4; i++ {
		_, err := responsesRepo.CreateResponse(ctx, &models.Response{
			IdeaID:       idea.ID,
			AuthorType:   models.AuthorTypeHuman,
			AuthorID:     testUser.ID,
			Content:      fmt.Sprintf("Response %d", i+1),
			ResponseType: models.ResponseTypeSupport,
		})
		if err != nil {
			t.Fatalf("failed to create response: %v", err)
		}
	}

	// Get recent activity
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

	// Verify comment counts for each post type
	foundQuestion := false
	foundProblem := false
	foundIdea := false

	for _, item := range items {
		switch item.Type {
		case "question":
			foundQuestion = true
			if item.AnswerCount != 2 {
				t.Errorf("expected question to have answer_count=2, got %d", item.AnswerCount)
			}
			if item.ApproachCount != 0 {
				t.Errorf("expected question to have approach_count=0, got %d", item.ApproachCount)
			}
		case "problem":
			foundProblem = true
			if item.ApproachCount != 3 {
				t.Errorf("expected problem to have approach_count=3, got %d", item.ApproachCount)
			}
			// Problems can have answer_count=0 or be nil, both are fine
		case "idea":
			foundIdea = true
			// Ideas use answer_count field for responses
			if item.AnswerCount != 4 {
				t.Errorf("expected idea to have answer_count=4 (responses), got %d", item.AnswerCount)
			}
			if item.ApproachCount != 0 {
				t.Errorf("expected idea to have approach_count=0, got %d", item.ApproachCount)
			}
		}
	}

	if !foundQuestion {
		t.Error("question not found in feed results")
	}
	if !foundProblem {
		t.Error("problem not found in feed results")
	}
	if !foundIdea {
		t.Error("idea not found in feed results")
	}

	// Clean up
	cleanupFeedTestData(t, pool)
}

// TestFeedRepository_GetStuckProblems_ApproachCounts tests that approach counts are correct for stuck problems.
func TestFeedRepository_GetStuckProblems_ApproachCounts(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	ctx := context.Background()
	feedRepo := NewFeedRepository(pool)
	postRepo := NewPostRepository(pool)
	userRepo := NewUserRepository(pool)
	approachesRepo := NewApproachesRepository(pool)

	// Clean up any existing test data
	cleanupFeedTestData(t, pool)

	// Create a test user
	testUser := createFeedTestUser(t, userRepo)

	// Create a problem that's stuck (has approach but old activity)
	// We'll mark it as in_progress which qualifies it as "stuck"
	problem := createFeedTestPost(t, postRepo, "Stuck Problem", models.PostTypeProblem, models.PostStatusInProgress, models.AuthorTypeHuman, testUser.ID)

	// Add 1 approach
	_, err := approachesRepo.CreateApproach(ctx, &models.Approach{
		ProblemID:  problem.ID,
		AuthorType: models.AuthorTypeHuman,
		AuthorID:   testUser.ID,
		Angle:      "Trying this approach",
		Status:     models.ApproachStatusWorking,
	})
	if err != nil {
		t.Fatalf("failed to create approach: %v", err)
	}

	// Get stuck problems
	items, _, err := feedRepo.GetStuckProblems(ctx, 1, 20)
	if err != nil {
		t.Fatalf("GetStuckProblems failed: %v", err)
	}

	// Find our problem
	found := false
	for _, item := range items {
		if item.ID == problem.ID {
			found = true
			if item.ApproachCount != 1 {
				t.Errorf("expected problem to have approach_count=1, got %d", item.ApproachCount)
			}
		}
	}

	if !found {
		t.Error("stuck problem not found in results")
	}

	// Clean up
	cleanupFeedTestData(t, pool)
}

// Helper function to clean up test data
func cleanupFeedTestData(t *testing.T, pool *Pool) {
	ctx := context.Background()
	// Clean in order: responses -> answers -> approaches -> posts -> agents -> users
	_, _ = pool.pool.Exec(ctx, "DELETE FROM responses")
	_, _ = pool.pool.Exec(ctx, "DELETE FROM answers")
	_, _ = pool.pool.Exec(ctx, "DELETE FROM approaches")
	_, _ = pool.pool.Exec(ctx, "DELETE FROM posts")
	_, _ = pool.pool.Exec(ctx, "DELETE FROM agents")
	_, _ = pool.pool.Exec(ctx, "DELETE FROM users")
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

// === Comment Count Tests (TDD: comment_count field in feed) ===

// TestFeedRepository_GetRecentActivity_IncludesCommentCount verifies that GetRecentActivity() returns comment count for each post.
// This test ensures that feed items display accurate comment counts.
func TestFeedRepository_GetRecentActivity_IncludesCommentCount(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	feedRepo := NewFeedRepository(pool)
	postsRepo := NewPostRepository(pool)
	commentsRepo := NewCommentsRepository(pool)
	ctx := context.Background()

	// Create a test post
	post, err := postsRepo.Create(ctx, &models.Post{
		Type:         models.PostTypeProblem,
		Title:        "Test Problem for Comments",
		Description:  "Test",
		PostedByType: models.AuthorTypeAgent,
		PostedByID:   "test_agent",
		Status:       models.PostStatusOpen,
	})
	if err != nil {
		t.Fatalf("failed to create test post: %v", err)
	}

	// Add 2 comments
	for i := 1; i <= 2; i++ {
		_, err = commentsRepo.Create(ctx, &models.Comment{
			TargetType: "post",
			TargetID:   post.ID,
			AuthorType: models.AuthorTypeAgent,
			AuthorID:   "test_agent",
			Content:    fmt.Sprintf("Comment %d", i),
		})
		if err != nil {
			t.Fatalf("failed to create comment %d: %v", i, err)
		}
	}

	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM comments WHERE target_id = $1", post.ID)
		_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE id = $1", post.ID)
	}()

	// Act: Get recent activity feed
	items, _, err := feedRepo.GetRecentActivity(ctx, 1, 10)
	if err != nil {
		t.Fatalf("GetRecentActivity() error = %v", err)
	}

	// Assert: Find the created post and verify comment count
	var found *models.FeedItem
	for i := range items {
		if items[i].ID == post.ID {
			found = &items[i]
			break
		}
	}

	if found == nil {
		t.Fatal("created post should be in feed")
	}

	if found.CommentCount != 2 {
		t.Errorf("expected CommentCount = 2, got %d", found.CommentCount)
	}
}
