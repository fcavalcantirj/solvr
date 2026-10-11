package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/auth"
	"github.com/fcavalcantirj/solvr/internal/models"
)

// MockAuthMethodRepository implements AuthMethodRepositoryInterface for testing
type MockAuthMethodRepository struct {
	methods map[string][]*models.AuthMethod
}

func NewMockAuthMethodRepository() *MockAuthMethodRepository {
	return &MockAuthMethodRepository{
		methods: make(map[string][]*models.AuthMethod),
	}
}

func (m *MockAuthMethodRepository) FindByUserID(ctx context.Context, userID string) ([]*models.AuthMethod, error) {
	methods, ok := m.methods[userID]
	if !ok {
		return []*models.AuthMethod{}, nil
	}
	return methods, nil
}

// Tests for GET /v1/me/auth-methods endpoint

func TestGetMyAuthMethods_SingleProvider(t *testing.T) {
	// Setup: create user with one auth method (Google)
	userRepo := NewMockMeUserRepository()
	authMethodRepo := NewMockAuthMethodRepository()

	userID := "test-user-id"
	authMethod := &models.AuthMethod{
		ID:             "method-1",
		UserID:         userID,
		AuthProvider:   "google",
		AuthProviderID: "google123",
		CreatedAt:      testTime.Add(-24 * time.Hour),
		LastUsedAt:     testTime,
	}
	authMethodRepo.methods[userID] = []*models.AuthMethod{authMethod}

	config := &OAuthConfig{JWTSecret: "test-secret-key"}
	handler := NewMeHandler(config, userRepo, nil, authMethodRepo, nil)

	// Create request with JWT
	req := httptest.NewRequest(http.MethodGet, "/v1/me/auth-methods", nil)
	claims := &auth.Claims{
		UserID: userID,
		Email:  "test@example.com",
		Role:   models.UserRoleUser,
	}
	ctx := auth.ContextWithClaims(req.Context(), claims)
	req = req.WithContext(ctx)

	// Execute
	rr := httptest.NewRecorder()
	handler.GetMyAuthMethods(rr, req)

	// Assert: 200 OK
	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	// Parse response
	var response map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	data, ok := response["data"].(map[string]interface{})
	if !ok {
		t.Fatal("response missing 'data' field")
	}

	authMethods, ok := data["auth_methods"].([]interface{})
	if !ok {
		t.Fatal("response missing 'auth_methods' field")
	}

	// Verify response contains auth method
	if len(authMethods) != 1 {
		t.Errorf("expected 1 auth method, got %d", len(authMethods))
	}

	method := authMethods[0].(map[string]interface{})
	if method["provider"] != "google" {
		t.Errorf("expected provider 'google', got %q", method["provider"])
	}
}

func TestGetMyAuthMethods_MultipleProviders(t *testing.T) {
	// Setup: create user with Google + email/password
	userRepo := NewMockMeUserRepository()
	authMethodRepo := NewMockAuthMethodRepository()

	userID := "multi-user-id"
	authMethodRepo.methods[userID] = []*models.AuthMethod{
		{
			ID:             "method-google",
			UserID:         userID,
			AuthProvider:   "google",
			AuthProviderID: "google123",
			CreatedAt:      testTime.Add(-48 * time.Hour),
			LastUsedAt:     testTime,
		},
		{
			ID:           "method-email",
			UserID:       userID,
			AuthProvider: "email",
			PasswordHash: "hashed-password",
			CreatedAt:    testTime.Add(-72 * time.Hour),
			LastUsedAt:   testTime.Add(-1 * time.Hour),
		},
	}

	config := &OAuthConfig{JWTSecret: "test-secret-key"}
	handler := NewMeHandler(config, userRepo, nil, authMethodRepo, nil)

	// Create request with JWT
	req := httptest.NewRequest(http.MethodGet, "/v1/me/auth-methods", nil)
	claims := &auth.Claims{
		UserID: userID,
		Email:  "test@example.com",
		Role:   models.UserRoleUser,
	}
	ctx := auth.ContextWithClaims(req.Context(), claims)
	req = req.WithContext(ctx)

	// Execute
	rr := httptest.NewRecorder()
	handler.GetMyAuthMethods(rr, req)

	// Assert: 200 OK
	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	// Parse response
	var response map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	data := response["data"].(map[string]interface{})
	authMethods := data["auth_methods"].([]interface{})

	// Verify response contains both auth methods
	if len(authMethods) != 2 {
		t.Errorf("expected 2 auth methods, got %d", len(authMethods))
	}

	// Check providers (order might vary)
	providers := make(map[string]bool)
	for _, m := range authMethods {
		method := m.(map[string]interface{})
		providers[method["provider"].(string)] = true
	}

	if !providers["google"] {
		t.Error("expected 'google' provider in response")
	}
	if !providers["email"] {
		t.Error("expected 'email' provider in response")
	}
}

