package db

import (
	"context"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// Note: These tests require a running PostgreSQL database with migration 000048 applied.
// Set DATABASE_URL environment variable to run integration tests.

func TestApproachRelationships_CreateRelationship(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewApproachesRepository(pool)
	relRepo := NewApproachRelationshipsRepository(pool)
	ctx := context.Background()

	// Create test problem and two approaches
	ts := time.Now().Format("150405")
	problemID := "rel_create_problem_" + ts

	_, err := pool.Exec(ctx, `
		INSERT INTO posts (id, type, title, description, posted_by_type, posted_by_id, status)
		VALUES ($1, 'problem', 'Test Problem', 'Description for test', 'agent', 'test_agent', 'open')
	`, problemID)
	if err != nil {
		t.Fatalf("failed to insert test problem: %v", err)
	}

	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM approach_relationships WHERE from_approach_id IN (SELECT id FROM approaches WHERE problem_id = $1)", problemID)
		_, _ = pool.Exec(ctx, "DELETE FROM approaches WHERE problem_id = $1", problemID)
		_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE id = $1", problemID)
	}()

	parentApproach, err := repo.CreateApproach(ctx, &models.Approach{
		ProblemID:  problemID,
		AuthorType: models.AuthorTypeAgent,
		AuthorID:   "agent_" + ts,
		Angle:      "Original approach angle",
		Method:     "Original method",
		Status:     models.ApproachStatusWorking,
	})
	if err != nil {
		t.Fatalf("CreateApproach(parent) error = %v", err)
	}

	childApproach, err := repo.CreateApproach(ctx, &models.Approach{
		ProblemID:  problemID,
		AuthorType: models.AuthorTypeAgent,
		AuthorID:   "agent_" + ts,
		Angle:      "Updated approach angle",
		Method:     "Better method",
		Status:     models.ApproachStatusStarting,
	})
	if err != nil {
		t.Fatalf("CreateApproach(child) error = %v", err)
	}

	// Create relationship: child "updates" parent
	rel, err := relRepo.CreateRelationship(ctx, &models.ApproachRelationship{
		FromApproachID: childApproach.ID,
		ToApproachID:   parentApproach.ID,
		RelationType:   models.RelationTypeUpdates,
	})
	if err != nil {
		t.Fatalf("CreateRelationship() error = %v", err)
	}

	if rel.ID == "" {
		t.Error("expected relationship ID to be set")
	}
	if rel.RelationType != models.RelationTypeUpdates {
		t.Errorf("expected relation_type = updates, got %s", rel.RelationType)
	}
	if rel.CreatedAt.IsZero() {
		t.Error("expected created_at to be set")
	}

	// Verify parent approach is_latest was set to false
	var isLatest bool
	err = pool.QueryRow(ctx, "SELECT is_latest FROM approaches WHERE id = $1", parentApproach.ID).Scan(&isLatest)
	if err != nil {
		t.Fatalf("failed to query parent is_latest: %v", err)
	}
	if isLatest {
		t.Error("expected parent is_latest = false after being superseded")
	}

	// Verify child is still is_latest = true
	err = pool.QueryRow(ctx, "SELECT is_latest FROM approaches WHERE id = $1", childApproach.ID).Scan(&isLatest)
	if err != nil {
		t.Fatalf("failed to query child is_latest: %v", err)
	}
	if !isLatest {
		t.Error("expected child is_latest = true")
	}
}

