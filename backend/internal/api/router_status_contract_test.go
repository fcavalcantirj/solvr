package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/hub"
	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// malformedResourceID is not a UUID, a slug or an agent id: it names nothing.
const malformedResourceID = "not-a-valid-id"

// newStatusContractServer serves the full router (rooms included) over a local test
// database and returns it with the mux, so a test can walk every mounted route.
func newStatusContractServer(t *testing.T) (*httptest.Server, *chi.Mux, *db.Pool) {
	t.Helper()
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	pool, err := db.NewPool(context.Background(), dbURL)
	require.NoError(t, err)
	registry := hub.NewPresenceRegistry()
	hubMgr := hub.NewHubManager(context.Background(), registry, slog.Default(), 0)
	router := NewRouter(pool, hubMgr, registry)
	ts := httptest.NewServer(router)
	t.Cleanup(func() {
		ts.Close()
		pool.Close()
	})
	return ts, router, pool
}

// statusContractIdentities returns an anonymous caller, an agent API key and a human JWT,
// removing the agent and the human when the test ends.
func statusContractIdentities(t *testing.T, ts *httptest.Server, pool *db.Pool) map[string]string {
	t.Helper()
	_, jwt := createLiveTestUser(t, pool, models.UserRoleUser)
	_, agentKey := statusContractAgent(t, ts, pool)
	return map[string]string{"anonymous": "", "agent key": agentKey, "human JWT": jwt}
}

// statusContractAgent registers an agent through the API and removes it when the test ends.
func statusContractAgent(t *testing.T, ts *httptest.Server, pool *db.Pool) (agentID, agentKey string) {
	t.Helper()
	agentID, agentKey = registerRoomTestAgent(t, ts)
	t.Cleanup(func() {
		ctx := context.Background()
		pool.Exec(ctx, "DELETE FROM claim_tokens WHERE agent_id = $1", agentID) //nolint:errcheck
		pool.Exec(ctx, "DELETE FROM agents WHERE id = $1", agentID)             //nolint:errcheck
	})
	return agentID, agentKey
}

type statusContractAnswer struct {
	status    int
	code      string
	message   string
	requestID string
	headerID  string
	body      string
}

// callStatusContract sends one request with a deadline, so a route that never answers
// fails the test instead of wedging the run.
func callStatusContract(client *http.Client, method, url, bearer, body string) (statusContractAnswer, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, reader)
	if err != nil {
		return statusContractAnswer{}, err
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	resp, err := client.Do(req)
	if err != nil {
		return statusContractAnswer{}, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return statusContractAnswer{}, err
	}
	var envelope struct {
		Error struct {
			Code      string `json:"code"`
			Message   string `json:"message"`
			RequestID string `json:"request_id"`
		} `json:"error"`
	}
	_ = json.Unmarshal(raw, &envelope)
	return statusContractAnswer{
		status:    resp.StatusCode,
		code:      envelope.Error.Code,
		message:   envelope.Error.Message,
		requestID: envelope.Error.RequestID,
		headerID:  resp.Header.Get("X-Request-ID"),
		body:      string(raw),
	}, nil
}

