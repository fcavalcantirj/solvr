package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/auth"
)

// ============================================================
// Logout Tests (PRD line 185 and 191)
// Per SPEC.md Part 5.2: POST /auth/logout -> Invalidate tokens
// ============================================================

// TestLogout_Success tests successful logout with valid JWT and refresh token.
func TestLogout_Success(t *testing.T) {
	cfg := &OAuthConfig{
		JWTSecret:     "test-jwt-secret-32-chars-long!!",
		JWTExpiry:     "15m",
		RefreshExpiry: "7d",
	}

	// Create mock token store with a valid token
	mockDB := &MockLogoutRefreshTokenDB{
		tokens:  map[string]*RefreshTokenRecordData{},
		deleted: []string{},
	}

	// Store a valid refresh token first
	testRefreshToken := "test-logout-refresh-token-abc123"
	tokenHash := hashRefreshTokenForTest(testRefreshToken)
	mockDB.tokens[tokenHash] = &RefreshTokenRecordData{
		ID:        "token-id-123",
		UserID:    "user-id-456",
		TokenHash: tokenHash,
		ExpiresAt: time.Now().Add(24 * time.Hour),
		CreatedAt: time.Now(),
	}

	handler := NewOAuthHandlersWithLogout(cfg, nil, mockDB, mockDB)

	// Create valid JWT for authentication
	jwt, err := auth.GenerateJWT(cfg.JWTSecret, "user-id-456", "test@example.com", "user", 15*time.Minute)
	if err != nil {
		t.Fatalf("failed to generate JWT: %v", err)
	}

	// Create request with refresh token
	body := `{"refresh_token":"test-logout-refresh-token-abc123"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/auth/logout", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+jwt)
	rec := httptest.NewRecorder()

	// Add claims to context (simulating JWT middleware)
	claims := &auth.Claims{
		UserID: "user-id-456",
		Email:  "test@example.com",
		Role:   "user",
	}
	ctx := auth.ContextWithClaims(req.Context(), claims)
	req = req.WithContext(ctx)

	handler.Logout(rec, req)

	// Should return 204 No Content per SPEC.md
	if rec.Code != http.StatusNoContent {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusNoContent, rec.Code, rec.Body.String())
	}

	// Body should be empty for 204
	if rec.Body.Len() != 0 {
		t.Errorf("expected empty body for 204, got: %s", rec.Body.String())
	}

	// Verify token was deleted from database
	if len(mockDB.deleted) != 1 {
		t.Errorf("expected 1 deleted token, got %d", len(mockDB.deleted))
	}
	if len(mockDB.deleted) > 0 && mockDB.deleted[0] != tokenHash {
		t.Errorf("expected deleted token hash %s, got %s", tokenHash, mockDB.deleted[0])
	}
}

// TestLogout_MissingRefreshToken tests logout without refresh token returns 400.
func TestLogout_MissingRefreshToken(t *testing.T) {
	cfg := &OAuthConfig{
		JWTSecret: "test-jwt-secret-32-chars-long!!",
	}

	mockDB := &MockLogoutRefreshTokenDB{
		tokens:  map[string]*RefreshTokenRecordData{},
		deleted: []string{},
	}

	handler := NewOAuthHandlersWithLogout(cfg, nil, mockDB, mockDB)

	// Create valid JWT for authentication
	jwt, err := auth.GenerateJWT(cfg.JWTSecret, "user-id-456", "test@example.com", "user", 15*time.Minute)
	if err != nil {
		t.Fatalf("failed to generate JWT: %v", err)
	}

	// Create request with empty body
	body := `{}`
	req := httptest.NewRequest(http.MethodPost, "/v1/auth/logout", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+jwt)
	rec := httptest.NewRecorder()

	// Add claims to context
	claims := &auth.Claims{
		UserID: "user-id-456",
		Email:  "test@example.com",
		Role:   "user",
	}
	ctx := auth.ContextWithClaims(req.Context(), claims)
	req = req.WithContext(ctx)

	handler.Logout(rec, req)

	// Should return 400 VALIDATION_ERROR
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusBadRequest, rec.Code, rec.Body.String())
	}

	var resp LogoutErrorResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Error.Code != "VALIDATION_ERROR" {
		t.Errorf("expected error code VALIDATION_ERROR, got %s", resp.Error.Code)
	}
}

// TestLogout_InvalidJSON tests logout with invalid JSON returns 400.
func TestLogout_InvalidJSON(t *testing.T) {
	cfg := &OAuthConfig{
		JWTSecret: "test-jwt-secret-32-chars-long!!",
	}

	mockDB := &MockLogoutRefreshTokenDB{
		tokens:  map[string]*RefreshTokenRecordData{},
		deleted: []string{},
	}

	handler := NewOAuthHandlersWithLogout(cfg, nil, mockDB, mockDB)

	// Create valid JWT for authentication
	jwt, err := auth.GenerateJWT(cfg.JWTSecret, "user-id-456", "test@example.com", "user", 15*time.Minute)
	if err != nil {
		t.Fatalf("failed to generate JWT: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/auth/logout", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+jwt)
	rec := httptest.NewRecorder()

	// Add claims to context
	claims := &auth.Claims{
		UserID: "user-id-456",
		Email:  "test@example.com",
		Role:   "user",
	}
	ctx := auth.ContextWithClaims(req.Context(), claims)
	req = req.WithContext(ctx)

	handler.Logout(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

// TestLogout_TokenNotFound tests logout with non-existent token still returns 204.
// Per best practices, logout should succeed even if token doesn't exist (idempotent).
func TestLogout_TokenNotFound(t *testing.T) {
	cfg := &OAuthConfig{
		JWTSecret: "test-jwt-secret-32-chars-long!!",
	}

	mockDB := &MockLogoutRefreshTokenDB{
		tokens:  map[string]*RefreshTokenRecordData{}, // Empty - token doesn't exist
		deleted: []string{},
	}

	handler := NewOAuthHandlersWithLogout(cfg, nil, mockDB, mockDB)

	// Create valid JWT for authentication
	jwt, err := auth.GenerateJWT(cfg.JWTSecret, "user-id-456", "test@example.com", "user", 15*time.Minute)
	if err != nil {
		t.Fatalf("failed to generate JWT: %v", err)
	}

	body := `{"refresh_token":"nonexistent-token"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/auth/logout", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+jwt)
	rec := httptest.NewRecorder()

	// Add claims to context
	claims := &auth.Claims{
		UserID: "user-id-456",
		Email:  "test@example.com",
		Role:   "user",
	}
	ctx := auth.ContextWithClaims(req.Context(), claims)
	req = req.WithContext(ctx)

	handler.Logout(rec, req)

	// Should return 204 even if token doesn't exist (idempotent logout)
	if rec.Code != http.StatusNoContent {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusNoContent, rec.Code, rec.Body.String())
	}
}

