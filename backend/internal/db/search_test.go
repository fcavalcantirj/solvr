// Package db provides database connection pool and helper functions.
package db

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// TestSearchRepository_Search tests the basic search functionality.
func TestSearchRepository_Search(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	repo := NewSearchRepository(pool)

	// Insert test data
	ctx := context.Background()
	post1ID := insertTestPost(t, pool, ctx, "post", "Race condition in PostgreSQL async queries",
		"When running multiple async queries to PostgreSQL, I encounter race conditions.", []string{"postgresql", "async"}, "open")

	// Search for "race condition"
	results, total, _, _, err := repo.Search(ctx, "race condition", models.SearchOptions{
		Page:    1,
		PerPage: 20,
	})

	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if total == 0 {
		t.Error("expected at least 1 result")
	}

	if len(results) == 0 {
		t.Fatal("expected results, got none")
	}

	// Verify the result is what we inserted
	found := false
	for _, r := range results {
		if r.ID == post1ID {
			found = true
			if r.Title != "Race condition in PostgreSQL async queries" {
				t.Errorf("expected title 'Race condition in PostgreSQL async queries', got '%s'", r.Title)
			}
			if r.Type != "post" {
				t.Errorf("expected type 'post', got '%s'", r.Type)
			}
		}
	}

	if !found {
		t.Error("expected to find inserted post in results")
	}
}

// TestSearchRepository_Search_RelevanceScore tests that ts_rank scoring works.
func TestSearchRepository_Search_RelevanceScore(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	repo := NewSearchRepository(pool)
	ctx := context.Background()

	// Insert two posts, one more relevant than the other
	postRel1ID := insertTestPost(t, pool, ctx, "post",
		"PostgreSQL PostgreSQL PostgreSQL connection issues",
		"Multiple mentions of PostgreSQL connection", []string{"postgresql"}, "open")
	insertTestPost(t, pool, ctx, "post",
		"How to connect to database",
		"Generic database question", []string{"database"}, "open")

	// Search for "PostgreSQL"
	results, _, _, _, err := repo.Search(ctx, "PostgreSQL", models.SearchOptions{
		Sort:    "relevance",
		Page:    1,
		PerPage: 20,
	})

	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(results) < 1 {
		t.Fatal("expected at least 1 result")
	}

	// First result should be the one with more PostgreSQL mentions
	if results[0].ID != postRel1ID {
		t.Errorf("expected most-relevant post to be first, got %s", results[0].ID)
	}

	// Verify score is populated
	if results[0].Score == 0 {
		t.Error("expected non-zero relevance score")
	}
}

// TestSearchRepository_Search_Snippet tests ts_headline snippet generation.
func TestSearchRepository_Search_Snippet(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	repo := NewSearchRepository(pool)
	ctx := context.Background()

	postSnip1ID := insertTestPost(t, pool, ctx, "post",
		"Async error handling in Go",
		"When handling errors in async Go code, you need to be careful with goroutines and channels.",
		[]string{"go", "async"}, "open")

	results, _, _, _, err := repo.Search(ctx, "async error", models.SearchOptions{
		Page:    1,
		PerPage: 20,
	})

	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(results) == 0 {
		t.Fatal("expected at least 1 result")
	}

	// Snippet should contain <mark> tags per SPEC.md Part 5.5
	snippet := results[0].Snippet
	if snippet == "" {
		t.Error("expected non-empty snippet")
	}

	// The snippet should have highlights (ts_headline wraps in <b>, we convert to <mark>)
	// Note: If using StartSel/StopSel options, it uses <mark> directly
	if len(snippet) > 0 && results[0].ID == postSnip1ID {
		// Just verify snippet is populated, highlighting tested by visual inspection
		t.Logf("Generated snippet: %s", snippet)
	}
}

// TestSearchRepository_Search_TypeFilter tests filtering by post type.
func TestSearchRepository_Search_TypeFilter(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	repo := NewSearchRepository(pool)
	ctx := context.Background()

	insertTestPost(t, pool, ctx, "post", "Test problem", "Description", []string{}, "open")
	insertTestPost(t, pool, ctx, "post", "Test question", "Description", []string{}, "open")
	insertTestPost(t, pool, ctx, "post", "Test idea", "Description", []string{}, "open")

	// Every post is type post (idx 68): the type filter keeps them all.
	results, _, _, _, err := repo.Search(ctx, "test", models.SearchOptions{
		Type:    "post",
		Page:    1,
		PerPage: 20,
	})

	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	for _, r := range results {
		if r.Type != "post" {
			t.Errorf("expected only post type, got %s", r.Type)
		}
	}
}

