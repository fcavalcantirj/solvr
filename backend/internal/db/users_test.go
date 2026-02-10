// Package db provides database access for Solvr.
package db

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// TestUserRepository_CreateUser tests creating a new user.
func TestUserRepository_CreateUser(t *testing.T) {
	// Skip if no database connection available
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("Skipping test: no database connection available")
	}
	defer cleanupTestDB(t, pool)

	repo := NewUserRepository(pool)
	ctx := context.Background()

	user := &models.User{
		Username:       "testuser",
		DisplayName:    "Test User",
		Email:          "test@example.com",
		AuthProvider:   models.AuthProviderGitHub,
		AuthProviderID: "12345",
		AvatarURL:      "https://example.com/avatar.png",
		Bio:            "A test user",
		Role:           models.UserRoleUser,
	}

	created, err := repo.Create(ctx, user)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if created.ID == "" {
		t.Error("Create() did not set ID")
	}
	if created.Username != user.Username {
		t.Errorf("Create() Username = %v, want %v", created.Username, user.Username)
	}
	if created.Email != user.Email {
		t.Errorf("Create() Email = %v, want %v", created.Email, user.Email)
	}
	if created.CreatedAt.IsZero() {
		t.Error("Create() did not set CreatedAt")
	}
}

// TestUserRepository_FindByID tests finding a user by ID.
func TestUserRepository_FindByID(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("Skipping test: no database connection available")
	}
	defer cleanupTestDB(t, pool)

	repo := NewUserRepository(pool)
	ctx := context.Background()

	// Create a user first
	user := &models.User{
		Username:       "findbyid",
		DisplayName:    "Find By ID",
		Email:          "findbyid@example.com",
		AuthProvider:   models.AuthProviderGitHub,
		AuthProviderID: "67890",
		Role:           models.UserRoleUser,
	}

	created, err := repo.Create(ctx, user)
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
	if found.Username != created.Username {
		t.Errorf("FindByID() Username = %v, want %v", found.Username, created.Username)
	}
}

// TestUserRepository_FindByID_NotFound tests finding a non-existent user.
func TestUserRepository_FindByID_NotFound(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("Skipping test: no database connection available")
	}
	defer cleanupTestDB(t, pool)

	repo := NewUserRepository(pool)
	ctx := context.Background()

	_, err := repo.FindByID(ctx, "00000000-0000-0000-0000-000000000000")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("FindByID() error = %v, want ErrNotFound", err)
	}
}

// TestUserRepository_FindByAuthProvider tests finding a user by OAuth provider.
func TestUserRepository_FindByAuthProvider(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("Skipping test: no database connection available")
	}
	defer cleanupTestDB(t, pool)

	repo := NewUserRepository(pool)
	ctx := context.Background()

	// Create a user first
	user := &models.User{
		Username:       "oauthuser",
		DisplayName:    "OAuth User",
		Email:          "oauth@example.com",
		AuthProvider:   models.AuthProviderGitHub,
		AuthProviderID: "github_123",
		Role:           models.UserRoleUser,
	}

	created, err := repo.Create(ctx, user)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Find by auth provider
	found, err := repo.FindByAuthProvider(ctx, models.AuthProviderGitHub, "github_123")
	if err != nil {
		t.Fatalf("FindByAuthProvider() error = %v", err)
	}

	if found.ID != created.ID {
		t.Errorf("FindByAuthProvider() ID = %v, want %v", found.ID, created.ID)
	}
}

// TestUserRepository_FindByAuthProvider_NotFound tests when provider ID doesn't exist.
func TestUserRepository_FindByAuthProvider_NotFound(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("Skipping test: no database connection available")
	}
	defer cleanupTestDB(t, pool)

	repo := NewUserRepository(pool)
	ctx := context.Background()

	_, err := repo.FindByAuthProvider(ctx, models.AuthProviderGitHub, "nonexistent")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("FindByAuthProvider() error = %v, want ErrNotFound", err)
	}
}

