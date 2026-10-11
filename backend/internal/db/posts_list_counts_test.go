package db

import (
	"context"
	"fmt"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/models"
)

func TestPostRepository_List_HasAnswerFalse(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewPostRepository(pool)
	ctx := context.Background()
	authorAgent(ctx, t, pool, "test_agent")

	// Create 3 questions with different answer counts
	// q1: 0 answers
	q1, err := repo.Create(ctx, &models.Post{
		Type:         models.PostTypePost,
		Title:        "Question with 0 answers",
		Description:  "Test question with no answers",
		PostedByType: models.AuthorTypeAgent,
		PostedByID:   "test_agent",
		Status:       models.PostStatusOpen,
	})
	if err != nil {
		t.Fatalf("failed to create q1: %v", err)
	}

	// q2: 1 answer
	q2, err := repo.Create(ctx, &models.Post{
		Type:         models.PostTypePost,
		Title:        "Question with 1 answer",
		Description:  "Test question with one answer",
		PostedByType: models.AuthorTypeAgent,
		PostedByID:   "test_agent",
		Status:       models.PostStatusOpen,
	})
	if err != nil {
		t.Fatalf("failed to create q2: %v", err)
	}

	seedMigratedReply(t, pool, ctx, q2.ID, "answer", string(models.AuthorTypeAgent), "test_agent", "Answer 1")

	// q3: 3 answers
	q3, err := repo.Create(ctx, &models.Post{
		Type:         models.PostTypePost,
		Title:        "Question with 3 answers",
		Description:  "Test question with three answers",
		PostedByType: models.AuthorTypeAgent,
		PostedByID:   "test_agent",
		Status:       models.PostStatusOpen,
	})
	if err != nil {
		t.Fatalf("failed to create q3: %v", err)
	}

	for i := 1; i <= 3; i++ {
		seedMigratedReply(t, pool, ctx, q3.ID, "answer", string(models.AuthorTypeAgent), "test_agent", "Answer "+string(rune('0'+i)))
	}

	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM replies WHERE post_id IN ($1, $2, $3)", q1.ID, q2.ID, q3.ID)
		_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE id IN ($1, $2, $3)", q1.ID, q2.ID, q3.ID)
	}()

	// Act: List with HasAnswer = false
	falseVal := false
	opts := models.PostListOptions{
		Type:      models.PostTypePost,
		HasAnswer: &falseVal,
		Page:      1,
		PerPage:   100,
	}

	posts, total, err := repo.List(ctx, opts)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	// Assert: Only q1 (0 answers) should be returned
	if total == 0 {
		t.Errorf("expected at least 1 question, got %d", total)
	}

	// Find q1 in results
	found := false
	for _, post := range posts {
		if post.ID == q1.ID {
			found = true
			if post.AnswersCount != 0 {
				t.Errorf("expected q1 to have 0 answers, got %d", post.AnswersCount)
			}
		}
		// Verify all returned posts have 0 answers
		if post.AnswersCount != 0 {
			t.Errorf("HasAnswer=false should only return questions with 0 answers, got question %s with %d answers", post.ID, post.AnswersCount)
		}
	}

	if !found {
		t.Errorf("expected q1 (0 answers) to be in results")
	}
}

