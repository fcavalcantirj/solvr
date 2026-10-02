package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// Task "Keep SDKs, CLI, MCP, skills, and webhooks consistent with the redesigned product",
// step 4, through the real router: every moderation event an agent reads — GET
// /v1/notifications and the inbox of its GET /v1/me — carries schema_version 1 and names the
// canonical post and reply it is about in "subject".

type notificationEvent struct {
	Type          string `json:"type"`
	SchemaVersion *int   `json:"schema_version"`
	Subject       *struct {
		PostID  string `json:"post_id"`
		ReplyID string `json:"reply_id"`
	} `json:"subject"`
}

func (e notificationEvent) String() string {
	b, _ := json.Marshal(e)
	return string(b)
}

// agentNotificationEvents reads the agent's notifications and its /v1/me inbox.
func agentNotificationEvents(t *testing.T, ts *httptest.Server, key string) (list, inbox []notificationEvent) {
	t.Helper()
	got, err := callStatusContract(http.DefaultClient, http.MethodGet, ts.URL+"/v1/notifications?per_page=50", key, "")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, got.status, got.body)
	var listed struct {
		Data []notificationEvent `json:"data"`
	}
	require.NoError(t, json.Unmarshal([]byte(got.body), &listed), got.body)

	me, err := callStatusContract(http.DefaultClient, http.MethodGet, ts.URL+"/v1/me", key, "")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, me.status, me.body)
	var briefing struct {
		Data struct {
			Inbox struct {
				Items []notificationEvent `json:"items"`
			} `json:"inbox"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal([]byte(me.body), &briefing), me.body)
	return listed.Data, briefing.Data.Inbox.Items
}

// requireEvent finds the one event of the given type and checks its contract fields.
func requireEvent(t *testing.T, where string, events []notificationEvent, eventType, postID, replyID string) {
	t.Helper()
	var found []notificationEvent
	for _, e := range events {
		if e.Type == eventType {
			found = append(found, e)
		}
	}
	require.Len(t, found, 1, "%s: one %s event in %v", where, eventType, events)
	e := found[0]
	require.NotNil(t, e.SchemaVersion, "%s: %s has schema_version", where, e)
	require.Equal(t, 1, *e.SchemaVersion, "%s: %s", where, e)
	require.NotNil(t, e.Subject, "%s: %s has a subject object", where, e)
	require.Equal(t, postID, e.Subject.PostID, "%s: %s", where, e)
	require.Equal(t, replyID, e.Subject.ReplyID, "%s: %s", where, e)
}

func TestNotificationEvents_PostModerationNamesThePost(t *testing.T) {
	mod := useRecordingModerator(t)
	ts, _, pool := newStatusContractServer(t)
	agentID, key := gateAgent(t, ts, pool)
	t.Cleanup(func() {
		pool.Exec(context.Background(), "DELETE FROM notifications WHERE agent_id = $1", agentID) //nolint:errcheck
	})

	mod.QueueResults(approved())
	ok := gateCall(t, ts, key, "/v1/posts", postBody("post", "Worker pool drains slowly under load "+uuid.NewString()[:8]))
	require.Equal(t, http.StatusCreated, ok.status, ok.body)
	waitForValue(t, pool, "1", `SELECT count(*)::text FROM notifications WHERE agent_id = $1 AND type = 'post.approved'`, agentID)

	mod.QueueResults(rejected("advertising"))
	bad := gateCall(t, ts, key, "/v1/posts", postBody("post", "Buy cheap followers for your repo "+uuid.NewString()[:8]))
	require.Equal(t, http.StatusCreated, bad.status, bad.body)
	waitForValue(t, pool, "1", `SELECT count(*)::text FROM notifications WHERE agent_id = $1 AND type = 'post.rejected'`, agentID)

	list, inbox := agentNotificationEvents(t, ts, key)
	for where, events := range map[string][]notificationEvent{"GET /v1/notifications": list, "GET /v1/me inbox": inbox} {
		requireEvent(t, where, events, "post.approved", ok.id, "")
		requireEvent(t, where, events, "post.rejected", bad.id, "")
	}
}

func TestNotificationEvents_ReplyModerationNamesThePostAndTheReply(t *testing.T) {
	mod := useRecordingModerator(t)
	ts, _, pool := newStatusContractServer(t)
	agentID, key := moderationAgent(t, ts, pool)
	post := seedOpenPost(t, pool, "post")

	mod.QueueResults(rejected(agentID))
	reply := gateCall(t, ts, key, "/v1/posts/"+post+"/replies", `{"body":"reply for the event contract `+uuid.NewString()[:8]+`"}`)
	require.Equal(t, http.StatusCreated, reply.status, reply.body)
	waitForValue(t, pool, "reply.removed", `SELECT string_agg(type, ',') FROM notifications WHERE agent_id = $1`, agentID)

	list, inbox := agentNotificationEvents(t, ts, key)
	requireEvent(t, "GET /v1/notifications", list, "reply.removed", post, reply.id)
	requireEvent(t, "GET /v1/me inbox", inbox, "reply.removed", post, reply.id)
}

func TestNotificationEvents_BlogModerationIsUnderTheContractWithNoSubject(t *testing.T) {
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
	waitForValue(t, pool, "1", `SELECT count(*)::text FROM notifications WHERE agent_id = $1 AND type = 'blog_post_rejected'`, agentID)

	list, inbox := agentNotificationEvents(t, ts, key)
	requireEvent(t, "GET /v1/notifications", list, "blog_post_rejected", "", "")
	requireEvent(t, "GET /v1/me inbox", inbox, "blog_post_rejected", "", "")
}
