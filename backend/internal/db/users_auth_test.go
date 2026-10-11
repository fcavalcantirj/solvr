package db

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// TestUserRepository_CreateEmailUser_WithPasswordHash tests creating a user with email/password auth.
// This test validates that the password_hash column exists and can store bcrypt hashes.
func TestUserRepository_CreateEmailUser_WithPasswordHash(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("Skipping test: no database connection available")
	}
	defer cleanupTestDB(t, pool)

	repo := NewUserRepository(pool)
	ctx := context.Background()

	// Create a user with email/password authentication
	// Note: In production, this would use bcrypt.GenerateFromPassword()
	// For testing, we use a pre-generated bcrypt hash of "testpassword123"
	bcryptHash := "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy"

	user := &models.User{
		Username:     "emailuser",
		DisplayName:  "Email User",
		Email:        "email@example.com",
		AuthProvider: "email", // New auth provider type
		PasswordHash: bcryptHash,
		Role:         models.UserRoleUser,
	}

	created, err := repo.Create(ctx, user)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if created.ID == "" {
		t.Error("Create() did not set ID")
	}
	if created.PasswordHash != bcryptHash {
		t.Errorf("Create() PasswordHash = %q, want %q", created.PasswordHash, bcryptHash)
	}
	if created.AuthProvider != "email" {
		t.Errorf("Create() AuthProvider = %q, want %q", created.AuthProvider, "email")
	}

	// Cleanup
	_, _ = pool.Exec(ctx, "DELETE FROM users WHERE id = $1", created.ID)
}

// TestUserRepository_CreateOAuthUser_Unaffected tests that OAuth users still work without password_hash.
func TestUserRepository_CreateOAuthUser_Unaffected(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("Skipping test: no database connection available")
	}
	defer cleanupTestDB(t, pool)

	repo := NewUserRepository(pool)
	ctx := context.Background()

	// Create a traditional OAuth user (no password)
	user := &models.User{
		Username:       "oauthuser",
		DisplayName:    "OAuth User",
		Email:          "oauth@example.com",
		AuthProvider:   models.AuthProviderGitHub,
		AuthProviderID: "github123",
		Role:           models.UserRoleUser,
		// PasswordHash intentionally left empty
	}

	created, err := repo.Create(ctx, user)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if created.ID == "" {
		t.Error("Create() did not set ID")
	}
	if created.PasswordHash != "" {
		t.Errorf("Create() PasswordHash should be empty for OAuth user, got %q", created.PasswordHash)
	}
	if created.AuthProvider != models.AuthProviderGitHub {
		t.Errorf("Create() AuthProvider = %q, want %q", created.AuthProvider, models.AuthProviderGitHub)
	}
	if created.AuthProviderID != "github123" {
		t.Errorf("Create() AuthProviderID = %q, want %q", created.AuthProviderID, "github123")
	}

	// Cleanup
	_, _ = pool.Exec(ctx, "DELETE FROM users WHERE id = $1", created.ID)
}

// TestUserRepository_FindByEmail_ReturnsPasswordHash tests that queries include password_hash.
func TestUserRepository_FindByEmail_ReturnsPasswordHash(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("Skipping test: no database connection available")
	}
	defer cleanupTestDB(t, pool)

	repo := NewUserRepository(pool)
	ctx := context.Background()

	bcryptHash := "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy"

	// Create email/password user
	user := &models.User{
		Username:     "findtest",
		DisplayName:  "Find Test",
		Email:        "findtest@example.com",
		AuthProvider: "email",
		PasswordHash: bcryptHash,
		Role:         models.UserRoleUser,
	}

	created, err := repo.Create(ctx, user)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM users WHERE id = $1", created.ID)
	}()

	// Find by email and verify password_hash is returned
	found, err := repo.FindByEmail(ctx, "findtest@example.com")
	if err != nil {
		t.Fatalf("FindByEmail() error = %v", err)
	}

	if found.PasswordHash != bcryptHash {
		t.Errorf("FindByEmail() PasswordHash = %q, want %q", found.PasswordHash, bcryptHash)
	}
}