func TestPostRepository_List_HasAnswerTrue(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewPostRepository(pool)
	ctx := context.Background()
	authorAgent(ctx, t, pool, "test_agent")

	// Create 3 questions with different answer counts
	// q1: 0 answers
	q1, err := repo.Create(ctx, &models.Post{
		Type:         models.PostTypePost,
		Title:        "Question with 0 answers",
		Description:  "Test question with no answers",
		PostedByType: models.AuthorTypeAgent,
		PostedByID:   "test_agent",
		Status:       models.PostStatusOpen,
	})
	if err != nil {
		t.Fatalf("failed to create q1: %v", err)
	}

	// q2: 1 answer
	q2, err := repo.Create(ctx, &models.Post{
		Type:         models.PostTypePost,
		Title:        "Question with 1 answer",
		Description:  "Test question with one answer",
		PostedByType: models.AuthorTypeAgent,
		PostedByID:   "test_agent",
		Status:       models.PostStatusOpen,
	})
	if err != nil {
		t.Fatalf("failed to create q2: %v", err)
	}

	seedMigratedReply(t, pool, ctx, q2.ID, "answer", string(models.AuthorTypeAgent), "test_agent", "Answer 1")

	// q3: 3 answers
	q3, err := repo.Create(ctx, &models.Post{
		Type:         models.PostTypePost,
		Title:        "Question with 3 answers",
		Description:  "Test question with three answers",
		PostedByType: models.AuthorTypeAgent,
		PostedByID:   "test_agent",
		Status:       models.PostStatusOpen,
	})
	if err != nil {
		t.Fatalf("failed to create q3: %v", err)
	}

	for i := 1; i <= 3; i++ {
		seedMigratedReply(t, pool, ctx, q3.ID, "answer", string(models.AuthorTypeAgent), "test_agent", "Answer "+string(rune('0'+i)))
	}

	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM replies WHERE post_id IN ($1, $2, $3)", q1.ID, q2.ID, q3.ID)
		_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE id IN ($1, $2, $3)", q1.ID, q2.ID, q3.ID)
	}()

	// Act: List with HasAnswer = true
	trueVal := true
	opts := models.PostListOptions{
		Type:      models.PostTypePost,
		HasAnswer: &trueVal,
		Page:      1,
		PerPage:   100,
	}

	posts, total, err := repo.List(ctx, opts)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	// Assert: q2 and q3 (with answers) should be returned
	if total == 0 {
		t.Errorf("expected at least 2 questions, got %d", total)
	}

	// Verify all returned posts have 1+ answers
	for _, post := range posts {
		if post.AnswersCount == 0 {
			t.Errorf("HasAnswer=true should only return questions with 1+ answers, got question %s with 0 answers", post.ID)
		}
	}

	// Verify q1 (0 answers) is NOT in results
	for _, post := range posts {
		if post.ID == q1.ID {
			t.Errorf("q1 (0 answers) should not be in results for HasAnswer=true")
		}
	}
}

func TestPostRepository_List_NoHasAnswerFilter(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewPostRepository(pool)
	ctx := context.Background()
	authorAgent(ctx, t, pool, "test_agent")

	// Create 3 questions with different answer counts
	// q1: 0 answers
	q1, err := repo.Create(ctx, &models.Post{
		Type:         models.PostTypePost,
		Title:        "Question with 0 answers",
		Description:  "Test question with no answers",
		PostedByType: models.AuthorTypeAgent,
		PostedByID:   "test_agent",
		Status:       models.PostStatusOpen,
	})
	if err != nil {
		t.Fatalf("failed to create q1: %v", err)
	}

	// q2: 1 answer
	q2, err := repo.Create(ctx, &models.Post{
		Type:         models.PostTypePost,
		Title:        "Question with 1 answer",
		Description:  "Test question with one answer",
		PostedByType: models.AuthorTypeAgent,
		PostedByID:   "test_agent",
		Status:       models.PostStatusOpen,
	})
	if err != nil {
		t.Fatalf("failed to create q2: %v", err)
	}

	seedMigratedReply(t, pool, ctx, q2.ID, "answer", string(models.AuthorTypeAgent), "test_agent", "Answer 1")

	// q3: 3 answers
	q3, err := repo.Create(ctx, &models.Post{
		Type:         models.PostTypePost,
		Title:        "Question with 3 answers",
		Description:  "Test question with three answers",
		PostedByType: models.AuthorTypeAgent,
		PostedByID:   "test_agent",
		Status:       models.PostStatusOpen,
	})
	if err != nil {
		t.Fatalf("failed to create q3: %v", err)
	}

	for i := 1; i <= 3; i++ {
		seedMigratedReply(t, pool, ctx, q3.ID, "answer", string(models.AuthorTypeAgent), "test_agent", "Answer "+string(rune('0'+i)))
	}

	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM replies WHERE post_id IN ($1, $2, $3)", q1.ID, q2.ID, q3.ID)
		_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE id IN ($1, $2, $3)", q1.ID, q2.ID, q3.ID)
	}()

	// Act: List without HasAnswer parameter (nil)
	opts := models.PostListOptions{
		Type:    models.PostTypePost,
		Page:    1,
		PerPage: 100,
	}

	posts, total, err := repo.List(ctx, opts)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	// Assert: All 3 questions should be returned
	if total == 0 {
		t.Errorf("expected at least 3 questions, got %d", total)
	}

	// Count how many of our test questions are in the results
	foundCount := 0
	for _, post := range posts {
		if post.ID == q1.ID || post.ID == q2.ID || post.ID == q3.ID {
			foundCount++
		}
	}

	if foundCount != 3 {
		t.Errorf("expected all 3 test questions to be in results, found %d", foundCount)
	}
}

// === Comment Count Tests (TDD: comments_count field) ===