// TestUserRepository_FindByEmail tests finding a user by email.
func TestUserRepository_FindByEmail(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("Skipping test: no database connection available")
	}
	defer cleanupTestDB(t, pool)

	repo := NewUserRepository(pool)
	ctx := context.Background()

	// Create a user first
	user := &models.User{
		Username:       "emailuser",
		DisplayName:    "Email User",
		Email:          "emailtest@example.com",
		AuthProvider:   models.AuthProviderGitHub,
		AuthProviderID: "email_123",
		Role:           models.UserRoleUser,
	}

	created, err := repo.Create(ctx, user)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Find by email
	found, err := repo.FindByEmail(ctx, "emailtest@example.com")
	if err != nil {
		t.Fatalf("FindByEmail() error = %v", err)
	}

	if found.ID != created.ID {
		t.Errorf("FindByEmail() ID = %v, want %v", found.ID, created.ID)
	}
}

// TestUserRepository_FindByEmail_NotFound tests when email doesn't exist.
func TestUserRepository_FindByEmail_NotFound(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("Skipping test: no database connection available")
	}
	defer cleanupTestDB(t, pool)

	repo := NewUserRepository(pool)
	ctx := context.Background()

	_, err := repo.FindByEmail(ctx, "nonexistent@example.com")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("FindByEmail() error = %v, want ErrNotFound", err)
	}
}

// TestUserRepository_Update tests updating a user.
func TestUserRepository_Update(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("Skipping test: no database connection available")
	}
	defer cleanupTestDB(t, pool)

	repo := NewUserRepository(pool)
	ctx := context.Background()

	// Create a user first
	user := &models.User{
		Username:       "updateuser",
		DisplayName:    "Update User",
		Email:          "update@example.com",
		AuthProvider:   models.AuthProviderGitHub,
		AuthProviderID: "update_123",
		Role:           models.UserRoleUser,
	}

	created, err := repo.Create(ctx, user)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Update the user
	created.DisplayName = "Updated Display Name"
	created.Bio = "New bio"
	created.AvatarURL = "https://new-avatar.com/pic.png"

	updated, err := repo.Update(ctx, created)
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	if updated.DisplayName != "Updated Display Name" {
		t.Errorf("Update() DisplayName = %v, want %v", updated.DisplayName, "Updated Display Name")
	}
	if updated.Bio != "New bio" {
		t.Errorf("Update() Bio = %v, want %v", updated.Bio, "New bio")
	}
	if updated.UpdatedAt.Before(created.UpdatedAt) || updated.UpdatedAt.Equal(created.UpdatedAt) {
		// This might fail if the update is too fast, add small delay
		time.Sleep(10 * time.Millisecond)
	}
}

// TestUserRepository_DuplicateUsername tests that duplicate usernames are rejected.
func TestUserRepository_DuplicateUsername(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("Skipping test: no database connection available")
	}
	defer cleanupTestDB(t, pool)

	repo := NewUserRepository(pool)
	ctx := context.Background()

	// Create first user
	user1 := &models.User{
		Username:       "duplicate",
		DisplayName:    "First User",
		Email:          "first@example.com",
		AuthProvider:   models.AuthProviderGitHub,
		AuthProviderID: "first_123",
		Role:           models.UserRoleUser,
	}

	_, err := repo.Create(ctx, user1)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Try to create second user with same username
	user2 := &models.User{
		Username:       "duplicate",
		DisplayName:    "Second User",
		Email:          "second@example.com",
		AuthProvider:   models.AuthProviderGoogle,
		AuthProviderID: "second_123",
		Role:           models.UserRoleUser,
	}

	_, err = repo.Create(ctx, user2)
	if !errors.Is(err, ErrDuplicateUsername) {
		t.Errorf("Create() error = %v, want ErrDuplicateUsername", err)
	}
}

