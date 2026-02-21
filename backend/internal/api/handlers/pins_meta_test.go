package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// --- Meta Filtering Tests (GET /v1/pins?meta=...) ---

func TestListPins_FilterByMetaSingleKey(t *testing.T) {
	repo := NewMockPinRepository()

	// Set up pins with different meta
	pin1 := createTestPin("pin-1", "QmTest1aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "user-123", "human")
	pin1.Meta = map[string]string{"type": "checkpoint"}
	pin2 := createTestPin("pin-2", "QmTest2aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "user-123", "human")
	pin2.Meta = map[string]string{"type": "checkpoint"}
	// pin3 has no matching meta — but mock returns filtered results, so just set 2
	repo.SetPins([]models.Pin{pin1, pin2}, 2)

	ipfs := NewMockIPFSPinner()
	handler := NewPinsHandler(repo, ipfs)

	req := httptest.NewRequest(http.MethodGet, `/v1/pins?meta={"type":"checkpoint"}`, nil)
	req = addPinsAuthContext(req, "user-123", "user")

	w := httptest.NewRecorder()
	handler.List(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)

	count := int(resp["count"].(float64))
	if count != 2 {
		t.Errorf("expected count 2, got %d", count)
	}

	results := resp["results"].([]interface{})
	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}

	// Verify meta was passed to repo
	if repo.lastListOpts == nil {
		t.Fatal("expected listOpts to be tracked")
	}
	if repo.lastListOpts.Meta == nil || repo.lastListOpts.Meta["type"] != "checkpoint" {
		t.Errorf("expected meta filter {type: checkpoint}, got %v", repo.lastListOpts.Meta)
	}
}

func TestListPins_FilterByMetaMultipleKeys(t *testing.T) {
	repo := NewMockPinRepository()
	pin1 := createTestPin("pin-1", "QmTest1aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "user-123", "human")
	pin1.Meta = map[string]string{"type": "checkpoint", "version": "1"}
	repo.SetPins([]models.Pin{pin1}, 1)

	ipfs := NewMockIPFSPinner()
	handler := NewPinsHandler(repo, ipfs)

	req := httptest.NewRequest(http.MethodGet, `/v1/pins?meta={"type":"checkpoint","version":"1"}`, nil)
	req = addPinsAuthContext(req, "user-123", "user")

	w := httptest.NewRecorder()
	handler.List(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify both meta keys were passed
	if repo.lastListOpts == nil {
		t.Fatal("expected listOpts to be tracked")
	}
	if repo.lastListOpts.Meta["type"] != "checkpoint" {
		t.Errorf("expected meta type=checkpoint, got %v", repo.lastListOpts.Meta["type"])
	}
	if repo.lastListOpts.Meta["version"] != "1" {
		t.Errorf("expected meta version=1, got %v", repo.lastListOpts.Meta["version"])
	}
}

func TestListPins_FilterByMetaNoMatch(t *testing.T) {
	repo := NewMockPinRepository()
	repo.SetPins([]models.Pin{}, 0) // no matches

	ipfs := NewMockIPFSPinner()
	handler := NewPinsHandler(repo, ipfs)

	req := httptest.NewRequest(http.MethodGet, `/v1/pins?meta={"type":"nonexistent"}`, nil)
	req = addPinsAuthContext(req, "user-123", "user")

	w := httptest.NewRecorder()
	handler.List(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)

	count := int(resp["count"].(float64))
	if count != 0 {
		t.Errorf("expected count 0, got %d", count)
	}

	results := resp["results"].([]interface{})
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestListPins_FilterByMetaCombinedWithStatus(t *testing.T) {
	repo := NewMockPinRepository()
	pin1 := createTestPin("pin-1", "QmTest1aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "user-123", "human")
	pin1.Meta = map[string]string{"type": "checkpoint"}
	pin1.Status = models.PinStatusPinned
	repo.SetPins([]models.Pin{pin1}, 1)

	ipfs := NewMockIPFSPinner()
	handler := NewPinsHandler(repo, ipfs)

	req := httptest.NewRequest(http.MethodGet, `/v1/pins?meta={"type":"checkpoint"}&status=pinned`, nil)
	req = addPinsAuthContext(req, "user-123", "user")

	w := httptest.NewRecorder()
	handler.List(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify both meta and status filters were passed
	if repo.lastListOpts == nil {
		t.Fatal("expected listOpts to be tracked")
	}
	if repo.lastListOpts.Meta["type"] != "checkpoint" {
		t.Errorf("expected meta type=checkpoint, got %v", repo.lastListOpts.Meta)
	}
	if repo.lastListOpts.Status != models.PinStatusPinned {
		t.Errorf("expected status filter pinned, got %v", repo.lastListOpts.Status)
	}
}

func TestListPins_InvalidMetaJSON(t *testing.T) {
	repo := NewMockPinRepository()
	ipfs := NewMockIPFSPinner()
	handler := NewPinsHandler(repo, ipfs)

	req := httptest.NewRequest(http.MethodGet, `/v1/pins?meta=not-valid-json`, nil)
	req = addPinsAuthContext(req, "user-123", "user")

	w := httptest.NewRecorder()
	handler.List(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d: %s", w.Code, w.Body.String())
	}
}

// --- Pin Name Auto-Generation Tests (POST /v1/pins) ---

func TestCreatePin_AutoGeneratesName(t *testing.T) {
	repo := NewMockPinRepository()
	ipfs := NewMockIPFSPinner()
	handler := NewPinsHandler(repo, ipfs)

	cid := "QmYwAPJzv5CZsnA625s3Xf2nemtYgPpHdWEz79ojWnPbdG"

	body := map[string]interface{}{
		"cid": cid,
		// No name provided — should auto-generate
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

	// Verify the created pin has auto-generated name
	if repo.createdPin == nil {
		t.Fatal("expected pin to be created")
	}

	name := repo.createdPin.Name
	// Format: pin_<CID_first8>_<YYYYMMDD>
	cidPrefix := cid[:8]
	expectedPrefix := "pin_" + cidPrefix + "_"
	if !strings.HasPrefix(name, expectedPrefix) {
		t.Errorf("expected auto-generated name starting with %q, got %q", expectedPrefix, name)
	}

	// Verify date part (YYYYMMDD format)
	today := time.Now().UTC().Format("20060102")
	expectedName := "pin_" + cidPrefix + "_" + today
	if name != expectedName {
		t.Errorf("expected auto-generated name %q, got %q", expectedName, name)
	}

	// Verify response also has the auto-generated name
	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	pin := resp["pin"].(map[string]interface{})
	if pin["name"] != expectedName {
		t.Errorf("expected name %q in response, got %v", expectedName, pin["name"])
	}
}

func TestCreatePin_PreservesExplicitName(t *testing.T) {
	repo := NewMockPinRepository()
	ipfs := NewMockIPFSPinner()
	handler := NewPinsHandler(repo, ipfs)

	body := map[string]interface{}{
		"cid":  "QmYwAPJzv5CZsnA625s3Xf2nemtYgPpHdWEz79ojWnPbdG",
		"name": "my-custom-name",
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

	// Verify explicit name was NOT overridden
	if repo.createdPin == nil {
		t.Fatal("expected pin to be created")
	}
	if repo.createdPin.Name != "my-custom-name" {
		t.Errorf("expected name 'my-custom-name', got %q", repo.createdPin.Name)
	}
}
