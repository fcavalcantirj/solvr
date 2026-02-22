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

func TestApproachesRepository_CreateApproach(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewApproachesRepository(pool)
	ctx := context.Background()

	// Create a test problem first
	timestamp := time.Now().Format("20060102150405")
	problemID := "approaches_create_problem_" + timestamp

	_, err := pool.Exec(ctx, `
		INSERT INTO posts (id, type, title, description, posted_by_type, posted_by_id, status)
		VALUES ($1, 'problem', 'Test Problem', 'Description', 'agent', 'test_agent', 'open')
	`, problemID)
	if err != nil {
		t.Fatalf("failed to insert test problem: %v", err)
	}

	// Cleanup
	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM approaches WHERE problem_id = $1", problemID)
		_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE id = $1", problemID)
	}()

	// Create approach
	approach := &models.Approach{
		ProblemID:   problemID,
		AuthorType:  models.AuthorTypeAgent,
		AuthorID:    "test_agent_" + timestamp,
		Angle:       "Test angle for the approach",
		Method:      "Test method",
		Assumptions: []string{"assumption1", "assumption2"},
		Status:      models.ApproachStatusStarting,
	}

	created, err := repo.CreateApproach(ctx, approach)
	if err != nil {
		t.Fatalf("CreateApproach() error = %v", err)
	}

	if created == nil {
		t.Fatal("expected non-nil approach")
	}

	if created.ID == "" {
		t.Error("expected ID to be set")
	}

	if created.ProblemID != problemID {
		t.Errorf("expected problem_id = %s, got %s", problemID, created.ProblemID)
	}

	if created.Angle != approach.Angle {
		t.Errorf("expected angle = %s, got %s", approach.Angle, created.Angle)
	}

	if created.Status != models.ApproachStatusStarting {
		t.Errorf("expected status = starting, got %s", created.Status)
	}

	if created.CreatedAt.IsZero() {
		t.Error("expected created_at to be set")
	}
}

func TestApproachesRepository_FindApproachByID(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewApproachesRepository(pool)
	ctx := context.Background()

	// Create test data
	timestamp := time.Now().Format("20060102150405")
	problemID := "approaches_find_problem_" + timestamp
	approachID := "approaches_find_approach_" + timestamp
	agentID := "test_agent_" + timestamp

	// Create problem
	_, err := pool.Exec(ctx, `
		INSERT INTO posts (id, type, title, description, posted_by_type, posted_by_id, status)
		VALUES ($1, 'problem', 'Test Problem', 'Description', 'agent', $2, 'open')
	`, problemID, agentID)
	if err != nil {
		t.Fatalf("failed to insert problem: %v", err)
	}

	// Create agent for author lookup
	_, err = pool.Exec(ctx, `
		INSERT INTO agents (id, display_name, api_key_hash, status)
		VALUES ($1, 'Test Agent', 'hash_test', 'active')
	`, agentID)
	if err != nil {
		t.Fatalf("failed to insert agent: %v", err)
	}

	// Create approach
	_, err = pool.Exec(ctx, `
		INSERT INTO approaches (id, problem_id, author_type, author_id, angle, method, status)
		VALUES ($1, $2, 'agent', $3, 'Test angle', 'Test method', 'starting')
	`, approachID, problemID, agentID)
	if err != nil {
		t.Fatalf("failed to insert approach: %v", err)
	}

	// Cleanup
	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM approaches WHERE id = $1", approachID)
		_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE id = $1", problemID)
		_, _ = pool.Exec(ctx, "DELETE FROM agents WHERE id = $1", agentID)
	}()

	// Find approach
	found, err := repo.FindApproachByID(ctx, approachID)
	if err != nil {
		t.Fatalf("FindApproachByID() error = %v", err)
	}

	if found == nil {
		t.Fatal("expected non-nil approach")
	}

	if found.ID != approachID {
		t.Errorf("expected ID = %s, got %s", approachID, found.ID)
	}

	if found.Author.DisplayName != "Test Agent" {
		t.Errorf("expected author display_name = 'Test Agent', got '%s'", found.Author.DisplayName)
	}
}

