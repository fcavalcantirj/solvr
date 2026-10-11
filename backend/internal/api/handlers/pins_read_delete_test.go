package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// --- GET /v1/pins/:requestid Tests ---

func TestPinsHandler_GetByRequestID_Success(t *testing.T) {
	repo := NewMockPinRepository()
	pin := createTestPin("pin-uuid-1", "QmYwAPJzv5CZsnA625s3Xf2nemtYgPpHdWEz79ojWnPbdG", "user-123", "human")
	repo.SetPin(&pin)
	ipfs := NewMockIPFSPinner()
	handler := NewPinsHandler(repo, ipfs)

	req := httptest.NewRequest(http.MethodGet, "/v1/pins/pin-uuid-1", nil)
	req = addPinsRouteContext(req, "requestid", "pin-uuid-1")
	req = addPinsAuthContext(req, "user-123", "user")

	w := httptest.NewRecorder()
	handler.GetByRequestID(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Response is raw Pinning Service API format (no data envelope)
	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["requestid"] != "pin-uuid-1" {
		t.Errorf("expected requestid pin-uuid-1, got %v", resp["requestid"])
	}
}

func TestPinsHandler_GetByRequestID_NotFound(t *testing.T) {
	repo := NewMockPinRepository()
	// No pin set — returns ErrPinNotFound
	ipfs := NewMockIPFSPinner()
	handler := NewPinsHandler(repo, ipfs)

	req := httptest.NewRequest(http.MethodGet, "/v1/pins/nonexistent", nil)
	req = addPinsRouteContext(req, "requestid", "nonexistent")
	req = addPinsAuthContext(req, "user-123", "user")

	w := httptest.NewRecorder()
	handler.GetByRequestID(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPinsHandler_GetByRequestID_Forbidden(t *testing.T) {
	repo := NewMockPinRepository()
	pin := createTestPin("pin-uuid-1", "QmYwAPJzv5CZsnA625s3Xf2nemtYgPpHdWEz79ojWnPbdG", "user-123", "human")
	repo.SetPin(&pin)
	ipfs := NewMockIPFSPinner()
	handler := NewPinsHandler(repo, ipfs)

	req := httptest.NewRequest(http.MethodGet, "/v1/pins/pin-uuid-1", nil)
	req = addPinsRouteContext(req, "requestid", "pin-uuid-1")
	req = addPinsAuthContext(req, "user-456", "user") // different user

	w := httptest.NewRecorder()
	handler.GetByRequestID(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected status 403, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPinsHandler_GetByRequestID_NoAuth(t *testing.T) {
	repo := NewMockPinRepository()
	ipfs := NewMockIPFSPinner()
	handler := NewPinsHandler(repo, ipfs)

	req := httptest.NewRequest(http.MethodGet, "/v1/pins/pin-uuid-1", nil)
	req = addPinsRouteContext(req, "requestid", "pin-uuid-1")
	// No auth

	w := httptest.NewRecorder()
	handler.GetByRequestID(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d: %s", w.Code, w.Body.String())
	}
}

// --- GET /v1/pins Tests ---

func TestPinsHandler_List_Success(t *testing.T) {
	repo := NewMockPinRepository()
	pins := []models.Pin{
		createTestPin("pin-1", "QmTest1", "user-123", "human"),
		createTestPin("pin-2", "QmTest2", "user-123", "human"),
	}
	repo.SetPins(pins, 2)
	ipfs := NewMockIPFSPinner()
	handler := NewPinsHandler(repo, ipfs)

	req := httptest.NewRequest(http.MethodGet, "/v1/pins", nil)
	req = addPinsAuthContext(req, "user-123", "user")

	w := httptest.NewRecorder()
	handler.List(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)

	count := resp["count"].(float64)
	if int(count) != 2 {
		t.Errorf("expected count 2, got %v", count)
	}

	results := resp["results"].([]interface{})
	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}
}

func TestPinsHandler_List_WithStatusFilter(t *testing.T) {
	repo := NewMockPinRepository()
	pinnedPin := createTestPin("pin-1", "QmTest1", "user-123", "human")
	pinnedPin.Status = models.PinStatusPinned
	repo.SetPins([]models.Pin{pinnedPin}, 1)
	ipfs := NewMockIPFSPinner()
	handler := NewPinsHandler(repo, ipfs)

	req := httptest.NewRequest(http.MethodGet, "/v1/pins?status=pinned", nil)
	req = addPinsAuthContext(req, "user-123", "user")

	w := httptest.NewRecorder()
	handler.List(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPinsHandler_List_NoAuth(t *testing.T) {
	repo := NewMockPinRepository()
	ipfs := NewMockIPFSPinner()
	handler := NewPinsHandler(repo, ipfs)

	req := httptest.NewRequest(http.MethodGet, "/v1/pins", nil)

	w := httptest.NewRecorder()
	handler.List(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPinsHandler_List_WithCIDFilter(t *testing.T) {
	repo := NewMockPinRepository()
	pin := createTestPin("pin-1", "QmSpecificCID", "user-123", "human")
	repo.SetPins([]models.Pin{pin}, 1)
	ipfs := NewMockIPFSPinner()
	handler := NewPinsHandler(repo, ipfs)

	req := httptest.NewRequest(http.MethodGet, "/v1/pins?cid=QmSpecificCID", nil)
	req = addPinsAuthContext(req, "user-123", "user")

	w := httptest.NewRecorder()
	handler.List(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPinsHandler_List_Pagination(t *testing.T) {
	repo := NewMockPinRepository()
	repo.SetPins([]models.Pin{}, 0)
	ipfs := NewMockIPFSPinner()
	handler := NewPinsHandler(repo, ipfs)

	req := httptest.NewRequest(http.MethodGet, "/v1/pins?limit=5", nil)
	req = addPinsAuthContext(req, "user-123", "user")

	w := httptest.NewRecorder()
	handler.List(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
}

// --- DELETE /v1/pins/:requestid Tests ---

func TestPinsHandler_Delete_Success(t *testing.T) {
	repo := NewMockPinRepository()
	pin := createTestPin("pin-uuid-1", "QmYwAPJzv5CZsnA625s3Xf2nemtYgPpHdWEz79ojWnPbdG", "user-123", "human")
	repo.SetPin(&pin)
	ipfs := NewMockIPFSPinner()
	handler := NewPinsHandler(repo, ipfs)

	req := httptest.NewRequest(http.MethodDelete, "/v1/pins/pin-uuid-1", nil)
	req = addPinsRouteContext(req, "requestid", "pin-uuid-1")
	req = addPinsAuthContext(req, "user-123", "user")

	w := httptest.NewRecorder()
	handler.Delete(w, req)

	if w.Code != http.StatusAccepted {
		t.Errorf("expected status 202, got %d: %s", w.Code, w.Body.String())
	}

	// Verify repo Delete was called
	if repo.deletedID != "pin-uuid-1" {
		t.Errorf("expected delete called with pin-uuid-1, got %s", repo.deletedID)
	}
}

func TestPinsHandler_Delete_NotFound(t *testing.T) {
	repo := NewMockPinRepository()
	// No pin set
	ipfs := NewMockIPFSPinner()
	handler := NewPinsHandler(repo, ipfs)

	req := httptest.NewRequest(http.MethodDelete, "/v1/pins/nonexistent", nil)
	req = addPinsRouteContext(req, "requestid", "nonexistent")
	req = addPinsAuthContext(req, "user-123", "user")

	w := httptest.NewRecorder()
	handler.Delete(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPinsHandler_Delete_Forbidden(t *testing.T) {
	repo := NewMockPinRepository()
	pin := createTestPin("pin-uuid-1", "QmTest", "user-123", "human")
	repo.SetPin(&pin)
	ipfs := NewMockIPFSPinner()
	handler := NewPinsHandler(repo, ipfs)

	req := httptest.NewRequest(http.MethodDelete, "/v1/pins/pin-uuid-1", nil)
	req = addPinsRouteContext(req, "requestid", "pin-uuid-1")
	req = addPinsAuthContext(req, "user-456", "user") // different user

	w := httptest.NewRecorder()
	handler.Delete(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected status 403, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPinsHandler_Delete_NoAuth(t *testing.T) {
	repo := NewMockPinRepository()
	ipfs := NewMockIPFSPinner()
	handler := NewPinsHandler(repo, ipfs)

	req := httptest.NewRequest(http.MethodDelete, "/v1/pins/pin-uuid-1", nil)
	req = addPinsRouteContext(req, "requestid", "pin-uuid-1")

	w := httptest.NewRecorder()
	handler.Delete(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d: %s", w.Code, w.Body.String())
	}
}

// --- asyncPin Size Tracking Tests ---

func TestAsyncPin_UpdatesSizeAfterSuccess(t *testing.T) {
	repo := NewMockPinRepository()
	ipfs := NewMockIPFSPinner()
	ipfs.objectStatSize = 1048576 // 1MB
	handler := NewPinsHandler(repo, ipfs)

	// Set up a pin that GetByID can return (needed for owner lookup)
	repo.SetPin(&models.Pin{
		ID:        "pin-123",
		CID:       "QmYwAPJzv5CZsnA625s3Xf2nemtYgPpHdWEz79ojWnPbdG",
		OwnerID:   "user-123",
		OwnerType: "human",
	})

	// Call asyncPin directly (it's synchronous despite the name)
	handler.asyncPin("pin-123", "QmYwAPJzv5CZsnA625s3Xf2nemtYgPpHdWEz79ojWnPbdG")

	// Verify ObjectStat was called with the right CID
	if ipfs.objectStatCID != "QmYwAPJzv5CZsnA625s3Xf2nemtYgPpHdWEz79ojWnPbdG" {
		t.Errorf("expected ObjectStat called with CID, got %q", ipfs.objectStatCID)
	}

	// Verify UpdateStatusAndSize was called (not just UpdateStatus)
	if repo.updatedSizeID != "pin-123" {
		t.Errorf("expected UpdateStatusAndSize called with pin-123, got %q", repo.updatedSizeID)
	}
	if repo.updatedSizeSt != models.PinStatusPinned {
		t.Errorf("expected status pinned, got %v", repo.updatedSizeSt)
	}
	if repo.updatedSizeBytes != 1048576 {
		t.Errorf("expected size 1048576, got %d", repo.updatedSizeBytes)
	}
}

func TestAsyncPin_IncrementsStorageUsage(t *testing.T) {
	repo := NewMockPinRepository()
	ipfs := NewMockIPFSPinner()
	ipfs.objectStatSize = 2097152 // 2MB
	storageRepo := NewMockStorageRepository()
	storageRepo.usedBytes = 0 // start at 0

	handler := NewPinsHandler(repo, ipfs)
	handler.SetStorageRepo(storageRepo)

	// Set up pin for GetByID
	repo.SetPin(&models.Pin{
		ID:        "pin-456",
		CID:       "QmYwAPJzv5CZsnA625s3Xf2nemtYgPpHdWEz79ojWnPbdG",
		OwnerID:   "user-789",
		OwnerType: "human",
	})

	handler.asyncPin("pin-456", "QmYwAPJzv5CZsnA625s3Xf2nemtYgPpHdWEz79ojWnPbdG")

	// Verify storage was incremented
	if storageRepo.updatedOwnerID != "user-789" {
		t.Errorf("expected storage updated for user-789, got %q", storageRepo.updatedOwnerID)
	}
	if storageRepo.updatedDelta != 2097152 {
		t.Errorf("expected storage delta 2097152, got %d", storageRepo.updatedDelta)
	}
}

func TestAsyncPin_NoStorageUpdate_WhenObjectStatFails(t *testing.T) {
	repo := NewMockPinRepository()
	ipfs := NewMockIPFSPinner()
	ipfs.objectStatErr = context.DeadlineExceeded // ObjectStat fails
	storageRepo := NewMockStorageRepository()

	handler := NewPinsHandler(repo, ipfs)
	handler.SetStorageRepo(storageRepo)

	repo.SetPin(&models.Pin{
		ID:        "pin-789",
		CID:       "QmYwAPJzv5CZsnA625s3Xf2nemtYgPpHdWEz79ojWnPbdG",
		OwnerID:   "user-123",
		OwnerType: "human",
	})

	handler.asyncPin("pin-789", "QmYwAPJzv5CZsnA625s3Xf2nemtYgPpHdWEz79ojWnPbdG")

	// Pin should still succeed (status pinned) even if ObjectStat fails
	// But UpdateStatusAndSize should be called with 0 size
	if repo.updatedSizeSt != models.PinStatusPinned {
		t.Errorf("expected status pinned even on ObjectStat failure, got %v", repo.updatedSizeSt)
	}

	// Storage should NOT be updated when size is 0
	if storageRepo.updatedOwnerID != "" {
		t.Errorf("expected no storage update when ObjectStat fails, but got update for %q", storageRepo.updatedOwnerID)
	}
}

func TestDelete_DecrementsStorageUsage(t *testing.T) {
	repo := NewMockPinRepository()
	ipfs := NewMockIPFSPinner()
	storageRepo := NewMockStorageRepository()
	storageRepo.usedBytes = 5242880 // 5MB used

	handler := NewPinsHandler(repo, ipfs)
	handler.SetStorageRepo(storageRepo)

	// Pin with known size
	size := int64(1048576) // 1MB
	repo.SetPin(&models.Pin{
		ID:        "pin-del-1",
		CID:       "QmYwAPJzv5CZsnA625s3Xf2nemtYgPpHdWEz79ojWnPbdG",
		Status:    models.PinStatusPinned,
		OwnerID:   "user-123",
		OwnerType: "human",
		SizeBytes: &size,
	})

	req := httptest.NewRequest(http.MethodDelete, "/v1/pins/pin-del-1", nil)
	req = addPinsAuthContext(req, "user-123", "user")
	req = addPinsRouteContext(req, "requestid", "pin-del-1")

	w := httptest.NewRecorder()
	handler.Delete(w, req)

	if w.Code != http.StatusAccepted {
		t.Errorf("expected status 202, got %d: %s", w.Code, w.Body.String())
	}

	// Verify storage was decremented
	if storageRepo.updatedOwnerID != "user-123" {
		t.Errorf("expected storage decremented for user-123, got %q", storageRepo.updatedOwnerID)
	}
	if storageRepo.updatedDelta != -1048576 {
		t.Errorf("expected storage delta -1048576, got %d", storageRepo.updatedDelta)
	}
}

// --- CID Validation Tests ---

func TestIsValidCID(t *testing.T) {
	tests := []struct {
		name  string
		cid   string
		valid bool
	}{
		{"CIDv0 valid", "QmYwAPJzv5CZsnA625s3Xf2nemtYgPpHdWEz79ojWnPbdG", true},
		{"CIDv1 bafy valid", "bafybeigdyrzt5sfp7udm7hu76uh7y26nf3efuylqabf3oclgtqy55fbzdi", true},
		{"CIDv1 bafk valid", "bafkreihdwdcefgh4dqkjv67uzcmw7ojee6xedzdetojuzjevtenera28714", true},
		{"empty string", "", false},
		{"random string", "not-a-cid", false},
		{"too short Qm", "Qm", false},
		{"number only", "12345", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsValidCID(tt.cid)
			if got != tt.valid {
				t.Errorf("IsValidCID(%q) = %v, want %v", tt.cid, got, tt.valid)
			}
		})
	}
}
