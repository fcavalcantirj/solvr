package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/auth"
	"github.com/fcavalcantirj/solvr/internal/models"
)

// MockBriefingInboxRepo implements BriefingInboxRepo for testing.
type MockBriefingInboxRepo struct {
	notifications []models.Notification
	totalUnread   int
	err           error
}

func (m *MockBriefingInboxRepo) GetRecentUnreadForAgent(ctx context.Context, agentID string, limit int) ([]models.Notification, int, error) {
	if m.err != nil {
		return nil, 0, m.err
	}
	// Filter notifications for this agent
	var result []models.Notification
	for _, n := range m.notifications {
		if n.AgentID != nil && *n.AgentID == agentID {
			result = append(result, n)
		}
	}
	if len(result) > limit {
		result = result[:limit]
	}
	return result, m.totalUnread, nil
}

// MockUpdateLastBriefingRepo implements UpdateLastBriefingRepo for testing.
type MockUpdateLastBriefingRepo struct {
	calledWith string
	err        error
}

func (m *MockUpdateLastBriefingRepo) UpdateLastBriefingAt(ctx context.Context, id string) error {
	m.calledWith = id
	return m.err
}

// TestAgentMe_IncludesInbox verifies that GET /me with agent API key includes inbox object.
func TestAgentMe_IncludesInbox(t *testing.T) {
	repo := NewMockMeUserRepository()
	config := &OAuthConfig{JWTSecret: "test-secret-key"}

	inboxRepo := &MockBriefingInboxRepo{
		notifications: []models.Notification{},
		totalUnread:   0,
	}
	briefingRepo := &MockUpdateLastBriefingRepo{}

	handler := NewMeHandler(config, repo, nil, nil, nil)
	handler.inboxRepo = inboxRepo
	handler.briefingRepo = briefingRepo

	req := httptest.NewRequest(http.MethodGet, "/v1/me", nil)
	agent := &models.Agent{
		ID:          "test_agent",
		DisplayName: "Test Agent",
		Status:      "active",
		Reputation:  100,
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

	// Inbox field must exist
	inbox, ok := data["inbox"].(map[string]interface{})
	if !ok {
		t.Fatal("response missing 'inbox' field or it's not an object")
	}

	// Should have unread_count and items
	if _, ok := inbox["unread_count"]; !ok {
		t.Error("inbox missing 'unread_count' field")
	}
	if _, ok := inbox["items"]; !ok {
		t.Error("inbox missing 'items' field")
	}
}

// TestAgentMe_InboxUnreadCount creates 5 notifications for agent and asserts unread_count = 5.
func TestAgentMe_InboxUnreadCount(t *testing.T) {
	repo := NewMockMeUserRepository()
	config := &OAuthConfig{JWTSecret: "test-secret-key"}
	agentID := "inbox_agent"

	// Create 5 notifications for the agent
	var notifications []models.Notification
	for i := 0; i < 5; i++ {
		notifications = append(notifications, models.Notification{
			ID:        "notif-" + string(rune('a'+i)),
			AgentID:   strPtr(agentID),
			Type:      "answer.created",
			Title:     "New answer",
			Body:      "Someone answered your question",
			Link:      "/questions/q1",
			CreatedAt: time.Now().Add(-time.Duration(i) * time.Minute),
		})
	}

	inboxRepo := &MockBriefingInboxRepo{
		notifications: notifications,
		totalUnread:   5,
	}
	briefingRepo := &MockUpdateLastBriefingRepo{}

	handler := NewMeHandler(config, repo, nil, nil, nil)
	handler.inboxRepo = inboxRepo
	handler.briefingRepo = briefingRepo

	req := httptest.NewRequest(http.MethodGet, "/v1/me", nil)
	agent := &models.Agent{
		ID:          agentID,
		DisplayName: "Inbox Agent",
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
	inbox := data["inbox"].(map[string]interface{})

	unreadCount := int(inbox["unread_count"].(float64))
	if unreadCount != 5 {
		t.Errorf("expected unread_count 5, got %d", unreadCount)
	}
}

// TestAgentMe_InboxItems creates 3 unread notifications with different types and verifies
// each item has type, title, body_preview (truncated 100 chars), link, created_at.
func TestAgentMe_InboxItems(t *testing.T) {
	repo := NewMockMeUserRepository()
	config := &OAuthConfig{JWTSecret: "test-secret-key"}
	agentID := "items_agent"

	longBody := strings.Repeat("A very detailed body that goes on and on. ", 10)

	notifications := []models.Notification{
		{
			ID:        "notif-1",
			AgentID:   strPtr(agentID),
			Type:      "answer.created",
			Title:     "New answer to your question",
			Body:      longBody,
			Link:      "/questions/q1",
			CreatedAt: time.Date(2026, 2, 19, 12, 0, 0, 0, time.UTC),
		},
		{
			ID:        "notif-2",
			AgentID:   strPtr(agentID),
			Type:      "comment.created",
			Title:     "New comment on your approach",
			Body:      "Short body",
			Link:      "/problems/p1",
			CreatedAt: time.Date(2026, 2, 19, 11, 0, 0, 0, time.UTC),
		},
		{
			ID:        "notif-3",
			AgentID:   strPtr(agentID),
			Type:      "approach.updated",
			Title:     "Approach status changed",
			Body:      "The approach was updated to stuck",
			Link:      "/problems/p2",
			CreatedAt: time.Date(2026, 2, 19, 10, 0, 0, 0, time.UTC),
		},
	}

	inboxRepo := &MockBriefingInboxRepo{
		notifications: notifications,
		totalUnread:   3,
	}
	briefingRepo := &MockUpdateLastBriefingRepo{}

	handler := NewMeHandler(config, repo, nil, nil, nil)
	handler.inboxRepo = inboxRepo
	handler.briefingRepo = briefingRepo

	req := httptest.NewRequest(http.MethodGet, "/v1/me", nil)
	agent := &models.Agent{
		ID:          agentID,
		DisplayName: "Items Agent",
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
	inbox := data["inbox"].(map[string]interface{})

	items, ok := inbox["items"].([]interface{})
	if !ok {
		t.Fatal("inbox.items is not an array")
	}
	if len(items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(items))
	}

	// Verify first item fields
	item0 := items[0].(map[string]interface{})
	if item0["type"] != "answer.created" {
		t.Errorf("expected type 'answer.created', got %q", item0["type"])
	}
	if item0["title"] != "New answer to your question" {
		t.Errorf("expected title 'New answer to your question', got %q", item0["title"])
	}
	if _, ok := item0["link"]; !ok {
		t.Error("item missing 'link' field")
	}
	if _, ok := item0["created_at"]; !ok {
		t.Error("item missing 'created_at' field")
	}

	// Verify body_preview is truncated to 100 chars for long body
	bodyPreview := item0["body_preview"].(string)
	if len(bodyPreview) > 100 {
		t.Errorf("body_preview should be truncated to 100 chars, got %d chars", len(bodyPreview))
	}

	// Verify second item has short body (not truncated)
	item1 := items[1].(map[string]interface{})
	if item1["body_preview"] != "Short body" {
		t.Errorf("expected body_preview 'Short body', got %q", item1["body_preview"])
	}

	// Verify third item
	item2 := items[2].(map[string]interface{})
	if item2["type"] != "approach.updated" {
		t.Errorf("expected type 'approach.updated', got %q", item2["type"])
	}
}

// TestAgentMe_InboxLimit creates 15 notifications and asserts inbox.items returns max 10.
func TestAgentMe_InboxLimit(t *testing.T) {
	repo := NewMockMeUserRepository()
	config := &OAuthConfig{JWTSecret: "test-secret-key"}
	agentID := "limit_agent"

	// Create 15 notifications
	var notifications []models.Notification
	for i := 0; i < 15; i++ {
		notifications = append(notifications, models.Notification{
			ID:        "notif-" + string(rune('a'+i)),
			AgentID:   strPtr(agentID),
			Type:      "answer.created",
			Title:     "Notification",
			Body:      "Body text",
			Link:      "/questions/q1",
			CreatedAt: time.Now().Add(-time.Duration(i) * time.Minute),
		})
	}

	inboxRepo := &MockBriefingInboxRepo{
		notifications: notifications,
		totalUnread:   15,
	}
	briefingRepo := &MockUpdateLastBriefingRepo{}

	handler := NewMeHandler(config, repo, nil, nil, nil)
	handler.inboxRepo = inboxRepo
	handler.briefingRepo = briefingRepo

	req := httptest.NewRequest(http.MethodGet, "/v1/me", nil)
	agent := &models.Agent{
		ID:          agentID,
		DisplayName: "Limit Agent",
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
	inbox := data["inbox"].(map[string]interface{})

	items := inbox["items"].([]interface{})
	if len(items) != 10 {
		t.Errorf("expected max 10 items, got %d", len(items))
	}

	// Total unread count should still reflect all 15
	unreadCount := int(inbox["unread_count"].(float64))
	if unreadCount != 15 {
		t.Errorf("expected unread_count 15, got %d", unreadCount)
	}
}

// TestAgentMe_InboxEmpty verifies agent with no notifications returns inbox.unread_count=0, inbox.items=[].
func TestAgentMe_InboxEmpty(t *testing.T) {
	repo := NewMockMeUserRepository()
	config := &OAuthConfig{JWTSecret: "test-secret-key"}

	inboxRepo := &MockBriefingInboxRepo{
		notifications: []models.Notification{},
		totalUnread:   0,
	}
	briefingRepo := &MockUpdateLastBriefingRepo{}

	handler := NewMeHandler(config, repo, nil, nil, nil)
	handler.inboxRepo = inboxRepo
	handler.briefingRepo = briefingRepo

	req := httptest.NewRequest(http.MethodGet, "/v1/me", nil)
	agent := &models.Agent{
		ID:          "empty_agent",
		DisplayName: "Empty Agent",
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
	inbox := data["inbox"].(map[string]interface{})

	unreadCount := int(inbox["unread_count"].(float64))
	if unreadCount != 0 {
		t.Errorf("expected unread_count 0, got %d", unreadCount)
	}

	items := inbox["items"].([]interface{})
	if len(items) != 0 {
		t.Errorf("expected 0 items, got %d", len(items))
	}
}

// TestAgentMe_InboxGracefulDegradation verifies that if the inbox repo fails,
// the /me response still works but inbox is nil.
func TestAgentMe_InboxGracefulDegradation(t *testing.T) {
	repo := NewMockMeUserRepository()
	config := &OAuthConfig{JWTSecret: "test-secret-key"}

	inboxRepo := &MockBriefingInboxRepo{
		err: context.DeadlineExceeded,
	}
	briefingRepo := &MockUpdateLastBriefingRepo{}

	handler := NewMeHandler(config, repo, nil, nil, nil)
	handler.inboxRepo = inboxRepo
	handler.briefingRepo = briefingRepo

	req := httptest.NewRequest(http.MethodGet, "/v1/me", nil)
	agent := &models.Agent{
		ID:          "degraded_agent",
		DisplayName: "Degraded Agent",
		Status:      "active",
	}
	ctx := auth.ContextWithAgent(req.Context(), agent)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.Me(rr, req)

	// Should still return 200 even if inbox fails
	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var response map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&response)
	data := response["data"].(map[string]interface{})

	// inbox should be null/nil (graceful degradation)
	if data["inbox"] != nil {
		t.Errorf("expected inbox to be nil on error, got %v", data["inbox"])
	}
}

// TestAgentMe_UpdatesLastBriefingAt verifies that calling GET /me updates last_briefing_at.
func TestAgentMe_UpdatesLastBriefingAt(t *testing.T) {
	repo := NewMockMeUserRepository()
	config := &OAuthConfig{JWTSecret: "test-secret-key"}

	inboxRepo := &MockBriefingInboxRepo{
		notifications: []models.Notification{},
		totalUnread:   0,
	}
	briefingRepo := &MockUpdateLastBriefingRepo{}

	handler := NewMeHandler(config, repo, nil, nil, nil)
	handler.inboxRepo = inboxRepo
	handler.briefingRepo = briefingRepo

	agentID := "briefing_agent"
	req := httptest.NewRequest(http.MethodGet, "/v1/me", nil)
	agent := &models.Agent{
		ID:          agentID,
		DisplayName: "Briefing Agent",
		Status:      "active",
	}
	ctx := auth.ContextWithAgent(req.Context(), agent)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.Me(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// Verify UpdateLastBriefingAt was called with the agent ID
	if briefingRepo.calledWith != agentID {
		t.Errorf("expected UpdateLastBriefingAt called with %q, got %q", agentID, briefingRepo.calledWith)
	}
}
