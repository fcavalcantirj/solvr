package db

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// TestIdeasCommentsCount verifies that ideas show correct comments_count.
// This is a TDD test to reproduce the bug where ideas show comments_count: 0
// when they actually have comments in the database.
func TestIdeasCommentsCount(t *testing.T) {
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

	// Create test user
	userRepo := NewUserRepository(pool)
	user := &models.User{
		Username:       "ideacmt" + time.Now().Format("150405"),
		DisplayName:    "Idea Tester",
		Email:          "ideatester" + time.Now().Format("20060102150405") + "@example.com",
		AuthProvider:   "github",
		AuthProviderID: "github_idea_" + time.Now().Format("20060102150405"),
		Role:           "user",
	}
	user, err = userRepo.Create(ctx, user)
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	// Create idea post
	postRepo := NewPostRepository(pool)
	idea := &models.Post{
		Type:         models.PostTypeIdea,
		Title:        "Test idea for comments count " + time.Now().Format("20060102150405.000000000"),
		Description:  "This is a test idea to verify comments_count works correctly",
		Tags:         []string{"test", "comments"},
		PostedByType: models.AuthorTypeHuman,
		PostedByID:   user.ID,
		Status:       models.PostStatusOpen,
	}
	idea, err = postRepo.Create(ctx, idea)
	if err != nil {
		t.Fatalf("failed to create test idea: %v", err)
	}

	// Create 3 comments on the idea
	commentsRepo := NewCommentsRepository(pool)
	for i := 0; i < 3; i++ {
		comment := &models.Comment{
			TargetType: models.CommentTargetPost,
			TargetID:   idea.ID,
			AuthorType: models.AuthorTypeHuman,
			AuthorID:   user.ID,
			Content:    "Test comment on idea",
		}
		_, err = commentsRepo.Create(ctx, comment)
		if err != nil {
			t.Fatalf("failed to create comment %d: %v", i+1, err)
		}
	}

	// Query the idea through List() - this is what the API uses
	ideas, _, err := postRepo.List(ctx, models.PostListOptions{
		Type:    models.PostTypeIdea,
		PerPage: 50,
	})
	if err != nil {
		t.Fatalf("failed to list ideas: %v", err)
	}

	// Find our test idea
	var foundIdea *models.PostWithAuthor
	for i := range ideas {
		if ideas[i].ID == idea.ID {
			foundIdea = &ideas[i]
			break
		}
	}

	if foundIdea == nil {
		t.Fatalf("test idea not found in list results")
	}

	// TDD ASSERTION: Idea should show 3 comments
	if foundIdea.CommentsCount != 3 {
		t.Errorf("FAIL: Expected idea to have comments_count=3, got %d", foundIdea.CommentsCount)

		// Debug: Check database directly
		var actualCount int
		err = pool.QueryRow(ctx,
			"SELECT COUNT(*) FROM comments WHERE target_id = $1 AND target_type = 'post' AND deleted_at IS NULL",
			idea.ID).Scan(&actualCount)
		if err != nil {
			t.Errorf("failed to query actual comment count: %v", err)
		} else {
			t.Errorf("Database has %d comments, but API returned %d", actualCount, foundIdea.CommentsCount)
		}
	}

	// Also test that the idea has correct other counts
	if foundIdea.AnswersCount != 0 {
		t.Errorf("idea should have 0 answers, got %d", foundIdea.AnswersCount)
	}
	if foundIdea.ApproachesCount != 0 {
		t.Errorf("idea should have 0 approaches, got %d", foundIdea.ApproachesCount)
	}
}

// TestQuestionsProblemssCommentsCount verifies that questions and problems also show correct comments_count.
// This ensures the fix doesn't break existing functionality.
func TestQuestionsProblemsCommentsCount(t *testing.T) {
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

	// Create test user
	userRepo := NewUserRepository(pool)
	user := &models.User{
		Username:       "comtst" + time.Now().Format("150405"),
		DisplayName:    "Comment Tester",
		Email:          "commtester" + time.Now().Format("20060102150405") + "@example.com",
		AuthProvider:   "github",
		AuthProviderID: "github_comm_" + time.Now().Format("20060102150405"),
		Role:           "user",
	}
	user, err = userRepo.Create(ctx, user)
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	postRepo := NewPostRepository(pool)
	commentsRepo := NewCommentsRepository(pool)

	// Test question with comments
	question := &models.Post{
		Type:         models.PostTypeQuestion,
		Title:        "Test question " + time.Now().Format("20060102150405.000000000"),
		Description:  "Test question",
		Tags:         []string{"test"},
		PostedByType: models.AuthorTypeHuman,
		PostedByID:   user.ID,
		Status:       models.PostStatusOpen,
	}
	question, err = postRepo.Create(ctx, question)
	if err != nil {
		t.Fatalf("failed to create question: %v", err)
	}

	// Add 2 comments to question
	for i := 0; i < 2; i++ {
		comment := &models.Comment{
			TargetType: models.CommentTargetPost,
			TargetID:   question.ID,
			AuthorType: models.AuthorTypeHuman,
			AuthorID:   user.ID,
			Content:    "Comment on question",
		}
		_, err = commentsRepo.Create(ctx, comment)
		if err != nil {
			t.Fatalf("failed to create comment on question: %v", err)
		}
	}

	// Test problem with comments
	problem := &models.Post{
		Type:         models.PostTypeProblem,
		Title:        "Test problem " + time.Now().Format("20060102150405.000000000"),
		Description:  "Test problem",
		Tags:         []string{"test"},
		PostedByType: models.AuthorTypeHuman,
		PostedByID:   user.ID,
		Status:       models.PostStatusOpen,
	}
	problem, err = postRepo.Create(ctx, problem)
	if err != nil {
		t.Fatalf("failed to create problem: %v", err)
	}

	// Add 4 comments to problem
	for i := 0; i < 4; i++ {
		comment := &models.Comment{
			TargetType: models.CommentTargetPost,
			TargetID:   problem.ID,
			AuthorType: models.AuthorTypeHuman,
			AuthorID:   user.ID,
			Content:    "Comment on problem",
		}
		_, err = commentsRepo.Create(ctx, comment)
		if err != nil {
			t.Fatalf("failed to create comment on problem: %v", err)
		}
	}

	// Query questions
	questions, _, err := postRepo.List(ctx, models.PostListOptions{
		Type:    models.PostTypeQuestion,
		PerPage: 50,
	})
	if err != nil {
		t.Fatalf("failed to list questions: %v", err)
	}

	var foundQuestion *models.PostWithAuthor
	for i := range questions {
		if questions[i].ID == question.ID {
			foundQuestion = &questions[i]
			break
		}
	}

	if foundQuestion == nil {
		t.Fatalf("test question not found")
	}

	if foundQuestion.CommentsCount != 2 {
		t.Errorf("question should have comments_count=2, got %d", foundQuestion.CommentsCount)
	}

	// Query problems
	problems, _, err := postRepo.List(ctx, models.PostListOptions{
		Type:    models.PostTypeProblem,
		PerPage: 50,
	})
	if err != nil {
		t.Fatalf("failed to list problems: %v", err)
	}

	var foundProblem *models.PostWithAuthor
	for i := range problems {
		if problems[i].ID == problem.ID {
			foundProblem = &problems[i]
			break
		}
	}

	if foundProblem == nil {
		t.Fatalf("test problem not found")
	}

	if foundProblem.CommentsCount != 4 {
		t.Errorf("problem should have comments_count=4, got %d", foundProblem.CommentsCount)
	}
}
