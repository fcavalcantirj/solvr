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

// MockBriefingOpportunitiesRepo implements BriefingOpportunitiesRepo for testing.
type MockBriefingOpportunitiesRepo struct {
	result *models.OpportunitiesSection
	err    error
}

func (m *MockBriefingOpportunitiesRepo) GetOpportunitiesForAgent(ctx context.Context, agentID string, specialties []string, limit int) (*models.OpportunitiesSection, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.result, nil
}

// TestAgentMe_OpportunitiesMatchSpecialties verifies that agent with specialties ['golang', 'database']
// gets opportunities for problems tagged with matching tags.
func TestAgentMe_OpportunitiesMatchSpecialties(t *testing.T) {
	repo := NewMockMeUserRepository()
	config := &OAuthConfig{JWTSecret: "test-secret-key"}

	opportunitiesRepo := &MockBriefingOpportunitiesRepo{
		result: &models.OpportunitiesSection{
			ProblemsInMyDomain: 1,
			Items: []models.Opportunity{
				{
					ID:              "prob-golang-1",
					Title:           "Race condition in Go channels",
					Tags:            []string{"golang", "concurrency"},
					ApproachesCount: 0,
					PostedBy:        "another_agent",
					AgeHours:        12,
				},
			},
		},
	}

	handler := NewMeHandler(config, repo, nil, nil, nil)
	handler.SetOpportunitiesRepo(opportunitiesRepo)

	req := httptest.NewRequest(http.MethodGet, "/v1/me", nil)
	agent := &models.Agent{
		ID:          "golang_agent",
		DisplayName: "Golang Agent",
		Status:      "active",
		Specialties: []string{"golang", "database"},
	}
	ctx := auth.ContextWithAgent(req.Context(), agent)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.Me(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var response map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&response)
	data := response["data"].(map[string]interface{})

	opportunities, ok := data["opportunities"].(map[string]interface{})
	if !ok {
		t.Fatal("response missing 'opportunities' field or it's not an object")
	}

	problemsCount := int(opportunities["problems_in_my_domain"].(float64))
	if problemsCount != 1 {
		t.Errorf("expected problems_in_my_domain=1, got %d", problemsCount)
	}

	items, ok := opportunities["items"].([]interface{})
	if !ok {
		t.Fatal("opportunities.items is not an array")
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 opportunity, got %d", len(items))
	}

	item0 := items[0].(map[string]interface{})
	if item0["id"] != "prob-golang-1" {
		t.Errorf("expected id 'prob-golang-1', got %q", item0["id"])
	}
	if item0["title"] != "Race condition in Go channels" {
		t.Errorf("expected title 'Race condition in Go channels', got %q", item0["title"])
	}
	if item0["posted_by"] != "another_agent" {
		t.Errorf("expected posted_by 'another_agent', got %q", item0["posted_by"])
	}
	if int(item0["approaches_count"].(float64)) != 0 {
		t.Errorf("expected approaches_count=0, got %v", item0["approaches_count"])
	}
}

// TestAgentMe_OpportunitiesExcludesOwnPosts verifies agent's own problems do NOT appear.
func TestAgentMe_OpportunitiesExcludesOwnPosts(t *testing.T) {
	repo := NewMockMeUserRepository()
	config := &OAuthConfig{JWTSecret: "test-secret-key"}

	// Repo returns only problems from other users (own posts excluded at query level)
	opportunitiesRepo := &MockBriefingOpportunitiesRepo{
		result: &models.OpportunitiesSection{
			ProblemsInMyDomain: 1,
			Items: []models.Opportunity{
				{
					ID:              "prob-other-1",
					Title:           "Someone else's problem",
					Tags:            []string{"golang"},
					ApproachesCount: 1,
					PostedBy:        "different_agent",
					AgeHours:        24,
				},
			},
		},
	}

	handler := NewMeHandler(config, repo, nil, nil, nil)
	handler.SetOpportunitiesRepo(opportunitiesRepo)

	req := httptest.NewRequest(http.MethodGet, "/v1/me", nil)
	agent := &models.Agent{
		ID:          "own_posts_agent",
		DisplayName: "Own Posts Agent",
		Status:      "active",
		Specialties: []string{"golang"},
	}
	ctx := auth.ContextWithAgent(req.Context(), agent)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.Me(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var response map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&response)
	data := response["data"].(map[string]interface{})
	opportunities := data["opportunities"].(map[string]interface{})
	items := opportunities["items"].([]interface{})

	// Verify none of the items are from the requesting agent
	for _, item := range items {
		itemMap := item.(map[string]interface{})
		if itemMap["posted_by"] == "own_posts_agent" {
			t.Errorf("expected own posts to be excluded, found posted_by='own_posts_agent'")
		}
	}
}

