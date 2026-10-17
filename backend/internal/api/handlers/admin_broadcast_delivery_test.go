package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// A broadcast's links and records (SPEC.md 16.6): the footer links the solvr.dev unsubscribe page
// with the address encoded, List-Unsubscribe is the one-click API URL (RFC 8058) and nothing else,
// and every send is recorded with the provider's id, a failure with the recipient's user id.

const broadcastTestJWTSecret = "broadcast-test-jwt-secret"

type broadcastFixture struct {
	h        *AdminHandler
	sender   *mockEmailSender
	recorder *fakeMessageRecorder
	logs     *bytes.Buffer
}

func newBroadcastFixture(t *testing.T, recipients ...models.EmailRecipient) broadcastFixture {
	t.Helper()
	t.Setenv("ADMIN_API_KEY", "test-admin-key")
	t.Setenv("JWT_SECRET", broadcastTestJWTSecret)
	f := broadcastFixture{
		h:        NewAdminHandler(nil),
		sender:   &mockEmailSender{failOnIdx: -1},
		recorder: &fakeMessageRecorder{},
		logs:     &bytes.Buffer{},
	}
	f.h.log = slog.New(slog.NewTextHandler(f.logs, nil))
	f.h.SetEmailSender(f.sender)
	f.h.SetEmailBroadcastRepo(&mockEmailBroadcastRepo{})
	f.h.SetUserEmailRepo(&mockUserEmailRepo{recipients: recipients})
	f.h.SetEmailMessageRecorder(f.recorder)
	return f
}

func (f broadcastFixture) post(t *testing.T, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/admin/email/broadcast", strings.NewReader(body))
	req.Header.Set("X-Admin-API-Key", "test-admin-key")
	rr := httptest.NewRecorder()
	f.h.BroadcastEmail(rr, req)
	return rr
}

func TestBroadcastEmail_FooterLinksTheSolvrPageWithTheAddressEncoded(t *testing.T) {
	f := newBroadcastFixture(t, models.EmailRecipient{ID: "u1", Email: "alice+tag@example.com", DisplayName: "Alice"})

	if rr := f.post(t, `{"subject":"Newsletter","body_html":"<p>Hi</p>"}`); rr.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rr.Code, rr.Body.String())
	}
	html := f.sender.calls[0].HTML
	if !strings.Contains(html, `href="https://solvr.dev/email/unsubscribe?email=alice%2Btag%40example.com&`) {
		t.Errorf("the footer does not link the solvr.dev page with the address encoded:\n%s", html)
	}
	if strings.Contains(html, "api.solvr.dev/v1/email/unsubscribe") {
		t.Error("the footer still links the raw API")
	}
	token := GenerateUnsubscribeToken("alice+tag@example.com", broadcastTestJWTSecret)
	if !strings.Contains(html, "token="+token) {
		t.Error("the footer link does not carry the address's token")
	}
}

// F4 is open (does unsubscribe@solvr.dev exist?), so the mailto: entry is not sent: PLAN-r1 keeps
// it only if the owner confirms the mailbox.
func TestBroadcastEmail_ListUnsubscribeIsTheOneClickURLOnly(t *testing.T) {
	f := newBroadcastFixture(t, models.EmailRecipient{ID: "u1", Email: "alice+tag@example.com", DisplayName: "Alice"})

	if rr := f.post(t, `{"subject":"Newsletter","body_html":"<p>Hi</p>"}`); rr.Code != http.StatusOK {
		t.Fatalf("status = %d", rr.Code)
	}
	headers := f.sender.calls[0].Headers
	token := GenerateUnsubscribeToken("alice+tag@example.com", broadcastTestJWTSecret)
	if want := "<https://api.solvr.dev/v1/email/unsubscribe?email=alice%2Btag%40example.com&token=" + token + ">"; headers["List-Unsubscribe"] != want {
		t.Errorf("List-Unsubscribe = %q, want %q", headers["List-Unsubscribe"], want)
	}
	if headers["List-Unsubscribe-Post"] != "List-Unsubscribe=One-Click" {
		t.Errorf("List-Unsubscribe-Post = %q", headers["List-Unsubscribe-Post"])
	}
}

func TestBroadcastEmail_StoresEachSendWithItsProviderID(t *testing.T) {
	f := newBroadcastFixture(t,
		models.EmailRecipient{ID: "u1", Email: "alice@example.com", DisplayName: "Alice"},
		models.EmailRecipient{ID: "u2", Email: "bob@example.com", DisplayName: "Bob"},
	)
	f.sender.failOnIdx = 1

	rr := f.post(t, `{"subject":"Newsletter","body_html":"<p>Hi</p>"}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d", rr.Code)
	}
	want := []models.EmailMessage{
		{Kind: models.EmailKindBroadcast, BroadcastID: "test-broadcast-id", UserID: "u1", ProviderMessageID: "re-msg-1",
			Subject: "Newsletter", Status: models.EmailStatusSent},
		{Kind: models.EmailKindBroadcast, BroadcastID: "test-broadcast-id", UserID: "u2",
			Subject: "Newsletter", Status: models.EmailStatusFailed},
	}
	if len(f.recorder.rows) != len(want) {
		t.Fatalf("recorded %d messages, want %d: %+v", len(f.recorder.rows), len(want), f.recorder.rows)
	}
	for i := range want {
		if f.recorder.rows[i] != want[i] {
			t.Errorf("row %d = %+v, want %+v", i, f.recorder.rows[i], want[i])
		}
	}
	logs := f.logs.String()
	if !strings.Contains(logs, "level=ERROR") || !strings.Contains(logs, "user_id=u2") {
		t.Errorf("the failure was not logged with the recipient's user id: %q", logs)
	}
	if strings.Contains(logs, "bob@example.com") {
		t.Errorf("the log names the address: %q", logs)
	}
}

func TestBroadcastEmail_DryRunPreviewNamesTheFooterURL(t *testing.T) {
	f := newBroadcastFixture(t, models.EmailRecipient{ID: "u1", Email: "alice+tag@example.com", DisplayName: "Alice"})

	rr := f.post(t, `{"subject":"Newsletter","body_html":"<p>Hi</p>","dry_run":true}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d", rr.Code)
	}
	var resp struct {
		Preview map[string]string `json:"preview"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if got := resp.Preview["footer_url"]; !strings.HasPrefix(got, "https://solvr.dev/email/unsubscribe?email=alice%2Btag%40example.com&") {
		t.Errorf("preview.footer_url = %q", got)
	}
	if len(f.sender.calls) != 0 || len(f.recorder.rows) != 0 {
		t.Error("a dry run sent or recorded")
	}
}
