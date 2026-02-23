// Package db provides database access for Solvr.
package db

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// TestListAnswersByAuthor tests that ListByAuthor returns answers for a specific author
// with question title context.
func TestListAnswersByAuthor(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewAnswersRepository(pool)
	ctx := context.Background()

	timestamp := time.Now().Format("20060102150405")
	agentID := "ans_by_author_agent_" + timestamp

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

	// Create test question
	var questionID string
	err = pool.QueryRow(ctx, `
		INSERT INTO posts (type, title, description, posted_by_type, posted_by_id, status)
		VALUES ('question', 'My Test Question Title', 'Description', 'agent', $1, 'open')
		RETURNING id::text
	`, agentID).Scan(&questionID)
	if err != nil {
		t.Fatalf("failed to insert question: %v", err)
	}

	// Create answer by the agent
	var answerID string
	err = pool.QueryRow(ctx, `
		INSERT INTO answers (question_id, author_type, author_id, content)
		VALUES ($1, 'agent', $2, 'Agent answer content')
		RETURNING id::text
	`, questionID, agentID).Scan(&answerID)
	if err != nil {
		if strings.Contains(err.Error(), "does not exist") {
			t.Skip("answers table does not exist, skipping")
		}
		t.Fatalf("failed to insert answer: %v", err)
	}

	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM answers WHERE id = $1", answerID)
		_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE id = $1", questionID)
		_, _ = pool.Exec(ctx, "DELETE FROM agents WHERE id = $1", agentID)
	}()

	// Test ListByAuthor
	answers, total, err := repo.ListByAuthor(ctx, string(models.AuthorTypeAgent), agentID, 1, 20)
	if err != nil {
		t.Fatalf("ListByAuthor() error = %v", err)
	}

	if total < 1 {
		t.Errorf("expected total >= 1, got %d", total)
	}

	// Find our test answer in the results
	var found bool
	for _, ans := range answers {
		if ans.ID == answerID {
			found = true
			if ans.QuestionTitle != "My Test Question Title" {
				t.Errorf("expected question_title = 'My Test Question Title', got '%s'", ans.QuestionTitle)
			}
			if ans.AuthorType != models.AuthorTypeAgent {
				t.Errorf("expected author_type = 'agent', got '%s'", ans.AuthorType)
			}
			if ans.AuthorID != agentID {
				t.Errorf("expected author_id = '%s', got '%s'", agentID, ans.AuthorID)
			}
			if ans.Author.DisplayName != "Test Agent" {
				t.Errorf("expected display_name = 'Test Agent', got '%s'", ans.Author.DisplayName)
			}
			break
		}
	}

	if !found {
		t.Error("expected to find test answer in ListByAuthor results")
	}
}

// TestListAnswersByAuthor_Empty tests that ListByAuthor returns empty for non-author.
func TestListAnswersByAuthor_Empty(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewAnswersRepository(pool)
	ctx := context.Background()

	// Query for a non-existent author
	answers, total, err := repo.ListByAuthor(ctx, "agent", "nonexistent_author_xyz", 1, 20)
	if err != nil {
		t.Fatalf("ListByAuthor() error = %v", err)
	}

	if total != 0 {
		t.Errorf("expected total = 0, got %d", total)
	}

	if answers == nil {
		t.Error("expected non-nil answers slice")
	}

	if len(answers) != 0 {
		t.Errorf("expected 0 answers, got %d", len(answers))
	}
}

// TestListAnswersByAuthor_Pagination tests pagination of ListByAuthor.
func TestListAnswersByAuthor_Pagination(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewAnswersRepository(pool)
	ctx := context.Background()

	// Test that page/perPage validation works (doesn't error on edge values)
	answers, _, err := repo.ListByAuthor(ctx, "agent", "nonexistent", 0, 0)
	if err != nil {
		t.Fatalf("ListByAuthor() with zero page/perPage error = %v", err)
	}
	if answers == nil {
		t.Error("expected non-nil answers slice")
	}

	// Test perPage cap at 50
	answers, _, err = repo.ListByAuthor(ctx, "agent", "nonexistent", 1, 100)
	if err != nil {
		t.Fatalf("ListByAuthor() with large perPage error = %v", err)
	}
	if answers == nil {
		t.Error("expected non-nil answers slice")
	}
}
