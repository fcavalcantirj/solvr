// Package db provides database connection pool and helper functions.
package db

import (
	"context"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// fixedEmbeddingService returns a constant query vector, so cosine similarity is fully
// deterministic and needs no external embedding API. BART-155 similarity math test.
type fixedEmbeddingService struct{ vec []float32 }

func (s *fixedEmbeddingService) GenerateQueryEmbedding(_ context.Context, _ string) ([]float32, error) {
	return s.vec, nil
}

// vec1024 builds a 1024-dim vector: first `ones` components = 1.0, the next `negs` = -1.0,
// the remainder = 0.0. Combined with a query of all-ones this yields predictable cosines.
func vec1024(ones, negs int) []float32 {
	v := make([]float32, 1024)
	for i := 0; i < ones && i < 1024; i++ {
		v[i] = 1.0
	}
	for i := ones; i < ones+negs && i < 1024; i++ {
		v[i] = -1.0
	}
	return v
}

// insertTestPostWithRawEmbedding inserts a public open post with a caller-supplied embedding
// vector (no API call). Returns the post ID.
func insertTestPostWithRawEmbedding(t *testing.T, pool *Pool, ctx context.Context, title, desc string, embedding []float32) string {
	t.Helper()
	var id string
	err := pool.QueryRow(ctx, `
		INSERT INTO posts (type, title, description, tags, status, posted_by_type, posted_by_id, embedding)
		VALUES ('problem', $1, $2, ARRAY[]::text[], 'open', 'human', 'test-user', $3::vector)
		RETURNING id::text
	`, title, desc, formatVectorLiteral(embedding)).Scan(&id)
	if err != nil {
		t.Fatalf("failed to insert post with raw embedding: %v", err)
	}
	return id
}

// findResult returns the result with the given ID, or nil.
func findResult(results []models.SearchResult, id string) *models.SearchResult {
	for i := range results {
		if results[i].ID == id {
			return &results[i]
		}
	}
	return nil
}

// TestSearch_Similarity_PopulatedAndTopSimilarity verifies the hybrid path computes a
// calibrated cosine similarity per result (1.0 for an identical embedding, ~0.5 for a
// partially-overlapping one) and that Search returns the max as top similarity. BART-155.
func TestSearch_Similarity_PopulatedAndTopSimilarity(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()
	ctx := context.Background()

	// Query vector = all ones. Post A embedding = all ones (cosine 1.0). Post B = 768 ones
	// + 256 neg-ones → cosine (512/1024) = 0.5. Both share the FTS keyword "zebracorn".
	queryVec := vec1024(1024, 0)
	idA := insertTestPostWithRawEmbedding(t, pool, ctx, "zebracorn alpha", "zebracorn identical vector post", vec1024(1024, 0))
	idB := insertTestPostWithRawEmbedding(t, pool, ctx, "zebracorn beta", "zebracorn partial vector post", vec1024(768, 256))

	repo := NewSearchRepository(pool)
	repo.SetEmbeddingService(&fixedEmbeddingService{vec: queryVec})

	results, total, method, topSim, err := repo.Search(ctx, "zebracorn", models.SearchOptions{Page: 1, PerPage: 20})
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}
	if method != "hybrid_rrf" {
		t.Fatalf("expected hybrid_rrf method, got %q", method)
	}
	if total < 2 {
		t.Fatalf("expected both posts returned, got total=%d", total)
	}

	a := findResult(results, idA)
	if a == nil || a.Similarity == nil {
		t.Fatalf("post A missing or similarity nil: %+v", a)
	}
	if *a.Similarity < 0.999 {
		t.Errorf("expected post A similarity ~1.0 (identical vector), got %v", *a.Similarity)
	}

	b := findResult(results, idB)
	if b == nil || b.Similarity == nil {
		t.Fatalf("post B missing or similarity nil: %+v", b)
	}
	if *b.Similarity < 0.49 || *b.Similarity > 0.51 {
		t.Errorf("expected post B similarity ~0.5, got %v", *b.Similarity)
	}

	if topSim == nil || *topSim < 0.999 {
		t.Errorf("expected top_similarity ~1.0, got %v", topSim)
	}
}

// TestSearch_MinSimilarity_HonestFilter verifies the opt-in floor drops below-bar results
// while still reporting the true best top similarity (computed pre-filter). BART-155.
func TestSearch_MinSimilarity_HonestFilter(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()
	ctx := context.Background()

	queryVec := vec1024(1024, 0)
	idA := insertTestPostWithRawEmbedding(t, pool, ctx, "zebracorn gamma", "zebracorn identical vector post", vec1024(1024, 0))
	idB := insertTestPostWithRawEmbedding(t, pool, ctx, "zebracorn delta", "zebracorn partial vector post", vec1024(768, 256))

	repo := NewSearchRepository(pool)
	repo.SetEmbeddingService(&fixedEmbeddingService{vec: queryVec})

	// Bar = 0.75: A (1.0) survives, B (0.5) is dropped.
	results, total, _, topSim, err := repo.Search(ctx, "zebracorn", models.SearchOptions{Page: 1, PerPage: 20, MinSimilarity: 0.75})
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}
	if total != 1 {
		t.Fatalf("expected exactly 1 result above bar, got total=%d", total)
	}
	if findResult(results, idA) == nil {
		t.Error("expected post A (similarity 1.0) to survive the filter")
	}
	if findResult(results, idB) != nil {
		t.Error("expected post B (similarity 0.5) to be dropped by min_similarity=0.75")
	}
	// top_similarity is computed BEFORE the filter, so it still reflects the best match.
	if topSim == nil || *topSim < 0.999 {
		t.Errorf("expected top_similarity ~1.0 (pre-filter best), got %v", topSim)
	}
}

// TestSearch_MinSimilarity_HonestEmpty verifies that when nothing clears the bar the result
// is a true empty (data:[], total:0) with a below-threshold top_similarity — the decidable
// "no confident match" the learning-wheel gate needs. BART-155.
func TestSearch_MinSimilarity_HonestEmpty(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()
	ctx := context.Background()

	// Query vector overlaps each post only in its first 64 dims → cosine ~0.25 for both.
	queryVec := vec1024(64, 0)
	insertTestPostWithRawEmbedding(t, pool, ctx, "zebracorn epsilon", "zebracorn identical vector post", vec1024(1024, 0))
	insertTestPostWithRawEmbedding(t, pool, ctx, "zebracorn zeta", "zebracorn partial vector post", vec1024(768, 256))

	repo := NewSearchRepository(pool)
	repo.SetEmbeddingService(&fixedEmbeddingService{vec: queryVec})

	// Bar = 0.5: both posts (~0.25) are dropped → honest empty.
	results, total, _, topSim, err := repo.Search(ctx, "zebracorn", models.SearchOptions{Page: 1, PerPage: 20, MinSimilarity: 0.5})
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}
	if total != 0 || len(results) != 0 {
		t.Fatalf("expected honest empty (total=0, no results), got total=%d len=%d", total, len(results))
	}
	// Best match still surfaced (below the bar) so the caller can decide to ASK.
	if topSim == nil {
		t.Fatal("expected a non-nil top_similarity even on empty result")
	}
	if *topSim >= 0.5 {
		t.Errorf("expected top_similarity below the 0.5 bar, got %v", *topSim)
	}
}