// TestSearchRepository_Search_StatusFilter tests filtering by status.
func TestSearchRepository_Search_StatusFilter(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	repo := NewSearchRepository(pool)
	ctx := context.Background()

	insertTestPost(t, pool, ctx, "post", "Test open", "Description", []string{}, "open")
	insertTestPost(t, pool, ctx, "post", "Test closed", "Description", []string{}, "closed")

	results, _, _, _, err := repo.Search(ctx, "test", models.SearchOptions{
		Status:  "closed",
		Page:    1,
		PerPage: 20,
	})

	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	for _, r := range results {
		if r.Status != "closed" {
			t.Errorf("expected only closed status, got %s", r.Status)
		}
	}
}

// TestSearchRepository_Search_TagsFilter tests filtering by tags.
func TestSearchRepository_Search_TagsFilter(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	repo := NewSearchRepository(pool)
	ctx := context.Background()

	insertTestPost(t, pool, ctx, "post", "Go concurrency test", "Description", []string{"go", "concurrency"}, "open")
	insertTestPost(t, pool, ctx, "post", "Python test", "Description", []string{"python"}, "open")

	results, _, _, _, err := repo.Search(ctx, "test", models.SearchOptions{
		Tags:    []string{"go"},
		Page:    1,
		PerPage: 20,
	})

	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	for _, r := range results {
		if !containsTag(r.Tags, "go") {
			t.Errorf("expected posts with 'go' tag, got tags %v", r.Tags)
		}
	}
}

// TestSearchRepository_Search_ExcludeDeleted tests that deleted posts are excluded.
func TestSearchRepository_Search_ExcludeDeleted(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	repo := NewSearchRepository(pool)
	ctx := context.Background()

	insertTestPost(t, pool, ctx, "post", "Active post searchable", "Description", []string{}, "open")
	deletedID := insertTestPostDeleted(t, pool, ctx, "post", "Deleted post not searchable", "Description", []string{}, "open")

	results, _, _, _, err := repo.Search(ctx, "post searchable", models.SearchOptions{
		Page:    1,
		PerPage: 20,
	})

	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	for _, r := range results {
		if r.ID == deletedID {
			t.Error("deleted posts should not appear in search results")
		}
	}
}

// TestSearchRepository_Search_SortNewest tests sorting by created_at DESC.
func TestSearchRepository_Search_SortNewest(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	repo := NewSearchRepository(pool)
	ctx := context.Background()

	// Use a unique term to avoid interference from production data
	uniqueTerm := fmt.Sprintf("xqzsortnewest%d", time.Now().UnixNano())

	insertTestPostWithTime(t, pool, ctx, "post", uniqueTerm+" old", "Description", []string{}, "open", time.Now().Add(-24*time.Hour))
	postNewID := insertTestPostWithTime(t, pool, ctx, "post", uniqueTerm+" new", "Description", []string{}, "open", time.Now())

	results, _, _, _, err := repo.Search(ctx, uniqueTerm, models.SearchOptions{
		Sort:    "newest",
		Page:    1,
		PerPage: 20,
	})

	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(results) >= 2 {
		if results[0].ID != postNewID {
			t.Errorf("expected newest post first, got %s", results[0].ID)
		}
	}
}

// TestSearchRepository_Search_SortVotes tests sorting by vote score.
func TestSearchRepository_Search_SortVotes(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	repo := NewSearchRepository(pool)
	ctx := context.Background()

	// Use a unique term to avoid interference from production data
	uniqueTerm := fmt.Sprintf("xqzsortvotes%d", time.Now().UnixNano())

	insertTestPostWithVotes(t, pool, ctx, "post", uniqueTerm+" low votes", "Description", []string{}, "open", 5, 3)                 // Score: 2
	postHighID := insertTestPostWithVotes(t, pool, ctx, "post", uniqueTerm+" high votes", "Description", []string{}, "open", 10, 1) // Score: 9

	results, _, _, _, err := repo.Search(ctx, uniqueTerm, models.SearchOptions{
		Sort:    "votes",
		Page:    1,
		PerPage: 20,
	})

	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(results) >= 2 {
		if results[0].ID != postHighID {
			t.Errorf("expected highest voted post first, got %s", results[0].ID)
		}
	}
}

