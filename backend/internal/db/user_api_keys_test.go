// Package db provides database access for Solvr.
package db

import (
	"context"
	"errors"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/auth"
	"github.com/fcavalcantirj/solvr/internal/models"
)

// TestUserAPIKeyRepository_Create tests creating a new API key.
func TestUserAPIKeyRepository_Create(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("Skipping test: no database connection available")
	}
	defer cleanupTestDBWithAPIKeys(t, pool)

	// Create a user first
	userRepo := NewUserRepository(pool)
	user, err := userRepo.Create(context.Background(), &models.User{
		Username:       "apikey_test_user",
		DisplayName:    "API Key Test User",
		Email:          "apikey_test@example.com",
		AuthProvider:   models.AuthProviderGitHub,
		AuthProviderID: "apikey_github_123",
		Role:           models.UserRoleUser,
	})
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	repo := NewUserAPIKeyRepository(pool)
	ctx := context.Background()

	// Generate and hash a test key
	rawKey := auth.GenerateAPIKey()
	hash, err := auth.HashAPIKey(rawKey)
	if err != nil {
		t.Fatalf("Failed to hash key: %v", err)
	}

	apiKey := &models.UserAPIKey{
		UserID:  user.ID,
		Name:    "Production",
		KeyHash: hash,
	}

	created, err := repo.Create(ctx, apiKey)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if created.ID == "" {
		t.Error("Create() did not set ID")
	}
	if created.UserID != user.ID {
		t.Errorf("Create() UserID = %v, want %v", created.UserID, user.ID)
	}
	if created.Name != "Production" {
		t.Errorf("Create() Name = %v, want %v", created.Name, "Production")
	}
	if created.CreatedAt.IsZero() {
		t.Error("Create() did not set CreatedAt")
	}
	if !created.IsActive() {
		t.Error("Create() key should be active")
	}
}

// TestUserAPIKeyRepository_FindByUserID tests finding all keys for a user.
func TestUserAPIKeyRepository_FindByUserID(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("Skipping test: no database connection available")
	}
	defer cleanupTestDBWithAPIKeys(t, pool)

	// Create a user
	userRepo := NewUserRepository(pool)
	user, err := userRepo.Create(context.Background(), &models.User{
		Username:       "findkeys_user",
		DisplayName:    "Find Keys User",
		Email:          "findkeys@example.com",
		AuthProvider:   models.AuthProviderGitHub,
		AuthProviderID: "findkeys_github_123",
		Role:           models.UserRoleUser,
	})
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	repo := NewUserAPIKeyRepository(pool)
	ctx := context.Background()

	// Create two keys
	for i, name := range []string{"Production", "Development"} {
		rawKey := auth.GenerateAPIKey()
		hash, _ := auth.HashAPIKey(rawKey)
		_, err := repo.Create(ctx, &models.UserAPIKey{
			UserID:  user.ID,
			Name:    name,
			KeyHash: hash,
		})
		if err != nil {
			t.Fatalf("Create() key %d error = %v", i, err)
		}
	}

	// Find by user ID
	keys, err := repo.FindByUserID(ctx, user.ID)
	if err != nil {
		t.Fatalf("FindByUserID() error = %v", err)
	}

	if len(keys) != 2 {
		t.Errorf("FindByUserID() returned %d keys, want 2", len(keys))
	}
}

// TestUserAPIKeyRepository_FindByUserID_OnlyActive tests that only active keys are returned.
func TestUserAPIKeyRepository_FindByUserID_OnlyActive(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("Skipping test: no database connection available")
	}
	defer cleanupTestDBWithAPIKeys(t, pool)

	// Create a user
	userRepo := NewUserRepository(pool)
	user, err := userRepo.Create(context.Background(), &models.User{
		Username:       "active_keys_user",
		DisplayName:    "Active Keys User",
		Email:          "activekeys@example.com",
		AuthProvider:   models.AuthProviderGitHub,
		AuthProviderID: "activekeys_github_123",
		Role:           models.UserRoleUser,
	})
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	repo := NewUserAPIKeyRepository(pool)
	ctx := context.Background()

	// Create two keys
	var keyToRevoke *models.UserAPIKey
	for i, name := range []string{"ToRevoke", "KeepActive"} {
		rawKey := auth.GenerateAPIKey()
		hash, _ := auth.HashAPIKey(rawKey)
		created, err := repo.Create(ctx, &models.UserAPIKey{
			UserID:  user.ID,
			Name:    name,
			KeyHash: hash,
		})
		if err != nil {
			t.Fatalf("Create() key %d error = %v", i, err)
		}
		if name == "ToRevoke" {
			keyToRevoke = created
		}
	}

	// Revoke one key
	err = repo.Revoke(ctx, keyToRevoke.ID, user.ID)
	if err != nil {
		t.Fatalf("Revoke() error = %v", err)
	}

	// Find by user ID should return only active key
	keys, err := repo.FindByUserID(ctx, user.ID)
	if err != nil {
		t.Fatalf("FindByUserID() error = %v", err)
	}

	if len(keys) != 1 {
		t.Errorf("FindByUserID() returned %d keys, want 1 (only active)", len(keys))
	}
	if keys[0].Name != "KeepActive" {
		t.Errorf("FindByUserID() returned key %s, want KeepActive", keys[0].Name)
	}
}

