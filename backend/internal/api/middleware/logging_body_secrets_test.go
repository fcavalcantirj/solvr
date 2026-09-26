package middleware

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// idx 75 step 5: the body of a failed request is logged, so redaction must not depend on the
// client sending a well-formed object with a field named exactly `password` or `token`.
// A malformed body is the case that fails validation most often, and the one a naive
// redactor logs whole.
func TestRedactRequestBody_NeverLogsASecret(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		secret  string
		visible string // something harmless that must survive, so the log stays useful
	}{
		{"new password", `{"current_password":"cp-secret-1","new_password":"np-secret-1","title":"visible"}`, "cp-secret-1", "visible"},
		{"room token from the retired handshake body", `{"room_token":"rt-secret-2","ttl_seconds":0}`, "rt-secret-2", "ttl_seconds"},
		{"claim token", `{"claim_token":"ct-secret-3","name":"visible"}`, "ct-secret-3", "visible"},
		{"stream ticket", `{"ticket":"solvr_st_secret4","name":"visible"}`, "solvr_st_secret4", "visible"},
		{"bare key", `{"key":"k-secret-5","name":"visible"}`, "k-secret-5", "visible"},
		{"client secret and mixed case", `{"Client_Secret":"cs-secret-6","name":"visible"}`, "cs-secret-6", "visible"},
		{"api key spelled with a dash", `{"API-Key":"ak-secret-7","name":"visible"}`, "ak-secret-7", "visible"},
		{"array of objects at the top", `[{"password":"arr-secret-8","name":"visible"}]`, "arr-secret-8", "visible"},
		{"array inside an array", `{"items":[[{"token":"deep-secret-9"}]],"name":"visible"}`, "deep-secret-9", "visible"},
		{"malformed JSON: missing the closing brace", `{"email":"a@b.c","password":"mal-secret-10"`, "mal-secret-10", "email"},
		{"malformed JSON: cut off inside the value", `{"email":"a@b.c","password":"cut-secret-11`, "cut-secret-11", "email"},
		{"malformed JSON: a trailing comma", `{"email":"a@b.c","token":"comma-secret-12",}`, "comma-secret-12", "email"},
		{"malformed JSON: single quotes", `{'email':'a@b.c','password':'quote-secret-13'}`, "quote-secret-13", "email"},
		{"form encoded", `username=visible&password=form-secret-14&x=1`, "form-secret-14", "username"},
		{"escaped quote inside the value", `{"password":"esc\"aped-secret-15","name":"visible"`, "aped-secret-15", "name"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := RedactRequestBody(tc.input)
			if strings.Contains(got, tc.secret) {
				t.Errorf("the secret %q reached the log: %s", tc.secret, got)
			}
			if !strings.Contains(got, tc.visible) {
				t.Errorf("redaction must keep the harmless %q so the log stays useful: %s", tc.visible, got)
			}
			if !strings.Contains(got, "***REDACTED***") {
				t.Errorf("expected a redaction marker: %s", got)
			}
		})
	}

	// Text with nothing sensitive in it is left exactly as it was (not JSON, or empty).
	for _, plain := range []string{strings.Repeat("x", 300), `not json at all`, `monkey=banana&keywords=go`, ``} {
		if got := RedactRequestBody(plain); got != plain {
			t.Errorf("text with no secret must be unchanged: %q became %q", plain, got)
		}
	}
	// An object with no secret keeps every field.
	if got := RedactRequestBody(`{"title":"Test","description":"Hello","keywords":["a"]}`); strings.Contains(got, "REDACTED") ||
		!strings.Contains(got, `"title":"Test"`) || !strings.Contains(got, `"keywords":["a"]`) {
		t.Errorf("an object with no secret keeps its fields: %s", got)
	}
}

func TestLogging_AFailedRequestWithAMalformedBodyNeverLogsItsSecret(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)

	handler := Logging(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"code":"VALIDATION_ERROR","message":"invalid request body"}}`))
	}))
	req := httptest.NewRequest(http.MethodPost, "/v1/auth/login", bytes.NewBufferString(`{"email":"a@b.c","password":"hunter2-malformed"`))
	req.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(httptest.NewRecorder(), req)

	start := bytes.IndexByte(buf.Bytes(), '{')
	if start == -1 {
		t.Fatal("expected a JSON log line")
	}
	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes()[start:], &entry); err != nil {
		t.Fatalf("parse log line: %v", err)
	}
	body, _ := entry["request_body"].(string)
	if body == "" {
		t.Fatal("the failed request's body is still logged (redacted), so the log stays useful")
	}
	if strings.Contains(buf.String(), "hunter2-malformed") {
		t.Errorf("the password of a malformed login body reached the log line: %s", buf.String())
	}
}
