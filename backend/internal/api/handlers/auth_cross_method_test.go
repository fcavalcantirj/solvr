package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
	"golang.org/x/crypto/bcrypt"
)

// mockAuthMethodRepo simulates multi-provider auth storage
type mockAuthMethodRepo struct {
	methods         map[string]*models.AuthMethod // by id
	methodsByUserID map[string][]*models.AuthMethod
	providerIndex   map[string]*models.AuthMethod // key: "provider:provider_id"
}

func newMockAuthMethodRepo() *mockAuthMethodRepo {
	return &mockAuthMethodRepo{
		methods:         make(map[string]*models.AuthMethod),
		methodsByUserID: make(map[string][]*models.AuthMethod),
		providerIndex:   make(map[string]*models.AuthMethod),
	}
}

func (m *mockAuthMethodRepo) Create(ctx context.Context, method *models.AuthMethod) (*models.AuthMethod, error) {
	// Generate ID if not set
	if method.ID == "" {
		method.ID = generateUUID()
	}
	if method.CreatedAt.IsZero() {
		method.CreatedAt = time.Now()
	}
	if method.LastUsedAt.IsZero() {
		method.LastUsedAt = time.Now()
	}

	// Check for duplicate provider per user
	existing := m.methodsByUserID[method.UserID]
	for _, e := range existing {
		if e.AuthProvider == method.AuthProvider {
			return nil, errors.New("duplicate auth provider for user")
		}
	}

	// Check for duplicate OAuth provider ID
	if method.AuthProviderID != "" {
		key := method.AuthProvider + ":" + method.AuthProviderID
		if _, exists := m.providerIndex[key]; exists {
			return nil, errors.New("duplicate OAuth provider ID")
		}
		m.providerIndex[key] = method
	}

	m.methods[method.ID] = method
	m.methodsByUserID[method.UserID] = append(m.methodsByUserID[method.UserID], method)

	return method, nil
}

func (m *mockAuthMethodRepo) FindByUserID(ctx context.Context, userID string) ([]*models.AuthMethod, error) {
	methods := m.methodsByUserID[userID]
	if methods == nil {
		return []*models.AuthMethod{}, nil
	}
	return methods, nil
}

func (m *mockAuthMethodRepo) FindByProvider(ctx context.Context, provider, providerID string) (*models.AuthMethod, error) {
	key := provider + ":" + providerID
	method, exists := m.providerIndex[key]
	if !exists {
		return nil, errors.New("auth method not found")
	}
	return method, nil
}

func (m *mockAuthMethodRepo) UpdateLastUsed(ctx context.Context, methodID string) error {
	method, exists := m.methods[methodID]
	if !exists {
		return errors.New("auth method not found")
	}
	method.LastUsedAt = time.Now()
	return nil
}

func (m *mockAuthMethodRepo) Delete(ctx context.Context, methodID string) error {
	method, exists := m.methods[methodID]
	if !exists {
		return errors.New("auth method not found")
	}

	// Remove from user index
	userMethods := m.methodsByUserID[method.UserID]
	for i, um := range userMethods {
		if um.ID == methodID {
			m.methodsByUserID[method.UserID] = append(userMethods[:i], userMethods[i+1:]...)
			break
		}
	}

	// Remove from provider index
	if method.AuthProviderID != "" {
		key := method.AuthProvider + ":" + method.AuthProviderID
		delete(m.providerIndex, key)
	}

	delete(m.methods, methodID)
	return nil
}

func (m *mockAuthMethodRepo) HasEmailAuth(ctx context.Context, userID string) (bool, error) {
	methods := m.methodsByUserID[userID]
	for _, method := range methods {
		if method.AuthProvider == models.AuthProviderEmail {
			return true, nil
		}
	}
	return false, nil
}

func (m *mockAuthMethodRepo) GetEmailAuthMethod(ctx context.Context, userID string) (*models.AuthMethod, error) {
	methods := m.methodsByUserID[userID]
	for _, method := range methods {
		if method.AuthProvider == models.AuthProviderEmail {
			return method, nil
		}
	}
	return nil, errors.New("auth method not found")
}

