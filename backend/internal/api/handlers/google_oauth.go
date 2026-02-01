// Package handlers provides HTTP handlers for the Solvr API.
package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// GoogleOAuthClient handles OAuth communication with Google.
// Per SPEC.md Part 5.2: Google OAuth integration.
type GoogleOAuthClient struct {
	clientID     string
	clientSecret string
	redirectURI  string
	baseURL      string // Base URL for Google OAuth (allows overriding for tests)
	httpClient   *http.Client
}

// GoogleTokenResponse represents the response from Google's token endpoint.
type GoogleTokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token,omitempty"`
	Scope        string `json:"scope"`
	IDToken      string `json:"id_token,omitempty"`
	Error        string `json:"error,omitempty"`
	ErrorDesc    string `json:"error_description,omitempty"`
}

// GoogleUser represents user information from Google API.
// Per SPEC.md Part 5.2: Google OAuth user info (sub, email, name, picture).
type GoogleUser struct {
	Sub           string `json:"sub"`           // Unique Google ID
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Name          string `json:"name"`
	GivenName     string `json:"given_name"`
	FamilyName    string `json:"family_name"`
	Picture       string `json:"picture"`
	Locale        string `json:"locale"`
}

// Google OAuth endpoints
const (
	googleTokenURL    = "https://oauth2.googleapis.com/token"
	googleUserInfoURL = "https://www.googleapis.com/oauth2/v3/userinfo"
)

// NewGoogleOAuthClient creates a new Google OAuth client.
func NewGoogleOAuthClient(clientID, clientSecret, redirectURI, baseURL string) *GoogleOAuthClient {
	// Default to Google's production URLs if not specified
	if baseURL == "" {
		baseURL = "https://oauth2.googleapis.com"
	}
	return &GoogleOAuthClient{
		clientID:     clientID,
		clientSecret: clientSecret,
		redirectURI:  redirectURI,
		baseURL:      baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// ExchangeCode exchanges an authorization code for an access token.
// Per SPEC.md Part 5.2: POST to Google token endpoint with client_id, client_secret, code.
func (c *GoogleOAuthClient) ExchangeCode(ctx context.Context, code string) (*GoogleTokenResponse, error) {
	// Build request body
	data := url.Values{
		"client_id":     {c.clientID},
		"client_secret": {c.clientSecret},
		"code":          {code},
		"grant_type":    {"authorization_code"},
		"redirect_uri":  {c.redirectURI},
	}

	// Determine token URL
	tokenURL := googleTokenURL
	if c.baseURL != "https://oauth2.googleapis.com" && c.baseURL != "" {
		tokenURL = c.baseURL + "/token"
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create token request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	// Send request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange code: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read token response: %w", err)
	}

	// Parse JSON response
	var tokenResp GoogleTokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("failed to parse token response: %w", err)
	}

	// Check for error in response body
	if tokenResp.Error != "" {
		return nil, &OAuthError{
			Code:        tokenResp.Error,
			Description: tokenResp.ErrorDesc,
		}
	}

	// Check for non-200 status
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("token exchange failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Validate we got an access token
	if tokenResp.AccessToken == "" {
		return nil, fmt.Errorf("no access token in response")
	}

	return &tokenResp, nil
}

// GetUser fetches the authenticated user's information from Google.
// Per SPEC.md Part 5.2: GET userinfo endpoint with Authorization header.
func (c *GoogleOAuthClient) GetUser(ctx context.Context, accessToken string) (*GoogleUser, error) {
	userInfoURL := googleUserInfoURL
	// For testing, if baseURL is not the default, use the test server
	if c.baseURL != "https://oauth2.googleapis.com" && c.baseURL != "" {
		userInfoURL = c.baseURL + "/userinfo"
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, userInfoURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create user request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch user info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to fetch user info: status %d, body: %s", resp.StatusCode, string(body))
	}

	var user GoogleUser
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, fmt.Errorf("failed to parse user info: %w", err)
	}

	return &user, nil
}
