// Package db provides database access for Solvr.
package db

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/jackc/pgx/v5/pgconn"
)

// Unit tests for helper functions (no database required)

func TestIsInvalidUUIDError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "generic error",
			err:      errors.New("some error"),
			expected: false,
		},
		{
			name: "invalid UUID error (22P02)",
			err: &pgconn.PgError{
				Code:    "22P02",
				Message: "invalid input syntax for type uuid",
			},
			expected: true,
		},
		{
			name: "other pg error",
			err: &pgconn.PgError{
				Code:    "23505",
				Message: "unique constraint violation",
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isInvalidUUIDError(tt.err)
			if result != tt.expected {
				t.Errorf("isInvalidUUIDError() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestIsTableNotFoundError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "generic error",
			err:      errors.New("some error"),
			expected: false,
		},
		{
			name: "table not found error (42P01)",
			err: &pgconn.PgError{
				Code:    "42P01",
				Message: "relation \"responses\" does not exist",
			},
			expected: true,
		},
		{
			name: "other pg error",
			err: &pgconn.PgError{
				Code:    "23505",
				Message: "unique constraint violation",
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isTableNotFoundError(tt.err)
			if result != tt.expected {
				t.Errorf("isTableNotFoundError() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// Note: These tests require a running PostgreSQL database.
// Set DATABASE_URL environment variable to run integration tests.
// Tests will be skipped if DATABASE_URL is not set.

func TestPostRepository_List_Empty(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewPostRepository(pool)
	ctx := context.Background()

	// List posts when none exist
	opts := models.PostListOptions{
		Page:    1,
		PerPage: 10,
	}

	posts, total, err := repo.List(ctx, opts)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	// Should return empty list, not nil
	if posts == nil {
		t.Error("expected non-nil posts slice")
	}

	if total < 0 {
		t.Errorf("expected total >= 0, got %d", total)
	}
}

func TestPostRepository_List_WithPosts(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewPostRepository(pool)
	ctx := context.Background()

	// Create test posts directly in the database
	timestamp := time.Now().Format("20060102150405")
	postIDs := []string{
		"list_test_1_" + timestamp,
		"list_test_2_" + timestamp,
		"list_test_3_" + timestamp,
	}

	// Insert test posts
	for i, postID := range postIDs {
		_, err := pool.Exec(ctx, `
			INSERT INTO posts (id, type, title, description, tags, posted_by_type, posted_by_id, status)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		`,
			postID,
			models.PostTypeProblem,
			"Test Post "+string(rune('A'+i)),
			"Description "+string(rune('A'+i)),
			[]string{"go", "testing"},
			models.AuthorTypeAgent,
			"test_agent_"+timestamp,
			models.PostStatusOpen,
		)
		if err != nil {
			t.Fatalf("failed to insert test post: %v", err)
		}
	}

	// Clean up after test
	defer func() {
		for _, postID := range postIDs {
			_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE id = $1", postID)
		}
	}()

	// List posts
	opts := models.PostListOptions{
		Page:    1,
		PerPage: 10,
	}

	posts, total, err := repo.List(ctx, opts)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if len(posts) < 3 {
		t.Errorf("expected at least 3 posts, got %d", len(posts))
	}

	if total < 3 {
		t.Errorf("expected total >= 3, got %d", total)
	}

	// Check that posts have author information
	for _, post := range posts {
		if post.Author.Type == "" {
			t.Error("expected author type to be set")
		}
		if post.Author.ID == "" {
			t.Error("expected author ID to be set")
		}
	}
}

func TestPostRepository_List_FilterByType(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewPostRepository(pool)
	ctx := context.Background()

	timestamp := time.Now().Format("20060102150405")

	// Insert posts of different types
	problemID := "type_problem_" + timestamp
	questionID := "type_question_" + timestamp
	ideaID := "type_idea_" + timestamp

	_, err := pool.Exec(ctx, `
		INSERT INTO posts (id, type, title, description, posted_by_type, posted_by_id, status)
		VALUES ($1, 'problem', 'Problem Title', 'Description', 'agent', 'test_agent', 'open')
	`, problemID)
	if err != nil {
		t.Fatalf("failed to insert problem: %v", err)
	}

	_, err = pool.Exec(ctx, `
		INSERT INTO posts (id, type, title, description, posted_by_type, posted_by_id, status)
		VALUES ($1, 'question', 'Question Title', 'Description', 'agent', 'test_agent', 'open')
	`, questionID)
	if err != nil {
		t.Fatalf("failed to insert question: %v", err)
	}

	_, err = pool.Exec(ctx, `
		INSERT INTO posts (id, type, title, description, posted_by_type, posted_by_id, status)
		VALUES ($1, 'idea', 'Idea Title', 'Description', 'agent', 'test_agent', 'open')
	`, ideaID)
	if err != nil {
		t.Fatalf("failed to insert idea: %v", err)
	}

	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE id IN ($1, $2, $3)", problemID, questionID, ideaID)
	}()

	// Filter by type: problem
	opts := models.PostListOptions{
		Type:    models.PostTypeProblem,
		Page:    1,
		PerPage: 100,
	}

	posts, _, err := repo.List(ctx, opts)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	// All returned posts should be problems
	for _, post := range posts {
		if post.Type != models.PostTypeProblem {
			t.Errorf("expected type problem, got %s", post.Type)
		}
	}
}

func TestPostRepository_List_FilterByStatus(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewPostRepository(pool)
	ctx := context.Background()

	timestamp := time.Now().Format("20060102150405")

	// Insert posts with different statuses
	openID := "status_open_" + timestamp
	solvedID := "status_solved_" + timestamp

	_, err := pool.Exec(ctx, `
		INSERT INTO posts (id, type, title, description, posted_by_type, posted_by_id, status)
		VALUES ($1, 'problem', 'Open Problem', 'Description', 'agent', 'test_agent', 'open')
	`, openID)
	if err != nil {
		t.Fatalf("failed to insert open post: %v", err)
	}

	_, err = pool.Exec(ctx, `
		INSERT INTO posts (id, type, title, description, posted_by_type, posted_by_id, status)
		VALUES ($1, 'problem', 'Solved Problem', 'Description', 'agent', 'test_agent', 'solved')
	`, solvedID)
	if err != nil {
		t.Fatalf("failed to insert solved post: %v", err)
	}

	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE id IN ($1, $2)", openID, solvedID)
	}()

	// Filter by status: open
	opts := models.PostListOptions{
		Status:  models.PostStatusOpen,
		Page:    1,
		PerPage: 100,
	}

	posts, _, err := repo.List(ctx, opts)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	// All returned posts should have status open
	for _, post := range posts {
		if post.Status != models.PostStatusOpen {
			t.Errorf("expected status open, got %s", post.Status)
		}
	}
}

func TestPostRepository_List_FilterByTags(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewPostRepository(pool)
	ctx := context.Background()

	timestamp := time.Now().Format("20060102150405")

	// Insert posts with different tags
	goPostID := "tags_go_" + timestamp
	rustPostID := "tags_rust_" + timestamp

	_, err := pool.Exec(ctx, `
		INSERT INTO posts (id, type, title, description, tags, posted_by_type, posted_by_id, status)
		VALUES ($1, 'problem', 'Go Post', 'Description', $2, 'agent', 'test_agent', 'open')
	`, goPostID, []string{"go", "backend"})
	if err != nil {
		t.Fatalf("failed to insert go post: %v", err)
	}

	_, err = pool.Exec(ctx, `
		INSERT INTO posts (id, type, title, description, tags, posted_by_type, posted_by_id, status)
		VALUES ($1, 'problem', 'Rust Post', 'Description', $2, 'agent', 'test_agent', 'open')
	`, rustPostID, []string{"rust", "backend"})
	if err != nil {
		t.Fatalf("failed to insert rust post: %v", err)
	}

	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE id IN ($1, $2)", goPostID, rustPostID)
	}()

	// Filter by tag: go
	opts := models.PostListOptions{
		Tags:    []string{"go"},
		Page:    1,
		PerPage: 100,
	}

	posts, _, err := repo.List(ctx, opts)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	// All returned posts should contain the "go" tag
	foundGoPost := false
	for _, post := range posts {
		hasGoTag := false
		for _, tag := range post.Tags {
			if tag == "go" {
				hasGoTag = true
				break
			}
		}
		if post.ID == goPostID {
			foundGoPost = true
			if !hasGoTag {
				t.Error("go post should have go tag")
			}
		}
	}

	if !foundGoPost {
		t.Error("expected to find go post in results")
	}
}

func TestPostRepository_List_Pagination(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewPostRepository(pool)
	ctx := context.Background()

	timestamp := time.Now().Format("20060102150405")

	// Insert multiple posts
	var postIDs []string
	for i := 0; i < 5; i++ {
		postID := "page_test_" + timestamp + "_" + string(rune('a'+i))
		postIDs = append(postIDs, postID)
		_, err := pool.Exec(ctx, `
			INSERT INTO posts (id, type, title, description, posted_by_type, posted_by_id, status)
			VALUES ($1, 'problem', $2, 'Description', 'agent', 'test_agent', 'open')
		`, postID, "Post "+string(rune('A'+i)))
		if err != nil {
			t.Fatalf("failed to insert post: %v", err)
		}
	}

	defer func() {
		for _, postID := range postIDs {
			_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE id = $1", postID)
		}
	}()

	// Page 1 with perPage 2
	opts := models.PostListOptions{
		Page:    1,
		PerPage: 2,
	}

	posts, total, err := repo.List(ctx, opts)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if len(posts) > 2 {
		t.Errorf("expected at most 2 posts on page 1, got %d", len(posts))
	}

	if total < 5 {
		t.Errorf("expected total >= 5, got %d", total)
	}

	// Page 2 with perPage 2
	opts.Page = 2
	posts2, _, err := repo.List(ctx, opts)
	if err != nil {
		t.Fatalf("List() page 2 error = %v", err)
	}

	if len(posts2) > 2 {
		t.Errorf("expected at most 2 posts on page 2, got %d", len(posts2))
	}
}

func TestPostRepository_List_ExcludesDeleted(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewPostRepository(pool)
	ctx := context.Background()

	timestamp := time.Now().Format("20060102150405")

	activeID := "deleted_test_active_" + timestamp
	deletedID := "deleted_test_deleted_" + timestamp

	// Insert active post
	_, err := pool.Exec(ctx, `
		INSERT INTO posts (id, type, title, description, posted_by_type, posted_by_id, status)
		VALUES ($1, 'problem', 'Active Post', 'Description', 'agent', 'test_agent', 'open')
	`, activeID)
	if err != nil {
		t.Fatalf("failed to insert active post: %v", err)
	}

	// Insert deleted post
	_, err = pool.Exec(ctx, `
		INSERT INTO posts (id, type, title, description, posted_by_type, posted_by_id, status, deleted_at)
		VALUES ($1, 'problem', 'Deleted Post', 'Description', 'agent', 'test_agent', 'open', NOW())
	`, deletedID)
	if err != nil {
		t.Fatalf("failed to insert deleted post: %v", err)
	}

	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE id IN ($1, $2)", activeID, deletedID)
	}()

	// List should not include deleted posts
	opts := models.PostListOptions{
		Page:    1,
		PerPage: 100,
	}

	posts, _, err := repo.List(ctx, opts)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	// Verify deleted post is not in results
	for _, post := range posts {
		if post.ID == deletedID {
			t.Error("deleted post should not be in list results")
		}
	}
}

func TestPostRepository_List_IncludesVoteScore(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewPostRepository(pool)
	ctx := context.Background()

	timestamp := time.Now().Format("20060102150405")
	postID := "votescore_test_" + timestamp

	// Insert post with votes
	_, err := pool.Exec(ctx, `
		INSERT INTO posts (id, type, title, description, posted_by_type, posted_by_id, status, upvotes, downvotes)
		VALUES ($1, 'problem', 'Voted Post', 'Description', 'agent', 'test_agent', 'open', 10, 3)
	`, postID)
	if err != nil {
		t.Fatalf("failed to insert post: %v", err)
	}

	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE id = $1", postID)
	}()

	opts := models.PostListOptions{
		Page:    1,
		PerPage: 100,
	}

	posts, _, err := repo.List(ctx, opts)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	// Find our post and verify vote score
	for _, post := range posts {
		if post.ID == postID {
			expectedVoteScore := 10 - 3 // upvotes - downvotes
			if post.VoteScore != expectedVoteScore {
				t.Errorf("expected vote score %d, got %d", expectedVoteScore, post.VoteScore)
			}
			return
		}
	}

	t.Error("test post not found in results")
}

func TestPostRepository_FindByID_Success(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewPostRepository(pool)
	ctx := context.Background()

	timestamp := time.Now().Format("20060102150405")
	postID := "findbyid_test_" + timestamp

	// Insert a test post with all fields populated
	_, err := pool.Exec(ctx, `
		INSERT INTO posts (id, type, title, description, tags, posted_by_type, posted_by_id,
			status, upvotes, downvotes, success_criteria, weight)
		VALUES ($1, 'problem', 'Find By ID Test', 'Test Description', $2, 'agent', 'test_agent_findbyid',
			'open', 5, 2, $3, 3)
	`, postID, []string{"go", "testing"}, []string{"Criterion 1", "Criterion 2"})
	if err != nil {
		t.Fatalf("failed to insert test post: %v", err)
	}

	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE id = $1", postID)
	}()

	// Call FindByID
	post, err := repo.FindByID(ctx, postID)
	if err != nil {
		t.Fatalf("FindByID() error = %v", err)
	}

	// Verify returned post
	if post == nil {
		t.Fatal("expected non-nil post")
	}

	if post.ID != postID {
		t.Errorf("expected ID %s, got %s", postID, post.ID)
	}

	if post.Type != models.PostTypeProblem {
		t.Errorf("expected type problem, got %s", post.Type)
	}

	if post.Title != "Find By ID Test" {
		t.Errorf("expected title 'Find By ID Test', got %s", post.Title)
	}

	if post.Description != "Test Description" {
		t.Errorf("expected description 'Test Description', got %s", post.Description)
	}

	if len(post.Tags) != 2 {
		t.Errorf("expected 2 tags, got %d", len(post.Tags))
	}

	if post.PostedByType != models.AuthorTypeAgent {
		t.Errorf("expected posted_by_type agent, got %s", post.PostedByType)
	}

	if post.Status != models.PostStatusOpen {
		t.Errorf("expected status open, got %s", post.Status)
	}

	if post.Upvotes != 5 {
		t.Errorf("expected 5 upvotes, got %d", post.Upvotes)
	}

	if post.Downvotes != 2 {
		t.Errorf("expected 2 downvotes, got %d", post.Downvotes)
	}

	// Verify vote score
	expectedVoteScore := 5 - 2
	if post.VoteScore != expectedVoteScore {
		t.Errorf("expected vote score %d, got %d", expectedVoteScore, post.VoteScore)
	}

	// Verify author information is populated
	if post.Author.Type != models.AuthorTypeAgent {
		t.Errorf("expected author type agent, got %s", post.Author.Type)
	}

	if post.Author.ID != "test_agent_findbyid" {
		t.Errorf("expected author ID test_agent_findbyid, got %s", post.Author.ID)
	}
}

func TestPostRepository_FindByID_NotFound(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewPostRepository(pool)
	ctx := context.Background()

	// Try to find a non-existent post
	post, err := repo.FindByID(ctx, "non_existent_post_id_12345")
	if err == nil {
		t.Fatal("expected error for non-existent post")
	}

	if err != ErrPostNotFound {
		t.Errorf("expected ErrPostNotFound, got %v", err)
	}

	if post != nil {
		t.Error("expected nil post for non-existent ID")
	}
}

func TestPostRepository_FindByID_ExcludesDeleted(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewPostRepository(pool)
	ctx := context.Background()

	timestamp := time.Now().Format("20060102150405")
	postID := "findbyid_deleted_" + timestamp

	// Insert a soft-deleted post
	_, err := pool.Exec(ctx, `
		INSERT INTO posts (id, type, title, description, posted_by_type, posted_by_id, status, deleted_at)
		VALUES ($1, 'problem', 'Deleted Post', 'Description', 'agent', 'test_agent', 'open', NOW())
	`, postID)
	if err != nil {
		t.Fatalf("failed to insert deleted post: %v", err)
	}

	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE id = $1", postID)
	}()

	// FindByID should not return deleted posts
	post, err := repo.FindByID(ctx, postID)
	if err == nil {
		t.Fatal("expected error for deleted post")
	}

	if err != ErrPostNotFound {
		t.Errorf("expected ErrPostNotFound for deleted post, got %v", err)
	}

	if post != nil {
		t.Error("expected nil post for deleted post")
	}
}

