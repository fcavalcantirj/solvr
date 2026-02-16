package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/auth"
	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
	"golang.org/x/crypto/bcrypt"
)

// mockUserRepoForAuth is a mock implementation of UserRepositoryForAuth.
type mockUserRepoForAuth struct {
	users            map[string]*models.User // keyed by email
	usersByUsername  map[string]*models.User // keyed by username
	createErr        error
	findByEmailErr   error
	findByUsernameErr error
}

func newMockUserRepoForAuth() *mockUserRepoForAuth {
	return &mockUserRepoForAuth{
		users:           make(map[string]*models.User),
		usersByUsername: make(map[string]*models.User),
	}
}

func (m *mockUserRepoForAuth) FindByEmail(ctx context.Context, email string) (*models.User, error) {
	if m.findByEmailErr != nil {
		return nil, m.findByEmailErr
	}
	user, exists := m.users[email]
	if !exists {
		return nil, db.ErrNotFound
	}
	return user, nil
}

func (m *mockUserRepoForAuth) FindByUsername(ctx context.Context, username string) (*models.User, error) {
	if m.findByUsernameErr != nil {
		return nil, m.findByUsernameErr
	}
	user, exists := m.usersByUsername[username]
	if !exists {
		return nil, db.ErrNotFound
	}
	return user, nil
}

func (m *mockUserRepoForAuth) Create(ctx context.Context, user *models.User) (*models.User, error) {
	if m.createErr != nil {
		return nil, m.createErr
	}
	// Check for duplicates
	if _, exists := m.users[user.Email]; exists {
		return nil, db.ErrDuplicateEmail
	}
	if _, exists := m.usersByUsername[user.Username]; exists {
		return nil, db.ErrDuplicateUsername
	}

	// Generate ID if not set
	if user.ID == "" {
		user.ID = "mock-user-id"
	}

	// Store in both maps
	m.users[user.Email] = user
	m.usersByUsername[user.Username] = user
	return user, nil
}

// mockAuthMethodRepoStub is a minimal mock for AuthMethodRepository
// that tracks auth methods in memory
type mockAuthMethodRepoStub struct {
	methods map[string][]*models.AuthMethod // keyed by user_id
}

func newMockAuthMethodRepoStub() *mockAuthMethodRepoStub {
	return &mockAuthMethodRepoStub{
		methods: make(map[string][]*models.AuthMethod),
	}
}

func (m *mockAuthMethodRepoStub) Create(ctx context.Context, method *models.AuthMethod) (*models.AuthMethod, error) {
	// Generate ID if not set
	if method.ID == "" {
		method.ID = "mock-auth-method-id"
	}
	// Store method
	m.methods[method.UserID] = append(m.methods[method.UserID], method)
	return method, nil
}

func (m *mockAuthMethodRepoStub) FindByUserID(ctx context.Context, userID string) ([]*models.AuthMethod, error) {
	methods, exists := m.methods[userID]
	if !exists {
		return []*models.AuthMethod{}, nil
	}
	return methods, nil
}

func (m *mockAuthMethodRepoStub) FindByProvider(ctx context.Context, provider, providerID string) (*models.AuthMethod, error) {
	for _, methods := range m.methods {
		for _, method := range methods {
			if method.AuthProvider == provider && method.AuthProviderID == providerID {
				return method, nil
			}
		}
	}
	return nil, db.ErrNotFound
}

func (m *mockAuthMethodRepoStub) GetEmailAuthMethod(ctx context.Context, userID string) (*models.AuthMethod, error) {
	methods, exists := m.methods[userID]
	if !exists {
		return nil, db.ErrNotFound
	}
	for _, method := range methods {
		if method.AuthProvider == models.AuthProviderEmail {
			return method, nil
		}
	}
	return nil, db.ErrNotFound
}

func (m *mockAuthMethodRepoStub) UpdateLastUsed(ctx context.Context, methodID string) error {
	// Find and update the method
	for _, methods := range m.methods {
		for _, method := range methods {
			if method.ID == methodID {
				// In a real implementation, this would update last_used_at timestamp
				return nil
			}
		}
	}
	return db.ErrNotFound
}

