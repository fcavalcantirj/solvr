package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
)

var errDatabaseError = errors.New("database error")

// --- Filter tests ---

func TestListNotifications_FilterUnread(t *testing.T) {
	userID := "user-filter-1"
	mockRepo := &MockNotificationsRepository{
		userNotifications:      []Notification{createTestNotification("n1", "Unread", "answer.created", &userID, nil)},
		userNotificationsTotal: 1,
	}

	handler := NewNotificationsHandler(mockRepo)
	req := httptest.NewRequest(http.MethodGet, "/v1/notifications?unread=true", nil)
	req = addNotificationsAuthContext(req, userID, "test@example.com", "user")
	w := httptest.NewRecorder()

	handler.List(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if mockRepo.lastFilters.Unread == nil || !*mockRepo.lastFilters.Unread {
		t.Error("expected unread filter to be true")
	}
}

func TestListNotifications_FilterType(t *testing.T) {
	userID := "user-filter-2"
	mockRepo := &MockNotificationsRepository{
		userNotifications:      []Notification{},
		userNotificationsTotal: 0,
	}

	handler := NewNotificationsHandler(mockRepo)
	req := httptest.NewRequest(http.MethodGet, "/v1/notifications?type=auto_solve_warning", nil)
	req = addNotificationsAuthContext(req, userID, "test@example.com", "user")
	w := httptest.NewRecorder()

	handler.List(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if mockRepo.lastFilters.Type != "auto_solve_warning" {
		t.Errorf("expected type filter 'auto_solve_warning', got %q", mockRepo.lastFilters.Type)
	}
}

func TestListNotifications_FilterBothUnreadAndType(t *testing.T) {
	userID := "user-filter-3"
	mockRepo := &MockNotificationsRepository{
		userNotifications:      []Notification{},
		userNotificationsTotal: 0,
	}

	handler := NewNotificationsHandler(mockRepo)
	req := httptest.NewRequest(http.MethodGet, "/v1/notifications?unread=true&type=mention", nil)
	req = addNotificationsAuthContext(req, userID, "test@example.com", "user")
	w := httptest.NewRecorder()

	handler.List(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if mockRepo.lastFilters.Unread == nil || !*mockRepo.lastFilters.Unread {
		t.Error("expected unread filter to be true")
	}
	if mockRepo.lastFilters.Type != "mention" {
		t.Errorf("expected type filter 'mention', got %q", mockRepo.lastFilters.Type)
	}
}

func TestListNotifications_NoFilters(t *testing.T) {
	userID := "user-filter-4"
	mockRepo := &MockNotificationsRepository{
		userNotifications:      []Notification{},
		userNotificationsTotal: 0,
	}

	handler := NewNotificationsHandler(mockRepo)
	req := httptest.NewRequest(http.MethodGet, "/v1/notifications", nil)
	req = addNotificationsAuthContext(req, userID, "test@example.com", "user")
	w := httptest.NewRecorder()

	handler.List(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if mockRepo.lastFilters.Unread != nil {
		t.Error("expected no unread filter")
	}
	if mockRepo.lastFilters.Type != "" {
		t.Errorf("expected no type filter, got %q", mockRepo.lastFilters.Type)
	}
}

// --- Delete single tests ---

func TestDelete_Success(t *testing.T) {
	userID := "user-del-1"
	notifID := "notif-del-1"
	notif := createTestNotification(notifID, "Delete me", "answer.created", &userID, nil)

	mockRepo := &MockNotificationsRepository{
		findByIDNotification: &notif,
	}

	handler := NewNotificationsHandler(mockRepo)
	req := httptest.NewRequest(http.MethodDelete, "/v1/notifications/"+notifID, nil)
	req = addNotificationsAuthContext(req, userID, "test@example.com", "user")
	req = addURLParam(req, "id", notifID)
	w := httptest.NewRecorder()

	handler.Delete(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d: %s", w.Code, w.Body.String())
	}
	if mockRepo.lastDeleteID != notifID {
		t.Errorf("expected lastDeleteID %q, got %q", notifID, mockRepo.lastDeleteID)
	}
}

func TestDelete_NoAuth(t *testing.T) {
	mockRepo := &MockNotificationsRepository{}
	handler := NewNotificationsHandler(mockRepo)
	req := httptest.NewRequest(http.MethodDelete, "/v1/notifications/notif-1", nil)
	req = addURLParam(req, "id", "notif-1")
	w := httptest.NewRecorder()

	handler.Delete(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestDelete_NotFound(t *testing.T) {
	userID := "user-del-2"
	mockRepo := &MockNotificationsRepository{
		findByIDErr: models.ErrNotificationNotFound,
	}

	handler := NewNotificationsHandler(mockRepo)
	req := httptest.NewRequest(http.MethodDelete, "/v1/notifications/notif-missing", nil)
	req = addNotificationsAuthContext(req, userID, "test@example.com", "user")
	req = addURLParam(req, "id", "notif-missing")
	w := httptest.NewRecorder()

	handler.Delete(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestDelete_Forbidden_NotOwner(t *testing.T) {
	ownerID := "user-owner"
	requesterID := "user-requester"
	notifID := "notif-forbidden"
	notif := createTestNotification(notifID, "Not yours", "answer.created", &ownerID, nil)

	mockRepo := &MockNotificationsRepository{
		findByIDNotification: &notif,
	}

	handler := NewNotificationsHandler(mockRepo)
	req := httptest.NewRequest(http.MethodDelete, "/v1/notifications/"+notifID, nil)
	req = addNotificationsAuthContext(req, requesterID, "test@example.com", "user")
	req = addURLParam(req, "id", notifID)
	w := httptest.NewRecorder()

	handler.Delete(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Code)
	}
}

func TestDelete_Success_ForAgent(t *testing.T) {
	agentID := "agent-del-1"
	notifID := "notif-agent-del"
	notif := createTestNotification(notifID, "Agent delete", "post.mentioned", nil, &agentID)

	mockRepo := &MockNotificationsRepository{
		findByIDNotification: &notif,
	}

	handler := NewNotificationsHandler(mockRepo)
	req := httptest.NewRequest(http.MethodDelete, "/v1/notifications/"+notifID, nil)
	req = addNotificationsAgentContext(req, agentID, "Test Agent")
	req = addURLParam(req, "id", notifID)
	w := httptest.NewRecorder()

	handler.Delete(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d: %s", w.Code, w.Body.String())
	}
}

func TestDelete_Forbidden_ForDifferentAgent(t *testing.T) {
	ownerAgentID := "agent-owner"
	requesterAgentID := "agent-requester"
	notifID := "notif-agent-forbidden"
	notif := createTestNotification(notifID, "Not yours", "post.mentioned", nil, &ownerAgentID)

	mockRepo := &MockNotificationsRepository{
		findByIDNotification: &notif,
	}

	handler := NewNotificationsHandler(mockRepo)
	req := httptest.NewRequest(http.MethodDelete, "/v1/notifications/"+notifID, nil)
	req = addNotificationsAgentContext(req, requesterAgentID, "Other Agent")
	req = addURLParam(req, "id", notifID)
	w := httptest.NewRecorder()

	handler.Delete(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Code)
	}
}

func TestDelete_DatabaseError(t *testing.T) {
	userID := "user-del-err"
	notifID := "notif-del-err"
	notif := createTestNotification(notifID, "Error test", "answer.created", &userID, nil)

	mockRepo := &MockNotificationsRepository{
		findByIDNotification: &notif,
		deleteErr:            errDatabaseError,
	}

	handler := NewNotificationsHandler(mockRepo)
	req := httptest.NewRequest(http.MethodDelete, "/v1/notifications/"+notifID, nil)
	req = addNotificationsAuthContext(req, userID, "test@example.com", "user")
	req = addURLParam(req, "id", notifID)
	w := httptest.NewRecorder()

	handler.Delete(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

// --- Bulk delete read tests ---

func TestDeleteAllRead_Success(t *testing.T) {
	userID := "user-delall-1"
	mockRepo := &MockNotificationsRepository{
		deleteAllReadForUserCount: 5,
	}

	handler := NewNotificationsHandler(mockRepo)
	req := httptest.NewRequest(http.MethodDelete, "/v1/notifications", nil)
	req = addNotificationsAuthContext(req, userID, "test@example.com", "user")
	w := httptest.NewRecorder()

	handler.DeleteAllRead(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if mockRepo.lastUserID != userID {
		t.Errorf("expected lastUserID %q, got %q", userID, mockRepo.lastUserID)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	data := response["data"].(map[string]interface{})
	if count := data["deleted_count"].(float64); int(count) != 5 {
		t.Errorf("expected deleted_count 5, got %v", data["deleted_count"])
	}
}

func TestDeleteAllRead_NoAuth(t *testing.T) {
	mockRepo := &MockNotificationsRepository{}
	handler := NewNotificationsHandler(mockRepo)
	req := httptest.NewRequest(http.MethodDelete, "/v1/notifications", nil)
	w := httptest.NewRecorder()

	handler.DeleteAllRead(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestDeleteAllRead_ZeroNotifications(t *testing.T) {
	userID := "user-delall-zero"
	mockRepo := &MockNotificationsRepository{
		deleteAllReadForUserCount: 0,
	}

	handler := NewNotificationsHandler(mockRepo)
	req := httptest.NewRequest(http.MethodDelete, "/v1/notifications", nil)
	req = addNotificationsAuthContext(req, userID, "test@example.com", "user")
	w := httptest.NewRecorder()

	handler.DeleteAllRead(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	data := response["data"].(map[string]interface{})
	if count := data["deleted_count"].(float64); int(count) != 0 {
		t.Errorf("expected deleted_count 0, got %v", data["deleted_count"])
	}
}

func TestDeleteAllRead_Success_ForAgent(t *testing.T) {
	agentID := "agent-delall-1"
	mockRepo := &MockNotificationsRepository{
		deleteAllReadForAgentCount: 3,
	}

	handler := NewNotificationsHandler(mockRepo)
	req := httptest.NewRequest(http.MethodDelete, "/v1/notifications", nil)
	req = addNotificationsAgentContext(req, agentID, "Test Agent")
	w := httptest.NewRecorder()

	handler.DeleteAllRead(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if mockRepo.lastAgentID != agentID {
		t.Errorf("expected lastAgentID %q, got %q", agentID, mockRepo.lastAgentID)
	}
}

func TestDeleteAllRead_DatabaseError(t *testing.T) {
	userID := "user-delall-err"
	mockRepo := &MockNotificationsRepository{
		deleteAllReadForUserErr: errDatabaseError,
	}

	handler := NewNotificationsHandler(mockRepo)
	req := httptest.NewRequest(http.MethodDelete, "/v1/notifications", nil)
	req = addNotificationsAuthContext(req, userID, "test@example.com", "user")
	w := httptest.NewRecorder()

	handler.DeleteAllRead(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

// Ensure errDatabaseError and addNotificationsAgentContext are available.
// They are defined in notifications_test.go in the same package.
// addURLParam is also from the same package.
// time import needed for createTestNotification used indirectly.
var _ = time.Now
