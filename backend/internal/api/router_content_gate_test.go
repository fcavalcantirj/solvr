package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// Anti-abuse W1 on the real router: every create path runs the content gate before it inserts.
// Each identity stays within the hourly create limits (W3), so a refusal here is the gate's.

type gateAnswer struct {
	status     int
	code       string
	rule       string
	existingID string
	id         string
	body       string
}

func gateCall(t *testing.T, ts *httptest.Server, bearer, path, body string) gateAnswer {
	t.Helper()
	answer, err := callStatusContract(http.DefaultClient, http.MethodPost, ts.URL+path, bearer, body)
	require.NoError(t, err, path)
	var out struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
		Error struct {
			Details struct {
				Rule       string `json:"rule"`
				ExistingID string `json:"existing_id"`
			} `json:"details"`
		} `json:"error"`
	}
	_ = json.Unmarshal([]byte(answer.body), &out)
	return gateAnswer{status: answer.status, code: answer.code, rule: out.Error.Details.Rule,
		existingID: out.Error.Details.ExistingID, id: out.Data.ID, body: answer.body}
}

func postBody(postType, title string) string {
	return fmt.Sprintf(`{"type":%q,"title":%q,"description":"A description long enough for validation, about retries and backoff %s."}`,
		postType, title, uuid.NewString())
}

// gateAgent registers a fresh agent (cleaned up with its posts) and returns its id and key.
func gateAgent(t *testing.T, ts *httptest.Server, pool *db.Pool) (string, string) {
	t.Helper()
	agentID, key := statusContractAgent(t, ts, pool)
	deletePostsBy(t, pool, agentID)
	return agentID, key
}

func requireRefused(t *testing.T, got gateAnswer, status int, code, rule, existingID string) {
	t.Helper()
	require.Equal(t, status, got.status, got.body)
	require.Equal(t, code, got.code, got.body)
	require.Equal(t, rule, got.rule, got.body)
	require.Equal(t, existingID, got.existingID, got.body)
}

// The required cases, on POST /v1/posts.
func TestContentGate_TitleRulesOnCanonicalPosts(t *testing.T) {
	ts, _, pool := newStatusContractServer(t)
	marker := uuid.NewString()[:8]

	_, a := gateAgent(t, ts, pool)
	first := gateCall(t, ts, a, "/v1/posts", postBody("question", "Quantum Monitoring Persistence Breakthrough: 47-Day Continuous Operation "+marker))
	require.Equal(t, http.StatusCreated, first.status, "an author's first day-counter title is allowed: %s", first.body)
	second := gateCall(t, ts, a, "/v1/posts", postBody("question", "Quantum Monitoring System Achieves 52-Day Persistence Verification "+marker))
	requireRefused(t, second, http.StatusUnprocessableEntity, "CONTENT_NOT_ALLOWED", "day_counter_series", first.id)

	_, b := gateAgent(t, ts, pool)
	require.Equal(t, http.StatusCreated, gateCall(t, ts, b, "/v1/posts", postBody("question", "Another author's 47-Day uptime report "+marker)).status,
		"a series belongs to one author")
	requireRefused(t, gateCall(t, ts, b, "/v1/posts", postBody("question", "Heartbeat Check - Tuesday Morning "+marker)),
		http.StatusUnprocessableEntity, "CONTENT_NOT_ALLOWED", "heartbeat", "")
	requireRefused(t, gateCall(t, ts, b, "/v1/posts", postBody("question", "[Watchdog] gateway down on node "+marker)),
		http.StatusUnprocessableEntity, "CONTENT_NOT_ALLOWED", "watchdog", "")

	_, c := gateAgent(t, ts, pool)
	legit := gateCall(t, ts, c, "/v1/posts", postBody("question", "How to cap retries in Go 1.22 services "+marker))
	require.Equal(t, http.StatusCreated, legit.status, "a unique legitimate post passes: %s", legit.body)
	requireRefused(t, gateCall(t, ts, c, "/v1/posts", postBody("question", "How to cap retries in Go 1.23 services "+marker)),
		http.StatusConflict, "DUPLICATE_CONTENT", "", legit.id)
	require.Equal(t, http.StatusCreated, gateCall(t, ts, c, "/v1/posts", postBody("question", "OpenClaw gateway dies every 2-4 hours "+marker)).status,
		"'every 2-4 hours' is not a day counter")
}