func TestGetMyAuthMethods_EmptyList(t *testing.T) {
	// Setup: user with no auth methods (edge case)
	userRepo := NewMockMeUserRepository()
	authMethodRepo := NewMockAuthMethodRepository()

	userID := "empty-user-id"
	// Don't add any methods to authMethodRepo

	config := &OAuthConfig{JWTSecret: "test-secret-key"}
	handler := NewMeHandler(config, userRepo, nil, authMethodRepo, nil)

	// Create request with JWT
	req := httptest.NewRequest(http.MethodGet, "/v1/me/auth-methods", nil)
	claims := &auth.Claims{
		UserID: userID,
		Email:  "test@example.com",
		Role:   models.UserRoleUser,
	}
	ctx := auth.ContextWithClaims(req.Context(), claims)
	req = req.WithContext(ctx)

	// Execute
	rr := httptest.NewRecorder()
	handler.GetMyAuthMethods(rr, req)

	// Assert: 200 OK (empty list is valid)
	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	// Parse response
	var response map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	data := response["data"].(map[string]interface{})
	authMethods := data["auth_methods"].([]interface{})

	// Should return empty array, not null
	if len(authMethods) != 0 {
		t.Errorf("expected 0 auth methods, got %d", len(authMethods))
	}
}

func TestGetMyAuthMethods_Unauthorized(t *testing.T) {
	// Setup: no JWT in context
	userRepo := NewMockMeUserRepository()
	authMethodRepo := NewMockAuthMethodRepository()

	config := &OAuthConfig{JWTSecret: "test-secret-key"}
	handler := NewMeHandler(config, userRepo, nil, authMethodRepo, nil)

	// Create request WITHOUT claims
	req := httptest.NewRequest(http.MethodGet, "/v1/me/auth-methods", nil)

	// Execute
	rr := httptest.NewRecorder()
	handler.GetMyAuthMethods(rr, req)

	// Assert: 401 Unauthorized
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, rr.Code)
	}

	// Parse response
	var response map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	errObj := response["error"].(map[string]interface{})
	if errObj["code"] != "UNAUTHORIZED" {
		t.Errorf("expected error code UNAUTHORIZED, got %q", errObj["code"])
	}
}

func TestGetMyAuthMethods_ExcludesSensitiveFields(t *testing.T) {
	// Setup: verify password_hash and auth_provider_id are not exposed
	userRepo := NewMockMeUserRepository()
	authMethodRepo := NewMockAuthMethodRepository()

	userID := "secure-user-id"
	authMethodRepo.methods[userID] = []*models.AuthMethod{
		{
			ID:             "method-1",
			UserID:         userID,
			AuthProvider:   "google",
			AuthProviderID: "sensitive-oauth-id-12345",
			CreatedAt:      testTime,
			LastUsedAt:     testTime,
		},
		{
			ID:           "method-2",
			UserID:       userID,
			AuthProvider: "email",
			PasswordHash: "sensitive-bcrypt-hash",
			CreatedAt:    testTime,
			LastUsedAt:   testTime,
		},
	}

	config := &OAuthConfig{JWTSecret: "test-secret-key"}
	handler := NewMeHandler(config, userRepo, nil, authMethodRepo, nil)

	// Create request with JWT
	req := httptest.NewRequest(http.MethodGet, "/v1/me/auth-methods", nil)
	claims := &auth.Claims{
		UserID: userID,
		Email:  "test@example.com",
		Role:   models.UserRoleUser,
	}
	ctx := auth.ContextWithClaims(req.Context(), claims)
	req = req.WithContext(ctx)

	// Execute
	rr := httptest.NewRecorder()
	handler.GetMyAuthMethods(rr, req)

	// Assert: 200 OK
	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	// Parse response
	var response map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	data := response["data"].(map[string]interface{})
	authMethods := data["auth_methods"].([]interface{})

	// Verify sensitive fields are NOT in response
	for _, m := range authMethods {
		method := m.(map[string]interface{})

		if _, hasPasswordHash := method["password_hash"]; hasPasswordHash {
			t.Error("response should not contain 'password_hash' field")
		}

		if _, hasProviderID := method["auth_provider_id"]; hasProviderID {
			t.Error("response should not contain 'auth_provider_id' field")
		}
	}
}