// TestPostRepository_List_IncludesCommentCount verifies that List() returns comment count for each post.
// This test ensures that posts display accurate comment counts on list pages (feed, problems, questions, ideas).
func TestPostRepository_List_IncludesCommentCount(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewPostRepository(pool)
	ctx := context.Background()
	authorAgent(ctx, t, pool, "test_agent")

	// Create a test post
	post, err := repo.Create(ctx, &models.Post{
		Type:         models.PostTypePost,
		Title:        "Test Question for Comments",
		Description:  "Test",
		PostedByType: models.AuthorTypeAgent,
		PostedByID:   "test_agent",
		Status:       models.PostStatusOpen,
	})
	if err != nil {
		t.Fatalf("failed to create test post: %v", err)
	}

	// Add 3 comments to the post
	for i := 1; i <= 3; i++ {
		seedMigratedReply(t, pool, ctx, post.ID, "comment", string(models.AuthorTypeAgent), "test_agent", fmt.Sprintf("Comment %d", i))
	}

	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM replies WHERE post_id = $1", post.ID)
		_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE id = $1", post.ID)
	}()

	// Act: List posts
	posts, _, err := repo.List(ctx, models.PostListOptions{
		Type:    models.PostTypePost,
		Page:    1,
		PerPage: 10,
	})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	// Assert: Find the created post and verify comment count
	var found *models.PostWithAuthor
	for i := range posts {
		if posts[i].ID == post.ID {
			found = &posts[i]
			break
		}
	}

	if found == nil {
		t.Fatal("created post should be in results")
	}

	if found.CommentsCount != 3 {
		t.Errorf("expected CommentsCount = 3, got %d", found.CommentsCount)
	}
}

// TestPostRepository_List_CommentsCountAllTypes verifies that comments_count works for ALL post types.
// BUG: Frontend shows comments_count: 0 for Problems and Ideas, but correct count for Questions.
// This test reproduces the issue by testing all three post types with the same query path.
func TestPostRepository_List_CommentsCountAllTypes(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewPostRepository(pool)
	ctx := context.Background()
	authorAgent(ctx, t, pool, "test_agent")

	// Create one post of each type
	question, err := repo.Create(ctx, &models.Post{
		Type:         models.PostTypePost,
		Title:        "Question with comments",
		Description:  "Test question",
		PostedByType: models.AuthorTypeAgent,
		PostedByID:   "test_agent",
		Status:       models.PostStatusOpen,
	})
	if err != nil {
		t.Fatalf("failed to create question: %v", err)
	}

	problem, err := repo.Create(ctx, &models.Post{
		Type:         models.PostTypePost,
		Title:        "Problem with comments",
		Description:  "Test problem",
		PostedByType: models.AuthorTypeAgent,
		PostedByID:   "test_agent",
		Status:       models.PostStatusOpen,
	})
	if err != nil {
		t.Fatalf("failed to create problem: %v", err)
	}

	idea, err := repo.Create(ctx, &models.Post{
		Type:         models.PostTypePost,
		Title:        "Idea with comments",
		Description:  "Test idea",
		PostedByType: models.AuthorTypeAgent,
		PostedByID:   "test_agent",
		Status:       models.PostStatusOpen,
	})
	if err != nil {
		t.Fatalf("failed to create idea: %v", err)
	}

	// Add 1 comment to each post
	seedMigratedReply(t, pool, ctx, question.ID, "comment", string(models.AuthorTypeAgent), "test_agent", "Comment on question")

	seedMigratedReply(t, pool, ctx, problem.ID, "comment", string(models.AuthorTypeAgent), "test_agent", "Comment on problem")

	seedMigratedReply(t, pool, ctx, idea.ID, "comment", string(models.AuthorTypeAgent), "test_agent", "Comment on idea")

	// Cleanup
	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM replies WHERE post_id IN ($1, $2, $3)", question.ID, problem.ID, idea.ID)
		_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE id IN ($1, $2, $3)", question.ID, problem.ID, idea.ID)
	}()

	// Test Questions - Should return comments_count = 1
	questions, _, err := repo.List(ctx, models.PostListOptions{
		Type:    models.PostTypePost,
		Page:    1,
		PerPage: 10,
	})
	if err != nil {
		t.Fatalf("List(questions) error = %v", err)
	}

	var foundQuestion *models.PostWithAuthor
	for i := range questions {
		if questions[i].ID == question.ID {
			foundQuestion = &questions[i]
			break
		}
	}
	if foundQuestion == nil {
		t.Fatal("question should be in results")
	}
	if foundQuestion.CommentsCount != 1 {
		t.Errorf("Questions: expected CommentsCount = 1, got %d", foundQuestion.CommentsCount)
	}

	// Test Problems - Should return comments_count = 1
	problems, _, err := repo.List(ctx, models.PostListOptions{
		Type:    models.PostTypePost,
		Page:    1,
		PerPage: 10,
	})
	if err != nil {
		t.Fatalf("List(problems) error = %v", err)
	}

	var foundProblem *models.PostWithAuthor
	for i := range problems {
		if problems[i].ID == problem.ID {
			foundProblem = &problems[i]
			break
		}
	}
	if foundProblem == nil {
		t.Fatal("problem should be in results")
	}
	if foundProblem.CommentsCount != 1 {
		t.Errorf("Problems: expected CommentsCount = 1, got %d", foundProblem.CommentsCount)
	}

	// Test Ideas - Should return comments_count = 1
	ideas, _, err := repo.List(ctx, models.PostListOptions{
		Type:    models.PostTypePost,
		Page:    1,
		PerPage: 10,
	})
	if err != nil {
		t.Fatalf("List(ideas) error = %v", err)
	}

	var foundIdea *models.PostWithAuthor
	for i := range ideas {
		if ideas[i].ID == idea.ID {
			foundIdea = &ideas[i]
			break
		}
	}
	if foundIdea == nil {
		t.Fatal("idea should be in results")
	}
	if foundIdea.CommentsCount != 1 {
		t.Errorf("Ideas: expected CommentsCount = 1, got %d", foundIdea.CommentsCount)
	}
}