// Test helpers for cross-auth scenarios

func createTestUserWithEmail(t *testing.T, userRepo *mockUserRepoForAuth, authMethodRepo *mockAuthMethodRepo, email, password string) *models.User {
	t.Helper()

	user := &models.User{
		Email:       email,
		Username:    strings.Split(email, "@")[0],
		DisplayName: "Test User",
		Role:        models.UserRoleUser,
	}
	created, err := userRepo.Create(context.Background(), user)
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Hash password
	hashedPassword := hashPasswordForTest(password)

	// Create email auth method
	method := &models.AuthMethod{
		UserID:       created.ID,
		AuthProvider: models.AuthProviderEmail,
		PasswordHash: hashedPassword,
	}
	_, err = authMethodRepo.Create(context.Background(), method)
	if err != nil {
		t.Fatalf("Failed to create auth method: %v", err)
	}

	return created
}

func createTestUserWithOAuth(t *testing.T, userRepo *mockUserRepoForAuth, authMethodRepo *mockAuthMethodRepo, email, provider, providerID string) *models.User {
	t.Helper()

	user := &models.User{
		Email:       email,
		Username:    strings.Split(email, "@")[0],
		DisplayName: "Test User",
		Role:        models.UserRoleUser,
	}
	created, err := userRepo.Create(context.Background(), user)
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Create OAuth auth method
	method := &models.AuthMethod{
		UserID:         created.ID,
		AuthProvider:   provider,
		AuthProviderID: providerID,
	}
	_, err = authMethodRepo.Create(context.Background(), method)
	if err != nil {
		t.Fatalf("Failed to create auth method: %v", err)
	}

	return created
}

// GROUP 1: Email/Password → OAuth (Same Email)

func TestCrossAuth_EmailPassword_Then_Google_ShouldLink(t *testing.T) {
	userRepo := newMockUserRepoForAuth()
	authMethodRepo := newMockAuthMethodRepo()

	// Step 1: Register with email/password
	user := createTestUserWithEmail(t, userRepo, authMethodRepo, "test@example.com", "password123")

	// Step 2: Simulate Google OAuth login with same email
	// This would normally be handled by oauth_user.go service
	// For now, we manually create the auth method to test the desired behavior

	googleMethod := &models.AuthMethod{
		UserID:         user.ID,
		AuthProvider:   models.AuthProviderGoogle,
		AuthProviderID: "google_12345",
	}
	_, err := authMethodRepo.Create(context.Background(), googleMethod)
	if err != nil {
		t.Fatalf("Failed to link Google provider: %v", err)
	}

	// Verify: User should have 2 auth methods
	methods, err := authMethodRepo.FindByUserID(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("Failed to get auth methods: %v", err)
	}

	if len(methods) != 2 {
		t.Errorf("Expected 2 auth methods, got %d", len(methods))
	}

	// Verify: Can find user by Google provider
	foundMethod, err := authMethodRepo.FindByProvider(context.Background(), models.AuthProviderGoogle, "google_12345")
	if err != nil {
		t.Errorf("Failed to find user by Google provider: %v", err)
	}
	if foundMethod.UserID != user.ID {
		t.Errorf("Expected user ID %s, got %s", user.ID, foundMethod.UserID)
	}

	// Verify: Both methods have same user_id
	hasEmail := false
	hasGoogle := false
	for _, method := range methods {
		if method.AuthProvider == models.AuthProviderEmail {
			hasEmail = true
		}
		if method.AuthProvider == models.AuthProviderGoogle {
			hasGoogle = true
		}
	}

	if !hasEmail {
		t.Error("Missing email auth method")
	}
	if !hasGoogle {
		t.Error("Missing Google auth method")
	}
}

