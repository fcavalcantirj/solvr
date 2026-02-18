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
	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/go-chi/chi/v5"
)

// --- Mock PinRepository ---

// MockPinRepository implements PinRepositoryInterface for testing.
type MockPinRepository struct {
	pin        *models.Pin
	pins       []models.Pin
	total      int
	createErr  error
	getErr     error
	listErr    error
	updateErr  error
	deleteErr  error
	createdPin *models.Pin // tracks what was passed to Create
	deletedID  string      // tracks what was passed to Delete
	updatedID  string      // tracks what was passed to UpdateStatus
	updatedSt  models.PinStatus
}

func NewMockPinRepository() *MockPinRepository {
	return &MockPinRepository{
		pins: []models.Pin{},
	}
}

func (m *MockPinRepository) Create(ctx context.Context, pin *models.Pin) error {
	m.createdPin = pin
	if m.createErr != nil {
		return m.createErr
	}
	// Simulate DB populating fields
	pin.ID = "pin-test-uuid"
	pin.CreatedAt = time.Now()
	pin.UpdatedAt = time.Now()
	return nil
}

func (m *MockPinRepository) GetByID(ctx context.Context, id string) (*models.Pin, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	if m.pin == nil {
		return nil, db.ErrPinNotFound
	}
	return m.pin, nil
}

func (m *MockPinRepository) GetByCID(ctx context.Context, cid, ownerID string) (*models.Pin, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	if m.pin == nil {
		return nil, db.ErrPinNotFound
	}
	return m.pin, nil
}

func (m *MockPinRepository) ListByOwner(ctx context.Context, ownerID, ownerType string, opts models.PinListOptions) ([]models.Pin, int, error) {
	if m.listErr != nil {
		return nil, 0, m.listErr
	}
	return m.pins, m.total, nil
}

func (m *MockPinRepository) UpdateStatus(ctx context.Context, id string, status models.PinStatus) error {
	m.updatedID = id
	m.updatedSt = status
	if m.updateErr != nil {
		return m.updateErr
	}
	return nil
}

func (m *MockPinRepository) Delete(ctx context.Context, id string) error {
	m.deletedID = id
	if m.deleteErr != nil {
		return m.deleteErr
	}
	return nil
}

func (m *MockPinRepository) SetPin(pin *models.Pin) {
	m.pin = pin
}

func (m *MockPinRepository) SetPins(pins []models.Pin, total int) {
	m.pins = pins
	m.total = total
}

func (m *MockPinRepository) SetCreateError(err error) {
	m.createErr = err
}

func (m *MockPinRepository) SetGetError(err error) {
	m.getErr = err
}

func (m *MockPinRepository) SetDeleteError(err error) {
	m.deleteErr = err
}

// --- Mock IPFS Pinner ---

// MockIPFSPinner implements IPFSPinner for testing.
type MockIPFSPinner struct {
	pinErr       error
	unpinErr     error
	pinnedCIDs   []string // tracks what was pinned
	unpinnedCIDs []string // tracks what was unpinned
}

func NewMockIPFSPinner() *MockIPFSPinner {
	return &MockIPFSPinner{}
}

func (m *MockIPFSPinner) Pin(ctx context.Context, cid string) error {
	m.pinnedCIDs = append(m.pinnedCIDs, cid)
	return m.pinErr
}

func (m *MockIPFSPinner) Unpin(ctx context.Context, cid string) error {
	m.unpinnedCIDs = append(m.unpinnedCIDs, cid)
	return m.unpinErr
}

// Verify MockIPFSPinner implements IPFSPinner at compile time.
var _ IPFSPinner = (*MockIPFSPinner)(nil)

// --- Test Helpers ---

func addPinsAuthContext(r *http.Request, userID, role string) *http.Request {
	claims := &auth.Claims{
		UserID: userID,
		Role:   role,
	}
	ctx := auth.ContextWithClaims(r.Context(), claims)
	return r.WithContext(ctx)
}

