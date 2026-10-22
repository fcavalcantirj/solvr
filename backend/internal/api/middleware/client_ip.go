package middleware

import (
	"errors"
	"net"
	"net/http"
	"net/netip"
	"strings"
)

// CFConnectingIPHeader is meaningful only after the Cloudflare ingress is verified.
const CFConnectingIPHeader = "CF-Connecting-IP"

// ClientIP returns the address normalized by RealClientIP at the router boundary.
// Reading headers here would allow later consumers to undo that trust decision.
func ClientIP(r *http.Request) string {
	host := r.RemoteAddr
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	host = strings.Trim(host, "[]")
	if addr, err := netip.ParseAddr(host); err == nil {
		return addr.Unmap().String()
	}
	return host
}

// NewClientIPMiddleware trusts only explicitly configured reverse proxies. Traefik
// must retain its default forwarded-header sanitization: X-Real-IP is overwritten
// with its actual peer, not accepted from public callers. Behind that boundary,
// CF-Connecting-IP is accepted only for a verified Cloudflare peer.
func NewClientIPMiddleware(cidrs string) (func(http.Handler) http.Handler, error) {
	var trusted []netip.Prefix
	if strings.TrimSpace(cidrs) != "" {
		for _, raw := range strings.Split(cidrs, ",") {
			prefix, err := netip.ParsePrefix(strings.TrimSpace(raw))
			if err != nil || prefix.Bits() == 0 || prefix.Addr().Is4In6() {
				return nil, errors.New("TRUSTED_PROXY_CIDRS must contain explicit, non-universal CIDR ranges")
			}
			trusted = append(trusted, prefix.Masked())
		}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			peer, err := netip.ParseAddr(ClientIP(r))
			if err == nil {
				client := peer
				if inPrefixes(peer, trusted) {
					if upstream, ok := singleHeaderIP(r, "X-Real-IP"); ok {
						client = upstream
						if inPrefixes(upstream, cloudflarePrefixes) {
							if visitor, ok := singleHeaderIP(r, CFConnectingIPHeader); ok {
								client = visitor
							}
						}
					}
				}
				r.RemoteAddr = client.Unmap().String()
			}
			next.ServeHTTP(w, r)
		})
	}, nil
}

func singleHeaderIP(r *http.Request, name string) (netip.Addr, bool) {
	values := r.Header.Values(name)
	if len(values) != 1 {
		return netip.Addr{}, false
	}
	addr, err := netip.ParseAddr(strings.TrimSpace(values[0]))
	if err != nil || addr.Zone() != "" {
		return netip.Addr{}, false
	}
	return addr.Unmap(), true
}

func inPrefixes(addr netip.Addr, prefixes []netip.Prefix) bool {
	for _, prefix := range prefixes {
		if prefix.Contains(addr.Unmap()) {
			return true
		}
	}
	return false
}

// RealClientIP is the safe default for callers without a configured proxy boundary.
func RealClientIP(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.RemoteAddr = ClientIP(r)
		next.ServeHTTP(w, r)
	})
}
