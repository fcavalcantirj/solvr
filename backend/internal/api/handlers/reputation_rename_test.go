package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/auth"
	"github.com/fcavalcantirj/solvr/internal/models"
)

// TestAgentResponse_ReturnsReputation_NotKarma verifies that the agent detail
// API response uses "reputation" field instead of "karma".
// This is a TDD RED test for the karma → reputation rename.
func TestAgentResponse_ReturnsReputation_NotKarma(t *testing.T) {
	repo := NewMockAgentRepository()
	handler := NewAgentsHandler(repo, "test-jwt-secret")

	// Seed an agent with reputation bonus
	repo.agents["test_rep_agent"] = &models.Agent{
		ID:          "test_rep_agent",
		DisplayName: "Rep Test Agent",
		Status:      "active",
		Reputation:  60, // renamed from Karma
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/agents/test_rep_agent", nil)
	rr := httptest.NewRecorder()
	handler.GetAgent(rr, req, "test_rep_agent")

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// Parse response as raw JSON to check field names
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(rr.Body.Bytes(), &raw); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	var data map[string]json.RawMessage
	if err := json.Unmarshal(raw["data"], &data); err != nil {
		t.Fatalf("failed to parse data: %v", err)
	}

	var agent map[string]json.RawMessage
	if err := json.Unmarshal(data["agent"], &agent); err != nil {
		t.Fatalf("failed to parse agent: %v", err)
	}

	// MUST have "reputation" field
	if _, ok := agent["reputation"]; !ok {
		t.Error("agent response missing 'reputation' field — should exist")
	}

	// MUST NOT have "karma" field
	if _, ok := agent["karma"]; ok {
		t.Error("agent response still has 'karma' field — should be renamed to 'reputation'")
	}

	// Verify the value
	var repValue int
	if err := json.Unmarshal(agent["reputation"], &repValue); err != nil {
		t.Fatalf("failed to parse reputation value: %v", err)
	}
	if repValue != 60 {
		t.Errorf("expected reputation=60, got %d", repValue)
	}
}

// TestAgentList_ReturnsReputation_NotKarma verifies agent list uses "reputation"
// not "karma" in the JSON response.
func TestAgentList_ReturnsReputation_NotKarma(t *testing.T) {
	repo := NewMockAgentRepository()
	handler := NewAgentsHandler(repo, "test-jwt-secret")

	// Seed agents
	repo.agents["rep_list_agent"] = &models.Agent{
		ID:          "rep_list_agent",
		DisplayName: "List Rep Agent",
		Status:      "active",
		Reputation:  50,
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/agents?sort=reputation", nil)
	rr := httptest.NewRecorder()
	handler.ListAgents(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// Check raw JSON for field names
	body := rr.Body.String()

	// Should contain "reputation" field
	if !json.Valid([]byte(body)) {
		t.Fatal("response is not valid JSON")
	}

	var resp map[string]json.RawMessage
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	var dataList []map[string]json.RawMessage
	if err := json.Unmarshal(resp["data"], &dataList); err != nil {
		t.Fatalf("failed to parse data array: %v", err)
	}

	if len(dataList) == 0 {
		t.Fatal("expected at least one agent in list")
	}

	firstAgent := dataList[0]

	// MUST have "reputation" field
	if _, ok := firstAgent["reputation"]; !ok {
		t.Error("agent list item missing 'reputation' field — should exist")
	}

	// MUST NOT have "karma" field
	if _, ok := firstAgent["karma"]; ok {
		t.Error("agent list item still has 'karma' field — should be renamed to 'reputation'")
	}
}

// TestMeEndpoint_Agent_ReturnsReputation_NotKarma verifies that GET /v1/me
// with agent API key auth returns "reputation" not "karma".
func TestMeEndpoint_Agent_ReturnsReputation_NotKarma(t *testing.T) {
	config := &OAuthConfig{JWTSecret: "test-secret"}
	userRepo := NewMockMeUserRepository()
	handler := NewMeHandler(config, userRepo, nil, nil)

	// Create request with agent context
	agent := &models.Agent{
		ID:          "me_rep_agent",
		DisplayName: "Me Rep Agent",
		Status:      "active",
		Reputation:  60,
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/me", nil)
	ctx := auth.ContextWithAgent(req.Context(), agent)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.Me(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// Parse response as raw JSON
	var resp map[string]json.RawMessage
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	var data map[string]json.RawMessage
	if err := json.Unmarshal(resp["data"], &data); err != nil {
		t.Fatalf("failed to parse data: %v", err)
	}

	// MUST have "reputation" field
	if _, ok := data["reputation"]; !ok {
		t.Error("agent /me response missing 'reputation' field — should exist")
	}

	// MUST NOT have "karma" field
	if _, ok := data["karma"]; ok {
		t.Error("agent /me response still has 'karma' field — should be renamed to 'reputation'")
	}

	// Verify the value
	var repValue int
	if err := json.Unmarshal(data["reputation"], &repValue); err != nil {
		t.Fatalf("failed to parse reputation value: %v", err)
	}
	if repValue != 60 {
		t.Errorf("expected reputation=60, got %d", repValue)
	}
}

// TestAgentStats_Reputation_IncludesBonus verifies that GetAgentStats reputation
// includes the bonus points (formerly karma) in the total.
func TestAgentStats_Reputation_IncludesBonus(t *testing.T) {
	repo := NewMockAgentRepository()
	handler := NewAgentsHandler(repo, "test-jwt-secret")

	// Seed agent with reputation bonus of 60 (50 claim + 10 model)
	repo.agents["stats_bonus_agent"] = &models.Agent{
		ID:          "stats_bonus_agent",
		DisplayName: "Stats Bonus Agent",
		Status:      "active",
		Reputation:  60,
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/agents/stats_bonus_agent", nil)
	rr := httptest.NewRecorder()
	handler.GetAgent(rr, req, "stats_bonus_agent")

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp GetAgentResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Stats reputation should include the bonus points
	// With no activity, reputation should equal the bonus (60)
	if resp.Data.Stats.Reputation < 60 {
		t.Errorf("expected stats.reputation >= 60 (includes bonus), got %d", resp.Data.Stats.Reputation)
	}
}

// TestClaimResponse_UsesReputation_NotKarma verifies the claim flow
// uses "reputation" terminology, not "karma".
func TestClaimResponse_UsesReputation_NotKarma(t *testing.T) {
	// The constant should be ReputationBonusOnClaim, not KarmaBonusOnClaim
	if ReputationBonusOnClaim != 50 {
		t.Errorf("expected ReputationBonusOnClaim=50, got %d", ReputationBonusOnClaim)
	}
}
