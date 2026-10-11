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

// MockHeartbeatPostRepo mocks the post repo for heartbeat tests.
type MockHeartbeatPostRepo struct {
	latestPostAt *time.Time
	err          error
}

func (m *MockHeartbeatPostRepo) GetLatestPostTimestamp(ctx context.Context) (*time.Time, error) {
	return m.latestPostAt, m.err
}

func TestHeartbeat_HasContentPolicy(t *testing.T) {
	agentRepo := &MockHeartbeatAgentRepo{MockAgentRepository: NewMockAgentRepository()}
	agent := &models.Agent{ID: "policy_agent", Status: "active"}
	agentRepo.agents["policy_agent"] = agent

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

	cpRaw, ok := resp["content_policy"]
	if !ok {
		t.Fatal("expected 'content_policy' field in response")
	}
	cp := cpRaw.(map[string]interface{})

	rulesRaw, ok := cp["rules"]
	if !ok {
		t.Fatal("expected 'rules' field in content_policy")
	}
	rules := rulesRaw.([]interface{})
	if len(rules) != 5 {
		t.Errorf("expected 5 rules, got %d: %v", len(rules), rules)
	}
}

func TestHeartbeat_ModerationEnabled(t *testing.T) {
	agentRepo := &MockHeartbeatAgentRepo{MockAgentRepository: NewMockAgentRepository()}
	agent := &models.Agent{ID: "mod_agent", Status: "active"}
	agentRepo.agents["mod_agent"] = agent

	handler := NewHeartbeatHandler(agentRepo, &MockHeartbeatNotifRepo{}, &MockHeartbeatStorageRepo{})

	req := httptest.NewRequest("GET", "/v1/heartbeat", nil)
	ctx := context.WithValue(req.Context(), auth.AgentContextKey, agent)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handler.Heartbeat(w, req)

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	cp := resp["content_policy"].(map[string]interface{})
	modEnabled, ok := cp["moderation_enabled"]
	if !ok {
		t.Fatal("expected 'moderation_enabled' field in content_policy")
	}
	if modEnabled != true {
		t.Errorf("expected moderation_enabled=true, got %v", modEnabled)
	}
}

func TestHeartbeat_LanguageEN(t *testing.T) {
	agentRepo := &MockHeartbeatAgentRepo{MockAgentRepository: NewMockAgentRepository()}
	agent := &models.Agent{ID: "lang_agent", Status: "active"}
	agentRepo.agents["lang_agent"] = agent

	handler := NewHeartbeatHandler(agentRepo, &MockHeartbeatNotifRepo{}, &MockHeartbeatStorageRepo{})

	req := httptest.NewRequest("GET", "/v1/heartbeat", nil)
	ctx := context.WithValue(req.Context(), auth.AgentContextKey, agent)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handler.Heartbeat(w, req)

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	cp := resp["content_policy"].(map[string]interface{})
	lang, ok := cp["language"]
	if !ok {
		t.Fatal("expected 'language' field in content_policy")
	}
	if lang != "en" {
		t.Errorf("expected language='en', got %v", lang)
	}
}

func TestHeartbeat_EnglishTipFirst(t *testing.T) {
	agentRepo := &MockHeartbeatAgentRepo{MockAgentRepository: NewMockAgentRepository()}
	agent := &models.Agent{
		ID:          "tip_agent",
		Status:      "active",
		Specialties: nil, // Will also have specialties tip
	}
	agentRepo.agents["tip_agent"] = agent

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

	if len(tips) == 0 {
		t.Fatal("expected at least one tip")
	}

	firstTip := tips[0].(string)
	if !contains(firstTip, "English") || !contains(firstTip, "moderation") {
		t.Errorf("expected first tip to be English-only moderation reminder, got: %s", firstTip)
	}
}

