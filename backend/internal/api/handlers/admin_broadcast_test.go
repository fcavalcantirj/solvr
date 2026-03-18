package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// mockEmailSender tracks calls and can be configured to fail.
type mockEmailSender struct {
	calls     []mockSendCall
	failOnIdx int // -1 means no failure
}

type mockSendCall struct {
	To      string
	Subject string
	HTML    string
	Text    string
	Headers map[string]string
}

func (m *mockEmailSender) Send(ctx context.Context, to, subject, htmlBody, textBody string, headers ...map[string]string) error {
	call := mockSendCall{To: to, Subject: subject, HTML: htmlBody, Text: textBody}
	if len(headers) > 0 {
		call.Headers = headers[0]
	}
	m.calls = append(m.calls, call)
	if m.failOnIdx >= 0 && len(m.calls)-1 == m.failOnIdx {
		return fmt.Errorf("send failed for %s", to)
	}
	return nil
}

// mockEmailBroadcastRepo tracks CreateLog and UpdateStatusAndCounts calls.
type mockEmailBroadcastRepo struct {
	createLogCalled bool
	createLogInput  *models.EmailBroadcast
	updateCalled    bool
	updateID        string
	updateStatus    string
	updateSent      int
	updateFailed    int
	listResult      []models.EmailBroadcast
	listErr         error
}

func (m *mockEmailBroadcastRepo) CreateLog(ctx context.Context, broadcast *models.EmailBroadcast) (*models.EmailBroadcast, error) {
	m.createLogCalled = true
	m.createLogInput = broadcast
	result := *broadcast
	result.ID = "test-broadcast-id"
	result.StartedAt = time.Now()
	result.CreatedAt = time.Now()
	return &result, nil
}

func (m *mockEmailBroadcastRepo) UpdateStatusAndCounts(ctx context.Context, id string, status string, sentCount, failedCount int, completedAt *time.Time) error {
	m.updateCalled = true
	m.updateID = id
	m.updateStatus = status
	m.updateSent = sentCount
	m.updateFailed = failedCount
	return nil
}

func (m *mockEmailBroadcastRepo) List(ctx context.Context) ([]models.EmailBroadcast, error) {
	return m.listResult, m.listErr
}

// mockUserEmailRepo returns a fixed list of recipients.
type mockUserEmailRepo struct {
	recipients []models.EmailRecipient
}

func (m *mockUserEmailRepo) ListActiveEmails(ctx context.Context) ([]models.EmailRecipient, error) {
	return m.recipients, nil
}

