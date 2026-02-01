package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestOAuthHandlers_GitHubRedirect tests the GitHub OAuth redirect endpoint.
func TestOAuthHandlers_GitHubRedirect(t *testing.T) {
	// Create config with GitHub OAuth settings
	cfg := &OAuthConfig{
		GitHubClientID:    "test-github-client-id",
		GitHubRedirectURI: "http://localhost:8080/v1/auth/github/callback",
	}

	handler := NewOAuthHandlers(cfg, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/v1/auth/github", nil)
	rec := httptest.NewRecorder()

	handler.GitHubRedirect(rec, req)

	// Should redirect to GitHub
	if rec.Code != http.StatusFound {
		t.Errorf("expected status %d, got %d", http.StatusFound, rec.Code)
	}

	location := rec.Header().Get("Location")
	if location == "" {
		t.Fatal("expected Location header")
	}

	// Verify redirect URL contains required parameters
	if !strings.Contains(location, "https://github.com/login/oauth/authorize") {
		t.Errorf("expected GitHub authorize URL, got %s", location)
	}
	if !strings.Contains(location, "client_id=test-github-client-id") {
		t.Errorf("expected client_id in URL, got %s", location)
	}
	if !strings.Contains(location, "redirect_uri=") {
		t.Errorf("expected redirect_uri in URL, got %s", location)
	}
	if !strings.Contains(location, "scope=user:email") {
		t.Errorf("expected scope=user:email in URL, got %s", location)
	}
}

// TestOAuthHandlers_GoogleRedirect tests the Google OAuth redirect endpoint.
func TestOAuthHandlers_GoogleRedirect(t *testing.T) {
	cfg := &OAuthConfig{
		GoogleClientID:    "test-google-client-id",
		GoogleRedirectURI: "http://localhost:8080/v1/auth/google/callback",
	}

	handler := NewOAuthHandlers(cfg, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/v1/auth/google", nil)
	rec := httptest.NewRecorder()

	handler.GoogleRedirect(rec, req)

	// Should redirect to Google
	if rec.Code != http.StatusFound {
		t.Errorf("expected status %d, got %d", http.StatusFound, rec.Code)
	}

	location := rec.Header().Get("Location")
	if location == "" {
		t.Fatal("expected Location header")
	}

	// Verify redirect URL contains required parameters
	if !strings.Contains(location, "https://accounts.google.com/o/oauth2/v2/auth") {
		t.Errorf("expected Google authorize URL, got %s", location)
	}
	if !strings.Contains(location, "client_id=test-google-client-id") {
		t.Errorf("expected client_id in URL, got %s", location)
	}
	if !strings.Contains(location, "redirect_uri=") {
		t.Errorf("expected redirect_uri in URL, got %s", location)
	}
	if !strings.Contains(location, "scope=") {
		t.Errorf("expected scope in URL, got %s", location)
	}
}

// TestOAuthHandlers_GitHubCallback_MissingCode tests GitHub callback without code.
func TestOAuthHandlers_GitHubCallback_MissingCode(t *testing.T) {
	cfg := &OAuthConfig{}
	handler := NewOAuthHandlers(cfg, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/v1/auth/github/callback", nil)
	rec := httptest.NewRecorder()

	handler.GitHubCallback(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}

	var resp ErrorResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Error.Code != "VALIDATION_ERROR" {
		t.Errorf("expected error code VALIDATION_ERROR, got %s", resp.Error.Code)
	}
}

// TestOAuthHandlers_GoogleCallback_MissingCode tests Google callback without code.
func TestOAuthHandlers_GoogleCallback_MissingCode(t *testing.T) {
	cfg := &OAuthConfig{}
	handler := NewOAuthHandlers(cfg, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/v1/auth/google/callback", nil)
	rec := httptest.NewRecorder()

	handler.GoogleCallback(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}

	var resp ErrorResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Error.Code != "VALIDATION_ERROR" {
		t.Errorf("expected error code VALIDATION_ERROR, got %s", resp.Error.Code)
	}
}

// TestOAuthHandlers_GitHubCallback_WithError tests GitHub callback with error from GitHub.
func TestOAuthHandlers_GitHubCallback_WithError(t *testing.T) {
	cfg := &OAuthConfig{}
	handler := NewOAuthHandlers(cfg, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/v1/auth/github/callback?error=access_denied&error_description=User+denied+access", nil)
	rec := httptest.NewRecorder()

	handler.GitHubCallback(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}

	var resp ErrorResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Error.Code != "OAUTH_ERROR" {
		t.Errorf("expected error code OAUTH_ERROR, got %s", resp.Error.Code)
	}
}

// TestOAuthHandlers_GoogleCallback_WithError tests Google callback with error from Google.
func TestOAuthHandlers_GoogleCallback_WithError(t *testing.T) {
	cfg := &OAuthConfig{}
	handler := NewOAuthHandlers(cfg, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/v1/auth/google/callback?error=access_denied", nil)
	rec := httptest.NewRecorder()

	handler.GoogleCallback(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}

	var resp ErrorResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Error.Code != "OAUTH_ERROR" {
		t.Errorf("expected error code OAUTH_ERROR, got %s", resp.Error.Code)
	}
}

// ErrorResponse is used for parsing error responses in tests.
type ErrorResponse struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// ============================================================
// GitHub OAuth Code Exchange Tests (PRD line 172)
// ============================================================

// TestGitHubTokenExchange_Success tests successful code-to-token exchange.
func TestGitHubTokenExchange_Success(t *testing.T) {
	// Create a mock server that simulates GitHub's token endpoint
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify the request
		if r.Method != http.MethodPost {
			t.Errorf("expected POST method, got %s", r.Method)
		}
		if r.URL.Path != "/login/oauth/access_token" {
			t.Errorf("expected /login/oauth/access_token, got %s", r.URL.Path)
		}

		// Check Accept header for JSON response
		if r.Header.Get("Accept") != "application/json" {
			t.Errorf("expected Accept: application/json header")
		}

		// Parse and verify form data
		if err := r.ParseForm(); err != nil {
			t.Fatalf("failed to parse form: %v", err)
		}

		if r.FormValue("client_id") != "test-client-id" {
			t.Errorf("expected client_id=test-client-id, got %s", r.FormValue("client_id"))
		}
		if r.FormValue("client_secret") != "test-client-secret" {
			t.Errorf("expected client_secret=test-client-secret, got %s", r.FormValue("client_secret"))
		}
		if r.FormValue("code") != "test-auth-code" {
			t.Errorf("expected code=test-auth-code, got %s", r.FormValue("code"))
		}

		// Return successful token response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token": "gho_test_access_token",
			"token_type":   "bearer",
			"scope":        "user:email",
		})
	}))
	defer mockServer.Close()

	client := NewGitHubOAuthClient("test-client-id", "test-client-secret", mockServer.URL)

	token, err := client.ExchangeCode(context.Background(), "test-auth-code")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if token.AccessToken != "gho_test_access_token" {
		t.Errorf("expected access_token=gho_test_access_token, got %s", token.AccessToken)
	}
	if token.TokenType != "bearer" {
		t.Errorf("expected token_type=bearer, got %s", token.TokenType)
	}
}

// TestGitHubTokenExchange_InvalidCode tests token exchange with invalid code.
func TestGitHubTokenExchange_InvalidCode(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// GitHub returns 200 with error in body for invalid code
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error":             "bad_verification_code",
			"error_description": "The code passed is incorrect or expired.",
		})
	}))
	defer mockServer.Close()

	client := NewGitHubOAuthClient("test-client-id", "test-client-secret", mockServer.URL)

	_, err := client.ExchangeCode(context.Background(), "invalid-code")
	if err == nil {
		t.Fatal("expected error for invalid code")
	}

	// Should be an OAuthError
	oauthErr, ok := err.(*OAuthError)
	if !ok {
		t.Fatalf("expected *OAuthError, got %T", err)
	}
	if oauthErr.Code != "bad_verification_code" {
		t.Errorf("expected error code bad_verification_code, got %s", oauthErr.Code)
	}
}

