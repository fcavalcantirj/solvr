package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/auth"
	"github.com/fcavalcantirj/solvr/internal/models"
)

// --- Test Helpers ---

func addCheckpointsAgentContext(r *http.Request, agentID string, humanID *string) *http.Request {
	agent := &models.Agent{
		ID:          agentID,
		DisplayName: "Test Agent",
		HumanID:     humanID,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	ctx := auth.ContextWithAgent(r.Context(), agent)
	return r.WithContext(ctx)
}

func addCheckpointsHumanContext(r *http.Request, userID, role string) *http.Request {
	claims := &auth.Claims{
		UserID: userID,
		Role:   role,
	}
	ctx := auth.ContextWithClaims(r.Context(), claims)
	return r.WithContext(ctx)
}

// --- POST /v1/agents/me/checkpoints Tests ---

func TestCreateCheckpoint_Success(t *testing.T) {
	repo := NewMockPinRepository()
	ipfs := NewMockIPFSPinner()
	handler := NewCheckpointsHandler(repo, ipfs)

	body := map[string]interface{}{
		"cid":          "bafybeigdyrzt5sfp7udm7hu76uh7y26nf3efuylqabf3oclgtqy55fbzdi",
		"death_count":  "5",
		"memory_hash":  "sha256:abc",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/agents/me/checkpoints", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req = addCheckpointsAgentContext(req, "agent-test-001", nil)

	w := httptest.NewRecorder()
	handler.Create(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("expected status 202, got %d: %s", w.Code, w.Body.String())
	}

	// Verify response format (PinResponse)
	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)

	if resp["requestid"] == nil || resp["requestid"] == "" {
		t.Error("expected requestid in response")
	}
	if resp["status"] != "queued" {
		t.Errorf("expected status 'queued', got %v", resp["status"])
	}

	pin := resp["pin"].(map[string]interface{})
	if pin["cid"] != "bafybeigdyrzt5sfp7udm7hu76uh7y26nf3efuylqabf3oclgtqy55fbzdi" {
		t.Errorf("expected CID in pin info, got %v", pin["cid"])
	}

	// Verify auto-injected meta
	meta := pin["meta"].(map[string]interface{})
	if meta["type"] != "amcp_checkpoint" {
		t.Errorf("expected meta.type='amcp_checkpoint', got %v", meta["type"])
	}
	if meta["agent_id"] != "agent-test-001" {
		t.Errorf("expected meta.agent_id='agent-test-001', got %v", meta["agent_id"])
	}
	// Dynamic fields preserved
	if meta["death_count"] != "5" {
		t.Errorf("expected meta.death_count='5', got %v", meta["death_count"])
	}
	if meta["memory_hash"] != "sha256:abc" {
		t.Errorf("expected meta.memory_hash='sha256:abc', got %v", meta["memory_hash"])
	}

	// Verify repo was called correctly
	if repo.createdPin == nil {
		t.Fatal("expected pin to be created via repo")
	}
	if repo.createdPin.OwnerID != "agent-test-001" {
		t.Errorf("expected owner ID agent-test-001, got %s", repo.createdPin.OwnerID)
	}
	if repo.createdPin.OwnerType != "agent" {
		t.Errorf("expected owner type agent, got %s", repo.createdPin.OwnerType)
	}
}

func TestCreateCheckpoint_AutoName(t *testing.T) {
	repo := NewMockPinRepository()
	ipfs := NewMockIPFSPinner()
	handler := NewCheckpointsHandler(repo, ipfs)

	body := map[string]interface{}{
		"cid": "bafybeigdyrzt5sfp7udm7hu76uh7y26nf3efuylqabf3oclgtqy55fbzdi",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/agents/me/checkpoints", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req = addCheckpointsAgentContext(req, "agent-test-001", nil)

	w := httptest.NewRecorder()
	handler.Create(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("expected status 202, got %d: %s", w.Code, w.Body.String())
	}

	// Verify auto-generated name format: checkpoint_<CID_first8>_<YYYYMMDD>
	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)

	pin := resp["pin"].(map[string]interface{})
	name, ok := pin["name"].(string)
	if !ok || name == "" {
		t.Fatal("expected auto-generated name in response")
	}

	cidPrefix := "bafybeig"
	today := time.Now().UTC().Format("20060102")
	expectedName := "checkpoint_" + cidPrefix + "_" + today
	if name != expectedName {
		t.Errorf("expected auto-generated name %q, got %q", expectedName, name)
	}
}

func TestCreateCheckpoint_QuotaExceeded(t *testing.T) {
	repo := NewMockPinRepository()
	ipfs := NewMockIPFSPinner()
	storageRepo := NewMockStorageRepository()
	storageRepo.usedBytes = storageRepo.quotaBytes // At quota limit

	handler := NewCheckpointsHandler(repo, ipfs)
	handler.SetStorageRepo(storageRepo)

	body := map[string]interface{}{
		"cid": "bafybeigdyrzt5sfp7udm7hu76uh7y26nf3efuylqabf3oclgtqy55fbzdi",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/agents/me/checkpoints", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req = addCheckpointsAgentContext(req, "agent-test-001", nil)

	w := httptest.NewRecorder()
	handler.Create(w, req)

	if w.Code != http.StatusPaymentRequired {
		t.Errorf("expected status 402, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCreateCheckpoint_Unauthenticated(t *testing.T) {
	repo := NewMockPinRepository()
	ipfs := NewMockIPFSPinner()
	handler := NewCheckpointsHandler(repo, ipfs)

	body := map[string]interface{}{
		"cid": "bafybeigdyrzt5sfp7udm7hu76uh7y26nf3efuylqabf3oclgtqy55fbzdi",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/agents/me/checkpoints", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	// No auth context

	w := httptest.NewRecorder()
	handler.Create(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCreateCheckpoint_HumanCannotCreate(t *testing.T) {
	repo := NewMockPinRepository()
	ipfs := NewMockIPFSPinner()
	handler := NewCheckpointsHandler(repo, ipfs)

	body := map[string]interface{}{
		"cid": "bafybeigdyrzt5sfp7udm7hu76uh7y26nf3efuylqabf3oclgtqy55fbzdi",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/agents/me/checkpoints", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req = addCheckpointsHumanContext(req, "user-123", "user")

	w := httptest.NewRecorder()
	handler.Create(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected status 403 for human, got %d: %s", w.Code, w.Body.String())
	}
}

// --- GET /v1/agents/{id}/checkpoints Tests ---

func TestListCheckpoints_OwnAgent(t *testing.T) {
	repo := NewMockPinRepository()
	ipfs := NewMockIPFSPinner()
	handler := NewCheckpointsHandler(repo, ipfs)

	// Set up 3 checkpoint pins (sorted DESC by created_at)
	now := time.Now()
	pins := []models.Pin{
		{
			ID:        "pin-3",
			CID:       "bafybeigdyrzt5sfp7udm7hu76uh7y26nf3efuylqabf3oclgtqy55fbzdi",
			Status:    models.PinStatusPinned,
			Name:      "checkpoint_bafybeig_20260221",
			Meta:      map[string]string{"type": "amcp_checkpoint", "agent_id": "agent-test-001"},
			OwnerID:   "agent-test-001",
			OwnerType: "agent",
			CreatedAt: now,
		},
		{
			ID:        "pin-2",
			CID:       "bafkreihdwdcefgh4dqkjv67uzcmw7ojee6xedzdetojuzjevtenera28714",
			Status:    models.PinStatusPinned,
			Name:      "checkpoint_bafkreih_20260220",
			Meta:      map[string]string{"type": "amcp_checkpoint", "agent_id": "agent-test-001"},
			OwnerID:   "agent-test-001",
			OwnerType: "agent",
			CreatedAt: now.Add(-24 * time.Hour),
		},
		{
			ID:        "pin-1",
			CID:       "QmYwAPJzv5CZsnA625s3Xf2nemtYgPpHdWEz79ojWnPbdG",
			Status:    models.PinStatusPinned,
			Name:      "checkpoint_QmYwAPJz_20260219",
			Meta:      map[string]string{"type": "amcp_checkpoint", "agent_id": "agent-test-001"},
			OwnerID:   "agent-test-001",
			OwnerType: "agent",
			CreatedAt: now.Add(-48 * time.Hour),
		},
	}
	repo.SetPins(pins, 3)

	req := httptest.NewRequest(http.MethodGet, "/v1/agents/agent-test-001/checkpoints", nil)
	req = addCheckpointsAgentContext(req, "agent-test-001", nil)

	w := httptest.NewRecorder()
	handler.ListCheckpoints(w, req, "agent-test-001")

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)

	count := resp["count"].(float64)
	if int(count) != 3 {
		t.Errorf("expected count 3, got %v", count)
	}

	results := resp["results"].([]interface{})
	if len(results) != 3 {
		t.Errorf("expected 3 results, got %d", len(results))
	}
}

func TestListCheckpoints_ClaimingHuman(t *testing.T) {
	repo := NewMockPinRepository()
	ipfs := NewMockIPFSPinner()
	handler := NewCheckpointsHandler(repo, ipfs)

	humanID := "human-owner-123"
	agentID := "agent-test-001"

	agentFinderRepo := &MockAgentFinderRepo{
		agents: map[string]*models.Agent{
			agentID: {ID: agentID, HumanID: &humanID},
		},
	}
	handler.SetAgentFinderRepo(agentFinderRepo)

	repo.SetPins([]models.Pin{
		{
			ID:        "pin-1",
			CID:       "bafybeigdyrzt5sfp7udm7hu76uh7y26nf3efuylqabf3oclgtqy55fbzdi",
			Status:    models.PinStatusPinned,
			Name:      "checkpoint_bafybeig_20260221",
			Meta:      map[string]string{"type": "amcp_checkpoint", "agent_id": agentID},
			OwnerID:   agentID,
			OwnerType: "agent",
			CreatedAt: time.Now(),
		},
	}, 1)

	// Human JWT auth — claiming human
	req := httptest.NewRequest(http.MethodGet, "/v1/agents/"+agentID+"/checkpoints", nil)
	req = addCheckpointsHumanContext(req, humanID, "user")

	w := httptest.NewRecorder()
	handler.ListCheckpoints(w, req, agentID)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for claiming human, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	count := resp["count"].(float64)
	if int(count) != 1 {
		t.Errorf("expected count=1, got %v", count)
	}
}

func TestListCheckpoints_SiblingAgent(t *testing.T) {
	repo := NewMockPinRepository()
	ipfs := NewMockIPFSPinner()
	handler := NewCheckpointsHandler(repo, ipfs)

	humanID := "human-shared-owner"
	agentAID := "agent-a"
	agentBID := "agent-b"

	agentFinderRepo := &MockAgentFinderRepo{
		agents: map[string]*models.Agent{
			agentAID: {ID: agentAID, HumanID: &humanID},
			agentBID: {ID: agentBID, HumanID: &humanID},
		},
	}
	handler.SetAgentFinderRepo(agentFinderRepo)

	repo.SetPins([]models.Pin{}, 0)

	// Agent-A (sibling) requests Agent-B's checkpoints
	callerAgent := &models.Agent{ID: agentAID, HumanID: &humanID}
	req := httptest.NewRequest(http.MethodGet, "/v1/agents/"+agentBID+"/checkpoints", nil)
	ctx := auth.ContextWithAgent(req.Context(), callerAgent)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handler.ListCheckpoints(w, req, agentBID)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for sibling agent, got %d: %s", w.Code, w.Body.String())
	}
}

func TestListCheckpoints_LatestField(t *testing.T) {
	repo := NewMockPinRepository()
	ipfs := NewMockIPFSPinner()
	handler := NewCheckpointsHandler(repo, ipfs)

	now := time.Now()
	pins := []models.Pin{
		{
			ID:        "pin-latest",
			CID:       "bafybeigdyrzt5sfp7udm7hu76uh7y26nf3efuylqabf3oclgtqy55fbzdi",
			Status:    models.PinStatusPinned,
			Name:      "checkpoint_bafybeig_20260221",
			Meta:      map[string]string{"type": "amcp_checkpoint", "agent_id": "agent-test-001"},
			OwnerID:   "agent-test-001",
			OwnerType: "agent",
			CreatedAt: now,
		},
		{
			ID:        "pin-older",
			CID:       "QmYwAPJzv5CZsnA625s3Xf2nemtYgPpHdWEz79ojWnPbdG",
			Status:    models.PinStatusPinned,
			Name:      "checkpoint_QmYwAPJz_20260220",
			Meta:      map[string]string{"type": "amcp_checkpoint", "agent_id": "agent-test-001"},
			OwnerID:   "agent-test-001",
			OwnerType: "agent",
			CreatedAt: now.Add(-24 * time.Hour),
		},
	}
	repo.SetPins(pins, 2)

	req := httptest.NewRequest(http.MethodGet, "/v1/agents/agent-test-001/checkpoints", nil)
	req = addCheckpointsAgentContext(req, "agent-test-001", nil)

	w := httptest.NewRecorder()
	handler.ListCheckpoints(w, req, "agent-test-001")

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)

	// Verify latest field points to most recent checkpoint
	latest, ok := resp["latest"].(map[string]interface{})
	if !ok {
		t.Fatal("expected 'latest' field in response")
	}
	if latest["requestid"] != "pin-latest" {
		t.Errorf("expected latest.requestid='pin-latest', got %v", latest["requestid"])
	}
}

func TestListCheckpoints_Empty(t *testing.T) {
	repo := NewMockPinRepository()
	ipfs := NewMockIPFSPinner()
	handler := NewCheckpointsHandler(repo, ipfs)

	repo.SetPins([]models.Pin{}, 0)

	req := httptest.NewRequest(http.MethodGet, "/v1/agents/agent-test-001/checkpoints", nil)
	req = addCheckpointsAgentContext(req, "agent-test-001", nil)

	w := httptest.NewRecorder()
	handler.ListCheckpoints(w, req, "agent-test-001")

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)

	count := resp["count"].(float64)
	if int(count) != 0 {
		t.Errorf("expected count 0, got %v", count)
	}

	results := resp["results"].([]interface{})
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}

	// latest should be null
	if resp["latest"] != nil {
		t.Errorf("expected latest=null for empty checkpoints, got %v", resp["latest"])
	}
}

func TestListCheckpoints_Unauthorized(t *testing.T) {
	repo := NewMockPinRepository()
	ipfs := NewMockIPFSPinner()
	handler := NewCheckpointsHandler(repo, ipfs)

	req := httptest.NewRequest(http.MethodGet, "/v1/agents/agent-test-001/checkpoints", nil)
	// No auth

	w := httptest.NewRecorder()
	handler.ListCheckpoints(w, req, "agent-test-001")

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d: %s", w.Code, w.Body.String())
	}
}

func TestListCheckpoints_NonFamilyAgent_Returns403(t *testing.T) {
	repo := NewMockPinRepository()
	ipfs := NewMockIPFSPinner()
	handler := NewCheckpointsHandler(repo, ipfs)

	agentFinderRepo := &MockAgentFinderRepo{
		agents: map[string]*models.Agent{
			"other-agent": {ID: "other-agent"},
		},
	}
	handler.SetAgentFinderRepo(agentFinderRepo)

	// Agent trying to access a different agent's checkpoints (no family)
	agent := &models.Agent{ID: "agent-intruder"}
	req := httptest.NewRequest(http.MethodGet, "/v1/agents/other-agent/checkpoints", nil)
	ctx := auth.ContextWithAgent(req.Context(), agent)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handler.ListCheckpoints(w, req, "other-agent")

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCreateCheckpoint_WithExplicitName(t *testing.T) {
	repo := NewMockPinRepository()
	ipfs := NewMockIPFSPinner()
	handler := NewCheckpointsHandler(repo, ipfs)

	body := map[string]interface{}{
		"cid":  "bafybeigdyrzt5sfp7udm7hu76uh7y26nf3efuylqabf3oclgtqy55fbzdi",
		"name": "my-custom-checkpoint",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/agents/me/checkpoints", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req = addCheckpointsAgentContext(req, "agent-test-001", nil)

	w := httptest.NewRecorder()
	handler.Create(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("expected status 202, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)

	pin := resp["pin"].(map[string]interface{})
	if pin["name"] != "my-custom-checkpoint" {
		t.Errorf("expected explicit name 'my-custom-checkpoint', got %v", pin["name"])
	}
}

func TestCreateCheckpoint_MetaInjectionsCannotBeOverridden(t *testing.T) {
	repo := NewMockPinRepository()
	ipfs := NewMockIPFSPinner()
	handler := NewCheckpointsHandler(repo, ipfs)

	// Try to override auto-injected meta fields
	body := map[string]interface{}{
		"cid":      "bafybeigdyrzt5sfp7udm7hu76uh7y26nf3efuylqabf3oclgtqy55fbzdi",
		"type":     "something_else",
		"agent_id": "fake-agent",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/agents/me/checkpoints", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req = addCheckpointsAgentContext(req, "agent-test-001", nil)

	w := httptest.NewRecorder()
	handler.Create(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("expected status 202, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)

	pin := resp["pin"].(map[string]interface{})
	meta := pin["meta"].(map[string]interface{})

	// Auto-injected fields should override user-provided values
	if meta["type"] != "amcp_checkpoint" {
		t.Errorf("expected meta.type='amcp_checkpoint' (auto-injected), got %v", meta["type"])
	}
	if meta["agent_id"] != "agent-test-001" {
		t.Errorf("expected meta.agent_id='agent-test-001' (auto-injected), got %v", meta["agent_id"])
	}
}

func TestListCheckpoints_MetaFilterApplied(t *testing.T) {
	repo := NewMockPinRepository()
	ipfs := NewMockIPFSPinner()
	handler := NewCheckpointsHandler(repo, ipfs)

	repo.SetPins([]models.Pin{}, 0)

	req := httptest.NewRequest(http.MethodGet, "/v1/agents/agent-test-001/checkpoints", nil)
	req = addCheckpointsAgentContext(req, "agent-test-001", nil)

	w := httptest.NewRecorder()
	handler.ListCheckpoints(w, req, "agent-test-001")

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify the list query used meta filter for type=amcp_checkpoint
	if repo.lastListOpts == nil {
		t.Fatal("expected list options to be tracked")
	}
	if repo.lastListOpts.Meta == nil {
		t.Fatal("expected meta filter to be applied")
	}
	if repo.lastListOpts.Meta["type"] != "amcp_checkpoint" {
		t.Errorf("expected meta filter type=amcp_checkpoint, got %v", repo.lastListOpts.Meta["type"])
	}
}
