// Package db provides database connection pool and helper functions.
package db

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// insertTestPostWithEmbedding inserts a test post and generates an embedding for it
// using the Voyage AI API. Returns the post ID.
func insertTestPostWithEmbedding(t *testing.T, pool *Pool, ctx context.Context, apiKey, postType, title, desc string, tags []string, status string) string {
	t.Helper()

	// Generate embedding for the post content
	embedding, err := generateTestEmbedding(ctx, apiKey, title+" "+desc, "document")
	if err != nil {
		t.Fatalf("failed to generate test embedding: %v", err)
	}

	// Format embedding as PostgreSQL vector literal
	embStr := formatVectorLiteral(embedding)

	var id string
	err = pool.QueryRow(ctx, `
		INSERT INTO posts (type, title, description, tags, status, posted_by_type, posted_by_id, embedding)
		VALUES ($1, $2, $3, $4, $5, 'human', 'test-user', $6::vector)
		RETURNING id::text
	`, postType, title, desc, tags, status, embStr).Scan(&id)
	if err != nil {
		t.Fatalf("failed to insert test post with embedding: %v", err)
	}
	return id
}

// formatVectorLiteral converts a float32 slice to PostgreSQL vector literal format: [1.0,2.0,3.0]
func formatVectorLiteral(embedding []float32) string {
	result := "["
	for i, v := range embedding {
		if i > 0 {
			result += ","
		}
		result += fmt.Sprintf("%f", v)
	}
	result += "]"
	return result
}

// TestSearchHybrid_SemanticSimilarity tests that hybrid search returns semantically
// similar results even when they don't share exact keywords.
// Creates 4 posts: 3 about Go concurrency (different wording) and 1 about Python.
// Searches for "golang race condition" and expects Go-related posts to rank higher.
func TestSearchHybrid_SemanticSimilarity(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	apiKey := requireEmbeddingAPIKey(t)
	ctx := context.Background()

	// Post 1: Exact keyword match for "golang"
	post1ID := insertTestPostWithEmbedding(t, pool, ctx, apiKey, "problem",
		"Concurrency Issues in Golang",
		"When using goroutines in Golang, we encounter race conditions that cause data corruption. Multiple goroutines access shared memory without proper synchronization primitives like mutexes or channels.",
		[]string{"golang", "concurrency"}, "open")

	// Post 2: Semantic match - uses "Go" instead of "golang", discusses thread safety
	post2ID := insertTestPostWithEmbedding(t, pool, ctx, apiKey, "problem",
		"Thread Safety Problems in Go",
		"Our Go application has thread safety issues where concurrent goroutine access to shared state causes intermittent failures. We need proper synchronization to prevent data races.",
		[]string{"go", "threading"}, "open")

	// Post 3: Related concept - mutex and race conditions
	post3ID := insertTestPostWithEmbedding(t, pool, ctx, apiKey, "problem",
		"Mutex and Race Condition Handling",
		"Dealing with race conditions requires careful mutex usage. When multiple threads compete for resources, deadlocks and data races emerge. Proper lock ordering and atomic operations are essential.",
		[]string{"concurrency", "mutex"}, "open")

	// Post 4: Unrelated - Python async (should NOT appear or rank low)
	insertTestPostWithEmbedding(t, pool, ctx, apiKey, "problem",
		"Python Async Programming",
		"Using asyncio in Python for asynchronous I/O operations. The event loop handles coroutines and futures for non-blocking network calls and file operations.",
		[]string{"python", "async"}, "open")

	embedSvc := &testEmbeddingService{apiKey: apiKey}
	repo := NewSearchRepository(pool)
	repo.SetEmbeddingService(embedSvc)

	results, total, _, err := repo.Search(ctx, "golang race condition", models.SearchOptions{
		Page:    1,
		PerPage: 20,
	})

	if err != nil {
		t.Fatalf("Hybrid semantic search failed: %v", err)
	}

	if total == 0 {
		t.Fatal("expected at least 1 result from hybrid semantic search")
	}

	// Check that Posts 1, 2, 3 are in results (Go-related content)
	foundPost1 := false
	foundPost2 := false
	foundPost3 := false
	foundPython := false

	for _, r := range results {
		switch r.ID {
		case post1ID:
			foundPost1 = true
		case post2ID:
			foundPost2 = true
		case post3ID:
			foundPost3 = true
		}
		// Check if Python post sneaked in - record but don't fail
		if r.Title == "Python Async Programming" {
			foundPython = true
		}
	}

	// Post 1 has exact keyword "golang" - must appear
	if !foundPost1 {
		t.Error("expected Post 1 ('Concurrency Issues in Golang') in results - has exact keyword match")
	}

	// Post 2 is semantically related via "Go" + "thread safety" - should appear via vector similarity
	if !foundPost2 {
		t.Error("expected Post 2 ('Thread Safety Problems in Go') in results - semantically related")
	}

	// Post 3 is about race conditions + mutex - related concept
	if !foundPost3 {
		t.Error("expected Post 3 ('Mutex and Race Condition Handling') in results - related concept")
	}

	// Post 1 should rank high (exact keyword match for "golang")
	if foundPost1 && len(results) > 0 {
		// Post 1 should be in top 3 results
		post1InTopResults := false
		maxRank := 3
		if len(results) < maxRank {
			maxRank = len(results)
		}
		for i := 0; i < maxRank; i++ {
			if results[i].ID == post1ID {
				post1InTopResults = true
				break
			}
		}
		if !post1InTopResults {
			t.Error("expected Post 1 (exact keyword match) to be in top 3 results")
		}
	}

	// Log whether Python post appeared (informational, not a hard failure
	// since hybrid search can sometimes surface loosely related content)
	if foundPython {
		t.Log("Note: Python post appeared in results (may be due to shared programming concepts)")
	}
}