// TestUserAPIKeyRepository_FindByUserID_Empty tests when user has no keys.
func TestUserAPIKeyRepository_FindByUserID_Empty(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("Skipping test: no database connection available")
	}
	defer cleanupTestDBWithAPIKeys(t, pool)

	// Create a user with no keys
	userRepo := NewUserRepository(pool)
	user, err := userRepo.Create(context.Background(), &models.User{
		Username:       "nokeys_user",
		DisplayName:    "No Keys User",
		Email:          "nokeys@example.com",
		AuthProvider:   models.AuthProviderGitHub,
		AuthProviderID: "nokeys_github_123",
		Role:           models.UserRoleUser,
	})
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	repo := NewUserAPIKeyRepository(pool)
	ctx := context.Background()

	keys, err := repo.FindByUserID(ctx, user.ID)
	if err != nil {
		t.Fatalf("FindByUserID() error = %v", err)
	}

	if len(keys) != 0 {
		t.Errorf("FindByUserID() returned %d keys, want 0", len(keys))
	}
}

// TestUserAPIKeyRepository_FindByID tests finding a specific key by ID.
func TestUserAPIKeyRepository_FindByID(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("Skipping test: no database connection available")
	}
	defer cleanupTestDBWithAPIKeys(t, pool)

	// Create a user
	userRepo := NewUserRepository(pool)
	user, err := userRepo.Create(context.Background(), &models.User{
		Username:       "findbyid_key_user",
		DisplayName:    "Find By ID Key User",
		Email:          "findbyidkey@example.com",
		AuthProvider:   models.AuthProviderGitHub,
		AuthProviderID: "findbyidkey_github_123",
		Role:           models.UserRoleUser,
	})
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	repo := NewUserAPIKeyRepository(pool)
	ctx := context.Background()

	// Create a key
	rawKey := auth.GenerateAPIKey()
	hash, _ := auth.HashAPIKey(rawKey)
	created, err := repo.Create(ctx, &models.UserAPIKey{
		UserID:  user.ID,
		Name:    "TestKey",
		KeyHash: hash,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Find by ID
	found, err := repo.FindByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("FindByID() error = %v", err)
	}

	if found.ID != created.ID {
		t.Errorf("FindByID() ID = %v, want %v", found.ID, created.ID)
	}
	if found.Name != "TestKey" {
		t.Errorf("FindByID() Name = %v, want TestKey", found.Name)
	}
}

// TestUserAPIKeyRepository_FindByID_NotFound tests finding a non-existent key.
func TestUserAPIKeyRepository_FindByID_NotFound(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("Skipping test: no database connection available")
	}
	defer cleanupTestDBWithAPIKeys(t, pool)

	repo := NewUserAPIKeyRepository(pool)
	ctx := context.Background()

	_, err := repo.FindByID(ctx, "00000000-0000-0000-0000-000000000000")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("FindByID() error = %v, want ErrNotFound", err)
	}
}

// TestUserAPIKeyRepository_Revoke tests revoking a key.
func TestUserAPIKeyRepository_Revoke(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("Skipping test: no database connection available")
	}
	defer cleanupTestDBWithAPIKeys(t, pool)

	// Create a user
	userRepo := NewUserRepository(pool)
	user, err := userRepo.Create(context.Background(), &models.User{
		Username:       "revoke_user",
		DisplayName:    "Revoke User",
		Email:          "revoke@example.com",
		AuthProvider:   models.AuthProviderGitHub,
		AuthProviderID: "revoke_github_123",
		Role:           models.UserRoleUser,
	})
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	repo := NewUserAPIKeyRepository(pool)
	ctx := context.Background()

	// Create a key
	rawKey := auth.GenerateAPIKey()
	hash, _ := auth.HashAPIKey(rawKey)
	created, err := repo.Create(ctx, &models.UserAPIKey{
		UserID:  user.ID,
		Name:    "ToRevoke",
		KeyHash: hash,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Revoke the key
	err = repo.Revoke(ctx, created.ID, user.ID)
	if err != nil {
		t.Fatalf("Revoke() error = %v", err)
	}

	// Verify it's revoked
	found, err := repo.FindByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("FindByID() after revoke error = %v", err)
	}

	if found.IsActive() {
		t.Error("Revoke() key should not be active")
	}
	if found.RevokedAt == nil {
		t.Error("Revoke() RevokedAt should be set")
	}
}

