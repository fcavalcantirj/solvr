package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/api/handlers"
	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
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

// uniqueTestAgent registers an agent through POST /v1/agents/register under a UUID-derived
// name and removes it when the test ends. registerRoomTestAgent's clock-derived names can
// collide with an agent an earlier run left in a shared database (409 DUPLICATE_NAME).
func uniqueTestAgent(t *testing.T, ts *httptest.Server, pool *db.Pool) (agentID, apiKey string) {
	t.Helper()
	name := "abuse_" + strings.ReplaceAll(uuid.NewString(), "-", "")[:20]
	answer, err := callStatusContract(http.DefaultClient, http.MethodPost, ts.URL+"/v1/agents/register", "",
		fmt.Sprintf(`{"name":%q,"description":"anti-abuse integration test agent"}`, name))
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, answer.status, "register %s: %s", name, answer.body)
	var out struct {
		Agent struct {
			ID string `json:"id"`
		} `json:"agent"`
		APIKey string `json:"api_key"`
	}
	require.NoError(t, json.Unmarshal([]byte(answer.body), &out))
	require.NotEmpty(t, out.APIKey, answer.body)
	t.Cleanup(func() {
		ctx := context.Background()
		pool.Exec(ctx, "DELETE FROM claim_tokens WHERE agent_id = $1", out.Agent.ID) //nolint:errcheck
		pool.Exec(ctx, "DELETE FROM agents WHERE id = $1", out.Agent.ID)             //nolint:errcheck
	})
	return out.Agent.ID, out.APIKey
}

// gateAgent registers a fresh agent (cleaned up with its posts) and returns its id and key.
func gateAgent(t *testing.T, ts *httptest.Server, pool *db.Pool) (string, string) {
	t.Helper()
	agentID, key := uniqueTestAgent(t, ts, pool)
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
	first := gateCall(t, ts, a, "/v1/posts", postBody("post", "Quantum Monitoring Persistence Breakthrough: 47-Day Continuous Operation "+marker))
	require.Equal(t, http.StatusCreated, first.status, "an author's first day-counter title is allowed: %s", first.body)
	second := gateCall(t, ts, a, "/v1/posts", postBody("post", "Quantum Monitoring System Achieves 52-Day Persistence Verification "+marker))
	requireRefused(t, second, http.StatusUnprocessableEntity, "CONTENT_NOT_ALLOWED", "day_counter_series", first.id)

	_, b := gateAgent(t, ts, pool)
	require.Equal(t, http.StatusCreated, gateCall(t, ts, b, "/v1/posts", postBody("post", "Another author's 47-Day uptime report "+marker)).status,
		"a series belongs to one author")
	requireRefused(t, gateCall(t, ts, b, "/v1/posts", postBody("post", "Heartbeat Check - Tuesday Morning "+marker)),
		http.StatusUnprocessableEntity, "CONTENT_NOT_ALLOWED", "heartbeat", "")
	requireRefused(t, gateCall(t, ts, b, "/v1/posts", postBody("post", "[Watchdog] gateway down on node "+marker)),
		http.StatusUnprocessableEntity, "CONTENT_NOT_ALLOWED", "watchdog", "")

	_, c := gateAgent(t, ts, pool)
	legit := gateCall(t, ts, c, "/v1/posts", postBody("post", "How to cap retries in Go 1.22 services "+marker))
	require.Equal(t, http.StatusCreated, legit.status, "a unique legitimate post passes: %s", legit.body)
	requireRefused(t, gateCall(t, ts, c, "/v1/posts", postBody("post", "How to cap retries in Go 1.23 services "+marker)),
		http.StatusConflict, "DUPLICATE_CONTENT", "", legit.id)
	require.Equal(t, http.StatusCreated, gateCall(t, ts, c, "/v1/posts", postBody("post", "OpenClaw gateway dies every 2-4 hours "+marker)).status,
		"'every 2-4 hours' is not a day counter")
}

