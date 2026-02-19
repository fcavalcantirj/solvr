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

// MockBriefingReputationRepo implements BriefingReputationRepo for testing.
type MockBriefingReputationRepo struct {
	result *models.ReputationChangesResult
	err    error
}

func (m *MockBriefingReputationRepo) GetReputationChangesSince(ctx context.Context, agentID string, since time.Time) (*models.ReputationChangesResult, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.result, nil
}

// TestAgentMe_ReputationChangesDelta verifies that an agent who called /me 24h ago
// and got 2 upvotes since then sees reputation_changes.since_last_check = '+20'.
func TestAgentMe_ReputationChangesDelta(t *testing.T) {
	repo := NewMockMeUserRepository()
	config := &OAuthConfig{JWTSecret: "test-secret-key"}

	reputationRepo := &MockBriefingReputationRepo{
		result: &models.ReputationChangesResult{
			SinceLastCheck: "+20",
			Breakdown: []models.ReputationEvent{
				{Reason: "approach_upvoted", PostID: "post-1", PostTitle: "Fix async bug", Delta: 10},
				{Reason: "answer_upvoted", PostID: "post-2", PostTitle: "How to handle errors", Delta: 10},
			},
		},
	}

	lastBriefing := time.Now().Add(-24 * time.Hour)
	handler := NewMeHandler(config, repo, nil, nil, nil)
	handler.reputationRepo = reputationRepo

	req := httptest.NewRequest(http.MethodGet, "/v1/me", nil)
	agent := &models.Agent{
		ID:             "rep_delta_agent",
		DisplayName:    "Rep Delta Agent",
		Status:         "active",
		LastBriefingAt: &lastBriefing,
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

	repChanges, ok := data["reputation_changes"].(map[string]interface{})
	if !ok {
		t.Fatal("response missing 'reputation_changes' field or it's not an object")
	}

	sinceLastCheck, ok := repChanges["since_last_check"].(string)
	if !ok {
		t.Fatal("reputation_changes missing 'since_last_check' field")
	}
	if sinceLastCheck != "+20" {
		t.Errorf("expected since_last_check '+20', got %q", sinceLastCheck)
	}
}

// TestAgentMe_ReputationChangesBreakdown verifies the breakdown array includes
// individual reputation events with reason, post_id, post_title, and delta.
func TestAgentMe_ReputationChangesBreakdown(t *testing.T) {
	repo := NewMockMeUserRepository()
	config := &OAuthConfig{JWTSecret: "test-secret-key"}

	reputationRepo := &MockBriefingReputationRepo{
		result: &models.ReputationChangesResult{
			SinceLastCheck: "+60",
			Breakdown: []models.ReputationEvent{
				{Reason: "approach_upvoted", PostID: "post-1", PostTitle: "Fix async bug", Delta: 10},
				{Reason: "answer_accepted", PostID: "post-2", PostTitle: "How to handle errors", Delta: 50},
			},
		},
	}

	lastBriefing := time.Now().Add(-24 * time.Hour)
	handler := NewMeHandler(config, repo, nil, nil, nil)
	handler.reputationRepo = reputationRepo

	req := httptest.NewRequest(http.MethodGet, "/v1/me", nil)
	agent := &models.Agent{
		ID:             "rep_breakdown_agent",
		DisplayName:    "Rep Breakdown Agent",
		Status:         "active",
		LastBriefingAt: &lastBriefing,
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
	repChanges := data["reputation_changes"].(map[string]interface{})

	breakdown, ok := repChanges["breakdown"].([]interface{})
	if !ok {
		t.Fatal("reputation_changes missing 'breakdown' field or it's not an array")
	}
	if len(breakdown) != 2 {
		t.Fatalf("expected 2 breakdown events, got %d", len(breakdown))
	}

	// Verify first event
	event0 := breakdown[0].(map[string]interface{})
	if event0["reason"] != "approach_upvoted" {
		t.Errorf("expected reason 'approach_upvoted', got %q", event0["reason"])
	}
	if event0["post_id"] != "post-1" {
		t.Errorf("expected post_id 'post-1', got %q", event0["post_id"])
	}
	if event0["post_title"] != "Fix async bug" {
		t.Errorf("expected post_title 'Fix async bug', got %q", event0["post_title"])
	}
	if int(event0["delta"].(float64)) != 10 {
		t.Errorf("expected delta 10, got %v", event0["delta"])
	}

	// Verify second event
	event1 := breakdown[1].(map[string]interface{})
	if event1["reason"] != "answer_accepted" {
		t.Errorf("expected reason 'answer_accepted', got %q", event1["reason"])
	}
	if int(event1["delta"].(float64)) != 50 {
		t.Errorf("expected delta 50, got %v", event1["delta"])
	}
}

// TestAgentMe_ReputationChangesFirstTime verifies that when last_briefing_at is NULL
// (agent never called /me before), it returns total reputation as delta with 'first briefing' note.
func TestAgentMe_ReputationChangesFirstTime(t *testing.T) {
	repo := NewMockMeUserRepository()
	config := &OAuthConfig{JWTSecret: "test-secret-key"}

	reputationRepo := &MockBriefingReputationRepo{
		result: &models.ReputationChangesResult{
			SinceLastCheck: "+150",
			Breakdown: []models.ReputationEvent{
				{Reason: "first_briefing", PostID: "", PostTitle: "All activity since registration", Delta: 150},
			},
		},
	}

	handler := NewMeHandler(config, repo, nil, nil, nil)
	handler.reputationRepo = reputationRepo

	req := httptest.NewRequest(http.MethodGet, "/v1/me", nil)
	agent := &models.Agent{
		ID:             "first_time_agent",
		DisplayName:    "First Time Agent",
		Status:         "active",
		Reputation:     150,
		CreatedAt:      time.Now().Add(-7 * 24 * time.Hour),
		LastBriefingAt: nil, // Never called /me before
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

	repChanges, ok := data["reputation_changes"].(map[string]interface{})
	if !ok {
		t.Fatal("response missing 'reputation_changes' field or it's not an object")
	}

	sinceLastCheck := repChanges["since_last_check"].(string)
	if sinceLastCheck != "+150" {
		t.Errorf("expected since_last_check '+150', got %q", sinceLastCheck)
	}

	breakdown := repChanges["breakdown"].([]interface{})
	if len(breakdown) != 1 {
		t.Fatalf("expected 1 breakdown event for first briefing, got %d", len(breakdown))
	}

	event0 := breakdown[0].(map[string]interface{})
	if event0["reason"] != "first_briefing" {
		t.Errorf("expected reason 'first_briefing', got %q", event0["reason"])
	}
}

// TestAgentMe_ReputationChangesNone verifies that when no activity since last briefing,
// since_last_check is '+0' and breakdown is empty.
func TestAgentMe_ReputationChangesNone(t *testing.T) {
	repo := NewMockMeUserRepository()
	config := &OAuthConfig{JWTSecret: "test-secret-key"}

	reputationRepo := &MockBriefingReputationRepo{
		result: &models.ReputationChangesResult{
			SinceLastCheck: "+0",
			Breakdown:      []models.ReputationEvent{},
		},
	}

	lastBriefing := time.Now().Add(-1 * time.Hour)
	handler := NewMeHandler(config, repo, nil, nil, nil)
	handler.reputationRepo = reputationRepo

	req := httptest.NewRequest(http.MethodGet, "/v1/me", nil)
	agent := &models.Agent{
		ID:             "no_rep_change_agent",
		DisplayName:    "No Rep Change Agent",
		Status:         "active",
		LastBriefingAt: &lastBriefing,
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

	repChanges, ok := data["reputation_changes"].(map[string]interface{})
	if !ok {
		t.Fatal("response missing 'reputation_changes' field or it's not an object")
	}

	sinceLastCheck := repChanges["since_last_check"].(string)
	if sinceLastCheck != "+0" {
		t.Errorf("expected since_last_check '+0', got %q", sinceLastCheck)
	}

	breakdown := repChanges["breakdown"].([]interface{})
	if len(breakdown) != 0 {
		t.Errorf("expected 0 breakdown events, got %d", len(breakdown))
	}
}

// TestAgentMe_ReputationChangesGracefulDegradation verifies that if the reputation repo
// fails, the /me response still works but reputation_changes is nil.
func TestAgentMe_ReputationChangesGracefulDegradation(t *testing.T) {
	repo := NewMockMeUserRepository()
	config := &OAuthConfig{JWTSecret: "test-secret-key"}

	reputationRepo := &MockBriefingReputationRepo{
		err: context.DeadlineExceeded,
	}

	handler := NewMeHandler(config, repo, nil, nil, nil)
	handler.reputationRepo = reputationRepo

	req := httptest.NewRequest(http.MethodGet, "/v1/me", nil)
	agent := &models.Agent{
		ID:          "degraded_rep_agent",
		DisplayName: "Degraded Rep Agent",
		Status:      "active",
	}
	ctx := auth.ContextWithAgent(req.Context(), agent)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.Me(rr, req)

	// Should still return 200 even if reputation repo fails
	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var response map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&response)
	data := response["data"].(map[string]interface{})

	// reputation_changes should be null/nil (graceful degradation)
	if data["reputation_changes"] != nil {
		t.Errorf("expected reputation_changes to be nil on error, got %v", data["reputation_changes"])
	}
}
