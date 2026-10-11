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
	authorAgent(ctx, t, pool, "test_agent_"+timestamp)

	// Insert test posts
	var postIDs []string
	for i := 0; i < 3; i++ {
		var id string
		err := pool.QueryRow(ctx, `
			INSERT INTO posts (type, title, description, tags, posted_by_type, posted_by_id, status)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
			RETURNING id::text
		`,
			models.PostTypePost,
			"Test Post "+string(rune('A'+i)),
			"Description "+string(rune('A'+i)),
			[]string{"go", "testing"},
			models.AuthorTypeAgent,
			"test_agent_"+timestamp,
			models.PostStatusOpen,
		).Scan(&id)
		if err != nil {
			t.Fatalf("failed to insert test post: %v", err)
		}
		postIDs = append(postIDs, id)
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
	authorAgent(ctx, t, pool, "test_agent")

	// Insert posts of different types
	var problemID, questionID, ideaID string

	err := pool.QueryRow(ctx, `
		INSERT INTO posts (type, title, description, posted_by_type, posted_by_id, status)
		VALUES ('post', 'Problem Title', 'Description', 'agent', 'test_agent', 'open')
		RETURNING id::text
	`).Scan(&problemID)
	if err != nil {
		t.Fatalf("failed to insert problem: %v", err)
	}

	err = pool.QueryRow(ctx, `
		INSERT INTO posts (type, title, description, posted_by_type, posted_by_id, status)
		VALUES ('post', 'Question Title', 'Description', 'agent', 'test_agent', 'open')
		RETURNING id::text
	`).Scan(&questionID)
	if err != nil {
		t.Fatalf("failed to insert question: %v", err)
	}

	err = pool.QueryRow(ctx, `
		INSERT INTO posts (type, title, description, posted_by_type, posted_by_id, status)
		VALUES ('post', 'Idea Title', 'Description', 'agent', 'test_agent', 'open')
		RETURNING id::text
	`).Scan(&ideaID)
	if err != nil {
		t.Fatalf("failed to insert idea: %v", err)
	}

	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE id IN ($1, $2, $3)", problemID, questionID, ideaID)
	}()

	// Filter by type: every post is type post (idx 68)
	opts := models.PostListOptions{
		Type:    models.PostTypePost,
		Page:    1,
		PerPage: 100,
	}

	posts, _, err := repo.List(ctx, opts)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	// All returned posts are type post
	for _, post := range posts {
		if post.Type != models.PostTypePost {
			t.Errorf("expected type post, got %s", post.Type)
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
	authorAgent(ctx, t, pool, "test_agent")

	// Insert posts with different statuses
	var openID, solvedID string

	err := pool.QueryRow(ctx, `
		INSERT INTO posts (type, title, description, posted_by_type, posted_by_id, status)
		VALUES ('post', 'Open Problem', 'Description', 'agent', 'test_agent', 'open')
		RETURNING id::text
	`).Scan(&openID)
	if err != nil {
		t.Fatalf("failed to insert open post: %v", err)
	}

	err = pool.QueryRow(ctx, `
		INSERT INTO posts (type, title, description, posted_by_type, posted_by_id, status)
		VALUES ('post', 'Closed Post', 'Description', 'agent', 'test_agent', 'closed')
		RETURNING id::text
	`).Scan(&solvedID)
	if err != nil {
		t.Fatalf("failed to insert closed post: %v", err)
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
	authorAgent(ctx, t, pool, "test_agent")

	// Insert posts with different tags
	var goPostID, rustPostID string

	err := pool.QueryRow(ctx, `
		INSERT INTO posts (type, title, description, tags, posted_by_type, posted_by_id, status)
		VALUES ('post', 'Go Post', 'Description', $1, 'agent', 'test_agent', 'open')
		RETURNING id::text
	`, []string{"go", "backend"}).Scan(&goPostID)
	if err != nil {
		t.Fatalf("failed to insert go post: %v", err)
	}

	err = pool.QueryRow(ctx, `
		INSERT INTO posts (type, title, description, tags, posted_by_type, posted_by_id, status)
		VALUES ('post', 'Rust Post', 'Description', $1, 'agent', 'test_agent', 'open')
		RETURNING id::text
	`, []string{"rust", "backend"}).Scan(&rustPostID)
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
	authorAgent(ctx, t, pool, "test_agent")

	// Insert multiple posts
	var postIDs []string
	for i := 0; i < 5; i++ {
		var id string
		err := pool.QueryRow(ctx, `
			INSERT INTO posts (type, title, description, posted_by_type, posted_by_id, status)
			VALUES ('post', $1, 'Description', 'agent', 'test_agent', 'open')
			RETURNING id::text
		`, "Post "+string(rune('A'+i))).Scan(&id)
		if err != nil {
			t.Fatalf("failed to insert post: %v", err)
		}
		postIDs = append(postIDs, id)
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
	authorAgent(ctx, t, pool, "test_agent")

	var activeID, deletedID string

	// Insert active post
	err := pool.QueryRow(ctx, `
		INSERT INTO posts (type, title, description, posted_by_type, posted_by_id, status)
		VALUES ('post', 'Active Post', 'Description', 'agent', 'test_agent', 'open')
		RETURNING id::text
	`).Scan(&activeID)
	if err != nil {
		t.Fatalf("failed to insert active post: %v", err)
	}

	// Insert deleted post
	err = pool.QueryRow(ctx, `
		INSERT INTO posts (type, title, description, posted_by_type, posted_by_id, status, deleted_at)
		VALUES ('post', 'Deleted Post', 'Description', 'agent', 'test_agent', 'open', NOW())
		RETURNING id::text
	`).Scan(&deletedID)
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
	authorAgent(ctx, t, pool, "test_agent")

	var postID string

	// Insert post with votes
	err := pool.QueryRow(ctx, `
		INSERT INTO posts (type, title, description, posted_by_type, posted_by_id, status, upvotes, downvotes)
		VALUES ('post', 'Voted Post', 'Description', 'agent', 'test_agent', 'open', 10, 3)
		RETURNING id::text
	`).Scan(&postID)
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
