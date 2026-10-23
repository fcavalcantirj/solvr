package middleware

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"testing"
)

// Traefik's reverse proxy appends its actual peer even when its entrypoint
// preserves caller-supplied headers. Exercise that wire behavior, not a fixture
// that assumes X-Real-IP was sanitized by the entrypoint.
func TestClientIPPolicy_ProxyPreservingForgedHeaders(t *testing.T) {
	policy, err := NewClientIPMiddleware("127.0.0.0/8")
	if err != nil {
		t.Fatal(err)
	}
	backend := httptest.NewServer(policy(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, writeErr := io.WriteString(w, ClientIP(r)); writeErr != nil {
			t.Errorf("write client address: %v", writeErr)
		}
	})))
	defer backend.Close()
	target, err := url.Parse(backend.URL)
	if err != nil {
		t.Fatal(err)
	}
	proxy := httptest.NewServer(httputil.NewSingleHostReverseProxy(target))
	defer proxy.Close()
	for _, forged := range []string{"198.51.100.24", "172.67.190.162", "bad, 172.67.190.162"} {
		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, proxy.URL, http.NoBody)
		req.RequestURI = ""
		req.Header.Set("X-Real-IP", forged)
		req.Header.Add("X-Forwarded-For", "172.67.190.162")
		req.Header.Add("X-Forwarded-For", "198.51.100.25")
		req.Header.Set("CF-Connecting-IP", "203.0.113.18")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(resp.Body)
		if closeErr := resp.Body.Close(); closeErr != nil {
			t.Fatal(closeErr)
		}
		if err != nil {
			t.Fatal(err)
		}
		if string(body) != "127.0.0.1" {
			t.Errorf("forged %q identified visitor as %q", forged, body)
		}
	}
}