// TestPostRepository_FindByID_InvalidUUID tests that an invalid UUID format returns ErrPostNotFound.
// This is important for security (no DB error exposure) and UX (consistent 404 behavior).
// FIX-007: Previously this returned 500 due to PostgreSQL UUID syntax error.
func TestPostRepository_FindByID_InvalidUUID(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewPostRepository(pool)
	ctx := context.Background()

	// Test various invalid UUID formats
	invalidIDs := []string{
		"test-123",           // Not a UUID
		"invalid",            // Plain string
		"123",                // Just numbers
		"not-a-valid-uuid",   // Looks like UUID but isn't
		"",                   // Empty string
		"abc",                // Short string
	}

	for _, invalidID := range invalidIDs {
		post, err := repo.FindByID(ctx, invalidID)
		if err == nil {
			t.Errorf("expected error for invalid UUID %q", invalidID)
			continue
		}

		if err != ErrPostNotFound {
			t.Errorf("for invalid UUID %q: expected ErrPostNotFound, got %v", invalidID, err)
		}

		if post != nil {
			t.Errorf("expected nil post for invalid UUID %q", invalidID)
		}
	}
}

// TestPostRepository_FindByID_ValidUUIDNotFound tests that a valid UUID format that doesn't exist returns ErrPostNotFound.
func TestPostRepository_FindByID_ValidUUIDNotFound(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewPostRepository(pool)
	ctx := context.Background()

	// Valid UUID format but doesn't exist in DB
	validUUID := "00000000-0000-0000-0000-000000000000"

	post, err := repo.FindByID(ctx, validUUID)
	if err == nil {
		t.Fatal("expected error for non-existent UUID")
	}

	if err != ErrPostNotFound {
		t.Errorf("expected ErrPostNotFound, got %v", err)
	}

	if post != nil {
		t.Error("expected nil post for non-existent UUID")
	}
}

