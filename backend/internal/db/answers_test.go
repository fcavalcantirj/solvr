// Package db provides database access for Solvr.
package db

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// Note: These tests require a running PostgreSQL database.
// Set DATABASE_URL environment variable to run integration tests.
// Tests will be skipped if DATABASE_URL is not set.

// TestListAnswersWithAgentAuthor tests that ListAnswers correctly
// retrieves agent author information including display_name.
// This is the TDD RED test for the bug where agent names don't show on question answers.
func TestListAnswersWithAgentAuthor(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewAnswersRepository(pool)
	ctx := context.Background()

	now := time.Now()
	ns := fmt.Sprintf("%04d", now.Nanosecond()/100000)
	timestamp := now.Format("20060102150405")
	agentID := "aaa_" + now.Format("150405") + ns
	displayName := "Ans Agent " + now.Format("150405") + ns

	// Create test agent with display_name
	_, err := pool.Exec(ctx, `
		INSERT INTO agents (id, display_name, api_key_hash, status)
		VALUES ($1, $2, $3, 'active')
	`, agentID, displayName, "hash_"+timestamp)
	if err != nil {
		// If agents table doesn't exist, skip
		if strings.Contains(err.Error(), "does not exist") {
			t.Skip("agents table does not exist, skipping")
		}
		t.Fatalf("failed to insert agent: %v", err)
	}

	// Create test question (stored in posts table with type='question')
	var questionID string
	err = pool.QueryRow(ctx, `
		INSERT INTO posts (type, title, description, posted_by_type, posted_by_id, status)
		VALUES ('question', 'Test Question', 'Description', 'agent', $1, 'open')
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
		// If answers table doesn't exist, skip
		if strings.Contains(err.Error(), "does not exist") {
			t.Skip("answers table does not exist, skipping")
		}
		t.Fatalf("failed to insert answer: %v", err)
	}

	// Cleanup
	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM answers WHERE id = $1", answerID)
		_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE id = $1", questionID)
		_, _ = pool.Exec(ctx, "DELETE FROM agents WHERE id = $1", agentID)
	}()

	// Test ListAnswers - this is the key test for the bug
	opts := models.AnswerListOptions{
		QuestionID: questionID,
		Page:       1,
		PerPage:    10,
	}

	answers, total, err := repo.ListAnswers(ctx, questionID, opts)
	if err != nil {
		t.Fatalf("ListAnswers() error = %v", err)
	}

	if total != 1 {
		t.Errorf("expected total = 1, got %d", total)
	}

	if len(answers) != 1 {
		t.Fatalf("expected 1 answer, got %d", len(answers))
	}

	// THIS IS THE KEY ASSERTION - agent's display_name must be retrieved correctly
	ans := answers[0]
	if ans.Author.DisplayName != displayName {
		t.Errorf("expected author display_name = '%s', got '%s'", displayName, ans.Author.DisplayName)
	}

	if ans.Author.Type != "agent" {
		t.Errorf("expected author type = 'agent', got '%s'", ans.Author.Type)
	}

	if ans.Author.ID != agentID {
		t.Errorf("expected author ID = '%s', got '%s'", agentID, ans.Author.ID)
	}
}

// TestListAnswersWithHumanAuthor tests that ListAnswers correctly
// retrieves human author information including display_name.
func TestListAnswersWithHumanAuthor(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewAnswersRepository(pool)
	ctx := context.Background()

	timestamp := time.Now().Format("20060102150405")
	answerID := "answers_human_author_ans_" + timestamp

	// Create test user with display_name
	userID := "550e8400-e29b-41d4-a716-446655440099" // Valid UUID format
	_, err := pool.Exec(ctx, `
		INSERT INTO users (id, email, display_name, github_id)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (id) DO UPDATE SET display_name = EXCLUDED.display_name
	`, userID, "testuser_"+timestamp+"@example.com", "Test Human Name", 123456789+len(timestamp))
	if err != nil {
		// If users table doesn't exist, skip
		if strings.Contains(err.Error(), "does not exist") {
			t.Skip("users table does not exist, skipping")
		}
		t.Fatalf("failed to insert user: %v", err)
	}

	// Create test question
	var questionID string
	err = pool.QueryRow(ctx, `
		INSERT INTO posts (type, title, description, posted_by_type, posted_by_id, status)
		VALUES ('question', 'Test Question', 'Description', 'human', $1, 'open')
		RETURNING id::text
	`, userID).Scan(&questionID)
	if err != nil {
		t.Fatalf("failed to insert question: %v", err)
	}

	// Create answer by the human
	_, err = pool.Exec(ctx, `
		INSERT INTO answers (id, question_id, author_type, author_id, content)
		VALUES ($1, $2, 'human', $3, 'Human answer content')
	`, answerID, questionID, userID)
	if err != nil {
		t.Fatalf("failed to insert answer: %v", err)
	}

	// Cleanup
	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM answers WHERE id = $1", answerID)
		_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE id = $1", questionID)
		_, _ = pool.Exec(ctx, "DELETE FROM users WHERE id = $1", userID)
	}()

	// Test ListAnswers
	opts := models.AnswerListOptions{
		QuestionID: questionID,
		Page:       1,
		PerPage:    10,
	}

	answers, total, err := repo.ListAnswers(ctx, questionID, opts)
	if err != nil {
		t.Fatalf("ListAnswers() error = %v", err)
	}

	if total != 1 {
		t.Errorf("expected total = 1, got %d", total)
	}

	if len(answers) != 1 {
		t.Fatalf("expected 1 answer, got %d", len(answers))
	}

	// Verify human's display_name was retrieved correctly
	ans := answers[0]
	if ans.Author.DisplayName != "Test Human Name" {
		t.Errorf("expected author display_name = 'Test Human Name', got '%s'", ans.Author.DisplayName)
	}

	if ans.Author.Type != "human" {
		t.Errorf("expected author type = 'human', got '%s'", ans.Author.Type)
	}

	if ans.Author.ID != userID {
		t.Errorf("expected author ID = '%s', got '%s'", userID, ans.Author.ID)
	}
}

// TestAnswersRepository_ListAnswers_Empty tests empty results.
func TestAnswersRepository_ListAnswers_Empty(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewAnswersRepository(pool)
	ctx := context.Background()

	// Create a test question to reference
	var questionID string
	err := pool.QueryRow(ctx, `
		INSERT INTO posts (type, title, description, posted_by_type, posted_by_id, status)
		VALUES ('question', 'Test Question', 'Test description', 'agent', 'test_agent', 'open')
		RETURNING id::text
	`).Scan(&questionID)
	if err != nil {
		t.Fatalf("failed to insert test question: %v", err)
	}
	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE id = $1", questionID)
	}()

	// List answers when none exist
	opts := models.AnswerListOptions{
		QuestionID: questionID,
		Page:       1,
		PerPage:    10,
	}

	answers, total, err := repo.ListAnswers(ctx, questionID, opts)
	if err != nil {
		t.Fatalf("ListAnswers() error = %v", err)
	}

	// Should return empty list, not nil
	if answers == nil {
		t.Error("expected non-nil answers slice")
	}

	if len(answers) != 0 {
		t.Errorf("expected 0 answers, got %d", len(answers))
	}

	if total != 0 {
		t.Errorf("expected total = 0, got %d", total)
	}
}

// TestAnswersRepository_CreateAnswer_Success tests answer creation.
func TestAnswersRepository_CreateAnswer_Success(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewAnswersRepository(pool)
	ctx := context.Background()

	// Create a test question
	timestamp := time.Now().Format("20060102150405")
	var questionID string
	err := pool.QueryRow(ctx, `
		INSERT INTO posts (type, title, description, posted_by_type, posted_by_id, status)
		VALUES ('question', 'Test Question', 'Description', 'agent', 'test_agent', 'open')
		RETURNING id::text
	`).Scan(&questionID)
	if err != nil {
		t.Fatalf("failed to insert question: %v", err)
	}

	// Create answer
	answer := &models.Answer{
		QuestionID: questionID,
		AuthorType: models.AuthorTypeAgent,
		AuthorID:   "test_agent_" + timestamp,
		Content:    "This is a test answer with helpful information.",
	}

	created, err := repo.CreateAnswer(ctx, answer)

	// Clean up
	defer func() {
		if created != nil && created.ID != "" {
			_, _ = pool.Exec(ctx, "DELETE FROM answers WHERE id = $1", created.ID)
		}
		_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE id = $1", questionID)
	}()

	if err != nil {
		t.Fatalf("CreateAnswer() error = %v", err)
	}

	if created == nil {
		t.Fatal("expected non-nil answer")
	}

	if created.ID == "" {
		t.Error("expected ID to be set")
	}

	if created.QuestionID != questionID {
		t.Errorf("expected question_id = %s, got %s", questionID, created.QuestionID)
	}

	if created.AuthorType != models.AuthorTypeAgent {
		t.Errorf("expected author_type = agent, got %s", created.AuthorType)
	}

	if created.Content != answer.Content {
		t.Errorf("expected content = %s, got %s", answer.Content, created.Content)
	}

	if created.Upvotes != 0 {
		t.Errorf("expected upvotes = 0, got %d", created.Upvotes)
	}

	if created.Downvotes != 0 {
		t.Errorf("expected downvotes = 0, got %d", created.Downvotes)
	}

	if created.CreatedAt.IsZero() {
		t.Error("expected created_at to be set")
	}
}

// TestCreateAnswer_SetsQuestionStatusToAnswered tests that creating an answer
// on an 'open' question updates its status to 'answered'.
func TestCreateAnswer_SetsQuestionStatusToAnswered(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewAnswersRepository(pool)
	ctx := context.Background()

	timestamp := time.Now().Format("20060102150405")

	// Create a question with status 'open'
	var questionID string
	err := pool.QueryRow(ctx, `
		INSERT INTO posts (type, title, description, posted_by_type, posted_by_id, status)
		VALUES ('question', 'Test Question', 'Description', 'agent', 'test_agent', 'open')
		RETURNING id::text
	`).Scan(&questionID)
	if err != nil {
		t.Fatalf("failed to insert question: %v", err)
	}

	// Create first answer
	answer := &models.Answer{
		QuestionID: questionID,
		AuthorType: models.AuthorTypeAgent,
		AuthorID:   "test_agent_" + timestamp,
		Content:    "This is a helpful answer.",
	}

	created, err := repo.CreateAnswer(ctx, answer)

	defer func() {
		if created != nil && created.ID != "" {
			_, _ = pool.Exec(ctx, "DELETE FROM answers WHERE id = $1", created.ID)
		}
		_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE id = $1", questionID)
	}()

	if err != nil {
		t.Fatalf("CreateAnswer() error = %v", err)
	}

	// Verify question status changed from 'open' to 'answered'
	var status string
	err = pool.QueryRow(ctx, "SELECT status FROM posts WHERE id = $1", questionID).Scan(&status)
	if err != nil {
		t.Fatalf("failed to query question status: %v", err)
	}

	if status != "answered" {
		t.Errorf("expected question status = 'answered', got '%s'", status)
	}
}

// TestCreateAnswer_DoesNotOverwriteSolvedStatus tests that creating an answer
// on a 'solved' question does NOT change its status back to 'answered'.
func TestCreateAnswer_DoesNotOverwriteSolvedStatus(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewAnswersRepository(pool)
	ctx := context.Background()

	timestamp := time.Now().Format("20060102150405")

	// Create a question with status 'solved' (already has accepted answer)
	var questionID string
	err := pool.QueryRow(ctx, `
		INSERT INTO posts (type, title, description, posted_by_type, posted_by_id, status)
		VALUES ('question', 'Test Question', 'Description', 'agent', 'test_agent', 'solved')
		RETURNING id::text
	`).Scan(&questionID)
	if err != nil {
		t.Fatalf("failed to insert question: %v", err)
	}

	// Create another answer on the already-solved question
	answer := &models.Answer{
		QuestionID: questionID,
		AuthorType: models.AuthorTypeAgent,
		AuthorID:   "test_agent_" + timestamp,
		Content:    "Another answer on solved question.",
	}

	created, err := repo.CreateAnswer(ctx, answer)

	defer func() {
		if created != nil && created.ID != "" {
			_, _ = pool.Exec(ctx, "DELETE FROM answers WHERE id = $1", created.ID)
		}
		_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE id = $1", questionID)
	}()

	if err != nil {
		t.Fatalf("CreateAnswer() error = %v", err)
	}

	// Verify question status is still 'solved' (not overwritten)
	var status string
	err = pool.QueryRow(ctx, "SELECT status FROM posts WHERE id = $1", questionID).Scan(&status)
	if err != nil {
		t.Fatalf("failed to query question status: %v", err)
	}

	if status != "solved" {
		t.Errorf("expected question status = 'solved', got '%s'", status)
	}
}
