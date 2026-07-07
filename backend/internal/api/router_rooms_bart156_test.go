package api

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// postHumanMessage posts a human comment to a room with the given bearer and returns the status.
func postHumanMessage(t *testing.T, url, bearer, content string) int {
	t.Helper()
	req, _ := http.NewRequest("POST", url, strings.NewReader(`{"content":"`+content+`"}`))
	req.Header.Set("Content-Type", "application/json")
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	io.Copy(io.Discard, resp.Body) //nolint:errcheck
	resp.Body.Close()
	return resp.StatusCode
}

// TestRoomSSE_AccessTokenQueryParam_AuthorizesPrivateStream proves the BART-156 SSE fix:
// a browser EventSource (no Authorization header) can authenticate a private room's stream
// via ?access_token= — with the human owner's JWT or a room bearer token — while an
// anonymous stream stays 403.
func TestRoomSSE_AccessTokenQueryParam_AuthorizesPrivateStream(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	roomPreCleanup(t, pool)

	_, ownerJWT := createRoomTestUser(t, pool)
	slug, roomToken := createClosedRoom(t, ts, ownerJWT)
	streamURL := ts.URL + "/v1/rooms/" + slug + "/stream"

	// Anonymous EventSource (no header, no query token) → 403 on a closed room.
	require.Equal(t, http.StatusForbidden, getStatus(t, streamURL, ""), "anon stream should be 403")

	// ?access_token=<owner JWT> → the owner path in RoomAccessGuard authorizes the stream.
	require.Equal(t, http.StatusOK, getStatus(t, streamURL+"?access_token="+ownerJWT, ""), "owner JWT via ?access_token should stream (200)")

	// ?access_token=<room bearer token> → also authorized (parity with the header path).
	require.Equal(t, http.StatusOK, getStatus(t, streamURL+"?access_token="+roomToken, ""), "room token via ?access_token should stream (200)")

	// A present Authorization header still wins / works unchanged.
	require.Equal(t, http.StatusOK, getStatus(t, streamURL, ownerJWT), "owner JWT via header should stream (200)")
}

// TestRoomHumanPost_PrivateRoom_MembersOnly proves the BART-156 POST gate: only the
// owner/family/members may post to a PRIVATE room, while a PUBLIC room stays open to any
// authenticated human.
func TestRoomHumanPost_PrivateRoom_MembersOnly(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	roomPreCleanup(t, pool)

	_, ownerJWT := createRoomTestUser(t, pool)
	_, strangerJWT := createRoomTestUser(t, pool)

	// Private room: owner can post, a non-member human cannot.
	privSlug, _ := createClosedRoom(t, ts, ownerJWT)
	privMsgURL := ts.URL + "/v1/rooms/" + privSlug + "/messages"
	require.Equal(t, http.StatusCreated, postHumanMessage(t, privMsgURL, ownerJWT, "owner posts"), "owner should post to own private room (201)")
	require.Equal(t, http.StatusForbidden, postHumanMessage(t, privMsgURL, strangerJWT, "stranger posts"), "non-member human should be 403 on a private room")

	// Public room: any authenticated human can post (guard allows public rooms).
	pubSlug, _ := createTestRoomWithToken(t, ts, ownerJWT)
	pubMsgURL := ts.URL + "/v1/rooms/" + pubSlug + "/messages"
	require.Equal(t, http.StatusCreated, postHumanMessage(t, pubMsgURL, strangerJWT, "stranger posts public"), "any authed human should post to a public room (201)")
}