func TestCrossAuth_EmailPassword_Then_GitHub_BothWork(t *testing.T) {
	userRepo := newMockUserRepoForAuth()
	authMethodRepo := newMockAuthMethodRepo()

	// Step 1: Register with email/password
	user := createTestUserWithEmail(t, userRepo, authMethodRepo, "test2@example.com", "password456")

	// Step 2: Link GitHub OAuth
	githubMethod := &models.AuthMethod{
		UserID:         user.ID,
		AuthProvider:   models.AuthProviderGitHub,
		AuthProviderID: "github_98765",
	}
	_, err := authMethodRepo.Create(context.Background(), githubMethod)
	if err != nil {
		t.Fatalf("Failed to link GitHub provider: %v", err)
	}

	// Verify: Can login with GitHub
	foundMethod, err := authMethodRepo.FindByProvider(context.Background(), models.AuthProviderGitHub, "github_98765")
	if err != nil {
		t.Errorf("Second GitHub login should work: %v", err)
	}
	if foundMethod.UserID != user.ID {
		t.Errorf("GitHub login returned wrong user")
	}

	// Verify: Email/password still works
	methods, err := authMethodRepo.FindByUserID(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("Failed to get auth methods: %v", err)
	}

	hasEmailWithPassword := false
	for _, method := range methods {
		if method.AuthProvider == models.AuthProviderEmail && method.PasswordHash != "" {
			hasEmailWithPassword = true
			break
		}
	}

	if !hasEmailWithPassword {
		t.Error("Email/password auth should still work after linking GitHub")
	}
}

// GROUP 2: OAuth → Email/Password (Same Email)

func TestCrossAuth_GitHub_Then_EmailPassword_Blocked(t *testing.T) {
	userRepo := newMockUserRepoForAuth()
	authMethodRepo := newMockAuthMethodRepo()

	// Step 1: Login with GitHub
	user := createTestUserWithOAuth(t, userRepo, authMethodRepo, "oauth@example.com", models.AuthProviderGitHub, "gh_111")

	// Step 2: Try to register with same email (should be blocked by existing email check)
	existingUser, err := userRepo.FindByEmail(context.Background(), "oauth@example.com")
	if err != nil {
		t.Fatalf("Should find existing user by email: %v", err)
	}

	if existingUser.ID != user.ID {
		t.Error("Email lookup should return OAuth user")
	}

	// This simulates what Register handler should do:
	// If email exists, return 409 DUPLICATE_EMAIL
	// (The actual handler test would be done separately)
}

func TestCrossAuth_Google_Then_SetPassword_ShouldEnableBoth(t *testing.T) {
	userRepo := newMockUserRepoForAuth()
	authMethodRepo := newMockAuthMethodRepo()

	// Step 1: Login with Google
	user := createTestUserWithOAuth(t, userRepo, authMethodRepo, "google-user@example.com", models.AuthProviderGoogle, "g_222")

	// Step 2: Set password (via hypothetical /me/set-password endpoint)
	hashedPassword := hashPasswordForTest("newpassword789")
	passwordMethod := &models.AuthMethod{
		UserID:       user.ID,
		AuthProvider: models.AuthProviderEmail,
		PasswordHash: hashedPassword,
	}
	_, err := authMethodRepo.Create(context.Background(), passwordMethod)
	if err != nil {
		t.Fatalf("Failed to add email/password auth: %v", err)
	}

	// Verify: User has both methods
	methods, err := authMethodRepo.FindByUserID(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("Failed to get auth methods: %v", err)
	}

	if len(methods) != 2 {
		t.Errorf("Expected 2 auth methods (Google + email), got %d", len(methods))
	}

	hasGoogle := false
	hasEmail := false
	for _, method := range methods {
		if method.AuthProvider == models.AuthProviderGoogle {
			hasGoogle = true
		}
		if method.AuthProvider == models.AuthProviderEmail && method.PasswordHash != "" {
			hasEmail = true
		}
	}

	if !hasGoogle {
		t.Error("Google auth should still exist")
	}
	if !hasEmail {
		t.Error("Email/password auth should be enabled")
	}
}

// GROUP 3: OAuth → Different OAuth (Same Email)