func (m *mockAuthMethodRepoStub) HasEmailAuth(ctx context.Context, userID string) (bool, error) {
	methods, exists := m.methods[userID]
	if !exists {
		return false, nil
	}
	for _, method := range methods {
		if method.AuthProvider == models.AuthProviderEmail {
			return true, nil
		}
	}
	return false, nil
}

// TestRegister_ValidRequest tests successful registration with valid input.
func TestRegister_ValidRequest(t *testing.T) {
	mockRepo := newMockUserRepoForAuth()
	config := &OAuthConfig{
		JWTSecret:    "test-secret",
		JWTExpiry:    "15m",
		RefreshExpiry: "168h",
	}
	mockAuthMethodRepo := newMockAuthMethodRepoStub()
	handler := NewAuthHandlers(config, mockRepo, mockAuthMethodRepo)

	reqBody := RegisterRequest{
		Email:       "newuser@example.com",
		Password:    "securepass123",
		Username:    "newuser",
		DisplayName: "New User",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/v1/auth/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.Register(w, req)

	// Verify 201 Created
	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d. Body: %s", w.Code, w.Body.String())
		return
	}

	// Verify response structure
	var resp RegisterResponse
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
	if resp.User.ID == "" {
		t.Error("user.id is empty")
	}
	if resp.User.Email != "newuser@example.com" {
		t.Errorf("expected email 'newuser@example.com', got '%s'", resp.User.Email)
	}
	if resp.User.Username != "newuser" {
		t.Errorf("expected username 'newuser', got '%s'", resp.User.Username)
	}
	if resp.User.Role != models.UserRoleUser {
		t.Errorf("expected role 'user', got '%s'", resp.User.Role)
	}

	// Verify JWT can be decoded with correct claims
	claims, err := auth.ValidateJWT(config.JWTSecret, resp.AccessToken)
	if err != nil {
		t.Errorf("JWT validation failed: %v", err)
	} else {
		if claims.UserID != "mock-user-id" {
			t.Errorf("expected user_id 'mock-user-id', got '%s'", claims.UserID)
		}
		if claims.Email != "newuser@example.com" {
			t.Errorf("expected email 'newuser@example.com', got '%s'", claims.Email)
		}
		if claims.Role != models.UserRoleUser {
			t.Errorf("expected role 'user', got '%s'", claims.Role)
		}
	}

	// Verify user was created in repo with bcrypt hash
	createdUser, err := mockRepo.FindByEmail(context.Background(), "newuser@example.com")
	if err != nil {
		t.Fatalf("user not found in repo: %v", err)
	}
	if createdUser.PasswordHash == "" {
		t.Error("password_hash is empty")
	}
	// Verify bcrypt format (starts with $2a$ or $2b$)
	if !strings.HasPrefix(createdUser.PasswordHash, "$2a$") && !strings.HasPrefix(createdUser.PasswordHash, "$2b$") {
		t.Errorf("password_hash does not appear to be bcrypt format: %s", createdUser.PasswordHash[:10])
	}
	// Verify password can be verified
	if err := bcrypt.CompareHashAndPassword([]byte(createdUser.PasswordHash), []byte("securepass123")); err != nil {
		t.Errorf("password hash verification failed: %v", err)
	}
}

