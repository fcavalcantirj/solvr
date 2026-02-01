package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// ============================================================
// Google OAuth Code Exchange Tests (PRD line 179)
// ============================================================

// TestGoogleTokenExchange_Success tests successful code-to-token exchange.
func TestGoogleTokenExchange_Success(t *testing.T) {
	// Create a mock server that simulates Google's token endpoint
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify the request
		if r.Method != http.MethodPost {
			t.Errorf("expected POST method, got %s", r.Method)
		}
		if r.URL.Path != "/token" {
			t.Errorf("expected /token, got %s", r.URL.Path)
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
		if r.FormValue("grant_type") != "authorization_code" {
			t.Errorf("expected grant_type=authorization_code, got %s", r.FormValue("grant_type"))
		}
		if r.FormValue("redirect_uri") != "http://localhost:8080/v1/auth/google/callback" {
			t.Errorf("expected redirect_uri to match, got %s", r.FormValue("redirect_uri"))
		}

		// Return successful token response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token":  "ya29.test_access_token",
			"token_type":    "Bearer",
			"expires_in":    3600,
			"refresh_token": "1//test_refresh_token",
			"scope":         "email profile",
		})
	}))
	defer mockServer.Close()

	client := NewGoogleOAuthClient(
		"test-client-id",
		"test-client-secret",
		"http://localhost:8080/v1/auth/google/callback",
		mockServer.URL,
	)

	token, err := client.ExchangeCode(context.Background(), "test-auth-code")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if token.AccessToken != "ya29.test_access_token" {
		t.Errorf("expected access_token=ya29.test_access_token, got %s", token.AccessToken)
	}
	if token.TokenType != "Bearer" {
		t.Errorf("expected token_type=Bearer, got %s", token.TokenType)
	}
	if token.ExpiresIn != 3600 {
		t.Errorf("expected expires_in=3600, got %d", token.ExpiresIn)
	}
}

// TestGoogleTokenExchange_InvalidCode tests token exchange with invalid code.
func TestGoogleTokenExchange_InvalidCode(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Google returns 400 with error in body for invalid code
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error":             "invalid_grant",
			"error_description": "Code not found or already used.",
		})
	}))
	defer mockServer.Close()

	client := NewGoogleOAuthClient(
		"test-client-id",
		"test-client-secret",
		"http://localhost:8080/v1/auth/google/callback",
		mockServer.URL,
	)

	_, err := client.ExchangeCode(context.Background(), "invalid-code")
	if err == nil {
		t.Fatal("expected error for invalid code")
	}

	// Should be an OAuthError
	oauthErr, ok := err.(*OAuthError)
	if !ok {
		t.Fatalf("expected *OAuthError, got %T: %v", err, err)
	}
	if oauthErr.Code != "invalid_grant" {
		t.Errorf("expected error code invalid_grant, got %s", oauthErr.Code)
	}
}

// TestGoogleTokenExchange_NetworkError tests handling of network errors.
func TestGoogleTokenExchange_NetworkError(t *testing.T) {
	// Use an invalid URL to simulate network error
	client := NewGoogleOAuthClient(
		"test-client-id",
		"test-client-secret",
		"http://localhost:8080/v1/auth/google/callback",
		"http://localhost:1",
	)

	_, err := client.ExchangeCode(context.Background(), "test-code")
	if err == nil {
		t.Fatal("expected network error")
	}
}

// TestGoogleTokenExchange_ServerError tests handling of 500 errors.
func TestGoogleTokenExchange_ServerError(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer mockServer.Close()

	client := NewGoogleOAuthClient(
		"test-client-id",
		"test-client-secret",
		"http://localhost:8080/v1/auth/google/callback",
		mockServer.URL,
	)

	_, err := client.ExchangeCode(context.Background(), "test-code")
	if err == nil {
		t.Fatal("expected error for server error response")
	}
}

// TestGoogleTokenExchange_MalformedResponse tests handling of malformed JSON response.
func TestGoogleTokenExchange_MalformedResponse(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("not valid json"))
	}))
	defer mockServer.Close()

	client := NewGoogleOAuthClient(
		"test-client-id",
		"test-client-secret",
		"http://localhost:8080/v1/auth/google/callback",
		mockServer.URL,
	)

	_, err := client.ExchangeCode(context.Background(), "test-code")
	if err == nil {
		t.Fatal("expected error for malformed response")
	}
}

// ============================================================
// Google OAuth User Info Fetch Tests (PRD line 179)
// ============================================================

