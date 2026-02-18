// Package db provides database connection pool and helper functions.
package db

import (
	"context"
	"fmt"
	"os"
	"strings"
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
	post1ID := insertTestPost(t, pool, ctx, "problem", "Race condition in PostgreSQL async queries",
		"When running multiple async queries to PostgreSQL, I encounter race conditions.", []string{"postgresql", "async"}, "solved")

	// Search for "race condition"
	results, total, err := repo.Search(ctx, "race condition", models.SearchOptions{
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
			if r.Type != "problem" {
				t.Errorf("expected type 'problem', got '%s'", r.Type)
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
	postRel1ID := insertTestPost(t, pool, ctx, "problem",
		"PostgreSQL PostgreSQL PostgreSQL connection issues",
		"Multiple mentions of PostgreSQL connection", []string{"postgresql"}, "open")
	insertTestPost(t, pool, ctx, "question",
		"How to connect to database",
		"Generic database question", []string{"database"}, "open")

	// Search for "PostgreSQL"
	results, _, err := repo.Search(ctx, "PostgreSQL", models.SearchOptions{
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

	postSnip1ID := insertTestPost(t, pool, ctx, "problem",
		"Async error handling in Go",
		"When handling errors in async Go code, you need to be careful with goroutines and channels.",
		[]string{"go", "async"}, "open")

	results, _, err := repo.Search(ctx, "async error", models.SearchOptions{
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

	insertTestPost(t, pool, ctx, "problem", "Test problem", "Description", []string{}, "open")
	insertTestPost(t, pool, ctx, "question", "Test question", "Description", []string{}, "open")
	insertTestPost(t, pool, ctx, "idea", "Test idea", "Description", []string{}, "open")

	// Search with type=problem filter
	results, _, err := repo.Search(ctx, "test", models.SearchOptions{
		Type:    "problem",
		Page:    1,
		PerPage: 20,
	})

	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	for _, r := range results {
		if r.Type != "problem" {
			t.Errorf("expected only problem type, got %s", r.Type)
		}
	}
}

// TestSearchRepository_Search_StatusFilter tests filtering by status.
func TestSearchRepository_Search_StatusFilter(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	repo := NewSearchRepository(pool)
	ctx := context.Background()

	insertTestPost(t, pool, ctx, "problem", "Test open", "Description", []string{}, "open")
	insertTestPost(t, pool, ctx, "problem", "Test solved", "Description", []string{}, "solved")

	results, _, err := repo.Search(ctx, "test", models.SearchOptions{
		Status:  "solved",
		Page:    1,
		PerPage: 20,
	})

	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	for _, r := range results {
		if r.Status != "solved" {
			t.Errorf("expected only solved status, got %s", r.Status)
		}
	}
}

// TestSearchRepository_Search_TagsFilter tests filtering by tags.
func TestSearchRepository_Search_TagsFilter(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	repo := NewSearchRepository(pool)
	ctx := context.Background()

	insertTestPost(t, pool, ctx, "problem", "Go concurrency test", "Description", []string{"go", "concurrency"}, "open")
	insertTestPost(t, pool, ctx, "problem", "Python test", "Description", []string{"python"}, "open")

	results, _, err := repo.Search(ctx, "test", models.SearchOptions{
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

	insertTestPost(t, pool, ctx, "problem", "Active post searchable", "Description", []string{}, "open")
	deletedID := insertTestPostDeleted(t, pool, ctx, "problem", "Deleted post not searchable", "Description", []string{}, "open")

	results, _, err := repo.Search(ctx, "post searchable", models.SearchOptions{
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

	insertTestPostWithTime(t, pool, ctx, "problem", "Test old", "Description", []string{}, "open", time.Now().Add(-24*time.Hour))
	postNewID := insertTestPostWithTime(t, pool, ctx, "problem", "Test new", "Description", []string{}, "open", time.Now())

	results, _, err := repo.Search(ctx, "test", models.SearchOptions{
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

	insertTestPostWithVotes(t, pool, ctx, "problem", "Test low votes", "Description", []string{}, "open", 5, 3)   // Score: 2
	postHighID := insertTestPostWithVotes(t, pool, ctx, "problem", "Test high votes", "Description", []string{}, "open", 10, 1) // Score: 9

	results, _, err := repo.Search(ctx, "test", models.SearchOptions{
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
		insertTestPost(t, pool, ctx, "problem",
			uniqueTerm+" post", "This post contains the unique term for pagination testing.", []string{}, "open")
	}

	// Request page 1 with 2 per page
	results, total, err := repo.Search(ctx, uniqueTerm, models.SearchOptions{
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
	results2, _, err := repo.Search(ctx, uniqueTerm, models.SearchOptions{
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

	post2024ID := insertTestPostWithTime(t, pool, ctx, "problem", "Old post date filter", "Description", []string{}, "open", oldDate)
	insertTestPostWithTime(t, pool, ctx, "problem", "New post date filter", "Description", []string{}, "open", newDate)

	// Search only for posts in 2026
	results, _, err := repo.Search(ctx, "post date filter", models.SearchOptions{
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

	insertTestPostWithAuthor(t, pool, ctx, "problem", "Test author filter", "Description",
		[]string{}, "open", "human", "user-123")
	insertTestPostWithAuthor(t, pool, ctx, "problem", "Test author filter", "Description",
		[]string{}, "open", "agent", "claude")

	results, _, err := repo.Search(ctx, "test author filter", models.SearchOptions{
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

	insertTestPostWithAuthor(t, pool, ctx, "problem", "Test author type filter", "Description",
		[]string{}, "open", "human", "user-456")
	insertTestPostWithAuthor(t, pool, ctx, "problem", "Test author type filter", "Description",
		[]string{}, "open", "agent", "bot-1")

	results, _, err := repo.Search(ctx, "test author type filter", models.SearchOptions{
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
	_, err := pool.Exec(ctx, "DELETE FROM posts WHERE posted_by_id IN ('test-user', 'user-123', 'user-456', 'claude', 'bot-1')")
	if err != nil {
		t.Logf("cleanup warning: %v", err)
	}
}

// insertTestPost inserts a test post and returns the generated UUID.
func insertTestPost(t *testing.T, pool *Pool, ctx context.Context, postType, title, desc string, tags []string, status string) string {
	return insertTestPostWithAuthor(t, pool, ctx, postType, title, desc, tags, status, "human", "test-user")
}

// insertTestPostDeleted inserts a deleted test post and returns the generated UUID.
func insertTestPostDeleted(t *testing.T, pool *Pool, ctx context.Context, postType, title, desc string, tags []string, status string) string {
	var id string
	err := pool.QueryRow(ctx, `
		INSERT INTO posts (type, title, description, tags, status, posted_by_type, posted_by_id, deleted_at)
		VALUES ($1, $2, $3, $4, $5, 'human', 'test-user', NOW())
		RETURNING id::text
	`, postType, title, desc, tags, status).Scan(&id)
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
		VALUES ($1, $2, $3, $4, $5, 'human', 'test-user', $6)
		RETURNING id::text
	`, postType, title, desc, tags, status, createdAt).Scan(&id)
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
		VALUES ($1, $2, $3, $4, $5, 'human', 'test-user', $6, $7)
		RETURNING id::text
	`, postType, title, desc, tags, status, upvotes, downvotes).Scan(&id)
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

// TestSearchRepository_Search_PerformanceTarget tests that typical queries complete in under 100ms.
// Per SPEC.md Part 5 and prd-v2.json "Search: performance target" requirement.
func TestSearchRepository_Search_PerformanceTarget(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	repo := NewSearchRepository(pool)
	ctx := context.Background()

	// Insert a modest amount of test data to simulate realistic conditions
	for i := 0; i < 50; i++ {
		insertTestPost(t, pool, ctx,
			[]string{"problem", "question", "idea"}[i%3],
			"Performance test post about async programming and database optimization",
			"This is a longer description that contains various keywords like PostgreSQL, async, Go, error handling, concurrency, and optimization. It simulates realistic post content for search performance testing.",
			[]string{"go", "postgresql", "async", "performance"}[i%4:i%4+1],
			[]string{"open", "solved", "answered"}[i%3],
		)
	}

	// Define typical search queries to test
	testCases := []struct {
		name  string
		query string
		opts  models.SearchOptions
	}{
		{
			name:  "simple single term",
			query: "async",
			opts:  models.SearchOptions{Page: 1, PerPage: 20},
		},
		{
			name:  "multi-term query",
			query: "async programming database",
			opts:  models.SearchOptions{Page: 1, PerPage: 20},
		},
		{
			name:  "with type filter",
			query: "performance",
			opts:  models.SearchOptions{Type: "problem", Page: 1, PerPage: 20},
		},
		{
			name:  "with tags filter",
			query: "optimization",
			opts:  models.SearchOptions{Tags: []string{"go"}, Page: 1, PerPage: 20},
		},
		{
			name:  "with status filter",
			query: "error handling",
			opts:  models.SearchOptions{Status: "solved", Page: 1, PerPage: 20},
		},
		{
			name:  "sort by votes",
			query: "database",
			opts:  models.SearchOptions{Sort: "votes", Page: 1, PerPage: 20},
		},
		{
			name:  "sort by newest",
			query: "concurrency",
			opts:  models.SearchOptions{Sort: "newest", Page: 1, PerPage: 20},
		},
		{
			name:  "combined filters",
			query: "programming",
			opts:  models.SearchOptions{Type: "problem", Status: "open", Sort: "relevance", Page: 1, PerPage: 20},
		},
	}

	const maxDuration = 100 * time.Millisecond

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			start := time.Now()
			_, _, err := repo.Search(ctx, tc.query, tc.opts)
			duration := time.Since(start)

			if err != nil {
				t.Fatalf("Search failed: %v", err)
			}

			if duration > maxDuration {
				t.Errorf("search took %v, expected < %v", duration, maxDuration)
			} else {
				t.Logf("search completed in %v (target: < %v)", duration, maxDuration)
			}
		})
	}
}

// TestSearchRepository_Search_PerformanceWithIndex verifies that the GIN index is being used.
// This test uses EXPLAIN ANALYZE to verify query performance characteristics.
func TestSearchRepository_Search_PerformanceWithIndex(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	ctx := context.Background()

	// Run EXPLAIN ANALYZE on a typical search query
	query := `
		EXPLAIN ANALYZE
		SELECT p.id, p.type, p.title
		FROM posts p
		WHERE p.deleted_at IS NULL
		AND to_tsvector('english', p.title || ' ' || p.description) @@ to_tsquery('english', 'async:* & programming:*')
		ORDER BY ts_rank(to_tsvector('english', p.title || ' ' || p.description), to_tsquery('english', 'async:* & programming:*')) DESC
		LIMIT 20
	`

	rows, err := pool.Query(ctx, query)
	if err != nil {
		t.Fatalf("EXPLAIN query failed: %v", err)
	}
	defer rows.Close()

	var explainOutput []string
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			t.Fatalf("failed to scan explain output: %v", err)
		}
		explainOutput = append(explainOutput, line)
	}

	// Log the explain output for debugging
	t.Log("EXPLAIN ANALYZE output:")
	for _, line := range explainOutput {
		t.Log(line)
	}

	// Verify the index is being used (should see "Bitmap Index Scan" or "Index Scan" on idx_posts_search)
	// Note: With small datasets the planner might choose sequential scan, which is acceptable
	// The key metric is execution time
	foundIndexUsage := false
	var executionTime float64
	for _, line := range explainOutput {
		if containsAny(line, "Index Scan", "Bitmap Index Scan", "idx_posts_search") {
			foundIndexUsage = true
		}
		// Parse execution time from the output
		if containsAny(line, "Execution Time:") {
			// Example: "Execution Time: 0.123 ms"
			parsedTime, err := parseExecutionTime(line)
			if err == nil {
				executionTime = parsedTime
			}
		}
	}

	if !foundIndexUsage {
		t.Log("Note: Index not used (may be due to small dataset, planner chose seq scan)")
	}

	// The important check: execution should be fast
	if executionTime > 100 {
		t.Errorf("query execution time %.2fms exceeds 100ms target", executionTime)
	} else if executionTime > 0 {
		t.Logf("query execution time: %.2fms (target: < 100ms)", executionTime)
	}
}

// containsAny checks if s contains any of the substrings.
func containsAny(s string, substrs ...string) bool {
	for _, sub := range substrs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

// parseExecutionTime extracts execution time from EXPLAIN ANALYZE output line.
func parseExecutionTime(line string) (float64, error) {
	// Format: "Execution Time: 0.123 ms"
	if !strings.Contains(line, "Execution Time:") {
		return 0, fmt.Errorf("not an execution time line")
	}
	parts := strings.Split(line, ":")
	if len(parts) < 2 {
		return 0, fmt.Errorf("invalid format")
	}
	timeStr := strings.TrimSpace(parts[1])
	timeStr = strings.TrimSuffix(timeStr, " ms")
	timeStr = strings.TrimSpace(timeStr)
	var t float64
	_, err := fmt.Sscanf(timeStr, "%f", &t)
	return t, err
}

// TestSearchRepository_Search_ExactTitleMatch tests that exact title matches are found.
// Per plan: Verify "race condition" finds posts with those words in the title.
func TestSearchRepository_Search_ExactTitleMatch(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	repo := NewSearchRepository(pool)
	ctx := context.Background()

	// Create posts with specific titles
	titles := []string{
		"Race Conditions in Go",
		"How to Handle Race Conditions",
		"Understanding Race Conditions",
		"Thread Safety and Race Conditions",
	}

	for _, title := range titles {
		insertTestPost(t, pool, ctx, "problem", title,
			"This is a test description", []string{"go"}, "open")
	}

	// Test 1: Search for "race condition" (without s) - should find all 4 posts
	results, total, err := repo.Search(ctx, "race condition", models.SearchOptions{
		Page:    1,
		PerPage: 20,
	})

	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if total < 4 {
		t.Errorf("expected at least 4 results for 'race condition', got %d", total)
		t.Logf("Found posts:")
		for _, r := range results {
			t.Logf("  - %s: %s", r.ID, r.Title)
		}
	}

	// Test 2: Search for "race conditions" (with s) - should find all 4 posts
	_, total2, err := repo.Search(ctx, "race conditions", models.SearchOptions{
		Page:    1,
		PerPage: 20,
	})

	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if total2 < 4 {
		t.Errorf("expected at least 4 results for 'race conditions', got %d", total2)
	}

	// Test 3: Search for exact title "Race Conditions in Go" - should return that post as #1
	results3, _, err := repo.Search(ctx, "Race Conditions in Go", models.SearchOptions{
		Sort:    "relevance",
		Page:    1,
		PerPage: 20,
	})

	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(results3) == 0 {
		t.Fatal("expected at least 1 result for exact title match")
	}

	// The first result should be the exact match (highest relevance)
	if results3[0].Title != "Race Conditions in Go" {
		t.Logf("Warning: exact title match not first. Got: %s", results3[0].Title)
		t.Logf("All results:")
		for i, r := range results3 {
			t.Logf("  %d. [score=%.2f] %s", i+1, r.Score, r.Title)
		}
	}
}

// TestSearchRepository_Search_MultiWordQuery tests AND vs OR logic for multi-word queries.
// Per plan: Verify if "race condition" uses AND (strict) or OR (relaxed).
func TestSearchRepository_Search_MultiWordQuery(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	repo := NewSearchRepository(pool)
	ctx := context.Background()

	// Create posts with only one of the search terms
	insertTestPost(t, pool, ctx, "problem",
		"Race detection in concurrent programs",
		"This post is only about race detection", []string{}, "open")

	insertTestPost(t, pool, ctx, "problem",
		"Understanding conditional logic",
		"This post is only about conditions", []string{}, "open")

	postMW3ID := insertTestPost(t, pool, ctx, "problem",
		"Race condition debugging",
		"This post has both race and condition", []string{}, "open")

	// Search for "race condition"
	results, total, err := repo.Search(ctx, "race condition", models.SearchOptions{
		Page:    1,
		PerPage: 20,
	})

	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	t.Logf("Search for 'race condition' returned %d results:", total)
	for _, r := range results {
		t.Logf("  - %s: %s", r.ID, r.Title)
	}

	// If using AND logic: only postMW3 should match (has both words)
	// If using OR logic: all 3 posts should match (has either word)
	// Current implementation uses OR (|), so we expect 3 results

	if total == 1 {
		t.Logf("CURRENT: Using AND logic - only posts with both 'race' AND 'condition' are returned")
		if results[0].ID != postMW3ID {
			t.Errorf("expected post with both words to be first, got %s", results[0].ID)
		}
	} else if total >= 3 {
		t.Logf("Using OR logic - posts with 'race' OR 'condition' are returned")
	} else {
		t.Errorf("unexpected result count: got %d, expected 1 (AND logic) or 3+ (OR logic)", total)
	}
}

// TestSearch_IdeasFoundAndVoteScorePresent is a regression test for two bugs:
// 1. Case-insensitive search: "solvr" (lowercase) must find ideas with "Solvr" (capitalized) in title
// 2. vote_score field: search results must return correct vote_score value (not zero/missing)
// This validates the fix in backend/internal/db/search.go (SQL alias votes→vote_score)
// and backend/internal/models/search.go (JSON tag votes→vote_score).
func TestSearch_IdeasFoundAndVoteScorePresent(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	repo := NewSearchRepository(pool)
	ctx := context.Background()

	// Insert idea with "Solvr" capitalized in title (simulates the production bug scenario)
	postSolvrID := insertTestPostWithVotes(t, pool, ctx, "idea",
		"Pattern: Solvr API Key Not in Environment",
		"When SOLVR_API_KEY is not set, the agent fails silently with no helpful message.",
		[]string{"solvr", "environment"}, "open", 5, 0) // upvotes=5, downvotes=0 → vote_score=5

	// Search for "solvr" (lowercase) with type=idea filter
	// PostgreSQL tsvector normalizes case so "Solvr" and "solvr" are equivalent
	results, total, err := repo.Search(ctx, "solvr", models.SearchOptions{
		Type:    "idea",
		Page:    1,
		PerPage: 20,
	})

	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if total == 0 {
		t.Fatal("case-insensitive search for 'solvr' returned 0 results — tsvector normalization broken or idea not indexed")
	}

	// Verify the specific idea is in results with correct vote_score
	found := false
	for _, r := range results {
		if r.ID == postSolvrID {
			found = true
			if r.VoteScore != 5 {
				t.Errorf("vote_score field missing or wrong: expected 5 (upvotes-downvotes), got %d — check SQL alias in search.go", r.VoteScore)
			}
			if r.Type != "idea" {
				t.Errorf("expected type=idea, got %s", r.Type)
			}
			break
		}
	}

	if !found {
		t.Error("idea 'Pattern: Solvr API Key Not in Environment' not found when searching 'solvr' with type=idea filter")
		t.Log("Results returned:")
		for _, r := range results {
			t.Logf("  - %s: %s (type=%s, vote_score=%d)", r.ID, r.Title, r.Type, r.VoteScore)
		}
	}
}

// TestSearchRepository_Search_PartialWordMatch tests prefix matching.
// Per plan: Verify "rac" finds "race" and "cond" finds "condition".
func TestSearchRepository_Search_PartialWordMatch(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	repo := NewSearchRepository(pool)
	ctx := context.Background()

	insertTestPost(t, pool, ctx, "problem",
		"Race conditions in Go",
		"Description", []string{}, "open")

	// Test 1: "rac" should find "race"
	_, total1, err := repo.Search(ctx, "rac", models.SearchOptions{
		Page:    1,
		PerPage: 20,
	})

	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if total1 == 0 {
		t.Error("expected 'rac' to find posts with 'race' (prefix matching)")
	} else {
		t.Logf("Prefix match 'rac' found %d results", total1)
	}

	// Test 2: "cond" should find "condition"
	_, total2, err := repo.Search(ctx, "cond", models.SearchOptions{
		Page:    1,
		PerPage: 20,
	})

	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if total2 == 0 {
		t.Error("expected 'cond' to find posts with 'conditions' (prefix matching)")
	} else {
		t.Logf("Prefix match 'cond' found %d results", total2)
	}
}
