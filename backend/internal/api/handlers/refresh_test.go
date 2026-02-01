package handlers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// ============================================================
// Refresh Token Tests (PRD lines 181-184)
// ============================================================

// TestRefreshToken_Success tests successful token refresh.
func TestRefreshToken_Success(t *testing.T) {
	cfg := &OAuthConfig{
		JWTSecret:     "test-jwt-secret-32-chars-long!!",
		JWTExpiry:     "15m",
		RefreshExpiry: "7d",
	}

	// Create mock token store with a valid token
	mockDB := &MockRefreshTokenDB{
		tokens: map[string]*RefreshTokenRecordData{},
	}

	// Store a valid refresh token first
	testRefreshToken := "test-valid-refresh-token-abc123"
	tokenHash := hashRefreshTokenForTest(testRefreshToken)
	mockDB.tokens[tokenHash] = &RefreshTokenRecordData{
		ID:        "token-id-123",
		UserID:    "user-id-456",
		TokenHash: tokenHash,
		ExpiresAt: time.Now().Add(24 * time.Hour), // Not expired
		CreatedAt: time.Now(),
	}

	// Create mock user repository
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

	handler := NewOAuthHandlersWithRefresh(cfg, nil, mockDB, mockUserRepo)

	// Create request with refresh token
	body := `{"refresh_token":"test-valid-refresh-token-abc123"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/auth/refresh", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.RefreshToken(rec, req)

	// Should return 200 with new tokens
	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var resp RefreshSuccessResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v. Body: %s", err, rec.Body.String())
	}

	if resp.Data.AccessToken == "" {
		t.Error("expected access_token to be set")
	}
	if resp.Data.RefreshToken == "" {
		t.Error("expected refresh_token to be set")
	}
	if resp.Data.TokenType != "Bearer" {
		t.Errorf("expected token_type=Bearer, got %s", resp.Data.TokenType)
	}
}

// TestRefreshToken_InvalidToken tests refresh with invalid token returns 401.
func TestRefreshToken_InvalidToken(t *testing.T) {
	cfg := &OAuthConfig{
		JWTSecret:     "test-jwt-secret-32-chars-long!!",
		JWTExpiry:     "15m",
		RefreshExpiry: "7d",
	}

	// Create mock token store with no tokens
	mockDB := &MockRefreshTokenDB{
		tokens: map[string]*RefreshTokenRecordData{},
	}

	mockUserRepo := &MockUserRepository{
		users: map[string]*UserData{},
	}

	handler := NewOAuthHandlersWithRefresh(cfg, nil, mockDB, mockUserRepo)

	// Create request with invalid refresh token
	body := `{"refresh_token":"nonexistent-token"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/auth/refresh", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.RefreshToken(rec, req)

	// Should return 401 per SPEC.md
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusUnauthorized, rec.Code, rec.Body.String())
	}

	var resp RefreshErrorResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Error.Code != "UNAUTHORIZED" {
		t.Errorf("expected error code UNAUTHORIZED, got %s", resp.Error.Code)
	}
}

