package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fcavalcantirj/solvr/internal/api/handlers"
	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/hub"
)

// What the people who run Solvr can read, and nobody else.
//
// Traffic, acquisition, retention, cost, raw diagnostics and growth planning
// are reporting Solvr does about ITSELF. They are not a feature of the
// product, so no credential the product issues reaches them: not a human
// account, not an agent key, not a user key, not a room owner, not a
// room-scoped token. The gate is the server's, and these tests call the real
// router over real HTTP with real credentials to prove it — an endpoint that
// merely has no link pointing at it is not protected.
//
// The second half of the danger is caching. An operator report that a shared
// cache is allowed to store can be handed to the next visitor who asks for the
// same URL, so every operator answer is unstorable and keyed on the
// credential, while the public overview stays cacheable and identical whoever
// asks for it.

const operatorTestKey = "router-operator-key-0123456789abcdef"

// setupOperatorTestServer is setupRoomTestServer plus the mux itself, because
// these tests enumerate the registered routes rather than trusting a list
// somebody maintained by hand.
func setupOperatorTestServer(t *testing.T) (*httptest.Server, *chi.Mux, *db.Pool, func()) {
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

	cleanup := func() {
		ts.Close()
		ctx := context.Background()
		pool.Exec(ctx, "DELETE FROM messages WHERE room_id IN (SELECT id FROM rooms WHERE slug LIKE 'test-%')")
		pool.Exec(ctx, "DELETE FROM agent_presence WHERE room_id IN (SELECT id FROM rooms WHERE slug LIKE 'test-%')")
		pool.Exec(ctx, "DELETE FROM rooms WHERE slug LIKE 'test-%'")
		pool.Exec(ctx, "DELETE FROM users WHERE username LIKE 'roomtest_%'")
		pool.Close()
	}
	return ts, router, pool, cleanup
}

// operatorTestCaller is one way of being somebody on Solvr.
type operatorTestCaller struct {
	name    string
	headers map[string]string
}

// productCredentials issues one of every credential the product hands out: an
// ordinary human account, an agent API key, a human who owns a room, and the
// room-scoped token that room handed back.
func productCredentials(t *testing.T, ts *httptest.Server, pool *db.Pool) []operatorTestCaller {
	t.Helper()

	_, humanJWT := createRoomTestUser(t, pool)
	_, ownerJWT := createRoomTestUser(t, pool)
	_, roomToken := createTestRoomWithToken(t, ts, ownerJWT)
	_, agentKey := registerTestAgent(t, ts, fmt.Sprintf("operator_probe_%d", time.Now().UnixNano()%1000000000))

	return []operatorTestCaller{
		{"anonymous", nil},
		{"an ordinary human account", map[string]string{"Authorization": "Bearer " + humanJWT}},
		{"an agent API key", map[string]string{"Authorization": "Bearer " + agentKey}},
		{"an agent API key in its own header", map[string]string{"X-API-Key": agentKey}},
		{"a room owner", map[string]string{"Authorization": "Bearer " + ownerJWT}},
		{"a room-scoped token", map[string]string{"Authorization": "Bearer " + roomToken}},
		{"a room owner carrying the room token too", map[string]string{
			"Authorization": "Bearer " + ownerJWT,
			"X-Room-Token":  roomToken,
		}},
		{"a guessed operator key", map[string]string{handlers.OperatorAccessHeader: "guessed-operator-key"}},
	}
}

// call issues one request with the given credential and returns the response
// and its body.
func call(t *testing.T, method, url string, headers map[string]string, body string) (*http.Response, string) {
	t.Helper()
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, url, reader) //nolint:noctx // test client
	require.NoError(t, err)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	for name, value := range headers {
		req.Header.Set(name, value)
	}
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp, string(raw)
}

// Not one credential the product issues reads an operator report.
func TestOperatorReports_AreRefusedToEveryProductCredential(t *testing.T) {
	ts, _, pool, cleanup := setupOperatorTestServer(t)
	defer cleanup()
	t.Setenv("ADMIN_API_KEY", operatorTestKey)

	callers := productCredentials(t, ts, pool)
	require.NotEmpty(t, handlers.OperatorReports)

	for _, report := range handlers.OperatorReports {
		for _, caller := range callers {
			t.Run(report.Method+" "+report.Path+" / "+caller.name, func(t *testing.T) {
				body := ""
				if report.Method == http.MethodPost {
					body = `{"query":"SELECT 1"}`
				}
				resp, raw := call(t, report.Method, ts.URL+report.Path, caller.headers, body)

				assert.Contains(t, []int{http.StatusUnauthorized, http.StatusForbidden}, resp.StatusCode,
					"%s %s answered %s with %d: %s", report.Method, report.Path, caller.name, resp.StatusCode, raw)
				assert.NotContains(t, raw, `"data"`, "the refusal carried a payload: %s", raw)
			})
		}
	}
}

