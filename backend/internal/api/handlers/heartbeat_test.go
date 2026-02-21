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

// MockHeartbeatNotifRepo mocks the notification unread count.
type MockHeartbeatNotifRepo struct {
	agentUnreadCount int
	userUnreadCount  int
	err              error
}

func (m *MockHeartbeatNotifRepo) GetUnreadCountForAgent(ctx context.Context, agentID string) (int, error) {
	return m.agentUnreadCount, m.err
}

func (m *MockHeartbeatNotifRepo) GetUnreadCountForUser(ctx context.Context, userID string) (int, error) {
	return m.userUnreadCount, m.err
}

// MockHeartbeatStorageRepo mocks storage usage.
type MockHeartbeatStorageRepo struct {
	used  int64
	quota int64
	err   error
}

func (m *MockHeartbeatStorageRepo) GetStorageUsage(ctx context.Context, ownerID, ownerType string) (int64, int64, error) {
	return m.used, m.quota, m.err
}

func (m *MockHeartbeatStorageRepo) UpdateStorageUsed(ctx context.Context, ownerID, ownerType string, deltaBytes int64) error {
	return nil
}

// MockHeartbeatAgentRepo tracks UpdateLastSeen calls.
type MockHeartbeatAgentRepo struct {
	*MockAgentRepository
	lastSeenCalled  bool
	lastSeenAgentID string
}

func (m *MockHeartbeatAgentRepo) UpdateLastSeen(ctx context.Context, id string) error {
	m.lastSeenCalled = true
	m.lastSeenAgentID = id
	return nil
}

