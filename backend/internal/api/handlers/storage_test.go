package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/auth"
	"github.com/fcavalcantirj/solvr/internal/models"
)

// --- Mock StorageRepository ---

// MockStorageRepository implements StorageRepositoryInterface for testing.
type MockStorageRepository struct {
	usedBytes  int64
	quotaBytes int64
	getErr     error
	updateErr  error

	// track calls
	updatedOwnerID   string
	updatedOwnerType string
	updatedDelta     int64
}

func NewMockStorageRepository() *MockStorageRepository {
	return &MockStorageRepository{
		quotaBytes: 104857600, // 100MB default
	}
}

func (m *MockStorageRepository) GetStorageUsage(ctx context.Context, ownerID, ownerType string) (used int64, quota int64, err error) {
	if m.getErr != nil {
		return 0, 0, m.getErr
	}
	return m.usedBytes, m.quotaBytes, nil
}

func (m *MockStorageRepository) UpdateStorageUsed(ctx context.Context, ownerID, ownerType string, deltaBytes int64) error {
	m.updatedOwnerID = ownerID
	m.updatedOwnerType = ownerType
	m.updatedDelta = deltaBytes
	if m.updateErr != nil {
		return m.updateErr
	}
	m.usedBytes += deltaBytes
	return nil
}

// Verify interface compliance
var _ StorageRepositoryInterface = (*MockStorageRepository)(nil)

// --- GET /v1/me/storage Tests ---

func TestStorageHandler_GetStorage_Human(t *testing.T) {
	repo := NewMockStorageRepository()
	repo.usedBytes = 52428800  // 50MB
	repo.quotaBytes = 104857600 // 100MB
	handler := NewStorageHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/me/storage", nil)
	req = addStorageAuthContext(req, "user-123", "user")

	w := httptest.NewRecorder()
	handler.GetStorage(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)

	data := resp["data"].(map[string]interface{})
	if int64(data["used"].(float64)) != 52428800 {
		t.Errorf("expected used=52428800, got %v", data["used"])
	}
	if int64(data["quota"].(float64)) != 104857600 {
		t.Errorf("expected quota=104857600, got %v", data["quota"])
	}
	pct := data["percentage"].(float64)
	if pct < 49.9 || pct > 50.1 {
		t.Errorf("expected percentage ~50.0, got %v", pct)
	}
}