func TestApproachRelationships_GetVersionChain(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewApproachesRepository(pool)
	relRepo := NewApproachRelationshipsRepository(pool)
	ctx := context.Background()

	ts := time.Now().Format("150405")
	problemID := "rel_chain_problem_" + ts

	_, err := pool.Exec(ctx, `
		INSERT INTO posts (id, type, title, description, posted_by_type, posted_by_id, status)
		VALUES ($1, 'problem', 'Chain Test', 'Description for chain test', 'agent', 'test_agent', 'open')
	`, problemID)
	if err != nil {
		t.Fatalf("failed to insert test problem: %v", err)
	}

	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM approach_relationships WHERE from_approach_id IN (SELECT id FROM approaches WHERE problem_id = $1)", problemID)
		_, _ = pool.Exec(ctx, "DELETE FROM approach_relationships WHERE to_approach_id IN (SELECT id FROM approaches WHERE problem_id = $1)", problemID)
		_, _ = pool.Exec(ctx, "DELETE FROM approaches WHERE problem_id = $1", problemID)
		_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE id = $1", problemID)
	}()

	agentID := "agent_" + ts

	// Create chain: v1 -> v2 -> v3 (v3 updates v2, v2 updates v1)
	v1, err := repo.CreateApproach(ctx, &models.Approach{
		ProblemID: problemID, AuthorType: models.AuthorTypeAgent, AuthorID: agentID,
		Angle: "v1 angle", Method: "v1 method", Status: models.ApproachStatusFailed,
	})
	if err != nil {
		t.Fatalf("CreateApproach(v1) error = %v", err)
	}

	v2, err := repo.CreateApproach(ctx, &models.Approach{
		ProblemID: problemID, AuthorType: models.AuthorTypeAgent, AuthorID: agentID,
		Angle: "v2 angle", Method: "v2 method", Status: models.ApproachStatusWorking,
	})
	if err != nil {
		t.Fatalf("CreateApproach(v2) error = %v", err)
	}

	v3, err := repo.CreateApproach(ctx, &models.Approach{
		ProblemID: problemID, AuthorType: models.AuthorTypeAgent, AuthorID: agentID,
		Angle: "v3 angle", Method: "v3 method", Status: models.ApproachStatusStarting,
	})
	if err != nil {
		t.Fatalf("CreateApproach(v3) error = %v", err)
	}

	// v2 updates v1
	_, err = relRepo.CreateRelationship(ctx, &models.ApproachRelationship{
		FromApproachID: v2.ID, ToApproachID: v1.ID, RelationType: models.RelationTypeUpdates,
	})
	if err != nil {
		t.Fatalf("CreateRelationship(v2->v1) error = %v", err)
	}

	// v3 updates v2
	_, err = relRepo.CreateRelationship(ctx, &models.ApproachRelationship{
		FromApproachID: v3.ID, ToApproachID: v2.ID, RelationType: models.RelationTypeUpdates,
	})
	if err != nil {
		t.Fatalf("CreateRelationship(v3->v2) error = %v", err)
	}

	// Get version chain from v3 (no depth limit)
	history, err := relRepo.GetVersionChain(ctx, v3.ID, 0)
	if err != nil {
		t.Fatalf("GetVersionChain() error = %v", err)
	}

	if history.Current.ID != v3.ID {
		t.Errorf("expected current = v3 (%s), got %s", v3.ID, history.Current.ID)
	}

	// History should contain v2 and v1 (oldest first)
	if len(history.History) != 2 {
		t.Fatalf("expected 2 history entries, got %d", len(history.History))
	}
	if history.History[0].ID != v1.ID {
		t.Errorf("expected history[0] = v1 (%s), got %s", v1.ID, history.History[0].ID)
	}
	if history.History[1].ID != v2.ID {
		t.Errorf("expected history[1] = v2 (%s), got %s", v2.ID, history.History[1].ID)
	}

	// Relationships should contain 2 entries
	if len(history.Relationships) != 2 {
		t.Fatalf("expected 2 relationships, got %d", len(history.Relationships))
	}
}

func TestApproachRelationships_GetVersionChain_WithDepth(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewApproachesRepository(pool)
	relRepo := NewApproachRelationshipsRepository(pool)
	ctx := context.Background()

	ts := time.Now().Format("150405")
	problemID := "rel_depth_problem_" + ts

	_, err := pool.Exec(ctx, `
		INSERT INTO posts (id, type, title, description, posted_by_type, posted_by_id, status)
		VALUES ($1, 'problem', 'Depth Test', 'Description for depth test', 'agent', 'test_agent', 'open')
	`, problemID)
	if err != nil {
		t.Fatalf("failed to insert test problem: %v", err)
	}

	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM approach_relationships WHERE from_approach_id IN (SELECT id FROM approaches WHERE problem_id = $1)", problemID)
		_, _ = pool.Exec(ctx, "DELETE FROM approach_relationships WHERE to_approach_id IN (SELECT id FROM approaches WHERE problem_id = $1)", problemID)
		_, _ = pool.Exec(ctx, "DELETE FROM approaches WHERE problem_id = $1", problemID)
		_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE id = $1", problemID)
	}()

	agentID := "agent_" + ts

	// Create chain: v1 -> v2 -> v3
	v1, _ := repo.CreateApproach(ctx, &models.Approach{
		ProblemID: problemID, AuthorType: models.AuthorTypeAgent, AuthorID: agentID,
		Angle: "v1", Status: models.ApproachStatusFailed,
	})
	v2, _ := repo.CreateApproach(ctx, &models.Approach{
		ProblemID: problemID, AuthorType: models.AuthorTypeAgent, AuthorID: agentID,
		Angle: "v2", Status: models.ApproachStatusWorking,
	})
	v3, _ := repo.CreateApproach(ctx, &models.Approach{
		ProblemID: problemID, AuthorType: models.AuthorTypeAgent, AuthorID: agentID,
		Angle: "v3", Status: models.ApproachStatusStarting,
	})

	_, _ = relRepo.CreateRelationship(ctx, &models.ApproachRelationship{
		FromApproachID: v2.ID, ToApproachID: v1.ID, RelationType: models.RelationTypeUpdates,
	})
	_, _ = relRepo.CreateRelationship(ctx, &models.ApproachRelationship{
		FromApproachID: v3.ID, ToApproachID: v2.ID, RelationType: models.RelationTypeUpdates,
	})

	// Depth=1 should return only v2 (one step back from v3)
	history, err := relRepo.GetVersionChain(ctx, v3.ID, 1)
	if err != nil {
		t.Fatalf("GetVersionChain(depth=1) error = %v", err)
	}

	if len(history.History) != 1 {
		t.Fatalf("expected 1 history entry with depth=1, got %d", len(history.History))
	}
	if history.History[0].ID != v2.ID {
		t.Errorf("expected history[0] = v2 (%s), got %s", v2.ID, history.History[0].ID)
	}
}

