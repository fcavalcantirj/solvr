package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// POST /admin/email/notice (SPEC.md 16.6) sends one transactional email to one user: from
// FROM_EMAIL_TRANSACTIONAL, with no branded marketing footer and no List-Unsubscribe headers,
// recorded as kind transactional. Without that sender it refuses to send and logs why; it never
// falls back to the marketing sender or its footer.

// fakeMessageRecorder records the email_messages rows a handler writes.
type fakeMessageRecorder struct {
	err  error
	rows []models.EmailMessage
}

func (f *fakeMessageRecorder) RecordMessage(ctx context.Context, m *models.EmailMessage) error {
	f.rows = append(f.rows, *m)
	return f.err
}

// fakeNoticeRecipients answers FindNoticeRecipient from a fixed list.
type fakeNoticeRecipients struct {
	err        error
	users      map[string]models.EmailRecipient
	suppressed map[string]bool
}

func (f *fakeNoticeRecipients) FindNoticeRecipient(ctx context.Context, email string) (*models.EmailRecipient, bool, error) {
	if f.err != nil {
		return nil, false, f.err
	}
	u, ok := f.users[email]
	if !ok {
		return nil, false, nil
	}
	return &u, f.suppressed[email], nil
}

type noticeFixture struct {
	h          *AdminHandler
	marketing  *mockEmailSender
	notice     *mockEmailSender
	recorder   *fakeMessageRecorder
	recipients *fakeNoticeRecipients
	logs       *bytes.Buffer
}

func newNoticeFixture(t *testing.T, withNoticeSender bool) noticeFixture {
	t.Helper()
	t.Setenv("ADMIN_API_KEY", "test-admin-key")
	f := noticeFixture{
		h:         NewAdminHandler(nil),
		marketing: &mockEmailSender{failOnIdx: -1},
		recorder:  &fakeMessageRecorder{},
		recipients: &fakeNoticeRecipients{
			users: map[string]models.EmailRecipient{
				"alice@example.com": {ID: "u-alice", Email: "alice@example.com", DisplayName: "Alice"},
				"bob@example.com":   {ID: "u-bob", Email: "bob@example.com", DisplayName: "Bob"},
			},
			suppressed: map[string]bool{"bob@example.com": true},
		},
		logs: &bytes.Buffer{},
	}
	f.h.log = slog.New(slog.NewTextHandler(f.logs, nil))
	f.h.SetEmailSender(f.marketing)
	f.h.SetEmailMessageRecorder(f.recorder)
	f.h.SetNoticeRecipientRepo(f.recipients)
	if withNoticeSender {
		f.notice = &mockEmailSender{failOnIdx: -1}
		f.h.SetNoticeSender(f.notice)
	}
	return f
}

func (f noticeFixture) post(t *testing.T, body map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/admin/email/notice", bytes.NewReader(raw))
	req.Header.Set("X-Admin-API-Key", "test-admin-key")
	rr := httptest.NewRecorder()
	f.h.SendNotice(rr, req)
	return rr
}

func noticeBody() map[string]any {
	return map[string]any{
		"to":        "alice@example.com",
		"subject":   "Your account",
		"body_html": "<p>Hello Alice, a note about your account.</p>",
		"body_text": "Hello Alice, a note about your account.",
	}
}

func TestSendNotice_SendsFromTheTransactionalSenderWithNoFooterAndNoListUnsubscribe(t *testing.T) {
	f := newNoticeFixture(t, true)

	rr := f.post(t, noticeBody())

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rr.Code, rr.Body.String())
	}
	if len(f.marketing.calls) != 0 {
		t.Errorf("the marketing sender sent %d emails", len(f.marketing.calls))
	}
	if len(f.notice.calls) != 1 {
		t.Fatalf("the transactional sender sent %d emails, want 1", len(f.notice.calls))
	}
	call := f.notice.calls[0]
	if call.To != "alice@example.com" || call.Subject != "Your account" {
		t.Errorf("sent to %q subject %q", call.To, call.Subject)
	}
	if call.HTML != "<p>Hello Alice, a note about your account.</p>" {
		t.Errorf("the body was changed (a footer or a template added): %q", call.HTML)
	}
	if call.Text != "Hello Alice, a note about your account." {
		t.Errorf("text = %q", call.Text)
	}
	if len(call.Headers) != 0 {
		t.Errorf("headers = %v, want none (no List-Unsubscribe)", call.Headers)
	}
	if len(f.recorder.rows) != 1 {
		t.Fatalf("recorded %d messages, want 1", len(f.recorder.rows))
	}
	row := f.recorder.rows[0]
	want := models.EmailMessage{Kind: models.EmailKindTransactional, UserID: "u-alice", ProviderMessageID: "re-msg-1",
		Subject: "Your account", Status: models.EmailStatusSent}
	if row != want {
		t.Errorf("recorded %+v, want %+v", row, want)
	}
	var resp map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp["provider_message_id"] != "re-msg-1" || resp["kind"] != "transactional" {
		t.Errorf("response = %v", resp)
	}
}

