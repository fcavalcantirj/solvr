package handlers

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

// requireCallbackErrorRedirect asserts that a browser OAuth callback ended on the frontend's
// /auth/callback page with ?error=code, carrying no login code and no token (every GitHub and
// Google callback error redirects; Felipe, 2026-09-30).
func requireCallbackErrorRedirect(t *testing.T, rec *httptest.ResponseRecorder, code string) {
	t.Helper()
	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d, want 302 to /auth/callback?error=%s; body: %s", rec.Code, code, rec.Body.String())
	}
	loc, err := url.Parse(rec.Header().Get("Location"))
	if err != nil {
		t.Fatalf("Location %q: %v", rec.Header().Get("Location"), err)
	}
	if loc.Path != "/auth/callback" || loc.Query().Get("error") != code {
		t.Errorf("Location = %q, want /auth/callback?error=%s", loc.String(), code)
	}
	if loc.Query().Has("code") || loc.Query().Has("token") {
		t.Errorf("an error redirect must carry no login code or token: %q", loc.String())
	}
}
