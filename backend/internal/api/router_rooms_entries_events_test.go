package api

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Typed coordination events through the canonical entries contract (task: "Offer one
// canonical room API with thin adapters for the existing transport routes"). POST
// /v1/rooms/{slug}/entries with kind=event and the legacy POST /r/{slug}/events adapter
// share ONE event submission path: same attribution, storage, validation, idempotency
// and rate-limit accounting. GET /r/{slug}/events reads the same timeline.

func TestRoomEntries_EventsShareOneSubmissionPath(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	roomPreCleanup(t, pool)
	ctx := context.Background()

	_, jwt := createRoomTestUser(t, pool)
	slug := entriesTestRoom(t, ts.URL, jwt, false)
	agentID, agentKey := registerRoomTestAgent(t, ts)
	roomTok := handshakeRoomToken(t, ts, slug, agentKey)
	entriesURL := ts.URL + "/v1/rooms/" + slug + "/entries"
	eventsURL := ts.URL + "/r/" + slug + "/events"

	storedWithKey := func(key string) int {
		var n int
		require.NoError(t, pool.QueryRow(ctx, `SELECT COUNT(*) FROM room_entries e JOIN rooms r ON r.id = e.room_id
			WHERE r.slug = $1 AND e.kind = 'event' AND e.author_id = $2 AND e.client_entry_id = $3`,
			slug, agentID, key).Scan(&n))
		return n
	}

	// Canonical event write with the room-scoped credential.
	status, out := doJSON(t, "POST", entriesURL, roomTok,
		`{"kind":"event","event_type":"CLAIM","issue":"APP-1","extension":{"pr":7},"client_entry_id":"e1"}`)
	require.Equal(t, http.StatusCreated, status, "canonical event write: %v", out)
	canonical := entryData(t, out)
	assert.Equal(t, "event", canonical["kind"])
	assert.Equal(t, "agent", canonical["author_type"])
	assert.Equal(t, agentID, canonical["author_id"])
	assert.Equal(t, "CLAIM", canonical["event_type"])
	assert.Equal(t, "APP-1", canonical["issue"])
	assert.Equal(t, map[string]any{"pr": float64(7)}, canonical["extension"])
	firstID := entryID(t, out)

	// Idempotent retry: same entry, one stored row.
	status, out = doJSON(t, "POST", entriesURL, roomTok,
		`{"kind":"event","event_type":"CLAIM","issue":"APP-1","client_entry_id":"e1"}`)
	require.Equal(t, http.StatusOK, status, "canonical event retry: %v", out)
	assert.Equal(t, firstID, entryID(t, out))
	meta, _ := out["meta"].(map[string]any)
	assert.Equal(t, true, meta["idempotent_replay"])
	assert.Equal(t, 1, storedWithKey("e1"))

	// Adapter write keeps its legacy response shape but is attributed to the token's
	// agent and stored in the same timeline, readable through the canonical route.
	status, out = doJSON(t, "POST", eventsURL, roomTok,
		`{"type":"BUILDING","issue":"APP-1","actor":"w1","payload":{"step":2},"client_entry_id":"e2"}`)
	require.Equal(t, http.StatusCreated, status, "adapter event write: %v", out)
	legacy := entryData(t, out)
	assert.Equal(t, "BUILDING", legacy["type"])
	assert.Equal(t, "w1", legacy["actor"])
	assert.Equal(t, "APP-1", legacy["issue"])
	assert.Equal(t, map[string]any{"step": float64(2)}, legacy["payload"])
	adapterID := entryID(t, out)

	status, out = doJSON(t, "GET", entriesURL+"/"+strconv.FormatInt(adapterID, 10), "", "")
	require.Equal(t, http.StatusOK, status, "canonical read of adapter event: %v", out)
	adapterEntry := entryData(t, out)
	assert.Equal(t, "event", adapterEntry["kind"])
	assert.Equal(t, canonical["author_type"], adapterEntry["author_type"])
	assert.Equal(t, canonical["author_id"], adapterEntry["author_id"])
	assert.Equal(t, "w1", adapterEntry["actor_label"])

	// Adapter retry and cross-path replay return the one stored entry.
	status, out = doJSON(t, "POST", eventsURL, roomTok,
		`{"type":"BUILDING","issue":"APP-1","actor":"w1","client_entry_id":"e2"}`)
	require.Equal(t, http.StatusOK, status, "adapter event retry: %v", out)
	assert.Equal(t, adapterID, entryID(t, out))
	assert.Equal(t, true, out["idempotent_replay"])
	status, out = doJSON(t, "POST", entriesURL, roomTok,
		`{"kind":"event","event_type":"BUILDING","client_entry_id":"e2"}`)
	require.Equal(t, http.StatusOK, status, "cross-path event replay: %v", out)
	assert.Equal(t, adapterID, entryID(t, out))
	assert.Equal(t, 1, storedWithKey("e2"))

	// The same agent through its account credential: identical attribution.
	status, out = doJSON(t, "POST", entriesURL, agentKey, `{"kind":"event","event_type":"CLAIM","issue":"APP-2"}`)
	require.Equal(t, http.StatusCreated, status, "account-key event write: %v", out)
	assert.Equal(t, agentID, entryData(t, out)["author_id"])

	// The legacy list adapter reads the same stored events, newest first, legacy shape.
	status, out = doJSON(t, "GET", eventsURL, roomTok, "")
	require.Equal(t, http.StatusOK, status, "legacy list: %v", out)
	listed, _ := out["data"].([]any)
	require.Len(t, listed, 3)
	newest := listed[0].(map[string]any)
	assert.Equal(t, "CLAIM", newest["type"])
	assert.Equal(t, "APP-2", newest["issue"])
	status, out = doJSON(t, "GET", eventsURL+"?type=CLAIM", roomTok, "")
	require.Equal(t, http.StatusOK, status)
	assert.Len(t, out["data"], 2)
	status, out = doJSON(t, "GET", eventsURL+"?issue=APP-1&type=BUILDING", roomTok, "")
	require.Equal(t, http.StatusOK, status)
	assert.Len(t, out["data"], 1)
	status, out = doJSON(t, "GET", entriesURL+"?kind=event", "", "")
	require.Equal(t, http.StatusOK, status)
	assert.Len(t, out["data"], 3)

	// Events are not messages: the message counter is untouched.
	var messages int
	require.NoError(t, pool.QueryRow(ctx, `SELECT message_count FROM rooms WHERE slug = $1`, slug).Scan(&messages))
	assert.Equal(t, 0, messages)

	// Shared validation: the same refusal through either route.
	big := `{"k":"` + strings.Repeat("x", 16400) + `"}`
	cases := []struct {
		name, url, body string
	}{
		{"canonical missing event_type", entriesURL, `{"kind":"event","issue":"x"}`},
		{"adapter missing type", eventsURL, `{"issue":"x","actor":"w1"}`},
		{"adapter missing actor", eventsURL, `{"type":"CLAIM"}`},
		{"canonical event_type too long", entriesURL, `{"kind":"event","event_type":"` + strings.Repeat("T", 51) + `"}`},
		{"adapter type too long", eventsURL, `{"type":"` + strings.Repeat("T", 51) + `","actor":"w1"}`},
		{"canonical issue too long", entriesURL, `{"kind":"event","event_type":"CLAIM","issue":"` + strings.Repeat("i", 201) + `"}`},
		{"adapter actor too long", eventsURL, `{"type":"CLAIM","actor":"` + strings.Repeat("a", 201) + `"}`},
		{"canonical extension too large", entriesURL, `{"kind":"event","event_type":"CLAIM","extension":` + big + `}`},
		{"adapter payload too large", eventsURL, `{"type":"CLAIM","actor":"w1","payload":` + big + `}`},
		{"canonical unknown kind", entriesURL, `{"kind":"reaction","body":"x"}`},
	}
	for _, c := range cases {
		status, out := doJSON(t, "POST", c.url, roomTok, c.body)
		assert.Equal(t, http.StatusBadRequest, status, "%s: %v", c.name, out)
		assert.Equal(t, "VALIDATION_ERROR", entryErrorCode(out), c.name)
	}
}

func TestRoomEntries_EventAccessDecisionsMatchAcrossPaths(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	roomPreCleanup(t, pool)

	_, ownerJWT := createRoomTestUser(t, pool)
	private := entriesTestRoom(t, ts.URL, ownerJWT, true)
	public := entriesTestRoom(t, ts.URL, ownerJWT, false)
	_, outsiderKey := registerRoomTestAgent(t, ts)
	publicTok := handshakeRoomToken(t, ts, public, outsiderKey) // valid, but for ANOTHER room

	canonical := ts.URL + "/v1/rooms/" + private + "/entries"
	adapter := ts.URL + "/r/" + private + "/events"
	eventBody := `{"kind":"event","event_type":"CLAIM"}`
	legacyBody := `{"type":"CLAIM","actor":"w1"}`

	for _, c := range []struct {
		name, url, bearer, body string
		want                    int
	}{
		{"other room's token canonical", canonical, publicTok, eventBody, http.StatusForbidden},
		{"other room's token adapter", adapter, publicTok, legacyBody, http.StatusForbidden},
		{"outsider agent key canonical", canonical, outsiderKey, eventBody, http.StatusForbidden},
		{"anonymous canonical", canonical, "", eventBody, http.StatusUnauthorized},
		{"anonymous adapter", adapter, "", legacyBody, http.StatusUnauthorized},
	} {
		status, out := doJSON(t, "POST", c.url, c.bearer, c.body)
		assert.Equal(t, c.want, status, "%s: %v", c.name, out)
	}
}

// Agent writes of messages and events count against ONE per-IP bucket, whichever route
// (canonical or adapter) carries them.
func TestRoomEntries_AgentEventAndMessageWritesShareOneRateLimit(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	roomPreCleanup(t, pool)

	_, jwt := createRoomTestUser(t, pool)
	slug := entriesTestRoom(t, ts.URL, jwt, false)
	_, agentKey := registerRoomTestAgent(t, ts)
	roomTok := handshakeRoomToken(t, ts, slug, agentKey)

	for i := 0; i < 30; i++ {
		status, out := doJSON(t, "POST", ts.URL+"/r/"+slug+"/message", roomTok,
			fmt.Sprintf(`{"agent_name":"a","content":"m%d"}`, i))
		require.Equal(t, http.StatusCreated, status, "message %d: %v", i, out)
	}
	for i := 0; i < 30; i++ {
		status, out := doJSON(t, "POST", ts.URL+"/v1/rooms/"+slug+"/entries", roomTok,
			fmt.Sprintf(`{"kind":"event","event_type":"TICK","issue":"%d"}`, i))
		require.Equal(t, http.StatusCreated, status, "event %d: %v", i, out)
	}
	status, _ := doJSON(t, "POST", ts.URL+"/r/"+slug+"/events", roomTok, `{"type":"TICK","actor":"a"}`)
	assert.Equal(t, http.StatusTooManyRequests, status, "61st agent write through the event adapter")
}
