package db

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
)

func setupSearchAnalyticsTest(t *testing.T) (*Pool, *SearchAnalyticsRepository) {
	t.Helper()
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
	_, _ = pool.Exec(ctx, "DELETE FROM search_queries WHERE query LIKE 'test_%'")

	repo := NewSearchAnalyticsRepository(pool)
	return pool, repo
}

func TestSearchAnalyticsRepository_Insert(t *testing.T) {
	pool, repo := setupSearchAnalyticsTest(t)
	defer pool.Close()

	ctx := context.Background()
	sq := models.SearchQuery{
		Query:           "test_golang error handling",
		QueryNormalized: "test_golang error handling",
		ResultsCount:    15,
		SearchMethod:    "hybrid",
		DurationMs:      250,
		SearcherType:    "agent",
		Page:            1,
		IPAddress:       "192.168.1.1",
		UserAgent:       "solvr-cli/1.0",
		SearchedAt:      time.Now(),
	}
	agentID := "agent-123"
	sq.SearcherID = &agentID

	err := repo.Insert(ctx, sq)
	if err != nil {
		t.Fatalf("Insert() error = %v", err)
	}

	// Verify it was inserted
	var count int
	err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM search_queries WHERE query = 'test_golang error handling'").Scan(&count)
	if err != nil {
		t.Fatalf("count query failed: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 row, got %d", count)
	}

	// Cleanup
	_, _ = pool.Exec(ctx, "DELETE FROM search_queries WHERE query LIKE 'test_%'")
}

func TestSearchAnalyticsRepository_Insert_Anonymous(t *testing.T) {
	pool, repo := setupSearchAnalyticsTest(t)
	defer pool.Close()

	ctx := context.Background()
	sq := models.SearchQuery{
		Query:           "test_anonymous search",
		QueryNormalized: "test_anonymous search",
		ResultsCount:    0,
		SearchMethod:    "fulltext",
		DurationMs:      50,
		SearcherType:    "anonymous",
		Page:            1,
		SearchedAt:      time.Now(),
	}

	err := repo.Insert(ctx, sq)
	if err != nil {
		t.Fatalf("Insert() error = %v", err)
	}

	// Verify NULL fields
	var searcherID *string
	var ipAddr *string
	err = pool.QueryRow(ctx,
		"SELECT searcher_id, ip_address::text FROM search_queries WHERE query = 'test_anonymous search'",
	).Scan(&searcherID, &ipAddr)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if searcherID != nil {
		t.Errorf("expected NULL searcher_id, got %v", *searcherID)
	}
	if ipAddr != nil {
		t.Errorf("expected NULL ip_address, got %v", *ipAddr)
	}

	// Cleanup
	_, _ = pool.Exec(ctx, "DELETE FROM search_queries WHERE query LIKE 'test_%'")
}

func TestSearchAnalyticsRepository_GetTrending(t *testing.T) {
	pool, repo := setupSearchAnalyticsTest(t)
	defer pool.Close()

	ctx := context.Background()
	now := time.Now()

	// Insert queries with different frequencies
	queries := []struct {
		query string
		count int
	}{
		{"test_popular query", 5},
		{"test_medium query", 3},
		{"test_rare query", 1},
	}

	for _, q := range queries {
		for i := 0; i < q.count; i++ {
			err := repo.Insert(ctx, models.SearchQuery{
				Query:           q.query,
				QueryNormalized: q.query,
				ResultsCount:    10,
				SearchMethod:    "hybrid",
				DurationMs:      100,
				SearcherType:    "anonymous",
				Page:            1,
				SearchedAt:      now.Add(-time.Duration(i) * time.Minute),
			})
			if err != nil {
				t.Fatalf("Insert() error = %v", err)
			}
		}
	}

	trending, err := repo.GetTrending(ctx, 7, 10)
	if err != nil {
		t.Fatalf("GetTrending() error = %v", err)
	}

	// Find our test queries in results
	testResults := map[string]int{}
	for _, tr := range trending {
		if tr.Query == "test_popular query" || tr.Query == "test_medium query" || tr.Query == "test_rare query" {
			testResults[tr.Query] = tr.Count
		}
	}

	if len(testResults) < 3 {
		t.Fatalf("expected 3 test trending results, got %d", len(testResults))
	}

	if testResults["test_popular query"] != 5 {
		t.Errorf("expected 'test_popular query' count=5, got %d", testResults["test_popular query"])
	}

	// Verify ordering: popular > medium > rare
	var prevCount int = 999
	for _, tr := range trending {
		if tr.Query == "test_popular query" || tr.Query == "test_medium query" || tr.Query == "test_rare query" {
			if tr.Count > prevCount {
				t.Errorf("trending not sorted: %s (%d) should be after previous (%d)", tr.Query, tr.Count, prevCount)
			}
			prevCount = tr.Count
		}
	}

	// Cleanup
	_, _ = pool.Exec(ctx, "DELETE FROM search_queries WHERE query LIKE 'test_%'")
}

