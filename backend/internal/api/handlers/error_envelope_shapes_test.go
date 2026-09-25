package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// decodeErrorObject asserts the standard envelope {"error":{"code","message",...}}
// and returns the inner error object. idx 73 step 1: every error carries a stable
// code and a human-readable message inside an "error" OBJECT, never a bare string.
func decodeErrorObject(t *testing.T, body []byte) map[string]interface{} {
	t.Helper()
	var resp map[string]interface{}
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("body is not JSON: %v (%s)", err, body)
	}
	errObj, ok := resp["error"].(map[string]interface{})
	if !ok {
		t.Fatalf(`"error" is %T, want object envelope; body=%s`, resp["error"], body)
	}
	if _, top := resp["message"]; top {
		t.Errorf("message must live inside error, not top-level; body=%s", body)
	}
	if msg, _ := errObj["message"].(string); msg == "" {
		t.Errorf("error.message empty; body=%s", body)
	}
	return errObj
}

func TestUnsubscribe_ErrorsUseEnvelope(t *testing.T) {
	email := "alice@example.com"
	valid := GenerateUnsubscribeToken(email, "k")
	cases := []struct {
		name   string
		repo   *mockUnsubscribeRepo
		query  string
		status int
		code   string
	}{
		{"missing params", &mockUnsubscribeRepo{}, "", http.StatusBadRequest, "MISSING_PARAMS"},
		{"invalid token", &mockUnsubscribeRepo{}, "?email=" + email + "&token=bad", http.StatusForbidden, "INVALID_TOKEN"},
		{"repo error", &mockUnsubscribeRepo{err: fmt.Errorf("db down")}, "?email=" + email + "&token=" + valid, http.StatusInternalServerError, "INTERNAL_ERROR"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := NewUnsubscribeHandler(tc.repo, "k")
			rr := httptest.NewRecorder()
			h.Unsubscribe(rr, httptest.NewRequest(http.MethodGet, "/v1/email/unsubscribe"+tc.query, nil))
			if rr.Code != tc.status {
				t.Fatalf("status = %d, want %d", rr.Code, tc.status)
			}
			errObj := decodeErrorObject(t, rr.Body.Bytes())
			if errObj["code"] != tc.code {
				t.Errorf("error.code = %v, want %s", errObj["code"], tc.code)
			}
			if bytes.Contains(rr.Body.Bytes(), []byte("db down")) {
				t.Errorf("internal error text leaked: %s", rr.Body.String())
			}
		})
	}
}

func TestBroadcastEmail_NotConfiguredUsesEnvelope(t *testing.T) {
	os.Setenv("ADMIN_API_KEY", "test-admin-key")
	defer os.Unsetenv("ADMIN_API_KEY")

	h := NewAdminHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/admin/email/broadcast",
		bytes.NewBufferString(`{"subject":"S","body_html":"<p>x</p>"}`))
	req.Header.Set("X-Admin-API-Key", "test-admin-key")
	rr := httptest.NewRecorder()
	h.BroadcastEmail(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rr.Code)
	}
	if errObj := decodeErrorObject(t, rr.Body.Bytes()); errObj["code"] != "EMAIL_NOT_CONFIGURED" {
		t.Errorf("error.code = %v, want EMAIL_NOT_CONFIGURED", errObj["code"])
	}
}

func TestBroadcastEmail_DuplicateUsesEnvelopeWithDetails(t *testing.T) {
	os.Setenv("ADMIN_API_KEY", "test-admin-key")
	defer os.Unsetenv("ADMIN_API_KEY")

	h := NewAdminHandler(nil)
	h.SetEmailSender(&mockEmailSender{failOnIdx: -1})
	h.SetEmailBroadcastRepo(&mockEmailBroadcastRepo{recentBroadcast: &models.EmailBroadcast{
		ID: "prev-id", Subject: "N", TotalRecipients: 3, SentCount: 3,
		Status: "completed", StartedAt: time.Now().Add(-time.Hour),
	}})
	h.SetUserEmailRepo(&mockUserEmailRepo{recipients: []models.EmailRecipient{{ID: "u1", Email: "a@b.c"}}})

	req := httptest.NewRequest(http.MethodPost, "/admin/email/broadcast",
		bytes.NewBufferString(`{"subject":"N","body_html":"<p>x</p>"}`))
	req.Header.Set("X-Admin-API-Key", "test-admin-key")
	rr := httptest.NewRecorder()
	h.BroadcastEmail(rr, req)

	if rr.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", rr.Code)
	}
	errObj := decodeErrorObject(t, rr.Body.Bytes())
	if errObj["code"] != "DUPLICATE_BROADCAST" {
		t.Errorf("error.code = %v, want DUPLICATE_BROADCAST", errObj["code"])
	}
	details, ok := errObj["details"].(map[string]interface{})
	if !ok {
		t.Fatalf("error.details = %T, want object; body=%s", errObj["details"], rr.Body.String())
	}
	if details["previous_broadcast"] != "prev-id" || details["previous_status"] != "completed" {
		t.Errorf("details = %v, want previous_broadcast=prev-id previous_status=completed", details)
	}
	if details["previous_sent"] != float64(3) || details["previous_started"] == nil {
		t.Errorf("details = %v, want previous_sent=3 and previous_started", details)
	}
}