// A resource id that names nothing — malformed or well formed but absent — is a 404
// NOT_FOUND in the public error envelope on every route that looks one up, never a 500.
func TestStatusContract_UnknownResourceIDIsNotFound(t *testing.T) {
	ts, _, pool := newStatusContractServer(t)
	ids := statusContractIdentities(t, ts, pool)
	agentKey := ids["agent key"]
	client := &http.Client{}

	cases := []struct {
		name, method, path, bearer, body string
	}{
		{"list comments on a post", "GET", "/v1/posts/{id}/comments", "", ""},
		{"list comments on an answer", "GET", "/v1/answers/{id}/comments", "", ""},
		{"list comments on an approach", "GET", "/v1/approaches/{id}/comments", "", ""},
		{"list comments on a response", "GET", "/v1/responses/{id}/comments", "", ""},
		// The legacy comment, answer and approach writes are retired (task idx 52: 410 for any
		// id, TestLegacyWriteRoutes_AnswerTheMigrationErrorAndWriteNothing); their canonical
		// replacements look the id up.
		{"reply on a post", "POST", "/v1/posts/{id}/replies", agentKey, `{"body":"status contract reply"}`},
		{"edit a reply", "PATCH", "/v1/replies/{id}", agentKey, `{"body":"status contract edit"}`},
		{"delete a reply", "DELETE", "/v1/replies/{id}", agentKey, ""},
		{"vote on a reply", "POST", "/v1/replies/{id}/vote", agentKey, `{"direction":"up"}`},
		{"approach history", "GET", "/v1/problems/" + uuid.NewString() + "/approaches/{id}/history", "", ""},
		{"mark a notification read", "POST", "/v1/notifications/{id}/read", agentKey, ""},
		{"delete a notification", "DELETE", "/v1/notifications/{id}", agentKey, ""},
		{"get a pin", "GET", "/v1/pins/{id}", agentKey, ""},
		{"delete a pin", "DELETE", "/v1/pins/{id}", agentKey, ""},
		{"remove a bookmark", "DELETE", "/v1/users/me/bookmarks/{id}", agentKey, ""},
		{"record a post view", "POST", "/v1/posts/{id}/view", "", ""},
		{"read a post view count", "GET", "/v1/posts/{id}/views", "", ""},
	}
	for _, tc := range cases {
		for _, id := range []struct{ kind, value string }{
			{"malformed id", malformedResourceID},
			{"absent id", uuid.NewString()},
		} {
			t.Run(tc.name+"/"+id.kind, func(t *testing.T) {
				url := ts.URL + strings.Replace(tc.path, "{id}", id.value, 1)
				got, err := callStatusContract(client, tc.method, url, tc.bearer, tc.body)
				require.NoError(t, err)
				require.Equal(t, http.StatusNotFound, got.status, "%s %s: %s", tc.method, url, got.body)
				require.Equal(t, "NOT_FOUND", got.code, got.body)
				require.NotEmpty(t, got.message, got.body)
				require.NotEmpty(t, got.headerID)
				require.Equal(t, got.headerID, got.requestID, "the envelope carries the response's request id")
			})
		}
	}
}

// No mounted route answers 5xx because a path id names nothing. Every route with a path
// parameter (rooms included) is called with a malformed and an absent id, as an anonymous
// caller, an agent and a human; each answer must be a client error or a success. Only the
// admin surface and the open-ended SSE streams are left out.
func TestStatusContract_NoRouteAnswers5xxForAnUnknownID(t *testing.T) {
	ts, router, pool := newStatusContractServer(t)
	ids := statusContractIdentities(t, ts, pool)
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}

	type route struct{ method, pattern string }
	var routes []route
	require.NoError(t, chi.Walk(router, func(method, pattern string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		if strings.Contains(pattern, "{") && !strings.HasPrefix(pattern, "/admin") && !strings.HasSuffix(pattern, "/stream") {
			routes = append(routes, route{method, strings.TrimSuffix(pattern, "/*")})
		}
		return nil
	}))
	var roomRoutes int
	for _, r := range routes {
		if strings.HasPrefix(r.pattern, "/v1/rooms/{") || strings.HasPrefix(r.pattern, "/r/{") {
			roomRoutes++
		}
	}
	require.Greater(t, len(routes), 60, "the walk covers the mounted API")
	require.Greater(t, roomRoutes, 20, "the walk covers the room routes")

	param := regexp.MustCompile(`\{[^}]+\}`)
	var failures []string
	for _, r := range routes {
		body := ""
		if r.method == "POST" || r.method == "PATCH" || r.method == "PUT" {
			body = `{}`
		}
		paths := map[string]string{
			"malformed id": param.ReplaceAllString(r.pattern, malformedResourceID),
			"absent id":    param.ReplaceAllStringFunc(r.pattern, func(string) string { return uuid.NewString() }),
		}
		for kind, path := range paths {
			for who, bearer := range ids {
				got, err := callStatusContract(client, r.method, ts.URL+path, bearer, body)
				switch {
				case errors.Is(err, context.DeadlineExceeded):
					failures = append(failures, r.method+" "+r.pattern+" ("+kind+", "+who+"): no answer within 10s")
				case err != nil:
					failures = append(failures, r.method+" "+r.pattern+" ("+kind+", "+who+"): "+err.Error())
				case got.status >= 500:
					failures = append(failures, r.method+" "+r.pattern+" ("+kind+", "+who+"): "+got.body)
				}
			}
		}
	}
	require.Empty(t, failures, "%d answers were 5xx or never came:\n%s", len(failures), strings.Join(failures, "\n"))
}
