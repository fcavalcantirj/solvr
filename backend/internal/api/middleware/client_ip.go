package middleware

import (
	"net"
	"net/http"
	"net/netip"
	"strings"
)

// CFConnectingIPHeader is the header Cloudflare sets, overwriting any value the caller sent,
// to the address of the client that connected to its edge.
const CFConnectingIPHeader = "CF-Connecting-IP"

// ClientIP is the one client address the whole API uses: rate limits, logs and analytics.
// It is CF-Connecting-IP when that header holds a valid IP, and otherwise the host of the
// connection's own address. X-Forwarded-For, X-Real-IP and True-Client-IP are never read:
// any caller can set them, and Cloudflare passes them through untouched.
func ClientIP(r *http.Request) string {
	if addr, err := netip.ParseAddr(strings.TrimSpace(r.Header.Get(CFConnectingIPHeader))); err == nil {
		return addr.Unmap().String()
	}
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	// No port: an address already reduced to its host (an in-process call copies it).
	return strings.Trim(r.RemoteAddr, "[]")
}

// RealClientIP sets r.RemoteAddr to ClientIP(r), so every later reader of RemoteAddr
// (the per-IP limiters, the request log, search analytics) sees the same client address.
func RealClientIP(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ip := ClientIP(r); ip != "" {
			r.RemoteAddr = ip
		}
		next.ServeHTTP(w, r)
	})
}