// TestSearchRepository_Search_Pagination tests pagination.
func TestSearchRepository_Search_Pagination(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	repo := NewSearchRepository(pool)
	ctx := context.Background()

	// Use a unique term unlikely to appear in other tests' data
	uniqueTerm := fmt.Sprintf("xqzpaginationtest%d", time.Now().UnixNano())

	// Insert 5 posts with the unique term
	for range 5 {
		insertTestPost(t, pool, ctx, "post",
			uniqueTerm+" post", "This post contains the unique term for pagination testing.", []string{}, "open")
	}

	// Request page 1 with 2 per page
	results, total, _, _, err := repo.Search(ctx, uniqueTerm, models.SearchOptions{
		Page:    1,
		PerPage: 2,
	})

	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("expected 2 results on page 1, got %d", len(results))
	}

	if total != 5 {
		t.Errorf("expected total 5, got %d", total)
	}

	// Request page 2
	results2, _, _, _, err := repo.Search(ctx, uniqueTerm, models.SearchOptions{
		Page:    2,
		PerPage: 2,
	})

	if err != nil {
		t.Fatalf("Search page 2 failed: %v", err)
	}

	if len(results2) != 2 {
		t.Errorf("expected 2 results on page 2, got %d", len(results2))
	}

	// Ensure no overlap
	for _, r1 := range results {
		for _, r2 := range results2 {
			if r1.ID == r2.ID {
				t.Errorf("duplicate result between pages: %s", r1.ID)
			}
		}
	}
}

// TestSearchRepository_Search_DateFilter tests from_date and to_date filters.
func TestSearchRepository_Search_DateFilter(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	repo := NewSearchRepository(pool)
	ctx := context.Background()

	oldDate := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	newDate := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)

	post2024ID := insertTestPostWithTime(t, pool, ctx, "post", "Old post date filter", "Description", []string{}, "open", oldDate)
	insertTestPostWithTime(t, pool, ctx, "post", "New post date filter", "Description", []string{}, "open", newDate)

	// Search only for posts in 2026
	results, _, _, _, err := repo.Search(ctx, "post date filter", models.SearchOptions{
		FromDate: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		ToDate:   time.Date(2026, 12, 31, 23, 59, 59, 0, time.UTC),
		Page:     1,
		PerPage:  20,
	})

	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	for _, r := range results {
		if r.ID == post2024ID {
			t.Error("post from 2024 should not appear when filtering for 2026")
		}
	}
}

// TestSearchRepository_Search_AuthorFilter tests filtering by author.
func TestSearchRepository_Search_AuthorFilter(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	repo := NewSearchRepository(pool)
	ctx := context.Background()
	authorAgent(ctx, t, pool, "claude")

	insertTestPostWithAuthor(t, pool, ctx, "post", "Test author filter", "Description",
		[]string{}, "open", "human", testUser(ctx, t, pool))
	insertTestPostWithAuthor(t, pool, ctx, "post", "Test author filter", "Description",
		[]string{}, "open", "agent", "claude")

	results, _, _, _, err := repo.Search(ctx, "test author filter", models.SearchOptions{
		Author:  "claude",
		Page:    1,
		PerPage: 20,
	})

	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	for _, r := range results {
		if r.AuthorID != "claude" {
			t.Errorf("expected only posts by claude, got author %s", r.AuthorID)
		}
	}
}

// TestSearchRepository_Search_AuthorTypeFilter tests filtering by author type.
func TestSearchRepository_Search_AuthorTypeFilter(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	repo := NewSearchRepository(pool)
	ctx := context.Background()
	authorAgent(ctx, t, pool, "bot-1")

	insertTestPostWithAuthor(t, pool, ctx, "post", "Test author type filter", "Description",
		[]string{}, "open", "human", testUser(ctx, t, pool))
	insertTestPostWithAuthor(t, pool, ctx, "post", "Test author type filter", "Description",
		[]string{}, "open", "agent", "bot-1")

	results, _, _, _, err := repo.Search(ctx, "test author type filter", models.SearchOptions{
		AuthorType: "agent",
		Page:       1,
		PerPage:    20,
	})

	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	for _, r := range results {
		if r.AuthorType != "agent" {
			t.Errorf("expected only agent posts, got type %s", r.AuthorType)
		}
	}
}

// Helper functions

func setupTestDB(t *testing.T) *Pool {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}

	ctx := context.Background()
	pool, err := NewPool(ctx, databaseURL)
	if err != nil {
		t.Fatalf("failed to connect to database: %v", err)
	}

	// Clean up test data
	cleanupTestData(t, pool, ctx)

	return pool
}

