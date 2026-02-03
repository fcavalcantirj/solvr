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
	"github.com/fcavalcantirj/solvr/internal/models"
)

// Tests for POST /v1/users/me/api-keys (CreateAPIKey)
// Per prd-v2.json: "Accept name for the key, Generate secure random key (solvr_sk_xxx),
// Return full key ONCE (never stored in plain text), Store hashed version"

func TestCreateAPIKey_Success(t *testing.T) {
	repo := NewMockUserAPIKeyRepository()
	handler := NewUserAPIKeysHandler(repo)
	userID := "user-123"

	// Create request body
	body := strings.NewReader(`{"name": "Production Key"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/users/me/api-keys", body)
	req.Header.Set("Content-Type", "application/json")

	// Add JWT claims
	claims := &auth.Claims{
		UserID: userID,
		Email:  "test@example.com",
		Role:   models.UserRoleUser,
	}
	ctx := auth.ContextWithClaims(req.Context(), claims)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.CreateAPIKey(rr, req)

	// Assert: 201 Created
	if rr.Code != http.StatusCreated {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusCreated, rr.Code, rr.Body.String())
	}

	// Assert: response contains key
	var response map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	data, ok := response["data"].(map[string]interface{})
	if !ok {
		t.Fatal("response missing 'data' field")
	}

	// Check required fields
	if data["id"] == nil || data["id"] == "" {
		t.Error("expected id to be present")
	}
	if data["name"] != "Production Key" {
		t.Errorf("expected name 'Production Key', got %v", data["name"])
	}
	if data["key"] == nil || data["key"] == "" {
		t.Error("expected key to be present (shown only once)")
	}

	// Verify key format: solvr_sk_xxx
	key := data["key"].(string)
	if !strings.HasPrefix(key, "solvr_sk_") {
		t.Errorf("expected key to start with 'solvr_sk_', got %s", key)
	}

	// Verify created_at is present
	if data["created_at"] == nil {
		t.Error("expected created_at to be present")
	}
}

func TestCreateAPIKey_NoAuth(t *testing.T) {
	repo := NewMockUserAPIKeyRepository()
	handler := NewUserAPIKeysHandler(repo)

	// Create request WITHOUT claims
	body := strings.NewReader(`{"name": "Test Key"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/users/me/api-keys", body)
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.CreateAPIKey(rr, req)

	// Assert: 401 Unauthorized
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, rr.Code)
	}

	var response map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&response)
	errObj := response["error"].(map[string]interface{})
	if errObj["code"] != "UNAUTHORIZED" {
		t.Errorf("expected error code UNAUTHORIZED, got %v", errObj["code"])
	}
}