func TestBroadcastEmail_Unauthorized(t *testing.T) {
	os.Setenv("ADMIN_API_KEY", "test-admin-key")
	defer os.Unsetenv("ADMIN_API_KEY")

	handler := NewAdminHandler(nil)

	body := `{"subject":"Test","body_html":"<p>Hi</p>"}`
	req := httptest.NewRequest(http.MethodPost, "/admin/email/broadcast", bytes.NewBufferString(body))
	// NO X-Admin-API-Key header
	w := httptest.NewRecorder()
	handler.BroadcastEmail(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestBroadcastEmail_MissingSubject(t *testing.T) {
	os.Setenv("ADMIN_API_KEY", "test-admin-key")
	defer os.Unsetenv("ADMIN_API_KEY")

	handler := NewAdminHandler(nil)
	handler.SetEmailSender(&mockEmailSender{failOnIdx: -1})

	body := `{"body_html":"<p>Hi</p>"}`
	req := httptest.NewRequest(http.MethodPost, "/admin/email/broadcast", bytes.NewBufferString(body))
	req.Header.Set("X-Admin-API-Key", "test-admin-key")
	w := httptest.NewRecorder()
	handler.BroadcastEmail(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	respJSON, _ := json.Marshal(resp)
	if !bytes.Contains(respJSON, []byte("MISSING_REQUIRED_FIELD")) {
		t.Errorf("expected response to contain MISSING_REQUIRED_FIELD, got: %s", respJSON)
	}
}

func TestBroadcastEmail_MissingBodyHTML(t *testing.T) {
	os.Setenv("ADMIN_API_KEY", "test-admin-key")
	defer os.Unsetenv("ADMIN_API_KEY")

	handler := NewAdminHandler(nil)
	handler.SetEmailSender(&mockEmailSender{failOnIdx: -1})

	body := `{"subject":"Test"}`
	req := httptest.NewRequest(http.MethodPost, "/admin/email/broadcast", bytes.NewBufferString(body))
	req.Header.Set("X-Admin-API-Key", "test-admin-key")
	w := httptest.NewRecorder()
	handler.BroadcastEmail(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	respJSON, _ := json.Marshal(resp)
	if !bytes.Contains(respJSON, []byte("MISSING_REQUIRED_FIELD")) {
		t.Errorf("expected response to contain MISSING_REQUIRED_FIELD, got: %s", respJSON)
	}
}

func TestBroadcastEmail_EmailNotConfigured(t *testing.T) {
	os.Setenv("ADMIN_API_KEY", "test-admin-key")
	defer os.Unsetenv("ADMIN_API_KEY")

	handler := NewAdminHandler(nil)
	// emailSender intentionally NOT set (nil)

	body := `{"subject":"Test","body_html":"<p>Hi</p>"}`
	req := httptest.NewRequest(http.MethodPost, "/admin/email/broadcast", bytes.NewBufferString(body))
	req.Header.Set("X-Admin-API-Key", "test-admin-key")
	w := httptest.NewRecorder()
	handler.BroadcastEmail(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	respJSON, _ := json.Marshal(resp)
	if !bytes.Contains(respJSON, []byte("EMAIL_NOT_CONFIGURED")) {
		t.Errorf("expected response to contain EMAIL_NOT_CONFIGURED, got: %s", respJSON)
	}
}

func TestBroadcastEmail_DryRun(t *testing.T) {
	os.Setenv("ADMIN_API_KEY", "test-admin-key")
	defer os.Unsetenv("ADMIN_API_KEY")

	sender := &mockEmailSender{failOnIdx: -1}
	broadcastRepo := &mockEmailBroadcastRepo{}
	userRepo := &mockUserEmailRepo{
		recipients: []models.EmailRecipient{
			{ID: "u1", Email: "alice@example.com", DisplayName: "Alice"},
			{ID: "u2", Email: "bob@example.com", DisplayName: "Bob"},
			{ID: "u3", Email: "charlie@example.com", DisplayName: "Charlie"},
		},
	}

	handler := NewAdminHandler(nil)
	handler.SetEmailSender(sender)
	handler.SetEmailBroadcastRepo(broadcastRepo)
	handler.SetUserEmailRepo(userRepo)

	body := `{"subject":"Newsletter","body_html":"<p>Hello world</p>","dry_run":true}`
	req := httptest.NewRequest(http.MethodPost, "/admin/email/broadcast", bytes.NewBufferString(body))
	req.Header.Set("X-Admin-API-Key", "test-admin-key")
	w := httptest.NewRecorder()
	handler.BroadcastEmail(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	wouldSend, ok := resp["would_send"]
	if !ok {
		t.Fatal("expected 'would_send' key in response")
	}
	if int(wouldSend.(float64)) != 3 {
		t.Errorf("expected would_send == 3, got %v", wouldSend)
	}

	recipients, ok := resp["recipients"]
	if !ok {
		t.Fatal("expected 'recipients' key in response")
	}
	recipList, ok := recipients.([]interface{})
	if !ok {
		t.Fatalf("expected recipients to be array, got %T", recipients)
	}
	if len(recipList) != 3 {
		t.Errorf("expected 3 recipients, got %d", len(recipList))
	}

	if len(sender.calls) != 0 {
		t.Errorf("expected 0 email sends in dry-run, got %d", len(sender.calls))
	}

	if broadcastRepo.createLogCalled {
		t.Error("expected no broadcast log created in dry-run")
	}
}

func TestBroadcastEmail_Success(t *testing.T) {
	os.Setenv("ADMIN_API_KEY", "test-admin-key")
	defer os.Unsetenv("ADMIN_API_KEY")

	sender := &mockEmailSender{failOnIdx: -1}
	broadcastRepo := &mockEmailBroadcastRepo{}
	userRepo := &mockUserEmailRepo{
		recipients: []models.EmailRecipient{
			{ID: "u1", Email: "alice@example.com", DisplayName: "Alice"},
			{ID: "u2", Email: "bob@example.com", DisplayName: "Bob"},
		},
	}

	handler := NewAdminHandler(nil)
	handler.SetEmailSender(sender)
	handler.SetEmailBroadcastRepo(broadcastRepo)
	handler.SetUserEmailRepo(userRepo)

	body := `{"subject":"Newsletter","body_html":"<p>Hello world</p>","dry_run":false}`
	req := httptest.NewRequest(http.MethodPost, "/admin/email/broadcast", bytes.NewBufferString(body))
	req.Header.Set("X-Admin-API-Key", "test-admin-key")
	w := httptest.NewRecorder()
	handler.BroadcastEmail(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp["broadcast_id"] != "test-broadcast-id" {
		t.Errorf("expected broadcast_id == 'test-broadcast-id', got %v", resp["broadcast_id"])
	}
	if int(resp["sent"].(float64)) != 2 {
		t.Errorf("expected sent == 2, got %v", resp["sent"])
	}
	if int(resp["failed"].(float64)) != 0 {
		t.Errorf("expected failed == 0, got %v", resp["failed"])
	}
	if int(resp["total"].(float64)) != 2 {
		t.Errorf("expected total == 2, got %v", resp["total"])
	}
	if _, ok := resp["duration_ms"]; !ok {
		t.Error("expected 'duration_ms' key in response")
	}

	if len(sender.calls) != 2 {
		t.Errorf("expected 2 email sends, got %d", len(sender.calls))
	}

	for i, call := range sender.calls {
		if call.Headers == nil {
			t.Errorf("call %d: expected List-Unsubscribe header, got nil headers", i)
			continue
		}
		if _, ok := call.Headers["List-Unsubscribe"]; !ok {
			t.Errorf("call %d: expected List-Unsubscribe key in headers, got %v", i, call.Headers)
		}
	}

	if !broadcastRepo.createLogCalled {
		t.Error("expected broadcast log to be created")
	}
	if !broadcastRepo.updateCalled {
		t.Error("expected broadcast log to be updated")
	}
	if broadcastRepo.updateStatus != "completed" {
		t.Errorf("expected updateStatus == 'completed', got %q", broadcastRepo.updateStatus)
	}
}

func TestBroadcastEmail_PartialFailure(t *testing.T) {
	os.Setenv("ADMIN_API_KEY", "test-admin-key")
	defer os.Unsetenv("ADMIN_API_KEY")

	// failOnIdx=1 means the second email (index 1) fails
	sender := &mockEmailSender{failOnIdx: 1}
	broadcastRepo := &mockEmailBroadcastRepo{}
	userRepo := &mockUserEmailRepo{
		recipients: []models.EmailRecipient{
			{ID: "u1", Email: "alice@example.com", DisplayName: "Alice"},
			{ID: "u2", Email: "bob@example.com", DisplayName: "Bob"},
			{ID: "u3", Email: "charlie@example.com", DisplayName: "Charlie"},
		},
	}

	handler := NewAdminHandler(nil)
	handler.SetEmailSender(sender)
	handler.SetEmailBroadcastRepo(broadcastRepo)
	handler.SetUserEmailRepo(userRepo)

	body := `{"subject":"Newsletter","body_html":"<p>Hello world</p>","dry_run":false}`
	req := httptest.NewRequest(http.MethodPost, "/admin/email/broadcast", bytes.NewBufferString(body))
	req.Header.Set("X-Admin-API-Key", "test-admin-key")
	w := httptest.NewRecorder()
	handler.BroadcastEmail(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if int(resp["sent"].(float64)) != 2 {
		t.Errorf("expected sent == 2, got %v", resp["sent"])
	}
	if int(resp["failed"].(float64)) != 1 {
		t.Errorf("expected failed == 1, got %v", resp["failed"])
	}
	if int(resp["total"].(float64)) != 3 {
		t.Errorf("expected total == 3, got %v", resp["total"])
	}

	if broadcastRepo.updateStatus != "completed" {
		t.Errorf("expected updateStatus == 'completed', got %q", broadcastRepo.updateStatus)
	}
	if broadcastRepo.updateSent != 2 {
		t.Errorf("expected updateSent == 2, got %d", broadcastRepo.updateSent)
	}
	if broadcastRepo.updateFailed != 1 {
		t.Errorf("expected updateFailed == 1, got %d", broadcastRepo.updateFailed)
	}
}

func TestListBroadcasts_Unauthorized(t *testing.T) {
	os.Setenv("ADMIN_API_KEY", "test-admin-key")
	defer os.Unsetenv("ADMIN_API_KEY")

	handler := NewAdminHandler(nil)

	req := httptest.NewRequest(http.MethodGet, "/admin/email/history", nil)
	// NO X-Admin-API-Key header
	w := httptest.NewRecorder()
	handler.ListBroadcasts(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestListBroadcasts_ReturnsBroadcasts(t *testing.T) {
	os.Setenv("ADMIN_API_KEY", "test-admin-key")
	defer os.Unsetenv("ADMIN_API_KEY")

	now := time.Now().UTC()
	completedAt := now.Add(12 * time.Second)
	broadcastRepo := &mockEmailBroadcastRepo{
		listResult: []models.EmailBroadcast{
			{
				ID:              "550e8400-e29b-41d4-a716-446655440000",
				Subject:         "Solvr Newsletter — March 2026",
				TotalRecipients: 87,
				SentCount:       85,
				FailedCount:     2,
				Status:          "completed",
				StartedAt:       now,
				CompletedAt:     &completedAt,
			},
			{
				ID:        "another-broadcast-id",
				Subject:   "Earlier Newsletter",
				SentCount: 10,
				Status:    "completed",
				StartedAt: now.Add(-24 * time.Hour),
			},
		},
	}

	handler := NewAdminHandler(nil)
	handler.SetEmailBroadcastRepo(broadcastRepo)

	req := httptest.NewRequest(http.MethodGet, "/admin/email/history", nil)
	req.Header.Set("X-Admin-API-Key", "test-admin-key")
	w := httptest.NewRecorder()
	handler.ListBroadcasts(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	broadcastsRaw, ok := resp["broadcasts"]
	if !ok {
		t.Fatal("expected 'broadcasts' key in response")
	}
	broadcastList, ok := broadcastsRaw.([]interface{})
	if !ok {
		t.Fatalf("expected broadcasts to be array, got %T", broadcastsRaw)
	}
	if len(broadcastList) != 2 {
		t.Errorf("expected 2 broadcasts, got %d", len(broadcastList))
	}

	first, ok := broadcastList[0].(map[string]interface{})
	if !ok {
		t.Fatalf("expected first broadcast to be object, got %T", broadcastList[0])
	}

	if first["broadcast_id"] != "550e8400-e29b-41d4-a716-446655440000" {
		t.Errorf("expected broadcast_id == '550e8400-e29b-41d4-a716-446655440000', got %v", first["broadcast_id"])
	}

	for _, key := range []string{"subject", "sent_count", "failed_count", "status", "started_at"} {
		if _, exists := first[key]; !exists {
			t.Errorf("expected key %q in first broadcast, not found", key)
		}
	}
}