// Fail closed: with no transactional sender the notice is refused with a logged error, and the
// marketing sender, which is configured, sends nothing.
func TestSendNotice_FailsClosedWithoutTheTransactionalSender(t *testing.T) {
	f := newNoticeFixture(t, false)

	for _, body := range []map[string]any{noticeBody(), func() map[string]any { b := noticeBody(); b["dry_run"] = true; return b }()} {
		rr := f.post(t, body)
		if rr.Code != http.StatusServiceUnavailable {
			t.Fatalf("status = %d, want 503: %s", rr.Code, rr.Body.String())
		}
		if code := decodeErrorObject(t, rr.Body.Bytes())["code"]; code != "TRANSACTIONAL_SENDER_NOT_CONFIGURED" {
			t.Errorf("code = %v, want TRANSACTIONAL_SENDER_NOT_CONFIGURED", code)
		}
	}
	if len(f.marketing.calls) != 0 {
		t.Errorf("fell back to the marketing sender: %d emails", len(f.marketing.calls))
	}
	if len(f.recorder.rows) != 0 {
		t.Errorf("recorded %d messages, want 0", len(f.recorder.rows))
	}
	if !strings.Contains(f.logs.String(), "level=ERROR") || !strings.Contains(f.logs.String(), "FROM_EMAIL_TRANSACTIONAL") {
		t.Errorf("no error naming FROM_EMAIL_TRANSACTIONAL was logged: %q", f.logs.String())
	}
}

func TestSendNotice_GoesToOneAddressOnly(t *testing.T) {
	f := newNoticeFixture(t, true)
	for _, to := range []string{"", "alice@example.com, bob@example.com", "alice@example.com;bob@example.com",
		"Alice <alice@example.com>", "not-an-address"} {
		b := noticeBody()
		b["to"] = to
		if rr := f.post(t, b); rr.Code != http.StatusBadRequest {
			t.Errorf("to %q: status = %d, want 400", to, rr.Code)
		}
	}
	if len(f.notice.calls) != 0 {
		t.Errorf("sent %d emails", len(f.notice.calls))
	}
}

func TestSendNotice_RequiresASubjectAndABody(t *testing.T) {
	f := newNoticeFixture(t, true)
	for _, field := range []string{"subject", "body_html"} {
		b := noticeBody()
		b[field] = "  "
		if rr := f.post(t, b); rr.Code != http.StatusBadRequest {
			t.Errorf("blank %s: status = %d, want 400", field, rr.Code)
		}
	}
}

func TestSendNotice_RefusesAnUnknownOrSuppressedRecipient(t *testing.T) {
	f := newNoticeFixture(t, true)

	b := noticeBody()
	b["to"] = "nobody@example.com"
	if rr := f.post(t, b); rr.Code != http.StatusNotFound {
		t.Errorf("unknown: status = %d, want 404", rr.Code)
	}
	b["to"] = "bob@example.com"
	rr := f.post(t, b)
	if rr.Code != http.StatusConflict {
		t.Errorf("suppressed: status = %d, want 409", rr.Code)
	} else if code := decodeErrorObject(t, rr.Body.Bytes())["code"]; code != "RECIPIENT_SUPPRESSED" {
		t.Errorf("suppressed: code = %v", code)
	}
	if len(f.notice.calls) != 0 {
		t.Errorf("sent %d emails", len(f.notice.calls))
	}
}

func TestSendNotice_DryRunSendsNothing(t *testing.T) {
	f := newNoticeFixture(t, true)
	b := noticeBody()
	b["dry_run"] = true

	rr := f.post(t, b)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rr.Code, rr.Body.String())
	}
	if len(f.notice.calls) != 0 || len(f.marketing.calls) != 0 || len(f.recorder.rows) != 0 {
		t.Errorf("a dry run sent or recorded: %d, %d, %d", len(f.notice.calls), len(f.marketing.calls), len(f.recorder.rows))
	}
	var resp struct {
		Kind            string `json:"kind"`
		RecipientID     string `json:"recipient_id"`
		WouldSend       int    `json:"would_send"`
		Footer          bool   `json:"footer"`
		ListUnsubscribe bool   `json:"list_unsubscribe"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.WouldSend != 1 || resp.Kind != "transactional" || resp.RecipientID != "u-alice" || resp.Footer || resp.ListUnsubscribe {
		t.Errorf("dry run = %+v", resp)
	}
}

// A failed send is recorded as failed, with no provider id, and logged with the recipient's user
// id, never the address.
func TestSendNotice_RecordsAndLogsAFailedSend(t *testing.T) {
	f := newNoticeFixture(t, true)
	f.notice.failOnIdx = 0

	rr := f.post(t, noticeBody())

	if rr.Code != http.StatusBadGateway {
		t.Errorf("status = %d, want 502", rr.Code)
	}
	if len(f.recorder.rows) != 1 || f.recorder.rows[0].Status != models.EmailStatusFailed || f.recorder.rows[0].ProviderMessageID != "" {
		t.Errorf("recorded %+v, want one failed row without a provider id", f.recorder.rows)
	}
	logs := f.logs.String()
	if !strings.Contains(logs, "u-alice") || strings.Contains(logs, "alice@example.com") {
		t.Errorf("logs = %q, want the user id and not the address", logs)
	}
}

func TestSendNotice_ALookupErrorSendsNothing(t *testing.T) {
	f := newNoticeFixture(t, true)
	f.recipients.err = errors.New("db down")

	if rr := f.post(t, noticeBody()); rr.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rr.Code)
	}
	if len(f.notice.calls) != 0 {
		t.Errorf("sent %d emails", len(f.notice.calls))
	}
}

func TestSendNotice_RequiresTheOperatorKey(t *testing.T) {
	f := newNoticeFixture(t, true)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/admin/email/notice", strings.NewReader(`{}`))
	rr := httptest.NewRecorder()
	f.h.SendNotice(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rr.Code)
	}
}
