package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientIP_IgnoresUnverifiedHeaders(t *testing.T) {
	for _, remote := range []string{"203.0.113.8:4321", "203.0.113.8", "[::ffff:203.0.113.8]:4321"} {
		r := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", http.NoBody)
		r.RemoteAddr = remote
		for _, header := range []string{"CF-Connecting-IP", "X-Real-IP", "X-Forwarded-For", "True-Client-IP"} {
			r.Header.Set(header, "198.51.100.9")
		}
		if got := ClientIP(r); got != "203.0.113.8" {
			t.Errorf("ClientIP(%q) = %q", remote, got)
		}
	}
}

func TestClientIPPolicy(t *testing.T) {
	policy, err := NewClientIPMiddleware("10.20.0.0/24, fd00:1234::/64")
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct{ name, peer, upstream, cf, want string }{
		{"direct caller cannot forge either header", "203.0.113.8:1234", "172.67.190.162", "198.51.100.9", "203.0.113.8"},
		{"unconfigured private proxy is untrusted", "10.21.0.2:1234", "172.67.190.162", "198.51.100.9", "10.21.0.2"},
		{"trusted proxy identifies direct visitor", "10.20.0.2:1234", "203.0.113.8", "", "203.0.113.8"},
		{"direct visitor cannot forge CF header", "10.20.0.2:1234", "203.0.113.8", "198.51.100.9", "203.0.113.8"},
		{"verified Cloudflare ingress", "10.20.0.2:1234", "172.67.190.162", "198.51.100.9", "198.51.100.9"},
		{"Cloudflare IPv6 ingress and visitor", "[fd00:1234::2]:1234", "2606:4700:3034::ac43:bea2", "2001:db8::9", "2001:db8::9"},
		{"mapped IPv4 addresses", "[::ffff:10.20.0.2]:1234", "::ffff:172.67.190.162", "::ffff:198.51.100.9", "198.51.100.9"},
		{"missing upstream ignores CF header", "10.20.0.2:1234", "", "198.51.100.9", "10.20.0.2"},
		{"invalid upstream ignores CF header", "10.20.0.2:1234", "not-an-ip", "198.51.100.9", "10.20.0.2"},
		{"upstream list refused", "10.20.0.2:1234", "172.67.190.162, 203.0.113.8", "198.51.100.9", "10.20.0.2"},
		{"upstream zone refused", "10.20.0.2:1234", "fe80::1%eth0", "198.51.100.9", "10.20.0.2"},
		{"invalid CF header falls back to verified upstream", "10.20.0.2:1234", "172.67.190.162", "invalid", "172.67.190.162"},
		{"CF list refused", "10.20.0.2:1234", "172.67.190.162", "198.51.100.9, 198.51.100.10", "172.67.190.162"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", http.NoBody)
			r.RemoteAddr = tc.peer
			r.Header.Set("X-Real-IP", tc.upstream)
			r.Header.Set("CF-Connecting-IP", tc.cf)
			r.Header.Set("X-Forwarded-For", "192.0.2.99")
			r.Header.Set("True-Client-IP", "192.0.2.98")
			policy(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				if got := ClientIP(r); got != tc.want || r.RemoteAddr != tc.want {
					t.Errorf("ClientIP = %q, RemoteAddr = %q; want %q", got, r.RemoteAddr, tc.want)
				}
			})).ServeHTTP(httptest.NewRecorder(), r)
		})
	}
}

func TestClientIPPolicy_ConfigAndDuplicateHeaders(t *testing.T) {
	for _, config := range []string{"not-a-cidr", "10.0.0.1", "10.0.0.0/8,", "0.0.0.0/0", "::/0"} {
		if _, err := NewClientIPMiddleware(config); err == nil {
			t.Errorf("accepted unsafe or invalid TRUSTED_PROXY_CIDRS %q", config)
		}
	}
	policy, err := NewClientIPMiddleware("10.20.0.0/24")
	if err != nil {
		t.Fatal(err)
	}
	for _, header := range []string{"X-Real-IP", "CF-Connecting-IP"} {
		r := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", http.NoBody)
		r.RemoteAddr = "10.20.0.2:1234"
		r.Header.Set("X-Real-IP", "172.67.190.162")
		r.Header.Set("CF-Connecting-IP", "198.51.100.9")
		r.Header.Add(header, "192.0.2.99")
		policy(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			want := "172.67.190.162"
			if header == "X-Real-IP" {
				want = "10.20.0.2"
			}
			if ClientIP(r) != want {
				t.Errorf("duplicate %s trusted: %q", header, ClientIP(r))
			}
		})).ServeHTTP(httptest.NewRecorder(), r)
	}
}

func TestRealClientIP_DefaultTrustsNoProxy(t *testing.T) {
	r := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", http.NoBody)
	r.RemoteAddr = "[2001:db8::8]:1234"
	r.Header.Set("CF-Connecting-IP", "203.0.113.99")
	RealClientIP(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		if got := ClientIP(r); got != "2001:db8::8" {
			t.Errorf("unverified header trusted: %q", got)
		}
	})).ServeHTTP(httptest.NewRecorder(), r)
}
