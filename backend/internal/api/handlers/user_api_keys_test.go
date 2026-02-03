package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/auth"
	"github.com/fcavalcantirj/solvr/internal/models"
)

// MockUserAPIKeyRepository implements UserAPIKeyRepositoryInterface for testing
type MockUserAPIKeyRepository struct {
	keys        map[string]*models.UserAPIKey
	keysByUser  map[string][]*models.UserAPIKey
	createError error
	findError   error
}

func NewMockUserAPIKeyRepository() *MockUserAPIKeyRepository {
	return &MockUserAPIKeyRepository{
		keys:       make(map[string]*models.UserAPIKey),
		keysByUser: make(map[string][]*models.UserAPIKey),
	}
}

func (m *MockUserAPIKeyRepository) Create(ctx context.Context, key *models.UserAPIKey) (*models.UserAPIKey, error) {
	if m.createError != nil {
		return nil, m.createError
	}
	// Simulate ID generation and timestamps
	key.ID = "key-" + key.Name
	key.CreatedAt = time.Now()
	key.UpdatedAt = time.Now()
	m.keys[key.ID] = key
	m.keysByUser[key.UserID] = append(m.keysByUser[key.UserID], key)
	return key, nil
}

func (m *MockUserAPIKeyRepository) FindByUserID(ctx context.Context, userID string) ([]*models.UserAPIKey, error) {
	if m.findError != nil {
		return nil, m.findError
	}
	keys, ok := m.keysByUser[userID]
	if !ok {
		return []*models.UserAPIKey{}, nil
	}
	// Filter out revoked keys
	var activeKeys []*models.UserAPIKey
	for _, k := range keys {
		if k.IsActive() {
			activeKeys = append(activeKeys, k)
		}
	}
	if activeKeys == nil {
		activeKeys = []*models.UserAPIKey{}
	}
	return activeKeys, nil
}

func (m *MockUserAPIKeyRepository) FindByID(ctx context.Context, id string) (*models.UserAPIKey, error) {
	if m.findError != nil {
		return nil, m.findError
	}
	key, ok := m.keys[id]
	if !ok {
		return nil, nil
	}
	return key, nil
}

// errMockNotFound is used by the mock to signal not found
var errMockNotFound = errors.New("not found")

func (m *MockUserAPIKeyRepository) Revoke(ctx context.Context, id, userID string) error {
	key, ok := m.keys[id]
	if !ok {
		return errMockNotFound
	}
	if key.UserID != userID {
		return errMockNotFound
	}
	if key.RevokedAt != nil {
		// Already revoked
		return errMockNotFound
	}
	now := time.Now()
	key.RevokedAt = &now
	return nil
}

func (m *MockUserAPIKeyRepository) UpdateLastUsed(ctx context.Context, id string) error {
	key, ok := m.keys[id]
	if !ok {
		return nil
	}
	now := time.Now()
	key.LastUsedAt = &now
	return nil
}

// Tests for GET /v1/users/me/api-keys (ListAPIKeys)

