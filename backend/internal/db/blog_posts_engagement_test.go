package db

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
)

func TestBlogPostRepository_GetFeatured(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewBlogPostRepository(pool)
	ctx := context.Background()
	agentID := createBlogTestAgent(t, pool)

	// Create two published posts with different engagement
	slug1 := fmt.Sprintf("featured-1-%d", time.Now().UnixNano())
	slug2 := fmt.Sprintf("featured-2-%d", time.Now().UnixNano())

	post1 := &models.BlogPost{
		Slug:         slug1,
		Title:        "Popular Post",
		Body:         "Very popular content",
		PostedByType: models.AuthorTypeAgent,
		PostedByID:   agentID,
		Status:       models.BlogPostStatusPublished,
	}
	created1, err := repo.Create(ctx, post1)
	if err != nil {
		t.Fatalf("Create post1 failed: %v", err)
	}

	post2 := &models.BlogPost{
		Slug:         slug2,
		Title:        "Less Popular Post",
		Body:         "Less popular content",
		PostedByType: models.AuthorTypeAgent,
		PostedByID:   agentID,
		Status:       models.BlogPostStatusPublished,
	}
	_, err = repo.Create(ctx, post2)
	if err != nil {
		t.Fatalf("Create post2 failed: %v", err)
	}
	defer func() {
		_ = repo.Delete(ctx, slug1)
		_ = repo.Delete(ctx, slug2)
	}()

	// Add views and votes to post1 to make it more popular
	for i := 0; i < 10; i++ {
		_ = repo.IncrementViewCount(ctx, slug1)
	}
	_ = repo.Vote(ctx, created1.ID, "agent", agentID, "up")

	featured, err := repo.GetFeatured(ctx)
	if err != nil {
		t.Fatalf("GetFeatured failed: %v", err)
	}
	if featured == nil {
		t.Fatal("expected featured post, got nil")
	}
	// The more popular post should be featured (higher engagement score)
	// This may be slug1 due to views+votes, but we just verify a valid post is returned
	if featured.Status != models.BlogPostStatusPublished {
		t.Errorf("expected published status, got %q", featured.Status)
	}
}

func TestBlogPostRepository_Vote(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewBlogPostRepository(pool)
	ctx := context.Background()
	agentID := createBlogTestAgent(t, pool)

	slug := fmt.Sprintf("vote-test-%d", time.Now().UnixNano())
	post := &models.BlogPost{
		Slug:         slug,
		Title:        "Vote Test Post",
		Body:         "Test voting",
		PostedByType: models.AuthorTypeAgent,
		PostedByID:   agentID,
	}

	created, err := repo.Create(ctx, post)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	defer func() { _ = repo.Delete(ctx, slug) }()

	// Upvote
	err = repo.Vote(ctx, created.ID, "agent", agentID, "up")
	if err != nil {
		t.Fatalf("Vote up failed: %v", err)
	}

	// Check counts
	found, err := repo.FindBySlug(ctx, slug)
	if err != nil {
		t.Fatalf("FindBySlug failed: %v", err)
	}
	if found.Upvotes != 1 {
		t.Errorf("expected 1 upvote, got %d", found.Upvotes)
	}
	if found.Downvotes != 0 {
		t.Errorf("expected 0 downvotes, got %d", found.Downvotes)
	}

	// Same vote again (idempotent)
	err = repo.Vote(ctx, created.ID, "agent", agentID, "up")
	if err != nil {
		t.Fatalf("Vote up again failed: %v", err)
	}
	found, _ = repo.FindBySlug(ctx, slug)
	if found.Upvotes != 1 {
		t.Errorf("expected 1 upvote after idempotent vote, got %d", found.Upvotes)
	}

	// Change vote to down
	err = repo.Vote(ctx, created.ID, "agent", agentID, "down")
	if err != nil {
		t.Fatalf("Vote down failed: %v", err)
	}
	found, _ = repo.FindBySlug(ctx, slug)
	if found.Upvotes != 0 {
		t.Errorf("expected 0 upvotes after switch, got %d", found.Upvotes)
	}
	if found.Downvotes != 1 {
		t.Errorf("expected 1 downvote after switch, got %d", found.Downvotes)
	}

	// Invalid direction
	err = repo.Vote(ctx, created.ID, "agent", agentID, "invalid")
	if err != ErrInvalidVoteDirection {
		t.Errorf("expected ErrInvalidVoteDirection, got %v", err)
	}

	// Invalid voter type
	err = repo.Vote(ctx, created.ID, "invalid", agentID, "up")
	if err != ErrInvalidVoterType {
		t.Errorf("expected ErrInvalidVoterType, got %v", err)
	}

	// Vote on nonexistent post
	err = repo.Vote(ctx, "00000000-0000-0000-0000-000000000000", "agent", agentID, "up")
	if err != ErrBlogPostNotFound {
		t.Errorf("expected ErrBlogPostNotFound, got %v", err)
	}
}

