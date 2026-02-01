// Package handlers provides HTTP handlers for the Solvr API.
package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/fcavalcantirj/solvr/internal/auth"
	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
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

// OAuthUserServiceInterface defines the interface for OAuth user management.
type OAuthUserServiceInterface interface {
	FindOrCreateUser(ctx context.Context, info *OAuthUserInfoData) (*OAuthUserResult, bool, error)
}

// OAuthUserInfoData contains user information from an OAuth provider.
type OAuthUserInfoData struct {
	Provider    string
	ProviderID  string
	Email       string
	DisplayName string
	AvatarURL   string
}

// OAuthUserResult represents the user result from FindOrCreateUser.
type OAuthUserResult struct {
	ID          string
	Username    string
	DisplayName string
	Email       string
	AvatarURL   string
	Role        string
}

// OAuthHandlers handles OAuth authentication endpoints.
// Per SPEC.md Part 5.2.
type OAuthHandlers struct {
	config        *OAuthConfig
	pool          *db.Pool
	tokenStore    *auth.RefreshTokenStore
	userService   OAuthUserServiceInterface
	gitHubBaseURL string // Allows overriding for tests
}

// NewOAuthHandlers creates a new OAuthHandlers instance.
func NewOAuthHandlers(config *OAuthConfig, pool *db.Pool, tokenStore *auth.RefreshTokenStore) *OAuthHandlers {
	return &OAuthHandlers{
		config:        config,
		pool:          pool,
		tokenStore:    tokenStore,
		gitHubBaseURL: "https://github.com",
	}
}

// NewOAuthHandlersWithDeps creates OAuthHandlers with all dependencies for testing.
func NewOAuthHandlersWithDeps(
	config *OAuthConfig,
	pool *db.Pool,
	tokenStore *auth.RefreshTokenStore,
	userService OAuthUserServiceInterface,
	gitHubBaseURL string,
) *OAuthHandlers {
	return &OAuthHandlers{
		config:        config,
		pool:          pool,
		tokenStore:    tokenStore,
		userService:   userService,
		gitHubBaseURL: gitHubBaseURL,
	}
}

// GitHub OAuth URLs
const (
	gitHubAuthorizeURL = "https://github.com/login/oauth/authorize"
)

// Google OAuth URLs
const (
	googleAuthorizeURL = "https://accounts.google.com/o/oauth2/v2/auth"
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
	ctx := r.Context()

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

	// Step 1: Exchange code for access token
	gitHubClient := NewGitHubOAuthClient(
		h.config.GitHubClientID,
		h.config.GitHubClientSecret,
		h.gitHubBaseURL,
	)

	tokenResp, err := gitHubClient.ExchangeCode(ctx, code)
	if err != nil {
		// Check if it's an OAuth error (e.g., invalid code)
		if oauthErr, ok := err.(*OAuthError); ok {
			writeOAuthError(w, oauthErr.Code, oauthErr.Description)
			return
		}
		// Other errors are gateway errors
		log.Printf("GitHub token exchange failed: %v", err)
		writeBadGateway(w, "Failed to communicate with GitHub")
		return
	}

	// Step 2: Fetch user info from GitHub
	ghUser, err := gitHubClient.GetUser(ctx, tokenResp.AccessToken)
	if err != nil {
		log.Printf("GitHub user fetch failed: %v", err)
		writeBadGateway(w, "Failed to fetch user info from GitHub")
		return
	}

	// Get email if not in user response
	email := ghUser.Email
	if email == "" {
		email, err = gitHubClient.GetPrimaryEmail(ctx, tokenResp.AccessToken)
		if err != nil {
			log.Printf("GitHub email fetch failed: %v", err)
			writeBadGateway(w, "Failed to fetch user email from GitHub")
			return
		}
	}

	// Step 3: Create or find user in database
	userInfo := &OAuthUserInfoData{
		Provider:    models.AuthProviderGitHub,
		ProviderID:  strconv.FormatInt(ghUser.ID, 10),
		Email:       email,
		DisplayName: ghUser.Name,
		AvatarURL:   ghUser.AvatarURL,
	}

	// Use display name fallback if empty
	if userInfo.DisplayName == "" {
		userInfo.DisplayName = ghUser.Login
	}

	var user *OAuthUserResult
	if h.userService != nil {
		user, _, err = h.userService.FindOrCreateUser(ctx, userInfo)
		if err != nil {
			log.Printf("User creation/lookup failed: %v", err)
			writeInternalError(w, "Failed to create or find user")
			return
		}
	} else {
		// Fallback for when user service is not injected (testing or minimal setup)
		user = &OAuthUserResult{
			ID:          "mock-user-id",
			Username:    ghUser.Login,
			DisplayName: userInfo.DisplayName,
			Email:       email,
			AvatarURL:   ghUser.AvatarURL,
			Role:        models.UserRoleUser,
		}
	}

	// Step 4: Generate JWT
	jwtExpiry, err := time.ParseDuration(h.config.JWTExpiry)
	if err != nil {
		jwtExpiry = 15 * time.Minute // Default
	}

	accessToken, err := auth.GenerateJWT(h.config.JWTSecret, user.ID, user.Email, user.Role, jwtExpiry)
	if err != nil {
		log.Printf("JWT generation failed: %v", err)
		writeInternalError(w, "Failed to generate access token")
		return
	}

	// Step 5: Generate refresh token
	refreshToken := auth.GenerateRefreshToken()

	// Store refresh token if token store is available
	if h.tokenStore != nil {
		refreshExpiry, err := time.ParseDuration(h.config.RefreshExpiry)
		if err != nil {
			refreshExpiry = 7 * 24 * time.Hour // Default 7 days
		}
		expiresAt := time.Now().Add(refreshExpiry)
		if err := h.tokenStore.StoreToken(ctx, user.ID, refreshToken, expiresAt); err != nil {
			log.Printf("Refresh token storage failed: %v", err)
			// Continue anyway - user can still use access token
		}
	}

	// Step 6: Return tokens and user info
	response := map[string]interface{}{
		"access_token":  accessToken,
		"refresh_token": refreshToken,
		"token_type":    "Bearer",
		"expires_in":    int(jwtExpiry.Seconds()),
		"user": map[string]interface{}{
			"id":           user.ID,
			"username":     user.Username,
			"display_name": user.DisplayName,
			"email":        user.Email,
			"avatar_url":   user.AvatarURL,
			"role":         user.Role,
		},
	}

	writeJSON(w, http.StatusOK, response)
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

	// TODO: Implement Google OAuth flow similar to GitHub
	// 1. Exchange code for access token
	// 2. Fetch user info from Google
	// 3. Create or update user in database
	// 4. Generate JWT and refresh token
	// 5. Return tokens to client

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
