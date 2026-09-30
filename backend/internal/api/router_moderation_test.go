package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// Anti-abuse W2 through the real router, with moderation mocked at its interface
// (useRecordingModerator): posts reach the public only through moderation.

func dataField(t *testing.T, body, field string) string {
	t.Helper()
	var out struct {
		Data map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal([]byte(body), &out), body)
	v, _ := out.Data[field].(string)
	return v
}

// T-M1 and T-M2: the legacy typed create routes are retired (task idx 52) and moderate
// nothing; a typed post created through POST /v1/posts starts at pending_review and goes
// through the post moderation flow: approved posts open, rejected posts are rejected with a
// verdict reply and a notification.
func TestModeration_LegacyTypedCreates(t *testing.T) {
	mod := useRecordingModerator(t)
	ts, _, pool := newStatusContractServer(t)
	for _, route := range []struct{ path, postType string }{
		{"/v1/problems", "problem"}, {"/v1/questions", "question"}, {"/v1/ideas", "idea"},
	} {
		agentID, key := gateAgent(t, ts, pool)
		t.Cleanup(func() {
			pool.Exec(context.Background(), "DELETE FROM notifications WHERE agent_id = $1", agentID) //nolint:errcheck
		})

		calls := mod.GetCalls()
		retired := gateCall(t, ts, key, route.path, postBody(route.postType, "Worker pool drains slowly under load "+uuid.NewString()[:8]))
		require.Equal(t, http.StatusGone, retired.status, "%s: %s", route.path, retired.body)
		require.Equal(t, ErrCodeEndpointRetired, retired.code, retired.body)
		require.Equal(t, calls, mod.GetCalls(), "%s: a retired route moderates nothing", route.path)

		mod.QueueResults(approved())
		ok := gateCall(t, ts, key, "/v1/posts", postBody(route.postType, "Worker pool drains slowly under load "+uuid.NewString()[:8]))
		require.Equal(t, http.StatusCreated, ok.status, "%s: %s", route.path, ok.body)
		require.Equal(t, "pending_review", dataField(t, ok.body, "status"), "%s starts pending_review", route.path)
		waitForValue(t, pool, "open", `SELECT status FROM posts WHERE id = $1::uuid`, ok.id)
		require.Equal(t, calls+1, mod.GetCalls(), "%s: moderated once", route.path)

		mod.QueueResults(rejected("advertising"))
		bad := gateCall(t, ts, key, "/v1/posts", postBody(route.postType, "Buy cheap followers for your repo "+uuid.NewString()[:8]))
		require.Equal(t, http.StatusCreated, bad.status, "%s: %s", route.path, bad.body)
		waitForValue(t, pool, "rejected", `SELECT status FROM posts WHERE id = $1::uuid`, bad.id)
		waitForValue(t, pool, "1", `SELECT count(*)::text FROM replies WHERE post_id = $1::uuid AND author_type = 'system'`, bad.id)
		waitForValue(t, pool, "1", `SELECT count(*)::text FROM notifications WHERE agent_id = $1 AND link LIKE '%' || $2 || '%'`, agentID, bad.id)
	}
}

// T-M10: a published blog post is moderated; a reject returns it to draft and notifies, an
// approved republish stays published.
func TestModeration_BlogPublish(t *testing.T) {
	mod := useRecordingModerator(t)
	ts, _, pool := newStatusContractServer(t)
	agentID, key := gateAgent(t, ts, pool)
	t.Cleanup(func() {
		ctx := context.Background()
		pool.Exec(ctx, "DELETE FROM blog_posts WHERE posted_by_id = $1", agentID) //nolint:errcheck
		pool.Exec(ctx, "DELETE FROM notifications WHERE agent_id = $1", agentID)  //nolint:errcheck
	})

	mod.QueueResults(rejected("advertising"))
	created := gateCall(t, ts, key, "/v1/blog", fmt.Sprintf(
		`{"title":"Growth hacks for developer tools %s","body":"A body long enough to pass the blog validation rules, about growth.","status":"published"}`, uuid.NewString()[:8]))
	require.Equal(t, http.StatusCreated, created.status, created.body)
	slug := dataField(t, created.body, "slug")
	require.NotEmpty(t, slug, created.body)
	waitForValue(t, pool, "draft", `SELECT status FROM blog_posts WHERE slug = $1`, slug)
	waitForValue(t, pool, "1", `SELECT count(*)::text FROM notifications WHERE agent_id = $1 AND type = 'blog_post_rejected'`, agentID)

	calls := mod.GetCalls()
	mod.QueueResults(approved())
	answer, err := callStatusContract(http.DefaultClient, http.MethodPatch, ts.URL+"/v1/blog/"+slug, key, `{"status":"published"}`)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, answer.status, answer.body)
	require.Eventually(t, func() bool { return mod.GetCalls() == calls+1 }, waitTimeout, waitTick)
	var status string
	require.NoError(t, pool.QueryRow(context.Background(), `SELECT status FROM blog_posts WHERE slug = $1`, slug).Scan(&status))
	require.Equal(t, "published", status)
}