func TestListAPIKeys_Success(t *testing.T) {
	// Setup: create mock with keys
	repo := NewMockUserAPIKeyRepository()
	userID := "user-123"

	// Add some keys
	now := time.Now()
	lastUsed := now.Add(-1 * time.Hour)
	repo.keys["key-1"] = &models.UserAPIKey{
		ID:         "key-1",
		UserID:     userID,
		Name:       "Production",
		KeyHash:    "hash1",
		LastUsedAt: &lastUsed,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	repo.keys["key-2"] = &models.UserAPIKey{
		ID:        "key-2",
		UserID:    userID,
		Name:      "Development",
		KeyHash:   "hash2",
		CreatedAt: now,
		UpdatedAt: now,
	}
	repo.keysByUser[userID] = []*models.UserAPIKey{repo.keys["key-1"], repo.keys["key-2"]}

	handler := NewUserAPIKeysHandler(repo)

	// Create request
	req := httptest.NewRequest(http.MethodGet, "/v1/users/me/api-keys", nil)

	// Add claims to context (simulating JWT middleware)
	claims := &auth.Claims{
		UserID: userID,
		Email:  "test@example.com",
		Role:   models.UserRoleUser,
	}
	ctx := auth.ContextWithClaims(req.Context(), claims)
	req = req.WithContext(ctx)

	// Execute
	rr := httptest.NewRecorder()
	handler.ListAPIKeys(rr, req)

	// Assert: 200 OK
	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	// Assert: response contains keys
	var response map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	data, ok := response["data"].([]interface{})
	if !ok {
		t.Fatal("response missing 'data' field or not array")
	}

	if len(data) != 2 {
		t.Errorf("expected 2 keys, got %d", len(data))
	}

	// Check first key has expected fields
	key1 := data[0].(map[string]interface{})
	if key1["id"] == "" {
		t.Error("expected key to have id")
	}
	if key1["name"] == "" {
		t.Error("expected key to have name")
	}
	if key1["key_preview"] == "" {
		t.Error("expected key to have key_preview")
	}
	// KeyHash should NOT be exposed
	if _, hasHash := key1["key_hash"]; hasHash {
		t.Error("key_hash should not be exposed in response")
	}
}

func TestListAPIKeys_NoAuth(t *testing.T) {
	repo := NewMockUserAPIKeyRepository()
	handler := NewUserAPIKeysHandler(repo)

	// Create request WITHOUT claims in context
	req := httptest.NewRequest(http.MethodGet, "/v1/users/me/api-keys", nil)

	// Execute
	rr := httptest.NewRecorder()
	handler.ListAPIKeys(rr, req)

	// Assert: 401 Unauthorized
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, rr.Code)
	}

	// Assert: error response
	var response map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	errObj, ok := response["error"].(map[string]interface{})
	if !ok {
		t.Fatal("response missing 'error' field")
	}

	if errObj["code"] != "UNAUTHORIZED" {
		t.Errorf("expected error code UNAUTHORIZED, got %q", errObj["code"])
	}
}

func TestListAPIKeys_Empty(t *testing.T) {
	// Setup: user with no keys
	repo := NewMockUserAPIKeyRepository()
	userID := "user-no-keys"

	handler := NewUserAPIKeysHandler(repo)

	// Create request
	req := httptest.NewRequest(http.MethodGet, "/v1/users/me/api-keys", nil)
	claims := &auth.Claims{
		UserID: userID,
		Email:  "test@example.com",
		Role:   models.UserRoleUser,
	}
	ctx := auth.ContextWithClaims(req.Context(), claims)
	req = req.WithContext(ctx)

	// Execute
	rr := httptest.NewRecorder()
	handler.ListAPIKeys(rr, req)

	// Assert: 200 OK with empty array
	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	data, ok := response["data"].([]interface{})
	if !ok {
		t.Fatal("response missing 'data' field or not array")
	}

	if len(data) != 0 {
		t.Errorf("expected 0 keys, got %d", len(data))
	}
}

func TestListAPIKeys_OnlyShowsOwnKeys(t *testing.T) {
	// Setup: create keys for two users
	repo := NewMockUserAPIKeyRepository()
	userID1 := "user-1"
	userID2 := "user-2"
	now := time.Now()

	// User 1's key
	repo.keys["key-1"] = &models.UserAPIKey{
		ID:        "key-1",
		UserID:    userID1,
		Name:      "User1Key",
		KeyHash:   "hash1",
		CreatedAt: now,
		UpdatedAt: now,
	}
	repo.keysByUser[userID1] = []*models.UserAPIKey{repo.keys["key-1"]}

	// User 2's key
	repo.keys["key-2"] = &models.UserAPIKey{
		ID:        "key-2",
		UserID:    userID2,
		Name:      "User2Key",
		KeyHash:   "hash2",
		CreatedAt: now,
		UpdatedAt: now,
	}
	repo.keysByUser[userID2] = []*models.UserAPIKey{repo.keys["key-2"]}

	handler := NewUserAPIKeysHandler(repo)

	// Create request as user1
	req := httptest.NewRequest(http.MethodGet, "/v1/users/me/api-keys", nil)
	claims := &auth.Claims{
		UserID: userID1,
		Email:  "user1@example.com",
		Role:   models.UserRoleUser,
	}
	ctx := auth.ContextWithClaims(req.Context(), claims)
	req = req.WithContext(ctx)

	// Execute
	rr := httptest.NewRecorder()
	handler.ListAPIKeys(rr, req)

	// Assert: only user1's key returned
	var response map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&response)
	data := response["data"].([]interface{})

	if len(data) != 1 {
		t.Errorf("expected 1 key, got %d", len(data))
	}

	key := data[0].(map[string]interface{})
	if key["name"] != "User1Key" {
		t.Errorf("expected User1Key, got %s", key["name"])
	}
}

