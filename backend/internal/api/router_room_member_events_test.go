package api

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// Task "Keep SDKs, CLI, MCP, skills, and webhooks consistent with the redesigned product",
// step 4, room events, through the real router: when a room owner admits an agent to a room
// or removes it, the agent is notified with an event that names the canonical room
// (subject.room_id, schema version 2) — in GET /v1/notifications, in its /v1/me inbox and at
// the webhooks it subscribed to the event. A retried admission is not a second event, and an
// agent joining a room by itself is not told what it did.

// memberEventAgent registers an agent with its own name prefix (other room tests delete
// every "roomtest_" agent) and removes it and its notifications when the test ends.
func memberEventAgent(t *testing.T, ts *httptest.Server, pool *db.Pool) (agentID, key string) {
	t.Helper()
	name := fmt.Sprintf("rmev_%s", strings.ReplaceAll(uuid.NewString()[:12], "-", ""))
	status, out := doJSON(t, http.MethodPost, ts.URL+"/v1/agents/register", "",
		`{"name":"`+name+`","description":"room member event test agent"}`)
	require.Equal(t, http.StatusCreated, status, "register: %v", out)
	agent, _ := out["agent"].(map[string]any)
	agentID, _ = agent["id"].(string)
	key, _ = out["api_key"].(string)
	require.NotEmpty(t, agentID, "%v", out)
	require.NotEmpty(t, key)
	t.Cleanup(func() {
		ctx := context.Background()
		pool.Exec(ctx, "DELETE FROM notifications WHERE agent_id = $1", agentID) //nolint:errcheck
		pool.Exec(ctx, "DELETE FROM claim_tokens WHERE agent_id = $1", agentID)  //nolint:errcheck
		pool.Exec(ctx, "DELETE FROM agents WHERE id = $1", agentID)              //nolint:errcheck
	})
	return agentID, key
}

// memberEventRoom creates a room as bearer and returns its slug and id, deleting it when the
// test ends.
func memberEventRoom(t *testing.T, ts *httptest.Server, pool *db.Pool, bearer string, private bool) (slug, roomID string) {
	t.Helper()
	slug = "rmev-" + uuid.NewString()[:8]
	status, out := doJSON(t, http.MethodPost, ts.URL+"/v1/rooms", bearer,
		fmt.Sprintf(`{"display_name":"Member events %s","slug":"%s","is_private":%t}`, slug, slug, private))
	require.Equal(t, http.StatusCreated, status, "create room: %v", out)
	data, _ := out["data"].(map[string]any)
	roomID, _ = data["id"].(string)
	require.NotEmpty(t, roomID, "%v", out)
	t.Cleanup(func() {
		pool.Exec(context.Background(), "DELETE FROM rooms WHERE slug = $1", slug) //nolint:errcheck
	})
	return slug, roomID
}

// roomEventsOf returns the agent's notifications of eventType from GET /v1/notifications, as
// answered, each checked against the published Notification schema.
func roomEventsOf(t *testing.T, ts *httptest.Server, key, eventType string) []map[string]any {
	t.Helper()
	got, err := callStatusContract(http.DefaultClient, http.MethodGet, ts.URL+"/v1/notifications?per_page=50&type="+eventType, key, "")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, got.status, got.body)
	var listed struct {
		Data []map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal([]byte(got.body), &listed), got.body)
	spec := servedSpec(t)
	for i, n := range listed.Data {
		require.Empty(t, schemaProblems(spec, n, map[string]interface{}{"$ref": "#/components/schemas/Notification"},
			fmt.Sprintf("notification[%d]", i)), "the answer is the published Notification")
	}
	return listed.Data
}