// TestPostRepository_FindByID_IncludesCommentCount verifies that FindByID() returns comment count.
// This test ensures that individual post detail pages display accurate comment counts.
func TestPostRepository_FindByID_IncludesCommentCount(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewPostRepository(pool)
	commentsRepo := NewCommentsRepository(pool)
	ctx := context.Background()

	// Create a test post
	post, err := repo.Create(ctx, &models.Post{
		Type:         models.PostTypeProblem,
		Title:        "Test Problem for Comments in FindByID",
		Description:  "Testing comment count in FindByID",
		PostedByType: models.AuthorTypeAgent,
		PostedByID:   "test_agent",
		Status:       models.PostStatusOpen,
	})
	if err != nil {
		t.Fatalf("failed to create test post: %v", err)
	}

	// Add 2 comments to the post
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

	// Act: FindByID
	found, err := repo.FindByID(ctx, post.ID)
	if err != nil {
		t.Fatalf("FindByID() error = %v", err)
	}

	// Assert: Verify comment count
	if found.CommentsCount != 2 {
		t.Errorf("expected CommentsCount = 2, got %d", found.CommentsCount)
	}
}

// === Create Tests ===

func TestPostRepository_Create_Problem(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewPostRepository(pool)
	ctx := context.Background()

	weight := 3
	post := &models.Post{
		Type:            models.PostTypeProblem,
		Title:           "Test Problem Creation",
		Description:     "This is a test problem for Create method",
		Tags:            []string{"go", "testing"},
		PostedByType:    models.AuthorTypeAgent,
		PostedByID:      "test_agent_create",
		Status:          models.PostStatusOpen,
		SuccessCriteria: []string{"Criterion 1", "Criterion 2"},
		Weight:          &weight,
	}

	createdPost, err := repo.Create(ctx, post)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE id = $1", createdPost.ID)
	}()

	// Verify ID was generated
	if createdPost.ID == "" {
		t.Error("expected ID to be generated")
	}

	// Verify fields are set correctly
	if createdPost.Type != models.PostTypeProblem {
		t.Errorf("expected type problem, got %s", createdPost.Type)
	}

	if createdPost.Title != "Test Problem Creation" {
		t.Errorf("expected title 'Test Problem Creation', got %s", createdPost.Title)
	}

	if createdPost.Description != "This is a test problem for Create method" {
		t.Errorf("expected description mismatch")
	}

	if len(createdPost.Tags) != 2 {
		t.Errorf("expected 2 tags, got %d", len(createdPost.Tags))
	}

	if createdPost.PostedByType != models.AuthorTypeAgent {
		t.Errorf("expected posted_by_type agent, got %s", createdPost.PostedByType)
	}

	if createdPost.PostedByID != "test_agent_create" {
		t.Errorf("expected posted_by_id test_agent_create, got %s", createdPost.PostedByID)
	}

	if createdPost.Status != models.PostStatusOpen {
		t.Errorf("expected status open, got %s", createdPost.Status)
	}

	if len(createdPost.SuccessCriteria) != 2 {
		t.Errorf("expected 2 success criteria, got %d", len(createdPost.SuccessCriteria))
	}

	if createdPost.Weight == nil || *createdPost.Weight != 3 {
		t.Error("expected weight 3")
	}

	// Verify timestamps are set
	if createdPost.CreatedAt.IsZero() {
		t.Error("expected created_at to be set")
	}

	if createdPost.UpdatedAt.IsZero() {
		t.Error("expected updated_at to be set")
	}

	// Verify votes are initialized to 0
	if createdPost.Upvotes != 0 {
		t.Errorf("expected 0 upvotes, got %d", createdPost.Upvotes)
	}

	if createdPost.Downvotes != 0 {
		t.Errorf("expected 0 downvotes, got %d", createdPost.Downvotes)
	}

	// Verify post can be retrieved
	retrievedPost, err := repo.FindByID(ctx, createdPost.ID)
	if err != nil {
		t.Fatalf("FindByID() error = %v", err)
	}

	if retrievedPost.ID != createdPost.ID {
		t.Errorf("retrieved post ID mismatch")
	}
}

func TestPostRepository_Create_Question(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewPostRepository(pool)
	ctx := context.Background()

	post := &models.Post{
		Type:         models.PostTypeQuestion,
		Title:        "Test Question Creation",
		Description:  "This is a test question for Create method",
		Tags:         []string{"go"},
		PostedByType: models.AuthorTypeHuman,
		PostedByID:   "test_user_create",
		Status:       models.PostStatusOpen,
	}

	createdPost, err := repo.Create(ctx, post)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE id = $1", createdPost.ID)
	}()

	if createdPost.Type != models.PostTypeQuestion {
		t.Errorf("expected type question, got %s", createdPost.Type)
	}

	// Questions should not have success_criteria or weight
	if len(createdPost.SuccessCriteria) > 0 {
		t.Error("question should not have success_criteria")
	}

	if createdPost.Weight != nil {
		t.Error("question should not have weight")
	}
}

func TestPostRepository_Create_Idea(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewPostRepository(pool)
	ctx := context.Background()

	post := &models.Post{
		Type:         models.PostTypeIdea,
		Title:        "Test Idea Creation",
		Description:  "This is a test idea for Create method",
		Tags:         []string{"brainstorm"},
		PostedByType: models.AuthorTypeAgent,
		PostedByID:   "test_agent_idea",
		Status:       models.PostStatusOpen,
	}

	createdPost, err := repo.Create(ctx, post)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE id = $1", createdPost.ID)
	}()

	if createdPost.Type != models.PostTypeIdea {
		t.Errorf("expected type idea, got %s", createdPost.Type)
	}
}

