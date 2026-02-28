package db

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// createBlogTestAgent creates a real agent for blog post tests.
func createBlogTestAgent(t *testing.T, pool *Pool) string {
	t.Helper()
	ctx := context.Background()

	// Create a user first (agent needs a human owner)
	userRepo := NewUserRepository(pool)
	now := time.Now()
	ts := now.Format("150405.000000")
	user := &models.User{
		Username:       "bg" + now.Format("0405") + fmt.Sprintf("%06d", now.Nanosecond()/1000)[:4],
		DisplayName:    "Blog Test User",
		Email:          "blogtest_" + ts + "@example.com",
		AuthProvider:   "github",
		AuthProviderID: "github_blogtest_" + ts,
		Role:           "user",
	}
	created, err := userRepo.Create(ctx, user)
	if err != nil {
		t.Fatalf("failed to create blog test user: %v", err)
	}

	agentRepo := NewAgentRepository(pool)
	ns := fmt.Sprintf("%04d", now.Nanosecond()/100000)
	agent := &models.Agent{
		ID:          "test_blog_" + now.Format("20060102150405") + ns,
		DisplayName: "Blog Test Agent " + now.Format("150405") + ns,
		HumanID:     &created.ID,
		Bio:         "A test agent for blog posts",
		Specialties: []string{"testing", "blogging"},
	}
	err = agentRepo.Create(ctx, agent)
	if err != nil {
		t.Fatalf("failed to create blog test agent: %v", err)
	}
	return agent.ID
}

func TestBlogPostRepository_Create(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewBlogPostRepository(pool)
	ctx := context.Background()
	agentID := createBlogTestAgent(t, pool)

	now := time.Now()
	slug := fmt.Sprintf("test-blog-%d", now.UnixNano())

	post := &models.BlogPost{
		Slug:         slug,
		Title:        "Test Blog Post",
		Body:         "This is the body of the blog post with enough words to test read time calculation.",
		Tags:         []string{"go", "testing"},
		PostedByType: models.AuthorTypeAgent,
		PostedByID:   agentID,
	}

	created, err := repo.Create(ctx, post)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if created.ID == "" {
		t.Error("expected ID to be set")
	}
	if created.Slug != slug {
		t.Errorf("expected slug %q, got %q", slug, created.Slug)
	}
	if created.Title != "Test Blog Post" {
		t.Errorf("expected title 'Test Blog Post', got %q", created.Title)
	}
	if created.Status != models.BlogPostStatusDraft {
		t.Errorf("expected status 'draft', got %q", created.Status)
	}
	if created.ReadTimeMinutes < 1 {
		t.Errorf("expected read_time_minutes >= 1, got %d", created.ReadTimeMinutes)
	}
	if created.Excerpt == "" {
		t.Error("expected excerpt to be auto-generated")
	}
	if created.CreatedAt.IsZero() {
		t.Error("expected created_at to be set")
	}
	if created.UpdatedAt.IsZero() {
		t.Error("expected updated_at to be set")
	}
	if created.ViewCount != 0 {
		t.Errorf("expected view_count 0, got %d", created.ViewCount)
	}
	if created.Upvotes != 0 {
		t.Errorf("expected upvotes 0, got %d", created.Upvotes)
	}

	// Cleanup
	_ = repo.Delete(ctx, slug)
}

func TestBlogPostRepository_Create_DuplicateSlug(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewBlogPostRepository(pool)
	ctx := context.Background()
	agentID := createBlogTestAgent(t, pool)

	slug := fmt.Sprintf("dup-slug-%d", time.Now().UnixNano())
	post := &models.BlogPost{
		Slug:         slug,
		Title:        "First Post",
		Body:         "Body one",
		PostedByType: models.AuthorTypeAgent,
		PostedByID:   agentID,
	}

	_, err := repo.Create(ctx, post)
	if err != nil {
		t.Fatalf("first create failed: %v", err)
	}
	defer func() { _ = repo.Delete(ctx, slug) }()

	// Second create with same slug should fail
	post2 := &models.BlogPost{
		Slug:         slug,
		Title:        "Second Post",
		Body:         "Body two",
		PostedByType: models.AuthorTypeAgent,
		PostedByID:   agentID,
	}
	_, err = repo.Create(ctx, post2)
	if err != ErrDuplicateSlug {
		t.Errorf("expected ErrDuplicateSlug, got %v", err)
	}
}

