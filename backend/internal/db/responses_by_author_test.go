// Package db provides database access for Solvr.
package db

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// TestListResponsesByAuthor tests that ListByAuthor returns responses for a specific author
// with idea title context.
func TestListResponsesByAuthor(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewResponsesRepository(pool)
	ctx := context.Background()

	timestamp := time.Now().Format("20060102150405")
	agentID := "resp_by_author_agent_" + timestamp
	ideaID := "resp_by_author_idea_" + timestamp

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

	// Create test idea
	_, err = pool.Exec(ctx, `
		INSERT INTO posts (id, type, title, description, posted_by_type, posted_by_id, status)
		VALUES ($1, 'idea', 'My Test Idea Title', 'Description', 'agent', $2, 'open')
	`, ideaID, agentID)
	if err != nil {
		t.Fatalf("failed to insert idea: %v", err)
	}

	// Create response by the agent
	response := &models.Response{
		IdeaID:       ideaID,
		AuthorType:   models.AuthorTypeAgent,
		AuthorID:     agentID,
		Content:      "Test response content",
		ResponseType: models.ResponseTypeBuild,
	}
	created, err := repo.CreateResponse(ctx, response)
	if err != nil {
		t.Fatalf("failed to create response: %v", err)
	}

	defer func() {
		if created != nil {
			_, _ = pool.Exec(ctx, "DELETE FROM responses WHERE id = $1", created.ID)
		}
		_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE id = $1", ideaID)
		_, _ = pool.Exec(ctx, "DELETE FROM agents WHERE id = $1", agentID)
	}()

	// Test ListByAuthor
	responses, total, err := repo.ListByAuthor(ctx, string(models.AuthorTypeAgent), agentID, 1, 20)
	if err != nil {
		t.Fatalf("ListByAuthor() error = %v", err)
	}

	if total < 1 {
		t.Errorf("expected total >= 1, got %d", total)
	}

	// Find our test response in results
	var found bool
	for _, resp := range responses {
		if resp.ID == created.ID {
			found = true
			if resp.IdeaTitle != "My Test Idea Title" {
				t.Errorf("expected idea_title = 'My Test Idea Title', got '%s'", resp.IdeaTitle)
			}
			if resp.AuthorType != models.AuthorTypeAgent {
				t.Errorf("expected author_type = 'agent', got '%s'", resp.AuthorType)
			}
			if resp.AuthorID != agentID {
				t.Errorf("expected author_id = '%s', got '%s'", agentID, resp.AuthorID)
			}
			if resp.Author.DisplayName != "Test Agent" {
				t.Errorf("expected display_name = 'Test Agent', got '%s'", resp.Author.DisplayName)
			}
			if resp.ResponseType != models.ResponseTypeBuild {
				t.Errorf("expected response_type = 'build', got '%s'", resp.ResponseType)
			}
			break
		}
	}

	if !found {
		t.Error("expected to find test response in ListByAuthor results")
	}
}

// TestListResponsesByAuthor_Empty tests that ListByAuthor returns empty for non-author.
func TestListResponsesByAuthor_Empty(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewResponsesRepository(pool)
	ctx := context.Background()

	responses, total, err := repo.ListByAuthor(ctx, "agent", "nonexistent_author_xyz", 1, 20)
	if err != nil {
		t.Fatalf("ListByAuthor() error = %v", err)
	}

	if total != 0 {
		t.Errorf("expected total = 0, got %d", total)
	}

	if responses == nil {
		t.Error("expected non-nil responses slice")
	}

	if len(responses) != 0 {
		t.Errorf("expected 0 responses, got %d", len(responses))
	}
}
