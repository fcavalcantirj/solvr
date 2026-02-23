// Package db provides database access for Solvr.
package db

import (
	"context"
	"testing"
	"time"
)

// Note: These tests require a running PostgreSQL database.
// Set DATABASE_URL environment variable to run integration tests.
// Tests will be skipped if DATABASE_URL is not set.

func TestTagsRepository_GetOrCreateTag_New(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewTagsRepository(pool)
	ctx := context.Background()

	timestamp := time.Now().Format("20060102150405")
	tagName := "test-tag-" + timestamp

	// Create new tag
	tag, err := repo.GetOrCreateTag(ctx, tagName)
	if err != nil {
		t.Fatalf("GetOrCreateTag() error = %v", err)
	}

	// Clean up
	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM tags WHERE name = $1", tagName)
	}()

	if tag == nil {
		t.Fatal("expected non-nil tag")
	}

	if tag.ID == "" {
		t.Error("expected ID to be set")
	}

	if tag.Name != tagName {
		t.Errorf("expected name = %s, got %s", tagName, tag.Name)
	}

	if tag.UsageCount != 0 {
		t.Errorf("expected usage_count = 0, got %d", tag.UsageCount)
	}
}

func TestTagsRepository_GetOrCreateTag_Existing(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewTagsRepository(pool)
	ctx := context.Background()

	timestamp := time.Now().Format("20060102150405")
	tagName := "existing-tag-" + timestamp

	// Create tag first time
	tag1, err := repo.GetOrCreateTag(ctx, tagName)
	if err != nil {
		t.Fatalf("GetOrCreateTag() first call error = %v", err)
	}

	// Clean up
	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM tags WHERE name = $1", tagName)
	}()

	// Get the same tag again
	tag2, err := repo.GetOrCreateTag(ctx, tagName)
	if err != nil {
		t.Fatalf("GetOrCreateTag() second call error = %v", err)
	}

	// Should return the same tag (same ID)
	if tag1.ID != tag2.ID {
		t.Errorf("expected same tag ID, got %s vs %s", tag1.ID, tag2.ID)
	}
}

func TestTagsRepository_AddTagsToPost(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewTagsRepository(pool)
	ctx := context.Background()

	timestamp := time.Now().Format("20060102150405")

	// Create test post — let DB generate a proper UUID
	var postID string
	err := pool.QueryRow(ctx, `
		INSERT INTO posts (type, title, description, posted_by_type, posted_by_id, status)
		VALUES ('idea', 'Test Post', 'Description', 'agent', 'test_agent', 'open')
		RETURNING id::text
	`).Scan(&postID)
	if err != nil {
		t.Fatalf("failed to insert post: %v", err)
	}

	tagNames := []string{"tag-a-" + timestamp, "tag-b-" + timestamp, "tag-c-" + timestamp}

	// Clean up
	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM post_tags WHERE post_id = $1", postID)
		_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE id = $1", postID)
		for _, name := range tagNames {
			_, _ = pool.Exec(ctx, "DELETE FROM tags WHERE name = $1", name)
		}
	}()

	// Add tags to post
	err = repo.AddTagsToPost(ctx, postID, tagNames)
	if err != nil {
		t.Fatalf("AddTagsToPost() error = %v", err)
	}

	// Verify tags were created and linked
	var count int
	err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM post_tags WHERE post_id = $1", postID).Scan(&count)
	if err != nil {
		t.Fatalf("failed to count post_tags: %v", err)
	}

	if count != 3 {
		t.Errorf("expected 3 post_tags, got %d", count)
	}

	// Verify usage counts were incremented
	for _, name := range tagNames {
		var usageCount int
		err = pool.QueryRow(ctx, "SELECT usage_count FROM tags WHERE name = $1", name).Scan(&usageCount)
		if err != nil {
			t.Fatalf("failed to get usage_count for %s: %v", name, err)
		}
		if usageCount != 1 {
			t.Errorf("expected usage_count = 1 for %s, got %d", name, usageCount)
		}
	}
}

func TestTagsRepository_GetTagsForPost(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewTagsRepository(pool)
	ctx := context.Background()

	timestamp := time.Now().Format("20060102150405")

	// Create test post — let DB generate a proper UUID
	var postID string
	err := pool.QueryRow(ctx, `
		INSERT INTO posts (type, title, description, posted_by_type, posted_by_id, status)
		VALUES ('idea', 'Test Post', 'Description', 'agent', 'test_agent', 'open')
		RETURNING id::text
	`).Scan(&postID)
	if err != nil {
		t.Fatalf("failed to insert post: %v", err)
	}

	tagNames := []string{"alpha-" + timestamp, "beta-" + timestamp}

	// Clean up
	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM post_tags WHERE post_id = $1", postID)
		_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE id = $1", postID)
		for _, name := range tagNames {
			_, _ = pool.Exec(ctx, "DELETE FROM tags WHERE name = $1", name)
		}
	}()

	// Add tags
	err = repo.AddTagsToPost(ctx, postID, tagNames)
	if err != nil {
		t.Fatalf("AddTagsToPost() error = %v", err)
	}

	// Get tags for post
	tags, err := repo.GetTagsForPost(ctx, postID)
	if err != nil {
		t.Fatalf("GetTagsForPost() error = %v", err)
	}

	if len(tags) != 2 {
		t.Errorf("expected 2 tags, got %d", len(tags))
	}

	// Verify tag names
	foundNames := make(map[string]bool)
	for _, tag := range tags {
		foundNames[tag.Name] = true
	}

	for _, name := range tagNames {
		if !foundNames[name] {
			t.Errorf("expected to find tag %s", name)
		}
	}
}

func TestTagsRepository_GetTrendingTags(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewTagsRepository(pool)
	ctx := context.Background()

	timestamp := time.Now().Format("20060102150405")

	// Create tags with different usage counts
	tagNames := []string{
		"trending-high-" + timestamp,
		"trending-med-" + timestamp,
		"trending-low-" + timestamp,
	}
	usageCounts := []int{100, 50, 10}

	for i, name := range tagNames {
		_, err := pool.Exec(ctx, `
			INSERT INTO tags (name, usage_count, created_at)
			VALUES ($1, $2, NOW())
		`, name, usageCounts[i])
		if err != nil {
			t.Fatalf("failed to insert tag %s: %v", name, err)
		}
	}

	// Clean up
	defer func() {
		for _, name := range tagNames {
			_, _ = pool.Exec(ctx, "DELETE FROM tags WHERE name = $1", name)
		}
	}()

	// Get trending tags
	trending, err := repo.GetTrendingTags(ctx, 10)
	if err != nil {
		t.Fatalf("GetTrendingTags() error = %v", err)
	}

	// Should have at least our 3 test tags
	if len(trending) < 3 {
		t.Errorf("expected at least 3 trending tags, got %d", len(trending))
	}

	// Verify ordering (highest usage first)
	prevCount := 999999
	for _, tag := range trending {
		if tag.UsageCount > prevCount {
			t.Errorf("trending tags not sorted by usage_count descending")
			break
		}
		prevCount = tag.UsageCount
	}
}