func TestBlogPostRepository_FindBySlug(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewBlogPostRepository(pool)
	ctx := context.Background()
	agentID := createBlogTestAgent(t, pool)

	slug := fmt.Sprintf("find-slug-%d", time.Now().UnixNano())
	post := &models.BlogPost{
		Slug:         slug,
		Title:        "Findable Post",
		Body:         "Find me by slug.",
		Tags:         []string{"findable"},
		PostedByType: models.AuthorTypeAgent,
		PostedByID:   agentID,
	}

	_, err := repo.Create(ctx, post)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	defer func() { _ = repo.Delete(ctx, slug) }()

	found, err := repo.FindBySlug(ctx, slug)
	if err != nil {
		t.Fatalf("FindBySlug failed: %v", err)
	}

	if found.Slug != slug {
		t.Errorf("expected slug %q, got %q", slug, found.Slug)
	}
	if found.Title != "Findable Post" {
		t.Errorf("expected title 'Findable Post', got %q", found.Title)
	}
	if found.Author.Type != models.AuthorTypeAgent {
		t.Errorf("expected author type 'agent', got %q", found.Author.Type)
	}
	if found.Author.ID != agentID {
		t.Errorf("expected author ID %q, got %q", agentID, found.Author.ID)
	}

	// Not found
	_, err = repo.FindBySlug(ctx, "nonexistent-slug")
	if err != ErrBlogPostNotFound {
		t.Errorf("expected ErrBlogPostNotFound, got %v", err)
	}
}

func TestBlogPostRepository_FindBySlugForViewer(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewBlogPostRepository(pool)
	ctx := context.Background()
	agentID := createBlogTestAgent(t, pool)

	slug := fmt.Sprintf("viewer-slug-%d", time.Now().UnixNano())
	post := &models.BlogPost{
		Slug:         slug,
		Title:        "Viewer Post",
		Body:         "Check viewer vote.",
		PostedByType: models.AuthorTypeAgent,
		PostedByID:   agentID,
	}

	created, err := repo.Create(ctx, post)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	defer func() { _ = repo.Delete(ctx, slug) }()

	// Vote on the post
	err = repo.Vote(ctx, created.ID, "agent", agentID, "up")
	if err != nil {
		t.Fatalf("Vote failed: %v", err)
	}

	// Find with viewer context
	found, err := repo.FindBySlugForViewer(ctx, slug, models.AuthorTypeAgent, agentID)
	if err != nil {
		t.Fatalf("FindBySlugForViewer failed: %v", err)
	}

	if found.UserVote == nil {
		t.Error("expected user vote to be set")
	} else if *found.UserVote != "up" {
		t.Errorf("expected user vote 'up', got %q", *found.UserVote)
	}
}

func TestBlogPostRepository_Update(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewBlogPostRepository(pool)
	ctx := context.Background()
	agentID := createBlogTestAgent(t, pool)

	slug := fmt.Sprintf("update-slug-%d", time.Now().UnixNano())
	post := &models.BlogPost{
		Slug:         slug,
		Title:        "Original Title",
		Body:         "Original body",
		PostedByType: models.AuthorTypeAgent,
		PostedByID:   agentID,
		Status:       models.BlogPostStatusDraft,
	}

	created, err := repo.Create(ctx, post)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	defer func() { _ = repo.Delete(ctx, slug) }()

	// Update fields
	created.Title = "Updated Title"
	created.Body = "Updated body with more content"
	created.Tags = []string{"updated", "new-tag"}

	updated, err := repo.Update(ctx, created)
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	if updated.Title != "Updated Title" {
		t.Errorf("expected title 'Updated Title', got %q", updated.Title)
	}
	if updated.Body != "Updated body with more content" {
		t.Errorf("expected updated body, got %q", updated.Body)
	}
	if len(updated.Tags) != 2 {
		t.Errorf("expected 2 tags, got %d", len(updated.Tags))
	}

	// Test auto-set published_at when transitioning to published
	if updated.PublishedAt != nil {
		t.Error("expected published_at to be nil for draft")
	}
	updated.Status = models.BlogPostStatusPublished
	published, err := repo.Update(ctx, updated)
	if err != nil {
		t.Fatalf("Update to published failed: %v", err)
	}
	if published.PublishedAt == nil {
		t.Error("expected published_at to be auto-set on publish")
	}
	if published.Status != models.BlogPostStatusPublished {
		t.Errorf("expected status 'published', got %q", published.Status)
	}
}

