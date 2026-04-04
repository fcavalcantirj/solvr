// Package middleware provides HTTP middleware for the Solvr API.
package middleware

import "net/http"

// SSENoBuffering sets the X-Accel-Buffering header to "no" to prevent
// reverse proxies (Traefik/Easypanel) from buffering SSE streams.
// Apply to all SSE route groups. Per Phase 14 D-02.
//
// Without this header, Traefik buffers SSE frames for ~30 seconds and delivers
// them in a batch, breaking real-time streaming. The header is safe to set on
// non-SSE responses because Traefik (and nginx) only honour it when the
// response Content-Type is text/event-stream.
func SSENoBuffering(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Accel-Buffering", "no")
		next.ServeHTTP(w, r)
	})
}