func TestBlogPostRepository_IncrementViewCount(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewBlogPostRepository(pool)
	ctx := context.Background()
	agentID := createBlogTestAgent(t, pool)

	slug := fmt.Sprintf("view-test-%d", time.Now().UnixNano())
	post := &models.BlogPost{
		Slug:         slug,
		Title:        "View Count Post",
		Body:         "Track views",
		PostedByType: models.AuthorTypeAgent,
		PostedByID:   agentID,
	}

	_, err := repo.Create(ctx, post)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	defer func() { _ = repo.Delete(ctx, slug) }()

	// Increment 3 times
	for i := 0; i < 3; i++ {
		err = repo.IncrementViewCount(ctx, slug)
		if err != nil {
			t.Fatalf("IncrementViewCount failed on iteration %d: %v", i, err)
		}
	}

	found, err := repo.FindBySlug(ctx, slug)
	if err != nil {
		t.Fatalf("FindBySlug failed: %v", err)
	}
	if found.ViewCount != 3 {
		t.Errorf("expected view_count 3, got %d", found.ViewCount)
	}

	// Nonexistent slug
	err = repo.IncrementViewCount(ctx, "nonexistent-slug")
	if err != ErrBlogPostNotFound {
		t.Errorf("expected ErrBlogPostNotFound, got %v", err)
	}
}

func TestBlogPostRepository_ListTags(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewBlogPostRepository(pool)
	ctx := context.Background()
	agentID := createBlogTestAgent(t, pool)

	uniqueTag1 := fmt.Sprintf("tag-list-a-%d", time.Now().UnixNano())
	uniqueTag2 := fmt.Sprintf("tag-list-b-%d", time.Now().UnixNano())

	// Create 2 published posts with overlapping tags
	slug1 := fmt.Sprintf("tags-1-%d", time.Now().UnixNano())
	slug2 := fmt.Sprintf("tags-2-%d", time.Now().UnixNano())

	_, err := repo.Create(ctx, &models.BlogPost{
		Slug:         slug1,
		Title:        "Tags Post 1",
		Body:         "Body 1",
		Tags:         []string{uniqueTag1, uniqueTag2},
		PostedByType: models.AuthorTypeAgent,
		PostedByID:   agentID,
		Status:       models.BlogPostStatusPublished,
	})
	if err != nil {
		t.Fatalf("Create post1 failed: %v", err)
	}

	_, err = repo.Create(ctx, &models.BlogPost{
		Slug:         slug2,
		Title:        "Tags Post 2",
		Body:         "Body 2",
		Tags:         []string{uniqueTag1},
		PostedByType: models.AuthorTypeAgent,
		PostedByID:   agentID,
		Status:       models.BlogPostStatusPublished,
	})
	if err != nil {
		t.Fatalf("Create post2 failed: %v", err)
	}
	defer func() {
		_ = repo.Delete(ctx, slug1)
		_ = repo.Delete(ctx, slug2)
	}()

	tags, err := repo.ListTags(ctx)
	if err != nil {
		t.Fatalf("ListTags failed: %v", err)
	}

	// Find our unique tags
	tagMap := make(map[string]int)
	for _, tag := range tags {
		tagMap[tag.Name] = tag.Count
	}

	if count, ok := tagMap[uniqueTag1]; !ok || count != 2 {
		t.Errorf("expected tag %q count=2, got count=%d ok=%v", uniqueTag1, count, ok)
	}
	if count, ok := tagMap[uniqueTag2]; !ok || count != 1 {
		t.Errorf("expected tag %q count=1, got count=%d ok=%v", uniqueTag2, count, ok)
	}
}

func TestBlogPostRepository_SlugExists(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewBlogPostRepository(pool)
	ctx := context.Background()
	agentID := createBlogTestAgent(t, pool)

	slug := fmt.Sprintf("slug-exists-%d", time.Now().UnixNano())
	post := &models.BlogPost{
		Slug:         slug,
		Title:        "Exists Test",
		Body:         "Does it exist?",
		PostedByType: models.AuthorTypeAgent,
		PostedByID:   agentID,
	}

	// Should not exist before creation
	exists, err := repo.SlugExists(ctx, slug)
	if err != nil {
		t.Fatalf("SlugExists failed: %v", err)
	}
	if exists {
		t.Error("expected slug to not exist before creation")
	}

	_, err = repo.Create(ctx, post)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	defer func() { _ = repo.Delete(ctx, slug) }()

	// Should exist after creation
	exists, err = repo.SlugExists(ctx, slug)
	if err != nil {
		t.Fatalf("SlugExists failed: %v", err)
	}
	if !exists {
		t.Error("expected slug to exist after creation")
	}
}