// TestAgentMe_OpportunitiesExcludesSolved verifies solved/closed problems do NOT appear.
func TestAgentMe_OpportunitiesExcludesSolved(t *testing.T) {
	repo := NewMockMeUserRepository()
	config := &OAuthConfig{JWTSecret: "test-secret-key"}

	// Repo returns empty because all matching problems are solved/closed (excluded at query level)
	opportunitiesRepo := &MockBriefingOpportunitiesRepo{
		result: &models.OpportunitiesSection{
			ProblemsInMyDomain: 0,
			Items:              []models.Opportunity{},
		},
	}

	handler := NewMeHandler(config, repo, nil, nil, nil)
	handler.SetOpportunitiesRepo(opportunitiesRepo)

	req := httptest.NewRequest(http.MethodGet, "/v1/me", nil)
	agent := &models.Agent{
		ID:          "solved_agent",
		DisplayName: "Solved Agent",
		Status:      "active",
		Specialties: []string{"golang"},
	}
	ctx := auth.ContextWithAgent(req.Context(), agent)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.Me(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var response map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&response)
	data := response["data"].(map[string]interface{})
	opportunities := data["opportunities"].(map[string]interface{})
	items := opportunities["items"].([]interface{})

	if len(items) != 0 {
		t.Errorf("expected 0 opportunities (solved excluded), got %d", len(items))
	}
}

// TestAgentMe_OpportunitiesPrioritizesZeroApproaches verifies problems with 0 approaches
// rank higher than those with approaches.
func TestAgentMe_OpportunitiesPrioritizesZeroApproaches(t *testing.T) {
	repo := NewMockMeUserRepository()
	config := &OAuthConfig{JWTSecret: "test-secret-key"}

	opportunitiesRepo := &MockBriefingOpportunitiesRepo{
		result: &models.OpportunitiesSection{
			ProblemsInMyDomain: 3,
			Items: []models.Opportunity{
				{ID: "prob-zero-1", Title: "Zero approaches", ApproachesCount: 0, PostedBy: "other1", AgeHours: 6},
				{ID: "prob-zero-2", Title: "Also zero", ApproachesCount: 0, PostedBy: "other2", AgeHours: 12},
				{ID: "prob-has-1", Title: "Has approaches", ApproachesCount: 2, PostedBy: "other3", AgeHours: 3},
			},
		},
	}

	handler := NewMeHandler(config, repo, nil, nil, nil)
	handler.SetOpportunitiesRepo(opportunitiesRepo)

	req := httptest.NewRequest(http.MethodGet, "/v1/me", nil)
	agent := &models.Agent{
		ID:          "priority_agent",
		DisplayName: "Priority Agent",
		Status:      "active",
		Specialties: []string{"golang"},
	}
	ctx := auth.ContextWithAgent(req.Context(), agent)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.Me(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var response map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&response)
	data := response["data"].(map[string]interface{})
	opportunities := data["opportunities"].(map[string]interface{})
	items := opportunities["items"].([]interface{})

	if len(items) < 2 {
		t.Fatalf("expected at least 2 items, got %d", len(items))
	}

	// First items should have 0 approaches (higher priority)
	item0 := items[0].(map[string]interface{})
	item1 := items[1].(map[string]interface{})

	count0 := int(item0["approaches_count"].(float64))
	count1 := int(item1["approaches_count"].(float64))

	if count0 != 0 {
		t.Errorf("expected first item to have 0 approaches, got %d", count0)
	}
	if count1 != 0 {
		t.Errorf("expected second item to have 0 approaches, got %d", count1)
	}
}

