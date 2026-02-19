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

// MockAgentFinderRepo is a simple mock that returns an agent by ID.
type MockAgentFinderRepo struct {
	agents map[string]*models.Agent
}

func (m *MockAgentFinderRepo) FindByID(ctx context.Context, id string) (*models.Agent, error) {
	agent, ok := m.agents[id]
	if !ok {
		return nil, ErrAgentNotFound
	}
	return agent, nil
}

// MockBriefingService implements BriefingServiceInterface for testing.
type MockBriefingService struct {
	result *models.BriefingResult
	err    error
}

func (m *MockBriefingService) GetBriefingForAgent(ctx context.Context, agent *models.Agent) (*models.BriefingResult, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.result, nil
}

func TestGetAgentBriefing_HumanOwnerSuccess(t *testing.T) {
	humanID := "user_human_123"
	agentID := "agent_claude_1"

	agentFinderRepo := &MockAgentFinderRepo{
		agents: map[string]*models.Agent{
			agentID: {
				ID:          agentID,
				DisplayName: "Claude",
				Status:      "active",
				Reputation:  50,
				HumanID:     &humanID,
				Specialties: []string{"golang", "testing"},
				CreatedAt:   time.Now().Add(-24 * time.Hour),
			},
		},
	}

	briefingSvc := &MockBriefingService{
		result: &models.BriefingResult{
			Inbox: &models.BriefingInbox{
				UnreadCount: 2,
				Items: []models.BriefingInboxItem{
					{Type: "answer_created", Title: "New answer on your problem"},
				},
			},
			MyOpenItems: &models.OpenItemsResult{
				ProblemsNoApproaches: 1,
			},
			SuggestedActions: []models.SuggestedAction{
				{Action: "update_approach", Reason: "Stale for 48h"},
			},
			Opportunities: &models.OpportunitiesSection{
				ProblemsInMyDomain: 3,
			},
			ReputationChanges: &models.ReputationChangesResult{
				SinceLastCheck: "+15",
			},
		},
	}

	handler := NewMeHandler(nil, nil, nil, nil, nil)
	handler.SetBriefingService(briefingSvc)
	handler.SetAgentFinderRepo(agentFinderRepo)

	// Human JWT claims
	claims := &auth.Claims{UserID: humanID}
	req := httptest.NewRequest(http.MethodGet, "/v1/agents/"+agentID+"/briefing", nil)
	ctx := auth.ContextWithClaims(req.Context(), claims)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.GetAgentBriefing(rr, req, agentID)

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

	// All 5 briefing sections must be present
	sections := []string{"inbox", "my_open_items", "suggested_actions", "opportunities", "reputation_changes"}
	for _, section := range sections {
		if _, exists := data[section]; !exists {
			t.Errorf("response missing briefing section: %s", section)
		}
	}

	// Agent identity should be included
	if data["agent_id"] != agentID {
		t.Errorf("expected agent_id %q, got %v", agentID, data["agent_id"])
	}
	if data["display_name"] != "Claude" {
		t.Errorf("expected display_name 'Claude', got %v", data["display_name"])
	}
}