func TestListAPIKeys_MasksKeyValue(t *testing.T) {
	// Setup: create a key
	repo := NewMockUserAPIKeyRepository()
	userID := "user-123"
	now := time.Now()

	repo.keys["key-1"] = &models.UserAPIKey{
		ID:        "key-1",
		UserID:    userID,
		Name:      "TestKey",
		KeyHash:   "$2a$10$somehashedvalue", // bcrypt hash
		CreatedAt: now,
		UpdatedAt: now,
	}
	repo.keysByUser[userID] = []*models.UserAPIKey{repo.keys["key-1"]}

	handler := NewUserAPIKeysHandler(repo)

	// Create request
	req := httptest.NewRequest(http.MethodGet, "/v1/users/me/api-keys", nil)
	claims := &auth.Claims{
		UserID: userID,
		Email:  "test@example.com",
		Role:   models.UserRoleUser,
	}
	ctx := auth.ContextWithClaims(req.Context(), claims)
	req = req.WithContext(ctx)

	// Execute
	rr := httptest.NewRecorder()
	handler.ListAPIKeys(rr, req)

	// Assert: key_preview is masked format
	var response map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&response)
	data := response["data"].([]interface{})
	key := data[0].(map[string]interface{})

	preview := key["key_preview"].(string)
	// Preview should be in format "solvr_...XXXX" (showing last 4 chars or similar mask)
	if preview == "" {
		t.Error("key_preview should not be empty")
	}
	// The preview should NOT contain the full hash
	if preview == "$2a$10$somehashedvalue" {
		t.Error("key_preview should not expose the hash")
	}
}

func TestListAPIKeys_IncludesTimestamps(t *testing.T) {
	// Setup
	repo := NewMockUserAPIKeyRepository()
	userID := "user-123"
	now := time.Now()
	lastUsed := now.Add(-1 * time.Hour)

	repo.keys["key-1"] = &models.UserAPIKey{
		ID:         "key-1",
		UserID:     userID,
		Name:       "TestKey",
		KeyHash:    "hash",
		LastUsedAt: &lastUsed,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	repo.keysByUser[userID] = []*models.UserAPIKey{repo.keys["key-1"]}

	handler := NewUserAPIKeysHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/users/me/api-keys", nil)
	claims := &auth.Claims{
		UserID: userID,
		Email:  "test@example.com",
		Role:   models.UserRoleUser,
	}
	ctx := auth.ContextWithClaims(req.Context(), claims)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.ListAPIKeys(rr, req)

	// Assert: timestamps are included
	var response map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&response)
	data := response["data"].([]interface{})
	key := data[0].(map[string]interface{})

	if key["created_at"] == nil {
		t.Error("expected created_at to be present")
	}
	if key["last_used_at"] == nil {
		t.Error("expected last_used_at to be present")
	}
}

func TestListAPIKeys_DoesNotShowRevokedKeys(t *testing.T) {
	// Setup: one active, one revoked key
	repo := NewMockUserAPIKeyRepository()
	userID := "user-123"
	now := time.Now()
	revokedAt := now.Add(-1 * time.Hour)

	activeKey := &models.UserAPIKey{
		ID:        "key-active",
		UserID:    userID,
		Name:      "ActiveKey",
		KeyHash:   "hash1",
		CreatedAt: now,
		UpdatedAt: now,
	}
	revokedKey := &models.UserAPIKey{
		ID:        "key-revoked",
		UserID:    userID,
		Name:      "RevokedKey",
		KeyHash:   "hash2",
		RevokedAt: &revokedAt,
		CreatedAt: now,
		UpdatedAt: now,
	}
	repo.keys["key-active"] = activeKey
	repo.keys["key-revoked"] = revokedKey
	repo.keysByUser[userID] = []*models.UserAPIKey{activeKey, revokedKey}

	handler := NewUserAPIKeysHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/users/me/api-keys", nil)
	claims := &auth.Claims{
		UserID: userID,
		Email:  "test@example.com",
		Role:   models.UserRoleUser,
	}
	ctx := auth.ContextWithClaims(req.Context(), claims)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.ListAPIKeys(rr, req)

	// Assert: only active key returned
	var response map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&response)
	data := response["data"].([]interface{})

	if len(data) != 1 {
		t.Errorf("expected 1 key (only active), got %d", len(data))
	}

	key := data[0].(map[string]interface{})
	if key["name"] != "ActiveKey" {
		t.Errorf("expected ActiveKey, got %s", key["name"])
	}
}

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