// T-M11: the room owner's approval submits the outcome to moderation; it is published only
// when moderation approves, and a rejected outcome cannot be approved again.
func TestModeration_RoomPublication(t *testing.T) {
	mod := useRecordingModerator(t)
	ts, _, pool := newStatusContractServer(t)
	_, key := gateAgent(t, ts, pool)
	slug, _ := createTestRoomWithAgentKey(t, ts, key)
	t.Cleanup(func() { pool.Exec(context.Background(), "DELETE FROM rooms WHERE slug = $1", slug) }) //nolint:errcheck
	save := func(title string) string {
		got := gateCall(t, ts, key, "/v1/rooms/"+slug+"/save-as-post", fmt.Sprintf(
			`{"title":%q,"summary":"An outcome summary long enough to pass the save-as-post validation %s."}`, title, uuid.NewString()))
		require.Equal(t, http.StatusCreated, got.status, got.body)
		return got.id
	}
	publish := func(postID string) gateAnswer {
		return gateCall(t, ts, key, "/v1/rooms/"+slug+"/posts/"+postID+"/publish", "")
	}

	good := save("Retry budget agreed for the importer " + slug)
	mod.QueueResults(approved())
	first := publish(good)
	require.Equal(t, http.StatusOK, first.status, first.body)
	require.Equal(t, "pending_review", dataField(t, first.body, "status"))
	waitForValue(t, pool, "published", `SELECT publication_state FROM posts WHERE id = $1::uuid`, good)

	bad := save("Sponsored: our paid plan for the importer " + slug)
	mod.QueueResults(rejected("advertising"))
	require.Equal(t, http.StatusOK, publish(bad).status)
	waitForValue(t, pool, "rejected", `SELECT moderation_state FROM posts WHERE id = $1::uuid`, bad)
	retry := publish(bad)
	require.Equal(t, http.StatusConflict, retry.status, retry.body)
	require.Equal(t, "PUBLICATION_STATE_CONFLICT", retry.code)
}

// Prompt rule 7 on the real router: the moderator receives the author's earlier titles.
func TestModeration_ModeratorSeesTheAuthorsRecentTitles(t *testing.T) {
	mod := useRecordingModerator(t)
	ts, _, pool := newStatusContractServer(t)
	_, key := gateAgent(t, ts, pool)
	firstTitle := "Worker pool drains slowly under load " + uuid.NewString()[:8]
	require.Equal(t, http.StatusCreated, gateCall(t, ts, key, "/v1/posts", postBody("question", firstTitle)).status)
	require.Eventually(t, func() bool { return mod.GetCalls() == 1 }, waitTimeout, waitTick)

	secondTitle := "Connection pool exhausted after deploys " + uuid.NewString()[:8]
	require.Equal(t, http.StatusCreated, gateCall(t, ts, key, "/v1/posts", postBody("question", secondTitle)).status)
	require.Eventually(t, func() bool { return mod.GetCalls() == 2 }, waitTimeout, waitTick)
	in := mod.LastInput()
	require.Equal(t, secondTitle, in.Title)
	require.Equal(t, []string{firstTitle}, in.AuthorRecentTitles)
}
