package db

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// Note: These tests require a running PostgreSQL database.
// Set DATABASE_URL environment variable to run integration tests.
// Tests will be skipped if DATABASE_URL is not set.

// createAgentTestUser creates a real user in the database to use as a humanID FK reference.
func createAgentTestUser(t *testing.T, pool *Pool) string {
	t.Helper()
	ctx := context.Background()
	userRepo := NewUserRepository(pool)
	now := time.Now()
	ts := now.Format("150405.000000")
	user := &models.User{
		Username:       "ag" + now.Format("0405") + fmt.Sprintf("%06d", now.Nanosecond()/1000)[:4],
		DisplayName:    "Agent Test User",
		Email:          "agenttest_" + ts + "@example.com",
		AuthProvider:   "github",
		AuthProviderID: "github_agenttest_" + ts,
		Role:           "user",
	}
	created, err := userRepo.Create(ctx, user)
	if err != nil {
		t.Fatalf("failed to create agent test user: %v", err)
	}
	return created.ID
}

func TestAgentRepository_Create(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewAgentRepository(pool)
	ctx := context.Background()

	humanID := createAgentTestUser(t, pool)
	now := time.Now()
	ns := fmt.Sprintf("%04d", now.Nanosecond()/100000)
	agent := &models.Agent{
		ID:          "test_agent_" + now.Format("20060102150405") + ns,
		DisplayName: "Test Agent " + now.Format("150405") + ns,
		HumanID:     &humanID,
		Bio:         "A test agent",
		Specialties: []string{"testing", "golang"},
	}

	err := repo.Create(ctx, agent)
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	// After Create, agent struct should be populated with DB values
	if agent.ID == "" {
		t.Error("expected ID to be set")
	}
	if agent.DisplayName == "" {
		t.Error("expected display name to be set")
	}
	if agent.CreatedAt.IsZero() {
		t.Error("expected created_at to be set")
	}
}

func TestAgentRepository_Create_Duplicate(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewAgentRepository(pool)
	ctx := context.Background()

	humanID := createAgentTestUser(t, pool)
	now2 := time.Now()
	ns2 := fmt.Sprintf("%04d", now2.Nanosecond()/100000)
	agentID := "dup_test_" + now2.Format("150405") + ns2
	agent := &models.Agent{
		ID:          agentID,
		DisplayName: "Dup Agent " + now2.Format("150405") + ns2,
		HumanID:     &humanID,
	}

	// Create first time
	err := repo.Create(ctx, agent)
	if err != nil {
		t.Fatalf("failed to create first agent: %v", err)
	}

	// Try to create duplicate (same ID, different display name)
	duplicate := &models.Agent{
		ID:          agentID,
		DisplayName: "Dup Agent Alt " + now2.Format("150405") + ns2,
		HumanID:     &humanID,
	}
	err = repo.Create(ctx, duplicate)
	if err != ErrDuplicateAgentID {
		t.Errorf("expected ErrDuplicateAgentID, got %v", err)
	}
}

func TestAgentRepository_FindByID(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewAgentRepository(pool)
	ctx := context.Background()

	humanID := createAgentTestUser(t, pool)
	now3 := time.Now()
	ns3 := fmt.Sprintf("%04d", now3.Nanosecond()/100000)
	agent := &models.Agent{
		ID:          "fbi_test_" + now3.Format("150405") + ns3,
		DisplayName: "FBI Agent " + now3.Format("150405") + ns3,
		HumanID:     &humanID,
		Bio:         "Bio here",
	}

	err := repo.Create(ctx, agent)
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	found, err := repo.FindByID(ctx, agent.ID)
	if err != nil {
		t.Fatalf("failed to find agent: %v", err)
	}

	if found.ID != agent.ID {
		t.Errorf("expected ID %s, got %s", agent.ID, found.ID)
	}
	if found.Bio != "Bio here" {
		t.Errorf("expected bio 'Bio here', got %s", found.Bio)
	}
}

func TestAgentRepository_FindByID_NotFound(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewAgentRepository(pool)
	ctx := context.Background()

	_, err := repo.FindByID(ctx, "nonexistent_agent")
	if err != ErrAgentNotFound {
		t.Errorf("expected ErrAgentNotFound, got %v", err)
	}
}

