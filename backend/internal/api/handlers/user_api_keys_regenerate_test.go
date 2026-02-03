package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/auth"
	"github.com/fcavalcantirj/solvr/internal/models"
)

// Tests for POST /v1/users/me/api-keys/:id/regenerate (RegenerateAPIKey)
// Per prd-v2.json: "Generate new key value (old one invalidated), Return new key ONCE,
// Keep same key ID/name for tracking, Log regeneration event"

func TestUserAPIKey_RegenerateAPIKey_Success(t *testing.T) {
	repo := NewMockUserAPIKeyRepository()
	handler := NewUserAPIKeysHandler(repo)
	userID := "user-123"
	keyID := "key-1"
	now := time.Now()
	oldHash := "$2a$10$oldhashvalue"

	// Add a key to the repo
	repo.keys[keyID] = &models.UserAPIKey{
		ID:        keyID,
		UserID:    userID,
		Name:      "Production Key",
		KeyHash:   oldHash,
		CreatedAt: now,
		UpdatedAt: now,
	}
	repo.keysByUser[userID] = []*models.UserAPIKey{repo.keys[keyID]}

	// Create request
	req := httptest.NewRequest(http.MethodPost, "/v1/users/me/api-keys/"+keyID+"/regenerate", nil)

	// Add JWT claims
	claims := &auth.Claims{
		UserID: userID,
		Email:  "test@example.com",
		Role:   models.UserRoleUser,
	}
	ctx := auth.ContextWithClaims(req.Context(), claims)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.RegenerateAPIKey(rr, req, keyID)

	// Assert: 200 OK
	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}

	// Assert: response contains new key
	var response map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	data, ok := response["data"].(map[string]interface{})
	if !ok {
		t.Fatal("response missing 'data' field")
	}

	// Check ID is same
	if data["id"] != keyID {
		t.Errorf("expected id %s, got %v", keyID, data["id"])
	}

	// Check name is preserved
	if data["name"] != "Production Key" {
		t.Errorf("expected name 'Production Key', got %v", data["name"])
	}

	// Check new key is present
	if data["key"] == nil || data["key"] == "" {
		t.Error("expected new key to be present")
	}

	// Verify key format: solvr_sk_xxx
	key := data["key"].(string)
	if !strings.HasPrefix(key, "solvr_sk_") {
		t.Errorf("expected key to start with 'solvr_sk_', got %s", key)
	}

	// Verify the stored hash is different (new key generated)
	newHash := repo.keys[keyID].KeyHash
	if newHash == oldHash {
		t.Error("expected key hash to change after regeneration")
	}
}

func TestUserAPIKey_RegenerateAPIKey_NoAuth(t *testing.T) {
	repo := NewMockUserAPIKeyRepository()
	handler := NewUserAPIKeysHandler(repo)

	// Create request WITHOUT claims
	req := httptest.NewRequest(http.MethodPost, "/v1/users/me/api-keys/key-1/regenerate", nil)

	rr := httptest.NewRecorder()
	handler.RegenerateAPIKey(rr, req, "key-1")

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

func TestUserAPIKey_RegenerateAPIKey_NotFound(t *testing.T) {
	repo := NewMockUserAPIKeyRepository()
	handler := NewUserAPIKeysHandler(repo)
	userID := "user-123"

	// Key doesn't exist in repo
	req := httptest.NewRequest(http.MethodPost, "/v1/users/me/api-keys/nonexistent/regenerate", nil)

	claims := &auth.Claims{
		UserID: userID,
		Email:  "test@example.com",
		Role:   models.UserRoleUser,
	}
	ctx := auth.ContextWithClaims(req.Context(), claims)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.RegenerateAPIKey(rr, req, "nonexistent")

	// Assert: 404 Not Found
	if rr.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusNotFound, rr.Code, rr.Body.String())
	}
}

func TestUserAPIKey_RegenerateAPIKey_WrongUser(t *testing.T) {
	repo := NewMockUserAPIKeyRepository()
	handler := NewUserAPIKeysHandler(repo)
	ownerID := "owner-user"
	attackerID := "attacker-user"
	keyID := "key-1"
	now := time.Now()
	oldHash := "$2a$10$oldhashvalue"

	// Key belongs to owner
	repo.keys[keyID] = &models.UserAPIKey{
		ID:        keyID,
		UserID:    ownerID,
		Name:      "Owner's Key",
		KeyHash:   oldHash,
		CreatedAt: now,
		UpdatedAt: now,
	}
	repo.keysByUser[ownerID] = []*models.UserAPIKey{repo.keys[keyID]}

	// Attacker tries to regenerate
	req := httptest.NewRequest(http.MethodPost, "/v1/users/me/api-keys/"+keyID+"/regenerate", nil)

	claims := &auth.Claims{
		UserID: attackerID,
		Email:  "attacker@example.com",
		Role:   models.UserRoleUser,
	}
	ctx := auth.ContextWithClaims(req.Context(), claims)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.RegenerateAPIKey(rr, req, keyID)

	// Assert: 404 Not Found (don't reveal that key exists for different user)
	if rr.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, rr.Code)
	}

	// Verify key hash is NOT changed
	if repo.keys[keyID].KeyHash != oldHash {
		t.Error("key hash should NOT change (attacker shouldn't have access)")
	}
}

func TestUserAPIKey_RegenerateAPIKey_AlreadyRevoked(t *testing.T) {
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

	req := httptest.NewRequest(http.MethodPost, "/v1/users/me/api-keys/"+keyID+"/regenerate", nil)

	claims := &auth.Claims{
		UserID: userID,
		Email:  "test@example.com",
		Role:   models.UserRoleUser,
	}
	ctx := auth.ContextWithClaims(req.Context(), claims)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.RegenerateAPIKey(rr, req, keyID)

	// Assert: 404 Not Found (revoked keys are not available)
	if rr.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
}

func TestUserAPIKey_RegenerateAPIKey_NewKeyDifferentFromOld(t *testing.T) {
	repo := NewMockUserAPIKeyRepository()
	handler := NewUserAPIKeysHandler(repo)
	userID := "user-123"
	keyID := "key-1"
	now := time.Now()
	oldHash := "$2a$10$oldhashvalue"

	repo.keys[keyID] = &models.UserAPIKey{
		ID:        keyID,
		UserID:    userID,
		Name:      "Test Key",
		KeyHash:   oldHash,
		CreatedAt: now,
		UpdatedAt: now,
	}
	repo.keysByUser[userID] = []*models.UserAPIKey{repo.keys[keyID]}

	req := httptest.NewRequest(http.MethodPost, "/v1/users/me/api-keys/"+keyID+"/regenerate", nil)

	claims := &auth.Claims{
		UserID: userID,
		Email:  "test@example.com",
		Role:   models.UserRoleUser,
	}
	ctx := auth.ContextWithClaims(req.Context(), claims)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.RegenerateAPIKey(rr, req, keyID)

	var response map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&response)
	data := response["data"].(map[string]interface{})
	newKey := data["key"].(string)

	// Verify the new key is stored hashed
	newHash := repo.keys[keyID].KeyHash
	if newHash == newKey {
		t.Error("key should be stored hashed, not plain text")
	}
	if !strings.HasPrefix(newHash, "$2a$") {
		t.Errorf("expected bcrypt hash (starting with $2a$), got %s", newHash)
	}
}
