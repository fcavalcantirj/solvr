package api

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// Task "Keep SDKs, CLI, MCP, skills, and webhooks consistent with the redesigned product",
// step 4, webhook half, through the real router: an agent subscribes its own webhook to a
// schema-1 notification event, the event is queued with the notification, and the delivery
// job — running on two API instances at once — sends it with one delivery ID that a retry
// preserves, so the receiver acts on it once.

// webhookReceiver is an HTTPS endpoint that answers the queued statuses in order (then 204)
// and performs one "action" per delivery ID it accepts.
type webhookReceiver struct {
	*httptest.Server
	mu       sync.Mutex
	answers  []int
	requests []receivedWebhook
	actions  map[string]int
}

type receivedWebhook struct {
	deliveryID, attempt, signature, event string
	body                                  []byte
	header                                http.Header
}

func newWebhookReceiver(t *testing.T, answers ...int) *webhookReceiver {
	t.Helper()
	rcv := &webhookReceiver{answers: answers, actions: map[string]int{}}
	rcv.Server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		rcv.mu.Lock()
		defer rcv.mu.Unlock()
		got := receivedWebhook{deliveryID: r.Header.Get("X-Solvr-Delivery-ID"), attempt: r.Header.Get("X-Solvr-Delivery-Attempt"),
			signature: r.Header.Get("X-Solvr-Signature"), event: r.Header.Get("X-Solvr-Event"), body: body,
			header: r.Header.Clone()}
		rcv.requests = append(rcv.requests, got)
		status := http.StatusNoContent
		if len(rcv.answers) > 0 {
			status, rcv.answers = rcv.answers[0], rcv.answers[1:]
		}
		if status < 300 {
			rcv.actions[got.deliveryID]++ // a receiver acting once per delivery ID
		}
		w.WriteHeader(status)
	}))
	t.Cleanup(rcv.Close)
	return rcv
}

func (rcv *webhookReceiver) received() []receivedWebhook {
	rcv.mu.Lock()
	defer rcv.mu.Unlock()
	return append([]receivedWebhook(nil), rcv.requests...)
}

// runDeliveryOnTwoInstances runs one delivery pass on two API instances at once, each with
// its own database pool, as two deployed servers would.
func runDeliveryOnTwoInstances(t *testing.T, pool *db.Pool, rcv *webhookReceiver) {
	t.Helper()
	second, err := db.NewPool(context.Background(), os.Getenv("DATABASE_URL"))
	require.NoError(t, err)
	defer second.Close()
	var wg sync.WaitGroup
	for _, p := range []*db.Pool{pool, second} {
		wg.Add(1)
		go func(p *db.Pool) {
			defer wg.Done()
			if _, err := NewWebhookDeliveryJob(p, rcv.Client()).RunOnce(context.Background()); err != nil {
				t.Errorf("delivery run: %v", err)
			}
		}(p)
	}
	wg.Wait()
}

func createWebhookThroughAPI(t *testing.T, ts *httptest.Server, key, agentID, url, secret string, events ...string) statusContractAnswer {
	t.Helper()
	body, _ := json.Marshal(map[string]any{"url": url, "events": events, "secret": secret})
	got, err := callStatusContract(http.DefaultClient, http.MethodPost, ts.URL+"/v1/agents/"+agentID+"/webhooks", key, string(body))
	require.NoError(t, err)
	return got
}