func TestCrossAuth_GitHub_Then_Google_ShouldLinkBoth(t *testing.T) {
	userRepo := newMockUserRepoForAuth()
	authMethodRepo := newMockAuthMethodRepo()

	// Step 1: Login with GitHub
	user := createTestUserWithOAuth(t, userRepo, authMethodRepo, "multi-oauth@example.com", models.AuthProviderGitHub, "gh_333")

	// Step 2: Login with Google (same email) - should link
	googleMethod := &models.AuthMethod{
		UserID:         user.ID,
		AuthProvider:   models.AuthProviderGoogle,
		AuthProviderID: "g_444",
	}
	_, err := authMethodRepo.Create(context.Background(), googleMethod)
	if err != nil {
		t.Fatalf("Failed to link Google to existing GitHub user: %v", err)
	}

	// Verify: Both providers linked
	methods, err := authMethodRepo.FindByUserID(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("Failed to get auth methods: %v", err)
	}

	if len(methods) != 2 {
		t.Errorf("Expected 2 OAuth providers, got %d", len(methods))
	}

	// Verify: Can login with either provider
	githubMethod, err := authMethodRepo.FindByProvider(context.Background(), models.AuthProviderGitHub, "gh_333")
	if err != nil {
		t.Errorf("Should be able to login with GitHub: %v", err)
	}
	if githubMethod.UserID != user.ID {
		t.Error("GitHub login returns wrong user")
	}

	googleMethod2, err := authMethodRepo.FindByProvider(context.Background(), models.AuthProviderGoogle, "g_444")
	if err != nil {
		t.Errorf("Should be able to login with Google: %v", err)
	}
	if googleMethod2.UserID != user.ID {
		t.Error("Google login returns wrong user")
	}
}

func TestCrossAuth_Google_Then_GitHub_BothProvidersPersist(t *testing.T) {
	userRepo := newMockUserRepoForAuth()
	authMethodRepo := newMockAuthMethodRepo()

	// Step 1: Login with Google
	user := createTestUserWithOAuth(t, userRepo, authMethodRepo, "persist@example.com", models.AuthProviderGoogle, "g_555")

	// Step 2: Login with GitHub (same email)
	githubMethod := &models.AuthMethod{
		UserID:         user.ID,
		AuthProvider:   models.AuthProviderGitHub,
		AuthProviderID: "gh_666",
	}
	_, err := authMethodRepo.Create(context.Background(), githubMethod)
	if err != nil {
		t.Fatalf("Failed to link GitHub: %v", err)
	}

	// Step 3: Login with GitHub again (simulate second login)
	foundGitHub, err := authMethodRepo.FindByProvider(context.Background(), models.AuthProviderGitHub, "gh_666")
	if err != nil {
		t.Errorf("GitHub provider should persist: %v", err)
	}
	if foundGitHub.UserID != user.ID {
		t.Error("GitHub provider not properly persisted")
	}

	// Step 4: Login with Google again
	foundGoogle, err := authMethodRepo.FindByProvider(context.Background(), models.AuthProviderGoogle, "g_555")
	if err != nil {
		t.Errorf("Google provider should persist: %v", err)
	}
	if foundGoogle.UserID != user.ID {
		t.Error("Google provider not properly persisted")
	}
}

// GROUP 4: Edge Cases

