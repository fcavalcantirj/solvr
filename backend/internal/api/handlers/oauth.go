// Package handlers provides HTTP handlers for the Solvr API.
package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/fcavalcantirj/solvr/internal/auth"
	"github.com/fcavalcantirj/solvr/internal/db"
)

// OAuthConfig contains OAuth provider configuration.
type OAuthConfig struct {
	// GitHub OAuth
	GitHubClientID     string
	GitHubClientSecret string
	GitHubRedirectURI  string

	// Google OAuth
	GoogleClientID     string
	GoogleClientSecret string
	GoogleRedirectURI  string

	// JWT configuration
	JWTSecret     string
	JWTExpiry     string // e.g., "15m"
	RefreshExpiry string // e.g., "7d"

	// Frontend URL for redirects after OAuth
	FrontendURL string
}

// OAuthHandlers handles OAuth authentication endpoints.
// Per SPEC.md Part 5.2.
type OAuthHandlers struct {
	config       *OAuthConfig
	pool         *db.Pool
	tokenStore   *auth.RefreshTokenStore
}

// NewOAuthHandlers creates a new OAuthHandlers instance.
func NewOAuthHandlers(config *OAuthConfig, pool *db.Pool, tokenStore *auth.RefreshTokenStore) *OAuthHandlers {
	return &OAuthHandlers{
		config:     config,
		pool:       pool,
		tokenStore: tokenStore,
	}
}

// GitHub OAuth URLs
const (
	gitHubAuthorizeURL = "https://github.com/login/oauth/authorize"
	gitHubTokenURL     = "https://github.com/login/oauth/access_token"
	gitHubUserURL      = "https://api.github.com/user"
	gitHubUserEmailURL = "https://api.github.com/user/emails"
)

// Google OAuth URLs
const (
	googleAuthorizeURL = "https://accounts.google.com/o/oauth2/v2/auth"
	googleTokenURL     = "https://oauth2.googleapis.com/token"
	googleUserInfoURL  = "https://www.googleapis.com/oauth2/v2/userinfo"
)

// GitHubRedirect handles GET /v1/auth/github
// Redirects to GitHub OAuth authorization page.
// Per SPEC.md Part 5.2: GitHub OAuth redirect endpoint.
func (h *OAuthHandlers) GitHubRedirect(w http.ResponseWriter, r *http.Request) {
	params := url.Values{
		"client_id":    {h.config.GitHubClientID},
		"redirect_uri": {h.config.GitHubRedirectURI},
		"scope":        {"user:email"},
		"state":        {generateState()}, // CSRF protection
	}

	redirectURL := gitHubAuthorizeURL + "?" + params.Encode()
	http.Redirect(w, r, redirectURL, http.StatusFound)
}

// GitHubCallback handles GET /v1/auth/github/callback
// Exchanges code for token, fetches user info, creates/updates user, returns tokens.
// Per SPEC.md Part 5.2: GitHub OAuth callback endpoint.
func (h *OAuthHandlers) GitHubCallback(w http.ResponseWriter, r *http.Request) {
	// Check for error from GitHub
	if errParam := r.URL.Query().Get("error"); errParam != "" {
		errDesc := r.URL.Query().Get("error_description")
		writeOAuthError(w, errParam, errDesc)
		return
	}

	// Extract authorization code
	code := r.URL.Query().Get("code")
	if code == "" {
		writeValidationError(w, "authorization code is required")
		return
	}

	// TODO: Implement the following steps:
	// 1. Exchange code for access token
	// 2. Fetch user info from GitHub
	// 3. Create or update user in database
	// 4. Generate JWT and refresh token
	// 5. Return tokens to client

	// For now, return not implemented
	writeInternalError(w, "GitHub OAuth flow not fully implemented")
}

// GoogleRedirect handles GET /v1/auth/google
// Redirects to Google OAuth authorization page.
// Per SPEC.md Part 5.2: Google OAuth redirect endpoint.
func (h *OAuthHandlers) GoogleRedirect(w http.ResponseWriter, r *http.Request) {
	params := url.Values{
		"client_id":     {h.config.GoogleClientID},
		"redirect_uri":  {h.config.GoogleRedirectURI},
		"response_type": {"code"},
		"scope":         {"email profile"},
		"state":         {generateState()}, // CSRF protection
	}

	redirectURL := googleAuthorizeURL + "?" + params.Encode()
	http.Redirect(w, r, redirectURL, http.StatusFound)
}

// GoogleCallback handles GET /v1/auth/google/callback
// Exchanges code for token, fetches user info, creates/updates user, returns tokens.
// Per SPEC.md Part 5.2: Google OAuth callback endpoint.
func (h *OAuthHandlers) GoogleCallback(w http.ResponseWriter, r *http.Request) {
	// Check for error from Google
	if errParam := r.URL.Query().Get("error"); errParam != "" {
		writeOAuthError(w, errParam, "")
		return
	}

	// Extract authorization code
	code := r.URL.Query().Get("code")
	if code == "" {
		writeValidationError(w, "authorization code is required")
		return
	}

	// TODO: Implement the following steps:
	// 1. Exchange code for access token
	// 2. Fetch user info from Google
	// 3. Create or update user in database
	// 4. Generate JWT and refresh token
	// 5. Return tokens to client

	// For now, return not implemented
	writeInternalError(w, "Google OAuth flow not fully implemented")
}

// generateState generates a random state parameter for CSRF protection.
// In a real implementation, this should be stored in a session or cookie
// and verified in the callback.
func generateState() string {
	// For now, return a simple placeholder
	// TODO: Implement proper state generation and verification
	return "state"
}

// Response types

// AuthTokenResponse is the response containing access and refresh tokens.
// Per SPEC.md Part 5.2.
type AuthTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"` // seconds
}

// Helper functions for writing responses

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{"data": data})
}

func writeValidationError(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]string{
			"code":    "VALIDATION_ERROR",
			"message": message,
		},
	})
}

func writeOAuthError(w http.ResponseWriter, errCode, errDesc string) {
	message := fmt.Sprintf("OAuth error: %s", errCode)
	if errDesc != "" {
		message = fmt.Sprintf("%s - %s", message, errDesc)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]string{
			"code":    "OAUTH_ERROR",
			"message": message,
		},
	})
}

func writeInternalError(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusInternalServerError)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]string{
			"code":    "INTERNAL_ERROR",
			"message": message,
		},
	})
}

func writeBadGateway(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadGateway)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]string{
			"code":    "BAD_GATEWAY",
			"message": message,
		},
	})
}