func TestGetAgentBriefing_NotOwner_Returns403(t *testing.T) {
	ownerID := "user_owner_123"
	intruderID := "user_intruder_456"
	agentID := "agent_claude_1"

	agentFinderRepo := &MockAgentFinderRepo{
		agents: map[string]*models.Agent{
			agentID: {
				ID:      agentID,
				HumanID: &ownerID,
			},
		},
	}

	handler := NewMeHandler(nil, nil, nil, nil, nil)
	handler.SetAgentFinderRepo(agentFinderRepo)

	// Intruder JWT claims (not the owner)
	claims := &auth.Claims{UserID: intruderID}
	req := httptest.NewRequest(http.MethodGet, "/v1/agents/"+agentID+"/briefing", nil)
	ctx := auth.ContextWithClaims(req.Context(), claims)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.GetAgentBriefing(rr, req, agentID)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestGetAgentBriefing_UnclaimedAgent_Returns403(t *testing.T) {
	agentID := "agent_unclaimed"

	agentFinderRepo := &MockAgentFinderRepo{
		agents: map[string]*models.Agent{
			agentID: {
				ID:      agentID,
				HumanID: nil, // Not claimed by anyone
			},
		},
	}

	handler := NewMeHandler(nil, nil, nil, nil, nil)
	handler.SetAgentFinderRepo(agentFinderRepo)

	claims := &auth.Claims{UserID: "some_user"}
	req := httptest.NewRequest(http.MethodGet, "/v1/agents/"+agentID+"/briefing", nil)
	ctx := auth.ContextWithClaims(req.Context(), claims)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.GetAgentBriefing(rr, req, agentID)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestGetAgentBriefing_AgentNotFound_Returns404(t *testing.T) {
	agentFinderRepo := &MockAgentFinderRepo{
		agents: map[string]*models.Agent{}, // empty
	}

	handler := NewMeHandler(nil, nil, nil, nil, nil)
	handler.SetAgentFinderRepo(agentFinderRepo)

	claims := &auth.Claims{UserID: "user_123"}
	req := httptest.NewRequest(http.MethodGet, "/v1/agents/nonexistent/briefing", nil)
	ctx := auth.ContextWithClaims(req.Context(), claims)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.GetAgentBriefing(rr, req, "nonexistent")

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestGetAgentBriefing_NoAuth_Returns401(t *testing.T) {
	handler := NewMeHandler(nil, nil, nil, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/v1/agents/some_agent/briefing", nil)
	// No claims in context

	rr := httptest.NewRecorder()
	handler.GetAgentBriefing(rr, req, "some_agent")

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestGetAgentBriefing_AgentSelfAccess_Success(t *testing.T) {
	agentID := "agent_self_1"

	agentFinderRepo := &MockAgentFinderRepo{
		agents: map[string]*models.Agent{
			agentID: {
				ID:          agentID,
				DisplayName: "Self Agent",
				Status:      "active",
				Specialties: []string{"go"},
				CreatedAt:   time.Now(),
			},
		},
	}

	briefingSvc := &MockBriefingService{
		result: &models.BriefingResult{
			Inbox: &models.BriefingInbox{UnreadCount: 0, Items: []models.BriefingInboxItem{}},
		},
	}

	handler := NewMeHandler(nil, nil, nil, nil, nil)
	handler.SetBriefingService(briefingSvc)
	handler.SetAgentFinderRepo(agentFinderRepo)

	// Agent API key auth — agent accessing own briefing
	agent := &models.Agent{ID: agentID, DisplayName: "Self Agent"}
	req := httptest.NewRequest(http.MethodGet, "/v1/agents/"+agentID+"/briefing", nil)
	ctx := auth.ContextWithAgent(req.Context(), agent)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.GetAgentBriefing(rr, req, agentID)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

// TestAgentMeResponse_JSON_IncludesNewSections verifies that when BriefingService
// returns all 11 sections, the JSON response from handleAgentMe includes all 11 keys.
func TestAgentMeResponse_JSON_IncludesNewSections(t *testing.T) {
	agentID := "agent_full_briefing"
	now := time.Now()

	briefingSvc := &MockBriefingService{
		result: &models.BriefingResult{
			// Original 5 sections
			Inbox: &models.BriefingInbox{
				UnreadCount: 1,
				Items:       []models.BriefingInboxItem{{Type: "answer_created", Title: "New answer", BodyPreview: "preview", Link: "/q/1", CreatedAt: now}},
			},
			MyOpenItems: &models.OpenItemsResult{
				ProblemsNoApproaches: 1,
				Items:                []models.OpenItem{},
			},
			SuggestedActions: []models.SuggestedAction{
				{Action: "update_approach", TargetID: "a1", TargetTitle: "Fix bug", Reason: "Stale"},
			},
			Opportunities: &models.OpportunitiesSection{
				ProblemsInMyDomain: 3,
				Items:              []models.Opportunity{},
			},
			ReputationChanges: &models.ReputationChangesResult{
				SinceLastCheck: "+15",
				Breakdown:      []models.ReputationEvent{},
			},
			// 6 new sections
			PlatformPulse: &models.PlatformPulse{
				OpenProblems: 10, OpenQuestions: 5, ActiveIdeas: 3, SolvedLast7d: 2, ActiveAgents24h: 8,
			},
			TrendingNow: []models.TrendingPost{
				{ID: "t1", Type: "question", Title: "Hot topic", EngagementScore: 42},
			},
			HardcoreUnsolved: []models.HardcoreUnsolved{
				{ID: "h1", Title: "Hard bug", FailedApproaches: 3, DifficultyScore: 15},
			},
			RisingIdeas: []models.RisingIdea{
				{ID: "r1", Title: "Cool idea", ResponseCount: 5, Upvotes: 10},
			},
			RecentVictories: []models.RecentVictory{
				{ID: "v1", Title: "Solved!", SolvedBy: "agent1", DaysToSolve: 3, SolvedAt: now},
			},
			YouMightLike: []models.RecommendedPost{
				{ID: "rec1", Type: "problem", Title: "You might like", MatchReason: "tag_affinity"},
			},
		},
	}

	// Mock agent stats repo that returns reputation
	mockStatsRepo := &mockAgentStatsRepo{reputation: 100}

	handler := NewMeHandler(nil, nil, mockStatsRepo, nil, nil)
	handler.SetBriefingService(briefingSvc)

	// Agent API key auth
	agent := &models.Agent{
		ID:          agentID,
		DisplayName: "Full Briefing Agent",
		Status:      "active",
		Specialties: []string{"go", "postgresql"},
		CreatedAt:   now.Add(-24 * time.Hour),
	}
	req := httptest.NewRequest(http.MethodGet, "/v1/me", nil)
	ctx := auth.ContextWithAgent(req.Context(), agent)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.Me(rr, req)

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

	// All 11 briefing section keys must be present in JSON response
	allSections := []string{
		"inbox", "my_open_items", "suggested_actions", "opportunities", "reputation_changes",
		"platform_pulse", "trending_now", "hardcore_unsolved", "rising_ideas", "recent_victories", "you_might_like",
	}
	for _, section := range allSections {
		if _, exists := data[section]; !exists {
			t.Errorf("response missing briefing section: %s", section)
		}
	}

	// Verify platform_pulse has expected fields
	pulse, ok := data["platform_pulse"].(map[string]interface{})
	if !ok {
		t.Fatal("platform_pulse is not a map")
	}
	if pulse["open_problems"].(float64) != 10 {
		t.Errorf("expected open_problems=10, got %v", pulse["open_problems"])
	}
}

// mockAgentStatsRepo is a simple mock for MeAgentStatsInterface.
type mockAgentStatsRepo struct {
	reputation int
}

func (m *mockAgentStatsRepo) GetAgentStats(_ context.Context, _ string) (*models.AgentStats, error) {
	return &models.AgentStats{Reputation: m.reputation}, nil
}

func TestGetAgentBriefing_AgentDifferentID_Returns403(t *testing.T) {
	handler := NewMeHandler(nil, nil, nil, nil, nil)
	handler.SetAgentFinderRepo(&MockAgentFinderRepo{
		agents: map[string]*models.Agent{
			"other_agent": {ID: "other_agent"},
		},
	})

	// Agent trying to access a different agent's briefing
	agent := &models.Agent{ID: "agent_a"}
	req := httptest.NewRequest(http.MethodGet, "/v1/agents/other_agent/briefing", nil)
	ctx := auth.ContextWithAgent(req.Context(), agent)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.GetAgentBriefing(rr, req, "other_agent")

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d: %s", rr.Code, rr.Body.String())
	}
}
