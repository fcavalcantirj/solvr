package api

import (
	"net/http"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Message reads as adapters over the canonical timeline (task: "Offer one canonical room
// API with thin adapters for the existing transport routes"). GET /v1/rooms/{slug}/messages
// and GET /v1/rooms/{slug}/messages/{id} read the message entries of the same timeline as
// GET /v1/rooms/{slug}/entries?kind=message, and decide access through the same policy.

// messageListRows lists a message route and returns its rows (id, author, content).
func messageListRows(t *testing.T, url, bearer string) []map[string]any {
	t.Helper()
	status, out := doJSON(t, "GET", url, bearer, "")
	require.Equal(t, http.StatusOK, status, "list %s: %v", url, out)
	data, ok := out["data"].([]any)
	require.True(t, ok, "list data: %v", out)
	rows := make([]map[string]any, 0, len(data))
	for _, raw := range data {
		rows = append(rows, raw.(map[string]any))
	}
	return rows
}

func TestRoomEntriesMessages_ListIsAnAdapterOverEntries(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	roomPreCleanup(t, pool)

	_, ownerJWT := createRoomTestUser(t, pool)
	slug := entriesTestRoom(t, ts.URL, ownerJWT, true)
	memberID, memberKey := registerRoomTestAgent(t, ts)
	status, out := doJSON(t, "POST", ts.URL+"/v1/rooms/"+slug+"/members", ownerJWT, `{"agent_id":"`+memberID+`"}`)
	require.Equal(t, http.StatusCreated, status, "owner admits member: %v", out)
	memberTok := handshakeRoomToken(t, ts, slug, memberKey)

	status, out = doJSON(t, "POST", ts.URL+"/v1/rooms/"+slug+"/entries", ownerJWT, `{"body":"human first"}`)
	require.Equal(t, http.StatusCreated, status, "canonical human message: %v", out)
	humanMsg := entryID(t, out)
	status, out = doJSON(t, "POST", ts.URL+"/v1/rooms/"+slug+"/entries", memberTok,
		`{"kind":"event","event_type":"CLAIM","issue":"ISSUE-5"}`)
	require.Equal(t, http.StatusCreated, status, "canonical event: %v", out)
	eventID := entryID(t, out)
	agentMsg := postRoomMessage(t, ts.URL, slug, memberTok, "w1", "agent second")

	entries := messageListRows(t, ts.URL+"/v1/rooms/"+slug+"/entries?kind=message", memberKey)
	require.Len(t, entries, 2, "entries?kind=message")

	lists := map[string][]map[string]any{
		"canonical /messages (owner JWT)":  messageListRows(t, ts.URL+"/v1/rooms/"+slug+"/messages", ownerJWT),
		"canonical /messages (agent key)":  messageListRows(t, ts.URL+"/v1/rooms/"+slug+"/messages", memberKey),
		"canonical /messages (room token)": messageListRows(t, ts.URL+"/v1/rooms/"+slug+"/messages", memberTok),
		"/r adapter /messages":             messageListRows(t, ts.URL+"/r/"+slug+"/messages", memberTok),
		"canonical /messages?after":        messageListRows(t, ts.URL+"/v1/rooms/"+slug+"/messages?after=1", memberKey),
	}
	for name, rows := range lists {
		require.Len(t, rows, 2, "%s: messages only, no event", name)
		for i, row := range rows {
			assert.Equal(t, entries[i]["id"], row["id"], "%s[%d] id", name, i)
			assert.Equal(t, entries[i]["author_type"], row["author_type"], "%s[%d] author_type", name, i)
			assert.Equal(t, entries[i]["author_id"], row["author_id"], "%s[%d] author_id", name, i)
			assert.Equal(t, entries[i]["body"], row["content"], "%s[%d] body/content", name, i)
		}
	}
	assert.Equal(t, float64(humanMsg), entries[0]["id"])
	assert.Equal(t, float64(agentMsg), entries[1]["id"])

	// ?before pages back over the same timeline.
	before := messageListRows(t, ts.URL+"/v1/rooms/"+slug+"/messages?before="+strconv.FormatInt(agentMsg, 10), memberKey)
	require.Len(t, before, 1)
	assert.Equal(t, float64(humanMsg), before[0]["id"])

	// Single-message lookup uses the entry id; an event entry id is not a message.
	for _, base := range []string{ts.URL + "/v1/rooms/" + slug, ts.URL + "/r/" + slug} {
		status, out = doJSON(t, "GET", base+"/messages/"+strconv.FormatInt(agentMsg, 10), memberTok, "")
		require.Equal(t, http.StatusOK, status, "%s single message: %v", base, out)
		assert.Equal(t, entries[1]["body"], entryData(t, out)["content"])
		status, _ = doJSON(t, "GET", base+"/messages/"+strconv.FormatInt(eventID, 10), memberTok, "")
		assert.Equal(t, http.StatusNotFound, status, "%s: event id through /messages/{id}", base)
	}
}

func TestRoomEntriesMessages_ReadsFollowTheOnePolicy(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	roomPreCleanup(t, pool)

	_, ownerJWT := createRoomTestUser(t, pool)
	_, outsiderJWT := createRoomTestUser(t, pool)
	private := entriesTestRoom(t, ts.URL, ownerJWT, true)
	public := entriesTestRoom(t, ts.URL, ownerJWT, false)

	memberID, memberKey := registerRoomTestAgent(t, ts)
	status, out := doJSON(t, "POST", ts.URL+"/v1/rooms/"+private+"/members", ownerJWT, `{"agent_id":"`+memberID+`"}`)
	require.Equal(t, http.StatusCreated, status, "owner admits member: %v", out)
	memberTok := handshakeRoomToken(t, ts, private, memberKey)
	_, outsiderKey := registerRoomTestAgent(t, ts)
	publicTok := handshakeRoomToken(t, ts, public, outsiderKey)
	const invalidTok = "solvr_rt_notarealtoken"

	privMsg := postRoomMessage(t, ts.URL, private, memberTok, "w1", "private hello")
	pubMsg := postRoomMessage(t, ts.URL, public, publicTok, "w2", "public hello")

	credentials := []struct {
		name   string
		room   string
		bearer string
	}{
		{"anonymous on private", private, ""},
		{"outsider agent key on private", private, outsiderKey},
		{"outsider human on private", private, outsiderJWT},
		{"other room's token on private", private, publicTok},
		{"invalid room token on private", private, invalidTok},
		{"member room token", private, memberTok},
		{"member agent key", private, memberKey},
		{"owner human JWT", private, ownerJWT},
		{"anonymous on public", public, ""},
		{"public room's own token", public, publicTok},
		{"other room's token on public", public, memberTok},
		{"invalid room token on public", public, invalidTok},
	}
	for _, c := range credentials {
		msgID := privMsg
		if c.room == public {
			msgID = pubMsg
		}
		base := ts.URL + "/v1/rooms/" + c.room
		want := getStatus(t, base+"/entries", c.bearer)
		assert.Equal(t, want, getStatus(t, base+"/messages", c.bearer), "%s: /messages must decide like /entries", c.name)
		assert.Equal(t, want, getStatus(t, base+"/messages/"+strconv.FormatInt(msgID, 10), c.bearer),
			"%s: /messages/{id} must decide like /entries", c.name)
	}

	// Pin the decisions themselves, so "equal" cannot mean "equally wrong".
	for _, c := range []struct {
		name   string
		url    string
		bearer string
		want   int
	}{
		{"private anonymous", ts.URL + "/v1/rooms/" + private + "/messages", "", http.StatusForbidden},
		{"private invalid token", ts.URL + "/v1/rooms/" + private + "/messages", invalidTok, http.StatusUnauthorized},
		{"private other room's token", ts.URL + "/v1/rooms/" + private + "/messages", publicTok, http.StatusForbidden},
		{"private member token", ts.URL + "/v1/rooms/" + private + "/messages", memberTok, http.StatusOK},
		{"public anonymous", ts.URL + "/v1/rooms/" + public + "/messages", "", http.StatusOK},
		{"public other room's token", ts.URL + "/v1/rooms/" + public + "/messages", memberTok, http.StatusForbidden},
		{"public invalid token", ts.URL + "/v1/rooms/" + public + "/messages", invalidTok, http.StatusUnauthorized},
		{"unknown room", ts.URL + "/v1/rooms/no-such-room-xyz/messages", "", http.StatusNotFound},
		{"adapter other room's token", ts.URL + "/r/" + private + "/messages", publicTok, http.StatusForbidden},
		{"adapter invalid token", ts.URL + "/r/" + private + "/messages", invalidTok, http.StatusUnauthorized},
		{"adapter member token", ts.URL + "/r/" + private + "/messages", memberTok, http.StatusOK},
	} {
		assert.Equal(t, c.want, getStatus(t, c.url, c.bearer), c.name)
	}
}