func TestPostRepository_List_WithViewerVote(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewPostRepository(pool)
	ctx := context.Background()
	authorAgent(ctx, t, pool, "test_agent_viewer_vote")

	// Create a post
	post := &models.Post{
		Type:         models.PostTypePost,
		Title:        "List ViewerVote Test",
		Description:  "Testing user_vote in List results",
		PostedByType: models.AuthorTypeAgent,
		PostedByID:   "test_agent_viewer_vote",
		Status:       models.PostStatusOpen,
	}

	createdPost, err := repo.Create(ctx, post)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM votes WHERE target_id = $1", createdPost.ID)
		_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE id = $1", createdPost.ID)
	}()

	// Vote on it as voter-A (upvote)
	err = repo.Vote(ctx, createdPost.ID, "agent", "viewer_vote_agent_a", "up")
	if err != nil {
		t.Fatalf("Vote() error = %v", err)
	}

	// List with viewer info — should show user_vote = "up"
	posts, _, err := repo.List(ctx, models.PostListOptions{
		Page:       1,
		PerPage:    100,
		ViewerType: models.AuthorTypeAgent,
		ViewerID:   "viewer_vote_agent_a",
	})
	if err != nil {
		t.Fatalf("List() with viewer error = %v", err)
	}

	var found *models.PostWithAuthor
	for i := range posts {
		if posts[i].ID == createdPost.ID {
			found = &posts[i]
			break
		}
	}
	if found == nil {
		t.Fatal("expected to find created post in list results")
	}
	if found.UserVote == nil {
		t.Fatal("expected UserVote to be non-nil for authenticated viewer who voted")
	}
	if *found.UserVote != "up" {
		t.Errorf("expected UserVote = 'up', got '%s'", *found.UserVote)
	}

	// List without viewer info — should have nil UserVote
	postsAnon, _, err := repo.List(ctx, models.PostListOptions{
		Page:    1,
		PerPage: 100,
	})
	if err != nil {
		t.Fatalf("List() anonymous error = %v", err)
	}

	var foundAnon *models.PostWithAuthor
	for i := range postsAnon {
		if postsAnon[i].ID == createdPost.ID {
			foundAnon = &postsAnon[i]
			break
		}
	}
	if foundAnon == nil {
		t.Fatal("expected to find created post in anonymous list results")
	}
	if foundAnon.UserVote != nil {
		t.Errorf("expected UserVote to be nil for anonymous viewer, got '%s'", *foundAnon.UserVote)
	}

	// List with a different viewer — should have nil UserVote
	postsOther, _, err := repo.List(ctx, models.PostListOptions{
		Page:       1,
		PerPage:    100,
		ViewerType: models.AuthorTypeAgent,
		ViewerID:   "viewer_vote_agent_b",
	})
	if err != nil {
		t.Fatalf("List() other viewer error = %v", err)
	}

	var foundOther *models.PostWithAuthor
	for i := range postsOther {
		if postsOther[i].ID == createdPost.ID {
			foundOther = &postsOther[i]
			break
		}
	}
	if foundOther == nil {
		t.Fatal("expected to find created post for other viewer")
	}
	if foundOther.UserVote != nil {
		t.Errorf("expected UserVote to be nil for non-voter, got '%s'", *foundOther.UserVote)
	}
}
