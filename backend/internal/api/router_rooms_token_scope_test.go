package api

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Room-scoped credentials (solvr_rt_) authorize only their own room's participant
// operations: entry reads/writes, the stream and the /r/{slug} transport. Room management
// and account operations (create, update, delete, archive, reopen, membership, handshake,
// save-as-post) require the human or agent ACCOUNT credential; a room token presented there
// is refused as a credential and changes nothing.
func TestRoomTokenScope_RoomTokenAuthorizesOnlyParticipantOperations(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	roomPreCleanup(t, pool)
	ctx := context.Background()

	_, jwt := createRoomTestUser(t, pool)
	slug := entriesTestRoom(t, ts.URL, jwt, false)
	agentID, agentKey := registerRoomTestAgent(t, ts)
	roomTok := handshakeRoomToken(t, ts, slug, agentKey)
	room := ts.URL + "/v1/rooms/" + slug

	// Permitted: the token is a live participant credential for THIS room.
	status, out := doJSON(t, "POST", room+"/entries", roomTok, `{"body":"scoped write"}`)
	require.Equal(t, http.StatusCreated, status, "room token writes its own room: %v", out)
	status, out = doJSON(t, "GET", room+"/entries", roomTok, "")
	require.Equal(t, http.StatusOK, status, "room token reads its own room: %v", out)

	refused := []struct{ method, url, body string }{
		{"POST", ts.URL + "/v1/rooms", `{"display_name":"Token Made","slug":"` + slug + `-tok"}`},
		{"PATCH", room, `{"display_name":"Renamed By Token"}`},
		{"DELETE", room, ""},
		{"POST", room + "/archive", ""},
		{"POST", room + "/reopen", ""},
		{"GET", room + "/members", ""},
		{"POST", room + "/members", `{"agent_id":"` + agentID + `"}`},
		{"DELETE", room + "/members/" + agentID, ""},
		{"POST", room + "/handshake", ""},
		{"POST", room + "/save-as-post", `{"title":"t","description":"d"}`},
	}
	for _, c := range refused {
		status, out := doJSON(t, c.method, c.url, roomTok, c.body)
		assert.Equal(t, http.StatusUnauthorized, status, "%s %s with a room token: %v", c.method, c.url, out)
	}

	// Nothing changed: same room, same name, not archived, not deleted, member still active,
	// no room created by the token.
	var name string
	var archived, deleted bool
	require.NoError(t, pool.QueryRow(ctx, `SELECT display_name, archived_at IS NOT NULL, deleted_at IS NOT NULL
		FROM rooms WHERE slug = $1`, slug).Scan(&name, &archived, &deleted))
	assert.Equal(t, "Entries "+slug, name)
	assert.False(t, archived)
	assert.False(t, deleted)
	var created int
	require.NoError(t, pool.QueryRow(ctx, `SELECT COUNT(*) FROM rooms WHERE slug = $1`, slug+"-tok").Scan(&created))
	assert.Equal(t, 0, created)

	// The token still works for its room afterwards (membership untouched).
	status, out = doJSON(t, "POST", room+"/entries", roomTok, `{"body":"still a participant"}`)
	assert.Equal(t, http.StatusCreated, status, "room token after refused management calls: %v", out)
}