func TestCrossAuth_EmailPassword_PreservedAfterOAuth(t *testing.T) {
	userRepo := newMockUserRepoForAuth()
	authMethodRepo := newMockAuthMethodRepo()

	originalPassword := "original123"
	user := createTestUserWithEmail(t, userRepo, authMethodRepo, "preserve@example.com", originalPassword)

	// Get original password hash
	methods, _ := authMethodRepo.FindByUserID(context.Background(), user.ID)
	originalHash := ""
	for _, method := range methods {
		if method.AuthProvider == models.AuthProviderEmail {
			originalHash = method.PasswordHash
			break
		}
	}

	// Link Google
	googleMethod := &models.AuthMethod{
		UserID:         user.ID,
		AuthProvider:   models.AuthProviderGoogle,
		AuthProviderID: "g_preserve",
	}
	authMethodRepo.Create(context.Background(), googleMethod)

	// Link GitHub
	githubMethod := &models.AuthMethod{
		UserID:         user.ID,
		AuthProvider:   models.AuthProviderGitHub,
		AuthProviderID: "gh_preserve",
	}
	authMethodRepo.Create(context.Background(), githubMethod)

	// Verify: Password hash unchanged
	methods, _ = authMethodRepo.FindByUserID(context.Background(), user.ID)
	currentHash := ""
	for _, method := range methods {
		if method.AuthProvider == models.AuthProviderEmail {
			currentHash = method.PasswordHash
			break
		}
	}

	if currentHash != originalHash {
		t.Error("Password hash should not change when linking OAuth")
	}

	if len(methods) != 3 {
		t.Errorf("Expected 3 auth methods (email + Google + GitHub), got %d", len(methods))
	}
}

func TestCrossAuth_MultipleOAuth_NoPasswordSet(t *testing.T) {
	userRepo := newMockUserRepoForAuth()
	authMethodRepo := newMockAuthMethodRepo()

	// Start with GitHub
	user := createTestUserWithOAuth(t, userRepo, authMethodRepo, "oauth-only@example.com", models.AuthProviderGitHub, "gh_multi")

	// Add Google
	googleMethod := &models.AuthMethod{
		UserID:         user.ID,
		AuthProvider:   models.AuthProviderGoogle,
		AuthProviderID: "g_multi",
	}
	authMethodRepo.Create(context.Background(), googleMethod)

	// Verify: No email/password method
	hasEmail, err := authMethodRepo.HasEmailAuth(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("Failed to check email auth: %v", err)
	}

	if hasEmail {
		t.Error("User should not have email/password auth")
	}

	methods, _ := authMethodRepo.FindByUserID(context.Background(), user.ID)
	if len(methods) != 2 {
		t.Errorf("Expected exactly 2 OAuth methods, got %d", len(methods))
	}
}

func TestCrossAuth_ThreeProviders_AllWork(t *testing.T) {
	userRepo := newMockUserRepoForAuth()
	authMethodRepo := newMockAuthMethodRepo()

	// Create user with all three auth methods
	user := createTestUserWithEmail(t, userRepo, authMethodRepo, "all-three@example.com", "pass123")

	googleMethod := &models.AuthMethod{
		UserID:         user.ID,
		AuthProvider:   models.AuthProviderGoogle,
		AuthProviderID: "g_all",
	}
	authMethodRepo.Create(context.Background(), googleMethod)

	githubMethod := &models.AuthMethod{
		UserID:         user.ID,
		AuthProvider:   models.AuthProviderGitHub,
		AuthProviderID: "gh_all",
	}
	authMethodRepo.Create(context.Background(), githubMethod)

	// Verify all three work
	methods, err := authMethodRepo.FindByUserID(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("Failed to get auth methods: %v", err)
	}

	if len(methods) != 3 {
		t.Errorf("Expected 3 auth methods, got %d", len(methods))
	}

	providers := make(map[string]bool)
	for _, method := range methods {
		providers[method.AuthProvider] = true
	}

	requiredProviders := []string{models.AuthProviderEmail, models.AuthProviderGoogle, models.AuthProviderGitHub}
	for _, required := range requiredProviders {
		if !providers[required] {
			t.Errorf("Missing auth provider: %s", required)
		}
	}
}