func TestCreateAPIKey_MissingName(t *testing.T) {
	repo := NewMockUserAPIKeyRepository()
	handler := NewUserAPIKeysHandler(repo)
	userID := "user-123"

	// Create request without name
	body := strings.NewReader(`{}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/users/me/api-keys", body)
	req.Header.Set("Content-Type", "application/json")

	claims := &auth.Claims{
		UserID: userID,
		Email:  "test@example.com",
		Role:   models.UserRoleUser,
	}
	ctx := auth.ContextWithClaims(req.Context(), claims)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.CreateAPIKey(rr, req)

	// Assert: 400 Bad Request
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusBadRequest, rr.Code, rr.Body.String())
	}

	var response map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&response)
	errObj := response["error"].(map[string]interface{})
	if errObj["code"] != "VALIDATION_ERROR" {
		t.Errorf("expected error code VALIDATION_ERROR, got %v", errObj["code"])
	}
}

func TestCreateAPIKey_EmptyName(t *testing.T) {
	repo := NewMockUserAPIKeyRepository()
	handler := NewUserAPIKeysHandler(repo)
	userID := "user-123"

	// Create request with empty name
	body := strings.NewReader(`{"name": ""}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/users/me/api-keys", body)
	req.Header.Set("Content-Type", "application/json")

	claims := &auth.Claims{
		UserID: userID,
		Email:  "test@example.com",
		Role:   models.UserRoleUser,
	}
	ctx := auth.ContextWithClaims(req.Context(), claims)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.CreateAPIKey(rr, req)

	// Assert: 400 Bad Request
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestCreateAPIKey_NameTooLong(t *testing.T) {
	repo := NewMockUserAPIKeyRepository()
	handler := NewUserAPIKeysHandler(repo)
	userID := "user-123"

	// Create request with name > 100 chars
	longName := strings.Repeat("a", 101)
	body := strings.NewReader(`{"name": "` + longName + `"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/users/me/api-keys", body)
	req.Header.Set("Content-Type", "application/json")

	claims := &auth.Claims{
		UserID: userID,
		Email:  "test@example.com",
		Role:   models.UserRoleUser,
	}
	ctx := auth.ContextWithClaims(req.Context(), claims)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.CreateAPIKey(rr, req)

	// Assert: 400 Bad Request
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestCreateAPIKey_InvalidJSON(t *testing.T) {
	repo := NewMockUserAPIKeyRepository()
	handler := NewUserAPIKeysHandler(repo)
	userID := "user-123"

	// Create request with invalid JSON
	body := strings.NewReader(`{invalid json}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/users/me/api-keys", body)
	req.Header.Set("Content-Type", "application/json")

	claims := &auth.Claims{
		UserID: userID,
		Email:  "test@example.com",
		Role:   models.UserRoleUser,
	}
	ctx := auth.ContextWithClaims(req.Context(), claims)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.CreateAPIKey(rr, req)

	// Assert: 400 Bad Request
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestCreateAPIKey_KeyIsStoredHashed(t *testing.T) {
	repo := NewMockUserAPIKeyRepository()
	handler := NewUserAPIKeysHandler(repo)
	userID := "user-123"

	body := strings.NewReader(`{"name": "Test Key"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/users/me/api-keys", body)
	req.Header.Set("Content-Type", "application/json")

	claims := &auth.Claims{
		UserID: userID,
		Email:  "test@example.com",
		Role:   models.UserRoleUser,
	}
	ctx := auth.ContextWithClaims(req.Context(), claims)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.CreateAPIKey(rr, req)

	// Get the returned key
	var response map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&response)
	data := response["data"].(map[string]interface{})
	returnedKey := data["key"].(string)

	// Get the stored key from repo
	keys, _ := repo.FindByUserID(context.Background(), userID)
	if len(keys) == 0 {
		t.Fatal("expected key to be stored in repo")
	}

	storedKeyHash := keys[0].KeyHash

	// Verify the stored hash is NOT the plain key
	if storedKeyHash == returnedKey {
		t.Error("key should be stored hashed, not plain text")
	}

	// Verify the hash starts with bcrypt identifier
	if !strings.HasPrefix(storedKeyHash, "$2a$") {
		t.Errorf("expected bcrypt hash (starting with $2a$), got %s", storedKeyHash)
	}
}

func TestCreateAPIKey_KeyStartsWithCorrectPrefix(t *testing.T) {
	repo := NewMockUserAPIKeyRepository()
	handler := NewUserAPIKeysHandler(repo)
	userID := "user-123"

	body := strings.NewReader(`{"name": "Test Key"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/users/me/api-keys", body)
	req.Header.Set("Content-Type", "application/json")

	claims := &auth.Claims{
		UserID: userID,
		Email:  "test@example.com",
		Role:   models.UserRoleUser,
	}
	ctx := auth.ContextWithClaims(req.Context(), claims)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.CreateAPIKey(rr, req)

	var response map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&response)
	data := response["data"].(map[string]interface{})
	key := data["key"].(string)

	// Key format per prd-v2.json is solvr_sk_xxx (sk for secret key)
	if !strings.HasPrefix(key, "solvr_sk_") {
		t.Errorf("expected key to start with 'solvr_sk_', got %s", key)
	}

	// Key should be reasonably long (base64 of 32 bytes + prefix)
	if len(key) < 40 {
		t.Errorf("key seems too short: %d chars", len(key))
	}
}

// Tests for DELETE /v1/users/me/api-keys/:id (RevokeAPIKey)
// Per prd-v2.json: "Soft delete the key, Immediately invalidate for auth, Return success"

func TestUserAPIKey_RevokeAPIKey_Success(t *testing.T) {
	repo := NewMockUserAPIKeyRepository()
	handler := NewUserAPIKeysHandler(repo)
	userID := "user-123"
	keyID := "key-1"
	now := time.Now()

	// Add a key to the repo
	repo.keys[keyID] = &models.UserAPIKey{
		ID:        keyID,
		UserID:    userID,
		Name:      "Test Key",
		KeyHash:   "hash",
		CreatedAt: now,
		UpdatedAt: now,
	}
	repo.keysByUser[userID] = []*models.UserAPIKey{repo.keys[keyID]}

	// Create request
	req := httptest.NewRequest(http.MethodDelete, "/v1/users/me/api-keys/"+keyID, nil)

	// Add JWT claims
	claims := &auth.Claims{
		UserID: userID,
		Email:  "test@example.com",
		Role:   models.UserRoleUser,
	}
	ctx := auth.ContextWithClaims(req.Context(), claims)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.RevokeAPIKey(rr, req, keyID)

	// Assert: 204 No Content
	if rr.Code != http.StatusNoContent {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusNoContent, rr.Code, rr.Body.String())
	}

	// Verify key is revoked
	key := repo.keys[keyID]
	if key.RevokedAt == nil {
		t.Error("expected key to be revoked (RevokedAt set)")
	}
}

func TestUserAPIKey_RevokeAPIKey_NoAuth(t *testing.T) {
	repo := NewMockUserAPIKeyRepository()
	handler := NewUserAPIKeysHandler(repo)

	// Create request WITHOUT claims
	req := httptest.NewRequest(http.MethodDelete, "/v1/users/me/api-keys/key-1", nil)

	rr := httptest.NewRecorder()
	handler.RevokeAPIKey(rr, req, "key-1")

	// Assert: 401 Unauthorized
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, rr.Code)
	}

	var response map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&response)
	errObj := response["error"].(map[string]interface{})
	if errObj["code"] != "UNAUTHORIZED" {
		t.Errorf("expected error code UNAUTHORIZED, got %v", errObj["code"])
	}
}

func TestUserAPIKey_RevokeAPIKey_NotFound(t *testing.T) {
	repo := NewMockUserAPIKeyRepository()
	handler := NewUserAPIKeysHandler(repo)
	userID := "user-123"

	// Key doesn't exist in repo
	req := httptest.NewRequest(http.MethodDelete, "/v1/users/me/api-keys/nonexistent", nil)

	claims := &auth.Claims{
		UserID: userID,
		Email:  "test@example.com",
		Role:   models.UserRoleUser,
	}
	ctx := auth.ContextWithClaims(req.Context(), claims)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.RevokeAPIKey(rr, req, "nonexistent")

	// Assert: 404 Not Found
	if rr.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusNotFound, rr.Code, rr.Body.String())
	}
}

