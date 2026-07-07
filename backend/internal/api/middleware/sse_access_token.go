package middleware

import "net/http"

// SSEAccessTokenToHeader promotes an `?access_token=` query parameter into the
// Authorization header when no header is present, so the downstream auth middleware and
// RoomAccessGuard treat it exactly like a normal `Authorization: Bearer` request.
//
// Browser EventSource cannot set request headers, so a human owner (or any bearer:
// JWT / user API key / room token) can only authenticate an SSE stream via the query
// string. This is wired ONLY on the SSE stream route — every other route stays
// header-only — which scopes the "token in the URL" exposure to SSE alone (BART-156).
func SSEAccessTokenToHeader(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			if tok := r.URL.Query().Get("access_token"); tok != "" {
				r.Header.Set("Authorization", "Bearer "+tok)
			}
		}
		next.ServeHTTP(w, r)
	})
}
