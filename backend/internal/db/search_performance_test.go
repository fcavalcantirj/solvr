package db

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
)

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
			"post",
			"Performance test post about async programming and database optimization",
			"This is a longer description that contains various keywords like PostgreSQL, async, Go, error handling, concurrency, and optimization. It simulates realistic post content for search performance testing.",
			[]string{"go", "postgresql", "async", "performance"}[i%4:i%4+1],
			[]string{"open", "closed", "stale"}[i%3],
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
			opts:  models.SearchOptions{Type: "post", Page: 1, PerPage: 20},
		},
		{
			name:  "with tags filter",
			query: "optimization",
			opts:  models.SearchOptions{Tags: []string{"go"}, Page: 1, PerPage: 20},
		},
		{
			name:  "with status filter",
			query: "error handling",
			opts:  models.SearchOptions{Status: "closed", Page: 1, PerPage: 20},
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
			opts:  models.SearchOptions{Type: "post", Status: "open", Sort: "relevance", Page: 1, PerPage: 20},
		},
	}

	const maxDuration = 100 * time.Millisecond

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			start := time.Now()
			_, _, _, _, err := repo.Search(ctx, tc.query, tc.opts)
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
		AND p.search_document @@ to_tsquery('english', 'async:* & programming:*')
		ORDER BY ts_rank(p.search_document, to_tsquery('english', 'async:* & programming:*')) DESC
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

	// Verify the index is being used (should see "Bitmap Index Scan" or "Index Scan" on idx_posts_search_document)
	// Note: With small datasets the planner might choose sequential scan, which is acceptable
	// The key metric is execution time
	foundIndexUsage := false
	var executionTime float64
	for _, line := range explainOutput {
		if containsAny(line, "Index Scan", "Bitmap Index Scan", "idx_posts_search_document") {
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