// The gate covers every operator route the router registers, including any
// added later — the test enumerates the mux rather than a list.
func TestEveryAdminRoute_IsGatedAndUnstorable(t *testing.T) {
	ts, router, pool, cleanup := setupOperatorTestServer(t)
	defer cleanup()
	t.Setenv("ADMIN_API_KEY", operatorTestKey)

	callers := productCredentials(t, ts, pool)

	adminRoutes := map[string][]string{}
	require.NoError(t, chi.Walk(router, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		if strings.HasPrefix(route, "/admin") {
			adminRoutes[route] = append(adminRoutes[route], method)
		}
		return nil
	}))
	require.NotEmpty(t, adminRoutes, "the router must register operator routes for this test to mean anything")

	placeholder := regexp.MustCompile(`\{[^}]+\}`)
	for route, methods := range adminRoutes {
		for _, method := range methods {
			url := ts.URL + placeholder.ReplaceAllString(route, "00000000-0000-0000-0000-000000000000")
			for _, caller := range callers {
				t.Run(method+" "+route+" / "+caller.name, func(t *testing.T) {
					body := ""
					if method == http.MethodPost || method == http.MethodPatch {
						body = `{"query":"SELECT 1"}`
					}
					resp, raw := call(t, method, url, caller.headers, body)

					assert.Contains(t, []int{http.StatusUnauthorized, http.StatusForbidden}, resp.StatusCode,
						"%s %s answered %s with %d: %s", method, route, caller.name, resp.StatusCode, raw)

					cacheControl := resp.Header.Get("Cache-Control")
					assert.Contains(t, cacheControl, "no-store", "%s %s must not be storable", method, route)
					assert.NotContains(t, cacheControl, "public", "%s %s must never be publicly cacheable", method, route)
					assert.Contains(t, resp.Header.Get("Vary"), handlers.OperatorAccessHeader,
						"%s %s must key its cache on the operator credential", method, route)
				})
			}
		}
	}
}

// Operator reporting still works for the operator. Keeping traffic off the
// public index is not the same as losing the ability to read it.
func TestOperatorReports_StillServeTheOperator(t *testing.T) {
	ts, _, _, cleanup := setupOperatorTestServer(t)
	defer cleanup()
	t.Setenv("ADMIN_API_KEY", operatorTestKey)

	readOnly := []string{
		"/admin/search-analytics/summary?days=7",
		"/admin/search-analytics/trending?days=7&limit=5",
		"/admin/email/history",
	}
	for _, path := range readOnly {
		t.Run(path, func(t *testing.T) {
			resp, raw := call(t, http.MethodGet, ts.URL+path,
				map[string]string{handlers.OperatorAccessHeader: operatorTestKey}, "")

			require.Equal(t, http.StatusOK, resp.StatusCode, "the operator was refused its own report: %s", raw)
			var payload map[string]any
			require.NoError(t, json.Unmarshal([]byte(raw), &payload), "body: %s", raw)
			assert.NotEmpty(t, payload, "the report answered empty")

			assert.Contains(t, resp.Header.Get("Cache-Control"), "no-store")
			assert.Contains(t, resp.Header.Get("Vary"), handlers.OperatorAccessHeader)
		})
	}
}

// A server with no operator key configured refuses the report rather than
// serving it to whoever asks.
func TestOperatorReports_WithNoKeyConfiguredAreClosed(t *testing.T) {
	ts, _, _, cleanup := setupOperatorTestServer(t)
	defer cleanup()
	t.Setenv("ADMIN_API_KEY", "")

	resp, raw := call(t, http.MethodGet, ts.URL+"/admin/search-analytics/summary", nil, "")
	assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode, "body: %s", raw)
	assert.NotContains(t, raw, `"summary"`)
}

