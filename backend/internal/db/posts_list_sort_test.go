package db

import (
	"context"
	"os"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// === Sort Order and Count Correctness Tests ===
// These tests verify that sort=answers and sort=approaches produce correct ordering,
// and that answers_count/approaches_count exclude soft-deleted rows.

// TestPostRepository_List_SortByAnswers creates two questions with different answer counts
// and verifies that sort=answers orders them correctly (most answers first).
func TestPostRepository_List_SortByAnswers(t *testing.T) {
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
	authorAgent(ctx, t, pool, "sort_test_agent")

	repo := NewPostRepository(pool)

	// Create two questions via repo.Create (generates valid UUIDs)
	q1, err := repo.Create(ctx, &models.Post{
		Type: models.PostTypePost, Title: "Q1 Many Answers",
		Description: "Sort test q1", PostedByType: models.AuthorTypeAgent,
		PostedByID: "sort_test_agent", Status: models.PostStatusOpen,
		Tags: []string{"sort_answers_test"},
	})
	if err != nil {
		t.Fatalf("failed to create q1: %v", err)
	}

	q2, err := repo.Create(ctx, &models.Post{
		Type: models.PostTypePost, Title: "Q2 Few Answers",
		Description: "Sort test q2", PostedByType: models.AuthorTypeAgent,
		PostedByID: "sort_test_agent", Status: models.PostStatusOpen,
		Tags: []string{"sort_answers_test"},
	})
	if err != nil {
		t.Fatalf("failed to create q2: %v", err)
	}

	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM replies WHERE post_id IN ($1, $2)", q1.ID, q2.ID)
		_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE id IN ($1, $2)", q1.ID, q2.ID)
	}()

	// Add 3 answers to q1, 1 answer to q2
	for i := 0; i < 3; i++ {
		seedMigratedReply(t, pool, ctx, q1.ID, "answer", "agent", "sort_test_agent", "Answer for q1")
	}
	seedMigratedReply(t, pool, ctx, q2.ID, "answer", "agent", "sort_test_agent", "Answer for q2")

	// List with sort=answers, filtered by tag to isolate our test data
	posts, _, err := repo.List(ctx, models.PostListOptions{
		Type: models.PostTypePost, Sort: "answers",
		Tags: []string{"sort_answers_test"}, Page: 1, PerPage: 10,
	})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if len(posts) < 2 {
		t.Fatalf("expected at least 2 posts, got %d", len(posts))
	}

	// Find positions of q1 and q2
	q1Idx, q2Idx := -1, -1
	for i, p := range posts {
		if p.ID == q1.ID {
			q1Idx = i
		}
		if p.ID == q2.ID {
			q2Idx = i
		}
	}

	if q1Idx == -1 || q2Idx == -1 {
		t.Fatalf("expected both posts in results, q1Idx=%d, q2Idx=%d", q1Idx, q2Idx)
	}

	if q1Idx > q2Idx {
		t.Errorf("q1 (3 answers) should appear before q2 (1 answer), got q1 at %d, q2 at %d", q1Idx, q2Idx)
	}

	// Also verify answers_count values
	if posts[q1Idx].AnswersCount != 3 {
		t.Errorf("expected q1 answers_count=3, got %d", posts[q1Idx].AnswersCount)
	}
	if posts[q2Idx].AnswersCount != 1 {
		t.Errorf("expected q2 answers_count=1, got %d", posts[q2Idx].AnswersCount)
	}
}

