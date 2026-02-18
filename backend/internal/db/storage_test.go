package db

import (
	"context"
	"os"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// Storage integration tests require a real PostgreSQL database.
// Skip when DATABASE_URL is not set.

func TestStorageRepository_GetStorageUsage_Human(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}

	ctx := context.Background()
	pool, err := NewPool(ctx, dbURL)
	if err != nil {
		t.Fatalf("failed to create pool: %v", err)
	}
	defer pool.Close()

	repo := NewStorageRepository(pool)

	// Create a test user
	userRepo := NewUserRepository(pool)
	testUser, err := createTestUserForStorage(ctx, userRepo, t)
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}
	defer cleanupTestUser(ctx, pool, testUser.ID)

	// Get storage usage — should be 0 used, 100MB quota (default)
	used, quota, err := repo.GetStorageUsage(ctx, testUser.ID, "human")
	if err != nil {
		t.Fatalf("GetStorageUsage failed: %v", err)
	}

	if used != 0 {
		t.Errorf("expected used=0, got %d", used)
	}
	if quota != 104857600 {
		t.Errorf("expected quota=104857600 (100MB default), got %d", quota)
	}
}

func TestStorageRepository_UpdateStorageUsed_Human(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}

	ctx := context.Background()
	pool, err := NewPool(ctx, dbURL)
	if err != nil {
		t.Fatalf("failed to create pool: %v", err)
	}
	defer pool.Close()

	repo := NewStorageRepository(pool)
	userRepo := NewUserRepository(pool)
	testUser, err := createTestUserForStorage(ctx, userRepo, t)
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}
	defer cleanupTestUser(ctx, pool, testUser.ID)

	// Add 50MB
	err = repo.UpdateStorageUsed(ctx, testUser.ID, "human", 52428800)
	if err != nil {
		t.Fatalf("UpdateStorageUsed failed: %v", err)
	}

	used, _, err := repo.GetStorageUsage(ctx, testUser.ID, "human")
	if err != nil {
		t.Fatalf("GetStorageUsage failed: %v", err)
	}
	if used != 52428800 {
		t.Errorf("expected used=52428800, got %d", used)
	}

	// Remove 20MB
	err = repo.UpdateStorageUsed(ctx, testUser.ID, "human", -20971520)
	if err != nil {
		t.Fatalf("UpdateStorageUsed decrement failed: %v", err)
	}

	used, _, err = repo.GetStorageUsage(ctx, testUser.ID, "human")
	if err != nil {
		t.Fatalf("GetStorageUsage after decrement failed: %v", err)
	}
	if used != 31457280 {
		t.Errorf("expected used=31457280 (50MB-20MB), got %d", used)
	}
}

func TestStorageRepository_UpdateStorageUsed_NoNegative(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}

	ctx := context.Background()
	pool, err := NewPool(ctx, dbURL)
	if err != nil {
		t.Fatalf("failed to create pool: %v", err)
	}
	defer pool.Close()

	repo := NewStorageRepository(pool)
	userRepo := NewUserRepository(pool)
	testUser, err := createTestUserForStorage(ctx, userRepo, t)
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}
	defer cleanupTestUser(ctx, pool, testUser.ID)

	// Try to subtract more than available — should clamp to 0
	err = repo.UpdateStorageUsed(ctx, testUser.ID, "human", -100000000)
	if err != nil {
		t.Fatalf("UpdateStorageUsed should not fail: %v", err)
	}

	used, _, err := repo.GetStorageUsage(ctx, testUser.ID, "human")
	if err != nil {
		t.Fatalf("GetStorageUsage failed: %v", err)
	}
	if used != 0 {
		t.Errorf("expected used=0 (clamped), got %d", used)
	}
}

func TestStorageRepository_GetStorageUsage_Agent(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}

	ctx := context.Background()
	pool, err := NewPool(ctx, dbURL)
	if err != nil {
		t.Fatalf("failed to create pool: %v", err)
	}
	defer pool.Close()

	repo := NewStorageRepository(pool)

	// Create test agent
	agentID := "storage-test-agent-" + randomSuffix()
	_, err = pool.Exec(ctx,
		`INSERT INTO agents (id, display_name, status, pinning_quota_bytes, storage_used_bytes) VALUES ($1, $2, 'active', 1073741824, 0)`,
		agentID, "Storage Test Agent",
	)
	if err != nil {
		t.Fatalf("failed to create test agent: %v", err)
	}
	defer pool.Exec(ctx, `DELETE FROM agents WHERE id = $1`, agentID)

	used, quota, err := repo.GetStorageUsage(ctx, agentID, "agent")
	if err != nil {
		t.Fatalf("GetStorageUsage for agent failed: %v", err)
	}

	if used != 0 {
		t.Errorf("expected used=0, got %d", used)
	}
	if quota != 1073741824 {
		t.Errorf("expected quota=1073741824 (1GB), got %d", quota)
	}
}

func TestStorageRepository_InvalidOwnerType(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}

	ctx := context.Background()
	pool, err := NewPool(ctx, dbURL)
	if err != nil {
		t.Fatalf("failed to create pool: %v", err)
	}
	defer pool.Close()

	repo := NewStorageRepository(pool)

	_, _, err = repo.GetStorageUsage(ctx, "some-id", "invalid")
	if err == nil {
		t.Error("expected error for invalid owner type")
	}

	err = repo.UpdateStorageUsed(ctx, "some-id", "invalid", 100)
	if err == nil {
		t.Error("expected error for invalid owner type")
	}
}

// --- Helpers ---

func createTestUserForStorage(ctx context.Context, repo *UserRepository, t *testing.T) (*models.User, error) {
	t.Helper()
	suffix := randomSuffix()
	user := &models.User{
		Username:       "storagetest" + suffix,
		DisplayName:    "Storage Test User",
		Email:          "storage" + suffix + "@test.com",
		AuthProvider:   "github",
		AuthProviderID: "gh-storage-" + suffix,
		Role:           "user",
	}
	return repo.Create(ctx, user)
}

func cleanupTestUser(ctx context.Context, pool *Pool, userID string) {
	pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, userID)
}