// requireRoomEvent checks one notification names the room under schema version 2.
func requireRoomEvent(t *testing.T, n map[string]any, eventType, roomID, slug string) {
	t.Helper()
	require.Equal(t, eventType, n["type"], "%v", n)
	require.Equal(t, float64(models.NotificationRoomSchemaVersion), n["schema_version"], "%v", n)
	require.Equal(t, map[string]any{"room_id": roomID}, n["subject"], "the subject names the canonical room only")
	require.Equal(t, "/rooms/"+slug, n["link"], "%v", n)
	require.NotEmpty(t, n["title"])
}

func TestRoomMemberEvents_TheAdmittedAndRemovedAgentIsNotifiedAndItsWebhookDelivered(t *testing.T) {
	ts, _, pool := newStatusContractServer(t)
	_, ownerKey := memberEventAgent(t, ts, pool)
	memberID, memberKey := memberEventAgent(t, ts, pool)
	slug, roomID := memberEventRoom(t, ts, pool, ownerKey, true)
	rcv := newWebhookReceiver(t)
	const secret = "whsec-room-member-events"
	hook := createWebhookThroughAPI(t, ts, memberKey, memberID, rcv.URL, secret,
		models.NotificationRoomMemberAdded, models.NotificationRoomMemberRemoved)
	require.Equal(t, http.StatusCreated, hook.status, "an agent subscribes to the room events: %s", hook.body)

	members := ts.URL + "/v1/rooms/" + slug + "/members"
	admit := func() {
		t.Helper()
		status, out := doJSON(t, http.MethodPost, members, ownerKey, `{"agent_id":"`+memberID+`"}`)
		require.Equal(t, http.StatusCreated, status, "admit: %v", out)
		data, _ := out["data"].(map[string]any)
		require.NotContains(t, data, "admitted", "the membership answer is unchanged")
	}
	admit()
	admit() // a retry of the same admission

	added := roomEventsOf(t, ts, memberKey, models.NotificationRoomMemberAdded)
	require.Len(t, added, 1, "one admission, one event: %v", added)
	requireRoomEvent(t, added[0], models.NotificationRoomMemberAdded, roomID, slug)
	require.Empty(t, roomEventsOf(t, ts, ownerKey, models.NotificationRoomMemberAdded), "the owner who acted is not notified")

	list, inbox := agentNotificationEvents(t, ts, memberKey)
	for where, events := range map[string][]notificationEvent{"list": list, "inbox": inbox} {
		require.Len(t, events, 1, "%s: %v", where, events)
		require.Equal(t, models.NotificationRoomMemberAdded, events[0].Type, where)
		require.Equal(t, 2, *events[0].SchemaVersion, where)
	}
	me, err := callStatusContract(http.DefaultClient, http.MethodGet, ts.URL+"/v1/me", memberKey, "")
	require.NoError(t, err)
	require.Contains(t, me.body, `"subject":{"room_id":"`+roomID+`"}`, "the /v1/me inbox names the room")

	runDeliveryOnTwoInstances(t, pool, rcv)
	got := rcv.received()
	require.Len(t, got, 1, "the admission is delivered once")

	status, out := doJSON(t, http.MethodDelete, members+"/"+memberID, ownerKey, "")
	require.Equal(t, http.StatusNoContent, status, "remove: %v", out)
	removed := roomEventsOf(t, ts, memberKey, models.NotificationRoomMemberRemoved)
	require.Len(t, removed, 1, "%v", removed)
	requireRoomEvent(t, removed[0], models.NotificationRoomMemberRemoved, roomID, slug)

	runDeliveryOnTwoInstances(t, pool, rcv)
	got = rcv.received()
	require.Len(t, got, 2, "the removal is delivered once")
	require.NotEqual(t, got[0].deliveryID, got[1].deliveryID, "two events, two delivery IDs")

	spec := servedSpec(t)
	deliverySchema := map[string]interface{}{"$ref": "#/components/schemas/WebhookDelivery"}
	for i, want := range []string{models.NotificationRoomMemberAdded, models.NotificationRoomMemberRemoved} {
		r := got[i]
		require.Equal(t, want, r.event)
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write(r.body)
		require.Equal(t, "sha256="+hex.EncodeToString(mac.Sum(nil)), r.signature)
		var payload map[string]any
		require.NoError(t, json.Unmarshal(r.body, &payload), string(r.body))
		require.Empty(t, schemaProblems(spec, payload, deliverySchema, "delivery"), "the delivery is the published WebhookDelivery")
		require.Equal(t, r.deliveryID, payload["id"])
		require.Equal(t, want, payload["event"])
		require.Equal(t, float64(2), payload["schema_version"])
		data, _ := payload["data"].(map[string]any)
		require.Equal(t, memberID, data["agent_id"])
		require.Equal(t, map[string]any{"room_id": roomID}, data["subject"], "the delivery names the canonical room")
	}

	admit() // a readmission is an admission again
	require.Len(t, roomEventsOf(t, ts, memberKey, models.NotificationRoomMemberAdded), 2)

	// A role change of an active member is not an admission, and a member that leaves by
	// itself is not told what it did.
	status, out = doJSON(t, http.MethodPost, members, ownerKey, `{"agent_id":"`+memberID+`","role":"owner"}`)
	require.Equal(t, http.StatusCreated, status, "promote: %v", out)
	require.Len(t, roomEventsOf(t, ts, memberKey, models.NotificationRoomMemberAdded), 2, "a promotion is not an admission")
	status, out = doJSON(t, http.MethodDelete, members+"/"+memberID, memberKey, "")
	require.Equal(t, http.StatusNoContent, status, "the promoted member removes itself: %v", out)
	require.Len(t, roomEventsOf(t, ts, memberKey, models.NotificationRoomMemberRemoved), 1, "leaving by itself is not an event")
}

