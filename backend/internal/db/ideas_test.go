// Package db provides database access for Solvr.
package db

import (
	"context"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/google/uuid"
)

// Note: These tests require a running PostgreSQL database.
// Set DATABASE_URL environment variable to run integration tests.
// Tests will be skipped if DATABASE_URL is not set.

func TestIdeasRepository_FindIdeaByID_Success(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewIdeasRepository(pool)
	ctx := context.Background()
	authorAgent(ctx, t, pool, "test_agent")

	// Create test idea
	var ideaID string
	err := pool.QueryRow(ctx, `
		INSERT INTO posts (type, title, description, posted_by_type, posted_by_id, status, tags)
		VALUES ('idea', 'Test Idea Title', 'Test idea description with lots of text', 'agent', 'test_agent', 'open', $1)
		RETURNING id::text
	`, []string{"test", "idea"}).Scan(&ideaID)
	if err != nil {
		t.Fatalf("failed to insert idea: %v", err)
	}

	// Clean up
	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE id = $1", ideaID)
	}()

	// Find idea
	idea, err := repo.FindIdeaByID(ctx, ideaID)
	if err != nil {
		t.Fatalf("FindIdeaByID() error = %v", err)
	}

	if idea == nil {
		t.Fatal("expected non-nil idea")
	}

	if idea.ID != ideaID {
		t.Errorf("expected id = %s, got %s", ideaID, idea.ID)
	}

	if idea.Type != models.PostTypeIdea {
		t.Errorf("expected type = idea, got %s", idea.Type)
	}

	if idea.Title != "Test Idea Title" {
		t.Errorf("expected title = 'Test Idea Title', got '%s'", idea.Title)
	}

	if idea.Author.Type == "" {
		t.Error("expected author type to be set")
	}
}

func TestIdeasRepository_FindIdeaByID_WrongType(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewIdeasRepository(pool)
	ctx := context.Background()
	authorAgent(ctx, t, pool, "test_agent")

	// Create a problem (not an idea)
	var problemID string
	err := pool.QueryRow(ctx, `
		INSERT INTO posts (type, title, description, posted_by_type, posted_by_id, status)
		VALUES ('problem', 'Test Problem', 'Description', 'agent', 'test_agent', 'open')
		RETURNING id::text
	`).Scan(&problemID)
	if err != nil {
		t.Fatalf("failed to insert problem: %v", err)
	}

	// Clean up
	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE id = $1", problemID)
	}()

	// Try to find it as an idea - should fail
	_, err = repo.FindIdeaByID(ctx, problemID)
	if err == nil {
		t.Error("expected error when finding non-idea as idea")
	}
	if err != ErrIdeaNotFound {
		t.Errorf("expected ErrIdeaNotFound, got %v", err)
	}
}

func TestIdeasRepository_FindIdeaByID_NotFound(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewIdeasRepository(pool)
	ctx := context.Background()

	// Try to find non-existent idea
	_, err := repo.FindIdeaByID(ctx, "non-existent-idea-id")
	if err == nil {
		t.Error("expected error when finding non-existent idea")
	}
	if err != ErrIdeaNotFound {
		t.Errorf("expected ErrIdeaNotFound, got %v", err)
	}
}

func TestIdeasRepository_CreateIdea_SetsType(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewIdeasRepository(pool)
	ctx := context.Background()

	timestamp := time.Now().Format("20060102150405")
	authorAgent(ctx, t, pool, "test_agent_"+timestamp)

	// Create idea without setting type explicitly
	post := &models.Post{
		Title:        "Test Created Idea",
		Description:  "This is a test idea created via repository",
		PostedByType: models.AuthorTypeAgent,
		PostedByID:   "test_agent_" + timestamp,
		Status:       models.PostStatusOpen,
		Tags:         []string{"test", "creation"},
	}

	created, err := repo.CreateIdea(ctx, post)

	// Clean up
	defer func() {
		if created != nil && created.ID != "" {
			_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE id = $1", created.ID)
		}
	}()

	if err != nil {
		t.Fatalf("CreateIdea() error = %v", err)
	}

	if created == nil {
		t.Fatal("expected non-nil created post")
	}

	// Verify type is set to 'idea'
	if created.Type != models.PostTypeIdea {
		t.Errorf("expected type = idea, got %s", created.Type)
	}

	if created.ID == "" {
		t.Error("expected ID to be set")
	}

	if created.Title != post.Title {
		t.Errorf("expected title = %s, got %s", post.Title, created.Title)
	}

	if created.CreatedAt.IsZero() {
		t.Error("expected created_at to be set")
	}
}

func TestIdeasRepository_AddEvolvedInto_Success(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewIdeasRepository(pool)
	ctx := context.Background()
	authorAgent(ctx, t, pool, "test_agent")

	ideaID := uuid.New().String()
	problemID := uuid.New().String()

	// Create idea
	_, err := pool.Exec(ctx, `
		INSERT INTO posts (id, type, title, description, posted_by_type, posted_by_id, status, evolved_into)
		VALUES ($1, 'idea', 'Test Idea', 'Description', 'agent', 'test_agent', 'open', '{}')
	`, ideaID)
	if err != nil {
		t.Fatalf("failed to insert idea: %v", err)
	}

	// Create problem (evolved from idea)
	_, err = pool.Exec(ctx, `
		INSERT INTO posts (id, type, title, description, posted_by_type, posted_by_id, status)
		VALUES ($1, 'problem', 'Evolved Problem', 'Description', 'agent', 'test_agent', 'open')
	`, problemID)
	if err != nil {
		t.Fatalf("failed to insert problem: %v", err)
	}

	// Clean up
	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE id = $1", ideaID)
		_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE id = $1", problemID)
	}()

	// Add evolved_into
	err = repo.AddEvolvedInto(ctx, ideaID, problemID)
	if err != nil {
		t.Fatalf("AddEvolvedInto() error = %v", err)
	}

	// Verify evolved_into contains the problem ID
	var evolvedInto []string
	err = pool.QueryRow(ctx, "SELECT evolved_into FROM posts WHERE id = $1", ideaID).Scan(&evolvedInto)
	if err != nil {
		t.Fatalf("failed to query evolved_into: %v", err)
	}

	found := false
	for _, id := range evolvedInto {
		if id == problemID {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected evolved_into to contain %s, got %v", problemID, evolvedInto)
	}

	// Verify status changed to 'evolved'
	var status string
	err = pool.QueryRow(ctx, "SELECT status FROM posts WHERE id = $1", ideaID).Scan(&status)
	if err != nil {
		t.Fatalf("failed to query status: %v", err)
	}
	if status != "evolved" {
		t.Errorf("expected status to be 'evolved', got %s", status)
	}
}
