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

func TestClaimTokenRepository_DeleteExpiredTokens(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewClaimTokenRepository(pool)
	ctx := context.Background()

	// First, clean up any existing test data
	_, _ = pool.Exec(ctx, "DELETE FROM claim_tokens WHERE agent_id LIKE 'cleanup_test_%'")

	// Create agents for testing (required for FK constraint).
	// The unique partial index allows only ONE unused token per agent, so we
	// use separate agents for each expired token.
	ts := time.Now().Format("20060102150405")
	agentID1 := "cleanup_test_1_" + ts
	agentID2 := "cleanup_test_2_" + ts
	agentID3 := "cleanup_test_3_" + ts
	for i, id := range []string{agentID1, agentID2, agentID3} {
		_, err := pool.Exec(ctx, `
			INSERT INTO agents (id, display_name)
			VALUES ($1, $2)
		`, id, fmt.Sprintf("Cleanup Test Agent %d", i+1))
		if err != nil {
			t.Fatalf("failed to create test agent %d: %v", i+1, err)
		}
		defer pool.Exec(ctx, "DELETE FROM agents WHERE id = $1", id)
	}
	// Clean up any leftover tokens for these agents
	for _, id := range []string{agentID1, agentID2, agentID3} {
		_, _ = pool.Exec(ctx, "DELETE FROM claim_tokens WHERE agent_id = $1", id)
	}

	// Insert expired tokens (should be deleted), one per agent to satisfy the unique index
	expiredTime := time.Now().Add(-1 * time.Hour)
	_, err := pool.Exec(ctx, `
		INSERT INTO claim_tokens (token, agent_id, expires_at, used_at)
		VALUES ('expired_token_1', $1, $2, NULL)
	`, agentID1, expiredTime)
	if err != nil {
		t.Fatalf("failed to insert expired token 1: %v", err)
	}
	_, err = pool.Exec(ctx, `
		INSERT INTO claim_tokens (token, agent_id, expires_at, used_at)
		VALUES ('expired_token_2', $1, $2, NULL)
	`, agentID2, expiredTime)
	if err != nil {
		t.Fatalf("failed to insert expired token 2: %v", err)
	}

	// Insert non-expired tokens (should NOT be deleted)
	futureTime := time.Now().Add(1 * time.Hour)
	_, err = pool.Exec(ctx, `
		INSERT INTO claim_tokens (token, agent_id, expires_at, used_at)
		VALUES ('active_token', $1, $2, NULL)
	`, agentID3, futureTime)
	if err != nil {
		t.Fatalf("failed to insert active token: %v", err)
	}

	// Insert expired but used tokens (should NOT be deleted - already used)
	_, err = pool.Exec(ctx, `
		INSERT INTO claim_tokens (token, agent_id, expires_at, used_at)
		VALUES ('used_token', $1, $2, NOW())
	`, agentID3, expiredTime)
	if err != nil {
		t.Fatalf("failed to insert used token: %v", err)
	}

	// Run the cleanup
	deleted, err := repo.DeleteExpiredTokens(ctx)
	if err != nil {
		t.Fatalf("DeleteExpiredTokens failed: %v", err)
	}

	// Should have deleted exactly 2 expired unused tokens
	if deleted != 2 {
		t.Errorf("expected 2 deleted tokens, got %d", deleted)
	}

	// Verify active token still exists
	var count int
	err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM claim_tokens WHERE token = 'active_token'").Scan(&count)
	if err != nil {
		t.Fatalf("failed to count active tokens: %v", err)
	}
	if count != 1 {
		t.Errorf("active token should still exist, count: %d", count)
	}

	// Verify used token still exists
	err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM claim_tokens WHERE token = 'used_token'").Scan(&count)
	if err != nil {
		t.Fatalf("failed to count used tokens: %v", err)
	}
	if count != 1 {
		t.Errorf("used token should still exist, count: %d", count)
	}
}