// TestRefreshToken_ExpiredToken tests refresh with expired token returns 401.
func TestRefreshToken_ExpiredToken(t *testing.T) {
	cfg := &OAuthConfig{
		JWTSecret:     "test-jwt-secret-32-chars-long!!",
		JWTExpiry:     "15m",
		RefreshExpiry: "7d",
	}

	// Create mock token store with an expired token
	mockDB := &MockRefreshTokenDB{
		tokens: map[string]*RefreshTokenRecordData{},
	}

	testRefreshToken := "test-expired-refresh-token"
	tokenHash := hashRefreshTokenForTest(testRefreshToken)
	mockDB.tokens[tokenHash] = &RefreshTokenRecordData{
		ID:        "token-id-expired",
		UserID:    "user-id-456",
		TokenHash: tokenHash,
		ExpiresAt: time.Now().Add(-1 * time.Hour), // Already expired
		CreatedAt: time.Now().Add(-7 * 24 * time.Hour),
	}

	mockUserRepo := &MockUserRepository{
		users: map[string]*UserData{},
	}

	handler := NewOAuthHandlersWithRefresh(cfg, nil, mockDB, mockUserRepo)

	body := `{"refresh_token":"test-expired-refresh-token"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/auth/refresh", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.RefreshToken(rec, req)

	// Should return 401 TOKEN_EXPIRED per SPEC.md
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusUnauthorized, rec.Code, rec.Body.String())
	}

	var resp RefreshErrorResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Error.Code != "TOKEN_EXPIRED" {
		t.Errorf("expected error code TOKEN_EXPIRED, got %s", resp.Error.Code)
	}
}

// TestRefreshToken_MissingToken tests refresh without token returns 400.
func TestRefreshToken_MissingToken(t *testing.T) {
	cfg := &OAuthConfig{
		JWTSecret:     "test-jwt-secret-32-chars-long!!",
		JWTExpiry:     "15m",
		RefreshExpiry: "7d",
	}

	mockDB := &MockRefreshTokenDB{tokens: map[string]*RefreshTokenRecordData{}}
	mockUserRepo := &MockUserRepository{users: map[string]*UserData{}}

	handler := NewOAuthHandlersWithRefresh(cfg, nil, mockDB, mockUserRepo)

	// Create request with empty body
	body := `{}`
	req := httptest.NewRequest(http.MethodPost, "/v1/auth/refresh", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.RefreshToken(rec, req)

	// Should return 400 VALIDATION_ERROR
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusBadRequest, rec.Code, rec.Body.String())
	}

	var resp RefreshErrorResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Error.Code != "VALIDATION_ERROR" {
		t.Errorf("expected error code VALIDATION_ERROR, got %s", resp.Error.Code)
	}
}

// TestRefreshToken_InvalidJSON tests refresh with invalid JSON returns 400.
func TestRefreshToken_InvalidJSON(t *testing.T) {
	cfg := &OAuthConfig{
		JWTSecret: "test-jwt-secret-32-chars-long!!",
	}

	mockDB := &MockRefreshTokenDB{tokens: map[string]*RefreshTokenRecordData{}}
	mockUserRepo := &MockUserRepository{users: map[string]*UserData{}}

	handler := NewOAuthHandlersWithRefresh(cfg, nil, mockDB, mockUserRepo)

	req := httptest.NewRequest(http.MethodPost, "/v1/auth/refresh", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.RefreshToken(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

// RefreshSuccessResponse is the success response for token refresh.
type RefreshSuccessResponse struct {
	Data struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		TokenType    string `json:"token_type"`
		ExpiresIn    int    `json:"expires_in"`
	} `json:"data"`
}

// RefreshErrorResponse is an error response for refresh token tests.
type RefreshErrorResponse struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// MockRefreshTokenDB is a mock implementation of RefreshTokenDBInterface.
type MockRefreshTokenDB struct {
	tokens  map[string]*RefreshTokenRecordData
	deleted []string
}

// GetByTokenHash looks up a token by its hash.
func (m *MockRefreshTokenDB) GetByTokenHash(ctx context.Context, tokenHash string) (*RefreshTokenRecordData, error) {
	if record, ok := m.tokens[tokenHash]; ok {
		return record, nil
	}
	return nil, nil // Not found
}

// MockUserRepository is a mock implementation of UserRepositoryInterface.
type MockUserRepository struct {
	users map[string]*UserData
}

// FindByID finds a user by ID.
func (m *MockUserRepository) FindByID(ctx context.Context, id string) (*UserData, error) {
	if user, ok := m.users[id]; ok {
		return user, nil
	}
	return nil, nil
}

// hashRefreshTokenForTest is a test helper that hashes tokens the same way the production code does.
func hashRefreshTokenForTest(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}