func TestPostRepository_Create_WithNilTags(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewPostRepository(pool)
	ctx := context.Background()

	post := &models.Post{
		Type:         models.PostTypeProblem,
		Title:        "Test No Tags",
		Description:  "Post without tags",
		Tags:         nil, // nil tags
		PostedByType: models.AuthorTypeAgent,
		PostedByID:   "test_agent_notags",
		Status:       models.PostStatusOpen,
	}

	createdPost, err := repo.Create(ctx, post)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE id = $1", createdPost.ID)
	}()

	// Should succeed with nil/empty tags
	if createdPost.ID == "" {
		t.Error("expected ID to be generated")
	}
}

func TestPostRepository_Create_DefaultStatus(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewPostRepository(pool)
	ctx := context.Background()

	post := &models.Post{
		Type:         models.PostTypeProblem,
		Title:        "Test Default Status",
		Description:  "Post with default status",
		PostedByType: models.AuthorTypeAgent,
		PostedByID:   "test_agent_defaultstatus",
		// Status not set - should default to draft
	}

	createdPost, err := repo.Create(ctx, post)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE id = $1", createdPost.ID)
	}()

	// If no status is provided, the database defaults to 'draft'
	if createdPost.Status != models.PostStatusDraft && createdPost.Status != "" {
		t.Logf("Note: status was %s", createdPost.Status)
	}
}

// === Update Tests ===