// TestGoogleGetUser_Success tests successful user info fetch.
func TestGoogleGetUser_Success(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/userinfo" {
			t.Errorf("expected /userinfo, got %s", r.URL.Path)
		}

		// Verify Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader != "Bearer test-access-token" {
			t.Errorf("expected Authorization: Bearer test-access-token, got %s", authHeader)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"sub":            "123456789012345678901",
			"email":          "testuser@gmail.com",
			"email_verified": true,
			"name":           "Test User",
			"given_name":     "Test",
			"family_name":    "User",
			"picture":        "https://lh3.googleusercontent.com/a/test-picture",
			"locale":         "en",
		})
	}))
	defer mockServer.Close()

	client := NewGoogleOAuthClient(
		"test-client-id",
		"test-client-secret",
		"http://localhost:8080/v1/auth/google/callback",
		mockServer.URL,
	)

	user, err := client.GetUser(context.Background(), "test-access-token")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if user.Sub != "123456789012345678901" {
		t.Errorf("expected Sub=123456789012345678901, got %s", user.Sub)
	}
	if user.Email != "testuser@gmail.com" {
		t.Errorf("expected Email=testuser@gmail.com, got %s", user.Email)
	}
	if !user.EmailVerified {
		t.Error("expected EmailVerified=true")
	}
	if user.Name != "Test User" {
		t.Errorf("expected Name=Test User, got %s", user.Name)
	}
	if user.GivenName != "Test" {
		t.Errorf("expected GivenName=Test, got %s", user.GivenName)
	}
	if user.FamilyName != "User" {
		t.Errorf("expected FamilyName=User, got %s", user.FamilyName)
	}
	if user.Picture != "https://lh3.googleusercontent.com/a/test-picture" {
		t.Errorf("expected Picture to be set, got %s", user.Picture)
	}
}

// TestGoogleGetUser_Unauthorized tests handling of unauthorized response.
func TestGoogleGetUser_Unauthorized(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]interface{}{
				"code":    401,
				"message": "Invalid Credentials",
				"status":  "UNAUTHENTICATED",
			},
		})
	}))
	defer mockServer.Close()

	client := NewGoogleOAuthClient(
		"test-client-id",
		"test-client-secret",
		"http://localhost:8080/v1/auth/google/callback",
		mockServer.URL,
	)

	_, err := client.GetUser(context.Background(), "invalid-token")
	if err == nil {
		t.Fatal("expected error for unauthorized response")
	}
}

// TestGoogleGetUser_MinimalResponse tests user fetch with minimal data.
func TestGoogleGetUser_MinimalResponse(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		// Google may return minimal data if user hasn't set all fields
		json.NewEncoder(w).Encode(map[string]interface{}{
			"sub":            "987654321098765432109",
			"email":          "minimal@gmail.com",
			"email_verified": true,
		})
	}))
	defer mockServer.Close()

	client := NewGoogleOAuthClient(
		"test-client-id",
		"test-client-secret",
		"http://localhost:8080/v1/auth/google/callback",
		mockServer.URL,
	)

	user, err := client.GetUser(context.Background(), "test-access-token")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if user.Sub != "987654321098765432109" {
		t.Errorf("expected Sub=987654321098765432109, got %s", user.Sub)
	}
	if user.Email != "minimal@gmail.com" {
		t.Errorf("expected Email=minimal@gmail.com, got %s", user.Email)
	}
	// Other fields should be empty strings (Go zero values)
	if user.Name != "" {
		t.Errorf("expected Name to be empty, got %s", user.Name)
	}
}

// TestGoogleGetUser_NetworkError tests handling of network errors during user fetch.
func TestGoogleGetUser_NetworkError(t *testing.T) {
	client := NewGoogleOAuthClient(
		"test-client-id",
		"test-client-secret",
		"http://localhost:8080/v1/auth/google/callback",
		"http://localhost:1",
	)

	_, err := client.GetUser(context.Background(), "test-token")
	if err == nil {
		t.Fatal("expected network error")
	}
}

// TestGoogleGetUser_MalformedResponse tests handling of malformed JSON in user response.
func TestGoogleGetUser_MalformedResponse(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("not valid json"))
	}))
	defer mockServer.Close()

	client := NewGoogleOAuthClient(
		"test-client-id",
		"test-client-secret",
		"http://localhost:8080/v1/auth/google/callback",
		mockServer.URL,
	)

	_, err := client.GetUser(context.Background(), "test-token")
	if err == nil {
		t.Fatal("expected error for malformed response")
	}
}

// ============================================================
// Google OAuth Complete Flow Tests (PRD lines 179-180)
// ============================================================