// TestUserRepository_MixedAuthFields tests that users can have both OAuth and password (future-proofing).
func TestUserRepository_MixedAuthFields(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("Skipping test: no database connection available")
	}
	defer cleanupTestDB(t, pool)

	repo := NewUserRepository(pool)
	ctx := context.Background()

	bcryptHash := "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy"

	// Create user with BOTH OAuth and password
	// This allows OAuth users to add password backup authentication later
	user := &models.User{
		Username:       "mixeduser",
		DisplayName:    "Mixed Auth User",
		Email:          "mixed@example.com",
		AuthProvider:   models.AuthProviderGitHub,
		AuthProviderID: "github456",
		PasswordHash:   bcryptHash,
		Role:           models.UserRoleUser,
	}

	created, err := repo.Create(ctx, user)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if created.PasswordHash != bcryptHash {
		t.Errorf("Create() PasswordHash = %q, want %q", created.PasswordHash, bcryptHash)
	}
	if created.AuthProvider != models.AuthProviderGitHub {
		t.Errorf("Create() AuthProvider = %q, want %q", created.AuthProvider, models.AuthProviderGitHub)
	}

	// Cleanup
	_, _ = pool.Exec(ctx, "DELETE FROM users WHERE id = $1", created.ID)
}

// TestUser_JSONSerializationSecurity verifies that password_hash is NEVER serialized to JSON.
// This is a critical security test - password hashes must never leak via API responses.
func TestUser_JSONSerializationSecurity(t *testing.T) {
	user := &models.User{
		ID:           "test-id",
		Username:     "testuser",
		DisplayName:  "Test User",
		Email:        "test@example.com",
		AuthProvider: models.AuthProviderEmail,
		PasswordHash: "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy",
		Role:         models.UserRoleUser,
		Status:       "active",
	}

	// Marshal to JSON
	jsonBytes, err := json.Marshal(user)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	jsonStr := string(jsonBytes)

	// Verify password_hash is NOT in JSON output
	if strings.Contains(jsonStr, "password_hash") {
		t.Errorf("JSON output contains 'password_hash' field - SECURITY VIOLATION!\nJSON: %s", jsonStr)
	}
	if strings.Contains(jsonStr, user.PasswordHash) {
		t.Errorf("JSON output contains password hash value - SECURITY VIOLATION!\nJSON: %s", jsonStr)
	}

	// Verify other fields ARE present (sanity check)
	if !strings.Contains(jsonStr, user.Username) {
		t.Error("JSON output missing username - serialization broken")
	}
	if !strings.Contains(jsonStr, user.Email) {
		t.Error("JSON output missing email - serialization broken")
	}
}

// TestUserRepository_FindByAuthProvider_NullPasswordHash tests that we can scan
// OAuth users who have NULL password_hash (they don't use password auth).
// This is a regression test for the bug: "can't scan into dest[6]: cannot scan NULL into *string"
// Per TDD: This test ensures OAuth users with NULL password_hash can be scanned successfully.
func TestUserRepository_FindByAuthProvider_NullPasswordHash(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("Skipping test: no database connection available")
	}
	defer cleanupTestDB(t, pool)

	ctx := context.Background()

	// Insert an OAuth user directly into DB with NULL password_hash
	// This simulates what happens in production for OAuth users
	insertQuery := `
		INSERT INTO users (username, display_name, email, auth_provider, auth_provider_id, password_hash, role)
		VALUES ($1, $2, $3, $4, $5, NULL, $6)
		RETURNING id
	`

	var userID string
	err := pool.QueryRow(ctx, insertQuery,
		"oauth_null_test",
		"OAuth Null Test User",
		"oauth_null@example.com",
		models.AuthProviderGoogle,
		"google_null_123",
		models.UserRoleUser,
	).Scan(&userID)

	if err != nil {
		t.Fatalf("Failed to insert OAuth user with NULL password_hash: %v", err)
	}

	// Insert auth_methods record (FindByAuthProvider queries this table via INNER JOIN)
	_, err = pool.Exec(ctx,
		`INSERT INTO auth_methods (user_id, auth_provider, auth_provider_id) VALUES ($1, $2, $3)`,
		userID, models.AuthProviderGoogle, "google_null_123",
	)
	if err != nil {
		t.Fatalf("Failed to insert auth_methods record: %v", err)
	}

	// Now try to find this user by auth provider
	// This should NOT fail with "can't scan into dest[6]: cannot scan NULL into *string"
	repo := NewUserRepository(pool)
	foundUser, err := repo.FindByAuthProvider(ctx, models.AuthProviderGoogle, "google_null_123")
	if err != nil {
		t.Fatalf("FindByAuthProvider failed with NULL password_hash: %v", err)
	}

	if foundUser == nil {
		t.Fatal("Expected user to be found, got nil")
	}

	// Verify user fields
	if foundUser.ID != userID {
		t.Errorf("Expected user ID %s, got %s", userID, foundUser.ID)
	}

	if foundUser.Username != "oauth_null_test" {
		t.Errorf("Expected username 'oauth_null_test', got '%s'", foundUser.Username)
	}

	if foundUser.Email != "oauth_null@example.com" {
		t.Errorf("Expected email 'oauth_null@example.com', got '%s'", foundUser.Email)
	}

	// Password hash should be empty string (not cause a scan error)
	if foundUser.PasswordHash != "" {
		t.Errorf("Expected empty password hash for OAuth user, got '%s'", foundUser.PasswordHash)
	}

	t.Logf("SUCCESS: Can scan OAuth user with NULL password_hash (no scan error)")
}

