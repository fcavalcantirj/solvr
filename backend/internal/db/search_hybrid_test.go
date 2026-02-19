// Package db provides database connection pool and helper functions.
package db

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// testEmbeddingService is a minimal embedding service for testing hybrid search.
// Uses the Voyage AI API directly to avoid import cycle with services package.
type testEmbeddingService struct {
	apiKey string
}

func (s *testEmbeddingService) GenerateQueryEmbedding(ctx context.Context, text string) ([]float32, error) {
	// Use the same Voyage AI API as the real service
	return generateTestEmbedding(ctx, s.apiKey, text, "query")
}

// brokenEmbeddingService always returns an error (for fallback testing).
type brokenEmbeddingService struct{}

func (s *brokenEmbeddingService) GenerateQueryEmbedding(_ context.Context, _ string) ([]float32, error) {
	return nil, fmt.Errorf("embedding service unavailable")
}

// TestSearchRepository_HybridSearch_WithEmbeddingService tests that when an embedding service
// is provided, the search method uses hybrid search (combining full-text and vector similarity).
func TestSearchRepository_HybridSearch_WithEmbeddingService(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	ctx := context.Background()

	// Insert a test post with a known keyword
	postID := insertTestPost(t, pool, ctx, "problem",
		"Golang concurrency patterns for microservices",
		"Using goroutines and channels for concurrent processing in microservice architecture.",
		[]string{"golang", "concurrency"}, "open")

	// Create embedding service (requires VOYAGE_API_KEY env var)
	embedSvc := newTestEmbeddingService(t)

	// Create search repo with embedding service
	repo := NewSearchRepository(pool)
	repo.SetEmbeddingService(embedSvc)

	// Search should work and return results
	results, total, err := repo.Search(ctx, "golang concurrency", models.SearchOptions{
		Page:    1,
		PerPage: 20,
	})

	if err != nil {
		t.Fatalf("Hybrid search failed: %v", err)
	}

	if total == 0 {
		t.Error("expected at least 1 result from hybrid search")
	}

	// Verify the test post is found
	found := false
	for _, r := range results {
		if r.ID == postID {
			found = true
			if r.Source != "post" {
				t.Errorf("expected source 'post', got '%s'", r.Source)
			}
			break
		}
	}

	if !found {
		t.Error("expected to find the test post in hybrid search results")
	}
}

// TestSearchRepository_HybridSearch_FallbackWithoutEmbeddingService tests that search
// falls back to full-text only when no embedding service is provided.
func TestSearchRepository_HybridSearch_FallbackWithoutEmbeddingService(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	ctx := context.Background()

	// Insert a test post
	postID := insertTestPost(t, pool, ctx, "problem",
		"Testing fallback search behavior",
		"This post should be found via full-text search without embeddings.",
		[]string{"testing"}, "open")

	// Create search repo WITHOUT embedding service (nil)
	repo := NewSearchRepository(pool)

	// Search should work using full-text only
	results, total, err := repo.Search(ctx, "fallback search", models.SearchOptions{
		Page:    1,
		PerPage: 20,
	})

	if err != nil {
		t.Fatalf("Fallback search failed: %v", err)
	}

	if total == 0 {
		t.Error("expected at least 1 result from fallback full-text search")
	}

	found := false
	for _, r := range results {
		if r.ID == postID {
			found = true
			break
		}
	}

	if !found {
		t.Error("expected to find the test post in fallback search results")
	}
}

// TestSearchRepository_HybridSearch_FiltersWork tests that type/status/tag filters
// still work with hybrid search.
func TestSearchRepository_HybridSearch_FiltersWork(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	ctx := context.Background()

	// Insert posts of different types
	insertTestPost(t, pool, ctx, "problem",
		"Hybrid filter test problem",
		"A problem to test filtering in hybrid search mode.",
		[]string{"hybrid"}, "open")

	insertTestPost(t, pool, ctx, "question",
		"Hybrid filter test question",
		"A question to test filtering in hybrid search mode.",
		[]string{"hybrid"}, "open")

	embedSvc := newTestEmbeddingService(t)
	repo := NewSearchRepository(pool)
	repo.SetEmbeddingService(embedSvc)

	// Search with type filter
	results, _, err := repo.Search(ctx, "hybrid filter test", models.SearchOptions{
		Type:    "problem",
		Page:    1,
		PerPage: 20,
	})

	if err != nil {
		t.Fatalf("Hybrid search with filter failed: %v", err)
	}

	for _, r := range results {
		if r.Type != "problem" {
			t.Errorf("expected type 'problem' with filter, got '%s'", r.Type)
		}
	}
}

// TestSearchRepository_HybridSearch_EmbeddingErrorFallback tests that if the embedding
// service returns an error, search gracefully falls back to full-text only.
func TestSearchRepository_HybridSearch_EmbeddingErrorFallback(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	ctx := context.Background()

	// Insert a test post
	postID := insertTestPost(t, pool, ctx, "problem",
		"Embedding error graceful fallback test",
		"This post should be found even when embeddings fail.",
		[]string{"fallback"}, "open")

	// Create a broken embedding service that always fails
	repo := NewSearchRepository(pool)
	repo.SetEmbeddingService(&brokenEmbeddingService{})

	// Search should fall back to full-text and still return results
	results, total, err := repo.Search(ctx, "embedding error graceful fallback", models.SearchOptions{
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
		t.Error("expected to find test post via full-text fallback")
	}
}

// TestSearchRepository_SetEmbeddingService tests the SetEmbeddingService setter method.
func TestSearchRepository_SetEmbeddingService(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	repo := NewSearchRepository(pool)

	// Initially nil
	if repo.embeddingService != nil {
		t.Error("expected embeddingService to be nil initially")
	}

	// Set embedding service
	repo.SetEmbeddingService(&brokenEmbeddingService{})

	if repo.embeddingService == nil {
		t.Error("expected embeddingService to be set after SetEmbeddingService call")
	}
}

// newTestEmbeddingService creates a test embedding service.
// Skips test if VOYAGE_API_KEY is not set.
func newTestEmbeddingService(t *testing.T) QueryEmbedder {
	t.Helper()

	apiKey := os.Getenv("VOYAGE_API_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("EMBEDDING_API_KEY")
	}
	if apiKey == "" {
		t.Skip("VOYAGE_API_KEY or EMBEDDING_API_KEY not set, skipping hybrid search test")
	}
	return &testEmbeddingService{apiKey: apiKey}
}

// generateTestEmbedding calls the Voyage AI API to generate an embedding.
// This avoids importing the services package which would cause an import cycle.
func generateTestEmbedding(ctx context.Context, apiKey, text, inputType string) ([]float32, error) {
	type embReq struct {
		Input     string `json:"input"`
		Model     string `json:"model"`
		InputType string `json:"input_type"`
	}
	type embData struct {
		Embedding []float32 `json:"embedding"`
	}
	type embResp struct {
		Data []embData `json:"data"`
	}

	reqBody := embReq{
		Input:     text,
		Model:     "voyage-code-3",
		InputType: inputType,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://api.voyageai.com/v1/embeddings", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("voyage API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var result embResp
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, err
	}
	if len(result.Data) == 0 {
		return nil, fmt.Errorf("empty embedding response")
	}
	return result.Data[0].Embedding, nil
}
