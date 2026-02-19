package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

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