func TestApproachesRepository_ListApproaches(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewApproachesRepository(pool)
	ctx := context.Background()

	// Create test data
	timestamp := time.Now().Format("20060102150405")
	problemID := "approaches_list_problem_" + timestamp
	agentID := "test_agent_" + timestamp

	// Create problem
	_, err := pool.Exec(ctx, `
		INSERT INTO posts (id, type, title, description, posted_by_type, posted_by_id, status)
		VALUES ($1, 'problem', 'Test Problem', 'Description', 'agent', $2, 'open')
	`, problemID, agentID)
	if err != nil {
		t.Fatalf("failed to insert problem: %v", err)
	}

	// Create agent
	_, err = pool.Exec(ctx, `
		INSERT INTO agents (id, display_name, api_key_hash, status)
		VALUES ($1, 'Test Agent', 'hash_test', 'active')
	`, agentID)
	if err != nil {
		t.Fatalf("failed to insert agent: %v", err)
	}

	// Create 3 approaches
	approachIDs := []string{
		"approaches_list_1_" + timestamp,
		"approaches_list_2_" + timestamp,
		"approaches_list_3_" + timestamp,
	}

	for i, id := range approachIDs {
		_, err := pool.Exec(ctx, `
			INSERT INTO approaches (id, problem_id, author_type, author_id, angle, status)
			VALUES ($1, $2, 'agent', $3, $4, 'starting')
		`, id, problemID, agentID, "Angle "+string(rune('A'+i)))
		if err != nil {
			t.Fatalf("failed to insert approach: %v", err)
		}
	}

	// Cleanup
	defer func() {
		for _, id := range approachIDs {
			_, _ = pool.Exec(ctx, "DELETE FROM approaches WHERE id = $1", id)
		}
		_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE id = $1", problemID)
		_, _ = pool.Exec(ctx, "DELETE FROM agents WHERE id = $1", agentID)
	}()

	// List approaches
	opts := models.ApproachListOptions{
		ProblemID: problemID,
		Page:      1,
		PerPage:   10,
	}

	approaches, total, err := repo.ListApproaches(ctx, problemID, opts)
	if err != nil {
		t.Fatalf("ListApproaches() error = %v", err)
	}

	if total != 3 {
		t.Errorf("expected total = 3, got %d", total)
	}

	if len(approaches) != 3 {
		t.Errorf("expected 3 approaches, got %d", len(approaches))
	}

	// Verify author info is populated
	for _, a := range approaches {
		if a.Author.DisplayName == "" {
			t.Error("expected author display_name to be set")
		}
	}
}

func TestApproachesRepository_UpdateApproach(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewApproachesRepository(pool)
	ctx := context.Background()

	// Create test data
	timestamp := time.Now().Format("20060102150405")
	problemID := "approaches_update_problem_" + timestamp
	approachID := "approaches_update_approach_" + timestamp

	// Create problem
	_, err := pool.Exec(ctx, `
		INSERT INTO posts (id, type, title, description, posted_by_type, posted_by_id, status)
		VALUES ($1, 'problem', 'Test Problem', 'Description', 'agent', 'test_agent', 'open')
	`, problemID)
	if err != nil {
		t.Fatalf("failed to insert problem: %v", err)
	}

	// Create approach
	_, err = pool.Exec(ctx, `
		INSERT INTO approaches (id, problem_id, author_type, author_id, angle, status)
		VALUES ($1, $2, 'agent', 'test_agent', 'Initial angle', 'starting')
	`, approachID, problemID)
	if err != nil {
		t.Fatalf("failed to insert approach: %v", err)
	}

	// Cleanup
	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM approaches WHERE id = $1", approachID)
		_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE id = $1", problemID)
	}()

	// Update approach
	approach := &models.Approach{
		ID:      approachID,
		Status:  models.ApproachStatusWorking,
		Outcome: "Work in progress",
	}

	updated, err := repo.UpdateApproach(ctx, approach)
	if err != nil {
		t.Fatalf("UpdateApproach() error = %v", err)
	}

	if updated.Status != models.ApproachStatusWorking {
		t.Errorf("expected status = working, got %s", updated.Status)
	}

	if updated.Outcome != "Work in progress" {
		t.Errorf("expected outcome = 'Work in progress', got '%s'", updated.Outcome)
	}
}

func TestApproachesRepository_NotFound(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewApproachesRepository(pool)
	ctx := context.Background()

	// Try to find non-existent approach
	_, err := repo.FindApproachByID(ctx, "00000000-0000-0000-0000-000000000000")
	if err == nil {
		t.Error("expected error for non-existent approach")
	}

	if err != ErrApproachNotFound {
		t.Errorf("expected ErrApproachNotFound, got %v", err)
	}
}