func TestHeartbeat_Agent_ReturnsFullStatus(t *testing.T) {
	agentRepo := &MockHeartbeatAgentRepo{MockAgentRepository: NewMockAgentRepository()}
	agent := &models.Agent{
		ID:                  "test_agent",
		DisplayName:         "Test Agent",
		Status:              "active",
		Reputation:          150,
		HasHumanBackedBadge: true,
		PinningQuotaBytes:   1073741824,
	}
	agentRepo.agents["test_agent"] = agent

	notifRepo := &MockHeartbeatNotifRepo{agentUnreadCount: 3}
	storageRepo := &MockHeartbeatStorageRepo{used: 6376, quota: 1073741824}

	handler := NewHeartbeatHandler(agentRepo, notifRepo, storageRepo)

	req := httptest.NewRequest("GET", "/v1/heartbeat", nil)
	ctx := context.WithValue(req.Context(), auth.AgentContextKey, agent)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handler.Heartbeat(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	// Check top-level status
	if resp["status"] != "ok" {
		t.Errorf("expected status 'ok', got %v", resp["status"])
	}

	// Check agent section
	agentData := resp["agent"].(map[string]interface{})
	if agentData["id"] != "test_agent" {
		t.Errorf("expected agent id 'test_agent', got %v", agentData["id"])
	}
	if agentData["display_name"] != "Test Agent" {
		t.Errorf("expected display_name 'Test Agent', got %v", agentData["display_name"])
	}
	if agentData["status"] != "active" {
		t.Errorf("expected status 'active', got %v", agentData["status"])
	}

	// Check notifications section
	notifs := resp["notifications"].(map[string]interface{})
	if int(notifs["unread_count"].(float64)) != 3 {
		t.Errorf("expected unread_count 3, got %v", notifs["unread_count"])
	}

	// Check storage section
	storage := resp["storage"].(map[string]interface{})
	if int64(storage["used_bytes"].(float64)) != 6376 {
		t.Errorf("expected used_bytes 6376, got %v", storage["used_bytes"])
	}
	if int64(storage["quota_bytes"].(float64)) != 1073741824 {
		t.Errorf("expected quota_bytes 1073741824, got %v", storage["quota_bytes"])
	}

	// Check platform section exists
	platform := resp["platform"].(map[string]interface{})
	if platform["timestamp"] == nil {
		t.Error("expected platform.timestamp to be present")
	}
}

func TestHeartbeat_Agent_UpdatesLastSeen(t *testing.T) {
	agentRepo := &MockHeartbeatAgentRepo{MockAgentRepository: NewMockAgentRepository()}
	agent := &models.Agent{ID: "seen_agent", Status: "active"}
	agentRepo.agents["seen_agent"] = agent

	handler := NewHeartbeatHandler(agentRepo, &MockHeartbeatNotifRepo{}, &MockHeartbeatStorageRepo{})

	req := httptest.NewRequest("GET", "/v1/heartbeat", nil)
	ctx := context.WithValue(req.Context(), auth.AgentContextKey, agent)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handler.Heartbeat(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	if !agentRepo.lastSeenCalled {
		t.Error("expected UpdateLastSeen to be called")
	}
	if agentRepo.lastSeenAgentID != "seen_agent" {
		t.Errorf("expected UpdateLastSeen called with 'seen_agent', got '%s'", agentRepo.lastSeenAgentID)
	}
}

func TestHeartbeat_Agent_IncludesUnreadCount(t *testing.T) {
	agentRepo := &MockHeartbeatAgentRepo{MockAgentRepository: NewMockAgentRepository()}
	agent := &models.Agent{ID: "notif_agent", Status: "active"}
	agentRepo.agents["notif_agent"] = agent

	notifRepo := &MockHeartbeatNotifRepo{agentUnreadCount: 7}
	handler := NewHeartbeatHandler(agentRepo, notifRepo, &MockHeartbeatStorageRepo{})

	req := httptest.NewRequest("GET", "/v1/heartbeat", nil)
	ctx := context.WithValue(req.Context(), auth.AgentContextKey, agent)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handler.Heartbeat(w, req)

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	notifs := resp["notifications"].(map[string]interface{})
	if int(notifs["unread_count"].(float64)) != 7 {
		t.Errorf("expected unread_count 7, got %v", notifs["unread_count"])
	}
}

func TestHeartbeat_Agent_IncludesStorage(t *testing.T) {
	agentRepo := &MockHeartbeatAgentRepo{MockAgentRepository: NewMockAgentRepository()}
	agent := &models.Agent{ID: "storage_agent", Status: "active"}
	agentRepo.agents["storage_agent"] = agent

	storageRepo := &MockHeartbeatStorageRepo{used: 524288000, quota: 1073741824}
	handler := NewHeartbeatHandler(agentRepo, &MockHeartbeatNotifRepo{}, storageRepo)

	req := httptest.NewRequest("GET", "/v1/heartbeat", nil)
	ctx := context.WithValue(req.Context(), auth.AgentContextKey, agent)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handler.Heartbeat(w, req)

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	storage := resp["storage"].(map[string]interface{})
	if int64(storage["used_bytes"].(float64)) != 524288000 {
		t.Errorf("expected used_bytes 524288000, got %v", storage["used_bytes"])
	}

	percentage := storage["percentage"].(float64)
	if percentage < 48.0 || percentage > 49.0 {
		t.Errorf("expected percentage ~48.83, got %v", percentage)
	}
}

func TestHeartbeat_Agent_TipsWhenSpecialtiesEmpty(t *testing.T) {
	agentRepo := &MockHeartbeatAgentRepo{MockAgentRepository: NewMockAgentRepository()}
	humanID := "owner-123"
	agent := &models.Agent{
		ID:          "tips_agent",
		Status:      "active",
		HumanID:     &humanID,
		Specialties: nil,
		Model:       "claude-opus-4",
	}
	// Set LastBriefingAt so briefing tip is NOT shown
	now := time.Now()
	agent.LastBriefingAt = &now
	agentRepo.agents["tips_agent"] = agent

	handler := NewHeartbeatHandler(agentRepo, &MockHeartbeatNotifRepo{}, &MockHeartbeatStorageRepo{})

	req := httptest.NewRequest("GET", "/v1/heartbeat", nil)
	ctx := context.WithValue(req.Context(), auth.AgentContextKey, agent)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handler.Heartbeat(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	tipsRaw, ok := resp["tips"]
	if !ok {
		t.Fatal("expected 'tips' field in response")
	}
	tips := tipsRaw.([]interface{})

	found := false
	for _, tip := range tips {
		if tipStr, ok := tip.(string); ok {
			if contains(tipStr, "specialties") {
				found = true
				break
			}
		}
	}
	if !found {
		t.Errorf("expected specialties tip in tips, got %v", tips)
	}
}

func TestHeartbeat_Agent_TipsWhenNeverBriefed(t *testing.T) {
	agentRepo := &MockHeartbeatAgentRepo{MockAgentRepository: NewMockAgentRepository()}
	humanID := "owner-123"
	agent := &models.Agent{
		ID:             "briefing_agent",
		Status:         "active",
		HumanID:        &humanID,
		Specialties:    []string{"go"},
		Model:          "claude-opus-4",
		LastBriefingAt: nil, // Never briefed
	}
	agentRepo.agents["briefing_agent"] = agent

	handler := NewHeartbeatHandler(agentRepo, &MockHeartbeatNotifRepo{}, &MockHeartbeatStorageRepo{})

	req := httptest.NewRequest("GET", "/v1/heartbeat", nil)
	ctx := context.WithValue(req.Context(), auth.AgentContextKey, agent)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handler.Heartbeat(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	tipsRaw, ok := resp["tips"]
	if !ok {
		t.Fatal("expected 'tips' field in response")
	}
	tips := tipsRaw.([]interface{})

	found := false
	for _, tip := range tips {
		if tipStr, ok := tip.(string); ok {
			if contains(tipStr, "briefing") || contains(tipStr, "/v1/me") {
				found = true
				break
			}
		}
	}
	if !found {
		t.Errorf("expected briefing tip in tips, got %v", tips)
	}
}

func TestHeartbeat_Agent_TipsWhenUnclaimed(t *testing.T) {
	agentRepo := &MockHeartbeatAgentRepo{MockAgentRepository: NewMockAgentRepository()}
	now := time.Now()
	agent := &models.Agent{
		ID:             "unclaimed_agent",
		Status:         "active",
		HumanID:        nil, // Unclaimed
		Specialties:    []string{"go"},
		Model:          "claude-opus-4",
		LastBriefingAt: &now,
	}
	agentRepo.agents["unclaimed_agent"] = agent

	handler := NewHeartbeatHandler(agentRepo, &MockHeartbeatNotifRepo{}, &MockHeartbeatStorageRepo{})

	req := httptest.NewRequest("GET", "/v1/heartbeat", nil)
	ctx := context.WithValue(req.Context(), auth.AgentContextKey, agent)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handler.Heartbeat(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	tipsRaw, ok := resp["tips"]
	if !ok {
		t.Fatal("expected 'tips' field in response")
	}
	tips := tipsRaw.([]interface{})

	found := false
	for _, tip := range tips {
		if tipStr, ok := tip.(string); ok {
			if contains(tipStr, "claim") || contains(tipStr, "+50 reputation") {
				found = true
				break
			}
		}
	}
	if !found {
		t.Errorf("expected claim tip in tips, got %v", tips)
	}
}

func TestHeartbeat_Agent_NoTipsWhenFullyConfigured(t *testing.T) {
	agentRepo := &MockHeartbeatAgentRepo{MockAgentRepository: NewMockAgentRepository()}
	humanID := "owner-123"
	now := time.Now()
	agent := &models.Agent{
		ID:             "configured_agent",
		Status:         "active",
		HumanID:        &humanID,
		Specialties:    []string{"go", "postgresql"},
		Model:          "claude-opus-4",
		LastBriefingAt: &now,
	}
	agentRepo.agents["configured_agent"] = agent

	handler := NewHeartbeatHandler(agentRepo, &MockHeartbeatNotifRepo{}, &MockHeartbeatStorageRepo{})

	req := httptest.NewRequest("GET", "/v1/heartbeat", nil)
	ctx := context.WithValue(req.Context(), auth.AgentContextKey, agent)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handler.Heartbeat(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	tipsRaw, ok := resp["tips"]
	if !ok {
		t.Fatal("expected 'tips' field in response")
	}
	tips := tipsRaw.([]interface{})

	if len(tips) != 0 {
		t.Errorf("expected empty tips for fully configured agent, got %v", tips)
	}
}

// MockHeartbeatCheckpointFinder mocks checkpoint lookups for heartbeat tests.
type MockHeartbeatCheckpointFinder struct {
	pin *models.Pin
	err error
}

func (m *MockHeartbeatCheckpointFinder) FindLatestCheckpoint(ctx context.Context, agentID string) (*models.Pin, error) {
	return m.pin, m.err
}

func TestHeartbeat_ShowsCheckpointInfo(t *testing.T) {
	agentRepo := &MockHeartbeatAgentRepo{MockAgentRepository: NewMockAgentRepository()}
	humanID := "owner-123"
	now := time.Now()
	agent := &models.Agent{
		ID:             "ckpt_agent",
		DisplayName:    "Checkpoint Agent",
		Status:         "active",
		Reputation:     100,
		HumanID:        &humanID,
		Specialties:    []string{"go"},
		Model:          "claude-opus-4",
		LastBriefingAt: &now,
	}
	agentRepo.agents["ckpt_agent"] = agent

	pinnedAt := now.Add(-1 * time.Hour)
	checkpointFinder := &MockHeartbeatCheckpointFinder{
		pin: &models.Pin{
			ID:        "chk-1",
			CID:       "bafyabc123",
			Status:    models.PinStatusPinned,
			Name:      "checkpoint_bafyabc1_20260221",
			Meta:      map[string]string{"type": "amcp_checkpoint", "agent_id": "ckpt_agent"},
			Delegates: []string{},
			OwnerID:   "ckpt_agent",
			OwnerType: "agent",
			CreatedAt: now.Add(-2 * time.Hour),
			PinnedAt:  &pinnedAt,
		},
	}

	handler := NewHeartbeatHandler(agentRepo, &MockHeartbeatNotifRepo{}, &MockHeartbeatStorageRepo{})
	handler.SetCheckpointFinder(checkpointFinder)

	req := httptest.NewRequest("GET", "/v1/heartbeat", nil)
	ctx := context.WithValue(req.Context(), auth.AgentContextKey, agent)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handler.Heartbeat(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	ckpt, ok := resp["checkpoint"]
	if !ok {
		t.Fatal("expected 'checkpoint' field in response")
	}
	if ckpt == nil {
		t.Fatal("expected checkpoint to be non-nil")
	}

	ckptData := ckpt.(map[string]interface{})
	if ckptData["cid"] != "bafyabc123" {
		t.Errorf("expected checkpoint cid 'bafyabc123', got %v", ckptData["cid"])
	}
	if ckptData["name"] != "checkpoint_bafyabc1_20260221" {
		t.Errorf("expected checkpoint name 'checkpoint_bafyabc1_20260221', got %v", ckptData["name"])
	}
	if ckptData["pinned_at"] == "" {
		t.Error("expected checkpoint pinned_at to be non-empty")
	}
}

func TestHeartbeat_NoCheckpoint(t *testing.T) {
	agentRepo := &MockHeartbeatAgentRepo{MockAgentRepository: NewMockAgentRepository()}
	humanID := "owner-123"
	now := time.Now()
	agent := &models.Agent{
		ID:             "no_ckpt_agent",
		Status:         "active",
		HumanID:        &humanID,
		Specialties:    []string{"go"},
		Model:          "claude-opus-4",
		LastBriefingAt: &now,
	}
	agentRepo.agents["no_ckpt_agent"] = agent

	checkpointFinder := &MockHeartbeatCheckpointFinder{
		pin: nil,
		err: nil,
	}

	handler := NewHeartbeatHandler(agentRepo, &MockHeartbeatNotifRepo{}, &MockHeartbeatStorageRepo{})
	handler.SetCheckpointFinder(checkpointFinder)

	req := httptest.NewRequest("GET", "/v1/heartbeat", nil)
	ctx := context.WithValue(req.Context(), auth.AgentContextKey, agent)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handler.Heartbeat(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	// checkpoint field should be null
	if resp["checkpoint"] != nil {
		t.Errorf("expected checkpoint to be nil when no checkpoint exists, got %v", resp["checkpoint"])
	}
}

func TestHeartbeat_CheckpointTip(t *testing.T) {
	agentRepo := &MockHeartbeatAgentRepo{MockAgentRepository: NewMockAgentRepository()}
	humanID := "owner-123"
	now := time.Now()
	agent := &models.Agent{
		ID:              "amcp_no_ckpt_agent",
		Status:          "active",
		HumanID:         &humanID,
		Specialties:     []string{"go"},
		Model:           "claude-opus-4",
		LastBriefingAt:  &now,
		HasAMCPIdentity: true, // Has AMCP identity
	}
	agentRepo.agents["amcp_no_ckpt_agent"] = agent

	// No checkpoint found
	checkpointFinder := &MockHeartbeatCheckpointFinder{pin: nil, err: nil}

	handler := NewHeartbeatHandler(agentRepo, &MockHeartbeatNotifRepo{}, &MockHeartbeatStorageRepo{})
	handler.SetCheckpointFinder(checkpointFinder)

	req := httptest.NewRequest("GET", "/v1/heartbeat", nil)
	ctx := context.WithValue(req.Context(), auth.AgentContextKey, agent)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handler.Heartbeat(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	tipsRaw, ok := resp["tips"]
	if !ok {
		t.Fatal("expected 'tips' field in response")
	}
	tips := tipsRaw.([]interface{})

	found := false
	for _, tip := range tips {
		if tipStr, ok := tip.(string); ok {
			if contains(tipStr, "checkpoint") && contains(tipStr, "POST /v1/agents/me/checkpoints") {
				found = true
				break
			}
		}
	}
	if !found {
		t.Errorf("expected checkpoint continuity tip, got %v", tips)
	}
}

func TestHeartbeat_Unauthenticated_Returns401(t *testing.T) {
	handler := NewHeartbeatHandler(
		&MockHeartbeatAgentRepo{MockAgentRepository: NewMockAgentRepository()},
		&MockHeartbeatNotifRepo{},
		&MockHeartbeatStorageRepo{},
	)

	req := httptest.NewRequest("GET", "/v1/heartbeat", nil)
	w := httptest.NewRecorder()
	handler.Heartbeat(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", w.Code, w.Body.String())
	}
}
