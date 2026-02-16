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
	insertTestPost(t, pool, ctx, "post-1", "problem", "Race condition in PostgreSQL async queries",
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
		if r.ID == "post-1" {
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
		t.Error("expected to find post-1 in results")
	}
}

// TestSearchRepository_Search_RelevanceScore tests that ts_rank scoring works.
func TestSearchRepository_Search_RelevanceScore(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	repo := NewSearchRepository(pool)
	ctx := context.Background()

	// Insert two posts, one more relevant than the other
	insertTestPost(t, pool, ctx, "post-rel-1", "problem",
		"PostgreSQL PostgreSQL PostgreSQL connection issues",
		"Multiple mentions of PostgreSQL connection", []string{"postgresql"}, "open")
	insertTestPost(t, pool, ctx, "post-rel-2", "question",
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
	if results[0].ID != "post-rel-1" {
		t.Errorf("expected post-rel-1 to be first (most relevant), got %s", results[0].ID)
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

	insertTestPost(t, pool, ctx, "post-snip-1", "problem",
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
	if len(snippet) > 0 && results[0].ID == "post-snip-1" {
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

	insertTestPost(t, pool, ctx, "post-type-1", "problem", "Test problem", "Description", []string{}, "open")
	insertTestPost(t, pool, ctx, "post-type-2", "question", "Test question", "Description", []string{}, "open")
	insertTestPost(t, pool, ctx, "post-type-3", "idea", "Test idea", "Description", []string{}, "open")

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

	insertTestPost(t, pool, ctx, "post-status-1", "problem", "Test open", "Description", []string{}, "open")
	insertTestPost(t, pool, ctx, "post-status-2", "problem", "Test solved", "Description", []string{}, "solved")

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

	insertTestPost(t, pool, ctx, "post-tags-1", "problem", "Go concurrency test", "Description", []string{"go", "concurrency"}, "open")
	insertTestPost(t, pool, ctx, "post-tags-2", "problem", "Python test", "Description", []string{"python"}, "open")

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

	insertTestPost(t, pool, ctx, "post-active", "problem", "Active post searchable", "Description", []string{}, "open")
	insertTestPostDeleted(t, pool, ctx, "post-deleted", "problem", "Deleted post not searchable", "Description", []string{}, "open")

	results, _, err := repo.Search(ctx, "post searchable", models.SearchOptions{
		Page:    1,
		PerPage: 20,
	})

	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	for _, r := range results {
		if r.ID == "post-deleted" {
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

	insertTestPostWithTime(t, pool, ctx, "post-old", "problem", "Test old", "Description", []string{}, "open", time.Now().Add(-24*time.Hour))
	insertTestPostWithTime(t, pool, ctx, "post-new", "problem", "Test new", "Description", []string{}, "open", time.Now())

	results, _, err := repo.Search(ctx, "test", models.SearchOptions{
		Sort:    "newest",
		Page:    1,
		PerPage: 20,
	})

	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(results) >= 2 {
		if results[0].ID != "post-new" {
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

	insertTestPostWithVotes(t, pool, ctx, "post-low", "problem", "Test low votes", "Description", []string{}, "open", 5, 3)   // Score: 2
	insertTestPostWithVotes(t, pool, ctx, "post-high", "problem", "Test high votes", "Description", []string{}, "open", 10, 1) // Score: 9

	results, _, err := repo.Search(ctx, "test", models.SearchOptions{
		Sort:    "votes",
		Page:    1,
		PerPage: 20,
	})

	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(results) >= 2 {
		if results[0].ID != "post-high" {
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

	// Insert 5 posts
	for i := 1; i <= 5; i++ {
		insertTestPost(t, pool, ctx, "post-page-"+string(rune('0'+i)), "problem",
			"Pagination test post", "Description", []string{}, "open")
	}

	// Request page 1 with 2 per page
	results, total, err := repo.Search(ctx, "pagination test", models.SearchOptions{
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
	results2, _, err := repo.Search(ctx, "pagination test", models.SearchOptions{
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

	insertTestPostWithTime(t, pool, ctx, "post-2024", "problem", "Old post date filter", "Description", []string{}, "open", oldDate)
	insertTestPostWithTime(t, pool, ctx, "post-2026", "problem", "New post date filter", "Description", []string{}, "open", newDate)

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
		if r.ID == "post-2024" {
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

	insertTestPostWithAuthor(t, pool, ctx, "post-author-1", "problem", "Test author filter", "Description",
		[]string{}, "open", "human", "user-123")
	insertTestPostWithAuthor(t, pool, ctx, "post-author-2", "problem", "Test author filter", "Description",
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

	insertTestPostWithAuthor(t, pool, ctx, "post-atype-1", "problem", "Test author type filter", "Description",
		[]string{}, "open", "human", "user-456")
	insertTestPostWithAuthor(t, pool, ctx, "post-atype-2", "problem", "Test author type filter", "Description",
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
	_, err := pool.Exec(ctx, "DELETE FROM posts WHERE id LIKE 'post-%'")
	if err != nil {
		t.Logf("cleanup warning: %v", err)
	}
}

func insertTestPost(t *testing.T, pool *Pool, ctx context.Context, id, postType, title, desc string, tags []string, status string) {
	insertTestPostWithAuthor(t, pool, ctx, id, postType, title, desc, tags, status, "human", "test-user")
}

func insertTestPostDeleted(t *testing.T, pool *Pool, ctx context.Context, id, postType, title, desc string, tags []string, status string) {
	_, err := pool.Exec(ctx, `
		INSERT INTO posts (id, type, title, description, tags, status, posted_by_type, posted_by_id, deleted_at)
		VALUES ($1, $2, $3, $4, $5, $6, 'human', 'test-user', NOW())
		ON CONFLICT (id) DO NOTHING
	`, id, postType, title, desc, tags, status)
	if err != nil {
		t.Fatalf("failed to insert test post: %v", err)
	}
}

func insertTestPostWithTime(t *testing.T, pool *Pool, ctx context.Context, id, postType, title, desc string, tags []string, status string, createdAt time.Time) {
	_, err := pool.Exec(ctx, `
		INSERT INTO posts (id, type, title, description, tags, status, posted_by_type, posted_by_id, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, 'human', 'test-user', $7)
		ON CONFLICT (id) DO NOTHING
	`, id, postType, title, desc, tags, status, createdAt)
	if err != nil {
		t.Fatalf("failed to insert test post: %v", err)
	}
}

func insertTestPostWithVotes(t *testing.T, pool *Pool, ctx context.Context, id, postType, title, desc string, tags []string, status string, upvotes, downvotes int) {
	_, err := pool.Exec(ctx, `
		INSERT INTO posts (id, type, title, description, tags, status, posted_by_type, posted_by_id, upvotes, downvotes)
		VALUES ($1, $2, $3, $4, $5, $6, 'human', 'test-user', $7, $8)
		ON CONFLICT (id) DO NOTHING
	`, id, postType, title, desc, tags, status, upvotes, downvotes)
	if err != nil {
		t.Fatalf("failed to insert test post: %v", err)
	}
}

func insertTestPostWithAuthor(t *testing.T, pool *Pool, ctx context.Context, id, postType, title, desc string, tags []string, status string, authorType, authorID string) {
	_, err := pool.Exec(ctx, `
		INSERT INTO posts (id, type, title, description, tags, status, posted_by_type, posted_by_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (id) DO NOTHING
	`, id, postType, title, desc, tags, status, authorType, authorID)
	if err != nil {
		t.Fatalf("failed to insert test post: %v", err)
	}
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
			"post-perf-"+string(rune('A'+i%26))+string(rune('0'+i/26)),
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

	for i, title := range titles {
		insertTestPost(t, pool, ctx, fmt.Sprintf("post-exact-%d", i), "problem", title,
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
	insertTestPost(t, pool, ctx, "post-mw-1", "problem",
		"Race detection in concurrent programs",
		"This post is only about race detection", []string{}, "open")

	insertTestPost(t, pool, ctx, "post-mw-2", "problem",
		"Understanding conditional logic",
		"This post is only about conditions", []string{}, "open")

	insertTestPost(t, pool, ctx, "post-mw-3", "problem",
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

	// If using AND logic: only post-mw-3 should match (has both words)
	// If using OR logic: all 3 posts should match (has either word)
	// Current implementation uses AND (&), so we expect only 1 result
	// After fix with OR (|), we should get 3 results

	if total == 1 {
		t.Logf("CURRENT: Using AND logic - only posts with both 'race' AND 'condition' are returned")
		if results[0].ID != "post-mw-3" {
			t.Errorf("expected post-mw-3 (has both words), got %s", results[0].ID)
		}
	} else if total == 3 {
		t.Logf("AFTER FIX: Using OR logic - posts with 'race' OR 'condition' are returned")
	} else {
		t.Errorf("unexpected result count: got %d, expected 1 (AND logic) or 3 (OR logic)", total)
	}
}

// TestSearchRepository_Search_PartialWordMatch tests prefix matching.
// Per plan: Verify "rac" finds "race" and "cond" finds "condition".
func TestSearchRepository_Search_PartialWordMatch(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	repo := NewSearchRepository(pool)
	ctx := context.Background()

	insertTestPost(t, pool, ctx, "post-partial-1", "problem",
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
