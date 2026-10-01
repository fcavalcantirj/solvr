package api

import (
	"net/http"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// A request body that is not valid JSON is one condition, and the public API names it with
// one code: 400 VALIDATION_ERROR in the error envelope, whichever route family reads the
// body. Clients branch on error.code, so INVALID_JSON, INVALID_REQUEST and BAD_REQUEST for
// the same malformed body would each need their own handling.
//
// The walk sends `{"broken":` to every mounted POST/PUT/PATCH route outside /admin: routes
// without a path parameter as they are, and routes under /v1/posts, /v1/replies, /v1/rooms
// and /r with a live post, reply or room so the handler reaches its body. Anonymous callers,
// an agent key, a human JWT, the room owner's agent key and its per-agent room token each
// make every call; a PATCH carries the If-Match of the caller's own read. A route
// that never reads a body answers 2xx/401/403/404/428 and is not judged here.
func TestStatusContract_MalformedJSONBodyIsOneValidationError(t *testing.T) {
	ts, router, pool := newStatusContractServer(t)
	roomPreCleanup(t, pool)
	callers := statusContractIdentities(t, ts, pool)
	ownerID, ownerKey := statusContractAgent(t, ts, pool)
	callers["room owner agent key"] = ownerKey
	postID := childContractPost(t, pool, ownerID, "public", "", false)
	replyID := childContractReply(t, pool, postID, ownerID, "malformed body contract reply")
	slug, roomToken := createTestRoomWithAgentKey(t, ts, ownerKey)
	callers["room token"] = roomToken
	t.Cleanup(func() { roomPreCleanup(t, pool) })
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}

	type route struct{ method, pattern string }
	var routes []route
	require.NoError(t, chi.Walk(router, func(method, pattern string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		if method != "POST" && method != "PUT" && method != "PATCH" {
			return nil
		}
		if strings.HasPrefix(pattern, "/admin") || strings.HasSuffix(pattern, "/stream") {
			return nil
		}
		routes = append(routes, route{method, strings.TrimSuffix(pattern, "/*")})
		return nil
	}))

	// liveFirstParam names the live resource that fills the first path parameter of a
	// pattern; any further parameter is an absent UUID. It reports false for a pattern
	// this walk has no live resource for.
	liveFirstParam := func(pattern string) (string, bool) {
		switch {
		case !strings.Contains(pattern, "{"):
			return "", true
		case strings.HasPrefix(pattern, "/v1/posts/{"):
			return postID, true
		case strings.HasPrefix(pattern, "/v1/replies/{"):
			return replyID, true
		case strings.HasPrefix(pattern, "/v1/rooms/{"), strings.HasPrefix(pattern, "/r/{"):
			return slug, true
		}
		return "", false
	}
	param := regexp.MustCompile(`\{[^}]+\}`)

	var wrongCode []string
	answered := map[string]bool{}
	var walked int
	for _, r := range routes {
		first, ok := liveFirstParam(r.pattern)
		if !ok {
			continue
		}
		walked++
		n := 0
		path := param.ReplaceAllStringFunc(r.pattern, func(string) string {
			n++
			if n == 1 {
				return first
			}
			return uuid.NewString()
		})
		for who, bearer := range callers {
			// An edit carries the If-Match of the caller's own read, as a client's does
			// since idx 74 step 5; without one it stops at 428 before reading its body.
			ifMatch := ""
			if r.method == "PATCH" {
				ifMatch = currentETag(t, ts.URL+path, bearer)
			}
			got, err := callStatusContractIfMatch(client, r.method, ts.URL+path, bearer, `{"broken":`, ifMatch)
			require.NoError(t, err, "%s %s (%s)", r.method, r.pattern, who)
			if got.status != http.StatusBadRequest {
				continue
			}
			if got.code != "VALIDATION_ERROR" || got.requestID == "" {
				wrongCode = append(wrongCode, r.method+" "+r.pattern+" ("+who+"): "+got.code+" "+got.message)
				continue
			}
			answered[r.method+" "+r.pattern] = true
		}
	}

	require.Greater(t, walked, 45, "the walk covers the mounted write routes")
	sort.Strings(wrongCode)
	require.Empty(t, wrongCode, "malformed JSON answered a code other than VALIDATION_ERROR on %d calls:\n%s",
		len(wrongCode), strings.Join(wrongCode, "\n"))
	for _, must := range []string{
		"POST /v1/rooms/{slug}/entries",
		"POST /v1/rooms/{slug}/handshake",
		"PATCH /v1/rooms/{slug}",
		"POST /v1/rooms/{slug}/archive",
		"POST /v1/rooms/{slug}/members",
		"POST /v1/posts/{id}/replies",
		"PATCH /v1/replies/{id}",
		"POST /v1/auth/login",
		"POST /v1/agents/register",
	} {
		require.True(t, answered[must], "%s answered 400 VALIDATION_ERROR to a malformed body (walk is not vacuous)", must)
	}
}