// The public overview and an operator report can never end up sharing a cache
// entry: they are different URLs, the operator answer is never stored at all,
// and the public answer is byte-for-byte the same whether or not the caller
// happens to hold the operator key — so an entry an operator's own request
// created is safe to serve to a visitor.
func TestPublicOverview_NeverSharesACacheEntryWithAnOperatorReport(t *testing.T) {
	ts, _, _, cleanup := setupOperatorTestServer(t)
	defer cleanup()
	t.Setenv("ADMIN_API_KEY", operatorTestKey)

	operatorPaths := map[string]bool{}
	for _, report := range handlers.OperatorReports {
		operatorPaths[report.Path] = true
	}

	publicOverview := []string{
		"/v1/homepage/overview",
		"/v1/homepage/rooms",
		"/v1/homepage/search",
		"/v1/homepage/api-usage",
	}

	for _, path := range publicOverview {
		t.Run(path, func(t *testing.T) {
			assert.False(t, operatorPaths[path], "%s is registered as an operator report", path)

			anonymous, anonymousBody := call(t, http.MethodGet, ts.URL+path, nil, "")
			require.Equal(t, http.StatusOK, anonymous.StatusCode)
			assert.NotContains(t, anonymous.Header.Get("Cache-Control"), "no-store",
				"the public overview is meant to be cacheable")

			// The same URL read by somebody holding the operator key must not
			// come back enriched — a cache would store that answer under the
			// public key and hand it to the next visitor.
			withKey, withKeyBody := call(t, http.MethodGet, ts.URL+path,
				map[string]string{handlers.OperatorAccessHeader: operatorTestKey}, "")
			require.Equal(t, http.StatusOK, withKey.StatusCode)

			assert.Equal(t, publicFieldNames(t, anonymousBody), publicFieldNames(t, withKeyBody),
				"%s answered the operator with a different shape than a visitor", path)
			assert.Empty(t, operatorTermsIn(t, withKeyBody),
				"%s carried operator material when the operator key was present", path)
		})
	}
}

// No public surface carries Google Analytics or Search Console material: not a
// credential, not a reporting property, not an export.
func TestPublicSurfaces_CarryNoAnalyticsCredentialsOrExports(t *testing.T) {
	ts, _, _, cleanup := setupOperatorTestServer(t)
	defer cleanup()

	for _, endpoint := range publicPrivacyEndpoints {
		t.Run(endpoint, func(t *testing.T) {
			_, raw := getPublicJSON(t, ts.URL+endpoint)
			assert.Empty(t, operatorTermsIn(t, raw), "%s published operator material", endpoint)
		})
	}
}

// A browser cannot even send the operator credential: the CORS allowlist does
// not admit it, so no page served from an allowed origin can call an operator
// report on a visitor's behalf.
func TestCORS_DoesNotAdmitTheOperatorCredential(t *testing.T) {
	ts, _, _, cleanup := setupOperatorTestServer(t)
	defer cleanup()

	req, err := http.NewRequest(http.MethodOptions, ts.URL+"/admin/search-analytics/summary", nil) //nolint:noctx // test client
	require.NoError(t, err)
	req.Header.Set("Origin", "https://solvr.dev")
	req.Header.Set("Access-Control-Request-Method", "GET")
	req.Header.Set("Access-Control-Request-Headers", handlers.OperatorAccessHeader)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	allowed := resp.Header.Get("Access-Control-Allow-Headers")
	assert.NotContains(t, strings.ToLower(allowed), strings.ToLower(handlers.OperatorAccessHeader),
		"the browser allowlist admits the operator credential")
}

// publicFieldNames collects every field name in a payload, so two answers can
// be compared by shape rather than by the numbers that move between reads.
func publicFieldNames(t *testing.T, raw string) []string {
	t.Helper()
	var payload any
	require.NoError(t, json.Unmarshal([]byte(raw), &payload), "body: %s", raw)

	seen := map[string]bool{}
	var walk func(node any)
	walk = func(node any) {
		switch v := node.(type) {
		case map[string]any:
			for key, child := range v {
				seen[key] = true
				walk(child)
			}
		case []any:
			for _, child := range v {
				walk(child)
			}
		}
	}
	walk(payload)

	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	sortStrings(names)
	return names
}

// operatorTermsIn reports the operator material found anywhere in a response
// body, field names and API-authored text alike.
func operatorTermsIn(t *testing.T, raw string) []string {
	t.Helper()
	var payload any
	require.NoError(t, json.Unmarshal([]byte(raw), &payload), "body: %s", raw)

	var found []string
	var walk func(node any)
	walk = func(node any) {
		switch v := node.(type) {
		case map[string]any:
			for key, child := range v {
				if term := handlers.OperatorCredentialTermInKey(key); term != "" {
					found = append(found, key+" ("+term+")")
				}
				if text, isText := child.(string); isText && handlers.PublicOverviewAuthoredText(key) {
					if term := handlers.OperatorCredentialTermIn(text); term != "" {
						found = append(found, key+": "+term)
					}
				}
				walk(child)
			}
		case []any:
			for _, child := range v {
				walk(child)
			}
		}
	}
	walk(payload)
	return found
}

// sortStrings keeps the field-name comparison order-independent.
func sortStrings(values []string) {
	for i := 1; i < len(values); i++ {
		for j := i; j > 0 && values[j] < values[j-1]; j-- {
			values[j], values[j-1] = values[j-1], values[j]
		}
	}
}