func TestSearchAnalyticsRepository_GetZeroResults(t *testing.T) {
	pool, repo := setupSearchAnalyticsTest(t)
	defer pool.Close()

	ctx := context.Background()
	now := time.Now()

	// Insert zero-result queries
	for i := 0; i < 3; i++ {
		err := repo.Insert(ctx, models.SearchQuery{
			Query:           "test_missing topic",
			QueryNormalized: "test_missing topic",
			ResultsCount:    0,
			SearchMethod:    "hybrid",
			DurationMs:      200,
			SearcherType:    "anonymous",
			Page:            1,
			SearchedAt:      now.Add(-time.Duration(i) * time.Minute),
		})
		if err != nil {
			t.Fatalf("Insert() error = %v", err)
		}
	}

	// Insert a query WITH results (should not appear in zero-results)
	err := repo.Insert(ctx, models.SearchQuery{
		Query:           "test_has results",
		QueryNormalized: "test_has results",
		ResultsCount:    10,
		SearchMethod:    "hybrid",
		DurationMs:      100,
		SearcherType:    "anonymous",
		Page:            1,
		SearchedAt:      now,
	})
	if err != nil {
		t.Fatalf("Insert() error = %v", err)
	}

	zeroResults, err := repo.GetZeroResults(ctx, 7, 10)
	if err != nil {
		t.Fatalf("GetZeroResults() error = %v", err)
	}

	found := false
	for _, zr := range zeroResults {
		if zr.Query == "test_missing topic" {
			found = true
			if zr.Count != 3 {
				t.Errorf("expected count=3, got %d", zr.Count)
			}
		}
		if zr.Query == "test_has results" {
			t.Error("'test_has results' should not appear in zero-result queries")
		}
	}
	if !found {
		t.Error("expected 'test_missing topic' in zero-result queries")
	}

	// Cleanup
	_, _ = pool.Exec(ctx, "DELETE FROM search_queries WHERE query LIKE 'test_%'")
}