func TestApproachesRepository_AddProgressNote(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewApproachesRepository(pool)
	ctx := context.Background()

	// Create test data
	timestamp := time.Now().Format("20060102150405")
	problemID := "progress_note_problem_" + timestamp
	approachID := "progress_note_approach_" + timestamp

	// Create problem
	_, err := pool.Exec(ctx, `
		INSERT INTO posts (id, type, title, description, posted_by_type, posted_by_id, status)
		VALUES ($1, 'problem', 'Test Problem', 'Description', 'agent', 'test_agent', 'open')
	`, problemID)
	if err != nil {
		t.Fatalf("failed to insert problem: %v", err)
	}

	// Create approach
	_, err = pool.Exec(ctx, `
		INSERT INTO approaches (id, problem_id, author_type, author_id, angle, status)
		VALUES ($1, $2, 'agent', 'test_agent', 'Test angle', 'working')
	`, approachID, problemID)
	if err != nil {
		t.Fatalf("failed to insert approach: %v", err)
	}

	// Cleanup
	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM progress_notes WHERE approach_id = $1", approachID)
		_, _ = pool.Exec(ctx, "DELETE FROM approaches WHERE id = $1", approachID)
		_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE id = $1", problemID)
	}()

	// Add progress note
	note := &models.ProgressNote{
		ApproachID: approachID,
		Content:    "Made some progress on the solution",
	}

	created, err := repo.AddProgressNote(ctx, note)
	if err != nil {
		t.Fatalf("AddProgressNote() error = %v", err)
	}

	if created.ID == "" {
		t.Error("expected ID to be set")
	}

	if created.Content != note.Content {
		t.Errorf("expected content = '%s', got '%s'", note.Content, created.Content)
	}
}

func TestApproachesRepository_GetProgressNotes(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewApproachesRepository(pool)
	ctx := context.Background()

	// Create test data
	timestamp := time.Now().Format("20060102150405")
	problemID := "get_notes_problem_" + timestamp
	approachID := "get_notes_approach_" + timestamp

	// Create problem
	_, err := pool.Exec(ctx, `
		INSERT INTO posts (id, type, title, description, posted_by_type, posted_by_id, status)
		VALUES ($1, 'problem', 'Test Problem', 'Description', 'agent', 'test_agent', 'open')
	`, problemID)
	if err != nil {
		t.Fatalf("failed to insert problem: %v", err)
	}

	// Create approach
	_, err = pool.Exec(ctx, `
		INSERT INTO approaches (id, problem_id, author_type, author_id, angle, status)
		VALUES ($1, $2, 'agent', 'test_agent', 'Test angle', 'working')
	`, approachID, problemID)
	if err != nil {
		t.Fatalf("failed to insert approach: %v", err)
	}

	// Add progress notes
	noteIDs := []string{
		"note_1_" + timestamp,
		"note_2_" + timestamp,
	}

	for i, id := range noteIDs {
		_, err := pool.Exec(ctx, `
			INSERT INTO progress_notes (id, approach_id, content)
			VALUES ($1, $2, $3)
		`, id, approachID, "Progress note "+string(rune('A'+i)))
		if err != nil {
			t.Fatalf("failed to insert note: %v", err)
		}
	}

	// Cleanup
	defer func() {
		for _, id := range noteIDs {
			_, _ = pool.Exec(ctx, "DELETE FROM progress_notes WHERE id = $1", id)
		}
		_, _ = pool.Exec(ctx, "DELETE FROM approaches WHERE id = $1", approachID)
		_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE id = $1", problemID)
	}()

	// Get progress notes
	notes, err := repo.GetProgressNotes(ctx, approachID)
	if err != nil {
		t.Fatalf("GetProgressNotes() error = %v", err)
	}

	if len(notes) != 2 {
		t.Errorf("expected 2 notes, got %d", len(notes))
	}
}

func TestApproachesRepository_FindApproachByID_ReturnsIsLatest(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewApproachesRepository(pool)
	ctx := context.Background()

	problemID := uuid.New().String()
	agentID := "islat_find_agent_" + time.Now().Format("150405")

	_, err := pool.Exec(ctx, `
		INSERT INTO posts (id, type, title, description, posted_by_type, posted_by_id, status)
		VALUES ($1, 'problem', 'IsLatest Test', 'Desc', 'agent', $2, 'open')
	`, problemID, agentID)
	if err != nil {
		t.Fatalf("insert problem: %v", err)
	}

	_, err = pool.Exec(ctx, `
		INSERT INTO agents (id, display_name, api_key_hash, status)
		VALUES ($1, 'IsLatest Agent', 'hash_islat', 'active')
	`, agentID)
	if err != nil {
		t.Fatalf("insert agent: %v", err)
	}

	approach := &models.Approach{
		ProblemID:  problemID,
		AuthorType: models.AuthorTypeAgent,
		AuthorID:   agentID,
		Angle:      "IsLatest test angle",
		Status:     models.ApproachStatusStarting,
	}

	created, err := repo.CreateApproach(ctx, approach)
	if err != nil {
		t.Fatalf("CreateApproach() error = %v", err)
	}

	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM approaches WHERE id = $1", created.ID)
		_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE id = $1", problemID)
		_, _ = pool.Exec(ctx, "DELETE FROM agents WHERE id = $1", agentID)
	}()

	found, err := repo.FindApproachByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("FindApproachByID() error = %v", err)
	}

	// is_latest defaults to TRUE in DB; must be returned correctly
	if !found.IsLatest {
		t.Error("expected IsLatest == true (DB default), got false — missing column in SELECT")
	}
}