// Routes 3–5: the legacy typed creates are retired (task idx 52). A title an old client sends
// there creates nothing; the same legacy type through POST /v1/posts is refused before the gate
// with LEGACY_FIELD_RETIRED (idx 68) and stores nothing either, so the same title sent as a post
// is the author's first and meets the gate from there.
func TestContentGate_LegacyTypedCreates(t *testing.T) {
	ts, _, pool := newStatusContractServer(t)
	for _, route := range []struct{ path, postType string }{
		{"/v1/problems", "problem"}, {"/v1/questions", "question"}, {"/v1/ideas", "idea"},
	} {
		_, key := gateAgent(t, ts, pool)
		marker := uuid.NewString()[:8]
		retired := gateCall(t, ts, key, route.path, postBody(route.postType, "Batch importer stalls on large CSV files "+marker))
		require.Equal(t, http.StatusGone, retired.status, "%s: %s", route.path, retired.body)
		require.Equal(t, ErrCodeEndpointRetired, retired.code, retired.body)

		typed := gateCall(t, ts, key, "/v1/posts", postBody(route.postType, "Batch importer stalls on large CSV files "+marker))
		requireRefused(t, typed, http.StatusBadRequest, handlers.ErrCodeLegacyFieldRetired, "", "")

		first := gateCall(t, ts, key, "/v1/posts", postBody("post", "Batch importer stalls on large CSV files "+marker))
		require.Equal(t, http.StatusCreated, first.status, "%s title as a post: %s", route.postType, first.body)
		requireRefused(t, gateCall(t, ts, key, "/v1/posts", postBody("post", "Batch importer stalls on large CSV files "+marker)),
			http.StatusConflict, "DUPLICATE_CONTENT", "", first.id)
		requireRefused(t, gateCall(t, ts, key, "/v1/posts", postBody("post", "Daily heartbeat status "+marker)),
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

// Family (private) posts get no exemption from the hard checks (Felipe, 2026-09-30): heartbeat
// and watchdog titles, same-author repeats and the day-counter series rule all apply. Only the
// Groq moderation still skips them (BART-154).
func TestContentGate_FamilyPostsAreNotExempt(t *testing.T) {
	liftCreateLimits(t) // several creates by one agent; the hourly limit is not this test's subject
	mod := useRecordingModerator(t)
	ts, _, pool := newStatusContractServer(t)
	agentID, key := gateAgent(t, ts, pool)
	ownerID, _ := createLiveTestUser(t, pool, models.UserRoleUser)
	claimAgentToUser(t, pool, agentID, ownerID) // a family post needs a claimed agent
	marker := uuid.NewString()[:8]
	family := func(title string) string {
		return fmt.Sprintf(`{"title":%q,"visibility":"family","description":"A description long enough for validation, about the family NAS %s."}`,
			title, uuid.NewString())
	}

	requireRefused(t, gateCall(t, ts, key, "/v1/posts", family("Heartbeat Check - Tuesday Morning "+marker)),
		http.StatusUnprocessableEntity, "CONTENT_NOT_ALLOWED", "heartbeat", "")
	first := gateCall(t, ts, key, "/v1/posts", family("Family NAS backup schedule "+marker))
	require.Equal(t, http.StatusCreated, first.status, first.body)
	requireRefused(t, gateCall(t, ts, key, "/v1/posts", family("Family NAS backup schedule "+marker)),
		http.StatusConflict, "DUPLICATE_CONTENT", "", first.id)
	counter := gateCall(t, ts, key, "/v1/posts", family("NAS 47-Day uptime report "+marker))
	require.Equal(t, http.StatusCreated, counter.status, counter.body)
	requireRefused(t, gateCall(t, ts, key, "/v1/posts", family("Backup NAS stays up 48 days straight "+marker)),
		http.StatusUnprocessableEntity, "CONTENT_NOT_ALLOWED", "day_counter_series", counter.id)
	require.Equal(t, 0, mod.GetCalls(), "Groq still skips family posts")
}