func TestAgentRepository_Update(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewAgentRepository(pool)
	ctx := context.Background()

	humanID := createAgentTestUser(t, pool)
	agent := &models.Agent{
		ID:          "update_test_" + time.Now().Format("20060102150405"),
		DisplayName: "Original Name",
		HumanID:     &humanID,
		Bio:         "Original bio",
	}

	err := repo.Create(ctx, agent)
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	// Update agent
	agent.DisplayName = "New Name"
	agent.Bio = "New bio"

	err = repo.Update(ctx, agent)
	if err != nil {
		t.Fatalf("failed to update agent: %v", err)
	}

	// After Update, agent struct should be populated with new values
	if agent.DisplayName != "New Name" {
		t.Errorf("expected display name 'New Name', got %s", agent.DisplayName)
	}
	if agent.Bio != "New bio" {
		t.Errorf("expected bio 'New bio', got %s", agent.Bio)
	}
}

func TestAgentRepository_GetAgentStats(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewAgentRepository(pool)
	ctx := context.Background()

	humanID := createAgentTestUser(t, pool)
	agent := &models.Agent{
		ID:          "stats_test_" + time.Now().Format("20060102150405"),
		DisplayName: "Stats Agent",
		HumanID:     &humanID,
	}

	err := repo.Create(ctx, agent)
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	stats, err := NewCanonicalReputationAgentRepository(pool).GetAgentStats(ctx, agent.ID)
	if err != nil {
		t.Fatalf("failed to get agent stats: %v", err)
	}

	// New agent should have zero stats
	if stats.PostsCreated != 0 || stats.Contributions != 0 || stats.UpvotesReceived != 0 {
		t.Errorf("expected zero stats for a new agent, got %+v", stats)
	}
	if stats.Reputation != 0 {
		t.Errorf("expected 0 reputation for new agent, got %d", stats.Reputation)
	}
}

func TestAgentRepository_UpdateAPIKeyHash(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewAgentRepository(pool)
	ctx := context.Background()

	humanID := createAgentTestUser(t, pool)
	agent := &models.Agent{
		ID:          "apikey_test_" + time.Now().Format("20060102150405"),
		DisplayName: "API Key Agent",
		HumanID:     &humanID,
		APIKeyHash:  "original_hash",
	}

	err := repo.Create(ctx, agent)
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	// Update API key hash
	err = repo.UpdateAPIKeyHash(ctx, agent.ID, "new_hash", "new_sha256")
	if err != nil {
		t.Fatalf("failed to update API key hash: %v", err)
	}

	// Verify update
	found, err := repo.FindByID(ctx, agent.ID)
	if err != nil {
		t.Fatalf("failed to find agent: %v", err)
	}
	if found.APIKeyHash != "new_hash" {
		t.Errorf("expected API key hash 'new_hash', got %s", found.APIKeyHash)
	}
}

func TestAgentRepository_RevokeAPIKey(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewAgentRepository(pool)
	ctx := context.Background()

	humanID := createAgentTestUser(t, pool)
	agent := &models.Agent{
		ID:          "revoke_test_" + time.Now().Format("20060102150405"),
		DisplayName: "Revoke Agent",
		HumanID:     &humanID,
		APIKeyHash:  "some_hash",
	}

	err := repo.Create(ctx, agent)
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	// Revoke API key
	err = repo.RevokeAPIKey(ctx, agent.ID)
	if err != nil {
		t.Fatalf("failed to revoke API key: %v", err)
	}

	// Verify revocation
	found, err := repo.FindByID(ctx, agent.ID)
	if err != nil {
		t.Fatalf("failed to find agent: %v", err)
	}
	if found.APIKeyHash != "" {
		t.Errorf("expected empty API key hash after revocation, got %s", found.APIKeyHash)
	}
}

func TestAgentRepository_FindByHumanID(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewAgentRepository(pool)
	ctx := context.Background()

	humanID := createAgentTestUser(t, pool)
	now4 := time.Now()
	ns4 := fmt.Sprintf("%04d", now4.Nanosecond()/100000)
	agent1 := &models.Agent{
		ID:          "agt1_" + now4.Format("150405") + ns4,
		DisplayName: "Agt1 " + now4.Format("150405") + ns4,
		HumanID:     &humanID,
	}
	agent2 := &models.Agent{
		ID:          "agt2_" + now4.Format("150405") + ns4 + "b",
		DisplayName: "Agt2 " + now4.Format("150405") + ns4,
		HumanID:     &humanID,
	}

	err := repo.Create(ctx, agent1)
	if err != nil {
		t.Fatalf("failed to create agent1: %v", err)
	}
	err = repo.Create(ctx, agent2)
	if err != nil {
		t.Fatalf("failed to create agent2: %v", err)
	}

	agents, err := repo.FindByHumanID(ctx, humanID)
	if err != nil {
		t.Fatalf("failed to find agents by human ID: %v", err)
	}

	if len(agents) != 2 {
		t.Errorf("expected 2 agents, got %d", len(agents))
	}
}