func TestPostRepository_Update_Success(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewPostRepository(pool)
	ctx := context.Background()

	// First create a post
	weight := 2
	post := &models.Post{
		Type:            models.PostTypeProblem,
		Title:           "Original Title",
		Description:     "Original description",
		Tags:            []string{"go"},
		PostedByType:    models.AuthorTypeAgent,
		PostedByID:      "test_agent_update",
		Status:          models.PostStatusOpen,
		SuccessCriteria: []string{"Original criteria"},
		Weight:          &weight,
	}

	createdPost, err := repo.Create(ctx, post)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE id = $1", createdPost.ID)
	}()

	// Update the post
	newWeight := 4
	createdPost.Title = "Updated Title"
	createdPost.Description = "Updated description"
	createdPost.Tags = []string{"go", "updated"}
	createdPost.Status = models.PostStatusInProgress
	createdPost.SuccessCriteria = []string{"Updated criteria 1", "Updated criteria 2"}
	createdPost.Weight = &newWeight

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

	if updatedPost.Status != models.PostStatusInProgress {
		t.Errorf("expected status in_progress, got %s", updatedPost.Status)
	}

	if len(updatedPost.SuccessCriteria) != 2 {
		t.Errorf("expected 2 success criteria, got %d", len(updatedPost.SuccessCriteria))
	}

	if updatedPost.Weight == nil || *updatedPost.Weight != 4 {
		t.Error("expected weight 4")
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
		Type:         models.PostTypeProblem,
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

	timestamp := time.Now().Format("20060102150405")
	postID := "update_deleted_" + timestamp

	// Insert a soft-deleted post
	_, err := pool.Exec(ctx, `
		INSERT INTO posts (id, type, title, description, posted_by_type, posted_by_id, status, deleted_at)
		VALUES ($1, 'problem', 'Deleted Post', 'Description', 'agent', 'test_agent', 'open', NOW())
	`, postID)
	if err != nil {
		t.Fatalf("failed to insert deleted post: %v", err)
	}

	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE id = $1", postID)
	}()

	// Try to update the deleted post
	post := &models.Post{
		ID:           postID,
		Type:         models.PostTypeProblem,
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

	// Create a post
	post := &models.Post{
		Type:         models.PostTypeProblem,
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
	if updatedPost.Type != models.PostTypeProblem {
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

	// Create a post
	post := &models.Post{
		Type:         models.PostTypeProblem,
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

	timestamp := time.Now().Format("20060102150405")
	postID := "delete_already_deleted_" + timestamp

	// Insert a soft-deleted post
	_, err := pool.Exec(ctx, `
		INSERT INTO posts (id, type, title, description, posted_by_type, posted_by_id, status, deleted_at)
		VALUES ($1, 'problem', 'Already Deleted', 'Description', 'agent', 'test_agent', 'open', NOW())
	`, postID)
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

	// Create a post
	post := &models.Post{
		Type:         models.PostTypeProblem,
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

// === Vote Tests ===

func TestPostRepository_Vote_Upvote(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewPostRepository(pool)
	ctx := context.Background()

	// Create a post to vote on
	post := &models.Post{
		Type:         models.PostTypeProblem,
		Title:        "Post for Upvote Test",
		Description:  "Testing upvote functionality",
		PostedByType: models.AuthorTypeAgent,
		PostedByID:   "test_agent_vote",
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

	// Vote upvote
	err = repo.Vote(ctx, createdPost.ID, "agent", "voter_agent_1", "up")
	if err != nil {
		t.Fatalf("Vote() error = %v", err)
	}

	// Verify post upvotes increased
	updatedPost, err := repo.FindByID(ctx, createdPost.ID)
	if err != nil {
		t.Fatalf("FindByID() error = %v", err)
	}

	if updatedPost.Upvotes != 1 {
		t.Errorf("expected 1 upvote, got %d", updatedPost.Upvotes)
	}

	if updatedPost.Downvotes != 0 {
		t.Errorf("expected 0 downvotes, got %d", updatedPost.Downvotes)
	}

	if updatedPost.VoteScore != 1 {
		t.Errorf("expected vote score 1, got %d", updatedPost.VoteScore)
	}
}

func TestPostRepository_Vote_Downvote(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewPostRepository(pool)
	ctx := context.Background()

	// Create a post to vote on
	post := &models.Post{
		Type:         models.PostTypeProblem,
		Title:        "Post for Downvote Test",
		Description:  "Testing downvote functionality",
		PostedByType: models.AuthorTypeAgent,
		PostedByID:   "test_agent_vote",
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

	// Vote downvote
	err = repo.Vote(ctx, createdPost.ID, "human", "voter_human_1", "down")
	if err != nil {
		t.Fatalf("Vote() error = %v", err)
	}

	// Verify post downvotes increased
	updatedPost, err := repo.FindByID(ctx, createdPost.ID)
	if err != nil {
		t.Fatalf("FindByID() error = %v", err)
	}

	if updatedPost.Upvotes != 0 {
		t.Errorf("expected 0 upvotes, got %d", updatedPost.Upvotes)
	}

	if updatedPost.Downvotes != 1 {
		t.Errorf("expected 1 downvote, got %d", updatedPost.Downvotes)
	}

	if updatedPost.VoteScore != -1 {
		t.Errorf("expected vote score -1, got %d", updatedPost.VoteScore)
	}
}

func TestPostRepository_Vote_ChangeVote(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewPostRepository(pool)
	ctx := context.Background()

	// Create a post to vote on
	post := &models.Post{
		Type:         models.PostTypeProblem,
		Title:        "Post for Change Vote Test",
		Description:  "Testing vote change functionality",
		PostedByType: models.AuthorTypeAgent,
		PostedByID:   "test_agent_vote",
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

	// First upvote
	err = repo.Vote(ctx, createdPost.ID, "agent", "voter_agent_change", "up")
	if err != nil {
		t.Fatalf("Vote() upvote error = %v", err)
	}

	// Verify upvote
	post1, _ := repo.FindByID(ctx, createdPost.ID)
	if post1.Upvotes != 1 || post1.Downvotes != 0 {
		t.Errorf("after upvote: expected upvotes=1, downvotes=0, got upvotes=%d, downvotes=%d", post1.Upvotes, post1.Downvotes)
	}

	// Change to downvote
	err = repo.Vote(ctx, createdPost.ID, "agent", "voter_agent_change", "down")
	if err != nil {
		t.Fatalf("Vote() downvote error = %v", err)
	}

	// Verify vote changed: upvotes should decrease, downvotes should increase
	post2, _ := repo.FindByID(ctx, createdPost.ID)
	if post2.Upvotes != 0 || post2.Downvotes != 1 {
		t.Errorf("after change to downvote: expected upvotes=0, downvotes=1, got upvotes=%d, downvotes=%d", post2.Upvotes, post2.Downvotes)
	}

	// Change back to upvote
	err = repo.Vote(ctx, createdPost.ID, "agent", "voter_agent_change", "up")
	if err != nil {
		t.Fatalf("Vote() change back error = %v", err)
	}

	// Verify vote changed back
	post3, _ := repo.FindByID(ctx, createdPost.ID)
	if post3.Upvotes != 1 || post3.Downvotes != 0 {
		t.Errorf("after change back to upvote: expected upvotes=1, downvotes=0, got upvotes=%d, downvotes=%d", post3.Upvotes, post3.Downvotes)
	}
}

func TestPostRepository_Vote_MultipleVoters(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewPostRepository(pool)
	ctx := context.Background()

	// Create a post to vote on
	post := &models.Post{
		Type:         models.PostTypeProblem,
		Title:        "Post for Multiple Voters Test",
		Description:  "Testing multiple voters",
		PostedByType: models.AuthorTypeAgent,
		PostedByID:   "test_agent_vote",
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

	// Multiple voters upvote
	err = repo.Vote(ctx, createdPost.ID, "agent", "voter1", "up")
	if err != nil {
		t.Fatalf("Vote() voter1 error = %v", err)
	}

	err = repo.Vote(ctx, createdPost.ID, "human", "voter2", "up")
	if err != nil {
		t.Fatalf("Vote() voter2 error = %v", err)
	}

	err = repo.Vote(ctx, createdPost.ID, "agent", "voter3", "down")
	if err != nil {
		t.Fatalf("Vote() voter3 error = %v", err)
	}

	// Verify counts
	updatedPost, err := repo.FindByID(ctx, createdPost.ID)
	if err != nil {
		t.Fatalf("FindByID() error = %v", err)
	}

	if updatedPost.Upvotes != 2 {
		t.Errorf("expected 2 upvotes, got %d", updatedPost.Upvotes)
	}

	if updatedPost.Downvotes != 1 {
		t.Errorf("expected 1 downvote, got %d", updatedPost.Downvotes)
	}

	if updatedPost.VoteScore != 1 {
		t.Errorf("expected vote score 1, got %d", updatedPost.VoteScore)
	}
}

func TestPostRepository_Vote_SameVoteTwice(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewPostRepository(pool)
	ctx := context.Background()

	// Create a post to vote on
	post := &models.Post{
		Type:         models.PostTypeProblem,
		Title:        "Post for Same Vote Test",
		Description:  "Testing same vote twice",
		PostedByType: models.AuthorTypeAgent,
		PostedByID:   "test_agent_vote",
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

	// Upvote
	err = repo.Vote(ctx, createdPost.ID, "agent", "same_voter", "up")
	if err != nil {
		t.Fatalf("Vote() first error = %v", err)
	}

	// Upvote again (same direction) - should not change counts
	err = repo.Vote(ctx, createdPost.ID, "agent", "same_voter", "up")
	if err != nil {
		t.Fatalf("Vote() second error = %v", err)
	}

	// Verify counts unchanged (still 1 upvote)
	updatedPost, err := repo.FindByID(ctx, createdPost.ID)
	if err != nil {
		t.Fatalf("FindByID() error = %v", err)
	}

	if updatedPost.Upvotes != 1 {
		t.Errorf("expected 1 upvote (unchanged), got %d", updatedPost.Upvotes)
	}
}

func TestPostRepository_Vote_PostNotFound(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewPostRepository(pool)
	ctx := context.Background()

	// Try to vote on non-existent post
	err := repo.Vote(ctx, "non_existent_post_id", "agent", "voter", "up")
	if err == nil {
		t.Fatal("expected error for non-existent post")
	}

	if err != ErrPostNotFound {
		t.Errorf("expected ErrPostNotFound, got %v", err)
	}
}

func TestPostRepository_Vote_InvalidDirection(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewPostRepository(pool)
	ctx := context.Background()

	// Create a post
	post := &models.Post{
		Type:         models.PostTypeProblem,
		Title:        "Post for Invalid Direction Test",
		Description:  "Testing invalid vote direction",
		PostedByType: models.AuthorTypeAgent,
		PostedByID:   "test_agent_vote",
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

	// Try invalid direction
	err = repo.Vote(ctx, createdPost.ID, "agent", "voter", "invalid")
	if err == nil {
		t.Fatal("expected error for invalid direction")
	}

	if err != ErrInvalidVoteDirection {
		t.Errorf("expected ErrInvalidVoteDirection, got %v", err)
	}
}

func TestPostRepository_Vote_InvalidVoterType(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewPostRepository(pool)
	ctx := context.Background()

	// Create a post
	post := &models.Post{
		Type:         models.PostTypeProblem,
		Title:        "Post for Invalid Voter Type Test",
		Description:  "Testing invalid voter type",
		PostedByType: models.AuthorTypeAgent,
		PostedByID:   "test_agent_vote",
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

	// Try invalid voter type
	err = repo.Vote(ctx, createdPost.ID, "invalid", "voter", "up")
	if err == nil {
		t.Fatal("expected error for invalid voter type")
	}

	if err != ErrInvalidVoterType {
		t.Errorf("expected ErrInvalidVoterType, got %v", err)
	}
}

// TestPostRepository_Vote_SetsConfirmedTrue verifies that votes are inserted
// with confirmed = true so they count toward reputation calculations.
func TestPostRepository_Vote_SetsConfirmedTrue(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewPostRepository(pool)
	ctx := context.Background()

	// Create a post to vote on
	post := &models.Post{
		Type:         models.PostTypeProblem,
		Title:        "Post for Confirmed Vote Test",
		Description:  "Testing that votes are auto-confirmed",
		PostedByType: models.AuthorTypeAgent,
		PostedByID:   "test_agent_vote",
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

	// Cast an upvote
	err = repo.Vote(ctx, createdPost.ID, "agent", "voter_agent_confirmed", "up")
	if err != nil {
		t.Fatalf("Vote() error = %v", err)
	}

	// Query the votes table directly to verify confirmed = true
	var confirmed bool
	err = pool.QueryRow(ctx,
		"SELECT confirmed FROM votes WHERE target_type = 'post' AND target_id = $1 AND voter_id = $2",
		createdPost.ID, "voter_agent_confirmed",
	).Scan(&confirmed)
	if err != nil {
		t.Fatalf("QueryRow() error = %v", err)
	}

	if !confirmed {
		t.Error("expected vote to be confirmed = true, got false")
	}
}

// TestPostRepository_Create_ReturnsViewCount verifies that Create returns a post
// with view_count field properly scanned from the RETURNING clause.
// FIX-030: Create was missing view_count in RETURNING, causing scan mismatch error.
func TestPostRepository_Create_ReturnsViewCount(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewPostRepository(pool)
	ctx := context.Background()

	post := &models.Post{
		Type:         models.PostTypeProblem,
		Title:        "Test ViewCount in Create Response",
		Description:  "FIX-030: Create must return view_count to avoid scan error",
		Tags:         []string{"fix-030"},
		PostedByType: models.AuthorTypeAgent,
		PostedByID:   "test_agent_viewcount",
		Status:       models.PostStatusOpen,
	}

	// This should succeed without scan error
	createdPost, err := repo.Create(ctx, post)
	if err != nil {
		t.Fatalf("Create() should not fail, but got error = %v", err)
	}

	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE id = $1", createdPost.ID)
	}()

	// Verify post was created and returned successfully
	if createdPost.ID == "" {
		t.Error("expected ID to be generated")
	}

	// Verify view_count is initialized to 0 (new posts have 0 views)
	if createdPost.ViewCount != 0 {
		t.Errorf("expected view_count to be 0 for new post, got %d", createdPost.ViewCount)
	}

	// Verify all other essential fields are populated
	if createdPost.Type != models.PostTypeProblem {
		t.Errorf("expected type problem, got %s", createdPost.Type)
	}

	if createdPost.Title != "Test ViewCount in Create Response" {
		t.Errorf("expected correct title, got %s", createdPost.Title)
	}

	if createdPost.CreatedAt.IsZero() {
		t.Error("expected created_at to be set")
	}

	// Additional verification: post can be found by ID after creation
	foundPost, err := repo.FindByID(ctx, createdPost.ID)
	if err != nil {
		t.Fatalf("FindByID() after Create should work, but got error = %v", err)
	}

	if foundPost.ViewCount != 0 {
		t.Errorf("expected view_count 0 from FindByID, got %d", foundPost.ViewCount)
	}
}

// === GetUserVote Tests ===

func TestPostRepository_GetUserVote_NoVote(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewPostRepository(pool)
	ctx := context.Background()

	// Create a post
	post := &models.Post{
		Type:         models.PostTypeQuestion,
		Title:        "Post for GetUserVote Test - No Vote",
		Description:  "Testing GetUserVote when user hasn't voted",
		PostedByType: models.AuthorTypeAgent,
		PostedByID:   "test_agent_getuservote",
		Status:       models.PostStatusOpen,
	}

	createdPost, err := repo.Create(ctx, post)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE id = $1", createdPost.ID)
	}()

	// Get vote when user hasn't voted
	vote, err := repo.GetUserVote(ctx, createdPost.ID, "human", "user123")
	if err != nil {
		t.Fatalf("GetUserVote() error = %v", err)
	}

	if vote != nil {
		t.Errorf("expected nil vote, got %v", *vote)
	}
}

func TestPostRepository_GetUserVote_Upvote(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewPostRepository(pool)
	ctx := context.Background()

	// Create a post
	post := &models.Post{
		Type:         models.PostTypeQuestion,
		Title:        "Post for GetUserVote Test - Upvote",
		Description:  "Testing GetUserVote after upvoting",
		PostedByType: models.AuthorTypeAgent,
		PostedByID:   "test_agent_getuservote",
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

	// Vote upvote
	err = repo.Vote(ctx, createdPost.ID, "human", "user456", "up")
	if err != nil {
		t.Fatalf("Vote() error = %v", err)
	}

	// Get user's vote
	vote, err := repo.GetUserVote(ctx, createdPost.ID, "human", "user456")
	if err != nil {
		t.Fatalf("GetUserVote() error = %v", err)
	}

	if vote == nil {
		t.Fatal("expected vote to be 'up', got nil")
	}

	if *vote != "up" {
		t.Errorf("expected vote 'up', got %s", *vote)
	}
}

func TestPostRepository_GetUserVote_Downvote(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewPostRepository(pool)
	ctx := context.Background()

	// Create a post
	post := &models.Post{
		Type:         models.PostTypeProblem,
		Title:        "Post for GetUserVote Test - Downvote",
		Description:  "Testing GetUserVote after downvoting",
		PostedByType: models.AuthorTypeAgent,
		PostedByID:   "test_agent_getuservote",
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

	// Vote downvote
	err = repo.Vote(ctx, createdPost.ID, "agent", "agent789", "down")
	if err != nil {
		t.Fatalf("Vote() error = %v", err)
	}

	// Get user's vote
	vote, err := repo.GetUserVote(ctx, createdPost.ID, "agent", "agent789")
	if err != nil {
		t.Fatalf("GetUserVote() error = %v", err)
	}

	if vote == nil {
		t.Fatal("expected vote to be 'down', got nil")
	}

	if *vote != "down" {
		t.Errorf("expected vote 'down', got %s", *vote)
	}
}

func TestPostRepository_GetUserVote_ChangeVote(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewPostRepository(pool)
	ctx := context.Background()

	// Create a post
	post := &models.Post{
		Type:         models.PostTypeIdea,
		Title:        "Post for GetUserVote Test - Change Vote",
		Description:  "Testing GetUserVote after changing vote",
		PostedByType: models.AuthorTypeHuman,
		PostedByID:   "test_human_getuservote",
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

	// Initial upvote
	err = repo.Vote(ctx, createdPost.ID, "human", "changer_user", "up")
	if err != nil {
		t.Fatalf("Vote() upvote error = %v", err)
	}

	// Verify upvote
	vote1, _ := repo.GetUserVote(ctx, createdPost.ID, "human", "changer_user")
	if vote1 == nil || *vote1 != "up" {
		t.Errorf("expected 'up', got %v", vote1)
	}

	// Change to downvote
	err = repo.Vote(ctx, createdPost.ID, "human", "changer_user", "down")
	if err != nil {
		t.Fatalf("Vote() downvote error = %v", err)
	}

	// Verify vote changed
	vote2, _ := repo.GetUserVote(ctx, createdPost.ID, "human", "changer_user")
	if vote2 == nil || *vote2 != "down" {
		t.Errorf("expected 'down', got %v", vote2)
	}
}

func TestPostRepository_GetUserVote_DifferentUsers(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewPostRepository(pool)
	ctx := context.Background()

	// Create a post
	post := &models.Post{
		Type:         models.PostTypeQuestion,
		Title:        "Post for GetUserVote Test - Different Users",
		Description:  "Testing GetUserVote with multiple users",
		PostedByType: models.AuthorTypeAgent,
		PostedByID:   "test_agent_getuservote",
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

	// User1 upvotes
	err = repo.Vote(ctx, createdPost.ID, "human", "user_a", "up")
	if err != nil {
		t.Fatalf("Vote() user_a error = %v", err)
	}

	// User2 downvotes
	err = repo.Vote(ctx, createdPost.ID, "agent", "user_b", "down")
	if err != nil {
		t.Fatalf("Vote() user_b error = %v", err)
	}

	// User3 doesn't vote

	// Get user_a's vote
	voteA, err := repo.GetUserVote(ctx, createdPost.ID, "human", "user_a")
	if err != nil {
		t.Fatalf("GetUserVote() user_a error = %v", err)
	}
	if voteA == nil || *voteA != "up" {
		t.Errorf("user_a: expected 'up', got %v", voteA)
	}

	// Get user_b's vote
	voteB, err := repo.GetUserVote(ctx, createdPost.ID, "agent", "user_b")
	if err != nil {
		t.Fatalf("GetUserVote() user_b error = %v", err)
	}
	if voteB == nil || *voteB != "down" {
		t.Errorf("user_b: expected 'down', got %v", voteB)
	}

	// Get user_c's vote (hasn't voted)
	voteC, err := repo.GetUserVote(ctx, createdPost.ID, "human", "user_c")
	if err != nil {
		t.Fatalf("GetUserVote() user_c error = %v", err)
	}
	if voteC != nil {
		t.Errorf("user_c: expected nil, got %v", *voteC)
	}
}

func TestPostRepository_GetUserVote_PostNotFound(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewPostRepository(pool)
	ctx := context.Background()

	// Try to get vote for non-existent post
	_, err := repo.GetUserVote(ctx, "non_existent_post_id", "human", "user999")
	if err == nil {
		t.Fatal("expected error for non-existent post")
	}

	if err != ErrPostNotFound {
		t.Errorf("expected ErrPostNotFound, got %v", err)
	}
}

func TestPostRepository_GetUserVote_InvalidUUID(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewPostRepository(pool)
	ctx := context.Background()

	// Try with invalid UUID
	_, err := repo.GetUserVote(ctx, "not-a-uuid", "human", "user999")
	if err == nil {
		t.Fatal("expected error for invalid UUID")
	}

	if err != ErrPostNotFound {
		t.Errorf("expected ErrPostNotFound, got %v", err)
	}
}

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

	repo := NewPostRepository(pool)

	// Create two questions via repo.Create (generates valid UUIDs)
	q1, err := repo.Create(ctx, &models.Post{
		Type: models.PostTypeQuestion, Title: "Q1 Many Answers",
		Description: "Sort test q1", PostedByType: models.AuthorTypeAgent,
		PostedByID: "sort_test_agent", Status: models.PostStatusOpen,
		Tags: []string{"sort_answers_test"},
	})
	if err != nil {
		t.Fatalf("failed to create q1: %v", err)
	}

	q2, err := repo.Create(ctx, &models.Post{
		Type: models.PostTypeQuestion, Title: "Q2 Few Answers",
		Description: "Sort test q2", PostedByType: models.AuthorTypeAgent,
		PostedByID: "sort_test_agent", Status: models.PostStatusOpen,
		Tags: []string{"sort_answers_test"},
	})
	if err != nil {
		t.Fatalf("failed to create q2: %v", err)
	}

	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM answers WHERE question_id IN ($1, $2)", q1.ID, q2.ID)
		_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE id IN ($1, $2)", q1.ID, q2.ID)
	}()

	// Add 3 answers to q1, 1 answer to q2
	for i := 0; i < 3; i++ {
		_, err := pool.Exec(ctx, `INSERT INTO answers (question_id, author_type, author_id, content)
			VALUES ($1, 'agent', 'sort_test_agent', $2)`, q1.ID, "Answer for q1")
		if err != nil {
			t.Fatalf("failed to insert answer for q1: %v", err)
		}
	}
	_, err = pool.Exec(ctx, `INSERT INTO answers (question_id, author_type, author_id, content)
		VALUES ($1, 'agent', 'sort_test_agent', 'Answer for q2')`, q2.ID)
	if err != nil {
		t.Fatalf("failed to insert answer for q2: %v", err)
	}

	// List with sort=answers, filtered by tag to isolate our test data
	posts, _, err := repo.List(ctx, models.PostListOptions{
		Type: models.PostTypeQuestion, Sort: "answers",
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

	repo := NewPostRepository(pool)

	p1, err := repo.Create(ctx, &models.Post{
		Type: models.PostTypeProblem, Title: "P1 Many Approaches",
		Description: "Sort test p1", PostedByType: models.AuthorTypeAgent,
		PostedByID: "sort_test_agent", Status: models.PostStatusOpen,
		Tags: []string{"sort_approaches_test"},
	})
	if err != nil {
		t.Fatalf("failed to create p1: %v", err)
	}

	p2, err := repo.Create(ctx, &models.Post{
		Type: models.PostTypeProblem, Title: "P2 Few Approaches",
		Description: "Sort test p2", PostedByType: models.AuthorTypeAgent,
		PostedByID: "sort_test_agent", Status: models.PostStatusOpen,
		Tags: []string{"sort_approaches_test"},
	})
	if err != nil {
		t.Fatalf("failed to create p2: %v", err)
	}

	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM approaches WHERE problem_id IN ($1, $2)", p1.ID, p2.ID)
		_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE id IN ($1, $2)", p1.ID, p2.ID)
	}()

	// Add 3 approaches to p1, 1 approach to p2
	for i := 0; i < 3; i++ {
		_, err := pool.Exec(ctx, `INSERT INTO approaches (problem_id, author_type, author_id, angle)
			VALUES ($1, 'agent', 'sort_test_agent', $2)`, p1.ID, "Approach for p1")
		if err != nil {
			t.Fatalf("failed to insert approach for p1: %v", err)
		}
	}
	_, err = pool.Exec(ctx, `INSERT INTO approaches (problem_id, author_type, author_id, angle)
		VALUES ($1, 'agent', 'sort_test_agent', 'Approach for p2')`, p2.ID)
	if err != nil {
		t.Fatalf("failed to insert approach for p2: %v", err)
	}

	posts, _, err := repo.List(ctx, models.PostListOptions{
		Type: models.PostTypeProblem, Sort: "approaches",
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

	repo := NewPostRepository(pool)

	q, err := repo.Create(ctx, &models.Post{
		Type: models.PostTypeQuestion, Title: "Q Soft Delete Count",
		Description: "Count test", PostedByType: models.AuthorTypeAgent,
		PostedByID: "count_test_agent", Status: models.PostStatusOpen,
		Tags: []string{"count_softdelete_test"},
	})
	if err != nil {
		t.Fatalf("failed to create question: %v", err)
	}

	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM answers WHERE question_id = $1", q.ID)
		_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE id = $1", q.ID)
	}()

	// Insert 3 answers
	for i := 0; i < 3; i++ {
		_, err := pool.Exec(ctx, `INSERT INTO answers (question_id, author_type, author_id, content)
			VALUES ($1, 'agent', 'count_test_agent', $2)`, q.ID, "Answer")
		if err != nil {
			t.Fatalf("failed to insert answer: %v", err)
		}
	}

	// Soft-delete one answer
	_, err = pool.Exec(ctx, `UPDATE answers SET deleted_at = NOW()
		WHERE question_id = $1 AND ctid = (SELECT ctid FROM answers WHERE question_id = $1 AND deleted_at IS NULL LIMIT 1)`, q.ID)
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

	// Create 3 posts via repo.Create (generates valid UUIDs)
	postHigh, err := repo.Create(ctx, &models.Post{
		Type:         models.PostTypeProblem,
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
		Type:         models.PostTypeProblem,
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
		Type:         models.PostTypeProblem,
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

	// Create 3 posts
	postHigh, err := repo.Create(ctx, &models.Post{
		Type:         models.PostTypeProblem,
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
		Type:         models.PostTypeProblem,
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

	// Create 3 posts with positive, zero, and negative scores
	postPositive, err := repo.Create(ctx, &models.Post{
		Type:         models.PostTypeQuestion,
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
		Type:         models.PostTypeQuestion,
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
		Type:         models.PostTypeQuestion,
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

func TestPostRepository_List_HasAnswerFalse(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewPostRepository(pool)
	answerRepo := NewAnswersRepository(pool)
	ctx := context.Background()

	// Create 3 questions with different answer counts
	// q1: 0 answers
	q1, err := repo.Create(ctx, &models.Post{
		Type:         models.PostTypeQuestion,
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
		Type:         models.PostTypeQuestion,
		Title:        "Question with 1 answer",
		Description:  "Test question with one answer",
		PostedByType: models.AuthorTypeAgent,
		PostedByID:   "test_agent",
		Status:       models.PostStatusOpen,
	})
	if err != nil {
		t.Fatalf("failed to create q2: %v", err)
	}

	_, err = answerRepo.CreateAnswer(ctx, &models.Answer{
		QuestionID: q2.ID,
		Content:    "Answer 1",
		AuthorType: models.AuthorTypeAgent,
		AuthorID:   "test_agent",
	})
	if err != nil {
		t.Fatalf("failed to create answer for q2: %v", err)
	}

	// q3: 3 answers
	q3, err := repo.Create(ctx, &models.Post{
		Type:         models.PostTypeQuestion,
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
		_, err = answerRepo.CreateAnswer(ctx, &models.Answer{
			QuestionID: q3.ID,
			Content:    "Answer " + string(rune('0'+i)),
			AuthorType: models.AuthorTypeAgent,
			AuthorID:   "test_agent",
		})
		if err != nil {
			t.Fatalf("failed to create answer %d for q3: %v", i, err)
		}
	}

	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM answers WHERE question_id IN ($1, $2, $3)", q1.ID, q2.ID, q3.ID)
		_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE id IN ($1, $2, $3)", q1.ID, q2.ID, q3.ID)
	}()

	// Act: List with HasAnswer = false
	falseVal := false
	opts := models.PostListOptions{
		Type:      models.PostTypeQuestion,
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
	answerRepo := NewAnswersRepository(pool)
	ctx := context.Background()

	// Create 3 questions with different answer counts
	// q1: 0 answers
	q1, err := repo.Create(ctx, &models.Post{
		Type:         models.PostTypeQuestion,
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
		Type:         models.PostTypeQuestion,
		Title:        "Question with 1 answer",
		Description:  "Test question with one answer",
		PostedByType: models.AuthorTypeAgent,
		PostedByID:   "test_agent",
		Status:       models.PostStatusOpen,
	})
	if err != nil {
		t.Fatalf("failed to create q2: %v", err)
	}

	_, err = answerRepo.CreateAnswer(ctx, &models.Answer{
		QuestionID: q2.ID,
		Content:    "Answer 1",
		AuthorType: models.AuthorTypeAgent,
		AuthorID:   "test_agent",
	})
	if err != nil {
		t.Fatalf("failed to create answer for q2: %v", err)
	}

	// q3: 3 answers
	q3, err := repo.Create(ctx, &models.Post{
		Type:         models.PostTypeQuestion,
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
		_, err = answerRepo.CreateAnswer(ctx, &models.Answer{
			QuestionID: q3.ID,
			Content:    "Answer " + string(rune('0'+i)),
			AuthorType: models.AuthorTypeAgent,
			AuthorID:   "test_agent",
		})
		if err != nil {
			t.Fatalf("failed to create answer %d for q3: %v", i, err)
		}
	}

	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM answers WHERE question_id IN ($1, $2, $3)", q1.ID, q2.ID, q3.ID)
		_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE id IN ($1, $2, $3)", q1.ID, q2.ID, q3.ID)
	}()

	// Act: List with HasAnswer = true
	trueVal := true
	opts := models.PostListOptions{
		Type:      models.PostTypeQuestion,
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
	answerRepo := NewAnswersRepository(pool)
	ctx := context.Background()

	// Create 3 questions with different answer counts
	// q1: 0 answers
	q1, err := repo.Create(ctx, &models.Post{
		Type:         models.PostTypeQuestion,
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
		Type:         models.PostTypeQuestion,
		Title:        "Question with 1 answer",
		Description:  "Test question with one answer",
		PostedByType: models.AuthorTypeAgent,
		PostedByID:   "test_agent",
		Status:       models.PostStatusOpen,
	})
	if err != nil {
		t.Fatalf("failed to create q2: %v", err)
	}

	_, err = answerRepo.CreateAnswer(ctx, &models.Answer{
		QuestionID: q2.ID,
		Content:    "Answer 1",
		AuthorType: models.AuthorTypeAgent,
		AuthorID:   "test_agent",
	})
	if err != nil {
		t.Fatalf("failed to create answer for q2: %v", err)
	}

	// q3: 3 answers
	q3, err := repo.Create(ctx, &models.Post{
		Type:         models.PostTypeQuestion,
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
		_, err = answerRepo.CreateAnswer(ctx, &models.Answer{
			QuestionID: q3.ID,
			Content:    "Answer " + string(rune('0'+i)),
			AuthorType: models.AuthorTypeAgent,
			AuthorID:   "test_agent",
		})
		if err != nil {
			t.Fatalf("failed to create answer %d for q3: %v", i, err)
		}
	}

	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM answers WHERE question_id IN ($1, $2, $3)", q1.ID, q2.ID, q3.ID)
		_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE id IN ($1, $2, $3)", q1.ID, q2.ID, q3.ID)
	}()

	// Act: List without HasAnswer parameter (nil)
	opts := models.PostListOptions{
		Type:    models.PostTypeQuestion,
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
	commentsRepo := NewCommentsRepository(pool)
	ctx := context.Background()

	// Create a test post
	post, err := repo.Create(ctx, &models.Post{
		Type:         models.PostTypeQuestion,
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

	// Act: List posts
	posts, _, err := repo.List(ctx, models.PostListOptions{
		Type:    models.PostTypeQuestion,
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
	commentsRepo := NewCommentsRepository(pool)
	ctx := context.Background()

	// Create one post of each type
	question, err := repo.Create(ctx, &models.Post{
		Type:         models.PostTypeQuestion,
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
		Type:         models.PostTypeProblem,
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
		Type:         models.PostTypeIdea,
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
	_, err = commentsRepo.Create(ctx, &models.Comment{
		TargetType: "post",
		TargetID:   question.ID,
		AuthorType: models.AuthorTypeAgent,
		AuthorID:   "test_agent",
		Content:    "Comment on question",
	})
	if err != nil {
		t.Fatalf("failed to create comment on question: %v", err)
	}

	_, err = commentsRepo.Create(ctx, &models.Comment{
		TargetType: "post",
		TargetID:   problem.ID,
		AuthorType: models.AuthorTypeAgent,
		AuthorID:   "test_agent",
		Content:    "Comment on problem",
	})
	if err != nil {
		t.Fatalf("failed to create comment on problem: %v", err)
	}

	_, err = commentsRepo.Create(ctx, &models.Comment{
		TargetType: "post",
		TargetID:   idea.ID,
		AuthorType: models.AuthorTypeAgent,
		AuthorID:   "test_agent",
		Content:    "Comment on idea",
	})
	if err != nil {
		t.Fatalf("failed to create comment on idea: %v", err)
	}

	// Cleanup
	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM comments WHERE target_id IN ($1, $2, $3)", question.ID, problem.ID, idea.ID)
		_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE id IN ($1, $2, $3)", question.ID, problem.ID, idea.ID)
	}()

	// Test Questions - Should return comments_count = 1
	questions, _, err := repo.List(ctx, models.PostListOptions{
		Type:    models.PostTypeQuestion,
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
		Type:    models.PostTypeProblem,
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
		Type:    models.PostTypeIdea,
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

	// Create a post
	post := &models.Post{
		Type:         models.PostTypeProblem,
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

func TestPostRepository_FindByID_WithViewerVote(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewPostRepository(pool)
	ctx := context.Background()

	// Create a post
	post := &models.Post{
		Type:         models.PostTypeQuestion,
		Title:        "FindByID ViewerVote Test",
		Description:  "Testing user_vote in FindByID result",
		PostedByType: models.AuthorTypeAgent,
		PostedByID:   "test_agent_findbyid_vote",
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

	// Vote as user-A (downvote)
	err = repo.Vote(ctx, createdPost.ID, "human", "findbyid_voter_human", "down")
	if err != nil {
		t.Fatalf("Vote() error = %v", err)
	}

	// FindByID with viewer info — should show user_vote = "down"
	found, err := repo.FindByIDForViewer(ctx, createdPost.ID, models.AuthorTypeHuman, "findbyid_voter_human")
	if err != nil {
		t.Fatalf("FindByIDForViewer() error = %v", err)
	}
	if found.UserVote == nil {
		t.Fatal("expected UserVote to be non-nil for viewer who voted")
	}
	if *found.UserVote != "down" {
		t.Errorf("expected UserVote = 'down', got '%s'", *found.UserVote)
	}

	// FindByID without viewer info — should have nil UserVote
	foundAnon, err := repo.FindByID(ctx, createdPost.ID)
	if err != nil {
		t.Fatalf("FindByID() error = %v", err)
	}
	if foundAnon.UserVote != nil {
		t.Errorf("expected UserVote to be nil for anonymous FindByID, got '%s'", *foundAnon.UserVote)
	}

	// FindByID with non-voter — should have nil UserVote
	foundOther, err := repo.FindByIDForViewer(ctx, createdPost.ID, models.AuthorTypeAgent, "some_other_agent")
	if err != nil {
		t.Fatalf("FindByIDForViewer() other error = %v", err)
	}
	if foundOther.UserVote != nil {
		t.Errorf("expected UserVote to be nil for non-voter, got '%s'", *foundOther.UserVote)
	}
}

// TestPostRepository_FindByID_OriginalLanguage is a regression test for the bug where
// &post.OriginalLanguage was missing from findByIDInternal's inline Scan call,
// causing "number of field descriptions must equal number of destinations, got 27 and 26".
func TestPostRepository_FindByID_OriginalLanguage(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewPostRepository(pool)
	ctx := context.Background()

	timestamp := time.Now().Format("150405")
	postID := "findbyid_lang_" + timestamp

	_, err := pool.Exec(ctx, `
		INSERT INTO posts (id, type, title, description, tags, posted_by_type, posted_by_id,
			status, upvotes, downvotes, original_language)
		VALUES ($1, 'problem', 'OriginalLanguage Test', 'Test', $2, 'agent', 'test_agent_lang',
			'open', 0, 0, 'pt')
	`, postID, []string{"test"})
	if err != nil {
		t.Fatalf("failed to insert test post: %v", err)
	}
	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE id = $1", postID)
	}()

	// This call would panic with "got 27 and 26" before the fix.
	post, err := repo.FindByID(ctx, postID)
	if err != nil {
		t.Fatalf("FindByID() error = %v", err)
	}

	if post.OriginalLanguage != "pt" {
		t.Errorf("expected original_language 'pt', got %q", post.OriginalLanguage)
	}
}
