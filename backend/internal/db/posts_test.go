// Package db provides database access for Solvr.
package db

import (
	"context"
	"errors"
	"fmt"
	"testing"

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

func TestPostRepository_FindByID_Success(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewPostRepository(pool)
	ctx := context.Background()
	authorAgent(ctx, t, pool, "test_agent_findbyid")

	var postID string

	// Insert a test post with all fields populated
	err := pool.QueryRow(ctx, `
		INSERT INTO posts (type, title, description, tags, posted_by_type, posted_by_id,
			status, upvotes, downvotes)
		VALUES ('post', 'Find By ID Test', 'Test Description', $1, 'agent', 'test_agent_findbyid',
			'open', 5, 2)
		RETURNING id::text
	`, []string{"go", "testing"}).Scan(&postID)
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

	if post.Type != models.PostTypePost {
		t.Errorf("expected type post, got %s", post.Type)
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
		"test-123",         // Not a UUID
		"invalid",          // Plain string
		"123",              // Just numbers
		"not-a-valid-uuid", // Looks like UUID but isn't
		"",                 // Empty string
		"abc",              // Short string
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
	ctx := context.Background()
	authorAgent(ctx, t, pool, "test_agent")

	// Create a test post
	post, err := repo.Create(ctx, &models.Post{
		Type:         models.PostTypePost,
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
		seedMigratedReply(t, pool, ctx, post.ID, "comment", string(models.AuthorTypeAgent), "test_agent", fmt.Sprintf("Comment %d", i))
	}

	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM replies WHERE post_id = $1", post.ID)
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

func TestPostRepository_FindByID_WithViewerVote(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewPostRepository(pool)
	ctx := context.Background()
	authorAgent(ctx, t, pool, "test_agent_findbyid_vote")

	// Create a post
	post := &models.Post{
		Type:         models.PostTypePost,
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
	found, err := repo.FindByIDForViewer(ctx, createdPost.ID, models.AuthorTypeHuman, "findbyid_voter_human", "")
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
	foundOther, err := repo.FindByIDForViewer(ctx, createdPost.ID, models.AuthorTypeAgent, "some_other_agent", "")
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
	authorAgent(ctx, t, pool, "test_agent_lang")

	var postID string
	err := pool.QueryRow(ctx, `
		INSERT INTO posts (type, title, description, tags, posted_by_type, posted_by_id,
			status, upvotes, downvotes, original_language)
		VALUES ('post', 'OriginalLanguage Test', 'Test', $1, 'agent', 'test_agent_lang',
			'open', 0, 0, 'pt')
		RETURNING id::text
	`, []string{"test"}).Scan(&postID)
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