// Note: getTestPool is defined in users_test.go and shared across db package tests

// TestAgentRepository_HardDelete tests permanently deleting an agent from the database.
func TestAgentRepository_HardDelete(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewAgentRepository(pool)
	ctx := context.Background()

	// Create an agent
	agent := &models.Agent{
		ID:          "harddelete_" + time.Now().Format("20060102150405"),
		DisplayName: "Hard Delete Agent",
	}

	err := repo.Create(ctx, agent)
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	// Hard delete the agent
	err = repo.HardDelete(ctx, agent.ID)
	if err != nil {
		t.Fatalf("HardDelete() error = %v", err)
	}

	// Verify agent is gone
	_, err = repo.FindByID(ctx, agent.ID)
	if err != ErrAgentNotFound {
		t.Errorf("FindByID() after hard delete error = %v, want ErrAgentNotFound", err)
	}
}

// TestAgentRepository_HardDelete_NotFound tests hard deleting a non-existent agent.
func TestAgentRepository_HardDelete_NotFound(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewAgentRepository(pool)
	ctx := context.Background()

	// Try to hard delete non-existent agent
	err := repo.HardDelete(ctx, "nonexistent_agent")
	if err != ErrAgentNotFound {
		t.Errorf("HardDelete() error = %v, want ErrAgentNotFound", err)
	}
}

// TestAgentRepository_ListDeleted tests listing soft-deleted agents.
func TestAgentRepository_ListDeleted(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewAgentRepository(pool)
	ctx := context.Background()

	// Create 2 agents and soft-delete them
	agent1 := &models.Agent{
		ID:          "deleted_agent1_" + time.Now().Format("20060102150405"),
		DisplayName: "Deleted Agent 1",
	}
	err := repo.Create(ctx, agent1)
	if err != nil {
		t.Fatalf("failed to create agent1: %v", err)
	}

	agent2 := &models.Agent{
		ID:          "deleted_agent2_" + time.Now().Format("20060102150405"),
		DisplayName: "Deleted Agent 2",
	}
	err = repo.Create(ctx, agent2)
	if err != nil {
		t.Fatalf("failed to create agent2: %v", err)
	}

	// Soft-delete both agents
	err = repo.Delete(ctx, agent1.ID)
	if err != nil {
		t.Fatalf("Delete(agent1) error = %v", err)
	}
	time.Sleep(10 * time.Millisecond) // Ensure different deleted_at timestamps
	err = repo.Delete(ctx, agent2.ID)
	if err != nil {
		t.Fatalf("Delete(agent2) error = %v", err)
	}

	// List deleted agents
	agents, total, err := repo.ListDeleted(ctx, 1, 10)
	if err != nil {
		t.Fatalf("ListDeleted() error = %v", err)
	}

	// Should find at least our 2 deleted agents
	if total < 2 {
		t.Errorf("ListDeleted() total = %d, want >= 2", total)
	}

	// Verify our deleted agents are in the list
	found1, found2 := false, false
	for _, a := range agents {
		if a.ID == agent1.ID {
			found1 = true
			if a.DeletedAt == nil {
				t.Error("ListDeleted() agent1 should have deleted_at set")
			}
		}
		if a.ID == agent2.ID {
			found2 = true
			if a.DeletedAt == nil {
				t.Error("ListDeleted() agent2 should have deleted_at set")
			}
		}
	}

	if !found1 {
		t.Error("ListDeleted() did not return agent1")
	}
	if !found2 {
		t.Error("ListDeleted() did not return agent2")
	}
}

// ============================================================================
// Tests for agent model field (prd-v4 requirement)
// ============================================================================