// TestRegister_DuplicateEmail tests registration with existing email returns 409.
func TestRegister_DuplicateEmail(t *testing.T) {
	mockRepo := newMockUserRepoForAuth()
	config := &OAuthConfig{
		JWTSecret:    "test-secret",
		JWTExpiry:    "15m",
		RefreshExpiry: "168h",
	}
	mockAuthMethodRepo := newMockAuthMethodRepoStub()
	handler := NewAuthHandlers(config, mockRepo, mockAuthMethodRepo)

	// Create existing user
	existingUser := &models.User{
		ID:           "existing-id",
		Email:        "existing@example.com",
		Username:     "existinguser",
		AuthProvider: models.AuthProviderEmail,
		Role:         models.UserRoleUser,
	}
	mockRepo.users["existing@example.com"] = existingUser
	mockRepo.usersByUsername["existinguser"] = existingUser

	// Attempt registration with same email, different username
	reqBody := RegisterRequest{
		Email:       "existing@example.com",
		Password:    "securepass123",
		Username:    "differentuser",
		DisplayName: "Different User",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/v1/auth/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.Register(w, req)

	// Verify 409 Conflict
	if w.Code != http.StatusConflict {
		t.Errorf("expected status 409, got %d. Body: %s", w.Code, w.Body.String())
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
	if errorObj["code"] != "DUPLICATE_EMAIL" {
		t.Errorf("expected error code 'DUPLICATE_EMAIL', got '%v'", errorObj["code"])
	}
}

// TestRegister_WeakPassword tests registration with passwords under 8 characters returns 400.
func TestRegister_WeakPassword(t *testing.T) {
	mockRepo := newMockUserRepoForAuth()
	config := &OAuthConfig{
		JWTSecret:    "test-secret",
		JWTExpiry:    "15m",
		RefreshExpiry: "168h",
	}
	mockAuthMethodRepo := newMockAuthMethodRepoStub()
	handler := NewAuthHandlers(config, mockRepo, mockAuthMethodRepo)

	testCases := []struct {
		name     string
		password string
	}{
		{"empty", ""},
		{"short", "short"},
		{"whitespace", "       "},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			reqBody := RegisterRequest{
				Email:       "test@example.com",
				Password:    tc.password,
				Username:    "testuser",
				DisplayName: "Test User",
			}
			body, _ := json.Marshal(reqBody)

			req := httptest.NewRequest(http.MethodPost, "/v1/auth/register", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			handler.Register(w, req)

			// Verify 400 Bad Request
			if w.Code != http.StatusBadRequest {
				t.Errorf("expected status 400, got %d. Body: %s", w.Code, w.Body.String())
			}

			// Verify error code
			var errResp map[string]interface{}
			if err := json.NewDecoder(w.Body).Decode(&errResp); err != nil {
				t.Fatalf("failed to decode error response: %v", err)
			}
			errorObj, ok := errResp["error"].(map[string]interface{})
			if !ok {
				t.Fatal("error object not found in response")
			}
			if errorObj["code"] != "INVALID_PASSWORD" {
				t.Errorf("expected error code 'INVALID_PASSWORD', got '%v'", errorObj["code"])
			}
			// Verify message mentions minimum 8 characters
			message := errorObj["message"].(string)
			if !strings.Contains(strings.ToLower(message), "8") {
				t.Errorf("error message should mention '8 characters': %s", message)
			}
		})
	}
}

// TestRegister_InvalidEmail tests registration with malformed emails returns 400.
func TestRegister_InvalidEmail(t *testing.T) {
	mockRepo := newMockUserRepoForAuth()
	config := &OAuthConfig{
		JWTSecret:    "test-secret",
		JWTExpiry:    "15m",
		RefreshExpiry: "168h",
	}
	mockAuthMethodRepo := newMockAuthMethodRepoStub()
	handler := NewAuthHandlers(config, mockRepo, mockAuthMethodRepo)

	testCases := []struct {
		name  string
		email string
	}{
		{"missing_at", "notanemail"},
		{"missing_domain", "test@"},
		{"empty", ""},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			reqBody := RegisterRequest{
				Email:       tc.email,
				Password:    "securepass123",
				Username:    "testuser",
				DisplayName: "Test User",
			}
			body, _ := json.Marshal(reqBody)

			req := httptest.NewRequest(http.MethodPost, "/v1/auth/register", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			handler.Register(w, req)

			// Verify 400 Bad Request
			if w.Code != http.StatusBadRequest {
				t.Errorf("expected status 400, got %d. Body: %s", w.Code, w.Body.String())
			}

			// Verify error code
			var errResp map[string]interface{}
			if err := json.NewDecoder(w.Body).Decode(&errResp); err != nil {
				t.Fatalf("failed to decode error response: %v", err)
			}
			errorObj, ok := errResp["error"].(map[string]interface{})
			if !ok {
				t.Fatal("error object not found in response")
			}
			if errorObj["code"] != "INVALID_EMAIL" {
				t.Errorf("expected error code 'INVALID_EMAIL', got '%v'", errorObj["code"])
			}
		})
	}
}