// TestGitHubTokenExchange_NetworkError tests handling of network errors.
func TestGitHubTokenExchange_NetworkError(t *testing.T) {
	// Use an invalid URL to simulate network error
	client := NewGitHubOAuthClient("test-client-id", "test-client-secret", "http://localhost:1")

	_, err := client.ExchangeCode(context.Background(), "test-code")
	if err == nil {
		t.Fatal("expected network error")
	}
}

// TestGitHubTokenExchange_ServerError tests handling of 500 errors.
func TestGitHubTokenExchange_ServerError(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer mockServer.Close()

	client := NewGitHubOAuthClient("test-client-id", "test-client-secret", mockServer.URL)

	_, err := client.ExchangeCode(context.Background(), "test-code")
	if err == nil {
		t.Fatal("expected error for server error response")
	}
}

// TestGitHubTokenExchange_MalformedResponse tests handling of malformed JSON response.
func TestGitHubTokenExchange_MalformedResponse(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("not valid json"))
	}))
	defer mockServer.Close()

	client := NewGitHubOAuthClient("test-client-id", "test-client-secret", mockServer.URL)

	_, err := client.ExchangeCode(context.Background(), "test-code")
	if err == nil {
		t.Fatal("expected error for malformed response")
	}
}

// ============================================================
// GitHub OAuth User Info Fetch Tests (PRD line 173)
// ============================================================