// TestGoogleCallback_CompleteFlow_NewUser tests the complete flow for a new user.
func TestGoogleCallback_CompleteFlow_NewUser(t *testing.T) {
	// Create a mock Google server
	mockGoogleServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		switch r.URL.Path {
		case "/token":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"access_token": "ya29.test_token",
				"token_type":   "Bearer",
				"expires_in":   3600,
			})
		case "/userinfo":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"sub":            "123456789012345678901",
				"email":          "newgoogleuser@gmail.com",
				"email_verified": true,
				"name":           "New Google User",
				"picture":        "https://lh3.googleusercontent.com/a/test",
			})
		default:
			t.Errorf("unexpected request to %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer mockGoogleServer.Close()

	// Create handlers with mock config
	cfg := &OAuthConfig{
		GoogleClientID:    "test-client-id",
		GoogleClientSecret: "test-client-secret",
		GoogleRedirectURI: "http://localhost:8080/v1/auth/google/callback",
		JWTSecret:         "test-jwt-secret-32-chars-long!!",
		JWTExpiry:         "15m",
		RefreshExpiry:     "7d",
	}

	// Create a mock user service
	mockUserService := &MockGoogleOAuthUserService{
		users: make(map[string]*MockGoogleUserData),
	}

	handler := NewOAuthHandlersWithAllDeps(cfg, nil, nil, mockUserService, "", mockGoogleServer.URL)

	// Make request with valid code
	req := httptest.NewRequest(http.MethodGet, "/v1/auth/google/callback?code=valid-code&state=state", nil)
	rec := httptest.NewRecorder()

	handler.GoogleCallback(rec, req)

	// Should return 200 with tokens
	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var resp GoogleAuthSuccessResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v. Body: %s", err, rec.Body.String())
	}

	if resp.Data.AccessToken == "" {
		t.Error("expected access_token to be set")
	}
	if resp.Data.RefreshToken == "" {
		t.Error("expected refresh_token to be set")
	}
	if resp.Data.User.Email != "newgoogleuser@gmail.com" {
		t.Errorf("expected user email newgoogleuser@gmail.com, got %s", resp.Data.User.Email)
	}
}

// TestGoogleCallback_CompleteFlow_ExistingUser tests login for an existing user.
func TestGoogleCallback_CompleteFlow_ExistingUser(t *testing.T) {
	mockGoogleServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		switch r.URL.Path {
		case "/token":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"access_token": "ya29.test_token",
				"token_type":   "Bearer",
			})
		case "/userinfo":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"sub":            "existing-google-sub",
				"email":          "existing@gmail.com",
				"email_verified": true,
				"name":           "Existing User",
				"picture":        "https://lh3.googleusercontent.com/a/existing",
			})
		}
	}))
	defer mockGoogleServer.Close()

	cfg := &OAuthConfig{
		GoogleClientID:    "test-client-id",
		GoogleClientSecret: "test-client-secret",
		GoogleRedirectURI: "http://localhost:8080/v1/auth/google/callback",
		JWTSecret:         "test-jwt-secret-32-chars-long!!",
		JWTExpiry:         "15m",
		RefreshExpiry:     "7d",
	}

	// Create a mock user service with existing user
	mockUserService := &MockGoogleOAuthUserService{
		users: map[string]*MockGoogleUserData{
			"google:existing-google-sub": {
				ID:       "existing-user-id",
				Email:    "existing@gmail.com",
				Username: "existinguser",
				IsNew:    false,
			},
		},
	}

	handler := NewOAuthHandlersWithAllDeps(cfg, nil, nil, mockUserService, "", mockGoogleServer.URL)

	req := httptest.NewRequest(http.MethodGet, "/v1/auth/google/callback?code=valid-code&state=state", nil)
	rec := httptest.NewRecorder()

	handler.GoogleCallback(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var resp GoogleAuthSuccessResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Data.User.ID != "existing-user-id" {
		t.Errorf("expected user ID existing-user-id, got %s", resp.Data.User.ID)
	}
}

// TestGoogleCallback_GoogleAPIError tests error handling when Google API fails.
func TestGoogleCallback_GoogleAPIError(t *testing.T) {
	mockGoogleServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer mockGoogleServer.Close()

	cfg := &OAuthConfig{
		GoogleClientID:    "test-client-id",
		GoogleClientSecret: "test-client-secret",
		GoogleRedirectURI: "http://localhost:8080/v1/auth/google/callback",
		JWTSecret:         "test-jwt-secret-32-chars-long!!",
	}

	handler := NewOAuthHandlersWithAllDeps(cfg, nil, nil, nil, "", mockGoogleServer.URL)

	req := httptest.NewRequest(http.MethodGet, "/v1/auth/google/callback?code=valid-code&state=state", nil)
	rec := httptest.NewRecorder()

	handler.GoogleCallback(rec, req)

	// Should return 502 BAD_GATEWAY per SPEC.md
	if rec.Code != http.StatusBadGateway {
		t.Errorf("expected status %d, got %d", http.StatusBadGateway, rec.Code)
	}

	var resp ErrorResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Error.Code != "BAD_GATEWAY" {
		t.Errorf("expected error code BAD_GATEWAY, got %s", resp.Error.Code)
	}
}