// TestUserAPIKeyRepository_Revoke_WrongUser tests that users can only revoke their own keys.
func TestUserAPIKeyRepository_Revoke_WrongUser(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("Skipping test: no database connection available")
	}
	defer cleanupTestDBWithAPIKeys(t, pool)

	userRepo := NewUserRepository(pool)

	// Create two users
	user1, _ := userRepo.Create(context.Background(), &models.User{
		Username:       "revoke_user1",
		DisplayName:    "Revoke User 1",
		Email:          "revoke1@example.com",
		AuthProvider:   models.AuthProviderGitHub,
		AuthProviderID: "revoke1_github_123",
		Role:           models.UserRoleUser,
	})

	user2, _ := userRepo.Create(context.Background(), &models.User{
		Username:       "revoke_user2",
		DisplayName:    "Revoke User 2",
		Email:          "revoke2@example.com",
		AuthProvider:   models.AuthProviderGitHub,
		AuthProviderID: "revoke2_github_123",
		Role:           models.UserRoleUser,
	})

	repo := NewUserAPIKeyRepository(pool)
	ctx := context.Background()

	// Create a key for user1
	rawKey := auth.GenerateAPIKey()
	hash, _ := auth.HashAPIKey(rawKey)
	created, _ := repo.Create(ctx, &models.UserAPIKey{
		UserID:  user1.ID,
		Name:    "User1Key",
		KeyHash: hash,
	})

	// Try to revoke user1's key as user2 - should fail
	err := repo.Revoke(ctx, created.ID, user2.ID)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("Revoke() with wrong user error = %v, want ErrNotFound", err)
	}
}

// TestUserAPIKeyRepository_UpdateLastUsed tests updating last_used_at.
func TestUserAPIKeyRepository_UpdateLastUsed(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("Skipping test: no database connection available")
	}
	defer cleanupTestDBWithAPIKeys(t, pool)

	// Create a user
	userRepo := NewUserRepository(pool)
	user, err := userRepo.Create(context.Background(), &models.User{
		Username:       "lastused_user",
		DisplayName:    "Last Used User",
		Email:          "lastused@example.com",
		AuthProvider:   models.AuthProviderGitHub,
		AuthProviderID: "lastused_github_123",
		Role:           models.UserRoleUser,
	})
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	repo := NewUserAPIKeyRepository(pool)
	ctx := context.Background()

	// Create a key
	rawKey := auth.GenerateAPIKey()
	hash, _ := auth.HashAPIKey(rawKey)
	created, err := repo.Create(ctx, &models.UserAPIKey{
		UserID:  user.ID,
		Name:    "TrackUsage",
		KeyHash: hash,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Initially last_used_at should be nil
	if created.LastUsedAt != nil {
		t.Error("Create() LastUsedAt should be nil initially")
	}

	// Update last used
	err = repo.UpdateLastUsed(ctx, created.ID)
	if err != nil {
		t.Fatalf("UpdateLastUsed() error = %v", err)
	}

	// Verify it's updated
	found, err := repo.FindByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("FindByID() error = %v", err)
	}

	if found.LastUsedAt == nil {
		t.Error("UpdateLastUsed() LastUsedAt should be set")
	}
}

// cleanupTestDBWithAPIKeys extends cleanup to include user_api_keys.
func cleanupTestDBWithAPIKeys(t *testing.T, pool *Pool) {
	t.Helper()
	if pool == nil {
		return
	}
	ctx := context.Background()
	// Clean up in reverse order of dependencies
	_, _ = pool.Exec(ctx, "DELETE FROM user_api_keys")
	_, _ = pool.Exec(ctx, "DELETE FROM refresh_tokens")
	_, _ = pool.Exec(ctx, "DELETE FROM notifications")
	_, _ = pool.Exec(ctx, "DELETE FROM agents")
	_, _ = pool.Exec(ctx, "DELETE FROM users")
	pool.Close()
}
