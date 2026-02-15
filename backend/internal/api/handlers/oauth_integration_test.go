package handlers

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/db"
)

// TestGitHubOAuthRedirect_Integration tests the GitHub OAuth redirect endpoint
// with a real HTTP server and router.
func TestGitHubOAuthRedirect_Integration(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}

	// Connect to database
	ctx := context.Background()
	pool, err := db.NewPool(ctx, dbURL)
	if err != nil {
		t.Fatalf("failed to connect to database: %v", err)
	}
	defer pool.Close()

	// Create OAuth config
	cfg := &OAuthConfig{
		GitHubClientID:    "test-github-client-id",
		GitHubRedirectURI: "http://localhost:8080/v1/auth/github/callback",
	}

	// Create handlers with real dependencies
	// Using NewOAuthHandlers with pool and nil tokenStore (tokenStore is optional)
	handler := NewOAuthHandlers(cfg, pool, nil)

	// Create test server with router
	router := setupTestRouter(handler)
	server := httptest.NewServer(router)
	defer server.Close()

	// Make request to GitHub OAuth redirect endpoint
	resp, err := http.Get(server.URL + "/v1/auth/github")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	// Should return 302 redirect
	if resp.StatusCode != http.StatusFound {
		t.Errorf("expected status %d, got %d", http.StatusFound, resp.StatusCode)
	}

	// Verify redirect location
	location := resp.Header.Get("Location")
	if location == "" {
		t.Fatal("expected Location header to be set")
	}

	// Parse redirect URL
	redirectURL, err := url.Parse(location)
	if err != nil {
		t.Fatalf("failed to parse redirect URL: %v", err)
	}

	// Verify redirect goes to GitHub
	if !strings.Contains(location, "github.com/login/oauth/authorize") {
		t.Errorf("expected redirect to GitHub OAuth, got %s", location)
	}

	// Verify required query parameters
	query := redirectURL.Query()
	if query.Get("client_id") != "test-github-client-id" {
		t.Errorf("expected client_id=test-github-client-id, got %s", query.Get("client_id"))
	}
	if query.Get("redirect_uri") == "" {
		t.Error("expected redirect_uri parameter")
	}
	if query.Get("scope") != "user:email" {
		t.Errorf("expected scope=user:email, got %s", query.Get("scope"))
	}
	if query.Get("state") == "" {
		t.Error("expected state parameter for CSRF protection")
	}
}

// TestGitHubCallback_MissingCode_Integration tests the callback endpoint
// when the code parameter is missing.
func TestGitHubCallback_MissingCode_Integration(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}

	// Connect to database
	ctx := context.Background()
	pool, err := db.NewPool(ctx, dbURL)
	if err != nil {
		t.Fatalf("failed to connect to database: %v", err)
	}
	defer pool.Close()

	// Create OAuth config
	cfg := &OAuthConfig{
		GitHubClientID:    "test-github-client-id",
		GitHubRedirectURI: "http://localhost:8080/v1/auth/github/callback",
		FrontendURL:       "http://localhost:3000",
	}

	// Create handlers with real dependencies
	handler := NewOAuthHandlers(cfg, pool, nil)

	// Create test server with router
	router := setupTestRouter(handler)
	server := httptest.NewServer(router)
	defer server.Close()

	// Make request to callback without code parameter
	resp, err := http.Get(server.URL + "/v1/auth/github/callback")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	// Should redirect to frontend with error
	if resp.StatusCode != http.StatusFound {
		t.Errorf("expected status %d, got %d", http.StatusFound, resp.StatusCode)
	}

	location := resp.Header.Get("Location")
	if !strings.Contains(location, "error=") {
		t.Errorf("expected error parameter in redirect URL, got %s", location)
	}
}

// TestGitHubCallback_GitHubError_Integration tests the callback endpoint
// when GitHub returns an error.
func TestGitHubCallback_GitHubError_Integration(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}

	// Connect to database
	ctx := context.Background()
	pool, err := db.NewPool(ctx, dbURL)
	if err != nil {
		t.Fatalf("failed to connect to database: %v", err)
	}
	defer pool.Close()

	// Create OAuth config
	cfg := &OAuthConfig{
		GitHubClientID:    "test-github-client-id",
		GitHubRedirectURI: "http://localhost:8080/v1/auth/github/callback",
		FrontendURL:       "http://localhost:3000",
	}

	// Create handlers with real dependencies
	handler := NewOAuthHandlers(cfg, pool, nil)

	// Create test server with router
	router := setupTestRouter(handler)
	server := httptest.NewServer(router)
	defer server.Close()

	// Make request to callback with GitHub error
	errorURL := fmt.Sprintf("%s/v1/auth/github/callback?error=access_denied&error_description=User+denied+access", server.URL)
	resp, err := http.Get(errorURL)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	// Should redirect to frontend with error forwarded
	if resp.StatusCode != http.StatusFound {
		t.Errorf("expected status %d, got %d", http.StatusFound, resp.StatusCode)
	}

	location := resp.Header.Get("Location")
	if !strings.Contains(location, "error=access_denied") {
		t.Errorf("expected error=access_denied in redirect URL, got %s", location)
	}
}

// TestGitHubCallback_InvalidCode_Integration tests the callback endpoint
// with an invalid authorization code.
func TestGitHubCallback_InvalidCode_Integration(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}

	// Connect to database
	ctx := context.Background()
	pool, err := db.NewPool(ctx, dbURL)
	if err != nil {
		t.Fatalf("failed to connect to database: %v", err)
	}
	defer pool.Close()

	// Create OAuth config with invalid credentials
	// This will cause the GitHub API call to fail
	cfg := &OAuthConfig{
		GitHubClientID:     "invalid-client-id",
		GitHubClientSecret: "invalid-client-secret",
		GitHubRedirectURI:  "http://localhost:8080/v1/auth/github/callback",
		FrontendURL:        "http://localhost:3000",
		JWTSecret:          "test-jwt-secret-32-chars-long!!",
		JWTExpiry:          "15m",
		RefreshExpiry:      "7d",
	}

	// Create handlers with real dependencies
	handler := NewOAuthHandlers(cfg, pool, nil)

	// Create test server with router
	router := setupTestRouter(handler)
	server := httptest.NewServer(router)
	defer server.Close()

	// Make request to callback with invalid code
	// This will attempt to call the real GitHub API and should fail
	invalidCodeURL := fmt.Sprintf("%s/v1/auth/github/callback?code=invalid_code_12345&state=state", server.URL)
	resp, err := http.Get(invalidCodeURL)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	// Should redirect to frontend with error
	if resp.StatusCode != http.StatusFound {
		t.Errorf("expected status %d, got %d", http.StatusFound, resp.StatusCode)
	}

	location := resp.Header.Get("Location")
	if !strings.Contains(location, "error=") {
		t.Errorf("expected error parameter in redirect URL due to invalid code, got %s", location)
	}
}

// setupTestRouter creates a minimal router for integration testing.
// This mimics the actual router setup in router.go.
func setupTestRouter(oauthHandler *OAuthHandlers) http.Handler {
	mux := http.NewServeMux()

	// OAuth routes
	mux.HandleFunc("GET /v1/auth/github", oauthHandler.GitHubRedirect)
	mux.HandleFunc("GET /v1/auth/github/callback", oauthHandler.GitHubCallback)

	return mux
}
