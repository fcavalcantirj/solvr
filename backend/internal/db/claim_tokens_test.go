package db

import (
	"context"
	"testing"
	"time"
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

	// Create an agent for testing (required for FK constraint)
	agentID := "cleanup_test_" + time.Now().Format("20060102150405")
	_, err := pool.Exec(ctx, `
		INSERT INTO agents (id, display_name)
		VALUES ($1, 'Cleanup Test Agent')
	`, agentID)
	if err != nil {
		t.Fatalf("failed to create test agent: %v", err)
	}
	defer pool.Exec(ctx, "DELETE FROM agents WHERE id = $1", agentID)

	// Insert expired tokens (should be deleted)
	expiredTime := time.Now().Add(-1 * time.Hour)
	_, err = pool.Exec(ctx, `
		INSERT INTO claim_tokens (token, agent_id, expires_at, used_at)
		VALUES
			('expired_token_1', $1, $2, NULL),
			('expired_token_2', $1, $2, NULL)
	`, agentID, expiredTime)
	if err != nil {
		t.Fatalf("failed to insert expired tokens: %v", err)
	}

	// Insert non-expired tokens (should NOT be deleted)
	futureTime := time.Now().Add(1 * time.Hour)
	_, err = pool.Exec(ctx, `
		INSERT INTO claim_tokens (token, agent_id, expires_at, used_at)
		VALUES ('active_token', $1, $2, NULL)
	`, agentID, futureTime)
	if err != nil {
		t.Fatalf("failed to insert active token: %v", err)
	}

	// Insert expired but used tokens (should NOT be deleted - already used)
	_, err = pool.Exec(ctx, `
		INSERT INTO claim_tokens (token, agent_id, expires_at, used_at)
		VALUES ('used_token', $1, $2, NOW())
	`, agentID, expiredTime)
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
