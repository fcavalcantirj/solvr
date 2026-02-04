// Package handlers contains HTTP request handlers for the Solvr API.
package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/auth"
	"github.com/fcavalcantirj/solvr/internal/models"
)

// MockNotificationsRepository is a mock implementation of NotificationsRepositoryInterface for testing.
type MockNotificationsRepository struct {
	// GetNotificationsForUser returns
	userNotifications      []Notification
	userNotificationsTotal int
	userNotificationsErr   error

	// GetNotificationsForAgent returns
	agentNotifications      []Notification
	agentNotificationsTotal int
	agentNotificationsErr   error

	// MarkRead returns
	markReadNotification *Notification
	markReadErr          error

	// MarkAllReadForUser returns
	markAllReadForUserCount int
	markAllReadForUserErr   error

	// MarkAllReadForAgent returns
	markAllReadForAgentCount int
	markAllReadForAgentErr   error

	// FindByID returns
	findByIDNotification *Notification
	findByIDErr          error

	// Track calls
	lastUserID      string
	lastAgentID     string
	lastPage        int
	lastPerPage     int
	lastMarkReadID  string
}

func (m *MockNotificationsRepository) GetNotificationsForUser(ctx context.Context, userID string, page, perPage int) ([]Notification, int, error) {
	m.lastUserID = userID
	m.lastPage = page
	m.lastPerPage = perPage
	return m.userNotifications, m.userNotificationsTotal, m.userNotificationsErr
}

func (m *MockNotificationsRepository) GetNotificationsForAgent(ctx context.Context, agentID string, page, perPage int) ([]Notification, int, error) {
	m.lastAgentID = agentID
	m.lastPage = page
	m.lastPerPage = perPage
	return m.agentNotifications, m.agentNotificationsTotal, m.agentNotificationsErr
}

func (m *MockNotificationsRepository) MarkRead(ctx context.Context, id string) (*Notification, error) {
	m.lastMarkReadID = id
	return m.markReadNotification, m.markReadErr
}

func (m *MockNotificationsRepository) MarkAllReadForUser(ctx context.Context, userID string) (int, error) {
	m.lastUserID = userID
	return m.markAllReadForUserCount, m.markAllReadForUserErr
}

func (m *MockNotificationsRepository) MarkAllReadForAgent(ctx context.Context, agentID string) (int, error) {
	m.lastAgentID = agentID
	return m.markAllReadForAgentCount, m.markAllReadForAgentErr
}

func (m *MockNotificationsRepository) FindByID(ctx context.Context, id string) (*Notification, error) {
	return m.findByIDNotification, m.findByIDErr
}

func createTestNotification(id, title, nType string, userID, agentID *string) Notification {
	n := Notification{
		ID:        id,
		Type:      nType,
		Title:     title,
		Body:      "Test notification body",
		Link:      "/posts/123",
		ReadAt:    nil,
		CreatedAt: time.Now(),
	}
	if userID != nil {
		n.UserID = userID
	}
	if agentID != nil {
		n.AgentID = agentID
	}
	return n
}

func addNotificationsAuthContext(req *http.Request, userID, email, role string) *http.Request {
	claims := &auth.Claims{
		UserID: userID,
		Email:  email,
		Role:   role,
	}
	ctx := auth.ContextWithClaims(req.Context(), claims)
	return req.WithContext(ctx)
}

// GET /v1/notifications - List notifications