// TestPostRepository_List_SortByApproaches creates two problems with different approach counts
// and verifies that sort=approaches orders them correctly (most approaches first).
func TestPostRepository_List_SortByApproaches(t *testing.T) {
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
	authorAgent(ctx, t, pool, "sort_test_agent")

	repo := NewPostRepository(pool)

	p1, err := repo.Create(ctx, &models.Post{
		Type: models.PostTypePost, Title: "P1 Many Approaches",
		Description: "Sort test p1", PostedByType: models.AuthorTypeAgent,
		PostedByID: "sort_test_agent", Status: models.PostStatusOpen,
		Tags: []string{"sort_approaches_test"},
	})
	if err != nil {
		t.Fatalf("failed to create p1: %v", err)
	}

	p2, err := repo.Create(ctx, &models.Post{
		Type: models.PostTypePost, Title: "P2 Few Approaches",
		Description: "Sort test p2", PostedByType: models.AuthorTypeAgent,
		PostedByID: "sort_test_agent", Status: models.PostStatusOpen,
		Tags: []string{"sort_approaches_test"},
	})
	if err != nil {
		t.Fatalf("failed to create p2: %v", err)
	}

	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE id IN ($1, $2)", p1.ID, p2.ID)
	}()

	// Add 3 approaches to p1, 1 approach to p2
	for i := 0; i < 3; i++ {
		seedMigratedReply(t, pool, ctx, p1.ID, "approach", "agent", "sort_test_agent", "Approach for p1")
	}
	seedMigratedReply(t, pool, ctx, p2.ID, "approach", "agent", "sort_test_agent", "Approach for p2")

	posts, _, err := repo.List(ctx, models.PostListOptions{
		Type: models.PostTypePost, Sort: "approaches",
		Tags: []string{"sort_approaches_test"}, Page: 1, PerPage: 10,
	})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if len(posts) < 2 {
		t.Fatalf("expected at least 2 posts, got %d", len(posts))
	}

	p1Idx, p2Idx := -1, -1
	for i, p := range posts {
		if p.ID == p1.ID {
			p1Idx = i
		}
		if p.ID == p2.ID {
			p2Idx = i
		}
	}

	if p1Idx == -1 || p2Idx == -1 {
		t.Fatalf("expected both posts in results, p1Idx=%d, p2Idx=%d", p1Idx, p2Idx)
	}

	if p1Idx > p2Idx {
		t.Errorf("p1 (3 approaches) should appear before p2 (1 approach), got p1 at %d, p2 at %d", p1Idx, p2Idx)
	}

	if posts[p1Idx].ApproachesCount != 3 {
		t.Errorf("expected p1 approaches_count=3, got %d", posts[p1Idx].ApproachesCount)
	}
	if posts[p2Idx].ApproachesCount != 1 {
		t.Errorf("expected p2 approaches_count=1, got %d", posts[p2Idx].ApproachesCount)
	}
}

// TestPostRepository_List_CountsExcludeSoftDeleted verifies that answers_count
// does not include soft-deleted answers.
func TestPostRepository_List_CountsExcludeSoftDeleted(t *testing.T) {
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
	authorAgent(ctx, t, pool, "count_test_agent")

	repo := NewPostRepository(pool)

	q, err := repo.Create(ctx, &models.Post{
		Type: models.PostTypePost, Title: "Q Soft Delete Count",
		Description: "Count test", PostedByType: models.AuthorTypeAgent,
		PostedByID: "count_test_agent", Status: models.PostStatusOpen,
		Tags: []string{"count_softdelete_test"},
	})
	if err != nil {
		t.Fatalf("failed to create question: %v", err)
	}

	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM replies WHERE post_id = $1", q.ID)
		_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE id = $1", q.ID)
	}()

	// Insert 3 answers
	var answers []string
	for i := 0; i < 3; i++ {
		answers = append(answers, seedMigratedReply(t, pool, ctx, q.ID, "answer", "agent", "count_test_agent", "Answer"))
	}

	// Soft-delete one answer
	_, err = pool.Exec(ctx, `UPDATE replies SET deleted_at = NOW() WHERE id = $1`, answers[0])
	if err != nil {
		t.Fatalf("failed to soft-delete answer: %v", err)
	}

	// List and verify count is 2 (not 3)
	posts, _, err := repo.List(ctx, models.PostListOptions{
		Tags: []string{"count_softdelete_test"}, Page: 1, PerPage: 10,
	})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	for _, p := range posts {
		if p.ID == q.ID {
			if p.AnswersCount != 2 {
				t.Errorf("expected answers_count=2 (1 soft-deleted), got %d", p.AnswersCount)
			}
			return
		}
	}
	t.Error("test question not found in results")
}

