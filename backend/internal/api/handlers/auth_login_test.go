package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/auth"
	"github.com/fcavalcantirj/solvr/internal/models"
	"golang.org/x/crypto/bcrypt"
)

// TestLogin_ValidCredentials tests successful login with correct email+password.
func TestLogin_ValidCredentials(t *testing.T) {
	mockRepo := newMockUserRepoForAuth()
	config := &OAuthConfig{
		JWTSecret:     "test-secret",
		JWTExpiry:     "15m",
		RefreshExpiry: "168h",
	}
	mockAuthMethodRepo := newMockAuthMethodRepoStub()
	handler := NewAuthHandlers(config, mockRepo, mockAuthMethodRepo, nil)

	// Create a user with password
	passwordHash, _ := bcrypt.GenerateFromPassword([]byte("correctpassword"), bcrypt.DefaultCost)
	existingUser := &models.User{
		ID:           "user-123",
		Email:        "user@example.com",
		Username:     "testuser",
		AuthProvider: models.AuthProviderEmail,
		PasswordHash: string(passwordHash),
		Role:         models.UserRoleUser,
	}
	mockRepo.users["user@example.com"] = existingUser

	// Create corresponding auth_method entry
	authMethod := &models.AuthMethod{
		ID:           "auth-method-123",
		UserID:       "user-123",
		AuthProvider: models.AuthProviderEmail,
		PasswordHash: string(passwordHash),
	}
	mockAuthMethodRepo.methods["user-123"] = []*models.AuthMethod{authMethod}

	// Attempt login
	reqBody := LoginRequest{
		Email:    "user@example.com",
		Password: "correctpassword",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/v1/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.Login(w, req)

	// Verify 200 OK
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d. Body: %s", w.Code, w.Body.String())
		return
	}

	// Verify response structure
	var resp LoginResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Verify access_token is present
	if resp.AccessToken == "" {
		t.Error("access_token is empty")
	}

	// Verify refresh_token is present
	if resp.RefreshToken == "" {
		t.Error("refresh_token is empty")
	}

	// Verify user object
	if resp.User.ID != "user-123" {
		t.Errorf("expected user.id 'user-123', got '%s'", resp.User.ID)
	}
	if resp.User.Email != "user@example.com" {
		t.Errorf("expected email 'user@example.com', got '%s'", resp.User.Email)
	}

	// Verify JWT can be decoded
	claims, err := auth.ValidateJWT(config.JWTSecret, resp.AccessToken)
	if err != nil {
		t.Errorf("JWT validation failed: %v", err)
	} else {
		if claims.UserID != "user-123" {
			t.Errorf("expected user_id 'user-123', got '%s'", claims.UserID)
		}
	}
}

// TestLogin_WrongPassword tests login with incorrect password returns 401.
func TestLogin_WrongPassword(t *testing.T) {
	mockRepo := newMockUserRepoForAuth()
	config := &OAuthConfig{
		JWTSecret:     "test-secret",
		JWTExpiry:     "15m",
		RefreshExpiry: "168h",
	}
	mockAuthMethodRepo := newMockAuthMethodRepoStub()
	handler := NewAuthHandlers(config, mockRepo, mockAuthMethodRepo, nil)

	// Create a user with password
	passwordHash, _ := bcrypt.GenerateFromPassword([]byte("correctpassword"), bcrypt.DefaultCost)
	existingUser := &models.User{
		ID:           "user-123",
		Email:        "user@example.com",
		Username:     "testuser",
		AuthProvider: models.AuthProviderEmail,
		PasswordHash: string(passwordHash),
		Role:         models.UserRoleUser,
	}
	mockRepo.users["user@example.com"] = existingUser

	// Create corresponding auth_method entry
	authMethod := &models.AuthMethod{
		ID:           "auth-method-123",
		UserID:       "user-123",
		AuthProvider: models.AuthProviderEmail,
		PasswordHash: string(passwordHash),
	}
	mockAuthMethodRepo.methods["user-123"] = []*models.AuthMethod{authMethod}

	// Attempt login with wrong password
	reqBody := LoginRequest{
		Email:    "user@example.com",
		Password: "wrongpassword",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/v1/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.Login(w, req)

	// Verify 401 Unauthorized
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d. Body: %s", w.Code, w.Body.String())
	}

	// Verify error response
	var errResp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&errResp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}
	errorObj, ok := errResp["error"].(map[string]interface{})
	if !ok {
		t.Fatal("error object not found in response")
	}
	if errorObj["code"] != "INVALID_CREDENTIALS" {
		t.Errorf("expected error code 'INVALID_CREDENTIALS', got '%v'", errorObj["code"])
	}
}

