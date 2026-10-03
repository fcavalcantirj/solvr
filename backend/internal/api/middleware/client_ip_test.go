package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// ClientIP trusts only Cloudflare's CF-Connecting-IP (when it is a valid IP) and otherwise
// the connection's own address. The forwarding headers a caller can set are ignored.
func TestClientIP(t *testing.T) {
	tests := []struct {
		name       string
		remoteAddr string
		headers    map[string]string
		want       string
	}{
		{name: "remote addr with port", remoteAddr: "192.168.1.1:12345", want: "192.168.1.1"},
		{name: "remote addr without port", remoteAddr: "192.168.1.1", want: "192.168.1.1"},
		{name: "bracketed ipv6 remote addr", remoteAddr: "[2001:db8::1]:12345", want: "2001:db8::1"},
		{name: "spoofed x-forwarded-for is ignored", remoteAddr: "10.0.0.1:12345",
			headers: map[string]string{"X-Forwarded-For": "203.0.113.1, 10.0.0.2"}, want: "10.0.0.1"},
		{name: "spoofed x-real-ip is ignored", remoteAddr: "10.0.0.1:12345",
			headers: map[string]string{"X-Real-IP": "203.0.113.5"}, want: "10.0.0.1"},
		{name: "spoofed true-client-ip is ignored", remoteAddr: "10.0.0.1:12345",
			headers: map[string]string{"True-Client-IP": "203.0.113.99"}, want: "10.0.0.1"},
		{name: "cf-connecting-ip is honoured", remoteAddr: "10.0.0.1:12345",
			headers: map[string]string{"CF-Connecting-IP": "203.0.113.7"}, want: "203.0.113.7"},
		{name: "cf-connecting-ip ipv6 is honoured", remoteAddr: "10.0.0.1:12345",
			headers: map[string]string{"CF-Connecting-IP": "2001:db8::7"}, want: "2001:db8::7"},
		{name: "cf-connecting-ip wins over every spoofed header", remoteAddr: "10.0.0.1:12345",
			headers: map[string]string{"CF-Connecting-IP": "203.0.113.7", "X-Forwarded-For": "198.51.100.1",
				"X-Real-IP": "198.51.100.2", "True-Client-IP": "198.51.100.3"}, want: "203.0.113.7"},
		{name: "invalid cf-connecting-ip falls back", remoteAddr: "10.0.0.1:12345",
			headers: map[string]string{"CF-Connecting-IP": "not-an-ip"}, want: "10.0.0.1"},
		{name: "a list in cf-connecting-ip falls back", remoteAddr: "10.0.0.1:12345",
			headers: map[string]string{"CF-Connecting-IP": "1.2.3.4, 5.6.7.8"}, want: "10.0.0.1"},
		{name: "empty cf-connecting-ip falls back", remoteAddr: "10.0.0.1:12345",
			headers: map[string]string{"CF-Connecting-IP": ""}, want: "10.0.0.1"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = tc.remoteAddr
			for k, v := range tc.headers {
				req.Header.Set(k, v)
			}
			if got := ClientIP(req); got != tc.want {
				t.Errorf("ClientIP = %q, want %q", got, tc.want)
			}
		})
	}
}

// RealClientIP rewrites RemoteAddr so every later reader (httprate's KeyByIP, the request
// log, search analytics) sees ClientIP.
func TestRealClientIP_RewritesRemoteAddr(t *testing.T) {
	var seen string
	h := RealClientIP(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { seen = r.RemoteAddr }))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.1:12345"
	req.Header.Set("True-Client-IP", "203.0.113.99")
	h.ServeHTTP(httptest.NewRecorder(), req)
	if seen != "10.0.0.1" {
		t.Errorf("with a spoofed True-Client-IP, RemoteAddr = %q, want 10.0.0.1", seen)
	}

	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.1:12345"
	req.Header.Set("CF-Connecting-IP", "203.0.113.99")
	h.ServeHTTP(httptest.NewRecorder(), req)
	if seen != "203.0.113.99" {
		t.Errorf("with CF-Connecting-IP, RemoteAddr = %q, want 203.0.113.99", seen)
	}
}