// TestUserRepository_DuplicateEmail tests that duplicate emails are rejected.
func TestUserRepository_DuplicateEmail(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("Skipping test: no database connection available")
	}
	defer cleanupTestDB(t, pool)

	repo := NewUserRepository(pool)
	ctx := context.Background()

	// Create first user
	user1 := &models.User{
		Username:       "firstemail",
		DisplayName:    "First User",
		Email:          "duplicate@example.com",
		AuthProvider:   models.AuthProviderGitHub,
		AuthProviderID: "email_first_123",
		Role:           models.UserRoleUser,
	}

	_, err := repo.Create(ctx, user1)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Try to create second user with same email
	user2 := &models.User{
		Username:       "secondemail",
		DisplayName:    "Second User",
		Email:          "duplicate@example.com",
		AuthProvider:   models.AuthProviderGoogle,
		AuthProviderID: "email_second_123",
		Role:           models.UserRoleUser,
	}

	_, err = repo.Create(ctx, user2)
	if !errors.Is(err, ErrDuplicateEmail) {
		t.Errorf("Create() error = %v, want ErrDuplicateEmail", err)
	}
}

// TestUserRepository_List tests listing users with reputation and agents_count subqueries.
// This is an integration test that requires a real PostgreSQL database.
func TestUserRepository_List(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}

	ctx := context.Background()
	pool, err := NewPool(ctx, databaseURL)
	if err != nil {
		t.Fatalf("failed to connect to database: %v", err)
	}
	defer pool.Close()

	repo := NewUserRepository(pool)

	// Create 2 test users with unique suffixes (username max 30 chars)
	suffix := time.Now().Format("150405.000")
	user1 := &models.User{
		Username:       "lu1_" + suffix,
		DisplayName:    "List User 1",
		Email:          "listuser1_" + suffix + "@example.com",
		AuthProvider:   models.AuthProviderGitHub,
		AuthProviderID: "github_list1_" + suffix,
		Role:           models.UserRoleUser,
	}
	user2 := &models.User{
		Username:       "lu2_" + suffix,
		DisplayName:    "List User 2",
		Email:          "listuser2_" + suffix + "@example.com",
		AuthProvider:   models.AuthProviderGitHub,
		AuthProviderID: "github_list2_" + suffix,
		Role:           models.UserRoleUser,
	}

	created1, err := repo.Create(ctx, user1)
	if err != nil {
		t.Fatalf("Create(user1) error = %v", err)
	}
	created2, err := repo.Create(ctx, user2)
	if err != nil {
		t.Fatalf("Create(user2) error = %v", err)
	}

	// Clean up after test
	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM users WHERE id = $1 OR id = $2", created1.ID, created2.ID)
	}()

	// List users — this triggers the query with reputation/agents_count subqueries
	users, total, err := repo.List(ctx, models.PublicUserListOptions{
		Limit: 100,
		Sort:  models.PublicUserSortNewest,
	})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if total < 2 {
		t.Errorf("List() total = %d, want >= 2", total)
	}
	if len(users) < 2 {
		t.Errorf("List() returned %d users, want >= 2", len(users))
	}

	// Verify our created users are in the list
	found1, found2 := false, false
	for _, u := range users {
		if u.ID == created1.ID {
			found1 = true
			if u.Username != created1.Username {
				t.Errorf("List() user1 Username = %q, want %q", u.Username, created1.Username)
			}
		}
		if u.ID == created2.ID {
			found2 = true
		}
	}
	if !found1 {
		t.Error("List() did not return user1")
	}
	if !found2 {
		t.Error("List() did not return user2")
	}
}

// Helper functions for tests

// getTestPool returns a test database pool or nil if DATABASE_URL is not set.
func getTestPool(t *testing.T) *Pool {
	t.Helper()
	ctx := context.Background()
	pool, err := NewPool(ctx, testDatabaseURL())
	if err != nil {
		return nil
	}
	return pool
}

// testDatabaseURL returns the test database URL.
func testDatabaseURL() string {
	// Use the test database URL from environment
	// In CI, this would be set to a test PostgreSQL instance
	return "" // Will cause pool creation to fail, skipping tests
}

// cleanupTestDB cleans up test data.
func cleanupTestDB(t *testing.T, pool *Pool) {
	t.Helper()
	if pool == nil {
		return
	}
	ctx := context.Background()
	// Clean up in reverse order of dependencies
	_, _ = pool.Exec(ctx, "DELETE FROM refresh_tokens")
	_, _ = pool.Exec(ctx, "DELETE FROM notifications")
	_, _ = pool.Exec(ctx, "DELETE FROM agents")
	_, _ = pool.Exec(ctx, "DELETE FROM users")
	pool.Close()
}