func TestRoomMemberEvents_AHumanOwnersAdmissionNotifiesTheAgentAndASelfJoinNotifiesNobody(t *testing.T) {
	ts, _, pool := newStatusContractServer(t)
	_, jwt := createLiveTestUser(t, pool, models.UserRoleUser)
	agentID, agentKey := memberEventAgent(t, ts, pool)

	slug, roomID := memberEventRoom(t, ts, pool, jwt, true)
	status, out := doJSON(t, http.MethodPost, ts.URL+"/v1/rooms/"+slug+"/members", jwt, `{"agent_id":"`+agentID+`"}`)
	require.Equal(t, http.StatusCreated, status, "a human owner admits the agent: %v", out)
	added := roomEventsOf(t, ts, agentKey, models.NotificationRoomMemberAdded)
	require.Len(t, added, 1)
	requireRoomEvent(t, added[0], models.NotificationRoomMemberAdded, roomID, slug)
	hs, _ := handshake(t, ts.URL, slug, agentKey, "")
	require.Equal(t, http.StatusCreated, hs, "the admitted agent can join")
	require.Len(t, roomEventsOf(t, ts, agentKey, models.NotificationRoomMemberAdded), 1, "joining after admission is not another event")

	joinerID, joinerKey := memberEventAgent(t, ts, pool)
	_, ownerKey := memberEventAgent(t, ts, pool)
	public, _ := memberEventRoom(t, ts, pool, ownerKey, false)
	hs, _ = handshake(t, ts.URL, public, joinerKey, "")
	require.Equal(t, http.StatusCreated, hs, "a public room is open to any agent")
	var joined bool
	require.NoError(t, pool.QueryRow(context.Background(), `SELECT EXISTS (SELECT 1 FROM room_members m JOIN rooms r ON r.id = m.room_id
		WHERE r.slug = $1 AND m.agent_id = $2 AND m.revoked_at IS NULL)`, public, joinerID).Scan(&joined))
	require.True(t, joined, "the handshake made the joiner a member")

	require.Empty(t, roomEventsOf(t, ts, joinerKey, models.NotificationRoomMemberAdded), "an agent joining by itself is not notified")
}
