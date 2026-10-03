package db

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// Benchmark tests for PostRepository.List() with different sort options.
// These measure the impact of correlated subqueries vs optimized JOINs.
//
// Run: DATABASE_URL=... go test ./internal/db/... -bench=BenchmarkPostRepository -benchmem -count=3

// setupBenchmarkData creates numPosts posts with answersPerPost answers and
// approachesPerPost approaches each. Returns a cleanup function.
func setupBenchmarkData(b *testing.B, pool *Pool, numPosts, answersPerPost, approachesPerPost int) func() {
	b.Helper()
	ctx := context.Background()

	repo := NewPostRepository(pool)
	var postIDs []string

	for i := 0; i < numPosts; i++ {
		post, err := repo.Create(ctx, &models.Post{
			Type:         models.PostTypePost,
			Title:        fmt.Sprintf("Bench Post %d", i),
			Description:  fmt.Sprintf("Benchmark post %d for perf testing", i),
			Tags:         []string{"bench", "perf"},
			PostedByType: models.AuthorTypeAgent,
			PostedByID:   "bench_agent",
			Status:       models.PostStatusOpen,
		})
		if err != nil {
			b.Fatalf("failed to create benchmark post %d: %v", i, err)
		}
		postIDs = append(postIDs, post.ID)

		// Replies shaped like migrated answers on even posts and approaches on odd ones, so the
		// listing's per-bucket counts have work to do.
		legacyType, n := "answer", answersPerPost
		if i%2 == 1 {
			legacyType, n = "approach", approachesPerPost
		}
		for j := 0; j < n; j++ {
			_, err := pool.Exec(ctx, `INSERT INTO replies (post_id, author_type, author_id, body, legacy_type, legacy_id)
				VALUES ($1, 'agent', 'bench_agent', $2, $3, gen_random_uuid())`, post.ID, fmt.Sprintf("%s %d", legacyType, j), legacyType)
			if err != nil {
				b.Fatalf("failed to insert benchmark %s reply: %v", legacyType, err)
			}
		}
	}

	return func() {
		for _, id := range postIDs {
			_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE id = $1", id)
		}
	}
}

func getBenchPool(b *testing.B) *Pool {
	b.Helper()
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		b.Skip("DATABASE_URL not set, skipping benchmark")
	}
	ctx := context.Background()
	pool, err := NewPool(ctx, databaseURL)
	if err != nil {
		b.Fatalf("failed to connect to database: %v", err)
	}
	return pool
}

func BenchmarkPostRepository_List_Newest(b *testing.B) {
	pool := getBenchPool(b)
	defer pool.Close()

	cleanup := setupBenchmarkData(b, pool, 100, 5, 3)
	defer cleanup()

	repo := NewPostRepository(pool)
	ctx := context.Background()
	opts := models.PostListOptions{Page: 1, PerPage: 20}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, err := repo.List(ctx, opts)
		if err != nil {
			b.Fatalf("List() error = %v", err)
		}
	}
}

func BenchmarkPostRepository_List_SortByVotes(b *testing.B) {
	pool := getBenchPool(b)
	defer pool.Close()

	cleanup := setupBenchmarkData(b, pool, 100, 5, 3)
	defer cleanup()

	repo := NewPostRepository(pool)
	ctx := context.Background()
	opts := models.PostListOptions{Page: 1, PerPage: 20, Sort: "votes"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, err := repo.List(ctx, opts)
		if err != nil {
			b.Fatalf("List() error = %v", err)
		}
	}
}

func BenchmarkPostRepository_List_SortByAnswers(b *testing.B) {
	pool := getBenchPool(b)
	defer pool.Close()

	cleanup := setupBenchmarkData(b, pool, 100, 5, 3)
	defer cleanup()

	repo := NewPostRepository(pool)
	ctx := context.Background()
	opts := models.PostListOptions{Page: 1, PerPage: 20, Sort: "answers"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, err := repo.List(ctx, opts)
		if err != nil {
			b.Fatalf("List() error = %v", err)
		}
	}
}

func BenchmarkPostRepository_List_SortByApproaches(b *testing.B) {
	pool := getBenchPool(b)
	defer pool.Close()

	cleanup := setupBenchmarkData(b, pool, 100, 5, 3)
	defer cleanup()

	repo := NewPostRepository(pool)
	ctx := context.Background()
	opts := models.PostListOptions{Page: 1, PerPage: 20, Sort: "approaches"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, err := repo.List(ctx, opts)
		if err != nil {
			b.Fatalf("List() error = %v", err)
		}
	}
}
