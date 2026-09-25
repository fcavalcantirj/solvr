package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// The public contract makes Idempotency-Key (creates), If-Match (conflicting edits) and
// Last-Event-ID (stream resume) request headers, and ETag / Idempotent-Replayed /
// Retry-After response headers. A browser only lets a cross-origin page send or read them
// when CORS says so, so the CORS policy must name every one of them (idx 74 steps 4-6).

func corsPreflight(t *testing.T, origin, method, requestHeaders string) *httptest.ResponseRecorder {
	t.Helper()
	t.Setenv("ALLOWED_ORIGINS", "")
	router := NewRouter(nil, nil, nil)
	req := httptest.NewRequest(http.MethodOptions, "/v1/rooms", nil)
	req.Header.Set("Origin", origin)
	req.Header.Set("Access-Control-Request-Method", method)
	req.Header.Set("Access-Control-Request-Headers", requestHeaders)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func TestCORSContract_PreflightAllowsTheContractRequestHeaders(t *testing.T) {
	cases := []struct{ method, header string }{
		{http.MethodPost, "Idempotency-Key"},
		{http.MethodPatch, "If-Match"},
		{http.MethodGet, "Last-Event-ID"},
		{http.MethodPatch, "Authorization, Content-Type, If-Match"},
		{http.MethodPost, "Authorization, Content-Type, Idempotency-Key, X-Request-ID"},
	}
	for _, tc := range cases {
		t.Run(tc.method+" "+tc.header, func(t *testing.T) {
			w := corsPreflight(t, "https://solvr.dev", tc.method, tc.header)
			if got := w.Header().Get("Access-Control-Allow-Origin"); got != "https://solvr.dev" {
				t.Fatalf("preflight naming %q was not allowed: Access-Control-Allow-Origin=%q", tc.header, got)
			}
			allowed := strings.ToLower(w.Header().Get("Access-Control-Allow-Headers"))
			for _, h := range strings.Split(tc.header, ",") {
				if !strings.Contains(allowed, strings.ToLower(strings.TrimSpace(h))) {
					t.Errorf("Access-Control-Allow-Headers = %q, want it to include %q", allowed, strings.TrimSpace(h))
				}
			}
		})
	}
}

func TestCORSContract_PreflightStillRefusesUnknownHeadersAndOrigins(t *testing.T) {
	w := corsPreflight(t, "https://solvr.dev", http.MethodPost, "X-Not-In-The-Contract")
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("an unlisted request header was allowed: Access-Control-Allow-Origin=%q", got)
	}
	w = corsPreflight(t, "https://malicious-site.com", http.MethodPost, "Idempotency-Key, If-Match")
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("an unlisted origin was allowed: Access-Control-Allow-Origin=%q", got)
	}
}

func TestCORSContract_ExposesTheContractResponseHeaders(t *testing.T) {
	t.Setenv("ALLOWED_ORIGINS", "")
	router := NewRouter(nil, nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.Header.Set("Origin", "https://solvr.dev")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	exposed := strings.ToLower(w.Header().Get("Access-Control-Expose-Headers"))
	for _, h := range []string{"ETag", "Idempotent-Replayed", "Retry-After", "X-Request-ID", "X-RateLimit-Limit"} {
		if !strings.Contains(exposed, strings.ToLower(h)) {
			t.Errorf("Access-Control-Expose-Headers = %q, want it to include %q", exposed, h)
		}
	}
}