func addPinsAgentContext(r *http.Request, agentID string) *http.Request {
	agent := &models.Agent{
		ID:          agentID,
		DisplayName: "Test Agent",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	ctx := auth.ContextWithAgent(r.Context(), agent)
	return r.WithContext(ctx)
}

func createTestPin(id, cid, ownerID, ownerType string) models.Pin {
	now := time.Now()
	return models.Pin{
		ID:        id,
		CID:       cid,
		Status:    models.PinStatusQueued,
		Name:      "test-pin",
		Origins:   []string{},
		Meta:      map[string]string{"key": "value"},
		Delegates: []string{},
		OwnerID:   ownerID,
		OwnerType: ownerType,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func addPinsRouteContext(r *http.Request, key, value string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, value)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

// --- POST /v1/pins Tests ---

func TestPinsHandler_Create_Success_Human(t *testing.T) {
	repo := NewMockPinRepository()
	ipfs := NewMockIPFSPinner()
	handler := NewPinsHandler(repo, ipfs)

	body := map[string]interface{}{
		"cid":  "QmYwAPJzv5CZsnA625s3Xf2nemtYgPpHdWEz79ojWnPbdG",
		"name": "my-test-file",
		"origins": []string{
			"/ip4/127.0.0.1/tcp/4001/p2p/12D3KooWTest",
		},
		"meta": map[string]string{
			"app": "solvr",
		},
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/pins", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req = addPinsAuthContext(req, "user-123", "user")

	w := httptest.NewRecorder()
	handler.Create(w, req)

	if w.Code != http.StatusAccepted {
		t.Errorf("expected status 202, got %d: %s", w.Code, w.Body.String())
	}

	// Response is raw Pinning Service API format (no data envelope)
	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)

	if resp["requestid"] == nil || resp["requestid"] == "" {
		t.Error("expected requestid in response")
	}
	if resp["status"] != "queued" {
		t.Errorf("expected status 'queued', got %v", resp["status"])
	}

	pin := resp["pin"].(map[string]interface{})
	if pin["cid"] != "QmYwAPJzv5CZsnA625s3Xf2nemtYgPpHdWEz79ojWnPbdG" {
		t.Errorf("expected CID in pin info, got %v", pin["cid"])
	}
	if pin["name"] != "my-test-file" {
		t.Errorf("expected name in pin info, got %v", pin["name"])
	}

	// Verify repo was called correctly
	if repo.createdPin == nil {
		t.Fatal("expected pin to be created via repo")
	}
	if repo.createdPin.CID != "QmYwAPJzv5CZsnA625s3Xf2nemtYgPpHdWEz79ojWnPbdG" {
		t.Errorf("expected CID to match, got %s", repo.createdPin.CID)
	}
	if repo.createdPin.OwnerID != "user-123" {
		t.Errorf("expected owner ID user-123, got %s", repo.createdPin.OwnerID)
	}
	if repo.createdPin.OwnerType != "human" {
		t.Errorf("expected owner type human, got %s", repo.createdPin.OwnerType)
	}
}

func TestPinsHandler_Create_Success_Agent(t *testing.T) {
	repo := NewMockPinRepository()
	ipfs := NewMockIPFSPinner()
	handler := NewPinsHandler(repo, ipfs)

	body := map[string]interface{}{
		"cid": "bafybeigdyrzt5sfp7udm7hu76uh7y26nf3efuylqabf3oclgtqy55fbzdi",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/pins", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req = addPinsAgentContext(req, "agent-test-001")

	w := httptest.NewRecorder()
	handler.Create(w, req)

	if w.Code != http.StatusAccepted {
		t.Errorf("expected status 202, got %d: %s", w.Code, w.Body.String())
	}

	// Verify agent owner info
	if repo.createdPin == nil {
		t.Fatal("expected pin to be created")
	}
	if repo.createdPin.OwnerID != "agent-test-001" {
		t.Errorf("expected owner ID agent-test-001, got %s", repo.createdPin.OwnerID)
	}
	if repo.createdPin.OwnerType != "agent" {
		t.Errorf("expected owner type agent, got %s", repo.createdPin.OwnerType)
	}
}

func TestPinsHandler_Create_NoAuth(t *testing.T) {
	repo := NewMockPinRepository()
	ipfs := NewMockIPFSPinner()
	handler := NewPinsHandler(repo, ipfs)

	body := map[string]interface{}{
		"cid": "QmYwAPJzv5CZsnA625s3Xf2nemtYgPpHdWEz79ojWnPbdG",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/pins", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	// No auth context

	w := httptest.NewRecorder()
	handler.Create(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPinsHandler_Create_InvalidJSON(t *testing.T) {
	repo := NewMockPinRepository()
	ipfs := NewMockIPFSPinner()
	handler := NewPinsHandler(repo, ipfs)

	req := httptest.NewRequest(http.MethodPost, "/v1/pins", bytes.NewReader([]byte("{invalid json")))
	req.Header.Set("Content-Type", "application/json")
	req = addPinsAuthContext(req, "user-123", "user")

	w := httptest.NewRecorder()
	handler.Create(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPinsHandler_Create_MissingCID(t *testing.T) {
	repo := NewMockPinRepository()
	ipfs := NewMockIPFSPinner()
	handler := NewPinsHandler(repo, ipfs)

	body := map[string]interface{}{
		"name": "no-cid-pin",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/pins", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req = addPinsAuthContext(req, "user-123", "user")

	w := httptest.NewRecorder()
	handler.Create(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	errObj := resp["error"].(map[string]interface{})
	if errObj["code"] != "VALIDATION_ERROR" {
		t.Errorf("expected VALIDATION_ERROR code, got %v", errObj["code"])
	}
}

func TestPinsHandler_Create_InvalidCID(t *testing.T) {
	repo := NewMockPinRepository()
	ipfs := NewMockIPFSPinner()
	handler := NewPinsHandler(repo, ipfs)

	body := map[string]interface{}{
		"cid": "not-a-valid-cid",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/pins", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req = addPinsAuthContext(req, "user-123", "user")

	w := httptest.NewRecorder()
	handler.Create(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	errObj := resp["error"].(map[string]interface{})
	if errObj["code"] != "VALIDATION_ERROR" {
		t.Errorf("expected VALIDATION_ERROR code, got %v", errObj["code"])
	}
}

func TestPinsHandler_Create_ValidCIDv0(t *testing.T) {
	repo := NewMockPinRepository()
	ipfs := NewMockIPFSPinner()
	handler := NewPinsHandler(repo, ipfs)

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
		t.Errorf("expected CIDv0 to be accepted, got status %d: %s", w.Code, w.Body.String())
	}
}

func TestPinsHandler_Create_ValidCIDv1(t *testing.T) {
	repo := NewMockPinRepository()
	ipfs := NewMockIPFSPinner()
	handler := NewPinsHandler(repo, ipfs)

	body := map[string]interface{}{
		"cid": "bafybeigdyrzt5sfp7udm7hu76uh7y26nf3efuylqabf3oclgtqy55fbzdi",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/pins", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req = addPinsAuthContext(req, "user-123", "user")

	w := httptest.NewRecorder()
	handler.Create(w, req)

	if w.Code != http.StatusAccepted {
		t.Errorf("expected CIDv1 to be accepted, got status %d: %s", w.Code, w.Body.String())
	}
}

func TestPinsHandler_Create_DuplicatePin(t *testing.T) {
	repo := NewMockPinRepository()
	repo.SetCreateError(db.ErrDuplicatePin)
	ipfs := NewMockIPFSPinner()
	handler := NewPinsHandler(repo, ipfs)

	body := map[string]interface{}{
		"cid": "QmYwAPJzv5CZsnA625s3Xf2nemtYgPpHdWEz79ojWnPbdG",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/pins", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req = addPinsAuthContext(req, "user-123", "user")

	w := httptest.NewRecorder()
	handler.Create(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("expected status 409, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPinsHandler_Create_InternalError(t *testing.T) {
	repo := NewMockPinRepository()
	repo.createErr = context.DeadlineExceeded
	ipfs := NewMockIPFSPinner()
	handler := NewPinsHandler(repo, ipfs)

	body := map[string]interface{}{
		"cid": "QmYwAPJzv5CZsnA625s3Xf2nemtYgPpHdWEz79ojWnPbdG",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/pins", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req = addPinsAuthContext(req, "user-123", "user")

	w := httptest.NewRecorder()
	handler.Create(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPinsHandler_Create_EmptyBody(t *testing.T) {
	repo := NewMockPinRepository()
	ipfs := NewMockIPFSPinner()
	handler := NewPinsHandler(repo, ipfs)

	req := httptest.NewRequest(http.MethodPost, "/v1/pins", bytes.NewReader([]byte("{}")))
	req.Header.Set("Content-Type", "application/json")
	req = addPinsAuthContext(req, "user-123", "user")

	w := httptest.NewRecorder()
	handler.Create(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for empty CID, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPinsHandler_Create_ResponseFormat(t *testing.T) {
	repo := NewMockPinRepository()
	ipfs := NewMockIPFSPinner()
	handler := NewPinsHandler(repo, ipfs)

	body := map[string]interface{}{
		"cid":  "QmYwAPJzv5CZsnA625s3Xf2nemtYgPpHdWEz79ojWnPbdG",
		"name": "test",
		"meta": map[string]string{"key": "val"},
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/pins", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req = addPinsAuthContext(req, "user-123", "user")

	w := httptest.NewRecorder()
	handler.Create(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", w.Code, w.Body.String())
	}

	// Check Content-Type
	ct := w.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %s", ct)
	}

	// Verify Pinning Service API response format (raw, no data envelope)
	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)

	// Must have: requestid, status, created, pin, delegates
	requiredFields := []string{"requestid", "status", "created", "pin", "delegates"}
	for _, field := range requiredFields {
		if resp[field] == nil {
			t.Errorf("missing required field '%s' in response", field)
		}
	}

	// delegates must be an array
	delegates, ok := resp["delegates"].([]interface{})
	if !ok {
		t.Error("delegates should be an array")
	}
	if delegates == nil {
		t.Error("delegates should not be nil")
	}

	// pin sub-object must have cid
	pinObj := resp["pin"].(map[string]interface{})
	if pinObj["cid"] != "QmYwAPJzv5CZsnA625s3Xf2nemtYgPpHdWEz79ojWnPbdG" {
		t.Errorf("expected CID in pin, got %v", pinObj["cid"])
	}
}

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