// TestSearchHybrid_FallbackToFullText tests that when the embedding service returns
// an error (simulating API failure), search gracefully falls back to full-text only
// and still returns results.
func TestSearchHybrid_FallbackToFullText(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	ctx := context.Background()

	// Insert a post with the keyword "golang" (no embedding needed for full-text)
	postID := insertTestPost(t, pool, ctx, "problem",
		"Advanced Golang concurrency patterns",
		"Using goroutines channels and mutexes for concurrent programming in golang applications.",
		[]string{"golang"}, "open")

	// Use broken embedding service that always errors
	repo := NewSearchRepository(pool)
	repo.SetEmbeddingService(&brokenEmbeddingService{})

	// Search should still work via full-text fallback
	results, total, _, err := repo.Search(ctx, "golang", models.SearchOptions{
		Page:    1,
		PerPage: 20,
	})

	if err != nil {
		t.Fatalf("Search should not fail when embedding service errors: %v", err)
	}

	if total == 0 {
		t.Error("expected results from full-text fallback when embeddings fail")
	}

	found := false
	for _, r := range results {
		if r.ID == postID {
			found = true
			break
		}
	}

	if !found {
		t.Error("expected to find test post via full-text fallback when embedding service fails")
	}
}

// TestSearchHybrid_EmptyEmbeddings tests that when posts have no embeddings
// (embedding IS NULL), the hybrid search still works by relying on full-text
// search only. This ensures no crashes on NULL embeddings.
func TestSearchHybrid_EmptyEmbeddings(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	apiKey := requireEmbeddingAPIKey(t)
	ctx := context.Background()

	// Insert posts WITHOUT embeddings (standard insertTestPost doesn't set embedding)
	postID := insertTestPost(t, pool, ctx, "problem",
		"Testing null embeddings in hybrid search",
		"This post has no embedding vector but should still be found by full-text search in hybrid mode.",
		[]string{"testing"}, "open")

	// Use real embedding service for the query (but posts have NULL embeddings)
	embedSvc := &testEmbeddingService{apiKey: apiKey}
	repo := NewSearchRepository(pool)
	repo.SetEmbeddingService(embedSvc)

	// Search should work - the hybrid_search SQL function handles NULL embeddings
	// via the "embedding IS NOT NULL" filter in the semantic CTE
	results, total, _, err := repo.Search(ctx, "null embeddings hybrid search", models.SearchOptions{
		Page:    1,
		PerPage: 20,
	})

	if err != nil {
		t.Fatalf("Hybrid search with NULL embeddings should not crash: %v", err)
	}

	if total == 0 {
		t.Error("expected results from full-text component when posts have NULL embeddings")
	}

	found := false
	for _, r := range results {
		if r.ID == postID {
			found = true
			break
		}
	}

	if !found {
		t.Error("expected to find post without embedding via full-text component of hybrid search")
	}
}

