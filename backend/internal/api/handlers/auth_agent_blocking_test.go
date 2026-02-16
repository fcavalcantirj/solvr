package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/api/middleware"
	"github.com/go-chi/chi/v5"
)

func TestRegister_BlocksAgentAPIKey(t *testing.T) {
	// Arrange
	authHandler := setupAuthTestHandler(t)

	// Wrap handler with middleware
	r := chi.NewRouter()
	r.With(middleware.BlockAgentAPIKeys).Post("/auth/register", authHandler.Register)

	payload := map[string]string{
		"email":        "agent@test.com",
		"password":     "SecurePass123",
		"username":     "test_agent",
		"display_name": "Test Agent",
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer solvr_test_key_123")
	rec := httptest.NewRecorder()

	// Act
	r.ServeHTTP(rec, req)

	// Assert
	if rec.Code != http.StatusForbidden {
		t.Errorf("Expected status 403, got %d", rec.Code)
	}

	// Check error message mentions correct registration endpoint
	bodyStr := rec.Body.String()
	if bodyStr != "" && !stringContains(bodyStr, "/v1/agents/register") {
		t.Errorf("Error message should mention /v1/agents/register endpoint")
	}
}

func TestRegister_AllowsHumanRegistration(t *testing.T) {
	// Arrange
	authHandler := setupAuthTestHandler(t)

	r := chi.NewRouter()
	r.With(middleware.BlockAgentAPIKeys).Post("/auth/register", authHandler.Register)

	payload := map[string]string{
		"email":        "human@test.com",
		"password":     "SecurePass123",
		"username":     "test_human",
		"display_name": "Test Human",
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	// No Authorization header - normal human registration
	rec := httptest.NewRecorder()

	// Act
	r.ServeHTTP(rec, req)

	// Assert
	// Should NOT be 403 (might be 400 validation error or 201 success depending on test DB state)
	if rec.Code == http.StatusForbidden {
		t.Errorf("Human registration should not be blocked, got 403")
	}
}

func TestRegister_AllowsHumanJWTAuth(t *testing.T) {
	// Arrange: Set up handler with middleware
	authHandler := setupAuthTestHandler(t)
	r := chi.NewRouter()
	r.With(middleware.BlockAgentAPIKeys).Post("/auth/register", authHandler.Register)

	// Create registration payload
	payload := map[string]string{
		"email":        "jwtuser@test.com",
		"password":     "SecurePass123",
		"username":     "jwt_user",
		"display_name": "JWT Test User",
	}
	body, _ := json.Marshal(payload)

	// Create request with JWT token (not agent API key)
	// Using a realistic JWT format: 3-part base64-encoded token
	req := httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c")
	rec := httptest.NewRecorder()

	// Act
	r.ServeHTTP(rec, req)

	// Assert: Should NOT be 403 (middleware should allow JWT through)
	if rec.Code == http.StatusForbidden {
		t.Errorf("JWT tokens should not be blocked by agent API key middleware, got 403")
	}

	// Additional check: Response should be normal registration flow
	// (Either 201 success or 400 validation error, but NOT 403 forbidden)
	if rec.Code != http.StatusCreated && rec.Code != http.StatusBadRequest {
		t.Logf("Expected 201 or 400, got %d. Body: %s", rec.Code, rec.Body.String())
	}
}

func TestLogin_BlocksAgentAPIKey(t *testing.T) {
	// Arrange
	authHandler := setupAuthTestHandler(t)

	r := chi.NewRouter()
	r.With(middleware.BlockAgentAPIKeys).Post("/auth/login", authHandler.Login)

	payload := map[string]string{
		"email":    "test@test.com",
		"password": "password",
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer solvr_test_key_123")
	rec := httptest.NewRecorder()

	// Act
	r.ServeHTTP(rec, req)

	// Assert
	if rec.Code != http.StatusForbidden {
		t.Errorf("Expected status 403, got %d", rec.Code)
	}
}

func TestOAuthRedirect_BlocksAgentAPIKey(t *testing.T) {
	// Arrange
	oauthHandler := setupOAuthTestHandler(t)

	r := chi.NewRouter()
	r.With(middleware.BlockAgentAPIKeys).Get("/auth/github", oauthHandler.GitHubRedirect)

	req := httptest.NewRequest(http.MethodGet, "/auth/github", nil)
	req.Header.Set("Authorization", "Bearer solvr_test_key_123")
	rec := httptest.NewRecorder()

	// Act
	r.ServeHTTP(rec, req)

	// Assert
	if rec.Code != http.StatusForbidden {
		t.Errorf("Expected status 403, got %d", rec.Code)
	}
}

func TestOAuthCallback_BlocksAgentAPIKey(t *testing.T) {
	// Arrange
	oauthHandler := setupOAuthTestHandler(t)

	r := chi.NewRouter()
	r.With(middleware.BlockAgentAPIKeys).Get("/auth/github/callback", oauthHandler.GitHubCallback)

	req := httptest.NewRequest(http.MethodGet, "/auth/github/callback?code=test&state=test", nil)
	req.Header.Set("Authorization", "Bearer solvr_test_key_123")
	rec := httptest.NewRecorder()

	// Act
	r.ServeHTTP(rec, req)

	// Assert
	if rec.Code != http.StatusForbidden {
		t.Errorf("Expected status 403, got %d", rec.Code)
	}
}

// Helper function for string contains check
func stringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// setupAuthTestHandler creates a test auth handler using mocks
func setupAuthTestHandler(t *testing.T) *AuthHandlers {
	t.Helper()
	mockUserRepo := newMockUserRepoForAuth()
	mockAuthMethodRepo := newMockAuthMethodRepoStub()
	config := &OAuthConfig{
		JWTSecret: "test-secret-key",
	}
	return NewAuthHandlers(config, mockUserRepo, mockAuthMethodRepo)
}

// setupOAuthTestHandler creates a test OAuth handler using minimal config
// For these tests, the handler methods won't actually be called because
// the middleware returns 403 before reaching handler logic
func setupOAuthTestHandler(t *testing.T) *OAuthHandlers {
	t.Helper()
	config := &OAuthConfig{
		JWTSecret:          "test-secret-key",
		GitHubClientID:     "test-id",
		GitHubClientSecret: "test-secret",
		GitHubRedirectURI:  "http://localhost:8080/auth/github/callback",
		GoogleClientID:     "test-id",
		GoogleClientSecret: "test-secret",
		GoogleRedirectURI:  "http://localhost:8080/auth/google/callback",
		FrontendURL:        "http://localhost:3000",
	}
	// Pass nil for pool and tokenStore since middleware blocks before handler runs
	// We can't construct these without a real DB connection
	return &OAuthHandlers{
		config:        config,
		pool:          nil,
		tokenStore:    nil,
		gitHubBaseURL: "https://github.com",
		googleBaseURL: "https://oauth2.googleapis.com",
	}
}