func TestStorageHandler_GetStorage_Agent(t *testing.T) {
	repo := NewMockStorageRepository()
	repo.usedBytes = 1073741824 // 1GB
	repo.quotaBytes = 1073741824 // 1GB (AMCP agent)
	handler := NewStorageHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/me/storage", nil)
	req = addStorageAgentContext(req, "agent-test-001")

	w := httptest.NewRecorder()
	handler.GetStorage(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	data := resp["data"].(map[string]interface{})
	if int64(data["used"].(float64)) != 1073741824 {
		t.Errorf("expected used=1073741824, got %v", data["used"])
	}
	pct := data["percentage"].(float64)
	if pct < 99.9 || pct > 100.1 {
		t.Errorf("expected percentage ~100.0, got %v", pct)
	}
}

func TestStorageHandler_GetStorage_NoAuth(t *testing.T) {
	repo := NewMockStorageRepository()
	handler := NewStorageHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/me/storage", nil)
	// No auth context

	w := httptest.NewRecorder()
	handler.GetStorage(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d: %s", w.Code, w.Body.String())
	}
}

func TestStorageHandler_GetStorage_ZeroQuota(t *testing.T) {
	repo := NewMockStorageRepository()
	repo.usedBytes = 0
	repo.quotaBytes = 0 // no quota allocated
	handler := NewStorageHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/me/storage", nil)
	req = addStorageAuthContext(req, "user-123", "user")

	w := httptest.NewRecorder()
	handler.GetStorage(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	data := resp["data"].(map[string]interface{})
	// Zero quota means 0% usage (no division by zero)
	if data["percentage"].(float64) != 0 {
		t.Errorf("expected percentage=0 for zero quota, got %v", data["percentage"])
	}
}

func TestStorageHandler_GetStorage_RepoError(t *testing.T) {
	repo := NewMockStorageRepository()
	repo.getErr = context.DeadlineExceeded
	handler := NewStorageHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/me/storage", nil)
	req = addStorageAuthContext(req, "user-123", "user")

	w := httptest.NewRecorder()
	handler.GetStorage(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d: %s", w.Code, w.Body.String())
	}
}

// --- Pin Create with Quota Enforcement Tests ---

func TestPinsHandler_Create_QuotaExceeded(t *testing.T) {
	pinRepo := NewMockPinRepository()
	ipfs := NewMockIPFSPinner()
	storageRepo := NewMockStorageRepository()
	storageRepo.usedBytes = 104857600 // already at 100MB limit
	storageRepo.quotaBytes = 104857600

	handler := NewPinsHandler(pinRepo, ipfs)
	handler.SetStorageRepo(storageRepo)

	body := map[string]interface{}{
		"cid": "QmYwAPJzv5CZsnA625s3Xf2nemtYgPpHdWEz79ojWnPbdG",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/pins", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req = addPinsAuthContext(req, "user-123", "user")

	w := httptest.NewRecorder()
	handler.Create(w, req)

	// Should return 402 Payment Required when quota exceeded
	if w.Code != http.StatusPaymentRequired {
		t.Errorf("expected status 402, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPinsHandler_Create_QuotaAllowed(t *testing.T) {
	pinRepo := NewMockPinRepository()
	ipfs := NewMockIPFSPinner()
	storageRepo := NewMockStorageRepository()
	storageRepo.usedBytes = 0
	storageRepo.quotaBytes = 104857600 // 100MB, plenty of space

	handler := NewPinsHandler(pinRepo, ipfs)
	handler.SetStorageRepo(storageRepo)

	body := map[string]interface{}{
		"cid": "QmYwAPJzv5CZsnA625s3Xf2nemtYgPpHdWEz79ojWnPbdG",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/pins", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req = addPinsAuthContext(req, "user-123", "user")

	w := httptest.NewRecorder()
	handler.Create(w, req)

	// Should pass quota check and create normally
	if w.Code != http.StatusAccepted {
		t.Errorf("expected status 202, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPinsHandler_Create_NoStorageRepo_Allowed(t *testing.T) {
	// When no storage repo is set (backward compat), pinning should still work
	pinRepo := NewMockPinRepository()
	ipfs := NewMockIPFSPinner()

	handler := NewPinsHandler(pinRepo, ipfs)
	// No SetStorageRepo call — should skip quota check

	body := map[string]interface{}{
		"cid": "QmYwAPJzv5CZsnA625s3Xf2nemtYgPpHdWEz79ojWnPbdG",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/pins", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req = addPinsAuthContext(req, "user-123", "user")

	w := httptest.NewRecorder()
	handler.Create(w, req)

	if w.Code != http.StatusAccepted {
		t.Errorf("expected status 202 without storage repo, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPinsHandler_Create_ZeroQuota_Blocked(t *testing.T) {
	pinRepo := NewMockPinRepository()
	ipfs := NewMockIPFSPinner()
	storageRepo := NewMockStorageRepository()
	storageRepo.usedBytes = 0
	storageRepo.quotaBytes = 0 // no quota at all

	handler := NewPinsHandler(pinRepo, ipfs)
	handler.SetStorageRepo(storageRepo)

	body := map[string]interface{}{
		"cid": "QmYwAPJzv5CZsnA625s3Xf2nemtYgPpHdWEz79ojWnPbdG",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/pins", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req = addPinsAuthContext(req, "user-123", "user")

	w := httptest.NewRecorder()
	handler.Create(w, req)

	// Zero quota means no pinning allowed
	if w.Code != http.StatusPaymentRequired {
		t.Errorf("expected status 402 for zero quota, got %d: %s", w.Code, w.Body.String())
	}
}

// --- Test Helpers ---

func addStorageAuthContext(r *http.Request, userID, role string) *http.Request {
	claims := &auth.Claims{
		UserID: userID,
		Role:   role,
	}
	ctx := auth.ContextWithClaims(r.Context(), claims)
	return r.WithContext(ctx)
}

func addStorageAgentContext(r *http.Request, agentID string) *http.Request {
	agent := &models.Agent{
		ID:          agentID,
		DisplayName: "Test Agent",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	ctx := auth.ContextWithAgent(r.Context(), agent)
	return r.WithContext(ctx)
}
