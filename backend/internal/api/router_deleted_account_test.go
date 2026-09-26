package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/auth"
	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// A deleted account stops authenticating (SPEC Part 20.2 and 20.3). The JWT minted before
// DELETE /v1/me, the user API key of that account and the key of an agent deleted through
// DELETE /v1/agents/me are invalid authentication: 401 where a route requires it, an
// anonymous caller where it is optional, never a success under the old identity and never
// a 5xx.
func TestDeletedAccount_CredentialsStopAuthenticating(t *testing.T) {
	ts, router, pool := newStatusContractServer(t)
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	userID, jwt := createLiveTestUser(t, pool, models.UserRoleUser)
	_, agentKey := statusContractAgent(t, ts, pool)

	got, err := callStatusContract(client, "POST", ts.URL+"/v1/users/me/api-keys", jwt, `{"name":"deleted account contract"}`)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, got.status, got.body)
	var created struct {
		Data struct {
			Key string `json:"key"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal([]byte(got.body), &created))
	require.True(t, strings.HasPrefix(created.Data.Key, auth.UserAPIKeyPrefix), got.body)
	slug := createPrivateRoomWithJWT(t, ts.URL, jwt)

	deleted := map[string]string{"human JWT": jwt, "user API key": created.Data.Key, "agent key": agentKey}
	for who, credential := range deleted {
		got, err := callStatusContract(client, "GET", ts.URL+"/v1/me", credential, "")
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, got.status, "%s works while its account is live: %s", who, got.body)
	}
	got, err = callStatusContract(client, "GET", ts.URL+"/v1/rooms/"+slug, jwt, "")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, got.status, "the owner reads the private room while live: %s", got.body)

	got, err = callStatusContract(client, "DELETE", ts.URL+"/v1/agents/me", agentKey, "")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, got.status, got.body)
	got, err = callStatusContract(client, "DELETE", ts.URL+"/v1/me", jwt, "")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, got.status, got.body)

	t.Run("routes that require authentication answer 401", func(t *testing.T) {
		required := []struct{ method, path, body string }{
			{"GET", "/v1/me", ""},
			{"GET", "/v1/heartbeat", ""},
			{"GET", "/v1/me/posts", ""},
			{"GET", "/v1/notifications", ""},
			{"POST", "/v1/problems", `{"title":"A deleted account must not publish this problem","description":"Written with a credential of an account that was deleted before this request was sent.","tags":["contract"]}`},
			{"POST", "/v1/rooms", fmt.Sprintf(`{"display_name":"Deleted account room","slug":"gone-%d"}`, time.Now().UnixNano()%1000000000)},
			{"POST", "/v1/users/me/api-keys", `{"name":"after deletion"}`},
		}
		for who, credential := range deleted {
			for _, rt := range required {
				got, err := callStatusContract(client, rt.method, ts.URL+rt.path, credential, rt.body)
				require.NoError(t, err)
				require.Equal(t, http.StatusUnauthorized, got.status, "%s %s with the deleted %s: %s", rt.method, rt.path, who, got.body)
				require.Contains(t, []string{auth.ErrCodeUnauthorized, auth.ErrCodeInvalidAPIKey}, got.code, got.body)
				require.Equal(t, got.headerID, got.requestID, got.body)
			}
		}
		got, err := callStatusContract(client, "POST", ts.URL+"/v1/agents/claim", jwt, `{"token":"`+uuid.NewString()+`"}`)
		require.NoError(t, err)
		require.Equal(t, http.StatusUnauthorized, got.status, "claiming an agent with the deleted JWT: %s", got.body)
	})

	t.Run("the deleted owner reads the private room no better than an anonymous caller", func(t *testing.T) {
		for _, path := range []string{"/v1/rooms/" + slug, "/v1/rooms/" + slug + "/messages", "/v1/rooms/" + slug + "/members"} {
			anonymous, err := callStatusContract(client, "GET", ts.URL+path, "", "")
			require.NoError(t, err)
			owner, err := callStatusContract(client, "GET", ts.URL+path, jwt, "")
			require.NoError(t, err)
			require.NotEqual(t, http.StatusOK, anonymous.status, "GET %s: %s", path, anonymous.body)
			require.NotEqual(t, http.StatusOK, owner.status, "GET %s: %s", path, owner.body)
			// idx 74 slice 19: a presented credential that no longer validates is invalid
			// authentication (401), not an anonymous caller; it was `anonymous.status` before.
			require.Equal(t, http.StatusUnauthorized, owner.status, "GET %s: %s", path, owner.body)
		}
	})

	// A deleted account's credential is worth exactly what a credential that was never issued
	// is worth, on every mounted route: the same status, whether the route requires
	// authentication (401), makes it optional (anonymous) or refuses the credential's kind.
	t.Run("every route answers a deleted credential like one never issued", func(t *testing.T) {
		forged, err := auth.GenerateJWT("a-secret-this-server-never-used-32c", userID, "gone@test.solvr.dev", "user", time.Hour)
		require.NoError(t, err)
		random := strings.ReplaceAll(uuid.NewString(), "-", "")
		neverIssued := map[string]string{
			"human JWT":    forged,
			"user API key": auth.UserAPIKeyPrefix + random,
			"agent key":    "solvr_" + random,
		}

		type route struct{ method, pattern string }
		var routes []route
		require.NoError(t, chi.Walk(router, func(method, pattern string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
			if !strings.HasPrefix(pattern, "/admin") && !strings.HasSuffix(pattern, "/stream") {
				routes = append(routes, route{method, strings.TrimSuffix(pattern, "/*")})
			}
			return nil
		}))
		require.Greater(t, len(routes), 150, "the walk covers the mounted API")

		param := regexp.MustCompile(`\{[^}]+\}`)
		var failures []string
		for _, r := range routes {
			body := ""
			if r.method == "POST" || r.method == "PATCH" || r.method == "PUT" {
				body = `{}`
			}
			path := param.ReplaceAllStringFunc(r.pattern, func(string) string { return uuid.NewString() })
			for who, credential := range deleted {
				want, err := callStatusContract(client, r.method, ts.URL+path, neverIssued[who], body)
				require.NoError(t, err, "%s %s", r.method, path)
				got, err := callStatusContract(client, r.method, ts.URL+path, credential, body)
				require.NoError(t, err, "%s %s", r.method, path)
				if got.status != want.status {
					failures = append(failures, fmt.Sprintf("%s %s (%s): deleted %d, never issued %d: %.160s",
						r.method, r.pattern, who, got.status, want.status, got.body))
				}
			}
		}
		require.Empty(t, failures, "%d answers treated a deleted credential differently:\n%s", len(failures), strings.Join(failures, "\n"))
	})
}