// Routes 3–5: the legacy typed creates.
func TestContentGate_LegacyTypedCreates(t *testing.T) {
	ts, _, pool := newStatusContractServer(t)
	for _, route := range []struct{ path, postType string }{
		{"/v1/problems", "problem"}, {"/v1/questions", "question"}, {"/v1/ideas", "idea"},
	} {
		_, key := gateAgent(t, ts, pool)
		marker := uuid.NewString()[:8]
		first := gateCall(t, ts, key, route.path, postBody(route.postType, "Batch importer stalls on large CSV files "+marker))
		require.Equal(t, http.StatusCreated, first.status, "%s: %s", route.path, first.body)
		requireRefused(t, gateCall(t, ts, key, route.path, postBody(route.postType, "Batch importer stalls on large CSV files "+marker)),
			http.StatusConflict, "DUPLICATE_CONTENT", "", first.id)
		requireRefused(t, gateCall(t, ts, key, route.path, postBody(route.postType, "Daily heartbeat status "+marker)),
			http.StatusUnprocessableEntity, "CONTENT_NOT_ALLOWED", "heartbeat", "")
	}
}

// Route 11: POST /v1/blog, compared against the author's blog posts.
func TestContentGate_BlogCreate(t *testing.T) {
	ts, _, pool := newStatusContractServer(t)
	agentID, jwt := gateAgent(t, ts, pool)
	t.Cleanup(func() {
		pool.Exec(context.Background(), "DELETE FROM blog_posts WHERE posted_by_id = $1", agentID) //nolint:errcheck
	})
	marker := uuid.NewString()[:8]
	blog := func(title string) string {
		return fmt.Sprintf(`{"title":%q,"body":"A blog body long enough to pass validation, about scaling Go workers %s."}`, title, uuid.NewString())
	}
	first := gateCall(t, ts, jwt, "/v1/blog", blog("Scaling Go workers in 2025 "+marker))
	require.Equal(t, http.StatusCreated, first.status, first.body)
	requireRefused(t, gateCall(t, ts, jwt, "/v1/blog", blog("Scaling Go workers in 2026 "+marker)),
		http.StatusConflict, "DUPLICATE_CONTENT", "", first.id)
	requireRefused(t, gateCall(t, ts, jwt, "/v1/blog", blog("[Watchdog] weekly uptime "+marker)),
		http.StatusUnprocessableEntity, "CONTENT_NOT_ALLOWED", "watchdog", "")
}

// Route 12: POST /v1/rooms/{slug}/save-as-post.
func TestContentGate_RoomSaveAsPost(t *testing.T) {
	ts, _, pool := newStatusContractServer(t)
	_, key := gateAgent(t, ts, pool)
	slug, _ := createTestRoomWithAgentKey(t, ts, key)
	t.Cleanup(func() { pool.Exec(context.Background(), "DELETE FROM rooms WHERE slug = $1", slug) }) //nolint:errcheck
	save := func(title string) gateAnswer {
		return gateCall(t, ts, key, "/v1/rooms/"+slug+"/save-as-post", fmt.Sprintf(
			`{"title":%q,"summary":"An outcome summary long enough to pass the save-as-post validation %s."}`, title, uuid.NewString()))
	}
	requireRefused(t, save("Agent death: worker-7 stopped responding"),
		http.StatusUnprocessableEntity, "CONTENT_NOT_ALLOWED", "watchdog", "")
	first := save("Retry budget agreed for the importer " + slug)
	require.Equal(t, http.StatusCreated, first.status, first.body)
	requireRefused(t, save("Retry budget agreed for the importer "+slug), http.StatusConflict, "DUPLICATE_CONTENT", "", first.id)
}
