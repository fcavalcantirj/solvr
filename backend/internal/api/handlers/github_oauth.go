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

// GitHubOAuthClient handles OAuth communication with GitHub.
// Per SPEC.md Part 5.2: GitHub OAuth integration.
type GitHubOAuthClient struct {
	clientID     string
	clientSecret string
	baseURL      string // Base URL for GitHub OAuth (allows overriding for tests)
	httpClient   *http.Client
}

// GitHubTokenResponse represents the response from GitHub's token endpoint.
type GitHubTokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	Scope        string `json:"scope"`
	Error        string `json:"error,omitempty"`
	ErrorDesc    string `json:"error_description,omitempty"`
	ErrorURI     string `json:"error_uri,omitempty"`
}

// GitHubUser represents user information from GitHub API.
// Per SPEC.md Part 5.2: GitHub OAuth user info.
type GitHubUser struct {
	ID        int64  `json:"id"`
	Login     string `json:"login"`
	Email     string `json:"email"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatar_url"`
}

// GitHubEmail represents an email from GitHub's /user/emails endpoint.
type GitHubEmail struct {
	Email    string `json:"email"`
	Primary  bool   `json:"primary"`
	Verified bool   `json:"verified"`
}

// OAuthError represents an OAuth error response.
type OAuthError struct {
	Code        string
	Description string
}

func (e *OAuthError) Error() string {
	if e.Description != "" {
		return fmt.Sprintf("oauth error: %s - %s", e.Code, e.Description)
	}
	return fmt.Sprintf("oauth error: %s", e.Code)
}

// NewGitHubOAuthClient creates a new GitHub OAuth client.
func NewGitHubOAuthClient(clientID, clientSecret, baseURL string) *GitHubOAuthClient {
	// Default to GitHub's production URL if not specified
	if baseURL == "" {
		baseURL = "https://github.com"
	}
	return &GitHubOAuthClient{
		clientID:     clientID,
		clientSecret: clientSecret,
		baseURL:      baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// ExchangeCode exchanges an authorization code for an access token.
// Per SPEC.md Part 5.2: POST to GitHub token endpoint with client_id, client_secret, code.
func (c *GitHubOAuthClient) ExchangeCode(ctx context.Context, code string) (*GitHubTokenResponse, error) {
	// Build request body
	data := url.Values{
		"client_id":     {c.clientID},
		"client_secret": {c.clientSecret},
		"code":          {code},
	}

	// Create request
	tokenURL := c.baseURL + "/login/oauth/access_token"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create token request: %w", err)
	}

	// Set headers - request JSON response
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

	// Check for non-200 status (GitHub typically returns 200 even for errors)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("token exchange failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Parse JSON response
	var tokenResp GitHubTokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("failed to parse token response: %w", err)
	}

	// Check for OAuth error in response body (GitHub returns 200 with error in body)
	if tokenResp.Error != "" {
		return nil, &OAuthError{
			Code:        tokenResp.Error,
			Description: tokenResp.ErrorDesc,
		}
	}

	// Validate we got an access token
	if tokenResp.AccessToken == "" {
		return nil, fmt.Errorf("no access token in response")
	}

	return &tokenResp, nil
}

// GetUser fetches the authenticated user's information from GitHub.
// Per SPEC.md Part 5.2: GET https://api.github.com/user with Authorization header.
func (c *GitHubOAuthClient) GetUser(ctx context.Context, accessToken string) (*GitHubUser, error) {
	apiURL := "https://api.github.com/user"
	// For testing, if baseURL is not github.com, use the test server
	if c.baseURL != "https://github.com" && c.baseURL != "" {
		apiURL = c.baseURL + "/user"
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
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

	var user GitHubUser
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, fmt.Errorf("failed to parse user info: %w", err)
	}

	return &user, nil
}

// GetPrimaryEmail fetches the authenticated user's primary email from GitHub.
// Per SPEC.md Part 5.2: Get user email from GitHub API.
func (c *GitHubOAuthClient) GetPrimaryEmail(ctx context.Context, accessToken string) (string, error) {
	apiURL := "https://api.github.com/user/emails"
	// For testing, if baseURL is not github.com, use the test server
	if c.baseURL != "https://github.com" && c.baseURL != "" {
		apiURL = c.baseURL + "/user/emails"
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create emails request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch user emails: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("failed to fetch user emails: status %d, body: %s", resp.StatusCode, string(body))
	}

	var emails []GitHubEmail
	if err := json.NewDecoder(resp.Body).Decode(&emails); err != nil {
		return "", fmt.Errorf("failed to parse user emails: %w", err)
	}

	// Find primary email
	for _, email := range emails {
		if email.Primary && email.Verified {
			return email.Email, nil
		}
	}

	// Fallback to first verified email
	for _, email := range emails {
		if email.Verified {
			return email.Email, nil
		}
	}

	// Last resort: return first email
	if len(emails) > 0 {
		return emails[0].Email, nil
	}

	return "", fmt.Errorf("no email found for user")
}
