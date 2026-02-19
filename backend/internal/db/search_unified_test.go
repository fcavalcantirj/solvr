// Package db provides database connection pool and helper functions.
package db

import (
	"context"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// TestSearchUnified_PostsAndAnswers tests that unified search finds results from both posts and answers.
func TestSearchUnified_PostsAndAnswers(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	repo := NewSearchRepository(pool)
	ctx := context.Background()

	// Insert a post about golang concurrency
	postID := insertTestPost(t, pool, ctx, "problem",
		"Golang concurrency patterns for web servers",
		"How to use goroutines and channels effectively in Go web applications.",
		[]string{"golang", "concurrency"}, "open")

	// Insert an answer about golang concurrency on a different question
	questionID := insertTestPost(t, pool, ctx, "question",
		"Best practices for Go APIs",
		"What are the best practices for building APIs in Go?",
		[]string{"golang", "api"}, "open")
	answerID := insertTestAnswer(t, pool, ctx, questionID,
		"Use goroutines for concurrent request handling in your Golang web server. Channels are great for coordination.",
		"human", "test-user")

	// Search with content_types=posts,answers — should find both
	results, total, _, err := repo.Search(ctx, "golang concurrency", models.SearchOptions{
		ContentTypes: []string{"posts", "answers"},
		Page:         1,
		PerPage:      20,
	})

	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	// Should find the post and potentially the answer
	if total == 0 {
		t.Fatal("expected at least 1 result from unified search")
	}

	// Check we found the post
	foundPost := false
	foundAnswer := false
	for _, r := range results {
		if r.ID == postID {
			foundPost = true
			if r.Source != "post" {
				t.Errorf("expected source 'post' for post result, got '%s'", r.Source)
			}
		}
		if r.ID == answerID {
			foundAnswer = true
			if r.Source != "answer" {
				t.Errorf("expected source 'answer' for answer result, got '%s'", r.Source)
			}
		}
	}

	if !foundPost {
		t.Errorf("expected to find post %s in results", postID)
	}

	if !foundAnswer {
		t.Errorf("expected to find answer %s in unified results (total: %d)", answerID, total)
	}
	t.Logf("Found post: %v, found answer: %v, total: %d", foundPost, foundAnswer, total)
}

// TestSearchUnified_PostsOnly tests that content_types=posts only returns posts.
func TestSearchUnified_PostsOnly(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	repo := NewSearchRepository(pool)
	ctx := context.Background()

	// Insert a post and an answer with the same keyword
	insertTestPost(t, pool, ctx, "problem",
		"Database optimization techniques",
		"How to optimize PostgreSQL queries for better performance.",
		[]string{"postgresql"}, "open")

	questionID := insertTestPost(t, pool, ctx, "question",
		"Query performance help",
		"Need help with query performance.",
		[]string{"postgresql"}, "open")
	insertTestAnswer(t, pool, ctx, questionID,
		"Database optimization is key. Use indexes and EXPLAIN ANALYZE for PostgreSQL queries.",
		"human", "test-user")

	// Search with content_types=posts only
	results, _, _, err := repo.Search(ctx, "database optimization", models.SearchOptions{
		ContentTypes: []string{"posts"},
		Page:         1,
		PerPage:      20,
	})

	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	// All results should be posts (source = "post")
	for _, r := range results {
		if r.Source != "post" {
			t.Errorf("expected only post results with content_types=posts, got source '%s' for %s", r.Source, r.ID)
		}
	}
}

// TestSearchUnified_SourceMetadata tests that results include correct source metadata.
func TestSearchUnified_SourceMetadata(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	repo := NewSearchRepository(pool)
	ctx := context.Background()

	// Insert a post
	insertTestPost(t, pool, ctx, "problem",
		"Microservices communication patterns",
		"How to implement reliable communication between microservices.",
		[]string{"microservices"}, "open")

	// Search for posts only and verify source is "post"
	results, _, _, err := repo.Search(ctx, "microservices communication", models.SearchOptions{
		ContentTypes: []string{"posts"},
		Page:         1,
		PerPage:      20,
	})

	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	for _, r := range results {
		if r.Source == "" {
			t.Error("expected non-empty source field in search results")
		}
		if r.Source != "post" {
			t.Errorf("expected source 'post', got '%s'", r.Source)
		}
	}
}

// Helper: insertTestAnswer inserts a test answer and returns its ID.
func insertTestAnswer(t *testing.T, pool *Pool, ctx context.Context, questionID, content, authorType, authorID string) string {
	var id string
	err := pool.QueryRow(ctx, `
		INSERT INTO answers (question_id, content, author_type, author_id)
		VALUES ($1::uuid, $2, $3, $4)
		RETURNING id::text
	`, questionID, content, authorType, authorID).Scan(&id)
	if err != nil {
		t.Fatalf("failed to insert test answer: %v", err)
	}
	return id
}

// Helper: insertTestApproach inserts a test approach and returns its ID.
func insertTestApproach(t *testing.T, pool *Pool, ctx context.Context, problemID, angle, method, authorType, authorID string) string {
	var id string
	err := pool.QueryRow(ctx, `
		INSERT INTO approaches (problem_id, angle, method, author_type, author_id, status)
		VALUES ($1::uuid, $2, $3, $4, $5, 'starting')
		RETURNING id::text
	`, problemID, angle, method, authorType, authorID).Scan(&id)
	if err != nil {
		t.Fatalf("failed to insert test approach: %v", err)
	}
	return id
}
