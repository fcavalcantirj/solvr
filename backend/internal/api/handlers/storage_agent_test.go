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

// MockStorageRepo implements StorageRepositoryInterface for agent storage tests.
type MockStorageRepo struct {
	used  int64
	quota int64
	err   error
}

func (m *MockStorageRepo) GetStorageUsage(ctx context.Context, ownerID, ownerType string) (int64, int64, error) {
	if m.err != nil {
		return 0, 0, m.err
	}
	return m.used, m.quota, nil
}

func (m *MockStorageRepo) UpdateStorageUsed(ctx context.Context, ownerID, ownerType string, deltaBytes int64) error {
	return nil
}

func TestGetAgentStorage_HumanOwnerSuccess(t *testing.T) {
	humanID := "user_human_123"
	agentID := "agent_test_1"

	agentFinderRepo := &MockAgentFinderRepo{
		agents: map[string]*models.Agent{
			agentID: {
				ID:      agentID,
				HumanID: &humanID,
			},
		},
	}

	storageRepo := &MockStorageRepo{
		used:  52428800, // 50MB
		quota: 1073741824, // 1GB
	}

	handler := NewStorageHandler(storageRepo)
	handler.SetAgentFinderRepo(agentFinderRepo)

	// Human JWT claims
	claims := &auth.Claims{UserID: humanID}
	req := httptest.NewRequest(http.MethodGet, "/v1/agents/"+agentID+"/storage", nil)
	ctx := auth.ContextWithClaims(req.Context(), claims)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.GetAgentStorage(rr, req, agentID)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		t.Fatal("response missing 'data' field")
	}

	if data["used"].(float64) != 52428800 {
		t.Errorf("expected used=52428800, got %v", data["used"])
	}
	if data["quota"].(float64) != 1073741824 {
		t.Errorf("expected quota=1073741824, got %v", data["quota"])
	}
	if data["percentage"].(float64) < 4.8 || data["percentage"].(float64) > 4.9 {
		t.Errorf("expected percentage ~4.88, got %v", data["percentage"])
	}
}

func TestGetAgentStorage_NotOwner_Returns403(t *testing.T) {
	ownerID := "user_owner_123"
	intruderID := "user_intruder_456"
	agentID := "agent_test_1"

	agentFinderRepo := &MockAgentFinderRepo{
		agents: map[string]*models.Agent{
			agentID: {
				ID:      agentID,
				HumanID: &ownerID,
			},
		},
	}

	handler := NewStorageHandler(&MockStorageRepo{})
	handler.SetAgentFinderRepo(agentFinderRepo)

	claims := &auth.Claims{UserID: intruderID}
	req := httptest.NewRequest(http.MethodGet, "/v1/agents/"+agentID+"/storage", nil)
	ctx := auth.ContextWithClaims(req.Context(), claims)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.GetAgentStorage(rr, req, agentID)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestGetAgentStorage_NoAuth_Returns401(t *testing.T) {
	handler := NewStorageHandler(&MockStorageRepo{})

	req := httptest.NewRequest(http.MethodGet, "/v1/agents/some_agent/storage", nil)

	rr := httptest.NewRecorder()
	handler.GetAgentStorage(rr, req, "some_agent")

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestGetAgentStorage_AgentNotFound_Returns404(t *testing.T) {
	agentFinderRepo := &MockAgentFinderRepo{
		agents: map[string]*models.Agent{},
	}

	handler := NewStorageHandler(&MockStorageRepo{})
	handler.SetAgentFinderRepo(agentFinderRepo)

	claims := &auth.Claims{UserID: "user_123"}
	req := httptest.NewRequest(http.MethodGet, "/v1/agents/nonexistent/storage", nil)
	ctx := auth.ContextWithClaims(req.Context(), claims)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.GetAgentStorage(rr, req, "nonexistent")

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestGetAgentStorage_AgentSelfAccess_Returns200(t *testing.T) {
	agentID := "agent_self_1"

	agentFinderRepo := &MockAgentFinderRepo{
		agents: map[string]*models.Agent{
			agentID: {ID: agentID},
		},
	}

	storageRepo := &MockStorageRepo{
		used:  1000,
		quota: 1073741824,
	}

	handler := NewStorageHandler(storageRepo)
	handler.SetAgentFinderRepo(agentFinderRepo)

	// Agent API key auth
	agent := &models.Agent{ID: agentID}
	req := httptest.NewRequest(http.MethodGet, "/v1/agents/"+agentID+"/storage", nil)
	ctx := auth.ContextWithAgent(req.Context(), agent)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.GetAgentStorage(rr, req, agentID)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}