// TestUserRepository_FindByAuthProvider_WithAuthMethodsTable tests finding user via auth_methods table.
// This test validates the fix for GitHub OAuth login failure.
// Users created with NULL auth_provider/auth_provider_id (post-migration 000032) should be
// findable via their auth_methods records.
// This is an integration test that requires a real PostgreSQL database.
func TestUserRepository_FindByAuthProvider_WithAuthMethodsTable(t *testing.T) {
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

	userRepo := NewUserRepository(pool)
	authMethodRepo := NewAuthMethodRepository(pool)
	suffix := time.Now().Format("150405.000")

	// Create user with NULL auth_provider/auth_provider_id (post-migration style)
	user := &models.User{
		Username:    "authmethoduser_" + suffix,
		DisplayName: "Auth Method User",
		Email:       "authmethod_" + suffix + "@example.com",
		Role:        models.UserRoleUser,
		// Note: auth_provider and auth_provider_id are NULL
	}

	created, err := userRepo.Create(ctx, user)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Clean up after test
	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM auth_methods WHERE user_id = $1", created.ID)
		_, _ = pool.Exec(ctx, "DELETE FROM users WHERE id = $1", created.ID)
	}()

	// Insert auth_methods record (this is how OAuth data is stored post-migration)
	authMethod := &models.AuthMethod{
		UserID:         created.ID,
		AuthProvider:   models.AuthProviderGitHub,
		AuthProviderID: "github_123",
	}
	_, err = authMethodRepo.Create(ctx, authMethod)
	if err != nil {
		t.Fatalf("AuthMethod.Create() error = %v", err)
	}

	// Query by OAuth provider (should find user via auth_methods JOIN)
	found, err := userRepo.FindByAuthProvider(ctx, models.AuthProviderGitHub, "github_123")
	if err != nil {
		t.Fatalf("FindByAuthProvider() error = %v, want success", err)
	}

	if found.ID != created.ID {
		t.Errorf("FindByAuthProvider() ID = %v, want %v", found.ID, created.ID)
	}
	if found.Email != created.Email {
		t.Errorf("FindByAuthProvider() Email = %v, want %v", found.Email, created.Email)
	}
}

