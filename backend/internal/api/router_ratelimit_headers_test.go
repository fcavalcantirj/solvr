package api

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// The per-IP room limits (agent writes, stream tickets) answer with the standard RateLimit-*
// headers next to httprate's X-RateLimit-*, and a browser page may read both families.

func TestRoomLimits_SendBothRateLimitHeaderFamilies(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	_, jwt := createRoomTestUser(t, pool)
	slug, roomToken := createTestRoomWithToken(t, ts, jwt)

	for _, c := range []struct {
		path, body, limit string
	}{
		{"/v1/rooms/" + slug + "/entries", fmt.Sprintf(`{"body":"limits","client_entry_id":"rl-%d"}`, time.Now().UnixNano()), "60"},
		{"/v1/rooms/" + slug + "/stream-ticket", "", "30"},
	} {
		resp := doRoomRequest(t, http.MethodPost, ts.URL+c.path, c.body, roomToken)
		raw, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		require.True(t, resp.StatusCode < 300, "POST %s: %d %s", c.path, resp.StatusCode, raw)
		for _, name := range []string{"RateLimit-Limit", "X-RateLimit-Limit"} {
			require.Equal(t, c.limit, resp.Header.Get(name), "POST %s: %s", c.path, name)
		}
		for _, name := range []string{"RateLimit-Remaining", "RateLimit-Reset", "X-RateLimit-Remaining", "X-RateLimit-Reset"} {
			require.NotEmpty(t, resp.Header.Get(name), "POST %s: %s", c.path, name)
		}
	}
}

func TestCORS_ExposesTheStandardRateLimitHeaders(t *testing.T) {
	router := NewRouter(nil, nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	exposed := map[string]bool{}
	for _, h := range strings.Split(w.Header().Get("Access-Control-Expose-Headers"), ",") {
		exposed[strings.ToLower(strings.TrimSpace(h))] = true
	}
	for _, h := range []string{"RateLimit-Limit", "RateLimit-Remaining", "RateLimit-Reset",
		"X-RateLimit-Limit", "X-RateLimit-Remaining", "X-RateLimit-Reset", "Retry-After"} {
		require.True(t, exposed[strings.ToLower(h)], "Access-Control-Expose-Headers lacks %s (exact token): %q",
			h, w.Header().Get("Access-Control-Expose-Headers"))
	}
}

func TestOpenAPIConventions_DocumentTheRateLimitHeaders(t *testing.T) {
	spec := servedSpec(t)
	rl := at(t, spec, "x-solvr-conventions", "rate_limits")
	headers := strings_(t, at(t, rl, "response_headers"))
	sameSet(t, "rate_limits.response_headers", headers, []string{
		"RateLimit-Limit", "RateLimit-Remaining", "RateLimit-Reset",
		"X-RateLimit-Limit", "X-RateLimit-Remaining", "X-RateLimit-Reset",
	})
	exposed := map[string]bool{}
	for _, h := range corsExposedHeaders {
		exposed[h] = true
	}
	for _, h := range append(headers, "Retry-After") {
		require.True(t, exposed[h], "%s is documented but a browser page may not read it", h)
	}
	require.Equal(t, "Retry-After", at(t, rl, "retry_header"))
	require.Equal(t, "CF-Connecting-IP", at(t, rl, "client_ip_header"))
	require.NotEmpty(t, at(t, rl, "limits"))

	for _, name := range []string{"RateLimitLimit", "RateLimitRemaining", "RateLimitReset"} {
		at(t, spec, "components", "headers", name)
	}
	limited := at(t, spec, "components", "responses", "RateLimited", "headers").(map[string]interface{})
	for _, h := range []string{"RateLimit-Limit", "RateLimit-Remaining", "RateLimit-Reset", "Retry-After"} {
		require.Contains(t, limited, h, "the 429 response documents %s", h)
	}
}
