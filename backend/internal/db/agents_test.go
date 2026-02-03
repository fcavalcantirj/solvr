package db

import (
	"context"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// Note: These tests require a running PostgreSQL database.
// Set DATABASE_URL environment variable to run integration tests.
// Tests will be skipped if DATABASE_URL is not set.

func TestAgentRepository_Create(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewAgentRepository(pool)
	ctx := context.Background()

	humanID := "test-user-id"
	agent := &models.Agent{
		ID:          "test_agent_" + time.Now().Format("20060102150405"),
		DisplayName: "Test Agent",
		HumanID:     &humanID,
		Bio:         "A test agent",
		Specialties: []string{"testing", "golang"},
	}

	created, err := repo.Create(ctx, agent)
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	if created.ID != agent.ID {
		t.Errorf("expected ID %s, got %s", agent.ID, created.ID)
	}
	if created.DisplayName != agent.DisplayName {
		t.Errorf("expected display name %s, got %s", agent.DisplayName, created.DisplayName)
	}
	if created.CreatedAt.IsZero() {
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

	humanID := "test-user-id"
	agentID := "duplicate_test_" + time.Now().Format("20060102150405")
	agent := &models.Agent{
		ID:          agentID,
		DisplayName: "Test Agent",
		HumanID:     &humanID,
	}

	// Create first time
	_, err := repo.Create(ctx, agent)
	if err != nil {
		t.Fatalf("failed to create first agent: %v", err)
	}

	// Try to create duplicate
	_, err = repo.Create(ctx, agent)
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

	humanID := "test-user-id"
	agent := &models.Agent{
		ID:          "findbyid_test_" + time.Now().Format("20060102150405"),
		DisplayName: "Test Agent",
		HumanID:     &humanID,
		Bio:         "Bio here",
	}

	created, err := repo.Create(ctx, agent)
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	found, err := repo.FindByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("failed to find agent: %v", err)
	}

	if found.ID != created.ID {
		t.Errorf("expected ID %s, got %s", created.ID, found.ID)
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

	humanID := "test-user-id"
	agent := &models.Agent{
		ID:          "update_test_" + time.Now().Format("20060102150405"),
		DisplayName: "Original Name",
		HumanID:     &humanID,
		Bio:         "Original bio",
	}

	created, err := repo.Create(ctx, agent)
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	// Update agent
	created.DisplayName = "New Name"
	created.Bio = "New bio"

	updated, err := repo.Update(ctx, created)
	if err != nil {
		t.Fatalf("failed to update agent: %v", err)
	}

	if updated.DisplayName != "New Name" {
		t.Errorf("expected display name 'New Name', got %s", updated.DisplayName)
	}
	if updated.Bio != "New bio" {
		t.Errorf("expected bio 'New bio', got %s", updated.Bio)
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

	humanID := "test-user-id"
	agent := &models.Agent{
		ID:          "stats_test_" + time.Now().Format("20060102150405"),
		DisplayName: "Stats Agent",
		HumanID:     &humanID,
	}

	created, err := repo.Create(ctx, agent)
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	stats, err := repo.GetAgentStats(ctx, created.ID)
	if err != nil {
		t.Fatalf("failed to get agent stats: %v", err)
	}

	// New agent should have zero stats
	if stats.ProblemsSolved != 0 {
		t.Errorf("expected 0 problems solved, got %d", stats.ProblemsSolved)
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

	humanID := "test-user-id"
	agent := &models.Agent{
		ID:          "apikey_test_" + time.Now().Format("20060102150405"),
		DisplayName: "API Key Agent",
		HumanID:     &humanID,
		APIKeyHash:  "original_hash",
	}

	created, err := repo.Create(ctx, agent)
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	// Update API key hash
	err = repo.UpdateAPIKeyHash(ctx, created.ID, "new_hash")
	if err != nil {
		t.Fatalf("failed to update API key hash: %v", err)
	}

	// Verify update
	found, err := repo.FindByID(ctx, created.ID)
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

	humanID := "test-user-id"
	agent := &models.Agent{
		ID:          "revoke_test_" + time.Now().Format("20060102150405"),
		DisplayName: "Revoke Agent",
		HumanID:     &humanID,
		APIKeyHash:  "some_hash",
	}

	created, err := repo.Create(ctx, agent)
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	// Revoke API key
	err = repo.RevokeAPIKey(ctx, created.ID)
	if err != nil {
		t.Fatalf("failed to revoke API key: %v", err)
	}

	// Verify revocation
	found, err := repo.FindByID(ctx, created.ID)
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

	humanID := "human_agents_test_" + time.Now().Format("20060102150405")
	agent1 := &models.Agent{
		ID:          "agent1_" + time.Now().Format("20060102150405"),
		DisplayName: "Agent 1",
		HumanID:     &humanID,
	}
	agent2 := &models.Agent{
		ID:          "agent2_" + time.Now().Format("20060102150405"),
		DisplayName: "Agent 2",
		HumanID:     &humanID,
	}

	_, err := repo.Create(ctx, agent1)
	if err != nil {
		t.Fatalf("failed to create agent1: %v", err)
	}
	_, err = repo.Create(ctx, agent2)
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

// ============================================================================
// Tests for AGENT-LINKING requirement: One human per agent (DB enforced)
// ============================================================================

func TestAgentRepository_LinkHuman_Success(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewAgentRepository(pool)
	ctx := context.Background()

	// Create agent without human_id
	agent := &models.Agent{
		ID:          "linktest_" + time.Now().Format("20060102150405"),
		DisplayName: "Link Test Agent",
	}

	_, err := repo.Create(ctx, agent)
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	// Link to human
	humanID := "test-human-id-123"
	err = repo.LinkHuman(ctx, agent.ID, humanID)
	if err != nil {
		t.Fatalf("failed to link human: %v", err)
	}

	// Verify link
	found, err := repo.FindByID(ctx, agent.ID)
	if err != nil {
		t.Fatalf("failed to find agent: %v", err)
	}
	if found.HumanID == nil || *found.HumanID != humanID {
		t.Errorf("expected human_id %s, got %v", humanID, found.HumanID)
	}
	if found.HumanClaimedAt == nil {
		t.Error("expected human_claimed_at to be set")
	}
}

func TestAgentRepository_LinkHuman_RejectReclaim(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewAgentRepository(pool)
	ctx := context.Background()

	// Create agent without human_id
	agent := &models.Agent{
		ID:          "reclaim_test_" + time.Now().Format("20060102150405"),
		DisplayName: "Reclaim Test Agent",
	}

	_, err := repo.Create(ctx, agent)
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	// First claim succeeds
	firstHumanID := "first-human-123"
	err = repo.LinkHuman(ctx, agent.ID, firstHumanID)
	if err != nil {
		t.Fatalf("first claim failed: %v", err)
	}

	// Second claim should fail with ErrAgentAlreadyClaimed
	secondHumanID := "second-human-456"
	err = repo.LinkHuman(ctx, agent.ID, secondHumanID)
	if err != ErrAgentAlreadyClaimed {
		t.Errorf("expected ErrAgentAlreadyClaimed, got %v", err)
	}

	// Verify original human is still linked
	found, err := repo.FindByID(ctx, agent.ID)
	if err != nil {
		t.Fatalf("failed to find agent: %v", err)
	}
	if found.HumanID == nil || *found.HumanID != firstHumanID {
		t.Errorf("expected human_id %s to be preserved, got %v", firstHumanID, found.HumanID)
	}
}

func TestAgentRepository_LinkHuman_AgentNotFound(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewAgentRepository(pool)
	ctx := context.Background()

	err := repo.LinkHuman(ctx, "nonexistent_agent", "human-123")
	if err != ErrAgentNotFound {
		t.Errorf("expected ErrAgentNotFound, got %v", err)
	}
}

func TestAgentRepository_AddKarma_Success(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewAgentRepository(pool)
	ctx := context.Background()

	agent := &models.Agent{
		ID:          "karma_test_" + time.Now().Format("20060102150405"),
		DisplayName: "Karma Test Agent",
	}

	_, err := repo.Create(ctx, agent)
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	// Add karma
	err = repo.AddKarma(ctx, agent.ID, 50)
	if err != nil {
		t.Fatalf("failed to add karma: %v", err)
	}

	// Verify karma
	found, err := repo.FindByID(ctx, agent.ID)
	if err != nil {
		t.Fatalf("failed to find agent: %v", err)
	}
	if found.Karma != 50 {
		t.Errorf("expected karma 50, got %d", found.Karma)
	}

	// Add more karma
	err = repo.AddKarma(ctx, agent.ID, 25)
	if err != nil {
		t.Fatalf("failed to add more karma: %v", err)
	}

	found, err = repo.FindByID(ctx, agent.ID)
	if err != nil {
		t.Fatalf("failed to find agent: %v", err)
	}
	if found.Karma != 75 {
		t.Errorf("expected karma 75, got %d", found.Karma)
	}
}

func TestAgentRepository_AddKarma_AgentNotFound(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewAgentRepository(pool)
	ctx := context.Background()

	err := repo.AddKarma(ctx, "nonexistent_agent", 50)
	if err != ErrAgentNotFound {
		t.Errorf("expected ErrAgentNotFound, got %v", err)
	}
}

func TestAgentRepository_GrantHumanBackedBadge_Success(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewAgentRepository(pool)
	ctx := context.Background()

	agent := &models.Agent{
		ID:          "badge_test_" + time.Now().Format("20060102150405"),
		DisplayName: "Badge Test Agent",
	}

	_, err := repo.Create(ctx, agent)
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	// Verify initially no badge
	found, err := repo.FindByID(ctx, agent.ID)
	if err != nil {
		t.Fatalf("failed to find agent: %v", err)
	}
	if found.HasHumanBackedBadge {
		t.Error("expected agent to not have badge initially")
	}

	// Grant badge
	err = repo.GrantHumanBackedBadge(ctx, agent.ID)
	if err != nil {
		t.Fatalf("failed to grant badge: %v", err)
	}

	// Verify badge granted
	found, err = repo.FindByID(ctx, agent.ID)
	if err != nil {
		t.Fatalf("failed to find agent: %v", err)
	}
	if !found.HasHumanBackedBadge {
		t.Error("expected agent to have Human-Backed badge")
	}
}

func TestAgentRepository_GrantHumanBackedBadge_AgentNotFound(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewAgentRepository(pool)
	ctx := context.Background()

	err := repo.GrantHumanBackedBadge(ctx, "nonexistent_agent")
	if err != ErrAgentNotFound {
		t.Errorf("expected ErrAgentNotFound, got %v", err)
	}
}
