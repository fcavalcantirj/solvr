package api

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Optional account auth rejects a presented credential it cannot validate (idx 74 slice 19),
// but a per-agent room token (solvr_rt_) is no account credential: the room guard resolves
// it. This pins, through the real router, that optional account auth never judges one (a
// room agent sends it on every call, search included), that the guard still does (401
// unknown, 403 another room), and that account credentials stay strict.
func TestRoomTokens_AreJudgedByTheRoomGuardNotByAccountAuth(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	roomPreCleanup(t, pool)

	_, jwt := createRoomTestUser(t, pool)
	slug := entriesTestRoom(t, ts.URL, jwt, false)
	otherSlug := entriesTestRoom(t, ts.URL, jwt, false)
	_, agentKey := registerRoomTestAgent(t, ts)
	roomTok := handshakeRoomToken(t, ts, slug, agentKey)
	entriesURL := ts.URL + "/v1/rooms/" + slug + "/entries"

	t.Run("a valid room token writes and reads its room", func(t *testing.T) {
		status, out := doJSON(t, "POST", entriesURL, roomTok, `{"body":"hello from a room token"}`)
		require.Equal(t, http.StatusCreated, status, "write: %v", out)
		for _, path := range []string{"/entries", "/messages", "/agents", "/connect", "/posts", ""} {
			status, out = doJSON(t, "GET", ts.URL+"/v1/rooms/"+slug+path, roomTok, "")
			assert.Equal(t, http.StatusOK, status, "GET /v1/rooms/{slug}%s: %v", path, out)
		}
	})

	t.Run("a room token is anonymous on optional routes outside the room layer", func(t *testing.T) {
		for _, path := range []string{"/v1/search?q=room-token-optional-auth", "/v1/posts?per_page=1"} {
			status, out := doJSON(t, "GET", ts.URL+path, roomTok, "")
			assert.Equal(t, http.StatusOK, status, "GET %s: %v", path, out)
		}
	})

	t.Run("an unknown room token is refused by the room guard", func(t *testing.T) {
		status, out := doJSON(t, "GET", entriesURL, "solvr_rt_thisTokenWasNeverIssued", "")
		assert.Equal(t, http.StatusUnauthorized, status, "%v", out)
		assert.Equal(t, "UNAUTHORIZED", entryErrorCode(out))
	})

	t.Run("a room token of another room is forbidden", func(t *testing.T) {
		status, out := doJSON(t, "GET", ts.URL+"/v1/rooms/"+otherSlug+"/entries", roomTok, "")
		assert.Equal(t, http.StatusForbidden, status, "%v", out)
		assert.Equal(t, "FORBIDDEN", entryErrorCode(out))
	})

	t.Run("an account credential that does not validate is still 401", func(t *testing.T) {
		status, out := doJSON(t, "GET", entriesURL, "solvr_notARealAgentKey", "")
		assert.Equal(t, http.StatusUnauthorized, status, "%v", out)
		assert.Equal(t, "INVALID_API_KEY", entryErrorCode(out))
	})
}
