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

func TestIdeasRepository_ListIdeas_FiltersToIdeasOnly(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewIdeasRepository(pool)
	ctx := context.Background()

	// Create posts of different types
	timestamp := time.Now().Format("20060102150405")
	problemID := "ideas_filter_problem_" + timestamp
	questionID := "ideas_filter_question_" + timestamp
	ideaID := "ideas_filter_idea_" + timestamp

	// Insert problem
	_, err := pool.Exec(ctx, `
		INSERT INTO posts (id, type, title, description, posted_by_type, posted_by_id, status)
		VALUES ($1, 'problem', 'Test Problem', 'Description', 'agent', 'test_agent', 'open')
	`, problemID)
	if err != nil {
		t.Fatalf("failed to insert problem: %v", err)
	}

	// Insert question
	_, err = pool.Exec(ctx, `
		INSERT INTO posts (id, type, title, description, posted_by_type, posted_by_id, status)
		VALUES ($1, 'question', 'Test Question', 'Description', 'agent', 'test_agent', 'open')
	`, questionID)
	if err != nil {
		t.Fatalf("failed to insert question: %v", err)
	}

	// Insert idea
	_, err = pool.Exec(ctx, `
		INSERT INTO posts (id, type, title, description, posted_by_type, posted_by_id, status)
		VALUES ($1, 'idea', 'Test Idea', 'Description', 'agent', 'test_agent', 'open')
	`, ideaID)
	if err != nil {
		t.Fatalf("failed to insert idea: %v", err)
	}

	// Clean up
	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE id = $1", problemID)
		_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE id = $1", questionID)
		_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE id = $1", ideaID)
	}()

	// List ideas - should only return the idea, not problem or question
	opts := models.PostListOptions{
		Page:    1,
		PerPage: 50,
	}

	ideas, total, err := repo.ListIdeas(ctx, opts)
	if err != nil {
		t.Fatalf("ListIdeas() error = %v", err)
	}

	// Check that only ideas are returned
	for _, idea := range ideas {
		if idea.Type != models.PostTypeIdea {
			t.Errorf("ListIdeas() returned non-idea post: type = %s", idea.Type)
		}
	}

	// Should have at least 1 idea
	if total < 1 {
		t.Errorf("expected at least 1 idea, got total = %d", total)
	}

	// Verify our test idea is in the results
	found := false
	for _, idea := range ideas {
		if idea.ID == ideaID {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected to find test idea %s in results", ideaID)
	}
}

func TestIdeasRepository_FindIdeaByID_Success(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewIdeasRepository(pool)
	ctx := context.Background()

	// Create test idea
	timestamp := time.Now().Format("20060102150405")
	ideaID := "ideas_find_idea_" + timestamp

	_, err := pool.Exec(ctx, `
		INSERT INTO posts (id, type, title, description, posted_by_type, posted_by_id, status, tags)
		VALUES ($1, 'idea', 'Test Idea Title', 'Test idea description with lots of text', 'agent', 'test_agent', 'open', $2)
	`, ideaID, []string{"test", "idea"})
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

	// Create a problem (not an idea)
	timestamp := time.Now().Format("20060102150405")
	problemID := "ideas_wrongtype_" + timestamp

	_, err := pool.Exec(ctx, `
		INSERT INTO posts (id, type, title, description, posted_by_type, posted_by_id, status)
		VALUES ($1, 'problem', 'Test Problem', 'Description', 'agent', 'test_agent', 'open')
	`, problemID)
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

func TestIdeasRepository_ListIdeas_FilterByStatus(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewIdeasRepository(pool)
	ctx := context.Background()

	timestamp := time.Now().Format("20060102150405")
	openIdeaID := "ideas_status_open_" + timestamp
	activeIdeaID := "ideas_status_active_" + timestamp

	// Create open idea
	_, err := pool.Exec(ctx, `
		INSERT INTO posts (id, type, title, description, posted_by_type, posted_by_id, status)
		VALUES ($1, 'idea', 'Open Idea', 'Description', 'agent', 'test_agent', 'open')
	`, openIdeaID)
	if err != nil {
		t.Fatalf("failed to insert open idea: %v", err)
	}

	// Create active idea
	_, err = pool.Exec(ctx, `
		INSERT INTO posts (id, type, title, description, posted_by_type, posted_by_id, status)
		VALUES ($1, 'idea', 'Active Idea', 'Description', 'agent', 'test_agent', 'active')
	`, activeIdeaID)
	if err != nil {
		t.Fatalf("failed to insert active idea: %v", err)
	}

	// Clean up
	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE id = $1", openIdeaID)
		_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE id = $1", activeIdeaID)
	}()

	// List only active ideas
	opts := models.PostListOptions{
		Status:  "active",
		Page:    1,
		PerPage: 50,
	}

	ideas, _, err := repo.ListIdeas(ctx, opts)
	if err != nil {
		t.Fatalf("ListIdeas() error = %v", err)
	}

	// Check all returned ideas are active
	for _, idea := range ideas {
		if idea.Status != "active" {
			t.Errorf("expected status = active, got %s", idea.Status)
		}
	}

	// Verify active idea is in results
	found := false
	for _, idea := range ideas {
		if idea.ID == activeIdeaID {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected to find active idea %s in results", activeIdeaID)
	}
}