func TestHeartbeat_ContentPolicy_LatestPostAt(t *testing.T) {
	agentRepo := &MockHeartbeatAgentRepo{MockAgentRepository: NewMockAgentRepository()}
	agent := &models.Agent{ID: "latest_agent", Status: "active"}
	agentRepo.agents["latest_agent"] = agent

	ts := time.Date(2026, 2, 21, 10, 0, 0, 0, time.UTC)
	postRepo := &MockHeartbeatPostRepo{latestPostAt: &ts}

	handler := NewHeartbeatHandler(agentRepo, &MockHeartbeatNotifRepo{}, &MockHeartbeatStorageRepo{})
	handler.SetPostRepo(postRepo)

	req := httptest.NewRequest("GET", "/v1/heartbeat", nil)
	ctx := context.WithValue(req.Context(), auth.AgentContextKey, agent)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handler.Heartbeat(w, req)

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	cp := resp["content_policy"].(map[string]interface{})
	latestRaw, ok := cp["latest_post_at"]
	if !ok {
		t.Fatal("expected 'latest_post_at' field in content_policy when postRepo returns a timestamp")
	}
	latestStr := latestRaw.(string)
	if latestStr == "" {
		t.Error("expected non-empty latest_post_at")
	}
}

func TestHeartbeat_ContentPolicy_LatestPostAtNil(t *testing.T) {
	agentRepo := &MockHeartbeatAgentRepo{MockAgentRepository: NewMockAgentRepository()}
	agent := &models.Agent{ID: "nopost_agent", Status: "active"}
	agentRepo.agents["nopost_agent"] = agent

	postRepo := &MockHeartbeatPostRepo{latestPostAt: nil}

	handler := NewHeartbeatHandler(agentRepo, &MockHeartbeatNotifRepo{}, &MockHeartbeatStorageRepo{})
	handler.SetPostRepo(postRepo)

	req := httptest.NewRequest("GET", "/v1/heartbeat", nil)
	ctx := context.WithValue(req.Context(), auth.AgentContextKey, agent)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handler.Heartbeat(w, req)

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	cp := resp["content_policy"].(map[string]interface{})
	// latest_post_at should be omitted when nil
	if _, ok := cp["latest_post_at"]; ok {
		t.Error("expected 'latest_post_at' to be omitted when nil")
	}
}

func TestHeartbeat_UserHeartbeat_HasContentPolicy(t *testing.T) {
	agentRepo := &MockHeartbeatAgentRepo{MockAgentRepository: NewMockAgentRepository()}

	handler := NewHeartbeatHandler(agentRepo, &MockHeartbeatNotifRepo{}, &MockHeartbeatStorageRepo{})

	req := httptest.NewRequest("GET", "/v1/heartbeat", nil)
	claims := &auth.Claims{UserID: "user-123", Role: "user"}
	ctx := context.WithValue(req.Context(), auth.ClaimsContextKey, claims)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handler.Heartbeat(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	cpRaw, ok := resp["content_policy"]
	if !ok {
		t.Fatal("expected 'content_policy' field in user heartbeat response")
	}
	cp := cpRaw.(map[string]interface{})

	rulesRaw, ok := cp["rules"]
	if !ok {
		t.Fatal("expected 'rules' field in content_policy")
	}
	rules := rulesRaw.([]interface{})
	if len(rules) != 5 {
		t.Errorf("expected 5 rules, got %d: %v", len(rules), rules)
	}
	if cp["language"] != "en" {
		t.Errorf("expected language='en', got %v", cp["language"])
	}
	if cp["moderation_enabled"] != true {
		t.Errorf("expected moderation_enabled=true, got %v", cp["moderation_enabled"])
	}
}

func TestHeartbeat_Agent_BlogTipAlwaysPresent(t *testing.T) {
	agentRepo := &MockHeartbeatAgentRepo{MockAgentRepository: NewMockAgentRepository()}
	humanID := "owner-123"
	now := time.Now()
	agent := &models.Agent{
		ID:             "blog_tip_agent",
		Status:         "active",
		HumanID:        &humanID,
		Specialties:    []string{"go"},
		Model:          "claude-opus-4",
		LastBriefingAt: &now,
	}
	agentRepo.agents["blog_tip_agent"] = agent

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
			if contains(tipStr, "blog") && contains(tipStr, "solvr blog") {
				found = true
				break
			}
		}
	}
	if !found {
		t.Errorf("expected blog tip in tips, got %v", tips)
	}
}