func TestClaimTokenRepository_Create(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewClaimTokenRepository(pool)
	ctx := context.Background()

	// Create test agent
	agentID := "create_token_test_" + time.Now().Format("20060102150405")
	_, err := pool.Exec(ctx, `
		INSERT INTO agents (id, display_name)
		VALUES ($1, 'Create Token Test Agent')
	`, agentID)
	if err != nil {
		t.Fatalf("failed to create test agent: %v", err)
	}
	defer pool.Exec(ctx, "DELETE FROM agents WHERE id = $1", agentID)

	// Create a claim token
	token := &models.ClaimToken{
		Token:     "test_token_abc123_" + time.Now().Format("150405"),
		AgentID:   agentID,
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}

	err = repo.Create(ctx, token)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Verify ID was populated
	if token.ID == "" {
		t.Error("expected ID to be populated after Create")
	}

	// Verify CreatedAt was populated
	if token.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be populated after Create")
	}

	// Verify token exists in database
	var count int
	err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM claim_tokens WHERE token = $1", token.Token).Scan(&count)
	if err != nil {
		t.Fatalf("failed to count tokens: %v", err)
	}
	if count != 1 {
		t.Errorf("expected token to exist in database, count: %d", count)
	}

	// Cleanup
	_, _ = pool.Exec(ctx, "DELETE FROM claim_tokens WHERE agent_id = $1", agentID)
}

func TestClaimTokenRepository_Create_DuplicateToken(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewClaimTokenRepository(pool)
	ctx := context.Background()

	// Create test agent
	agentID := "dup_token_test_" + time.Now().Format("20060102150405")
	_, err := pool.Exec(ctx, `
		INSERT INTO agents (id, display_name)
		VALUES ($1, 'Duplicate Token Test Agent')
	`, agentID)
	if err != nil {
		t.Fatalf("failed to create test agent: %v", err)
	}
	defer pool.Exec(ctx, "DELETE FROM agents WHERE id = $1", agentID)

	// Create first token
	tokenValue := "dup_test_token_" + time.Now().Format("150405")
	token1 := &models.ClaimToken{
		Token:     tokenValue,
		AgentID:   agentID,
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}

	err = repo.Create(ctx, token1)
	if err != nil {
		t.Fatalf("first Create failed: %v", err)
	}

	// Try to create duplicate token - should fail
	token2 := &models.ClaimToken{
		Token:     tokenValue, // Same token value
		AgentID:   agentID,
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}

	err = repo.Create(ctx, token2)
	if err == nil {
		t.Error("expected error for duplicate token, got nil")
	}

	// Cleanup
	_, _ = pool.Exec(ctx, "DELETE FROM claim_tokens WHERE agent_id = $1", agentID)
}

func TestClaimTokenRepository_FindByToken(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewClaimTokenRepository(pool)
	ctx := context.Background()

	// Create test agent
	agentID := "find_token_test_" + time.Now().Format("20060102150405")
	_, err := pool.Exec(ctx, `
		INSERT INTO agents (id, display_name)
		VALUES ($1, 'Find Token Test Agent')
	`, agentID)
	if err != nil {
		t.Fatalf("failed to create test agent: %v", err)
	}
	defer pool.Exec(ctx, "DELETE FROM agents WHERE id = $1", agentID)

	// Create a token
	tokenValue := "find_test_token_" + time.Now().Format("150405")
	createToken := &models.ClaimToken{
		Token:     tokenValue,
		AgentID:   agentID,
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}
	err = repo.Create(ctx, createToken)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Find by token
	found, err := repo.FindByToken(ctx, tokenValue)
	if err != nil {
		t.Fatalf("FindByToken failed: %v", err)
	}

	// Verify token was found
	if found == nil {
		t.Fatal("expected token to be found, got nil")
	}

	// Verify all fields
	if found.ID != createToken.ID {
		t.Errorf("expected ID %s, got %s", createToken.ID, found.ID)
	}
	if found.Token != tokenValue {
		t.Errorf("expected token %s, got %s", tokenValue, found.Token)
	}
	if found.AgentID != agentID {
		t.Errorf("expected agent_id %s, got %s", agentID, found.AgentID)
	}
	if found.UsedAt != nil {
		t.Error("expected UsedAt to be nil")
	}
	if found.UsedByHumanID != nil {
		t.Error("expected UsedByHumanID to be nil")
	}

	// Cleanup
	_, _ = pool.Exec(ctx, "DELETE FROM claim_tokens WHERE agent_id = $1", agentID)
}

func TestClaimTokenRepository_FindByToken_NotFound(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewClaimTokenRepository(pool)
	ctx := context.Background()

	// Find non-existent token
	found, err := repo.FindByToken(ctx, "non_existent_token_xyz")
	if err != nil {
		t.Fatalf("FindByToken failed: %v", err)
	}

	// Should return nil for non-existent token
	if found != nil {
		t.Errorf("expected nil for non-existent token, got %+v", found)
	}
}