// TestAgentMe_OpportunitiesLimit verifies max 5 opportunities are returned.
func TestAgentMe_OpportunitiesLimit(t *testing.T) {
	repo := NewMockMeUserRepository()
	config := &OAuthConfig{JWTSecret: "test-secret-key"}

	// Create 5 items (limit applied at query level, handler trusts repo)
	var items []models.Opportunity
	for i := 0; i < 5; i++ {
		items = append(items, models.Opportunity{
			ID:              "prob-" + string(rune('a'+i)),
			Title:           "Problem " + string(rune('A'+i)),
			Tags:            []string{"golang"},
			ApproachesCount: 0,
			PostedBy:        "other_agent",
			AgeHours:        i * 6,
		})
	}

	opportunitiesRepo := &MockBriefingOpportunitiesRepo{
		result: &models.OpportunitiesSection{
			ProblemsInMyDomain: 10, // Total count is higher than items returned
			Items:              items,
		},
	}

	handler := NewMeHandler(config, repo, nil, nil, nil)
	handler.SetOpportunitiesRepo(opportunitiesRepo)

	req := httptest.NewRequest(http.MethodGet, "/v1/me", nil)
	agent := &models.Agent{
		ID:          "limit_opp_agent",
		DisplayName: "Limit Opp Agent",
		Status:      "active",
		Specialties: []string{"golang"},
	}
	ctx := auth.ContextWithAgent(req.Context(), agent)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.Me(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var response map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&response)
	data := response["data"].(map[string]interface{})
	opportunities := data["opportunities"].(map[string]interface{})
	respItems := opportunities["items"].([]interface{})

	if len(respItems) > 5 {
		t.Errorf("expected max 5 opportunities, got %d", len(respItems))
	}

	// Total count should reflect all matching, not just returned
	total := int(opportunities["problems_in_my_domain"].(float64))
	if total != 10 {
		t.Errorf("expected problems_in_my_domain=10, got %d", total)
	}
}

// TestAgentMe_OpportunitiesNoSpecialties verifies agent with empty specialties
// returns nil opportunities (no spam — don't show everything).
func TestAgentMe_OpportunitiesNoSpecialties(t *testing.T) {
	repo := NewMockMeUserRepository()
	config := &OAuthConfig{JWTSecret: "test-secret-key"}

	// Repo should NOT be called when specialties are empty
	opportunitiesRepo := &MockBriefingOpportunitiesRepo{
		result: &models.OpportunitiesSection{
			ProblemsInMyDomain: 99,
			Items:              []models.Opportunity{{ID: "should-not-appear"}},
		},
	}

	handler := NewMeHandler(config, repo, nil, nil, nil)
	handler.SetOpportunitiesRepo(opportunitiesRepo)

	req := httptest.NewRequest(http.MethodGet, "/v1/me", nil)
	agent := &models.Agent{
		ID:          "no_spec_agent",
		DisplayName: "No Specialties Agent",
		Status:      "active",
		Specialties: []string{}, // Empty specialties
	}
	ctx := auth.ContextWithAgent(req.Context(), agent)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.Me(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var response map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&response)
	data := response["data"].(map[string]interface{})

	// Opportunities should be nil/null when agent has no specialties
	if data["opportunities"] != nil {
		t.Errorf("expected opportunities to be nil for agent with no specialties, got %v", data["opportunities"])
	}
}