// === Vote Sorting Tests (TDD: sort=top parameter) ===

// TestPostRepository_List_SortByTop verifies that sort="top" orders posts by vote score descending.
// This test will FAIL initially (RED phase) until backend implements "top" as an alias for "votes".
func TestPostRepository_List_SortByTop(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewPostRepository(pool)
	ctx := context.Background()
	authorAgent(ctx, t, pool, "test_agent_sort_top")

	// Create 3 posts via repo.Create (generates valid UUIDs)
	postHigh, err := repo.Create(ctx, &models.Post{
		Type:         models.PostTypePost,
		Title:        "High Votes Post",
		Description:  "This post has 5 upvotes",
		PostedByType: models.AuthorTypeAgent,
		PostedByID:   "test_agent_sort_top",
		Status:       models.PostStatusOpen,
		Tags:         []string{"sort_top_test"},
	})
	if err != nil {
		t.Fatalf("failed to create high votes post: %v", err)
	}

	postMed, err := repo.Create(ctx, &models.Post{
		Type:         models.PostTypePost,
		Title:        "Med Votes Post",
		Description:  "This post has 2 net votes (3 up, 1 down)",
		PostedByType: models.AuthorTypeAgent,
		PostedByID:   "test_agent_sort_top",
		Status:       models.PostStatusOpen,
		Tags:         []string{"sort_top_test"},
	})
	if err != nil {
		t.Fatalf("failed to create med votes post: %v", err)
	}

	postLow, err := repo.Create(ctx, &models.Post{
		Type:         models.PostTypePost,
		Title:        "Low Votes Post",
		Description:  "This post has 1 upvote",
		PostedByType: models.AuthorTypeAgent,
		PostedByID:   "test_agent_sort_top",
		Status:       models.PostStatusOpen,
		Tags:         []string{"sort_top_test"},
	})
	if err != nil {
		t.Fatalf("failed to create low votes post: %v", err)
	}

	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE id IN ($1, $2, $3)", postHigh.ID, postMed.ID, postLow.ID)
	}()

	// Update vote counts directly in database
	// High: 5 upvotes, 0 downvotes (score: 5)
	_, err = pool.Exec(ctx, "UPDATE posts SET upvotes = 5, downvotes = 0 WHERE id = $1", postHigh.ID)
	if err != nil {
		t.Fatalf("failed to update high post votes: %v", err)
	}

	// Med: 3 upvotes, 1 downvote (score: 2)
	_, err = pool.Exec(ctx, "UPDATE posts SET upvotes = 3, downvotes = 1 WHERE id = $1", postMed.ID)
	if err != nil {
		t.Fatalf("failed to update med post votes: %v", err)
	}

	// Low: 1 upvote, 0 downvotes (score: 1)
	_, err = pool.Exec(ctx, "UPDATE posts SET upvotes = 1, downvotes = 0 WHERE id = $1", postLow.ID)
	if err != nil {
		t.Fatalf("failed to update low post votes: %v", err)
	}

	// Execute: List with sort="top"
	posts, _, err := repo.List(ctx, models.PostListOptions{
		Sort:    "top",
		Tags:    []string{"sort_top_test"},
		PerPage: 10,
	})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	// Assert: Posts returned in descending vote score order
	if len(posts) < 3 {
		t.Fatalf("expected at least 3 posts, got %d", len(posts))
	}

	// Find our test posts in results
	var foundHigh, foundMed, foundLow int = -1, -1, -1
	for i, post := range posts {
		if post.ID == postHigh.ID {
			foundHigh = i
		} else if post.ID == postMed.ID {
			foundMed = i
		} else if post.ID == postLow.ID {
			foundLow = i
		}
	}

	if foundHigh == -1 || foundMed == -1 || foundLow == -1 {
		t.Fatalf("not all test posts found: high=%d, med=%d, low=%d", foundHigh, foundMed, foundLow)
	}

	// Verify order: High (5) before Med (2) before Low (1)
	if foundHigh > foundMed {
		t.Errorf("high votes post (score 5) should appear before med votes post (score 2): high at %d, med at %d", foundHigh, foundMed)
	}
	if foundMed > foundLow {
		t.Errorf("med votes post (score 2) should appear before low votes post (score 1): med at %d, low at %d", foundMed, foundLow)
	}

	// Verify vote scores are calculated correctly
	if posts[foundHigh].VoteScore != 5 {
		t.Errorf("high post: expected vote score 5, got %d", posts[foundHigh].VoteScore)
	}
	if posts[foundMed].VoteScore != 2 {
		t.Errorf("med post: expected vote score 2, got %d", posts[foundMed].VoteScore)
	}
	if posts[foundLow].VoteScore != 1 {
		t.Errorf("low post: expected vote score 1, got %d", posts[foundLow].VoteScore)
	}
}

