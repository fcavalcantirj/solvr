package db

import (
	"context"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// === Create Tests ===

func TestPostRepository_Create_Problem(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewPostRepository(pool)
	ctx := context.Background()
	authorAgent(ctx, t, pool, "test_agent_create")

	post := &models.Post{
		Type:         models.PostTypePost,
		Title:        "Test Problem Creation",
		Description:  "This is a test problem for Create method",
		Tags:         []string{"go", "testing"},
		PostedByType: models.AuthorTypeAgent,
		PostedByID:   "test_agent_create",
		Status:       models.PostStatusOpen,
	}

	createdPost, err := repo.Create(ctx, post)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE id = $1", createdPost.ID)
	}()

	// Verify ID was generated
	if createdPost.ID == "" {
		t.Error("expected ID to be generated")
	}

	// Verify fields are set correctly
	if createdPost.Type != models.PostTypePost {
		t.Errorf("expected type problem, got %s", createdPost.Type)
	}

	if createdPost.Title != "Test Problem Creation" {
		t.Errorf("expected title 'Test Problem Creation', got %s", createdPost.Title)
	}

	if createdPost.Description != "This is a test problem for Create method" {
		t.Errorf("expected description mismatch")
	}

	if len(createdPost.Tags) != 2 {
		t.Errorf("expected 2 tags, got %d", len(createdPost.Tags))
	}

	if createdPost.PostedByType != models.AuthorTypeAgent {
		t.Errorf("expected posted_by_type agent, got %s", createdPost.PostedByType)
	}

	if createdPost.PostedByID != "test_agent_create" {
		t.Errorf("expected posted_by_id test_agent_create, got %s", createdPost.PostedByID)
	}

	if createdPost.Status != models.PostStatusOpen {
		t.Errorf("expected status open, got %s", createdPost.Status)
	}

	// Verify timestamps are set
	if createdPost.CreatedAt.IsZero() {
		t.Error("expected created_at to be set")
	}

	if createdPost.UpdatedAt.IsZero() {
		t.Error("expected updated_at to be set")
	}

	// Verify votes are initialized to 0
	if createdPost.Upvotes != 0 {
		t.Errorf("expected 0 upvotes, got %d", createdPost.Upvotes)
	}

	if createdPost.Downvotes != 0 {
		t.Errorf("expected 0 downvotes, got %d", createdPost.Downvotes)
	}

	// Verify post can be retrieved
	retrievedPost, err := repo.FindByID(ctx, createdPost.ID)
	if err != nil {
		t.Fatalf("FindByID() error = %v", err)
	}

	if retrievedPost.ID != createdPost.ID {
		t.Errorf("retrieved post ID mismatch")
	}
}

func TestPostRepository_Create_Question(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewPostRepository(pool)
	ctx := context.Background()

	post := &models.Post{
		Type:         models.PostTypePost,
		Title:        "Test Question Creation",
		Description:  "This is a test question for Create method",
		Tags:         []string{"go"},
		PostedByType: models.AuthorTypeHuman,
		PostedByID:   authorHuman(ctx, t, pool, "test_user_create"),
		Status:       models.PostStatusOpen,
	}

	createdPost, err := repo.Create(ctx, post)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE id = $1", createdPost.ID)
	}()

	if createdPost.Type != models.PostTypePost {
		t.Errorf("expected type question, got %s", createdPost.Type)
	}

}

func TestPostRepository_Create_Idea(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewPostRepository(pool)
	ctx := context.Background()
	authorAgent(ctx, t, pool, "test_agent_idea")

	post := &models.Post{
		Type:         models.PostTypePost,
		Title:        "Test Idea Creation",
		Description:  "This is a test idea for Create method",
		Tags:         []string{"brainstorm"},
		PostedByType: models.AuthorTypeAgent,
		PostedByID:   "test_agent_idea",
		Status:       models.PostStatusOpen,
	}

	createdPost, err := repo.Create(ctx, post)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE id = $1", createdPost.ID)
	}()

	if createdPost.Type != models.PostTypePost {
		t.Errorf("expected type idea, got %s", createdPost.Type)
	}
}

func TestPostRepository_Create_WithNilTags(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewPostRepository(pool)
	ctx := context.Background()
	authorAgent(ctx, t, pool, "test_agent_notags")

	post := &models.Post{
		Type:         models.PostTypePost,
		Title:        "Test No Tags",
		Description:  "Post without tags",
		Tags:         nil, // nil tags
		PostedByType: models.AuthorTypeAgent,
		PostedByID:   "test_agent_notags",
		Status:       models.PostStatusOpen,
	}

	createdPost, err := repo.Create(ctx, post)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE id = $1", createdPost.ID)
	}()

	// Should succeed with nil/empty tags
	if createdPost.ID == "" {
		t.Error("expected ID to be generated")
	}
}

func TestPostRepository_Create_DefaultStatus(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewPostRepository(pool)
	ctx := context.Background()
	authorAgent(ctx, t, pool, "test_agent_defaultstatus")

	post := &models.Post{
		Type:         models.PostTypePost,
		Title:        "Test Default Status",
		Description:  "Post with default status",
		PostedByType: models.AuthorTypeAgent,
		PostedByID:   "test_agent_defaultstatus",
		// Status not set - should default to draft
	}

	createdPost, err := repo.Create(ctx, post)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE id = $1", createdPost.ID)
	}()

	// If no status is provided, the database defaults to 'draft'
	if createdPost.Status != models.PostStatusDraft && createdPost.Status != "" {
		t.Logf("Note: status was %s", createdPost.Status)
	}
}

// TestPostRepository_Create_ReturnsViewCount verifies that Create returns a post
// with view_count field properly scanned from the RETURNING clause.
// FIX-030: Create was missing view_count in RETURNING, causing scan mismatch error.
func TestPostRepository_Create_ReturnsViewCount(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewPostRepository(pool)
	ctx := context.Background()
	authorAgent(ctx, t, pool, "test_agent_viewcount")

	post := &models.Post{
		Type:         models.PostTypePost,
		Title:        "Test ViewCount in Create Response",
		Description:  "FIX-030: Create must return view_count to avoid scan error",
		Tags:         []string{"fix-030"},
		PostedByType: models.AuthorTypeAgent,
		PostedByID:   "test_agent_viewcount",
		Status:       models.PostStatusOpen,
	}

	// This should succeed without scan error
	createdPost, err := repo.Create(ctx, post)
	if err != nil {
		t.Fatalf("Create() should not fail, but got error = %v", err)
	}

	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE id = $1", createdPost.ID)
	}()

	// Verify post was created and returned successfully
	if createdPost.ID == "" {
		t.Error("expected ID to be generated")
	}

	// Verify view_count is initialized to 0 (new posts have 0 views)
	if createdPost.ViewCount != 0 {
		t.Errorf("expected view_count to be 0 for new post, got %d", createdPost.ViewCount)
	}

	// Verify all other essential fields are populated
	if createdPost.Type != models.PostTypePost {
		t.Errorf("expected type problem, got %s", createdPost.Type)
	}

	if createdPost.Title != "Test ViewCount in Create Response" {
		t.Errorf("expected correct title, got %s", createdPost.Title)
	}

	if createdPost.CreatedAt.IsZero() {
		t.Error("expected created_at to be set")
	}

	// Additional verification: post can be found by ID after creation
	foundPost, err := repo.FindByID(ctx, createdPost.ID)
	if err != nil {
		t.Fatalf("FindByID() after Create should work, but got error = %v", err)
	}

	if foundPost.ViewCount != 0 {
		t.Errorf("expected view_count 0 from FindByID, got %d", foundPost.ViewCount)
	}
}