func TestWebhooks_TheAgentsEventIsDeliveredOnceWithOneDeliveryIDAcrossRetriesAndInstances(t *testing.T) {
	mod := useRecordingModerator(t)
	ts, _, pool := newStatusContractServer(t)
	agentID, key := moderationAgent(t, ts, pool)
	post := seedOpenPost(t, pool, "post")
	rcv := newWebhookReceiver(t, http.StatusInternalServerError) // the first attempt fails
	const secret = "whsec-router-test-secret"

	created := createWebhookThroughAPI(t, ts, key, agentID, rcv.URL, secret, "reply.removed")
	require.Equal(t, http.StatusCreated, created.status, created.body)
	require.NotContains(t, created.body, secret, "the secret is never answered")
	var hook struct {
		Data map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal([]byte(created.body), &hook))
	webhookID, _ := hook.Data["id"].(string)
	require.NotEmpty(t, webhookID, created.body)
	require.Equal(t, []any{"reply.removed"}, hook.Data["events"])
	require.NotContains(t, hook.Data, "secret_hash")

	listed, err := callStatusContract(http.DefaultClient, http.MethodGet, ts.URL+"/v1/agents/"+agentID+"/webhooks", key, "")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, listed.status, listed.body)
	require.Contains(t, listed.body, webhookID)

	mod.QueueResults(rejected(agentID))
	reply := gateCall(t, ts, key, "/v1/posts/"+post+"/replies", `{"body":"reply delivered to a webhook `+uuid.NewString()[:8]+`"}`)
	require.Equal(t, http.StatusCreated, reply.status, reply.body)
	waitForValue(t, pool, "1", `SELECT count(*)::text FROM webhook_deliveries WHERE webhook_id = $1::uuid`, webhookID)

	runDeliveryOnTwoInstances(t, pool, rcv)
	require.Len(t, rcv.received(), 1, "two instances sent the due delivery once")
	waitForValue(t, pool, "pending 1", `SELECT status || ' ' || attempts FROM webhook_deliveries WHERE webhook_id = $1::uuid`, webhookID)

	_, err = pool.Exec(context.Background(), `UPDATE webhook_deliveries SET next_attempt_at = NOW() WHERE webhook_id = $1::uuid`, webhookID)
	require.NoError(t, err)
	runDeliveryOnTwoInstances(t, pool, rcv)
	got := rcv.received()
	require.Len(t, got, 2, "the retry was sent once")
	waitForValue(t, pool, "delivered 2", `SELECT status || ' ' || attempts FROM webhook_deliveries WHERE webhook_id = $1::uuid`, webhookID)

	_, err = pool.Exec(context.Background(), `UPDATE webhook_deliveries SET next_attempt_at = NOW() WHERE webhook_id = $1::uuid`, webhookID)
	require.NoError(t, err)
	runDeliveryOnTwoInstances(t, pool, rcv)
	require.Len(t, rcv.received(), 2, "a delivered event is never sent again")

	first, retry := got[0], got[1]
	require.Equal(t, first.deliveryID, retry.deliveryID, "the retry preserves the delivery ID")
	require.Equal(t, string(first.body), string(retry.body), "the retry sends the same event")
	require.Equal(t, []string{"1", "2"}, []string{first.attempt, retry.attempt})
	require.Equal(t, map[string]int{first.deliveryID: 1}, rcv.actions, "the receiver acted once")
	for _, r := range got {
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write(r.body)
		require.Equal(t, "sha256="+hex.EncodeToString(mac.Sum(nil)), r.signature)
		require.Equal(t, "reply.removed", r.event)
	}

	var payload struct {
		ID            string `json:"id"`
		Event         string `json:"event"`
		SchemaVersion int    `json:"schema_version"`
		Data          struct {
			NotificationID string `json:"notification_id"`
			AgentID        string `json:"agent_id"`
			Subject        struct {
				PostID  string `json:"post_id"`
				ReplyID string `json:"reply_id"`
			} `json:"subject"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(first.body, &payload), string(first.body))
	require.Equal(t, first.deliveryID, payload.ID)
	require.Equal(t, "reply.removed", payload.Event)
	require.Equal(t, 1, payload.SchemaVersion)
	require.Equal(t, agentID, payload.Data.AgentID)
	require.Equal(t, post, payload.Data.Subject.PostID, "the event names the canonical post")
	require.Equal(t, reply.id, payload.Data.Subject.ReplyID, "the event names the canonical reply")
	list, _ := agentNotificationEvents(t, ts, key)
	requireEvent(t, "GET /v1/notifications", list, "reply.removed", post, reply.id)
	var notificationID string
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT id::text FROM notifications WHERE agent_id = $1 AND type = 'reply.removed'`, agentID).Scan(&notificationID))
	require.Equal(t, notificationID, payload.Data.NotificationID, "the delivery is the notification the agent reads")
}

func TestWebhooks_RetiredAndUnknownEventNamesAreRefusedWithTheSupportedNames(t *testing.T) {
	ts, _, pool := newStatusContractServer(t)
	agentID, key := gateAgent(t, ts, pool)
	supported := []any{"post.approved", "post.rejected", "reply.removed", "reply.flagged", "blog_post_rejected",
		"room.member_added", "room.member_removed"}

	for _, retired := range []string{"answer.created", "comment.created", "approach.stuck", "problem.solved", "mention"} {
		got := createWebhookThroughAPI(t, ts, key, agentID, "https://receiver.example/hook", "whsec", "reply.removed", retired)
		require.Equal(t, http.StatusBadRequest, got.status, got.body)
		require.Equal(t, "EVENT_RETIRED", got.code, got.body)
		require.Contains(t, got.message, retired)
		require.Equal(t, got.headerID, got.requestID, "the error carries the request id")
		var e struct {
			Error struct {
				Details map[string]any `json:"details"`
			} `json:"error"`
		}
		require.NoError(t, json.Unmarshal([]byte(got.body), &e))
		require.Equal(t, retired, e.Error.Details["retired_event"])
		require.Contains(t, e.Error.Details, "replacement")
		require.Nil(t, e.Error.Details["replacement"], "nothing produces a replacement for %s", retired)
		require.Equal(t, supported, e.Error.Details["supported_events"])
	}
	unknown := createWebhookThroughAPI(t, ts, key, agentID, "https://receiver.example/hook", "whsec", "post.created")
	require.Equal(t, http.StatusBadRequest, unknown.status, unknown.body)
	require.Equal(t, "INVALID_EVENT_TYPE", unknown.code, unknown.body)
	require.Contains(t, unknown.body, `"supported_events":["post.approved"`)

	var stored int
	require.NoError(t, pool.QueryRow(context.Background(), `SELECT count(*) FROM webhooks WHERE agent_id = $1`, agentID).Scan(&stored))
	require.Zero(t, stored, "a refused subscription stores nothing")
}

