package db

import (
	"context"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
	"golang.org/x/crypto/bcrypt"
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

	err := repo.Create(ctx, agent)
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	// After Create, agent struct should be populated with DB values
	if agent.ID == "" {
		t.Error("expected ID to be set")
	}
	if agent.DisplayName != "Test Agent" {
		t.Errorf("expected display name 'Test Agent', got %s", agent.DisplayName)
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

	humanID := "test-user-id"
	agentID := "duplicate_test_" + time.Now().Format("20060102150405")
	agent := &models.Agent{
		ID:          agentID,
		DisplayName: "Test Agent",
		HumanID:     &humanID,
	}

	// Create first time
	err := repo.Create(ctx, agent)
	if err != nil {
		t.Fatalf("failed to create first agent: %v", err)
	}

	// Try to create duplicate
	duplicate := &models.Agent{
		ID:          agentID,
		DisplayName: "Test Agent Duplicate",
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

	humanID := "test-user-id"
	agent := &models.Agent{
		ID:          "findbyid_test_" + time.Now().Format("20060102150405"),
		DisplayName: "Test Agent",
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

	humanID := "test-user-id"
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

	humanID := "test-user-id"
	agent := &models.Agent{
		ID:          "stats_test_" + time.Now().Format("20060102150405"),
		DisplayName: "Stats Agent",
		HumanID:     &humanID,
	}

	err := repo.Create(ctx, agent)
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	stats, err := repo.GetAgentStats(ctx, agent.ID)
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

	err := repo.Create(ctx, agent)
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	// Update API key hash
	err = repo.UpdateAPIKeyHash(ctx, agent.ID, "new_hash")
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

	humanID := "test-user-id"
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

	err := repo.Create(ctx, agent)
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

	err := repo.Create(ctx, agent)
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

	err := repo.Create(ctx, agent)
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

	err := repo.Create(ctx, agent)
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

// ============================================================================
// Tests for GetAgentByAPIKeyHash (API-CRITICAL requirement)
// Uses bcrypt.CompareHashAndPassword to find agent by raw API key
// ============================================================================

func TestAgentRepository_GetAgentByAPIKeyHash_ValidKey(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewAgentRepository(pool)
	ctx := context.Background()

	// Generate a real API key and hash it using bcrypt
	rawAPIKey := "solvr_test_key_" + time.Now().Format("20060102150405")
	hashedKey, err := hashTestAPIKey(rawAPIKey)
	if err != nil {
		t.Fatalf("failed to hash API key: %v", err)
	}

	agent := &models.Agent{
		ID:          "apikey_lookup_" + time.Now().Format("20060102150405"),
		DisplayName: "API Key Lookup Agent",
		APIKeyHash:  hashedKey,
	}

	err = repo.Create(ctx, agent)
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	// GetAgentByAPIKeyHash should find the agent using bcrypt comparison
	found, err := repo.GetAgentByAPIKeyHash(ctx, rawAPIKey)
	if err != nil {
		t.Fatalf("GetAgentByAPIKeyHash failed: %v", err)
	}

	if found == nil {
		t.Fatal("expected to find agent, got nil")
	}

	if found.ID != agent.ID {
		t.Errorf("expected agent ID %s, got %s", agent.ID, found.ID)
	}
}

func TestAgentRepository_GetAgentByAPIKeyHash_InvalidKey(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewAgentRepository(pool)
	ctx := context.Background()

	// Generate a real API key and hash it using bcrypt
	rawAPIKey := "solvr_real_key_" + time.Now().Format("20060102150405")
	hashedKey, err := hashTestAPIKey(rawAPIKey)
	if err != nil {
		t.Fatalf("failed to hash API key: %v", err)
	}

	agent := &models.Agent{
		ID:          "apikey_invalid_" + time.Now().Format("20060102150405"),
		DisplayName: "API Key Invalid Test Agent",
		APIKeyHash:  hashedKey,
	}

	err = repo.Create(ctx, agent)
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	// Try to find with wrong key - should return nil, nil (not found)
	found, err := repo.GetAgentByAPIKeyHash(ctx, "solvr_wrong_key")
	if err != nil {
		t.Fatalf("GetAgentByAPIKeyHash should not error on wrong key: %v", err)
	}

	if found != nil {
		t.Errorf("expected nil agent for invalid key, got %+v", found)
	}
}

func TestAgentRepository_GetAgentByAPIKeyHash_NoAgentsWithKey(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewAgentRepository(pool)
	ctx := context.Background()

	// Try to find agent when no agents have API keys
	found, err := repo.GetAgentByAPIKeyHash(ctx, "solvr_nonexistent_key")
	if err != nil {
		t.Fatalf("GetAgentByAPIKeyHash should not error when no agents: %v", err)
	}

	if found != nil {
		t.Errorf("expected nil agent when no matching key, got %+v", found)
	}
}

// hashTestAPIKey is a test helper that hashes an API key using bcrypt.
// Uses the same bcrypt cost as the production code (10).
func hashTestAPIKey(key string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(key), 10)
	if err != nil {
		return "", err
	}
	return string(hash), nil
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

	humanID := "test-user-id"
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

	humanID := "test-user-id"
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

	humanID := "test-user-id"
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