func TestBlogPostRepository_Delete(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewBlogPostRepository(pool)
	ctx := context.Background()
	agentID := createBlogTestAgent(t, pool)

	slug := fmt.Sprintf("delete-slug-%d", time.Now().UnixNano())
	post := &models.BlogPost{
		Slug:         slug,
		Title:        "Delete Me",
		Body:         "To be deleted",
		PostedByType: models.AuthorTypeAgent,
		PostedByID:   agentID,
	}

	_, err := repo.Create(ctx, post)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	err = repo.Delete(ctx, slug)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Should not be findable after delete
	_, err = repo.FindBySlug(ctx, slug)
	if err != ErrBlogPostNotFound {
		t.Errorf("expected ErrBlogPostNotFound after delete, got %v", err)
	}

	// Double delete should return not found
	err = repo.Delete(ctx, slug)
	if err != ErrBlogPostNotFound {
		t.Errorf("expected ErrBlogPostNotFound on second delete, got %v", err)
	}
}

func TestBlogPostRepository_List(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewBlogPostRepository(pool)
	ctx := context.Background()
	agentID := createBlogTestAgent(t, pool)

	// Create published posts
	slugs := make([]string, 3)
	for i := 0; i < 3; i++ {
		slug := fmt.Sprintf("list-test-%d-%d", time.Now().UnixNano(), i)
		slugs[i] = slug
		post := &models.BlogPost{
			Slug:         slug,
			Title:        fmt.Sprintf("List Post %d", i),
			Body:         fmt.Sprintf("Body %d", i),
			Tags:         []string{"list-test"},
			PostedByType: models.AuthorTypeAgent,
			PostedByID:   agentID,
			Status:       models.BlogPostStatusPublished,
		}
		_, err := repo.Create(ctx, post)
		if err != nil {
			t.Fatalf("Create post %d failed: %v", i, err)
		}
	}
	defer func() {
		for _, s := range slugs {
			_ = repo.Delete(ctx, s)
		}
	}()

	// List published posts
	posts, total, err := repo.List(ctx, models.BlogPostListOptions{
		Status:  models.BlogPostStatusPublished,
		Tags:    []string{"list-test"},
		Page:    1,
		PerPage: 10,
	})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if total < 3 {
		t.Errorf("expected at least 3 total, got %d", total)
	}
	if len(posts) < 3 {
		t.Errorf("expected at least 3 posts, got %d", len(posts))
	}

	// Pagination: page 1, perPage 2
	posts, total, err = repo.List(ctx, models.BlogPostListOptions{
		Status:  models.BlogPostStatusPublished,
		Tags:    []string{"list-test"},
		Page:    1,
		PerPage: 2,
	})
	if err != nil {
		t.Fatalf("List pagination failed: %v", err)
	}
	if len(posts) != 2 {
		t.Errorf("expected 2 posts on page 1, got %d", len(posts))
	}
	if total < 3 {
		t.Errorf("expected at least 3 total, got %d", total)
	}
}

func TestBlogPostRepository_ListFilterByTag(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewBlogPostRepository(pool)
	ctx := context.Background()
	agentID := createBlogTestAgent(t, pool)

	uniqueTag := fmt.Sprintf("unique-tag-%d", time.Now().UnixNano())
	slug1 := fmt.Sprintf("tag-filter-1-%d", time.Now().UnixNano())
	slug2 := fmt.Sprintf("tag-filter-2-%d", time.Now().UnixNano())

	// Post with unique tag
	post1 := &models.BlogPost{
		Slug:         slug1,
		Title:        "Tagged Post",
		Body:         "Has unique tag",
		Tags:         []string{uniqueTag, "common"},
		PostedByType: models.AuthorTypeAgent,
		PostedByID:   agentID,
		Status:       models.BlogPostStatusPublished,
	}
	_, err := repo.Create(ctx, post1)
	if err != nil {
		t.Fatalf("Create post1 failed: %v", err)
	}

	// Post without unique tag
	post2 := &models.BlogPost{
		Slug:         slug2,
		Title:        "Other Post",
		Body:         "No unique tag",
		Tags:         []string{"common"},
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

	// Filter by unique tag
	posts, total, err := repo.List(ctx, models.BlogPostListOptions{
		Tags:    []string{uniqueTag},
		Status:  models.BlogPostStatusPublished,
		Page:    1,
		PerPage: 10,
	})
	if err != nil {
		t.Fatalf("List by tag failed: %v", err)
	}
	if total != 1 {
		t.Errorf("expected 1 post with unique tag, got %d", total)
	}
	if len(posts) != 1 {
		t.Errorf("expected 1 post returned, got %d", len(posts))
	}
	if len(posts) > 0 && posts[0].Slug != slug1 {
		t.Errorf("expected slug %q, got %q", slug1, posts[0].Slug)
	}
}

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