func TestSearchAnalyticsRepository_GetSummary(t *testing.T) {
	pool, repo := setupSearchAnalyticsTest(t)
	defer pool.Close()

	ctx := context.Background()
	now := time.Now()

	// Insert mix of queries
	inserts := []models.SearchQuery{
		{Query: "test_sum q1", QueryNormalized: "test_sum q1", ResultsCount: 10, SearchMethod: "hybrid", DurationMs: 100, SearcherType: "human", Page: 1, SearchedAt: now},
		{Query: "test_sum q2", QueryNormalized: "test_sum q2", ResultsCount: 5, SearchMethod: "hybrid", DurationMs: 200, SearcherType: "agent", Page: 1, SearchedAt: now},
		{Query: "test_sum q3", QueryNormalized: "test_sum q3", ResultsCount: 0, SearchMethod: "fulltext", DurationMs: 50, SearcherType: "anonymous", Page: 1, SearchedAt: now},
		{Query: "test_sum q1", QueryNormalized: "test_sum q1", ResultsCount: 8, SearchMethod: "hybrid", DurationMs: 150, SearcherType: "human", Page: 1, SearchedAt: now},
	}

	for _, sq := range inserts {
		if err := repo.Insert(ctx, sq); err != nil {
			t.Fatalf("Insert() error = %v", err)
		}
	}

	summary, err := repo.GetSummary(ctx, 7)
	if err != nil {
		t.Fatalf("GetSummary() error = %v", err)
	}

	// We inserted 4 queries but there may be other test data; check minimums
	if summary.TotalSearches < 4 {
		t.Errorf("expected TotalSearches >= 4, got %d", summary.TotalSearches)
	}
	if summary.UniqueQueries < 3 {
		t.Errorf("expected UniqueQueries >= 3, got %d", summary.UniqueQueries)
	}
	if summary.AvgDurationMs <= 0 {
		t.Errorf("expected positive AvgDurationMs, got %.2f", summary.AvgDurationMs)
	}
	if summary.ZeroResultRate <= 0 {
		t.Errorf("expected positive ZeroResultRate, got %.4f", summary.ZeroResultRate)
	}

	// Verify searcher type breakdown
	if summary.BySearcherType["human"] < 2 {
		t.Errorf("expected at least 2 human searches, got %d", summary.BySearcherType["human"])
	}
	if summary.BySearcherType["agent"] < 1 {
		t.Errorf("expected at least 1 agent search, got %d", summary.BySearcherType["agent"])
	}

	// Cleanup
	_, _ = pool.Exec(ctx, "DELETE FROM search_queries WHERE query LIKE 'test_%'")
}

func TestSearchAnalyticsRepository_DeleteOlderThan(t *testing.T) {
	pool, repo := setupSearchAnalyticsTest(t)
	defer pool.Close()

	ctx := context.Background()
	now := time.Now()

	// Insert old and new queries
	err := repo.Insert(ctx, models.SearchQuery{
		Query: "test_old query", QueryNormalized: "test_old query",
		ResultsCount: 5, SearchMethod: "hybrid", DurationMs: 100,
		SearcherType: "anonymous", Page: 1,
		SearchedAt: now.Add(-48 * time.Hour),
	})
	if err != nil {
		t.Fatalf("Insert() error = %v", err)
	}

	err = repo.Insert(ctx, models.SearchQuery{
		Query: "test_new query", QueryNormalized: "test_new query",
		ResultsCount: 10, SearchMethod: "hybrid", DurationMs: 200,
		SearcherType: "anonymous", Page: 1,
		SearchedAt: now,
	})
	if err != nil {
		t.Fatalf("Insert() error = %v", err)
	}

	// Delete queries older than 24 hours
	cutoff := now.Add(-24 * time.Hour)
	deleted, err := repo.DeleteOlderThan(ctx, cutoff)
	if err != nil {
		t.Fatalf("DeleteOlderThan() error = %v", err)
	}
	if deleted < 1 {
		t.Errorf("expected at least 1 deleted, got %d", deleted)
	}

	// Verify old query is gone
	var count int
	err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM search_queries WHERE query = 'test_old query'").Scan(&count)
	if err != nil {
		t.Fatalf("count query failed: %v", err)
	}
	if count != 0 {
		t.Errorf("expected old query deleted, but found %d rows", count)
	}

	// Verify new query still exists
	err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM search_queries WHERE query = 'test_new query'").Scan(&count)
	if err != nil {
		t.Fatalf("count query failed: %v", err)
	}
	if count != 1 {
		t.Errorf("expected new query to exist, found %d rows", count)
	}

	// Cleanup
	_, _ = pool.Exec(ctx, "DELETE FROM search_queries WHERE query LIKE 'test_%'")
}

func TestSearchAnalyticsRepository_GetTrending_Empty(t *testing.T) {
	pool, repo := setupSearchAnalyticsTest(t)
	defer pool.Close()

	ctx := context.Background()

	// No data → should return empty slice, not nil
	trending, err := repo.GetTrending(ctx, 7, 10)
	if err != nil {
		t.Fatalf("GetTrending() error = %v", err)
	}
	if trending == nil {
		t.Error("expected empty slice, got nil")
	}
	// May contain data from other tests, just verify no error
}
