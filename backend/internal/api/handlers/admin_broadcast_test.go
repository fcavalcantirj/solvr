package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
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
	createLogCalled      bool
	createLogInput       *models.EmailBroadcast
	updateCalled         bool
	updateID             string
	updateStatus         string
	updateSent           int
	updateFailed         int
	listResult           []models.EmailBroadcast
	listErr              error
	recentBroadcast      *models.EmailBroadcast // returned by HasRecentBroadcast
	recentBroadcastErr   error
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

func (m *mockEmailBroadcastRepo) HasRecentBroadcast(ctx context.Context, subject string, window time.Duration) (*models.EmailBroadcast, error) {
	return m.recentBroadcast, m.recentBroadcastErr
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

func TestSubstituteTemplateVars(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		nameVal  string
		code     string
		link     string
		expected string
	}{
		{
			name:     "name only",
			body:     "Hello {name}!",
			nameVal:  "Alice",
			code:     "ABC123",
			link:     "https://solvr.dev/join?ref=ABC123",
			expected: "Hello Alice!",
		},
		{
			name:     "referral_code only",
			body:     "Code: {referral_code}",
			nameVal:  "Bob",
			code:     "XYZ789",
			link:     "https://solvr.dev/join?ref=XYZ789",
			expected: "Code: XYZ789",
		},
		{
			name:     "referral_link only",
			body:     "Click {referral_link}",
			nameVal:  "Carol",
			code:     "DEF456",
			link:     "https://solvr.dev/join?ref=DEF456",
			expected: "Click https://solvr.dev/join?ref=DEF456",
		},
		{
			name:     "all three vars",
			body:     "Hi {name}, share {referral_link} (code {referral_code})",
			nameVal:  "Dave",
			code:     "GHI012",
			link:     "https://solvr.dev/join?ref=GHI012",
			expected: "Hi Dave, share https://solvr.dev/join?ref=GHI012 (code GHI012)",
		},
		{
			name:     "no vars",
			body:     "Plain text email",
			nameVal:  "Eve",
			code:     "JKL345",
			link:     "https://solvr.dev/join?ref=JKL345",
			expected: "Plain text email",
		},
		{
			name:     "empty name",
			body:     "Hello {name}!",
			nameVal:  "",
			code:     "MNO678",
			link:     "https://solvr.dev/join?ref=MNO678",
			expected: "Hello !",
		},
		{
			name:     "empty code",
			body:     "Code: {referral_code}",
			nameVal:  "Frank",
			code:     "",
			link:     "",
			expected: "Code: ",
		},
		{
			name:     "multiple occurrences",
			body:     "{name} and {name}",
			nameVal:  "Grace",
			code:     "PQR901",
			link:     "https://solvr.dev/join?ref=PQR901",
			expected: "Grace and Grace",
		},
		{
			name:     "HTML body",
			body:     `<p>Hi {name}, use <a href="{referral_link}">link</a></p>`,
			nameVal:  "Hank",
			code:     "STU234",
			link:     "https://solvr.dev/join?ref=STU234",
			expected: `<p>Hi Hank, use <a href="https://solvr.dev/join?ref=STU234">link</a></p>`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := substituteTemplateVars(tt.body, tt.nameVal, tt.code, tt.link)
			if got != tt.expected {
				t.Errorf("substituteTemplateVars(%q, %q, %q, %q) = %q, want %q",
					tt.body, tt.nameVal, tt.code, tt.link, got, tt.expected)
			}
		})
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

func TestBroadcastEmail_TemplateSubstitution(t *testing.T) {
	os.Setenv("ADMIN_API_KEY", "test-admin-key")
	defer os.Unsetenv("ADMIN_API_KEY")

	sender := &mockEmailSender{failOnIdx: -1}
	broadcastRepo := &mockEmailBroadcastRepo{}
	userRepo := &mockUserEmailRepo{
		recipients: []models.EmailRecipient{
			{ID: "u1", Email: "alice@example.com", DisplayName: "Alice", ReferralCode: "ALICE123"},
			{ID: "u2", Email: "bob@example.com", DisplayName: "Bob", ReferralCode: "BOB456"},
		},
	}

	handler := NewAdminHandler(nil)
	handler.SetEmailSender(sender)
	handler.SetEmailBroadcastRepo(broadcastRepo)
	handler.SetUserEmailRepo(userRepo)

	bodyHTML := `<p>Hi {name}, share {referral_link} (code: {referral_code})</p>`
	bodyText := `Hi {name}, share {referral_link}`
	body := `{"subject":"Test","body_html":"` + bodyHTML + `","body_text":"` + bodyText + `","dry_run":false}`

	req := httptest.NewRequest(http.MethodPost, "/admin/email/broadcast", bytes.NewBufferString(body))
	req.Header.Set("X-Admin-API-Key", "test-admin-key")
	w := httptest.NewRecorder()
	handler.BroadcastEmail(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	if len(sender.calls) != 2 {
		t.Fatalf("expected 2 email sends, got %d", len(sender.calls))
	}

	// Verify recipient 1 (Alice)
	call0 := sender.calls[0]
	if !bytes.Contains([]byte(call0.HTML), []byte("Alice")) {
		t.Errorf("call[0].HTML should contain 'Alice', got: %s", call0.HTML)
	}
	if !bytes.Contains([]byte(call0.HTML), []byte("ALICE123")) {
		t.Errorf("call[0].HTML should contain 'ALICE123', got: %s", call0.HTML)
	}
	if !bytes.Contains([]byte(call0.HTML), []byte("https://solvr.dev/join?ref=ALICE123")) {
		t.Errorf("call[0].HTML should contain full referral link for ALICE123, got: %s", call0.HTML)
	}
	if !bytes.Contains([]byte(call0.Text), []byte("Alice")) {
		t.Errorf("call[0].Text should contain 'Alice', got: %s", call0.Text)
	}
	if !bytes.Contains([]byte(call0.Text), []byte("https://solvr.dev/join?ref=ALICE123")) {
		t.Errorf("call[0].Text should contain full referral link for ALICE123, got: %s", call0.Text)
	}

	// Verify recipient 2 (Bob)
	call1 := sender.calls[1]
	if !bytes.Contains([]byte(call1.HTML), []byte("Bob")) {
		t.Errorf("call[1].HTML should contain 'Bob', got: %s", call1.HTML)
	}
	if !bytes.Contains([]byte(call1.HTML), []byte("BOB456")) {
		t.Errorf("call[1].HTML should contain 'BOB456', got: %s", call1.HTML)
	}
	if !bytes.Contains([]byte(call1.HTML), []byte("https://solvr.dev/join?ref=BOB456")) {
		t.Errorf("call[1].HTML should contain full referral link for BOB456, got: %s", call1.HTML)
	}
	if !bytes.Contains([]byte(call1.Text), []byte("Bob")) {
		t.Errorf("call[1].Text should contain 'Bob', got: %s", call1.Text)
	}
	if !bytes.Contains([]byte(call1.Text), []byte("https://solvr.dev/join?ref=BOB456")) {
		t.Errorf("call[1].Text should contain full referral link for BOB456, got: %s", call1.Text)
	}

	// Verify raw template vars do NOT appear in sent bodies (EML-01, EML-02, EML-04)
	for i, call := range sender.calls {
		for _, token := range []string{"{name}", "{referral_code}", "{referral_link}"} {
			if bytes.Contains([]byte(call.HTML), []byte(token)) {
				t.Errorf("call[%d].HTML should NOT contain raw token %q, got: %s", i, token, call.HTML)
			}
			if bytes.Contains([]byte(call.Text), []byte(token)) {
				t.Errorf("call[%d].Text should NOT contain raw token %q, got: %s", i, token, call.Text)
			}
		}
	}
}

func TestBroadcastEmail_DryRunShowsSubstitutedPreview(t *testing.T) {
	os.Setenv("ADMIN_API_KEY", "test-admin-key")
	defer os.Unsetenv("ADMIN_API_KEY")

	sender := &mockEmailSender{failOnIdx: -1}
	broadcastRepo := &mockEmailBroadcastRepo{}
	userRepo := &mockUserEmailRepo{
		recipients: []models.EmailRecipient{
			{ID: "u1", Email: "alice@example.com", DisplayName: "Alice", ReferralCode: "ALICE123"},
			{ID: "u2", Email: "bob@example.com", DisplayName: "Bob", ReferralCode: "BOB456"},
		},
	}

	handler := NewAdminHandler(nil)
	handler.SetEmailSender(sender)
	handler.SetEmailBroadcastRepo(broadcastRepo)
	handler.SetUserEmailRepo(userRepo)

	bodyHTML := `<p>Hi {name}, use {referral_link} (code: {referral_code})</p>`
	bodyText := `Hi {name}, ref: {referral_link}`
	body := `{"subject":"Test","body_html":"` + bodyHTML + `","body_text":"` + bodyText + `","dry_run":true}`

	req := httptest.NewRequest(http.MethodPost, "/admin/email/broadcast", bytes.NewBufferString(body))
	req.Header.Set("X-Admin-API-Key", "test-admin-key")
	w := httptest.NewRecorder()
	handler.BroadcastEmail(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// No emails should be sent in dry-run
	if len(sender.calls) != 0 {
		t.Errorf("expected 0 email sends in dry-run, got %d", len(sender.calls))
	}

	// Preview must be present
	previewRaw, ok := resp["preview"]
	if !ok {
		t.Fatal("expected 'preview' key in dry-run response")
	}
	preview, ok := previewRaw.(map[string]interface{})
	if !ok {
		t.Fatalf("expected preview to be object, got %T", previewRaw)
	}

	// preview.body_html should have Alice's substituted values
	previewHTML, ok := preview["body_html"].(string)
	if !ok {
		t.Fatal("expected preview.body_html to be a string")
	}
	if !bytes.Contains([]byte(previewHTML), []byte("Alice")) {
		t.Errorf("preview.body_html should contain 'Alice', got: %s", previewHTML)
	}
	if !bytes.Contains([]byte(previewHTML), []byte("ALICE123")) {
		t.Errorf("preview.body_html should contain 'ALICE123', got: %s", previewHTML)
	}
	if !bytes.Contains([]byte(previewHTML), []byte("https://solvr.dev/join?ref=ALICE123")) {
		t.Errorf("preview.body_html should contain referral link for ALICE123, got: %s", previewHTML)
	}

	// preview.body_text should have Alice's substituted values
	previewText, ok := preview["body_text"].(string)
	if !ok {
		t.Fatal("expected preview.body_text to be a string")
	}
	if !bytes.Contains([]byte(previewText), []byte("Alice")) {
		t.Errorf("preview.body_text should contain 'Alice', got: %s", previewText)
	}
	if !bytes.Contains([]byte(previewText), []byte("https://solvr.dev/join?ref=ALICE123")) {
		t.Errorf("preview.body_text should contain referral link for ALICE123, got: %s", previewText)
	}

	// Raw template vars must NOT appear in preview
	for _, token := range []string{"{name}", "{referral_code}", "{referral_link}"} {
		if bytes.Contains([]byte(previewHTML), []byte(token)) {
			t.Errorf("preview.body_html should NOT contain raw token %q, got: %s", token, previewHTML)
		}
		if bytes.Contains([]byte(previewText), []byte(token)) {
			t.Errorf("preview.body_text should NOT contain raw token %q, got: %s", token, previewText)
		}
	}
}

// TestBroadcastEmail_Deduplication_BlocksDuplicate tests that a broadcast
// with the same subject as a recent completed broadcast is blocked.
func TestBroadcastEmail_Deduplication_BlocksDuplicate(t *testing.T) {
	os.Setenv("ADMIN_API_KEY", "test-admin-key")
	defer os.Unsetenv("ADMIN_API_KEY")

	sender := &mockEmailSender{failOnIdx: -1}
	broadcastRepo := &mockEmailBroadcastRepo{
		recentBroadcast: &models.EmailBroadcast{
			ID:              "prev-broadcast-id",
			Subject:         "Newsletter",
			TotalRecipients: 315,
			SentCount:       315,
			Status:          "completed",
			StartedAt:       time.Now().Add(-30 * time.Minute),
		},
	}
	userRepo := &mockUserEmailRepo{
		recipients: []models.EmailRecipient{
			{ID: "u1", Email: "alice@example.com", DisplayName: "Alice"},
		},
	}

	handler := NewAdminHandler(nil)
	handler.SetEmailSender(sender)
	handler.SetEmailBroadcastRepo(broadcastRepo)
	handler.SetUserEmailRepo(userRepo)

	body := `{"subject":"Newsletter","body_html":"<p>Hello</p>","dry_run":false}`
	req := httptest.NewRequest(http.MethodPost, "/admin/email/broadcast", strings.NewReader(body))
	req.Header.Set("X-Admin-API-Key", "test-admin-key")
	rr := httptest.NewRecorder()

	handler.BroadcastEmail(rr, req)

	if rr.Code != http.StatusConflict {
		t.Errorf("expected 409 Conflict, got %d", rr.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp["error"] != "DUPLICATE_BROADCAST" {
		t.Errorf("expected DUPLICATE_BROADCAST error, got %v", resp["error"])
	}

	// No emails should be sent
	if len(sender.calls) != 0 {
		t.Errorf("expected 0 email sends, got %d", len(sender.calls))
	}
}

// TestBroadcastEmail_Deduplication_ForceOverride tests that force=true bypasses dedup.
func TestBroadcastEmail_Deduplication_ForceOverride(t *testing.T) {
	os.Setenv("ADMIN_API_KEY", "test-admin-key")
	defer os.Unsetenv("ADMIN_API_KEY")

	sender := &mockEmailSender{failOnIdx: -1}
	broadcastRepo := &mockEmailBroadcastRepo{
		recentBroadcast: &models.EmailBroadcast{
			ID:              "prev-broadcast-id",
			Subject:         "Newsletter",
			TotalRecipients: 315,
			SentCount:       315,
			Status:          "completed",
			StartedAt:       time.Now().Add(-30 * time.Minute),
		},
	}
	userRepo := &mockUserEmailRepo{
		recipients: []models.EmailRecipient{
			{ID: "u1", Email: "alice@example.com", DisplayName: "Alice"},
		},
	}

	handler := NewAdminHandler(nil)
	handler.SetEmailSender(sender)
	handler.SetEmailBroadcastRepo(broadcastRepo)
	handler.SetUserEmailRepo(userRepo)

	body := `{"subject":"Newsletter","body_html":"<p>Hello</p>","dry_run":false,"force":true}`
	req := httptest.NewRequest(http.MethodPost, "/admin/email/broadcast", strings.NewReader(body))
	req.Header.Set("X-Admin-API-Key", "test-admin-key")
	rr := httptest.NewRecorder()

	handler.BroadcastEmail(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200 OK with force=true, got %d: %s", rr.Code, rr.Body.String())
	}

	// Email SHOULD be sent with force=true
	if len(sender.calls) != 1 {
		t.Errorf("expected 1 email send with force=true, got %d", len(sender.calls))
	}
}

// TestBroadcastEmail_Deduplication_SingleRecipientAlsoBlocked tests that the "to" field
// does NOT bypass dedup — a retry to a single recipient should also be blocked.
func TestBroadcastEmail_Deduplication_SingleRecipientAlsoBlocked(t *testing.T) {
	os.Setenv("ADMIN_API_KEY", "test-admin-key")
	defer os.Unsetenv("ADMIN_API_KEY")

	sender := &mockEmailSender{failOnIdx: -1}
	broadcastRepo := &mockEmailBroadcastRepo{
		recentBroadcast: &models.EmailBroadcast{
			ID:              "prev-broadcast-id",
			Subject:         "Newsletter",
			TotalRecipients: 315,
			SentCount:       315,
			Status:          "completed",
			StartedAt:       time.Now().Add(-30 * time.Minute),
		},
	}
	userRepo := &mockUserEmailRepo{
		recipients: []models.EmailRecipient{
			{ID: "u1", Email: "alice@example.com", DisplayName: "Alice"},
		},
	}

	handler := NewAdminHandler(nil)
	handler.SetEmailSender(sender)
	handler.SetEmailBroadcastRepo(broadcastRepo)
	handler.SetUserEmailRepo(userRepo)

	body := `{"subject":"Newsletter","body_html":"<p>Hello</p>","to":"alice@example.com"}`
	req := httptest.NewRequest(http.MethodPost, "/admin/email/broadcast", strings.NewReader(body))
	req.Header.Set("X-Admin-API-Key", "test-admin-key")
	rr := httptest.NewRecorder()

	handler.BroadcastEmail(rr, req)

	// Single-recipient sends should ALSO be blocked by dedup
	if rr.Code != http.StatusConflict {
		t.Errorf("expected 409 Conflict for single-recipient dedup, got %d: %s", rr.Code, rr.Body.String())
	}
	if len(sender.calls) != 0 {
		t.Errorf("expected 0 email sends, got %d", len(sender.calls))
	}
}

// TestBroadcastEmail_Deduplication_RepoErrorFailsClosed tests that if HasRecentBroadcast
// returns an error, the broadcast is BLOCKED (fail-closed), not silently allowed.
func TestBroadcastEmail_Deduplication_RepoErrorFailsClosed(t *testing.T) {
	os.Setenv("ADMIN_API_KEY", "test-admin-key")
	defer os.Unsetenv("ADMIN_API_KEY")

	sender := &mockEmailSender{failOnIdx: -1}
	broadcastRepo := &mockEmailBroadcastRepo{
		recentBroadcastErr: fmt.Errorf("database connection lost"),
	}
	userRepo := &mockUserEmailRepo{
		recipients: []models.EmailRecipient{
			{ID: "u1", Email: "alice@example.com", DisplayName: "Alice"},
		},
	}

	handler := NewAdminHandler(nil)
	handler.SetEmailSender(sender)
	handler.SetEmailBroadcastRepo(broadcastRepo)
	handler.SetUserEmailRepo(userRepo)

	body := `{"subject":"Newsletter","body_html":"<p>Hello</p>"}`
	req := httptest.NewRequest(http.MethodPost, "/admin/email/broadcast", strings.NewReader(body))
	req.Header.Set("X-Admin-API-Key", "test-admin-key")
	rr := httptest.NewRecorder()

	handler.BroadcastEmail(rr, req)

	// Should fail-closed: return 500, NOT send emails
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 on dedup check error, got %d: %s", rr.Code, rr.Body.String())
	}
	if len(sender.calls) != 0 {
		t.Errorf("expected 0 email sends on dedup error, got %d", len(sender.calls))
	}
}

// TestBroadcastEmail_Deduplication_SendingStatusBlocks tests that a broadcast still
// in "sending" status (not yet completed) also blocks retries.
func TestBroadcastEmail_Deduplication_SendingStatusBlocks(t *testing.T) {
	os.Setenv("ADMIN_API_KEY", "test-admin-key")
	defer os.Unsetenv("ADMIN_API_KEY")

	sender := &mockEmailSender{failOnIdx: -1}
	broadcastRepo := &mockEmailBroadcastRepo{
		recentBroadcast: &models.EmailBroadcast{
			ID:              "in-flight-broadcast",
			Subject:         "Newsletter",
			TotalRecipients: 315,
			SentCount:       150, // still sending, only 150 of 315 done
			Status:          "sending",
			StartedAt:       time.Now().Add(-45 * time.Second),
		},
	}
	userRepo := &mockUserEmailRepo{
		recipients: []models.EmailRecipient{
			{ID: "u1", Email: "alice@example.com", DisplayName: "Alice"},
		},
	}

	handler := NewAdminHandler(nil)
	handler.SetEmailSender(sender)
	handler.SetEmailBroadcastRepo(broadcastRepo)
	handler.SetUserEmailRepo(userRepo)

	body := `{"subject":"Newsletter","body_html":"<p>Hello</p>"}`
	req := httptest.NewRequest(http.MethodPost, "/admin/email/broadcast", strings.NewReader(body))
	req.Header.Set("X-Admin-API-Key", "test-admin-key")
	rr := httptest.NewRecorder()

	handler.BroadcastEmail(rr, req)

	// In-flight broadcast should block retry
	if rr.Code != http.StatusConflict {
		t.Errorf("expected 409 Conflict for in-flight broadcast, got %d: %s", rr.Code, rr.Body.String())
	}
	if len(sender.calls) != 0 {
		t.Errorf("expected 0 email sends, got %d", len(sender.calls))
	}
}

// TestBroadcastEmail_DryRun_SkipsDedup tests that dry-run mode does NOT trigger dedup,
// even when a recent broadcast with the same subject exists.
func TestBroadcastEmail_DryRun_SkipsDedup(t *testing.T) {
	os.Setenv("ADMIN_API_KEY", "test-admin-key")
	defer os.Unsetenv("ADMIN_API_KEY")

	sender := &mockEmailSender{failOnIdx: -1}
	broadcastRepo := &mockEmailBroadcastRepo{
		recentBroadcast: &models.EmailBroadcast{
			ID:              "prev-broadcast-id",
			Subject:         "Newsletter",
			TotalRecipients: 315,
			SentCount:       315,
			Status:          "completed",
			StartedAt:       time.Now().Add(-30 * time.Minute),
		},
	}
	userRepo := &mockUserEmailRepo{
		recipients: []models.EmailRecipient{
			{ID: "u1", Email: "alice@example.com", DisplayName: "Alice"},
		},
	}

	handler := NewAdminHandler(nil)
	handler.SetEmailSender(sender)
	handler.SetEmailBroadcastRepo(broadcastRepo)
	handler.SetUserEmailRepo(userRepo)

	body := `{"subject":"Newsletter","body_html":"<p>Hello</p>","dry_run":true}`
	req := httptest.NewRequest(http.MethodPost, "/admin/email/broadcast", strings.NewReader(body))
	req.Header.Set("X-Admin-API-Key", "test-admin-key")
	rr := httptest.NewRecorder()

	handler.BroadcastEmail(rr, req)

	// Dry-run should return 200 with preview, NOT 409
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200 OK for dry-run (should skip dedup), got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&resp)
	if _, ok := resp["would_send"]; !ok {
		t.Error("expected dry-run response with 'would_send' field")
	}
}

// TestBroadcastEmail_Deduplication_FullyFailedBroadcastAlsoBlocks tests that a broadcast
// where ALL emails failed (sent_count=0) still blocks retries.
func TestBroadcastEmail_Deduplication_FullyFailedBroadcastAlsoBlocks(t *testing.T) {
	os.Setenv("ADMIN_API_KEY", "test-admin-key")
	defer os.Unsetenv("ADMIN_API_KEY")

	sender := &mockEmailSender{failOnIdx: -1}
	broadcastRepo := &mockEmailBroadcastRepo{
		recentBroadcast: &models.EmailBroadcast{
			ID:              "failed-broadcast-id",
			Subject:         "Newsletter",
			TotalRecipients: 315,
			SentCount:       0,
			FailedCount:     315,
			Status:          "completed",
			StartedAt:       time.Now().Add(-5 * time.Minute),
		},
	}
	userRepo := &mockUserEmailRepo{
		recipients: []models.EmailRecipient{
			{ID: "u1", Email: "alice@example.com", DisplayName: "Alice"},
		},
	}

	handler := NewAdminHandler(nil)
	handler.SetEmailSender(sender)
	handler.SetEmailBroadcastRepo(broadcastRepo)
	handler.SetUserEmailRepo(userRepo)

	body := `{"subject":"Newsletter","body_html":"<p>Hello</p>"}`
	req := httptest.NewRequest(http.MethodPost, "/admin/email/broadcast", strings.NewReader(body))
	req.Header.Set("X-Admin-API-Key", "test-admin-key")
	rr := httptest.NewRecorder()

	handler.BroadcastEmail(rr, req)

	// Even fully-failed broadcasts should block (use force=true to override)
	if rr.Code != http.StatusConflict {
		t.Errorf("expected 409 for fully-failed broadcast dedup, got %d: %s", rr.Code, rr.Body.String())
	}
	if len(sender.calls) != 0 {
		t.Errorf("expected 0 email sends, got %d", len(sender.calls))
	}
}
