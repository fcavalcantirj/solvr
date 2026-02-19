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

// MockBriefingSuggestedActionsRepo implements BriefingSuggestedActionsRepo for testing.
type MockBriefingSuggestedActionsRepo struct {
	actions []SuggestedAction
	err     error
}

func (m *MockBriefingSuggestedActionsRepo) GetSuggestedActionsForAgent(ctx context.Context, agentID string) ([]SuggestedAction, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.actions, nil
}

// TestAgentMe_SuggestedActionsStaleApproach verifies that an agent with a stale approach
// (marked 'working' 3 days ago) gets a suggested action to update the approach status.
func TestAgentMe_SuggestedActionsStaleApproach(t *testing.T) {
	repo := NewMockMeUserRepository()
	config := &OAuthConfig{JWTSecret: "test-secret-key"}

	suggestedActionsRepo := &MockBriefingSuggestedActionsRepo{
		actions: []SuggestedAction{
			{
				Action:      "update_approach_status",
				TargetID:    "approach-stale-1",
				TargetTitle: "Fix async bug",
				Reason:      "Marked working 3 days ago. Succeeded or failed?",
			},
		},
	}

	handler := NewMeHandler(config, repo, nil, nil, nil)
	handler.suggestedActionsRepo = suggestedActionsRepo

	req := httptest.NewRequest(http.MethodGet, "/v1/me", nil)
	agent := &models.Agent{
		ID:          "stale_approach_agent",
		DisplayName: "Stale Approach Agent",
		Status:      "active",
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

	actions, ok := data["suggested_actions"].([]interface{})
	if !ok {
		t.Fatal("response missing 'suggested_actions' field or it's not an array")
	}
	if len(actions) != 1 {
		t.Fatalf("expected 1 suggested action, got %d", len(actions))
	}

	action0 := actions[0].(map[string]interface{})
	if action0["action"] != "update_approach_status" {
		t.Errorf("expected action 'update_approach_status', got %q", action0["action"])
	}
	if action0["target_id"] != "approach-stale-1" {
		t.Errorf("expected target_id 'approach-stale-1', got %q", action0["target_id"])
	}
	if action0["target_title"] != "Fix async bug" {
		t.Errorf("expected target_title 'Fix async bug', got %q", action0["target_title"])
	}
	if action0["reason"] != "Marked working 3 days ago. Succeeded or failed?" {
		t.Errorf("expected reason about stale approach, got %q", action0["reason"])
	}
}

// TestAgentMe_SuggestedActionsRespondToComment verifies that when someone comments on
// the agent's problem asking for clarification, the agent gets a suggested action.
func TestAgentMe_SuggestedActionsRespondToComment(t *testing.T) {
	repo := NewMockMeUserRepository()
	config := &OAuthConfig{JWTSecret: "test-secret-key"}

	suggestedActionsRepo := &MockBriefingSuggestedActionsRepo{
		actions: []SuggestedAction{
			{
				Action:      "respond_to_comment",
				TargetID:    "comment-clarify-1",
				TargetTitle: "Race condition in async handler",
				Reason:      "Someone asked for clarification on your problem",
			},
		},
	}

	handler := NewMeHandler(config, repo, nil, nil, nil)
	handler.suggestedActionsRepo = suggestedActionsRepo

	req := httptest.NewRequest(http.MethodGet, "/v1/me", nil)
	agent := &models.Agent{
		ID:          "comment_agent",
		DisplayName: "Comment Agent",
		Status:      "active",
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

	actions := data["suggested_actions"].([]interface{})
	if len(actions) != 1 {
		t.Fatalf("expected 1 suggested action, got %d", len(actions))
	}

	action0 := actions[0].(map[string]interface{})
	if action0["action"] != "respond_to_comment" {
		t.Errorf("expected action 'respond_to_comment', got %q", action0["action"])
	}
	if action0["target_id"] != "comment-clarify-1" {
		t.Errorf("expected target_id 'comment-clarify-1', got %q", action0["target_id"])
	}
	if action0["reason"] != "Someone asked for clarification on your problem" {
		t.Errorf("expected reason about clarification, got %q", action0["reason"])
	}
}

// TestAgentMe_SuggestedActionsLimit verifies that max 5 suggested actions are returned
// when many nudge conditions exist, prioritized by urgency.
func TestAgentMe_SuggestedActionsLimit(t *testing.T) {
	repo := NewMockMeUserRepository()
	config := &OAuthConfig{JWTSecret: "test-secret-key"}

	// Create 8 actions - repo returns many, but handler should cap at 5
	var manyActions []SuggestedAction
	for i := 0; i < 8; i++ {
		manyActions = append(manyActions, SuggestedAction{
			Action:      "update_approach_status",
			TargetID:    "approach-" + string(rune('a'+i)),
			TargetTitle: "Problem " + string(rune('A'+i)),
			Reason:      "Stale approach needs update",
		})
	}

	suggestedActionsRepo := &MockBriefingSuggestedActionsRepo{
		actions: manyActions,
	}

	handler := NewMeHandler(config, repo, nil, nil, nil)
	handler.suggestedActionsRepo = suggestedActionsRepo

	req := httptest.NewRequest(http.MethodGet, "/v1/me", nil)
	agent := &models.Agent{
		ID:          "limit_actions_agent",
		DisplayName: "Limit Actions Agent",
		Status:      "active",
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

	actions := data["suggested_actions"].([]interface{})
	if len(actions) > 5 {
		t.Errorf("expected max 5 suggested actions, got %d", len(actions))
	}
	if len(actions) != 5 {
		t.Errorf("expected exactly 5 suggested actions (capped), got %d", len(actions))
	}
}

// TestAgentMe_SuggestedActionsEmpty verifies that an agent with no stale work
// returns an empty suggested_actions array (not nil).
func TestAgentMe_SuggestedActionsEmpty(t *testing.T) {
	repo := NewMockMeUserRepository()
	config := &OAuthConfig{JWTSecret: "test-secret-key"}

	suggestedActionsRepo := &MockBriefingSuggestedActionsRepo{
		actions: []SuggestedAction{},
	}

	handler := NewMeHandler(config, repo, nil, nil, nil)
	handler.suggestedActionsRepo = suggestedActionsRepo

	req := httptest.NewRequest(http.MethodGet, "/v1/me", nil)
	agent := &models.Agent{
		ID:          "no_actions_agent",
		DisplayName: "No Actions Agent",
		Status:      "active",
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

	actions, ok := data["suggested_actions"].([]interface{})
	if !ok {
		t.Fatal("response missing 'suggested_actions' field or it's not an array")
	}
	if len(actions) != 0 {
		t.Errorf("expected 0 suggested actions, got %d", len(actions))
	}
}

// TestAgentMe_SuggestedActionsGracefulDegradation verifies that if the suggested actions
// repo fails, the /me response still works but suggested_actions defaults to empty array.
func TestAgentMe_SuggestedActionsGracefulDegradation(t *testing.T) {
	repo := NewMockMeUserRepository()
	config := &OAuthConfig{JWTSecret: "test-secret-key"}

	suggestedActionsRepo := &MockBriefingSuggestedActionsRepo{
		err: context.DeadlineExceeded,
	}

	handler := NewMeHandler(config, repo, nil, nil, nil)
	handler.suggestedActionsRepo = suggestedActionsRepo

	req := httptest.NewRequest(http.MethodGet, "/v1/me", nil)
	agent := &models.Agent{
		ID:          "degraded_actions_agent",
		DisplayName: "Degraded Actions Agent",
		Status:      "active",
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

	// suggested_actions should default to empty array on error, not nil
	actions, ok := data["suggested_actions"].([]interface{})
	if !ok {
		t.Fatal("expected suggested_actions to be an array even on error")
	}
	if len(actions) != 0 {
		t.Errorf("expected 0 suggested actions on error, got %d", len(actions))
	}
}
