package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// GrantReputationOnce grants points under grantKey once per agent (agent_reputation_grants).
func (m *MockAgentRepository) GrantReputationOnce(ctx context.Context, agentID, grantKey string, points int) (bool, error) {
	agent, exists := m.agents[agentID]
	if !exists {
		return false, ErrAgentNotFound
	}
	if m.grants == nil {
		m.grants = make(map[string]bool)
	}
	if m.grants[agentID+"/"+grantKey] {
		return false, nil
	}
	m.grants[agentID+"/"+grantKey] = true
	agent.Reputation += points
	return true, nil
}

// The model bonus is an activation granted once per agent (idx 77, migration 000121). Before
// it, PATCH /v1/agents/{id} added +10 every time the model went from empty to set, so clearing
// and re-setting the model farmed the bonus (measured through the API: 60 after five cycles).
func TestUpdateAgent_ModelBonusIsGrantedOncePerAgentAcrossClearAndSetCycles(t *testing.T) {
	repo := NewMockAgentRepository()
	handler := NewAgentsHandler(repo, "test-jwt-secret")
	humanID := "user-123"
	repo.agents["cycling_agent"] = &models.Agent{
		ID: "cycling_agent", DisplayName: "Cycling Agent", HumanID: &humanID,
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	patchModel := func(model string) {
		t.Helper()
		body, _ := json.Marshal(UpdateAgentRequest{Model: strPtr(model)})
		req := httptest.NewRequest(http.MethodPatch, "/v1/agents/cycling_agent", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req = addJWTClaimsToContext(req, "user-123", "user@example.com", "user")
		rr := httptest.NewRecorder()
		handler.UpdateAgent(rr, req, "cycling_agent")
		if rr.Code != http.StatusOK {
			t.Fatalf("PATCH model=%q: expected 200, got %d: %s", model, rr.Code, rr.Body.String())
		}
	}

	patchModel("model-a")
	if got := repo.agents["cycling_agent"].Reputation; got != ReputationBonusOnModel {
		t.Fatalf("expected the model bonus %d after the first model, got %d", ReputationBonusOnModel, got)
	}
	for range 5 {
		patchModel("")
		patchModel("model-b")
	}
	if got := repo.agents["cycling_agent"].Reputation; got != ReputationBonusOnModel {
		t.Errorf("expected reputation %d after five clear-and-set cycles, got %d", ReputationBonusOnModel, got)
	}
}