func TestUserAPIKey_RevokeAPIKey_WrongUser(t *testing.T) {
	repo := NewMockUserAPIKeyRepository()
	handler := NewUserAPIKeysHandler(repo)
	ownerID := "owner-user"
	attackerID := "attacker-user"
	keyID := "key-1"
	now := time.Now()

	// Key belongs to owner
	repo.keys[keyID] = &models.UserAPIKey{
		ID:        keyID,
		UserID:    ownerID,
		Name:      "Owner's Key",
		KeyHash:   "hash",
		CreatedAt: now,
		UpdatedAt: now,
	}
	repo.keysByUser[ownerID] = []*models.UserAPIKey{repo.keys[keyID]}

	// Attacker tries to revoke
	req := httptest.NewRequest(http.MethodDelete, "/v1/users/me/api-keys/"+keyID, nil)

	claims := &auth.Claims{
		UserID: attackerID,
		Email:  "attacker@example.com",
		Role:   models.UserRoleUser,
	}
	ctx := auth.ContextWithClaims(req.Context(), claims)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.RevokeAPIKey(rr, req, keyID)

	// Assert: 404 Not Found (don't reveal that key exists for different user)
	if rr.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, rr.Code)
	}

	// Verify key is NOT revoked
	key := repo.keys[keyID]
	if key.RevokedAt != nil {
		t.Error("key should NOT be revoked (attacker shouldn't have access)")
	}
}

func TestUserAPIKey_RevokeAPIKey_AlreadyRevoked(t *testing.T) {
	repo := NewMockUserAPIKeyRepository()
	handler := NewUserAPIKeysHandler(repo)
	userID := "user-123"
	keyID := "key-1"
	now := time.Now()
	revokedAt := now.Add(-1 * time.Hour)

	// Key is already revoked
	repo.keys[keyID] = &models.UserAPIKey{
		ID:        keyID,
		UserID:    userID,
		Name:      "Revoked Key",
		KeyHash:   "hash",
		RevokedAt: &revokedAt,
		CreatedAt: now,
		UpdatedAt: now,
	}

	req := httptest.NewRequest(http.MethodDelete, "/v1/users/me/api-keys/"+keyID, nil)

	claims := &auth.Claims{
		UserID: userID,
		Email:  "test@example.com",
		Role:   models.UserRoleUser,
	}
	ctx := auth.ContextWithClaims(req.Context(), claims)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.RevokeAPIKey(rr, req, keyID)

	// Assert: 404 Not Found (already revoked keys are "not found")
	if rr.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
}