// TestPostRepository_List_SortByVotes verifies that sort="votes" still works correctly.
// This ensures we don't break existing functionality when adding "top" alias.
func TestPostRepository_List_SortByVotes(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewPostRepository(pool)
	ctx := context.Background()
	authorAgent(ctx, t, pool, "test_agent_sort_votes")

	// Create 3 posts
	postHigh, err := repo.Create(ctx, &models.Post{
		Type:         models.PostTypePost,
		Title:        "High Votes Post (votes param)",
		Description:  "Testing sort=votes",
		PostedByType: models.AuthorTypeAgent,
		PostedByID:   "test_agent_sort_votes",
		Status:       models.PostStatusOpen,
		Tags:         []string{"sort_votes_test"},
	})
	if err != nil {
		t.Fatalf("failed to create high votes post: %v", err)
	}

	postLow, err := repo.Create(ctx, &models.Post{
		Type:         models.PostTypePost,
		Title:        "Low Votes Post (votes param)",
		Description:  "Testing sort=votes",
		PostedByType: models.AuthorTypeAgent,
		PostedByID:   "test_agent_sort_votes",
		Status:       models.PostStatusOpen,
		Tags:         []string{"sort_votes_test"},
	})
	if err != nil {
		t.Fatalf("failed to create low votes post: %v", err)
	}

	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE id IN ($1, $2)", postHigh.ID, postLow.ID)
	}()

	// Set vote counts: High=10, Low=1
	_, err = pool.Exec(ctx, "UPDATE posts SET upvotes = 10, downvotes = 0 WHERE id = $1", postHigh.ID)
	if err != nil {
		t.Fatalf("failed to update high post votes: %v", err)
	}
	_, err = pool.Exec(ctx, "UPDATE posts SET upvotes = 1, downvotes = 0 WHERE id = $1", postLow.ID)
	if err != nil {
		t.Fatalf("failed to update low post votes: %v", err)
	}

	// Execute: List with sort="votes" (existing parameter)
	posts, _, err := repo.List(ctx, models.PostListOptions{
		Sort:    "votes",
		Tags:    []string{"sort_votes_test"},
		PerPage: 10,
	})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if len(posts) < 2 {
		t.Fatalf("expected at least 2 posts, got %d", len(posts))
	}

	// Find positions
	var foundHigh, foundLow int = -1, -1
	for i, post := range posts {
		if post.ID == postHigh.ID {
			foundHigh = i
		} else if post.ID == postLow.ID {
			foundLow = i
		}
	}

	if foundHigh == -1 || foundLow == -1 {
		t.Fatalf("not all test posts found: high=%d, low=%d", foundHigh, foundLow)
	}

	// Verify order: High before Low
	if foundHigh > foundLow {
		t.Errorf("high votes post should appear before low votes post: high at %d, low at %d", foundHigh, foundLow)
	}
}

