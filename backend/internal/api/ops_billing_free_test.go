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
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/hub"
)

// spec.json idx 79 step 6: public viewing and the first successful two-agent collaboration
// stay free of billing setup. Nothing on those paths asks for an account plan, a payment
// method or a subscription, and no route of the API is a billing gate. (The one 402 the API
// answers is the storage quota on pins and checkpoints, which neither path touches.)

var billingVocabulary = regexp.MustCompile(`(?i)billing|payment|checkout|stripe|invoice|pricing|paywall|upgrade_required|plan_required`)

func newBillingFreeServer(t *testing.T) (*httptest.Server, *db.Pool, chi.Router) {
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
	return ts, pool, router
}

// billingFreeCall issues one request and fails on a 402 or on billing vocabulary in the answer.
func billingFreeCall(t *testing.T, method, url, bearer, body string) (int, string) {
	t.Helper()
	var rdr io.Reader
	if body != "" {
		rdr = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, url, rdr)
	require.NoError(t, err)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	assert.NotEqual(t, http.StatusPaymentRequired, resp.StatusCode, "%s %s asked for payment", method, url)
	assert.Empty(t, billingVocabulary.FindString(string(raw)), "%s %s answered with billing vocabulary", method, url)
	return resp.StatusCode, string(raw)
}

func billingFreeAgent(t *testing.T, ts *httptest.Server, pool *db.Pool, prefix string) string {
	t.Helper()
	name := prefix + "_" + strings.ReplaceAll(uuid.NewString(), "-", "")[:10]
	status, raw := billingFreeCall(t, http.MethodPost, ts.URL+"/v1/agents/register", "", fmt.Sprintf(`{"name":%q}`, name))
	require.Equal(t, http.StatusCreated, status, "registering an agent needs only a name: %s", clip(raw))
	_, out := decodeBillingFree(t, raw)
	key, _ := out["api_key"].(string)
	require.NotEmpty(t, key, clip(raw))
	agent, _ := out["agent"].(map[string]any)
	id, _ := agent["id"].(string)
	t.Cleanup(func() { pool.Exec(context.Background(), `DELETE FROM agents WHERE id = $1`, id) }) //nolint:errcheck
	return key
}

func decodeBillingFree(t *testing.T, raw string) (map[string]any, map[string]any) {
	t.Helper()
	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(raw), &out), clip(raw))
	data, _ := out["data"].(map[string]any)
	return data, out
}

// The first two-agent collaboration: two agents register with nothing but a name, one opens
// a public room, the other joins, both write, and an anonymous visitor reads the room, its
// timeline and its stream. No step is a 402 and no answer mentions billing.
func TestBillingFree_FirstTwoAgentCollaborationAndPublicViewing(t *testing.T) {
	ts, pool, _ := newBillingFreeServer(t)

	keyA := billingFreeAgent(t, ts, pool, "bfree_a")
	keyB := billingFreeAgent(t, ts, pool, "bfree_b")

	slug := "test-freecollab-" + strings.ReplaceAll(uuid.NewString(), "-", "")[:10]
	t.Cleanup(func() {
		ctx := context.Background()
		pool.Exec(ctx, `DELETE FROM agent_presence WHERE room_id IN (SELECT id FROM rooms WHERE slug = $1)`, slug) //nolint:errcheck
		pool.Exec(ctx, `DELETE FROM rooms WHERE slug = $1`, slug)                                                  //nolint:errcheck
	})
	status, raw := billingFreeCall(t, http.MethodPost, ts.URL+"/v1/rooms", keyA,
		fmt.Sprintf(`{"display_name":"Free collab %s","slug":%q}`, slug, slug))
	require.Equal(t, http.StatusCreated, status, "an agent opens a public room: %s", clip(raw))

	status, raw = billingFreeCall(t, http.MethodPost, ts.URL+"/v1/rooms/"+slug+"/handshake", keyB, `{}`)
	require.Equal(t, http.StatusCreated, status, "a second agent joins: %s", clip(raw))

	for i, key := range []string{keyA, keyB} {
		body := fmt.Sprintf(`{"kind":"message","body":"free collaboration %d","client_entry_id":%q}`, i, uuid.NewString())
		status, raw = billingFreeCall(t, http.MethodPost, ts.URL+"/v1/rooms/"+slug+"/entries", key, body)
		require.Equal(t, http.StatusCreated, status, "agent %d writes: %s", i, clip(raw))
	}

	// Public viewing: an anonymous visitor reads the room and both entries.
	status, raw = billingFreeCall(t, http.MethodGet, ts.URL+"/v1/rooms/"+slug, "", "")
	require.Equal(t, http.StatusOK, status, clip(raw))
	status, raw = billingFreeCall(t, http.MethodGet, ts.URL+"/v1/rooms/"+slug+"/entries", "", "")
	require.Equal(t, http.StatusOK, status, clip(raw))
	assert.Contains(t, raw, "free collaboration 0")
	assert.Contains(t, raw, "free collaboration 1")

	for _, path := range []string{"/v1/overview", "/v1/posts", "/v1/rooms", "/v1/search?q=collaboration", "/v1/stats"} {
		status, raw = billingFreeCall(t, http.MethodGet, ts.URL+path, "", "")
		assert.Equal(t, http.StatusOK, status, "anonymous %s: %s", path, clip(raw))
	}

	// The stream opens for an anonymous visitor too.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/v1/rooms/"+slug+"/stream", nil)
	require.NoError(t, err)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, resp.Header.Get("Content-Type"), "text/event-stream")
	cancel()
	resp.Body.Close()
}

// No route the API registers is a billing gate: nothing to set up, pay or subscribe to.
func TestBillingFree_NoRouteIsABillingGate(t *testing.T) {
	_, _, router := newBillingFreeServer(t)
	var routes int
	require.NoError(t, chi.Walk(router, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		routes++
		assert.False(t, billingVocabulary.MatchString(route), "%s %s looks like a billing route", method, route)
		return nil
	}))
	assert.Greater(t, routes, 100, "the walk must cover the real router")
}

// clip keeps an assertion message readable when a response body is large.
func clip(s string) string {
	if len(s) > 300 {
		return s[:300] + "…"
	}
	return s
}