// TestLogin_NonExistentEmail tests login with non-existent email returns 401 (no email enumeration).
func TestLogin_NonExistentEmail(t *testing.T) {
	mockRepo := newMockUserRepoForAuth()
	config := &OAuthConfig{
		JWTSecret:     "test-secret",
		JWTExpiry:     "15m",
		RefreshExpiry: "168h",
	}
	mockAuthMethodRepo := newMockAuthMethodRepoStub()
	handler := NewAuthHandlers(config, mockRepo, mockAuthMethodRepo, nil)

	// Attempt login with non-existent email
	reqBody := LoginRequest{
		Email:    "nonexistent@example.com",
		Password: "anypassword",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/v1/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.Login(w, req)

	// Verify 401 Unauthorized (same as wrong password)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d. Body: %s", w.Code, w.Body.String())
	}

	// Verify error response uses same code as wrong password (no email enumeration)
	var errResp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&errResp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}
	errorObj, ok := errResp["error"].(map[string]interface{})
	if !ok {
		t.Fatal("error object not found in response")
	}
	if errorObj["code"] != "INVALID_CREDENTIALS" {
		t.Errorf("expected error code 'INVALID_CREDENTIALS', got '%v'", errorObj["code"])
	}
}

// TestLogin_OAuthOnlyUser tests login with OAuth-only user (no password_hash) returns 401 with specific message.
func TestLogin_OAuthOnlyUser(t *testing.T) {
	mockRepo := newMockUserRepoForAuth()
	config := &OAuthConfig{
		JWTSecret:     "test-secret",
		JWTExpiry:     "15m",
		RefreshExpiry: "168h",
	}
	mockAuthMethodRepo := newMockAuthMethodRepoStub()
	handler := NewAuthHandlers(config, mockRepo, mockAuthMethodRepo, nil)

	// Create an OAuth-only user (no password_hash)
	oauthUser := &models.User{
		ID:             "user-456",
		Email:          "oauth@example.com",
		Username:       "oauthuser",
		AuthProvider:   models.AuthProviderGoogle,
		AuthProviderID: "google-123",
		PasswordHash:   "", // No password
		Role:           models.UserRoleUser,
	}
	mockRepo.users["oauth@example.com"] = oauthUser

	// Create OAuth auth_method entry (no email/password method)
	oauthMethod := &models.AuthMethod{
		ID:             "oauth-method-456",
		UserID:         "user-456",
		AuthProvider:   models.AuthProviderGoogle,
		AuthProviderID: "google-123",
	}
	mockAuthMethodRepo.methods["user-456"] = []*models.AuthMethod{oauthMethod}

	// Attempt login with password
	reqBody := LoginRequest{
		Email:    "oauth@example.com",
		Password: "anypassword",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/v1/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.Login(w, req)

	// Verify 401 Unauthorized
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d. Body: %s", w.Code, w.Body.String())
	}

	// Verify specific error message for OAuth users
	var errResp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&errResp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}
	errorObj, ok := errResp["error"].(map[string]interface{})
	if !ok {
		t.Fatal("error object not found in response")
	}
	message := errorObj["message"].(string)
	if !strings.Contains(strings.ToLower(message), "google") && !strings.Contains(strings.ToLower(message), "github") {
		t.Errorf("error message should mention OAuth providers: %s", message)
	}
}