func TestClaimTokenRepository_FindActiveByAgentID(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewClaimTokenRepository(pool)
	ctx := context.Background()

	// Create test agent
	agentID := "active_agent_test_" + time.Now().Format("20060102150405")
	_, err := pool.Exec(ctx, `
		INSERT INTO agents (id, display_name)
		VALUES ($1, 'Active Agent Test')
	`, agentID)
	if err != nil {
		t.Fatalf("failed to create test agent: %v", err)
	}
	defer pool.Exec(ctx, "DELETE FROM agents WHERE id = $1", agentID)

	// Create an active (unexpired, unused) token
	activeToken := &models.ClaimToken{
		Token:     "active_token_" + time.Now().Format("150405"),
		AgentID:   agentID,
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}
	err = repo.Create(ctx, activeToken)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Find active token by agent ID
	found, err := repo.FindActiveByAgentID(ctx, agentID)
	if err != nil {
		t.Fatalf("FindActiveByAgentID failed: %v", err)
	}

	// Verify token was found
	if found == nil {
		t.Fatal("expected active token to be found, got nil")
	}

	// Verify it's the correct token
	if found.Token != activeToken.Token {
		t.Errorf("expected token %s, got %s", activeToken.Token, found.Token)
	}
	if found.AgentID != agentID {
		t.Errorf("expected agent_id %s, got %s", agentID, found.AgentID)
	}

	// Cleanup
	_, _ = pool.Exec(ctx, "DELETE FROM claim_tokens WHERE agent_id = $1", agentID)
}

func TestClaimTokenRepository_FindActiveByAgentID_ExpiredNotReturned(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewClaimTokenRepository(pool)
	ctx := context.Background()

	// Create test agent
	agentID := "expired_agent_test_" + time.Now().Format("20060102150405")
	_, err := pool.Exec(ctx, `
		INSERT INTO agents (id, display_name)
		VALUES ($1, 'Expired Agent Test')
	`, agentID)
	if err != nil {
		t.Fatalf("failed to create test agent: %v", err)
	}
	defer pool.Exec(ctx, "DELETE FROM agents WHERE id = $1", agentID)

	// Insert an expired token directly (bypassing Create to set past expiry)
	_, err = pool.Exec(ctx, `
		INSERT INTO claim_tokens (token, agent_id, expires_at, used_at)
		VALUES ($1, $2, $3, NULL)
	`, "expired_token_"+time.Now().Format("150405"), agentID, time.Now().Add(-1*time.Hour))
	if err != nil {
		t.Fatalf("failed to insert expired token: %v", err)
	}

	// Find active token - should return nil (expired token not returned)
	found, err := repo.FindActiveByAgentID(ctx, agentID)
	if err != nil {
		t.Fatalf("FindActiveByAgentID failed: %v", err)
	}

	if found != nil {
		t.Errorf("expected nil for expired token, got %+v", found)
	}

	// Cleanup
	_, _ = pool.Exec(ctx, "DELETE FROM claim_tokens WHERE agent_id = $1", agentID)
}

func TestClaimTokenRepository_FindActiveByAgentID_UsedNotReturned(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewClaimTokenRepository(pool)
	ctx := context.Background()

	// Create test agent
	agentID := "used_agent_test_" + time.Now().Format("20060102150405")
	_, err := pool.Exec(ctx, `
		INSERT INTO agents (id, display_name)
		VALUES ($1, 'Used Agent Test')
	`, agentID)
	if err != nil {
		t.Fatalf("failed to create test agent: %v", err)
	}
	defer pool.Exec(ctx, "DELETE FROM agents WHERE id = $1", agentID)

	// Insert a used token (not expired but used_at is set)
	_, err = pool.Exec(ctx, `
		INSERT INTO claim_tokens (token, agent_id, expires_at, used_at)
		VALUES ($1, $2, $3, NOW())
	`, "used_token_"+time.Now().Format("150405"), agentID, time.Now().Add(24*time.Hour))
	if err != nil {
		t.Fatalf("failed to insert used token: %v", err)
	}

	// Find active token - should return nil (used token not returned)
	found, err := repo.FindActiveByAgentID(ctx, agentID)
	if err != nil {
		t.Fatalf("FindActiveByAgentID failed: %v", err)
	}

	if found != nil {
		t.Errorf("expected nil for used token, got %+v", found)
	}

	// Cleanup
	_, _ = pool.Exec(ctx, "DELETE FROM claim_tokens WHERE agent_id = $1", agentID)
}

