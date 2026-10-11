package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/auth"
	"github.com/fcavalcantirj/solvr/internal/models"
)

// Notification struct tests

func TestNotification_JSONSerialization(t *testing.T) {
	userID := "user-123"
	now := time.Date(2026, 2, 1, 12, 0, 0, 0, time.UTC)
	notification := Notification{
		ID:        "notif-123",
		UserID:    &userID,
		AgentID:   nil,
		Type:      "answer.created",
		Title:     "New answer to your question",
		Body:      "Someone answered your question...",
		Link:      "/questions/q-123",
		ReadAt:    nil,
		CreatedAt: now,
	}

	jsonBytes, err := json.Marshal(notification)
	if err != nil {
		t.Fatalf("failed to marshal notification: %v", err)
	}

	jsonStr := string(jsonBytes)

	expectedFields := []string{
		`"id":"notif-123"`,
		`"type":"answer.created"`,
		`"title":"New answer to your question"`,
		`"link":"/questions/q-123"`,
	}

	for _, field := range expectedFields {
		if !containsString(jsonStr, field) {
			t.Errorf("expected JSON to contain %s, got %s", field, jsonStr)
		}
	}
}

// Error handling tests

func TestListNotifications_DatabaseError(t *testing.T) {
	userID := "user-123"
	mockRepo := &MockNotificationsRepository{
		userNotificationsErr: errors.New("database connection failed"),
	}

	handler := NewNotificationsHandler(mockRepo)

	req := httptest.NewRequest("GET", "/v1/notifications", nil)
	req = addNotificationsAuthContext(req, userID, "test@example.com", "user")
	w := httptest.NewRecorder()

	handler.List(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	errObj, ok := response["error"].(map[string]interface{})
	if !ok {
		t.Fatal("expected error object in response")
	}

	if errObj["code"] != "INTERNAL_ERROR" {
		t.Errorf("expected error code INTERNAL_ERROR, got %v", errObj["code"])
	}
}

func TestMarkRead_DatabaseError(t *testing.T) {
	userID := "user-123"
	notification := &Notification{
		ID:     "notif-1",
		UserID: &userID,
	}

	mockRepo := &MockNotificationsRepository{
		findByIDNotification: notification,
		markReadErr:          errors.New("database error"),
	}

	handler := NewNotificationsHandler(mockRepo)

	req := httptest.NewRequest("POST", "/v1/notifications/notif-1/read", nil)
	req = addNotificationsAuthContext(req, userID, "test@example.com", "user")
	req = addURLParam(req, "id", "notif-1")
	w := httptest.NewRecorder()

	handler.MarkRead(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}
}

// Agent authentication tests (API key auth)
// Per FIX-018: Notifications should work with both JWT and API key auth

// addNotificationsAgentContext adds agent authentication context to a request.
func addNotificationsAgentContext(req *http.Request, agentID, displayName string) *http.Request {
	agent := &models.Agent{
		ID:          agentID,
		DisplayName: displayName,
	}
	ctx := auth.ContextWithAgent(req.Context(), agent)
	return req.WithContext(ctx)
}

func TestListNotifications_Success_ForAgent(t *testing.T) {
	agentID := "test_agent"
	notifications := []Notification{
		createTestNotification("notif-1", "Answer to your question", "answer.created", nil, &agentID),
		createTestNotification("notif-2", "Comment on your post", "comment.created", nil, &agentID),
	}

	mockRepo := &MockNotificationsRepository{
		agentNotifications:      notifications,
		agentNotificationsTotal: 2,
	}

	handler := NewNotificationsHandler(mockRepo)

	req := httptest.NewRequest("GET", "/v1/notifications", nil)
	req = addNotificationsAgentContext(req, agentID, "Test Agent")
	w := httptest.NewRecorder()

	handler.List(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	// Verify we called GetNotificationsForAgent, not GetNotificationsForUser
	if mockRepo.lastAgentID != agentID {
		t.Errorf("expected lastAgentID %s, got %s", agentID, mockRepo.lastAgentID)
	}

	var response NotificationsResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(response.Data) != 2 {
		t.Errorf("expected 2 notifications, got %d", len(response.Data))
	}
}

func TestMarkRead_Success_ForAgent(t *testing.T) {
	agentID := "test_agent"
	now := time.Now()
	notification := &Notification{
		ID:        "notif-1",
		AgentID:   &agentID,
		Type:      "answer.created",
		Title:     "New answer",
		ReadAt:    &now,
		CreatedAt: now,
	}

	mockRepo := &MockNotificationsRepository{
		findByIDNotification: notification,
		markReadNotification: notification,
	}

	handler := NewNotificationsHandler(mockRepo)

	req := httptest.NewRequest("POST", "/v1/notifications/notif-1/read", nil)
	req = addNotificationsAgentContext(req, agentID, "Test Agent")
	req = addURLParam(req, "id", "notif-1")
	w := httptest.NewRecorder()

	handler.MarkRead(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}
}

func TestMarkRead_Forbidden_ForDifferentAgent(t *testing.T) {
	agentID := "owner_agent"
	requesterID := "different_agent"
	notification := &Notification{
		ID:      "notif-1",
		AgentID: &agentID,
		Type:    "answer.created",
		Title:   "New answer",
	}

	mockRepo := &MockNotificationsRepository{
		findByIDNotification: notification,
	}

	handler := NewNotificationsHandler(mockRepo)

	req := httptest.NewRequest("POST", "/v1/notifications/notif-1/read", nil)
	req = addNotificationsAgentContext(req, requesterID, "Different Agent")
	req = addURLParam(req, "id", "notif-1")
	w := httptest.NewRecorder()

	handler.MarkRead(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected status %d, got %d", http.StatusForbidden, w.Code)
	}
}

func TestMarkAllRead_Success_ForAgent(t *testing.T) {
	agentID := "test_agent"

	mockRepo := &MockNotificationsRepository{
		markAllReadForAgentCount: 5,
	}

	handler := NewNotificationsHandler(mockRepo)

	req := httptest.NewRequest("POST", "/v1/notifications/read-all", nil)
	req = addNotificationsAgentContext(req, agentID, "Test Agent")
	w := httptest.NewRecorder()

	handler.MarkAllRead(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	// Verify we called MarkAllReadForAgent
	if mockRepo.lastAgentID != agentID {
		t.Errorf("expected lastAgentID %s, got %s", agentID, mockRepo.lastAgentID)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	data, ok := response["data"].(map[string]interface{})
	if !ok {
		t.Fatal("expected data object in response")
	}

	if count, ok := data["marked_count"].(float64); !ok || int(count) != 5 {
		t.Errorf("expected marked_count 5, got %v", data["marked_count"])
	}
}
