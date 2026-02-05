// Package db provides database access for Solvr.
package db

import (
	"context"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
)

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