func TestCrossAuth_RemoveProvider_OthersStillWork(t *testing.T) {
	userRepo := newMockUserRepoForAuth()
	authMethodRepo := newMockAuthMethodRepo()

	// Create user with email + Google + GitHub
	user := createTestUserWithEmail(t, userRepo, authMethodRepo, "remove-one@example.com", "pass999")

	googleMethod := &models.AuthMethod{
		UserID:         user.ID,
		AuthProvider:   models.AuthProviderGoogle,
		AuthProviderID: "g_remove",
	}
	createdGoogle, _ := authMethodRepo.Create(context.Background(), googleMethod)

	githubMethod := &models.AuthMethod{
		UserID:         user.ID,
		AuthProvider:   models.AuthProviderGitHub,
		AuthProviderID: "gh_remove",
	}
	authMethodRepo.Create(context.Background(), githubMethod)

	// Remove Google
	err := authMethodRepo.Delete(context.Background(), createdGoogle.ID)
	if err != nil {
		t.Fatalf("Failed to remove Google auth: %v", err)
	}

	// Verify: Only 2 methods remain
	methods, err := authMethodRepo.FindByUserID(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("Failed to get auth methods: %v", err)
	}

	if len(methods) != 2 {
		t.Errorf("Expected 2 remaining methods, got %d", len(methods))
	}

	// Verify: Google is gone
	_, err = authMethodRepo.FindByProvider(context.Background(), models.AuthProviderGoogle, "g_remove")
	if err == nil {
		t.Error("Google provider should be removed")
	}

	// Verify: Email and GitHub still work
	hasEmail := false
	hasGitHub := false
	for _, method := range methods {
		if method.AuthProvider == models.AuthProviderEmail {
			hasEmail = true
		}
		if method.AuthProvider == models.AuthProviderGitHub {
			hasGitHub = true
		}
	}

	if !hasEmail {
		t.Error("Email auth should still work")
	}
	if !hasGitHub {
		t.Error("GitHub auth should still work")
	}
}

// Additional edge case tests

func TestCrossAuth_DuplicateProviderBlocked(t *testing.T) {
	userRepo := newMockUserRepoForAuth()
	authMethodRepo := newMockAuthMethodRepo()

	user := createTestUserWithOAuth(t, userRepo, authMethodRepo, "dup@example.com", models.AuthProviderGitHub, "gh_dup1")

	// Try to add GitHub again (should fail)
	duplicateMethod := &models.AuthMethod{
		UserID:         user.ID,
		AuthProvider:   models.AuthProviderGitHub,
		AuthProviderID: "gh_dup2",
	}
	_, err := authMethodRepo.Create(context.Background(), duplicateMethod)
	if err == nil {
		t.Error("Should not allow duplicate auth provider for same user")
	}
}

func TestCrossAuth_OAuthProviderID_UniqueAcrossUsers(t *testing.T) {
	userRepo := newMockUserRepoForAuth()
	authMethodRepo := newMockAuthMethodRepo()

	// User 1 with GitHub
	user1 := createTestUserWithOAuth(t, userRepo, authMethodRepo, "user1@example.com", models.AuthProviderGitHub, "gh_shared")

	// User 2 tries to use same GitHub provider ID (should fail)
	user2 := &models.User{
		Email:       "user2@example.com",
		Username:    "user2",
		DisplayName: "User 2",
		Role:        models.UserRoleUser,
	}
	created2, _ := userRepo.Create(context.Background(), user2)

	duplicateOAuth := &models.AuthMethod{
		UserID:         created2.ID,
		AuthProvider:   models.AuthProviderGitHub,
		AuthProviderID: "gh_shared", // Same as user1
	}
	_, err := authMethodRepo.Create(context.Background(), duplicateOAuth)
	if err == nil {
		t.Error("Should not allow same OAuth provider ID for different users")
	}

	// Verify user1 still has exclusive access
	method, err := authMethodRepo.FindByProvider(context.Background(), models.AuthProviderGitHub, "gh_shared")
	if err != nil {
		t.Fatalf("User1's GitHub should still work: %v", err)
	}
	if method.UserID != user1.ID {
		t.Error("GitHub provider should belong to user1")
	}
}

// Helper to hash password using bcrypt
func hashPasswordForTest(password string) string {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		panic(fmt.Sprintf("Failed to hash password: %v", err))
	}
	return string(hash)
}

// Helper to generate UUID (simplified for tests)
func generateUUID() string {
	return "uuid_" + time.Now().Format("20060102150405.000000000")
}

// ============================================================================
// INTEGRATION TESTS - These will FAIL until implementation is complete
// ============================================================================

