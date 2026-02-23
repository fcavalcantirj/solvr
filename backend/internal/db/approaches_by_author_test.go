// Package db provides database access for Solvr.
package db

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// TestListApproachesByAuthor tests that ListByAuthor returns approaches for a specific author
// with problem title context.
func TestListApproachesByAuthor(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewApproachesRepository(pool)
	ctx := context.Background()

	timestamp := time.Now().Format("20060102150405")
	agentID := "app_by_author_agent_" + timestamp

	// Create test agent
	_, err := pool.Exec(ctx, `
		INSERT INTO agents (id, display_name, api_key_hash, status)
		VALUES ($1, $2, $3, 'active')
	`, agentID, "Test Agent", "hash_"+timestamp)
	if err != nil {
		if strings.Contains(err.Error(), "does not exist") {
			t.Skip("agents table does not exist, skipping")
		}
		t.Fatalf("failed to insert agent: %v", err)
	}

	// Create test problem
	var problemID string
	err = pool.QueryRow(ctx, `
		INSERT INTO posts (type, title, description, posted_by_type, posted_by_id, status)
		VALUES ('problem', 'My Test Problem Title', 'Description', 'agent', $1, 'open')
		RETURNING id::text
	`, agentID).Scan(&problemID)
	if err != nil {
		t.Fatalf("failed to insert problem: %v", err)
	}

	// Create approach by the agent
	approach := &models.Approach{
		ProblemID:  problemID,
		AuthorType: models.AuthorTypeAgent,
		AuthorID:   agentID,
		Angle:      "Test angle",
		Method:     "Test method",
		Status:     models.ApproachStatusWorking,
	}
	created, err := repo.CreateApproach(ctx, approach)
	if err != nil {
		t.Fatalf("failed to create approach: %v", err)
	}

	defer func() {
		if created != nil {
			_, _ = pool.Exec(ctx, "DELETE FROM approaches WHERE id = $1", created.ID)
		}
		_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE id = $1", problemID)
		_, _ = pool.Exec(ctx, "DELETE FROM agents WHERE id = $1", agentID)
	}()

	// Test ListByAuthor
	approaches, total, err := repo.ListByAuthor(ctx, string(models.AuthorTypeAgent), agentID, 1, 20)
	if err != nil {
		t.Fatalf("ListByAuthor() error = %v", err)
	}

	if total < 1 {
		t.Errorf("expected total >= 1, got %d", total)
	}

	// Find our test approach in results
	var found bool
	for _, app := range approaches {
		if app.ID == created.ID {
			found = true
			if app.ProblemTitle != "My Test Problem Title" {
				t.Errorf("expected problem_title = 'My Test Problem Title', got '%s'", app.ProblemTitle)
			}
			if app.AuthorType != models.AuthorTypeAgent {
				t.Errorf("expected author_type = 'agent', got '%s'", app.AuthorType)
			}
			if app.AuthorID != agentID {
				t.Errorf("expected author_id = '%s', got '%s'", agentID, app.AuthorID)
			}
			if app.Author.DisplayName != "Test Agent" {
				t.Errorf("expected display_name = 'Test Agent', got '%s'", app.Author.DisplayName)
			}
			if app.Angle != "Test angle" {
				t.Errorf("expected angle = 'Test angle', got '%s'", app.Angle)
			}
			break
		}
	}

	if !found {
		t.Error("expected to find test approach in ListByAuthor results")
	}
}

// TestListApproachesByAuthor_Empty tests that ListByAuthor returns empty for non-author.
func TestListApproachesByAuthor_Empty(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewApproachesRepository(pool)
	ctx := context.Background()

	approaches, total, err := repo.ListByAuthor(ctx, "agent", "nonexistent_author_xyz", 1, 20)
	if err != nil {
		t.Fatalf("ListByAuthor() error = %v", err)
	}

	if total != 0 {
		t.Errorf("expected total = 0, got %d", total)
	}

	if approaches == nil {
		t.Error("expected non-nil approaches slice")
	}

	if len(approaches) != 0 {
		t.Errorf("expected 0 approaches, got %d", len(approaches))
	}
}