func TestAgentRepository_Create_WithModel(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewAgentRepository(pool)
	ctx := context.Background()

	humanID := createAgentTestUser(t, pool)
	agent := &models.Agent{
		ID:          "model_test_" + time.Now().Format("20060102150405"),
		DisplayName: "Model Test Agent",
		HumanID:     &humanID,
		Bio:         "A test agent with model",
		Model:       "claude-opus-4",
	}

	err := repo.Create(ctx, agent)
	if err != nil {
		t.Fatalf("failed to create agent with model: %v", err)
	}

	// Verify model is persisted
	found, err := repo.FindByID(ctx, agent.ID)
	if err != nil {
		t.Fatalf("failed to find agent: %v", err)
	}

	if found.Model != "claude-opus-4" {
		t.Errorf("expected model 'claude-opus-4', got %s", found.Model)
	}
}

func TestAgentRepository_Create_WithoutModel(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewAgentRepository(pool)
	ctx := context.Background()

	humanID := createAgentTestUser(t, pool)
	agent := &models.Agent{
		ID:          "no_model_test_" + time.Now().Format("20060102150405"),
		DisplayName: "No Model Test Agent",
		HumanID:     &humanID,
	}

	err := repo.Create(ctx, agent)
	if err != nil {
		t.Fatalf("failed to create agent without model: %v", err)
	}

	// Verify model is empty string when not set
	found, err := repo.FindByID(ctx, agent.ID)
	if err != nil {
		t.Fatalf("failed to find agent: %v", err)
	}

	if found.Model != "" {
		t.Errorf("expected empty model, got %s", found.Model)
	}
}

func TestAgentRepository_Update_WithModel(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewAgentRepository(pool)
	ctx := context.Background()

	humanID := createAgentTestUser(t, pool)
	agent := &models.Agent{
		ID:          "update_model_test_" + time.Now().Format("20060102150405"),
		DisplayName: "Update Model Test Agent",
		HumanID:     &humanID,
	}

	err := repo.Create(ctx, agent)
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	// Update agent with model
	agent.Model = "gpt-4-turbo"
	err = repo.Update(ctx, agent)
	if err != nil {
		t.Fatalf("failed to update agent with model: %v", err)
	}

	// Verify model is updated
	found, err := repo.FindByID(ctx, agent.ID)
	if err != nil {
		t.Fatalf("failed to find agent: %v", err)
	}

	if found.Model != "gpt-4-turbo" {
		t.Errorf("expected model 'gpt-4-turbo', got %s", found.Model)
	}
}

func TestAgentRepository_FindByID_AllNullFields(t *testing.T) {
	// TDD: This test verifies that agents with ALL nullable fields as NULL can be retrieved.
	// pgx cannot scan NULL into non-pointer Go types (string, []string).
	// We use COALESCE in agentColumns to convert NULL to empty values.
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewAgentRepository(pool)
	ctx := context.Background()

	// Create agent via raw SQL with ALL nullable fields as NULL
	agentID := "all_null_test_" + time.Now().Format("20060102150405")
	_, err := pool.Exec(ctx, `
		INSERT INTO agents (id, display_name, bio, specialties, avatar_url, api_key_hash, moltbook_id, model, status)
		VALUES ($1, $2, NULL, NULL, NULL, NULL, NULL, NULL, 'active')
	`, agentID, "All Null Agent")
	if err != nil {
		t.Fatalf("failed to insert agent with NULL fields: %v", err)
	}

	// This should NOT fail - COALESCE handles all NULL values
	found, err := repo.FindByID(ctx, agentID)
	if err != nil {
		t.Fatalf("FindByID should handle NULL fields gracefully, got error: %v", err)
	}

	if found.ID != agentID {
		t.Errorf("expected ID %s, got %s", agentID, found.ID)
	}
	if found.DisplayName != "All Null Agent" {
		t.Errorf("expected display name 'All Null Agent', got %s", found.DisplayName)
	}
	// All nullable string fields should be empty string when NULL in database
	if found.Bio != "" {
		t.Errorf("expected empty bio for NULL in DB, got %s", found.Bio)
	}
	if found.AvatarURL != "" {
		t.Errorf("expected empty avatar_url for NULL in DB, got %s", found.AvatarURL)
	}
	if found.APIKeyHash != "" {
		t.Errorf("expected empty api_key_hash for NULL in DB, got %s", found.APIKeyHash)
	}
	if found.MoltbookID != "" {
		t.Errorf("expected empty moltbook_id for NULL in DB, got %s", found.MoltbookID)
	}
	if found.Model != "" {
		t.Errorf("expected empty model for NULL in DB, got %s", found.Model)
	}
	// Specialties should be empty slice when NULL in database
	if len(found.Specialties) != 0 {
		t.Errorf("expected empty specialties for NULL in DB, got %v", found.Specialties)
	}
}