// TestPostRepository_List_SortByTop_WithNegativeScores verifies that sort="top" correctly
// handles posts with negative vote scores (more downvotes than upvotes).
func TestPostRepository_List_SortByTop_WithNegativeScores(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewPostRepository(pool)
	ctx := context.Background()
	authorAgent(ctx, t, pool, "test_agent_negative")

	// Create 3 posts with positive, zero, and negative scores
	postPositive, err := repo.Create(ctx, &models.Post{
		Type:         models.PostTypePost,
		Title:        "Positive Score Post",
		Description:  "10 up, 2 down = score 8",
		PostedByType: models.AuthorTypeAgent,
		PostedByID:   "test_agent_negative",
		Status:       models.PostStatusOpen,
		Tags:         []string{"negative_score_test"},
	})
	if err != nil {
		t.Fatalf("failed to create positive post: %v", err)
	}

	postZero, err := repo.Create(ctx, &models.Post{
		Type:         models.PostTypePost,
		Title:        "Zero Score Post",
		Description:  "5 up, 5 down = score 0",
		PostedByType: models.AuthorTypeAgent,
		PostedByID:   "test_agent_negative",
		Status:       models.PostStatusOpen,
		Tags:         []string{"negative_score_test"},
	})
	if err != nil {
		t.Fatalf("failed to create zero post: %v", err)
	}

	postNegative, err := repo.Create(ctx, &models.Post{
		Type:         models.PostTypePost,
		Title:        "Negative Score Post",
		Description:  "2 up, 5 down = score -3",
		PostedByType: models.AuthorTypeAgent,
		PostedByID:   "test_agent_negative",
		Status:       models.PostStatusOpen,
		Tags:         []string{"negative_score_test"},
	})
	if err != nil {
		t.Fatalf("failed to create negative post: %v", err)
	}

	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE id IN ($1, $2, $3)", postPositive.ID, postZero.ID, postNegative.ID)
	}()

	// Set vote counts
	_, err = pool.Exec(ctx, "UPDATE posts SET upvotes = 10, downvotes = 2 WHERE id = $1", postPositive.ID)
	if err != nil {
		t.Fatalf("failed to update positive post votes: %v", err)
	}
	_, err = pool.Exec(ctx, "UPDATE posts SET upvotes = 5, downvotes = 5 WHERE id = $1", postZero.ID)
	if err != nil {
		t.Fatalf("failed to update zero post votes: %v", err)
	}
	_, err = pool.Exec(ctx, "UPDATE posts SET upvotes = 2, downvotes = 5 WHERE id = $1", postNegative.ID)
	if err != nil {
		t.Fatalf("failed to update negative post votes: %v", err)
	}

	// Execute: List with sort="top"
	posts, _, err := repo.List(ctx, models.PostListOptions{
		Sort:    "top",
		Tags:    []string{"negative_score_test"},
		PerPage: 10,
	})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if len(posts) < 3 {
		t.Fatalf("expected at least 3 posts, got %d", len(posts))
	}

	// Find positions
	var foundPos, foundZero, foundNeg int = -1, -1, -1
	for i, post := range posts {
		if post.ID == postPositive.ID {
			foundPos = i
		} else if post.ID == postZero.ID {
			foundZero = i
		} else if post.ID == postNegative.ID {
			foundNeg = i
		}
	}

	if foundPos == -1 || foundZero == -1 || foundNeg == -1 {
		t.Fatalf("not all test posts found: positive=%d, zero=%d, negative=%d", foundPos, foundZero, foundNeg)
	}

	// Verify order: Positive (8) before Zero (0) before Negative (-3)
	if foundPos > foundZero {
		t.Errorf("positive score (8) should appear before zero score (0): positive at %d, zero at %d", foundPos, foundZero)
	}
	if foundZero > foundNeg {
		t.Errorf("zero score (0) should appear before negative score (-3): zero at %d, negative at %d", foundZero, foundNeg)
	}

	// Verify vote scores
	if posts[foundPos].VoteScore != 8 {
		t.Errorf("positive post: expected vote score 8, got %d", posts[foundPos].VoteScore)
	}
	if posts[foundZero].VoteScore != 0 {
		t.Errorf("zero post: expected vote score 0, got %d", posts[foundZero].VoteScore)
	}
	if posts[foundNeg].VoteScore != -3 {
		t.Errorf("negative post: expected vote score -3, got %d", posts[foundNeg].VoteScore)
	}
}