func cleanupTestData(t *testing.T, pool *Pool, ctx context.Context) {
	// Clean up posts created by test authors (posted_by_id is a text column, safe to use string matching)
	// ('test-user', 'user-123' and 'user-456' are the labels written before 000117.)
	_, err := pool.Exec(ctx, "DELETE FROM posts WHERE posted_by_id IN ('"+testUserID+"', 'test-user', 'user-123', 'user-456', 'claude', 'bot-1')")
	if err != nil {
		t.Logf("cleanup warning: %v", err)
	}
	// Clean up posts and agents created by inferred-specialties and stale-content test helpers.
	// posted_by_id and author_id are text columns — prefix match is safe.
	_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE posted_by_id LIKE 'ia_%' OR posted_by_id LIKE 'stale_agent_%'")
	_, _ = pool.Exec(ctx, "DELETE FROM agents WHERE id LIKE 'ia_%' OR id LIKE 'stale_agent_%'")
	_, _ = pool.Exec(ctx, "DELETE FROM users WHERE username LIKE 'iu%'")
}

// insertTestPost inserts a test post and returns the generated UUID.
func insertTestPost(t *testing.T, pool *Pool, ctx context.Context, postType, title, desc string, tags []string, status string) string {
	return insertTestPostWithAuthor(t, pool, ctx, postType, title, desc, tags, status, "human", testUser(ctx, t, pool))
}

// insertTestPostDeleted inserts a deleted test post and returns the generated UUID.
func insertTestPostDeleted(t *testing.T, pool *Pool, ctx context.Context, postType, title, desc string, tags []string, status string) string {
	var id string
	err := pool.QueryRow(ctx, `
		INSERT INTO posts (type, title, description, tags, status, posted_by_type, posted_by_id, deleted_at)
		VALUES ($1, $2, $3, $4, $5, 'human', $6, NOW())
		RETURNING id::text
	`, postType, title, desc, tags, status, testUser(ctx, t, pool)).Scan(&id)
	if err != nil {
		t.Fatalf("failed to insert deleted test post: %v", err)
	}
	return id
}

// insertTestPostWithTime inserts a test post with a specific created_at and returns the generated UUID.
func insertTestPostWithTime(t *testing.T, pool *Pool, ctx context.Context, postType, title, desc string, tags []string, status string, createdAt time.Time) string {
	var id string
	err := pool.QueryRow(ctx, `
		INSERT INTO posts (type, title, description, tags, status, posted_by_type, posted_by_id, created_at)
		VALUES ($1, $2, $3, $4, $5, 'human', $7, $6)
		RETURNING id::text
	`, postType, title, desc, tags, status, createdAt, testUser(ctx, t, pool)).Scan(&id)
	if err != nil {
		t.Fatalf("failed to insert test post with time: %v", err)
	}
	return id
}

// insertTestPostWithVotes inserts a test post with vote counts and returns the generated UUID.
func insertTestPostWithVotes(t *testing.T, pool *Pool, ctx context.Context, postType, title, desc string, tags []string, status string, upvotes, downvotes int) string {
	var id string
	err := pool.QueryRow(ctx, `
		INSERT INTO posts (type, title, description, tags, status, posted_by_type, posted_by_id, upvotes, downvotes)
		VALUES ($1, $2, $3, $4, $5, 'human', $8, $6, $7)
		RETURNING id::text
	`, postType, title, desc, tags, status, upvotes, downvotes, testUser(ctx, t, pool)).Scan(&id)
	if err != nil {
		t.Fatalf("failed to insert test post with votes: %v", err)
	}
	return id
}

// insertTestPostWithAuthor inserts a test post with a specific author and returns the generated UUID.
func insertTestPostWithAuthor(t *testing.T, pool *Pool, ctx context.Context, postType, title, desc string, tags []string, status string, authorType, authorID string) string {
	var id string
	err := pool.QueryRow(ctx, `
		INSERT INTO posts (type, title, description, tags, status, posted_by_type, posted_by_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id::text
	`, postType, title, desc, tags, status, authorType, authorID).Scan(&id)
	if err != nil {
		t.Fatalf("failed to insert test post with author: %v", err)
	}
	return id
}

func containsTag(tags []string, target string) bool {
	for _, tag := range tags {
		if tag == target {
			return true
		}
	}
	return false
}