func TestClaimTokenRepository_MarkUsed(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewClaimTokenRepository(pool)
	ctx := context.Background()

	// Create test human user
	humanID := "00000000-0000-0000-0000-000000000001"
	_, _ = pool.Exec(ctx, `
		INSERT INTO users (id, username, display_name, email, auth_provider, auth_provider_id)
		VALUES ($1, 'markused_test_user', 'Mark Used Test', 'markused@test.com', 'github', 'gh_markused')
		ON CONFLICT (id) DO NOTHING
	`, humanID)
	defer pool.Exec(ctx, "DELETE FROM users WHERE id = $1", humanID)

	// Create test agent
	agentID := "markused_test_" + time.Now().Format("20060102150405")
	_, err := pool.Exec(ctx, `
		INSERT INTO agents (id, display_name)
		VALUES ($1, 'Mark Used Test Agent')
	`, agentID)
	if err != nil {
		t.Fatalf("failed to create test agent: %v", err)
	}
	defer pool.Exec(ctx, "DELETE FROM agents WHERE id = $1", agentID)

	// Create a token
	token := &models.ClaimToken{
		Token:     "markused_token_" + time.Now().Format("150405"),
		AgentID:   agentID,
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}
	err = repo.Create(ctx, token)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Mark the token as used
	err = repo.MarkUsed(ctx, token.ID, humanID)
	if err != nil {
		t.Fatalf("MarkUsed failed: %v", err)
	}

	// Verify token is marked as used in database
	found, err := repo.FindByToken(ctx, token.Token)
	if err != nil {
		t.Fatalf("FindByToken failed: %v", err)
	}

	if found == nil {
		t.Fatal("expected token to exist")
	}
	if found.UsedAt == nil {
		t.Error("expected UsedAt to be set after MarkUsed")
	}
	if found.UsedByHumanID == nil || *found.UsedByHumanID != humanID {
		t.Errorf("expected UsedByHumanID to be %s, got %v", humanID, found.UsedByHumanID)
	}

	// Verify token is no longer active
	active, err := repo.FindActiveByAgentID(ctx, agentID)
	if err != nil {
		t.Fatalf("FindActiveByAgentID failed: %v", err)
	}
	if active != nil {
		t.Error("expected no active token after MarkUsed")
	}

	// Cleanup
	_, _ = pool.Exec(ctx, "DELETE FROM claim_tokens WHERE agent_id = $1", agentID)
}

func TestClaimTokenRepository_MarkUsed_NotFound(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewClaimTokenRepository(pool)
	ctx := context.Background()

	// Try to mark a non-existent token (must be valid UUID format)
	err := repo.MarkUsed(ctx, "00000000-0000-0000-0000-000000000000", "00000000-0000-0000-0000-000000000000")
	if err == nil {
		t.Error("expected error for non-existent token, got nil")
	}
	if err != ErrClaimTokenNotFound {
		t.Errorf("expected ErrClaimTokenNotFound, got %v", err)
	}
}

func TestClaimTokenRepository_DeleteExpiredTokens_NoExpired(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewClaimTokenRepository(pool)
	ctx := context.Background()

	// Clean up any existing test data first
	_, _ = pool.Exec(ctx, "DELETE FROM claim_tokens WHERE agent_id LIKE 'no_expired_test_%'")

	// Create test agent
	agentID := "no_expired_test_" + time.Now().Format("20060102150405")
	_, err := pool.Exec(ctx, `
		INSERT INTO agents (id, display_name)
		VALUES ($1, 'No Expired Test Agent')
	`, agentID)
	if err != nil {
		t.Fatalf("failed to create test agent: %v", err)
	}
	defer pool.Exec(ctx, "DELETE FROM agents WHERE id = $1", agentID)

	// Insert only non-expired tokens
	futureTime := time.Now().Add(24 * time.Hour)
	_, err = pool.Exec(ctx, `
		INSERT INTO claim_tokens (token, agent_id, expires_at, used_at)
		VALUES ('future_token', $1, $2, NULL)
	`, agentID, futureTime)
	if err != nil {
		t.Fatalf("failed to insert future token: %v", err)
	}

	// Run cleanup - should delete nothing
	deleted, err := repo.DeleteExpiredTokens(ctx)
	if err != nil {
		t.Fatalf("DeleteExpiredTokens failed: %v", err)
	}

	if deleted != 0 {
		t.Errorf("expected 0 deleted tokens when no expired tokens exist, got %d", deleted)
	}
}
