package api

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRoomEntries_ClientEntryReuseWithDifferentPayloadIs409 pins the timeline half of the
// retry contract through the real router: a client_entry_id replays only the same write.
// Reusing it for a different payload is 409 CLIENT_ENTRY_ID_REUSED in the standard error
// envelope on the canonical route and on every adapter, and stores or counts nothing.
func TestRoomEntries_ClientEntryReuseWithDifferentPayloadIs409(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	roomPreCleanup(t, pool)
	ctx := context.Background()

	_, jwt := createRoomTestUser(t, pool)
	slug := entriesTestRoom(t, ts.URL, jwt, false)
	agentID, agentKey := registerRoomTestAgent(t, ts)
	roomTok := handshakeRoomToken(t, ts, slug, agentKey)
	entriesURL := ts.URL + "/v1/rooms/" + slug + "/entries"
	messageURL := ts.URL + "/r/" + slug + "/message"
	eventsURL := ts.URL + "/r/" + slug + "/events"

	stored := func() (entries, messages int) {
		require.NoError(t, pool.QueryRow(ctx, `SELECT COUNT(*) FROM room_entries e JOIN rooms r ON r.id = e.room_id
			WHERE r.slug = $1`, slug).Scan(&entries))
		require.NoError(t, pool.QueryRow(ctx, `SELECT message_count FROM rooms WHERE slug = $1`, slug).Scan(&messages))
		return entries, messages
	}
	conflict := func(name, url, body string) {
		t.Helper()
		status, out := doJSON(t, "POST", url, roomTok, body)
		assert.Equal(t, http.StatusConflict, status, "%s: %v", name, out)
		assert.Equal(t, "CLIENT_ENTRY_ID_REUSED", entryErrorCode(out), name)
		e, _ := out["error"].(map[string]any)
		assert.NotEmpty(t, e["request_id"], "%s: conflict carries the standard error envelope: %v", name, out)
	}

	status, out := doJSON(t, "POST", entriesURL, roomTok,
		`{"body":"plan: step one","extension":{"step":1},"client_entry_id":"c1"}`)
	require.Equal(t, http.StatusCreated, status, "first message: %v", out)
	firstID := entryID(t, out)
	status, out = doJSON(t, "POST", entriesURL, roomTok,
		`{"kind":"event","event_type":"CLAIM","issue":"APP-1","extension":{"pr":7},"client_entry_id":"e1"}`)
	require.Equal(t, http.StatusCreated, status, "first event: %v", out)
	eventID := entryID(t, out)

	conflict("canonical message, different body", entriesURL,
		`{"body":"plan: step TWO","extension":{"step":1},"client_entry_id":"c1"}`)
	conflict("canonical message, different extension", entriesURL,
		`{"body":"plan: step one","extension":{"step":2},"client_entry_id":"c1"}`)
	conflict("adapter message, different content", messageURL,
		`{"agent_name":"`+agentID+`","content":"something else","metadata":{"step":1},"client_entry_id":"c1"}`)
	conflict("adapter message, legacy client_message_id", messageURL,
		`{"agent_name":"`+agentID+`","content":"something else","client_message_id":"c1"}`)
	conflict("canonical event, different issue", entriesURL,
		`{"kind":"event","event_type":"CLAIM","issue":"APP-2","extension":{"pr":7},"client_entry_id":"e1"}`)
	conflict("adapter event, different payload", eventsURL,
		`{"type":"CLAIM","issue":"APP-1","actor":"w1","payload":{"pr":8},"client_entry_id":"e1"}`)
	conflict("event key reused for a message", entriesURL, `{"body":"hello","client_entry_id":"e1"}`)
	conflict("message key reused for an event", entriesURL,
		`{"kind":"event","event_type":"CLAIM","client_entry_id":"c1"}`)

	entries, messages := stored()
	assert.Equal(t, 2, entries, "a refused reuse stores nothing")
	assert.Equal(t, 1, messages, "a refused reuse counts nothing")

	// The same write still replays, through any path and with equivalent JSON.
	status, out = doJSON(t, "POST", entriesURL, roomTok,
		`{"body":"plan: step one","extension":{ "step": 1 },"client_entry_id":"c1"}`)
	require.Equal(t, http.StatusOK, status, "identical canonical retry: %v", out)
	assert.Equal(t, firstID, entryID(t, out))
	status, out = doJSON(t, "POST", messageURL, roomTok,
		`{"agent_name":"`+agentID+`","content":"plan: step one","metadata":{"step":1},"client_entry_id":"c1"}`)
	require.Equal(t, http.StatusOK, status, "identical adapter retry: %v", out)
	assert.Equal(t, firstID, entryID(t, out))
	status, out = doJSON(t, "POST", eventsURL, roomTok,
		`{"type":"CLAIM","issue":"APP-1","actor":"w1","payload":{"pr":7},"client_entry_id":"e1"}`)
	require.Equal(t, http.StatusOK, status, "identical adapter event retry: %v", out)
	assert.Equal(t, eventID, entryID(t, out))

	entries, messages = stored()
	assert.Equal(t, 2, entries)
	assert.Equal(t, 1, messages)
}