func TestWebhooks_OnlyTheAgentAndItsOwnerManageItsWebhooks(t *testing.T) {
	ts, _, pool := newStatusContractServer(t)
	agentID, key := gateAgent(t, ts, pool)
	_, otherKey := gateAgent(t, ts, pool)
	_, humanJWT := createLiveTestUser(t, pool, models.UserRoleUser)

	created := createWebhookThroughAPI(t, ts, key, agentID, "https://receiver.example/hook", "whsec", "post.approved")
	require.Equal(t, http.StatusCreated, created.status, created.body)
	var hook struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal([]byte(created.body), &hook))
	one := "/v1/agents/" + agentID + "/webhooks/" + hook.Data.ID

	for who, bearer := range map[string]string{"another agent": otherKey, "a human who does not own it": humanJWT} {
		for _, c := range []struct{ method, path, body string }{
			{http.MethodPost, "/v1/agents/" + agentID + "/webhooks", `{"url":"https://x.example/h","events":["post.approved"],"secret":"s"}`},
			{http.MethodGet, "/v1/agents/" + agentID + "/webhooks", ""},
			{http.MethodGet, one, ""},
			{http.MethodPatch, one, `{"status":"paused"}`},
			{http.MethodDelete, one, ""},
		} {
			got, err := callStatusContract(http.DefaultClient, c.method, ts.URL+c.path, bearer, c.body)
			require.NoError(t, err)
			require.Equal(t, http.StatusForbidden, got.status, "%s: %s %s: %s", who, c.method, c.path, got.body)
		}
	}
	anonymous, err := callStatusContract(http.DefaultClient, http.MethodGet, ts.URL+"/v1/agents/"+agentID+"/webhooks", "", "")
	require.NoError(t, err)
	require.Equal(t, http.StatusUnauthorized, anonymous.status, anonymous.body)

	paused, err := callStatusContract(http.DefaultClient, http.MethodPatch, ts.URL+one, key, `{"status":"paused","events":["reply.flagged"]}`)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, paused.status, paused.body)
	require.Contains(t, paused.body, `"status":"paused"`)
	deleted, err := callStatusContract(http.DefaultClient, http.MethodDelete, ts.URL+one, key, "")
	require.NoError(t, err)
	require.Equal(t, http.StatusNoContent, deleted.status, deleted.body)
}

// The discovery document advertises webhooks only because they are served: the CRUD
// routes are mounted and the sources schedule the delivery job.
func TestWebhooks_DiscoveryAdvertisesServedAndDeliveredWebhooks(t *testing.T) {
	ts, router, _ := newStatusContractServer(t)
	got, err := callStatusContract(http.DefaultClient, http.MethodGet, ts.URL+"/.well-known/ai-agent.json", "", "")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, got.status)
	var discovery struct {
		Capabilities []string `json:"capabilities"`
	}
	require.NoError(t, json.Unmarshal([]byte(got.body), &discovery))
	require.Contains(t, discovery.Capabilities, "webhooks")

	served := map[string]bool{}
	require.NoError(t, chi.Walk(router, func(method, pattern string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		served[method+" "+strings.TrimSuffix(pattern, "/")] = true
		return nil
	}))
	for _, route := range []string{"POST /v1/agents/{id}/webhooks", "GET /v1/agents/{id}/webhooks",
		"GET /v1/agents/{id}/webhooks/{wh_id}", "PATCH /v1/agents/{id}/webhooks/{wh_id}", "DELETE /v1/agents/{id}/webhooks/{wh_id}"} {
		require.True(t, served[route], "%s is served", route)
	}
	src, err := os.ReadFile("../../cmd/api/main.go")
	require.NoError(t, err)
	require.Contains(t, string(src), "NewWebhookDeliveryJob(", "cmd/api/main.go schedules the delivery job")
}