// TestRegister_DuplicateUsername tests registration with existing username returns 409.
func TestRegister_DuplicateUsername(t *testing.T) {
	mockRepo := newMockUserRepoForAuth()
	config := &OAuthConfig{
		JWTSecret:    "test-secret",
		JWTExpiry:    "15m",
		RefreshExpiry: "168h",
	}
	mockAuthMethodRepo := newMockAuthMethodRepoStub()
	handler := NewAuthHandlers(config, mockRepo, mockAuthMethodRepo)

	// Create existing user
	existingUser := &models.User{
		ID:           "existing-id",
		Email:        "existing@example.com",
		Username:     "existinguser",
		AuthProvider: models.AuthProviderEmail,
		Role:         models.UserRoleUser,
	}
	mockRepo.users["existing@example.com"] = existingUser
	mockRepo.usersByUsername["existinguser"] = existingUser

	// Attempt registration with same username, different email
	reqBody := RegisterRequest{
		Email:       "different@example.com",
		Password:    "securepass123",
		Username:    "existinguser",
		DisplayName: "Different User",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/v1/auth/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.Register(w, req)

	// Verify 409 Conflict
	if w.Code != http.StatusConflict {
		t.Errorf("expected status 409, got %d. Body: %s", w.Code, w.Body.String())
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
	if errorObj["code"] != "DUPLICATE_USERNAME" {
		t.Errorf("expected error code 'DUPLICATE_USERNAME', got '%v'", errorObj["code"])
	}
}

// TestRegister_InvalidUsername tests registration with invalid username format returns 400.
func TestRegister_InvalidUsername(t *testing.T) {
	mockRepo := newMockUserRepoForAuth()
	config := &OAuthConfig{
		JWTSecret:    "test-secret",
		JWTExpiry:    "15m",
		RefreshExpiry: "168h",
	}
	mockAuthMethodRepo := newMockAuthMethodRepoStub()
	handler := NewAuthHandlers(config, mockRepo, mockAuthMethodRepo)

	testCases := []struct {
		name     string
		username string
	}{
		{"too_short", "ab"},
		{"too_long", "thisusernameiswaytoolongandexceedsthirtychars"},
		{"special_chars", "test@user"},
		{"spaces", "test user"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			reqBody := RegisterRequest{
				Email:       "test@example.com",
				Password:    "securepass123",
				Username:    tc.username,
				DisplayName: "Test User",
			}
			body, _ := json.Marshal(reqBody)

			req := httptest.NewRequest(http.MethodPost, "/v1/auth/register", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			handler.Register(w, req)

			// Verify 400 Bad Request
			if w.Code != http.StatusBadRequest {
				t.Errorf("expected status 400, got %d. Body: %s", w.Code, w.Body.String())
			}

			// Verify error code
			var errResp map[string]interface{}
			if err := json.NewDecoder(w.Body).Decode(&errResp); err != nil {
				t.Fatalf("failed to decode error response: %v", err)
			}
			errorObj, ok := errResp["error"].(map[string]interface{})
			if !ok {
				t.Fatal("error object not found in response")
			}
			if errorObj["code"] != "INVALID_USERNAME" {
				t.Errorf("expected error code 'INVALID_USERNAME', got '%v'", errorObj["code"])
			}
			// Verify message mentions character requirements
			message := errorObj["message"].(string)
			if !strings.Contains(strings.ToLower(message), "alphanumeric") && !strings.Contains(strings.ToLower(message), "underscore") {
				t.Errorf("error message should mention character requirements: %s", message)
			}
		})
	}
}

// TestLogin_ValidCredentials tests successful login with correct email+password.
func TestLogin_ValidCredentials(t *testing.T) {
	mockRepo := newMockUserRepoForAuth()
	config := &OAuthConfig{
		JWTSecret:     "test-secret",
		JWTExpiry:     "15m",
		RefreshExpiry: "168h",
	}
	mockAuthMethodRepo := newMockAuthMethodRepoStub()
	handler := NewAuthHandlers(config, mockRepo, mockAuthMethodRepo)

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
	handler := NewAuthHandlers(config, mockRepo, mockAuthMethodRepo)

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
	handler := NewAuthHandlers(config, mockRepo, mockAuthMethodRepo)

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
	handler := NewAuthHandlers(config, mockRepo, mockAuthMethodRepo)

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