// TestGoogleCallback_InvalidCode tests error handling when code is invalid.
func TestGoogleCallback_InvalidCode(t *testing.T) {
	mockGoogleServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error":             "invalid_grant",
			"error_description": "Code not found or already used.",
		})
	}))
	defer mockGoogleServer.Close()

	cfg := &OAuthConfig{
		GoogleClientID:    "test-client-id",
		GoogleClientSecret: "test-client-secret",
		GoogleRedirectURI: "http://localhost:8080/v1/auth/google/callback",
		JWTSecret:         "test-jwt-secret-32-chars-long!!",
	}

	handler := NewOAuthHandlersWithAllDeps(cfg, nil, nil, nil, "", mockGoogleServer.URL)

	req := httptest.NewRequest(http.MethodGet, "/v1/auth/google/callback?code=invalid-code&state=state", nil)
	rec := httptest.NewRecorder()

	handler.GoogleCallback(rec, req)

	// Should return 400 for invalid code
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

// TestGoogleCallback_UserInfoFetchFails tests when user info fetch fails.
func TestGoogleCallback_UserInfoFetchFails(t *testing.T) {
	mockGoogleServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case "/token":
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"access_token": "ya29.test_token",
				"token_type":   "Bearer",
			})
		case "/userinfo":
			// User info fetch fails
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": "invalid_token",
			})
		}
	}))
	defer mockGoogleServer.Close()

	cfg := &OAuthConfig{
		GoogleClientID:    "test-client-id",
		GoogleClientSecret: "test-client-secret",
		GoogleRedirectURI: "http://localhost:8080/v1/auth/google/callback",
		JWTSecret:         "test-jwt-secret-32-chars-long!!",
	}

	handler := NewOAuthHandlersWithAllDeps(cfg, nil, nil, nil, "", mockGoogleServer.URL)

	req := httptest.NewRequest(http.MethodGet, "/v1/auth/google/callback?code=valid-code&state=state", nil)
	rec := httptest.NewRecorder()

	handler.GoogleCallback(rec, req)

	// Should return 502 BAD_GATEWAY
	if rec.Code != http.StatusBadGateway {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusBadGateway, rec.Code, rec.Body.String())
	}
}

// Mock types for Google OAuth testing

// MockGoogleOAuthUserService is a mock implementation of OAuthUserServiceInterface for Google tests.
type MockGoogleOAuthUserService struct {
	users map[string]*MockGoogleUserData
}

// MockGoogleUserData represents mock user data for Google OAuth tests.
type MockGoogleUserData struct {
	ID       string
	Email    string
	Username string
	IsNew    bool
}

// FindOrCreateUser mocks the user service for Google OAuth.
func (m *MockGoogleOAuthUserService) FindOrCreateUser(ctx context.Context, info *OAuthUserInfoData) (*OAuthUserResult, bool, error) {
	key := info.Provider + ":" + info.ProviderID

	if userData, ok := m.users[key]; ok {
		return &OAuthUserResult{
			ID:          userData.ID,
			Email:       userData.Email,
			Username:    userData.Username,
			DisplayName: info.DisplayName,
			AvatarURL:   info.AvatarURL,
			Role:        "user",
		}, false, nil
	}

	// Create new user
	newUser := &OAuthUserResult{
		ID:          "new-user-" + info.ProviderID,
		Email:       info.Email,
		Username:    info.DisplayName,
		DisplayName: info.DisplayName,
		AvatarURL:   info.AvatarURL,
		Role:        "user",
	}

	return newUser, true, nil
}

// GoogleAuthSuccessResponse is the success response for Google OAuth.
type GoogleAuthSuccessResponse struct {
	Data struct {
		AccessToken  string             `json:"access_token"`
		RefreshToken string             `json:"refresh_token"`
		TokenType    string             `json:"token_type"`
		ExpiresIn    int                `json:"expires_in"`
		User         GoogleUserResponse `json:"user"`
	} `json:"data"`
}

// GoogleUserResponse represents a user in the Google OAuth response.
type GoogleUserResponse struct {
	ID          string `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	Email       string `json:"email"`
	AvatarURL   string `json:"avatar_url,omitempty"`
	Role        string `json:"role,omitempty"`
}
