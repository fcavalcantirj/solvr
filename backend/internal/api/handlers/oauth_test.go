package handlers

import (
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
