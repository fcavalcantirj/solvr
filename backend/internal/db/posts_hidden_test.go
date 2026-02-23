// Package db provides database access for Solvr.
package db

import (
	"context"
	"os"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// TestListPosts_ExcludesPendingReview verifies that posts with pending_review status
// are excluded from listing when IncludeHidden is false (default).
func TestListPosts_ExcludesPendingReview(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	repo := NewPostRepository(pool)
	ctx := context.Background()

	// Insert a post with pending_review status
	insertTestPost(t, pool, ctx, "question", "Pending review question about Go testing",
		"This question is pending moderation review and should not appear in listings.",
		[]string{"go", "testing"}, "pending_review")

	// Also insert a visible post to confirm listing works
	insertTestPost(t, pool, ctx, "question", "Visible question about Go testing",
		"This question is open and should appear in listings.",
		[]string{"go", "testing"}, "open")

	// List without author filter (public listing) — should exclude pending_review
	posts, total, err := repo.List(ctx, models.PostListOptions{
		Page:    1,
		PerPage: 50,
	})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	// Should NOT return any pending_review posts (may include production data)
	for _, p := range posts {
		if p.Status == models.PostStatusPendingReview {
			t.Errorf("expected pending_review posts to be excluded, but found post %s with status pending_review", p.ID)
		}
	}
	if total == 0 {
		t.Errorf("expected at least 1 post (the visible open post), got %d", total)
	}
}

// TestListPosts_ExcludesRejected verifies that posts with rejected status
// are excluded from listing when IncludeHidden is false (default).
func TestListPosts_ExcludesRejected(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	repo := NewPostRepository(pool)
	ctx := context.Background()

	// Insert a rejected post
	insertTestPost(t, pool, ctx, "idea", "Rejected idea about AI agents",
		"This idea was rejected by moderation and should not appear in listings.",
		[]string{"ai", "agents"}, "rejected")

	// Insert a visible post
	insertTestPost(t, pool, ctx, "idea", "Visible idea about AI agents",
		"This idea is open and should appear in listings.",
		[]string{"ai", "agents"}, "open")

	// List without author filter — should exclude rejected
	posts, total, err := repo.List(ctx, models.PostListOptions{
		Page:    1,
		PerPage: 50,
	})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	for _, p := range posts {
		if p.Status == models.PostStatusRejected {
			t.Errorf("expected rejected posts to be excluded, but found post %s with status rejected", p.ID)
		}
	}
	if total == 0 {
		t.Errorf("expected at least 1 post (the visible open post), got %d", total)
	}
}

// TestListPosts_ExcludesDraft verifies that posts with draft status
// are excluded from listing when IncludeHidden is false (default).
func TestListPosts_ExcludesDraft(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	repo := NewPostRepository(pool)
	ctx := context.Background()

	// Insert a draft post
	insertTestPost(t, pool, ctx, "problem", "Draft problem about performance",
		"This problem is a draft and should not appear in public listings.",
		[]string{"performance"}, "draft")

	// Insert a visible post
	insertTestPost(t, pool, ctx, "problem", "Visible problem about performance",
		"This problem is open and should appear in listings.",
		[]string{"performance"}, "open")

	// List without author filter — should exclude draft
	posts, total, err := repo.List(ctx, models.PostListOptions{
		Page:    1,
		PerPage: 50,
	})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	for _, p := range posts {
		if p.Status == models.PostStatusDraft {
			t.Errorf("expected draft posts to be excluded, but found post %s with status draft", p.ID)
		}
	}
	if total == 0 {
		t.Errorf("expected at least 1 post (the visible open post), got %d", total)
	}
}

// TestListPosts_AuthorSeesOwnHidden verifies that when IncludeHidden is true,
// posts with pending_review status are included (author self-view).
func TestListPosts_AuthorSeesOwnHidden(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	repo := NewPostRepository(pool)
	ctx := context.Background()

	authorType := "human"
	authorID := "test-hidden-author"

	// Insert a pending_review post by specific author
	insertTestPostWithAuthor(t, pool, ctx, "question", "My pending review question",
		"This question is pending review but the author should see it.",
		[]string{"go"}, "pending_review", authorType, authorID)

	// List with IncludeHidden=true and matching author filter
	posts, total, err := repo.List(ctx, models.PostListOptions{
		Page:          1,
		PerPage:       50,
		AuthorType:    models.AuthorType(authorType),
		AuthorID:      authorID,
		IncludeHidden: true,
	})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if total != 1 {
		t.Errorf("expected total=1 (author should see own pending_review post), got %d", total)
	}
	if len(posts) != 1 {
		t.Errorf("expected 1 post, got %d", len(posts))
	}
	if len(posts) > 0 && posts[0].Status != models.PostStatusPendingReview {
		t.Errorf("expected status pending_review, got %s", posts[0].Status)
	}
}

// TestSearchPosts_ExcludesPendingReview verifies that posts with pending_review status
// are excluded from search results.
func TestSearchPosts_ExcludesPendingReview(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}

	pool := setupTestDB(t)
	defer pool.Close()

	searchRepo := NewSearchRepository(pool)
	ctx := context.Background()

	// Insert a pending_review post with searchable content
	insertTestPost(t, pool, ctx, "problem", "Pending review database optimization techniques",
		"Advanced database optimization techniques for PostgreSQL pending moderation review.",
		[]string{"postgresql", "optimization"}, "pending_review")

	// Insert a visible post with similar content
	insertTestPost(t, pool, ctx, "problem", "Open database optimization techniques for PostgreSQL",
		"Standard database optimization techniques that are publicly visible.",
		[]string{"postgresql", "optimization"}, "open")

	// Search for "database optimization"
	results, total, _, err := searchRepo.Search(ctx, "database optimization", models.SearchOptions{
		Page:    1,
		PerPage: 50,
	})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	// pending_review post should not appear in results
	for _, r := range results {
		if r.Status == "pending_review" {
			t.Errorf("expected pending_review posts to be excluded from search, but found result %s with status pending_review", r.ID)
		}
	}

	// Should find at least the open post
	if total == 0 {
		t.Error("expected at least 1 search result (the open post)")
	}

	// Verify the open post is found
	foundOpen := false
	for _, r := range results {
		if r.Status == "open" {
			foundOpen = true
		}
	}
	if !foundOpen {
		t.Error("expected to find the open post in search results")
	}
}
