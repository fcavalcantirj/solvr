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

func (m *MockUserAPIKeyRepository) Revoke(ctx context.Context, id, userID string) error {
	key, ok := m.keys[id]
	if !ok {
		return nil
	}
	if key.UserID != userID {
		return nil
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