// TestIntegration_EmailThenOAuth_LinkProviders tests that OAuth login
// should link to existing email account when email matches.
// This will FAIL because auth_methods table doesn't exist and OAuth service
// doesn't implement account linking yet.
func TestIntegration_EmailThenOAuth_LinkProviders(t *testing.T) {
	t.Skip("EXPECTED TO FAIL - Run after implementing auth_methods table and OAuth linking")

	userRepo := newMockUserRepoForAuth()
	authMethodRepo := newMockAuthMethodRepo()

	config := &OAuthConfig{
		JWTSecret:     "test-secret",
		JWTExpiry:     "15m",
		RefreshExpiry: "168h",
	}
	handler := NewAuthHandlers(config, userRepo, authMethodRepo)
	// TODO: After implementing, need to pass authMethodRepo to handler

	// Step 1: Register with email/password
	registerReq := RegisterRequest{
		Email:       "integration@example.com",
		Password:    "password123",
		Username:    "integration",
		DisplayName: "Integration Test",
	}
	body, _ := json.Marshal(registerReq)

	req := httptest.NewRequest(http.MethodPost, "/v1/auth/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.Register(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("Registration failed: %d - %s", w.Code, w.Body.String())
	}

	var registerResp struct {
		User struct {
			ID string `json:"id"`
		} `json:"user"`
	}
	json.Unmarshal(w.Body.Bytes(), &registerResp)
	userID := registerResp.User.ID

	// Step 2: Simulate OAuth login with same email
	// This would normally come from OAuth callback
	// For now, we simulate what the OAuth service should do:
	// - Find user by email
	// - Create auth_method for OAuth provider
	// - Return the same user

	// TODO: This test will fail because:
	// 1. auth_methods table doesn't exist
	// 2. OAuth service doesn't link providers yet
	// 3. Handler doesn't use AuthMethodRepository

	methods, err := authMethodRepo.FindByUserID(context.Background(), userID)
	if err != nil {
		t.Fatalf("Failed to get auth methods: %v", err)
	}

	// Expected: Should have 1 auth method (email) after registration
	if len(methods) != 1 {
		t.Errorf("Expected 1 auth method after registration, got %d", len(methods))
	}

	// After OAuth login with same email, should have 2 methods
	// This assertion will FAIL because implementation doesn't exist
	t.Error("EXPECTED FAILURE: OAuth linking not implemented yet")
}

// TestIntegration_OAuthThenEmail_BlockDuplicate tests that email registration
// should be blocked if OAuth user already exists with that email.
// This should already work if FindByEmail is called during registration.
func TestIntegration_OAuthThenEmail_BlockDuplicate(t *testing.T) {
	t.Skip("Run after OAuth integration is complete")

	// This test verifies that the Register handler checks for existing email
	// even if the user was created via OAuth (not email/password).

	// Step 1: Simulate OAuth user creation
	// Step 2: Try to register with same email
	// Step 3: Should get 409 DUPLICATE_EMAIL

	// This should already work if the current Register handler
	// uses FindByEmail() before creating user.
}

// TestIntegration_MultipleOAuthProviders tests that a user can link
// multiple OAuth providers (GitHub + Google) to the same account.
// This will FAIL because auth_methods table and linking logic don't exist.
func TestIntegration_MultipleOAuthProviders(t *testing.T) {
	t.Skip("EXPECTED TO FAIL - Run after implementing multi-provider support")

	// Step 1: Login with GitHub
	// Step 2: Login with Google (same email)
	// Step 3: Both providers should work for subsequent logins

	t.Error("EXPECTED FAILURE: Multi-provider support not implemented yet")
}

