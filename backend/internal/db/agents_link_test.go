package db

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
	"golang.org/x/crypto/bcrypt"
)

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
	now5 := time.Now()
	ns5 := fmt.Sprintf("%04d", now5.Nanosecond()/100000)
	agent := &models.Agent{
		ID:          "lnktest_" + now5.Format("150405") + ns5,
		DisplayName: "Link Agent " + now5.Format("150405") + ns5,
	}

	err := repo.Create(ctx, agent)
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	// Link to human (must be real UUID since human_id is FK to users)
	humanID := createAgentTestUser(t, pool)
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
	now6 := time.Now()
	ns6 := fmt.Sprintf("%04d", now6.Nanosecond()/100000)
	agent := &models.Agent{
		ID:          "rclaim_" + now6.Format("150405") + ns6,
		DisplayName: "Reclaim Agent " + now6.Format("150405") + ns6,
	}

	err := repo.Create(ctx, agent)
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	// First claim succeeds (must be real UUIDs — FK to users)
	firstHumanID := createAgentTestUser(t, pool)
	err = repo.LinkHuman(ctx, agent.ID, firstHumanID)
	if err != nil {
		t.Fatalf("first claim failed: %v", err)
	}

	// Second claim should fail with ErrAgentAlreadyClaimed
	secondHumanID := createAgentTestUser(t, pool)
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

	err := repo.LinkHuman(ctx, "nonexistent_agent", "00000000-0000-0000-0000-000000000000")
	if err != ErrAgentNotFound {
		t.Errorf("expected ErrAgentNotFound, got %v", err)
	}
}

func TestAgentRepository_AddReputation_Success(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewAgentRepository(pool)
	ctx := context.Background()

	agent := &models.Agent{
		ID:          "rep_test_" + time.Now().Format("20060102150405"),
		DisplayName: "Reputation Test Agent",
	}

	err := repo.Create(ctx, agent)
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	// Add reputation
	err = repo.AddReputation(ctx, agent.ID, 50)
	if err != nil {
		t.Fatalf("failed to add reputation: %v", err)
	}

	// Verify reputation
	found, err := repo.FindByID(ctx, agent.ID)
	if err != nil {
		t.Fatalf("failed to find agent: %v", err)
	}
	if found.Reputation != 50 {
		t.Errorf("expected reputation 50, got %d", found.Reputation)
	}

	// Add more reputation
	err = repo.AddReputation(ctx, agent.ID, 25)
	if err != nil {
		t.Fatalf("failed to add more reputation: %v", err)
	}

	found, err = repo.FindByID(ctx, agent.ID)
	if err != nil {
		t.Fatalf("failed to find agent: %v", err)
	}
	if found.Reputation != 75 {
		t.Errorf("expected reputation 75, got %d", found.Reputation)
	}
}

func TestAgentRepository_AddReputation_AgentNotFound(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewAgentRepository(pool)
	ctx := context.Background()

	err := repo.AddReputation(ctx, "nonexistent_agent", 50)
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
