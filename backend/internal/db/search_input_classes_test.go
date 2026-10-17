package db

import (
	"context"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/stretchr/testify/assert"
)

// SPEC.md 22.7: the search answers every input class, on the keyword path and on the hybrid
// path production runs (every logged production search since 2026-10-05 was hybrid).
func TestSearchRepository_AnswersEveryInputClass(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()
	ctx := context.Background()
	insertTestPost(t, pool, ctx, "post", "Cannot find module express at runtime",
		"The module express is missing from List<String> builds; KeyError body in the handler.", []string{"node"}, "open")

	keyword := NewSearchRepository(pool)
	hybrid := NewSearchRepository(pool)
	hybrid.SetEmbeddingService(fixedQueryEmbedder{})
	for name, repo := range map[string]*SearchRepository{"keyword": keyword, "hybrid": hybrid} {
		for _, in := range tsqueryInputClasses {
			_, _, _, _, err := repo.Search(ctx, in, models.SearchOptions{Page: 1, PerPage: 20})
			assert.NoError(t, err, "%s path, input %q", name, in)
		}
	}
}
