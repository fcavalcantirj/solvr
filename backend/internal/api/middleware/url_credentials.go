package middleware

import (
	"net/http"
	"slices"
)

// urlCredentialParams are the query parameters that once carried a credential on a
// room route: `?token=` a per-agent room token, `?access_token=` a session JWT.
var urlCredentialParams = []string{"token", "access_token"}

// RefuseURLCredentials makes the room routes header-only (idx 75 step 5). A URL is
// copied out of the address bar, written by every proxy in front of the API, kept in
// browser history and pasted into screenshots, so a credential that outlives the page
// must not ride in one. A request that puts one in the query string is refused with a
// 400 that names the header and never echoes the value, instead of being silently
// downgraded to an anonymous caller.
//
// The one URL credential a browser needs is the short-lived, room-bound `?ticket=` of
// the read stream (SSEStreamTicket): it is not a bearer, authorizes no write, and is
// left alone here.
func RefuseURLCredentials(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if slices.ContainsFunc(urlCredentialParams, r.URL.Query().Has) {
			roomGuardError(w, http.StatusBadRequest, "VALIDATION_ERROR",
				"credentials in the URL are refused: send Authorization: Bearer <credential> in the request header; "+
					"a browser stream opens with ?ticket= from POST /v1/rooms/{slug}/stream-ticket")
			return
		}
		next.ServeHTTP(w, r)
	})
}