// TestGitHubGetUser_Success tests successful user info fetch.
func TestGitHubGetUser_Success(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/user" {
			t.Errorf("expected /user, got %s", r.URL.Path)
		}

		// Verify Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader != "Bearer test-access-token" {
			t.Errorf("expected Authorization: Bearer test-access-token, got %s", authHeader)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":         12345,
			"login":      "testuser",
			"email":      "testuser@example.com",
			"name":       "Test User",
			"avatar_url": "https://avatars.githubusercontent.com/u/12345",
		})
	}))
	defer mockServer.Close()

	client := NewGitHubOAuthClient("test-client-id", "test-client-secret", mockServer.URL)

	user, err := client.GetUser(context.Background(), "test-access-token")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if user.ID != 12345 {
		t.Errorf("expected ID=12345, got %d", user.ID)
	}
	if user.Login != "testuser" {
		t.Errorf("expected Login=testuser, got %s", user.Login)
	}
	if user.Email != "testuser@example.com" {
		t.Errorf("expected Email=testuser@example.com, got %s", user.Email)
	}
	if user.Name != "Test User" {
		t.Errorf("expected Name=Test User, got %s", user.Name)
	}
	if user.AvatarURL != "https://avatars.githubusercontent.com/u/12345" {
		t.Errorf("expected AvatarURL to be set, got %s", user.AvatarURL)
	}
}

// TestGitHubGetUser_Unauthorized tests handling of unauthorized response.
func TestGitHubGetUser_Unauthorized(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"message": "Bad credentials",
		})
	}))
	defer mockServer.Close()

	client := NewGitHubOAuthClient("test-client-id", "test-client-secret", mockServer.URL)

	_, err := client.GetUser(context.Background(), "invalid-token")
	if err == nil {
		t.Fatal("expected error for unauthorized response")
	}
}

// TestGitHubGetPrimaryEmail_Success tests successful email fetch.
func TestGitHubGetPrimaryEmail_Success(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/user/emails" {
			t.Errorf("expected /user/emails, got %s", r.URL.Path)
		}

		// Verify Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader != "Bearer test-access-token" {
			t.Errorf("expected Authorization: Bearer test-access-token, got %s", authHeader)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode([]map[string]interface{}{
			{"email": "work@example.com", "primary": false, "verified": true},
			{"email": "personal@example.com", "primary": true, "verified": true},
			{"email": "unverified@example.com", "primary": false, "verified": false},
		})
	}))
	defer mockServer.Close()

	client := NewGitHubOAuthClient("test-client-id", "test-client-secret", mockServer.URL)

	email, err := client.GetPrimaryEmail(context.Background(), "test-access-token")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if email != "personal@example.com" {
		t.Errorf("expected primary email personal@example.com, got %s", email)
	}
}

// TestGitHubGetPrimaryEmail_FallbackToVerified tests fallback to verified email when no primary.
func TestGitHubGetPrimaryEmail_FallbackToVerified(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode([]map[string]interface{}{
			{"email": "unverified@example.com", "primary": true, "verified": false},
			{"email": "verified@example.com", "primary": false, "verified": true},
		})
	}))
	defer mockServer.Close()

	client := NewGitHubOAuthClient("test-client-id", "test-client-secret", mockServer.URL)

	email, err := client.GetPrimaryEmail(context.Background(), "test-access-token")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if email != "verified@example.com" {
		t.Errorf("expected fallback to verified email, got %s", email)
	}
}

// TestGitHubGetPrimaryEmail_NoEmails tests handling of user with no emails.
func TestGitHubGetPrimaryEmail_NoEmails(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode([]map[string]interface{}{})
	}))
	defer mockServer.Close()

	client := NewGitHubOAuthClient("test-client-id", "test-client-secret", mockServer.URL)

	_, err := client.GetPrimaryEmail(context.Background(), "test-access-token")
	if err == nil {
		t.Fatal("expected error when no emails found")
	}
}
