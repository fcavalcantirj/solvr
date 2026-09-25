package db

import (
	"context"
	"strings"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/auth"
	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/google/uuid"
)

// createActiveTestUser inserts a live user whose unique fields come from a fresh UUID, so
// rows other tests leave in a shared test database cannot collide, and removes it (with its
// API keys) when the test ends.
func createActiveTestUser(t *testing.T, pool *Pool) *models.User {
	t.Helper()
	suffix := strings.ReplaceAll(uuid.NewString(), "-", "")
	user, err := NewUserRepository(pool).Create(context.Background(), &models.User{
		Username:       "act_" + suffix[:12],
		DisplayName:    "Active Account Test",
		Email:          "act_" + suffix[:12] + "@test.solvr.dev",
		AuthProvider:   models.AuthProviderGitHub,
		AuthProviderID: "act_" + suffix,
		Role:           models.UserRoleUser,
	})
	if err != nil {
		t.Fatalf("create test user: %v", err)
	}
	t.Cleanup(func() {
		ctx := context.Background()
		pool.Exec(ctx, "DELETE FROM user_api_keys WHERE user_id = $1", user.ID) //nolint:errcheck
		pool.Exec(ctx, "DELETE FROM users WHERE id = $1", user.ID)              //nolint:errcheck
	})
	return user
}

// A JWT outlives the account it was minted for, so authentication asks whether its subject
// is still a live account: present and not soft-deleted (SPEC Part 20.2).
func TestUserRepository_IsActiveUser(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("Skipping test: no database connection available")
	}
	defer pool.Close()
	repo := NewUserRepository(pool)
	ctx := context.Background()
	user := createActiveTestUser(t, pool)

	cases := []struct {
		name, id string
		want     bool
	}{
		{"live account", user.ID, true},
		{"absent account", uuid.NewString(), false},
		{"malformed id", "user-123", false},
		{"empty id", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := repo.IsActiveUser(ctx, tc.id)
			if err != nil {
				t.Fatalf("IsActiveUser(%q) error = %v", tc.id, err)
			}
			if got != tc.want {
				t.Errorf("IsActiveUser(%q) = %v, want %v", tc.id, got, tc.want)
			}
		})
	}

	t.Run("soft-deleted account", func(t *testing.T) {
		if err := repo.Delete(ctx, user.ID); err != nil {
			t.Fatalf("Delete() error = %v", err)
		}
		got, err := repo.IsActiveUser(ctx, user.ID)
		if err != nil {
			t.Fatalf("IsActiveUser() error = %v", err)
		}
		if got {
			t.Error("IsActiveUser() = true for a soft-deleted account, want false")
		}
	})
}

// The user API keys of a soft-deleted account stop authenticating, the same way a deleted
// agent's key does (SPEC Part 20.2 and 20.3).
func TestUserAPIKeyRepository_GetUserByAPIKey_DeletedAccount(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("Skipping test: no database connection available")
	}
	defer pool.Close()
	ctx := context.Background()
	user := createActiveTestUser(t, pool)
	keys := NewUserAPIKeyRepository(pool)

	for _, path := range []struct {
		name     string
		indexed  bool
		keyLabel string
	}{
		{"indexed sha256 lookup", true, "indexed"},
		{"bcrypt fallback lookup", false, "fallback"},
	} {
		t.Run(path.name, func(t *testing.T) {
			plain := "solvr_sk_" + strings.ReplaceAll(uuid.NewString(), "-", "")
			hash, err := auth.HashAPIKey(plain)
			if err != nil {
				t.Fatalf("hash key: %v", err)
			}
			key := &models.UserAPIKey{UserID: user.ID, Name: "deleted account " + path.keyLabel, KeyHash: hash}
			if path.indexed {
				key.KeySHA256 = auth.SHA256APIKey(plain)
			}
			created, err := keys.Create(ctx, key)
			if err != nil {
				t.Fatalf("create key: %v", err)
			}
			if _, err := pool.Exec(ctx, "UPDATE users SET deleted_at = NULL WHERE id = $1", user.ID); err != nil {
				t.Fatalf("restore user: %v", err)
			}
			found, _, err := keys.GetUserByAPIKey(ctx, plain)
			if err != nil || found == nil || found.ID != user.ID {
				t.Fatalf("live account: GetUserByAPIKey() = %v, %v; want the user", found, err)
			}
			if !path.indexed {
				// The live lookup backfilled the hash; clear it so the fallback scan runs again.
				if _, err := pool.Exec(ctx, "UPDATE user_api_keys SET key_sha256 = NULL WHERE id = $1", created.ID); err != nil {
					t.Fatalf("clear key hash: %v", err)
				}
			}
			if _, err := pool.Exec(ctx, "UPDATE users SET deleted_at = NOW() WHERE id = $1", user.ID); err != nil {
				t.Fatalf("soft-delete user: %v", err)
			}
			found, foundKey, err := keys.GetUserByAPIKey(ctx, plain)
			if err != nil {
				t.Fatalf("deleted account: GetUserByAPIKey() error = %v", err)
			}
			if found != nil || foundKey != nil {
				t.Errorf("deleted account: GetUserByAPIKey() found user %v key %v, want none", found, foundKey)
			}
		})
	}
}
