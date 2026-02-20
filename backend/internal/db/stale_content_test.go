package db

import (
	"context"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// createStaleTestAgent creates a test agent for stale content tests.
func createStaleTestAgent(t *testing.T, pool *Pool, suffix string) *models.Agent {
	t.Helper()
	ctx := context.Background()
	agentRepo := NewAgentRepository(pool)

	agentID := "stale_agent_" + suffix + "_" + time.Now().Format("20060102150405.000000000")
	agent := &models.Agent{
		ID:          agentID,
		DisplayName: "Stale Test Agent " + suffix,
		Status:      "active",
	}
	if err := agentRepo.Create(ctx, agent); err != nil {
		t.Fatalf("failed to create test agent: %v", err)
	}
	return agent
}

// createStaleTestProblem creates a problem post with a specific created_at time.
func createStaleTestProblem(t *testing.T, pool *Pool, authorID string, createdAt time.Time) string {
	t.Helper()
	ctx := context.Background()

	var id string
	err := pool.QueryRow(ctx, `
		INSERT INTO posts (type, title, description, tags, status, posted_by_type, posted_by_id, created_at, updated_at)
		VALUES ('problem', 'Stale test problem', 'A stale test problem description for testing',
		        ARRAY['test'], 'open', 'agent', $1, $2, $2)
		RETURNING id::text
	`, authorID, createdAt).Scan(&id)
	if err != nil {
		t.Fatalf("failed to create stale test problem: %v", err)
	}
	return id
}

// createStaleTestApproach creates an approach with a specific status and updated_at time.
func createStaleTestApproach(t *testing.T, pool *Pool, problemID, authorID string, status string, updatedAt time.Time) string {
	t.Helper()
	ctx := context.Background()

	var id string
	err := pool.QueryRow(ctx, `
		INSERT INTO approaches (problem_id, author_type, author_id, angle, status, created_at, updated_at)
		VALUES ($1, 'agent', $2, 'Test angle for stale approach', $3, $4, $4)
		RETURNING id::text
	`, problemID, authorID, status, updatedAt).Scan(&id)
	if err != nil {
		t.Fatalf("failed to create stale test approach: %v", err)
	}
	return id
}

func TestStaleContent_AbandonApproaches_Integration(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	ctx := context.Background()
	notifRepo := NewNotificationsRepository(pool)
	repo := NewStaleContentRepository(pool, notifRepo)

	agent := createStaleTestAgent(t, pool, "abandon")
	now := time.Now()
	problemID := createStaleTestProblem(t, pool, agent.ID, now.Add(-90*24*time.Hour))

	// Create a working approach that is 35 days old — should be abandoned
	staleApproachID := createStaleTestApproach(t, pool, problemID, agent.ID, "working", now.Add(-35*24*time.Hour))

	// Abandon approaches older than 30 days
	count, err := repo.AbandonStaleApproaches(ctx, 30*24*time.Hour)
	if err != nil {
		t.Fatalf("AbandonStaleApproaches failed: %v", err)
	}
	if count < 1 {
		t.Errorf("expected at least 1 abandoned approach, got %d", count)
	}

	// Verify the approach status is now 'abandoned'
	var status string
	err = pool.QueryRow(ctx, `SELECT status FROM approaches WHERE id = $1`, staleApproachID).Scan(&status)
	if err != nil {
		t.Fatalf("failed to query approach status: %v", err)
	}
	if status != "abandoned" {
		t.Errorf("expected approach status 'abandoned', got '%s'", status)
	}
}

func TestStaleContent_WarnApproaches_Integration(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	ctx := context.Background()
	notifRepo := NewNotificationsRepository(pool)
	repo := NewStaleContentRepository(pool, notifRepo)

	agent := createStaleTestAgent(t, pool, "warn")
	now := time.Now()
	problemID := createStaleTestProblem(t, pool, agent.ID, now.Add(-90*24*time.Hour))

	// Create a working approach that is 25 days old — within the 23-30 day warning window
	createStaleTestApproach(t, pool, problemID, agent.ID, "working", now.Add(-25*24*time.Hour))

	// Warn approaches between 23 and 30 days old
	count, err := repo.WarnApproachesApproachingAbandonment(ctx, 23*24*time.Hour, 30*24*time.Hour)
	if err != nil {
		t.Fatalf("WarnApproachesApproachingAbandonment failed: %v", err)
	}
	if count < 1 {
		t.Errorf("expected at least 1 warned approach, got %d", count)
	}

	// Verify a warning notification was created for the agent
	notifications, _, err := notifRepo.GetNotificationsForAgent(ctx, agent.ID, 1, 20)
	if err != nil {
		t.Fatalf("failed to get notifications: %v", err)
	}

	found := false
	for _, n := range notifications {
		if n.Type == "approach_abandonment_warning" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected to find approach_abandonment_warning notification for agent")
	}
}

func TestStaleContent_MarkDormant_Integration(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	ctx := context.Background()
	notifRepo := NewNotificationsRepository(pool)
	repo := NewStaleContentRepository(pool, notifRepo)

	agent := createStaleTestAgent(t, pool, "dormant")
	now := time.Now()

	// Create an open problem 65 days old with NO approaches — should become dormant
	dormantProblemID := createStaleTestProblem(t, pool, agent.ID, now.Add(-65*24*time.Hour))

	// Mark dormant posts older than 60 days with zero approaches
	count, err := repo.MarkDormantPosts(ctx, 60*24*time.Hour)
	if err != nil {
		t.Fatalf("MarkDormantPosts failed: %v", err)
	}
	if count < 1 {
		t.Errorf("expected at least 1 dormant post, got %d", count)
	}

	// Verify the post status is now 'dormant'
	var status string
	err = pool.QueryRow(ctx, `SELECT status FROM posts WHERE id = $1`, dormantProblemID).Scan(&status)
	if err != nil {
		t.Fatalf("failed to query post status: %v", err)
	}
	if status != "dormant" {
		t.Errorf("expected post status 'dormant', got '%s'", status)
	}
}

func TestStaleContent_SkipsRecentContent(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	ctx := context.Background()
	notifRepo := NewNotificationsRepository(pool)
	repo := NewStaleContentRepository(pool, notifRepo)

	agent := createStaleTestAgent(t, pool, "recent")
	now := time.Now()
	problemID := createStaleTestProblem(t, pool, agent.ID, now.Add(-90*24*time.Hour))

	// Create a working approach that is only 10 days old — should NOT be abandoned
	recentApproachID := createStaleTestApproach(t, pool, problemID, agent.ID, "working", now.Add(-10*24*time.Hour))

	// Also create a recent open problem (10 days old, no approaches) — should NOT be marked dormant
	recentProblemID := createStaleTestProblem(t, pool, agent.ID, now.Add(-10*24*time.Hour))

	// Try to abandon — the 10-day approach should survive
	_, err := repo.AbandonStaleApproaches(ctx, 30*24*time.Hour)
	if err != nil {
		t.Fatalf("AbandonStaleApproaches failed: %v", err)
	}

	// Verify the recent approach is still 'working'
	var approachStatus string
	err = pool.QueryRow(ctx, `SELECT status FROM approaches WHERE id = $1`, recentApproachID).Scan(&approachStatus)
	if err != nil {
		t.Fatalf("failed to query approach status: %v", err)
	}
	if approachStatus != "working" {
		t.Errorf("expected recent approach status to remain 'working', got '%s'", approachStatus)
	}

	// Try to mark dormant — the 10-day problem should survive
	_, err = repo.MarkDormantPosts(ctx, 60*24*time.Hour)
	if err != nil {
		t.Fatalf("MarkDormantPosts failed: %v", err)
	}

	// Verify the recent problem is still 'open'
	var postStatus string
	err = pool.QueryRow(ctx, `SELECT status FROM posts WHERE id = $1`, recentProblemID).Scan(&postStatus)
	if err != nil {
		t.Fatalf("failed to query post status: %v", err)
	}
	if postStatus != "open" {
		t.Errorf("expected recent problem status to remain 'open', got '%s'", postStatus)
	}
}