func TestListNotifications_Success_ForUser(t *testing.T) {
	userID := "user-123"
	notifications := []Notification{
		createTestNotification("n1", "New answer", "answer.created", &userID, nil),
		createTestNotification("n2", "New comment", "comment.created", &userID, nil),
	}

	mockRepo := &MockNotificationsRepository{
		userNotifications:      notifications,
		userNotificationsTotal: 2,
	}

	handler := NewNotificationsHandler(mockRepo)

	req := httptest.NewRequest("GET", "/v1/notifications", nil)
	req = addNotificationsAuthContext(req, userID, "test@example.com", "user")
	w := httptest.NewRecorder()

	handler.List(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response NotificationsResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(response.Data) != 2 {
		t.Errorf("expected 2 notifications, got %d", len(response.Data))
	}

	if response.Meta.Total != 2 {
		t.Errorf("expected total 2, got %d", response.Meta.Total)
	}
}

func TestListNotifications_NoAuth(t *testing.T) {
	mockRepo := &MockNotificationsRepository{}
	handler := NewNotificationsHandler(mockRepo)

	req := httptest.NewRequest("GET", "/v1/notifications", nil)
	// No auth context added
	w := httptest.NewRecorder()

	handler.List(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	errObj, ok := response["error"].(map[string]interface{})
	if !ok {
		t.Fatal("expected error object in response")
	}

	if errObj["code"] != "UNAUTHORIZED" {
		t.Errorf("expected error code UNAUTHORIZED, got %v", errObj["code"])
	}
}

func TestListNotifications_Pagination(t *testing.T) {
	userID := "user-123"
	notifications := []Notification{
		createTestNotification("n11", "Notification 11", "answer.created", &userID, nil),
	}

	mockRepo := &MockNotificationsRepository{
		userNotifications:      notifications,
		userNotificationsTotal: 25, // Total 25, so page 2 of 10 has more
	}

	handler := NewNotificationsHandler(mockRepo)

	req := httptest.NewRequest("GET", "/v1/notifications?page=2&per_page=10", nil)
	req = addNotificationsAuthContext(req, userID, "test@example.com", "user")
	w := httptest.NewRecorder()

	handler.List(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response NotificationsResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Meta.Page != 2 {
		t.Errorf("expected page 2, got %d", response.Meta.Page)
	}

	if response.Meta.PerPage != 10 {
		t.Errorf("expected per_page 10, got %d", response.Meta.PerPage)
	}

	if !response.Meta.HasMore {
		t.Error("expected has_more to be true")
	}

	// Verify repo was called with correct pagination
	if mockRepo.lastPage != 2 {
		t.Errorf("expected repo to be called with page 2, got %d", mockRepo.lastPage)
	}
	if mockRepo.lastPerPage != 10 {
		t.Errorf("expected repo to be called with perPage 10, got %d", mockRepo.lastPerPage)
	}
}

func TestListNotifications_PerPageMax(t *testing.T) {
	userID := "user-123"
	mockRepo := &MockNotificationsRepository{
		userNotifications:      []Notification{},
		userNotificationsTotal: 0,
	}

	handler := NewNotificationsHandler(mockRepo)

	// Request per_page > 50, should be capped at 50
	req := httptest.NewRequest("GET", "/v1/notifications?per_page=100", nil)
	req = addNotificationsAuthContext(req, userID, "test@example.com", "user")
	w := httptest.NewRecorder()

	handler.List(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response NotificationsResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Meta.PerPage != 50 {
		t.Errorf("expected per_page capped at 50, got %d", response.Meta.PerPage)
	}
}

func TestListNotifications_EmptyResult(t *testing.T) {
	userID := "user-123"
	mockRepo := &MockNotificationsRepository{
		userNotifications:      []Notification{},
		userNotificationsTotal: 0,
	}

	handler := NewNotificationsHandler(mockRepo)

	req := httptest.NewRequest("GET", "/v1/notifications", nil)
	req = addNotificationsAuthContext(req, userID, "test@example.com", "user")
	w := httptest.NewRecorder()

	handler.List(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response NotificationsResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(response.Data) != 0 {
		t.Errorf("expected 0 notifications, got %d", len(response.Data))
	}

	if response.Meta.HasMore {
		t.Error("expected has_more to be false")
	}
}

func TestListNotifications_OrderByCreatedAtDesc(t *testing.T) {
	userID := "user-123"
	now := time.Now()
	notifications := []Notification{
		{ID: "n1", Type: "answer.created", Title: "Newest", CreatedAt: now, UserID: &userID},
		{ID: "n2", Type: "answer.created", Title: "Older", CreatedAt: now.Add(-1 * time.Hour), UserID: &userID},
	}

	mockRepo := &MockNotificationsRepository{
		userNotifications:      notifications,
		userNotificationsTotal: 2,
	}

	handler := NewNotificationsHandler(mockRepo)

	req := httptest.NewRequest("GET", "/v1/notifications", nil)
	req = addNotificationsAuthContext(req, userID, "test@example.com", "user")
	w := httptest.NewRecorder()

	handler.List(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response NotificationsResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// First should be newest (repository handles ordering)
	if response.Data[0].Title != "Newest" {
		t.Errorf("expected first notification to be 'Newest', got %s", response.Data[0].Title)
	}
}

// POST /v1/notifications/:id/read - Mark notification as read

func TestMarkRead_Success(t *testing.T) {
	userID := "user-123"
	now := time.Now()
	notification := &Notification{
		ID:        "notif-1",
		Type:      "answer.created",
		Title:     "New answer to your question",
		UserID:    &userID,
		ReadAt:    &now,
		CreatedAt: now.Add(-1 * time.Hour),
	}

	mockRepo := &MockNotificationsRepository{
		findByIDNotification: notification,
		markReadNotification: notification,
	}

	handler := NewNotificationsHandler(mockRepo)

	req := httptest.NewRequest("POST", "/v1/notifications/notif-1/read", nil)
	req = addNotificationsAuthContext(req, userID, "test@example.com", "user")
	// Add URL param (simulating router)
	req = addURLParam(req, "id", "notif-1")
	w := httptest.NewRecorder()

	handler.MarkRead(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response Notification
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.ReadAt == nil {
		t.Error("expected read_at to be set")
	}

	if mockRepo.lastMarkReadID != "notif-1" {
		t.Errorf("expected MarkRead to be called with 'notif-1', got %s", mockRepo.lastMarkReadID)
	}
}

func TestMarkRead_NoAuth(t *testing.T) {
	mockRepo := &MockNotificationsRepository{}
	handler := NewNotificationsHandler(mockRepo)

	req := httptest.NewRequest("POST", "/v1/notifications/notif-1/read", nil)
	// No auth context
	req = addURLParam(req, "id", "notif-1")
	w := httptest.NewRecorder()

	handler.MarkRead(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

func TestMarkRead_NotFound(t *testing.T) {
	userID := "user-123"
	mockRepo := &MockNotificationsRepository{
		findByIDErr: ErrNotificationNotFound,
	}

	handler := NewNotificationsHandler(mockRepo)

	req := httptest.NewRequest("POST", "/v1/notifications/notif-unknown/read", nil)
	req = addNotificationsAuthContext(req, userID, "test@example.com", "user")
	req = addURLParam(req, "id", "notif-unknown")
	w := httptest.NewRecorder()

	handler.MarkRead(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}

func TestMarkRead_Forbidden_NotOwner(t *testing.T) {
	userID := "user-123"
	otherUserID := "other-user"
	notification := &Notification{
		ID:     "notif-1",
		Type:   "answer.created",
		Title:  "Someone else's notification",
		UserID: &otherUserID,
	}

	mockRepo := &MockNotificationsRepository{
		findByIDNotification: notification,
	}

	handler := NewNotificationsHandler(mockRepo)

	req := httptest.NewRequest("POST", "/v1/notifications/notif-1/read", nil)
	req = addNotificationsAuthContext(req, userID, "test@example.com", "user")
	req = addURLParam(req, "id", "notif-1")
	w := httptest.NewRecorder()

	handler.MarkRead(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected status %d, got %d", http.StatusForbidden, w.Code)
	}
}

// POST /v1/notifications/read-all - Mark all notifications as read

func TestMarkAllRead_Success(t *testing.T) {
	userID := "user-123"
	mockRepo := &MockNotificationsRepository{
		markAllReadForUserCount: 5,
	}

	handler := NewNotificationsHandler(mockRepo)

	req := httptest.NewRequest("POST", "/v1/notifications/read-all", nil)
	req = addNotificationsAuthContext(req, userID, "test@example.com", "user")
	w := httptest.NewRecorder()

	handler.MarkAllRead(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Should return count of marked notifications
	data, ok := response["data"].(map[string]interface{})
	if !ok {
		t.Fatal("expected data object in response")
	}

	if data["marked_count"].(float64) != 5 {
		t.Errorf("expected marked_count 5, got %v", data["marked_count"])
	}

	if mockRepo.lastUserID != userID {
		t.Errorf("expected MarkAllReadForUser to be called with %s, got %s", userID, mockRepo.lastUserID)
	}
}

func TestMarkAllRead_NoAuth(t *testing.T) {
	mockRepo := &MockNotificationsRepository{}
	handler := NewNotificationsHandler(mockRepo)

	req := httptest.NewRequest("POST", "/v1/notifications/read-all", nil)
	// No auth context
	w := httptest.NewRecorder()

	handler.MarkAllRead(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

func TestMarkAllRead_ZeroNotifications(t *testing.T) {
	userID := "user-123"
	mockRepo := &MockNotificationsRepository{
		markAllReadForUserCount: 0,
	}

	handler := NewNotificationsHandler(mockRepo)

	req := httptest.NewRequest("POST", "/v1/notifications/read-all", nil)
	req = addNotificationsAuthContext(req, userID, "test@example.com", "user")
	w := httptest.NewRecorder()

	handler.MarkAllRead(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	data, ok := response["data"].(map[string]interface{})
	if !ok {
		t.Fatal("expected data object in response")
	}

	if data["marked_count"].(float64) != 0 {
		t.Errorf("expected marked_count 0, got %v", data["marked_count"])
	}
}

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

// Helper function to add URL parameter to request context
// Note: urlParamKey type is defined in errors.go
func addURLParam(req *http.Request, key, value string) *http.Request {
	ctx := context.WithValue(req.Context(), urlParamKey(key), value)
	return req.WithContext(ctx)
}

func containsString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
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
