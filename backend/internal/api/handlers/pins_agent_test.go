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

// MockPinRepo implements PinRepositoryInterface for agent pin tests.
type MockPinRepo struct {
	pins  []models.Pin
	total int
	err   error
}

func (m *MockPinRepo) Create(ctx context.Context, pin *models.Pin) error   { return nil }
func (m *MockPinRepo) GetByID(ctx context.Context, id string) (*models.Pin, error) {
	return nil, nil
}
func (m *MockPinRepo) GetByCID(ctx context.Context, cid, ownerID string) (*models.Pin, error) {
	return nil, nil
}
func (m *MockPinRepo) ListByOwner(ctx context.Context, ownerID, ownerType string, opts models.PinListOptions) ([]models.Pin, int, error) {
	if m.err != nil {
		return nil, 0, m.err
	}
	return m.pins, m.total, nil
}
func (m *MockPinRepo) UpdateStatus(ctx context.Context, id string, status models.PinStatus) error {
	return nil
}
func (m *MockPinRepo) UpdateStatusAndSize(ctx context.Context, id string, status models.PinStatus, sizeBytes int64) error {
	return nil
}
func (m *MockPinRepo) Delete(ctx context.Context, id string) error { return nil }

func TestListAgentPins_HumanOwnerSuccess(t *testing.T) {
	humanID := "user_human_123"
	agentID := "agent_test_1"
	sizeBytes := int64(5000)

	agentFinderRepo := &MockAgentFinderRepo{
		agents: map[string]*models.Agent{
			agentID: {
				ID:      agentID,
				HumanID: &humanID,
			},
		},
	}

	pinRepo := &MockPinRepo{
		pins: []models.Pin{
			{
				ID:        "pin-1",
				CID:       "QmTestCID123456789012345678901234567890123",
				Status:    models.PinStatusPinned,
				Name:      "test-pin",
				OwnerID:   agentID,
				OwnerType: "agent",
				SizeBytes: &sizeBytes,
				CreatedAt: time.Now(),
			},
		},
		total: 1,
	}

	handler := NewPinsHandler(pinRepo, nil)
	handler.SetAgentFinderRepo(agentFinderRepo)

	// Human JWT claims
	claims := &auth.Claims{UserID: humanID}
	req := httptest.NewRequest(http.MethodGet, "/v1/agents/"+agentID+"/pins", nil)
	ctx := auth.ContextWithClaims(req.Context(), claims)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.ListAgentPins(rr, req, agentID)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	count, ok := resp["count"].(float64)
	if !ok || count != 1 {
		t.Errorf("expected count=1, got %v", resp["count"])
	}

	results, ok := resp["results"].([]interface{})
	if !ok || len(results) != 1 {
		t.Errorf("expected 1 result, got %v", resp["results"])
	}
}

func TestListAgentPins_NotOwner_Returns403(t *testing.T) {
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

	handler := NewPinsHandler(&MockPinRepo{}, nil)
	handler.SetAgentFinderRepo(agentFinderRepo)

	claims := &auth.Claims{UserID: intruderID}
	req := httptest.NewRequest(http.MethodGet, "/v1/agents/"+agentID+"/pins", nil)
	ctx := auth.ContextWithClaims(req.Context(), claims)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.ListAgentPins(rr, req, agentID)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestListAgentPins_UnclaimedAgent_Returns403(t *testing.T) {
	agentID := "agent_unclaimed"

	agentFinderRepo := &MockAgentFinderRepo{
		agents: map[string]*models.Agent{
			agentID: {
				ID:      agentID,
				HumanID: nil,
			},
		},
	}

	handler := NewPinsHandler(&MockPinRepo{}, nil)
	handler.SetAgentFinderRepo(agentFinderRepo)

	claims := &auth.Claims{UserID: "some_user"}
	req := httptest.NewRequest(http.MethodGet, "/v1/agents/"+agentID+"/pins", nil)
	ctx := auth.ContextWithClaims(req.Context(), claims)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.ListAgentPins(rr, req, agentID)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestListAgentPins_AgentNotFound_Returns404(t *testing.T) {
	agentFinderRepo := &MockAgentFinderRepo{
		agents: map[string]*models.Agent{},
	}

	handler := NewPinsHandler(&MockPinRepo{}, nil)
	handler.SetAgentFinderRepo(agentFinderRepo)

	claims := &auth.Claims{UserID: "user_123"}
	req := httptest.NewRequest(http.MethodGet, "/v1/agents/nonexistent/pins", nil)
	ctx := auth.ContextWithClaims(req.Context(), claims)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.ListAgentPins(rr, req, "nonexistent")

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestListAgentPins_NoAuth_Returns401(t *testing.T) {
	handler := NewPinsHandler(&MockPinRepo{}, nil)

	req := httptest.NewRequest(http.MethodGet, "/v1/agents/some_agent/pins", nil)

	rr := httptest.NewRecorder()
	handler.ListAgentPins(rr, req, "some_agent")

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestListAgentPins_AgentSelfAccess_Returns200(t *testing.T) {
	agentID := "agent_self_1"

	agentFinderRepo := &MockAgentFinderRepo{
		agents: map[string]*models.Agent{
			agentID: {ID: agentID},
		},
	}

	pinRepo := &MockPinRepo{
		pins:  []models.Pin{},
		total: 0,
	}

	handler := NewPinsHandler(pinRepo, nil)
	handler.SetAgentFinderRepo(agentFinderRepo)

	// Agent API key auth
	agent := &models.Agent{ID: agentID}
	req := httptest.NewRequest(http.MethodGet, "/v1/agents/"+agentID+"/pins", nil)
	ctx := auth.ContextWithAgent(req.Context(), agent)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.ListAgentPins(rr, req, agentID)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestListAgentPins_AgentDifferentID_Returns403(t *testing.T) {
	handler := NewPinsHandler(&MockPinRepo{}, nil)
	handler.SetAgentFinderRepo(&MockAgentFinderRepo{
		agents: map[string]*models.Agent{
			"other_agent": {ID: "other_agent"},
		},
	})

	// Agent trying to access a different agent's pins
	agent := &models.Agent{ID: "agent_a"}
	req := httptest.NewRequest(http.MethodGet, "/v1/agents/other_agent/pins", nil)
	ctx := auth.ContextWithAgent(req.Context(), agent)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.ListAgentPins(rr, req, "other_agent")

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d: %s", rr.Code, rr.Body.String())
	}
}