// TestLogout_NoAuth tests logout without JWT returns 401.
// Note: This would be handled by JWT middleware in production.
func TestLogout_NoAuth(t *testing.T) {
	cfg := &OAuthConfig{
		JWTSecret: "test-jwt-secret-32-chars-long!!",
	}

	mockDB := &MockLogoutRefreshTokenDB{
		tokens:  map[string]*RefreshTokenRecordData{},
		deleted: []string{},
	}

	handler := NewOAuthHandlersWithLogout(cfg, nil, mockDB, mockDB)

	body := `{"refresh_token":"some-token"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/auth/logout", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	// No Authorization header
	rec := httptest.NewRecorder()

	// No claims in context (simulating missing JWT middleware)
	handler.Logout(rec, req)

	// Should return 401 UNAUTHORIZED
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusUnauthorized, rec.Code, rec.Body.String())
	}

	var resp LogoutErrorResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Error.Code != "UNAUTHORIZED" {
		t.Errorf("expected error code UNAUTHORIZED, got %s", resp.Error.Code)
	}
}

// TestLogout_SubsequentRefreshFails tests that refresh fails after logout.
// This is the integration test for PRD line 191.
func TestLogout_SubsequentRefreshFails(t *testing.T) {
	cfg := &OAuthConfig{
		JWTSecret:     "test-jwt-secret-32-chars-long!!",
		JWTExpiry:     "15m",
		RefreshExpiry: "7d",
	}

	// Create mock DB that also implements RefreshTokenDBInterface
	mockDB := &MockLogoutRefreshTokenDB{
		tokens:  map[string]*RefreshTokenRecordData{},
		deleted: []string{},
	}

	mockUserRepo := &MockUserRepository{
		users: map[string]*UserData{
			"user-id-456": {
				ID:          "user-id-456",
				Username:    "testuser",
				Email:       "test@example.com",
				DisplayName: "Test User",
				Role:        "user",
			},
		},
	}

	// Store a valid refresh token
	testRefreshToken := "test-refresh-token-for-logout-integration"
	tokenHash := hashRefreshTokenForTest(testRefreshToken)
	mockDB.tokens[tokenHash] = &RefreshTokenRecordData{
		ID:        "token-id-123",
		UserID:    "user-id-456",
		TokenHash: tokenHash,
		ExpiresAt: time.Now().Add(24 * time.Hour),
		CreatedAt: time.Now(),
	}

	// Create handlers
	logoutHandler := NewOAuthHandlersWithLogout(cfg, nil, mockDB, mockDB)
	refreshHandler := NewOAuthHandlersWithRefresh(cfg, nil, mockDB, mockUserRepo)

	// Create valid JWT for authentication
	jwt, err := auth.GenerateJWT(cfg.JWTSecret, "user-id-456", "test@example.com", "user", 15*time.Minute)
	if err != nil {
		t.Fatalf("failed to generate JWT: %v", err)
	}

	// Step 1: Perform logout
	logoutBody := `{"refresh_token":"test-refresh-token-for-logout-integration"}`
	logoutReq := httptest.NewRequest(http.MethodPost, "/v1/auth/logout", strings.NewReader(logoutBody))
	logoutReq.Header.Set("Content-Type", "application/json")
	logoutReq.Header.Set("Authorization", "Bearer "+jwt)
	logoutRec := httptest.NewRecorder()

	// Add claims to context
	claims := &auth.Claims{
		UserID: "user-id-456",
		Email:  "test@example.com",
		Role:   "user",
	}
	ctx := auth.ContextWithClaims(logoutReq.Context(), claims)
	logoutReq = logoutReq.WithContext(ctx)

	logoutHandler.Logout(logoutRec, logoutReq)

	if logoutRec.Code != http.StatusNoContent {
		t.Fatalf("logout failed: expected 204, got %d. Body: %s", logoutRec.Code, logoutRec.Body.String())
	}

	// Step 2: Try to refresh with the same token (should fail)
	refreshBody := `{"refresh_token":"test-refresh-token-for-logout-integration"}`
	refreshReq := httptest.NewRequest(http.MethodPost, "/v1/auth/refresh", strings.NewReader(refreshBody))
	refreshReq.Header.Set("Content-Type", "application/json")
	refreshRec := httptest.NewRecorder()

	refreshHandler.RefreshToken(refreshRec, refreshReq)

	// Should return 401 UNAUTHORIZED because token was deleted
	if refreshRec.Code != http.StatusUnauthorized {
		t.Errorf("expected refresh to fail with 401 after logout, got %d. Body: %s", refreshRec.Code, refreshRec.Body.String())
	}

	var resp LogoutErrorResponse
	if err := json.NewDecoder(refreshRec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Error.Code != "UNAUTHORIZED" {
		t.Errorf("expected error code UNAUTHORIZED, got %s", resp.Error.Code)
	}
}

// LogoutErrorResponse is an error response for logout tests.
type LogoutErrorResponse struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// MockLogoutRefreshTokenDB is a mock implementation that supports both
// RefreshTokenDBInterface and deletion for logout.
type MockLogoutRefreshTokenDB struct {
	tokens  map[string]*RefreshTokenRecordData
	deleted []string
}

// GetByTokenHash looks up a token by its hash.
func (m *MockLogoutRefreshTokenDB) GetByTokenHash(ctx context.Context, tokenHash string) (*RefreshTokenRecordData, error) {
	if record, ok := m.tokens[tokenHash]; ok {
		return record, nil
	}
	return nil, nil // Not found
}

// DeleteByTokenHash deletes a token by its hash.
func (m *MockLogoutRefreshTokenDB) DeleteByTokenHash(ctx context.Context, tokenHash string) error {
	m.deleted = append(m.deleted, tokenHash)
	delete(m.tokens, tokenHash)
	return nil
}