func TestApproachRelationships_ListStaleApproaches(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	relRepo := NewApproachRelationshipsRepository(pool)
	ctx := context.Background()

	ts := time.Now().Format("150405")
	problemID := "rel_stale_problem_" + ts

	_, err := pool.Exec(ctx, `
		INSERT INTO posts (id, type, title, description, posted_by_type, posted_by_id, status)
		VALUES ($1, 'problem', 'Stale Test', 'Description for stale test', 'agent', 'test_agent', 'open')
	`, problemID)
	if err != nil {
		t.Fatalf("failed to insert test problem: %v", err)
	}

	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM approach_relationships WHERE from_approach_id IN (SELECT id FROM approaches WHERE problem_id = $1)", problemID)
		_, _ = pool.Exec(ctx, "DELETE FROM approach_relationships WHERE to_approach_id IN (SELECT id FROM approaches WHERE problem_id = $1)", problemID)
		_, _ = pool.Exec(ctx, "DELETE FROM approaches WHERE problem_id = $1", problemID)
		_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE id = $1", problemID)
	}()

	agentID := "agent_" + ts

	// Create an old failed approach (> 90 days)
	_, err = pool.Exec(ctx, `
		INSERT INTO approaches (problem_id, author_type, author_id, angle, status, created_at, updated_at)
		VALUES ($1, 'agent', $2, 'Old failed approach', 'failed', NOW() - INTERVAL '100 days', NOW() - INTERVAL '100 days')
	`, problemID, agentID)
	if err != nil {
		t.Fatalf("failed to insert old failed approach: %v", err)
	}

	// Create a recent failed approach (< 90 days) — should NOT be stale
	_, err = pool.Exec(ctx, `
		INSERT INTO approaches (problem_id, author_type, author_id, angle, status, created_at, updated_at)
		VALUES ($1, 'agent', $2, 'Recent failed approach', 'failed', NOW() - INTERVAL '30 days', NOW() - INTERVAL '30 days')
	`, problemID, agentID)
	if err != nil {
		t.Fatalf("failed to insert recent failed approach: %v", err)
	}

	// Create an old superseded approach (is_latest=false, > 180 days)
	_, err = pool.Exec(ctx, `
		INSERT INTO approaches (problem_id, author_type, author_id, angle, status, is_latest, created_at, updated_at)
		VALUES ($1, 'agent', $2, 'Old superseded approach', 'working', false, NOW() - INTERVAL '200 days', NOW() - INTERVAL '200 days')
	`, problemID, agentID)
	if err != nil {
		t.Fatalf("failed to insert old superseded approach: %v", err)
	}

	// List stale approaches: failed > 90 days, superseded > 180 days
	stale, err := relRepo.ListStaleApproaches(ctx, 90, 180)
	if err != nil {
		t.Fatalf("ListStaleApproaches() error = %v", err)
	}

	// Should find at least 2 stale approaches (old failed + old superseded)
	foundOldFailed := false
	foundOldSuperseded := false
	for _, a := range stale {
		if a.Angle == "Old failed approach" {
			foundOldFailed = true
		}
		if a.Angle == "Old superseded approach" {
			foundOldSuperseded = true
		}
		// Should NOT find the recent failed approach
		if a.Angle == "Recent failed approach" {
			t.Error("recent failed approach should NOT be stale")
		}
	}

	if !foundOldFailed {
		t.Error("expected to find old failed approach in stale list")
	}
	if !foundOldSuperseded {
		t.Error("expected to find old superseded approach in stale list")
	}
}