// TestSearchHybrid_RRFWeighting tests that Reciprocal Rank Fusion (RRF) properly
// combines results from both full-text and vector search. Posts that rank high in
// one method should still appear in top results even if they rank lower in the other.
func TestSearchHybrid_RRFWeighting(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	apiKey := requireEmbeddingAPIKey(t)
	ctx := context.Background()

	// Post A: Strong keyword match for "database optimization" but weaker semantic relation
	// to the query "improve database query performance"
	postAID := insertTestPostWithEmbedding(t, pool, ctx, apiKey, "problem",
		"Database Optimization Techniques for Query Performance",
		"Database optimization and query performance tuning. Use indexes, query plans, and database optimization strategies to improve query execution speed.",
		[]string{"database", "optimization"}, "open")

	// Post B: Weaker keyword match but strong semantic relation
	// Uses different vocabulary but means the same thing
	postBID := insertTestPostWithEmbedding(t, pool, ctx, apiKey, "problem",
		"Speeding Up Slow SQL Queries",
		"When your SQL statements take too long to execute, analyze the execution plan, add proper indexing, denormalize where appropriate, and consider caching frequently accessed data.",
		[]string{"sql", "performance"}, "open")

	embedSvc := &testEmbeddingService{apiKey: apiKey}
	repo := NewSearchRepository(pool)
	repo.SetEmbeddingService(embedSvc)

	results, total, _, err := repo.Search(ctx, "improve database query performance", models.SearchOptions{
		Page:    1,
		PerPage: 20,
	})

	if err != nil {
		t.Fatalf("RRF weighting search failed: %v", err)
	}

	if total == 0 {
		t.Fatal("expected results from RRF-weighted hybrid search")
	}

	// Both posts should appear in results - Post A via keyword match,
	// Post B via semantic similarity (both discuss the same concept)
	foundA := false
	foundB := false

	for _, r := range results {
		switch r.ID {
		case postAID:
			foundA = true
		case postBID:
			foundB = true
		}
	}

	if !foundA {
		t.Error("expected Post A (strong keyword match) to appear in RRF results")
	}

	if !foundB {
		t.Error("expected Post B (strong semantic match) to appear in RRF results")
	}

	// Both should be in top results due to RRF fusion boosting them
	if len(results) >= 2 && foundA && foundB {
		topN := 5
		if topN > len(results) {
			topN = len(results)
		}
		aInTop := false
		bInTop := false
		for i := 0; i < topN; i++ {
			if results[i].ID == postAID {
				aInTop = true
			}
			if results[i].ID == postBID {
				bInTop = true
			}
		}
		if !aInTop {
			t.Error("expected Post A to be in top 5 results due to RRF fusion")
		}
		if !bInTop {
			t.Error("expected Post B to be in top 5 results due to RRF fusion")
		}
	}
}

// requireEmbeddingAPIKey returns the embedding API key or skips the test.
func requireEmbeddingAPIKey(t *testing.T) string {
	t.Helper()
	apiKey := os.Getenv("VOYAGE_API_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("EMBEDDING_API_KEY")
	}
	if apiKey == "" {
		t.Skip("VOYAGE_API_KEY or EMBEDDING_API_KEY not set, skipping hybrid semantic search test")
	}
	return apiKey
}