// TestUserRepository_FindByAuthProvider_MultipleAuthMethods tests finding user with multiple auth methods.
// Verifies that a user with both GitHub and Google OAuth can be found by either provider.
// This is an integration test that requires a real PostgreSQL database.
func TestUserRepository_FindByAuthProvider_MultipleAuthMethods(t *testing.T) {
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

	userRepo := NewUserRepository(pool)
	authMethodRepo := NewAuthMethodRepository(pool)
	suffix := time.Now().Format("150405.000")

	// Create user
	user := &models.User{
		Username:    "multiauth_" + suffix,
		DisplayName: "Multi Auth User",
		Email:       "multiauth_" + suffix + "@example.com",
		Role:        models.UserRoleUser,
	}

	created, err := userRepo.Create(ctx, user)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Clean up after test
	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM auth_methods WHERE user_id = $1", created.ID)
		_, _ = pool.Exec(ctx, "DELETE FROM users WHERE id = $1", created.ID)
	}()

	// Add GitHub auth method
	githubMethod := &models.AuthMethod{
		UserID:         created.ID,
		AuthProvider:   models.AuthProviderGitHub,
		AuthProviderID: "gh_123",
	}
	_, err = authMethodRepo.Create(ctx, githubMethod)
	if err != nil {
		t.Fatalf("Create GitHub auth method error = %v", err)
	}

	// Add Google auth method
	googleMethod := &models.AuthMethod{
		UserID:         created.ID,
		AuthProvider:   models.AuthProviderGoogle,
		AuthProviderID: "gg_456",
	}
	_, err = authMethodRepo.Create(ctx, googleMethod)
	if err != nil {
		t.Fatalf("Create Google auth method error = %v", err)
	}

	// Should be able to find user by GitHub
	foundGH, err := userRepo.FindByAuthProvider(ctx, models.AuthProviderGitHub, "gh_123")
	if err != nil {
		t.Errorf("FindByAuthProvider(github) error = %v", err)
	} else if foundGH.ID != created.ID {
		t.Errorf("FindByAuthProvider(github) ID = %v, want %v", foundGH.ID, created.ID)
	}

	// Should be able to find user by Google
	foundGG, err := userRepo.FindByAuthProvider(ctx, models.AuthProviderGoogle, "gg_456")
	if err != nil {
		t.Errorf("FindByAuthProvider(google) error = %v", err)
	} else if foundGG.ID != created.ID {
		t.Errorf("FindByAuthProvider(google) ID = %v, want %v", foundGG.ID, created.ID)
	}
}

// TestUserRepository_FindByAuthProvider_DeletedAuthMethod tests that deleted auth methods return ErrNotFound.
// Verifies that when an auth method is deleted (user unlinks provider), the query correctly fails.
// This is an integration test that requires a real PostgreSQL database.
func TestUserRepository_FindByAuthProvider_DeletedAuthMethod(t *testing.T) {
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

	userRepo := NewUserRepository(pool)
	authMethodRepo := NewAuthMethodRepository(pool)
	suffix := time.Now().Format("150405.000")

	// Create user
	user := &models.User{
		Username:    "deleteauth_" + suffix,
		DisplayName: "Delete Auth User",
		Email:       "deleteauth_" + suffix + "@example.com",
		Role:        models.UserRoleUser,
	}

	created, err := userRepo.Create(ctx, user)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Clean up after test
	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM auth_methods WHERE user_id = $1", created.ID)
		_, _ = pool.Exec(ctx, "DELETE FROM users WHERE id = $1", created.ID)
	}()

	// Add GitHub auth method
	githubMethod := &models.AuthMethod{
		UserID:         created.ID,
		AuthProvider:   models.AuthProviderGitHub,
		AuthProviderID: "gh_123",
	}
	createdMethod, err := authMethodRepo.Create(ctx, githubMethod)
	if err != nil {
		t.Fatalf("Create auth method error = %v", err)
	}

	// Verify we can find the user
	found, err := userRepo.FindByAuthProvider(ctx, models.AuthProviderGitHub, "gh_123")
	if err != nil {
		t.Fatalf("FindByAuthProvider() before delete error = %v", err)
	}
	if found.ID != created.ID {
		t.Errorf("FindByAuthProvider() before delete ID = %v, want %v", found.ID, created.ID)
	}

	// Delete the auth method (unlink)
	err = authMethodRepo.Delete(ctx, createdMethod.ID)
	if err != nil {
		t.Fatalf("Delete auth method error = %v", err)
	}

	// Should NOT find user after deletion
	_, err = userRepo.FindByAuthProvider(ctx, models.AuthProviderGitHub, "gh_123")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("FindByAuthProvider() after delete error = %v, want ErrNotFound", err)
	}
}