func TestApproachesRepository_ListApproaches_ReturnsIsLatest(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewApproachesRepository(pool)
	ctx := context.Background()

	problemID := uuid.New().String()
	agentID := "islat_list_agent_" + time.Now().Format("150405")

	_, err := pool.Exec(ctx, `
		INSERT INTO posts (id, type, title, description, posted_by_type, posted_by_id, status)
		VALUES ($1, 'problem', 'IsLatest List Test', 'Desc', 'agent', $2, 'open')
	`, problemID, agentID)
	if err != nil {
		t.Fatalf("insert problem: %v", err)
	}

	_, err = pool.Exec(ctx, `
		INSERT INTO agents (id, display_name, api_key_hash, status)
		VALUES ($1, 'IsLatest List Agent', 'hash_islat_list', 'active')
	`, agentID)
	if err != nil {
		t.Fatalf("insert agent: %v", err)
	}

	approach := &models.Approach{
		ProblemID:  problemID,
		AuthorType: models.AuthorTypeAgent,
		AuthorID:   agentID,
		Angle:      "IsLatest list test angle",
		Status:     models.ApproachStatusStarting,
	}

	created, err := repo.CreateApproach(ctx, approach)
	if err != nil {
		t.Fatalf("CreateApproach() error = %v", err)
	}

	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM approaches WHERE id = $1", created.ID)
		_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE id = $1", problemID)
		_, _ = pool.Exec(ctx, "DELETE FROM agents WHERE id = $1", agentID)
	}()

	opts := models.ApproachListOptions{ProblemID: problemID, Page: 1, PerPage: 10}
	approaches, _, err := repo.ListApproaches(ctx, problemID, opts)
	if err != nil {
		t.Fatalf("ListApproaches() error = %v", err)
	}

	if len(approaches) == 0 {
		t.Fatal("expected at least one approach")
	}

	// is_latest defaults to TRUE in DB; must be returned correctly
	if !approaches[0].IsLatest {
		t.Error("expected IsLatest == true (DB default), got false — missing column in SELECT")
	}
}

// TestApproachesPersistInDatabase verifies approaches are stored in database, not in-memory.
// This catches bugs where in-memory repositories are used instead of database.
func TestApproachesPersistInDatabase(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewApproachesRepository(pool)
	ctx := context.Background()

	// Create test data
	timestamp := time.Now().Format("20060102150405")
	problemID := "persist_check_problem_" + timestamp

	// Create problem
	_, err := pool.Exec(ctx, `
		INSERT INTO posts (id, type, title, description, posted_by_type, posted_by_id, status)
		VALUES ($1, 'problem', 'Test Problem', 'Description', 'agent', 'test_agent', 'open')
	`, problemID)
	if err != nil {
		t.Fatalf("failed to insert problem: %v", err)
	}

	// Cleanup
	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM approaches WHERE problem_id = $1", problemID)
		_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE id = $1", problemID)
	}()

	// Create approach via repository
	approach := &models.Approach{
		ProblemID:  problemID,
		AuthorType: models.AuthorTypeAgent,
		AuthorID:   "test_agent",
		Angle:      "Persistence test angle",
		Status:     models.ApproachStatusStarting,
	}

	created, err := repo.CreateApproach(ctx, approach)
	if err != nil {
		t.Fatalf("CreateApproach() error = %v", err)
	}

	// Verify DIRECTLY in database (not via repository)
	// This catches in-memory implementations that would pass API tests
	var count int
	err = pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM approaches WHERE id = $1
	`, created.ID).Scan(&count)
	if err != nil {
		t.Fatalf("direct DB query error = %v", err)
	}

	if count != 1 {
		t.Errorf("approach not persisted to database: expected count=1, got %d", count)
	}
}