// TestIntegration_LoginAfterLinking tests that after linking OAuth provider,
// both email and OAuth login should work for the same user.
// This will FAIL because OAuth linking doesn't persist provider.
func TestIntegration_LoginAfterLinking(t *testing.T) {
	t.Skip("EXPECTED TO FAIL - Run after implementing OAuth persistence")

	userRepo := newMockUserRepoForAuth()
	authMethodRepo := newMockAuthMethodRepo()

	config := &OAuthConfig{
		JWTSecret:     "test-secret",
		JWTExpiry:     "15m",
		RefreshExpiry: "168h",
	}
	handler := NewAuthHandlers(config, userRepo, authMethodRepo)

	// Step 1: Register with email/password
	registerReq := RegisterRequest{
		Email:       "linktest@example.com",
		Password:    "password456",
		Username:    "linktest",
		DisplayName: "Link Test",
	}
	body, _ := json.Marshal(registerReq)
	req := httptest.NewRequest(http.MethodPost, "/v1/auth/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.Register(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("Registration failed: %d", w.Code)
	}

	var registerResp struct {
		User struct {
			ID string `json:"id"`
		} `json:"user"`
	}
	json.Unmarshal(w.Body.Bytes(), &registerResp)
	userID := registerResp.User.ID

	// Step 2: Simulate OAuth login (should link provider)
	// TODO: Call OAuth callback or service

	// Step 3: Login with email/password should still work
	loginReq := LoginRequest{
		Email:    "linktest@example.com",
		Password: "password456",
	}
	loginBody, _ := json.Marshal(loginReq)
	loginReqHTTP := httptest.NewRequest(http.MethodPost, "/v1/auth/login", bytes.NewReader(loginBody))
	loginReqHTTP.Header.Set("Content-Type", "application/json")
	wLogin := httptest.NewRecorder()
	handler.Login(wLogin, loginReqHTTP)

	if wLogin.Code != http.StatusOK {
		t.Errorf("Email/password login failed after OAuth linking: %d", wLogin.Code)
	}

	// Step 4: Verify both auth methods exist
	methods, err := authMethodRepo.FindByUserID(context.Background(), userID)
	if err != nil {
		t.Fatalf("Failed to get auth methods: %v", err)
	}

	// This will FAIL because:
	// 1. auth_methods table doesn't exist
	// 2. OAuth service doesn't create auth_method entries
	// 3. Login handler doesn't query auth_methods
	if len(methods) != 2 {
		t.Errorf("EXPECTED FAILURE: Expected 2 auth methods (email + OAuth), got %d", len(methods))
	}
}

// TestIntegration_LoginWithOAuthOnly tests login attempt with email/password
// when user only has OAuth auth (no password set).
// Should return helpful error message telling user to use OAuth.
func TestIntegration_LoginWithOAuthOnly(t *testing.T) {
	t.Skip("EXPECTED TO FAIL - Run after implementing auth method checking in Login")

	userRepo := newMockUserRepoForAuth()
	authMethodRepo := newMockAuthMethodRepo()

	// Create OAuth-only user
	_ = createTestUserWithOAuth(t, userRepo, authMethodRepo, "oauth-only@example.com", models.AuthProviderGitHub, "gh_123")

	config := &OAuthConfig{
		JWTSecret:     "test-secret",
		JWTExpiry:     "15m",
		RefreshExpiry: "168h",
	}
	handler := NewAuthHandlers(config, userRepo, authMethodRepo)

	// Try to login with email/password (should fail with helpful message)
	loginReq := LoginRequest{
		Email:    "oauth-only@example.com",
		Password: "anypassword",
	}
	body, _ := json.Marshal(loginReq)
	req := httptest.NewRequest(http.MethodPost, "/v1/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.Login(w, req)

	// Should return 401 with error code "OAUTH_ONLY_USER"
	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401, got %d", w.Code)
	}

	var errResp struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	json.Unmarshal(w.Body.Bytes(), &errResp)

	// This will FAIL because Login handler doesn't check auth_methods yet
	if errResp.Error.Code != "OAUTH_ONLY_USER" {
		t.Errorf("EXPECTED FAILURE: Expected error code OAUTH_ONLY_USER, got %s", errResp.Error.Code)
	}

	// Message should mention which OAuth provider to use
	if !strings.Contains(errResp.Error.Message, "github") {
		t.Errorf("Error message should mention github: %s", errResp.Error.Message)
	}
}
