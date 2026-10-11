package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// Helper to hash password using bcrypt
func hashPasswordForTest(password string) string {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		panic(fmt.Sprintf("Failed to hash password: %v", err))
	}
	return string(hash)
}

// Helper to generate UUID (simplified for tests)
// generateUUID was a timestamp, but the clock here moves in microseconds: two methods created
// in one microsecond shared an id and Delete removed the wrong one (81 failing runs in 300).
func generateUUID() string {
	return "uuid_" + uuid.NewString()
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
	handler := NewAuthHandlers(config, userRepo, authMethodRepo, nil)
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
	handler := NewAuthHandlers(config, userRepo, authMethodRepo, nil)

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
	handler := NewAuthHandlers(config, userRepo, authMethodRepo, nil)

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
