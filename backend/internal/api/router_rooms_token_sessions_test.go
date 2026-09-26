package api

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/fcavalcantirj/solvr/internal/db"
)

// idx 75 step 1: two sessions of one agent must not silently invalidate each other by
// following the same instructions. A handshake ADDS a session token; only an explicit
// {"rotate": true} replaces the others, and a replaced token gets a recoverable
// CREDENTIAL_ROTATED answer (REST, /r adapters and an open stream) instead of the bare 401
// a token that never existed gets.

func sessionError(t *testing.T, out map[string]any) (code, message, requestID string) {
	t.Helper()
	e, ok := out["error"].(map[string]any)
	require.True(t, ok, "an error envelope, got %v", out)
	code, _ = e["code"].(string)
	message, _ = e["message"].(string)
	requestID, _ = e["request_id"].(string)
	return code, message, requestID
}

func handshakeBody(t *testing.T, inst *roomInstance, room *accessRoom, agentKey, body string) (int, map[string]any) {
	t.Helper()
	return doJSON(t, "POST", inst.ts.URL+"/v1/rooms/"+room.slug+"/handshake", agentKey, body)
}

func tokenOf(out map[string]any) string {
	data, _ := out["data"].(map[string]any)
	tok, _ := data["room_token"].(string)
	return tok
}

// everyRoomSurface calls one read and one write on the canonical routes and on the /r
// adapters with tok, on instance inst, and returns the status and body of each.
func everyRoomSurface(t *testing.T, inst *roomInstance, room *accessRoom, tok string) map[string]struct {
	Status int
	Out    map[string]any
} {
	t.Helper()
	res := map[string]struct {
		Status int
		Out    map[string]any
	}{}
	add := func(name string, status int, out map[string]any) {
		res[name] = struct {
			Status int
			Out    map[string]any
		}{status, out}
	}
	s, o := doJSON(t, "GET", inst.ts.URL+"/v1/rooms/"+room.slug+"/entries", tok, "")
	add("GET /v1/rooms/{slug}/entries", s, o)
	s, o = doJSON(t, "POST", inst.ts.URL+"/v1/rooms/"+room.slug+"/entries", tok, `{"body":"session write"}`)
	add("POST /v1/rooms/{slug}/entries", s, o)
	s, o = doJSON(t, "GET", inst.ts.URL+"/r/"+room.slug+"/messages", tok, "")
	add("GET /r/{slug}/messages", s, o)
	s, o = doJSON(t, "POST", inst.ts.URL+"/r/"+room.slug+"/message", tok, `{"agent_name":"session","content":"session write"}`)
	add("POST /r/{slug}/message", s, o)
	return res
}

func TestRoomTokenSessions_SecondHandshakeLeavesTheFirstSessionAlone(t *testing.T) {
	opts := RoomRelayOptions{SweepInterval: -1}
	a := startRoomInstance(t, opts)
	b := startRoomInstance(t, opts)
	room := newAccessRoom(t, a, true)

	sessionOne := room.executorTok
	streamOne := openAccessStream(t, room.streamURL(b), sessionOne)
	adapterOne := openAccessStream(t, b.ts.URL+"/r/"+room.slug+"/stream", sessionOne)

	// Session two reads the same instructions and follows them: it handshakes.
	status, out := handshakeBody(t, b, room, room.executorKey, `{}`)
	require.Equal(t, http.StatusCreated, status, "the second session's handshake: %v", out)
	sessionTwo := tokenOf(out)
	require.NotEmpty(t, sessionTwo)
	require.NotEqual(t, sessionOne, sessionTwo, "each handshake issues its own token")
	data := out["data"].(map[string]any)
	require.Equal(t, false, data["rotated"], "a plain handshake replaces nothing and says so")

	for name, tok := range map[string]string{"session one": sessionOne, "session two": sessionTwo} {
		for _, inst := range []*roomInstance{a, b} {
			for surface, res := range everyRoomSurface(t, inst, room, tok) {
				require.Less(t, res.Status, 300, "%s on %s must still be authorized: %v", name, surface, res.Out)
			}
		}
	}
	require.False(t, streamOne.endedWithin(500*time.Millisecond), "session one's open stream keeps running")
	require.False(t, adapterOne.endedWithin(100*time.Millisecond), "session one's /r stream keeps running")
}

func TestRoomTokenSessions_ExplicitRotationEndsTheOtherSessionsWithARecoverableError(t *testing.T) {
	opts := RoomRelayOptions{SweepInterval: -1}
	a := startRoomInstance(t, opts)
	b := startRoomInstance(t, opts)
	room := newAccessRoom(t, a, true)

	sessionOne := room.executorTok
	status, out := handshakeBody(t, a, room, room.executorKey, `{}`)
	require.Equal(t, http.StatusCreated, status, "%v", out)
	sessionTwo := tokenOf(out)

	streamOne := openAccessStream(t, room.streamURL(b), sessionOne)
	adapterOne := openAccessStream(t, b.ts.URL+"/r/"+room.slug+"/stream", sessionOne)
	streamTwo := openAccessStream(t, room.streamURL(b), sessionTwo)
	byKey := openAccessStream(t, room.streamURL(b), room.executorKey)
	planner := openAccessStream(t, room.streamURL(b), room.plannerTok)

	status, out = handshakeBody(t, a, room, room.executorKey, `{"rotate":true}`)
	require.Equal(t, http.StatusCreated, status, "explicit rotation: %v", out)
	rotatedIn := tokenOf(out)
	require.NotEmpty(t, rotatedIn)
	require.Equal(t, true, out["data"].(map[string]any)["rotated"], "the response says earlier tokens were replaced")

	// Both replaced sessions end with the rotation event, on the instance that did not serve the change.
	for name, s := range map[string]*accessStream{"session one": streamOne, "session one /r": adapterOne, "session two": streamTwo} {
		require.True(t, s.endedWithin(3*time.Second), "%s stream ends", name)
		require.Contains(t, s.events(), "credential_rotated", "%s stream says the credential was rotated", name)
		require.NotContains(t, s.events(), "access_revoked", "%s: rotation is not a loss of access", name)
	}
	require.False(t, byKey.endedWithin(300*time.Millisecond), "the account-key stream is not a session token: it continues")
	require.False(t, planner.endedWithin(100*time.Millisecond), "the planner is untouched")

	// Every surface on every instance answers the replaced tokens with the recoverable code.
	for name, tok := range map[string]string{"session one": sessionOne, "session two": sessionTwo} {
		for _, inst := range []*roomInstance{a, b} {
			for surface, res := range everyRoomSurface(t, inst, room, tok) {
				require.Equal(t, http.StatusUnauthorized, res.Status, "%s on %s", name, surface)
				code, msg, reqID := sessionError(t, res.Out)
				require.Equal(t, "CREDENTIAL_ROTATED", code, "%s on %s", name, surface)
				require.Contains(t, msg, "/v1/rooms/"+room.slug+"/handshake", "the answer says how to recover")
				require.NotEmpty(t, reqID, "the envelope carries request_id")
			}
		}
	}

	// The rotated-in token works everywhere; the planner never noticed.
	for _, inst := range []*roomInstance{a, b} {
		for surface, res := range everyRoomSurface(t, inst, room, rotatedIn) {
			require.Less(t, res.Status, 300, "the new token on %s: %v", surface, res.Out)
		}
	}
	status, _ = doJSON(t, "GET", b.ts.URL+"/v1/rooms/"+room.slug+"/entries", room.plannerTok, "")
	require.Equal(t, http.StatusOK, status)

	// A token that never existed is still a plain 401, and a stream opened with a replaced
	// token is refused with the recoverable code, not opened.
	status, out = doJSON(t, "GET", a.ts.URL+"/v1/rooms/"+room.slug+"/entries", "solvr_rt_"+strings.Repeat("0", 64), "")
	require.Equal(t, http.StatusUnauthorized, status)
	code, _, _ := sessionError(t, out)
	require.Equal(t, "UNAUTHORIZED", code, "an unknown token is not 'rotated'")
	status, out = doJSON(t, "GET", a.ts.URL+"/v1/rooms/"+room.slug+"/stream", sessionOne, "")
	require.Equal(t, http.StatusUnauthorized, status)
	code, _, _ = sessionError(t, out)
	require.Equal(t, "CREDENTIAL_ROTATED", code, "a reconnect with the replaced token is told to re-handshake")

	// Recovery is one call: handshake again with the agent key.
	status, out = handshakeBody(t, b, room, room.executorKey, `{}`)
	require.Equal(t, http.StatusCreated, status, "recover by handshaking again: %v", out)
	status, _ = doJSON(t, "GET", b.ts.URL+"/v1/rooms/"+room.slug+"/entries", tokenOf(out), "")
	require.Equal(t, http.StatusOK, status)
	status, _ = doJSON(t, "GET", b.ts.URL+"/v1/rooms/"+room.slug+"/entries", rotatedIn, "")
	require.Equal(t, http.StatusOK, status, "recovering did not invalidate the session that rotated")
}

func TestRoomTokenSessions_TheLimitIsAnExplicitConflictThatRotationClears(t *testing.T) {
	a := startRoomInstance(t, RoomRelayOptions{SweepInterval: -1})
	room := newAccessRoom(t, a, true)

	tokens := []string{room.executorTok}
	for len(tokens) < db.MaxLiveRoomAgentTokens {
		status, out := handshakeBody(t, a, room, room.executorKey, `{}`)
		require.Equal(t, http.StatusCreated, status, "session %d: %v", len(tokens)+1, out)
		tokens = append(tokens, tokenOf(out))
	}
	status, out := handshakeBody(t, a, room, room.executorKey, `{}`)
	require.Equal(t, http.StatusConflict, status, "one session too many: %v", out)
	code, msg, reqID := sessionError(t, out)
	require.Equal(t, "TOKEN_LIMIT_REACHED", code)
	require.Contains(t, msg, "rotate", "the refusal names the explicit way out")
	require.NotEmpty(t, reqID)
	for _, tok := range []string{tokens[0], tokens[len(tokens)-1]} {
		status, _ = doJSON(t, "GET", a.ts.URL+"/v1/rooms/"+room.slug+"/entries", tok, "")
		require.Equal(t, http.StatusOK, status, "a refused handshake replaces nobody")
	}

	status, out = handshakeBody(t, a, room, room.executorKey, `{"rotate":true}`)
	require.Equal(t, http.StatusCreated, status, "rotation is never refused for the limit: %v", out)
	for _, tok := range tokens {
		status, out = doJSON(t, "GET", a.ts.URL+"/v1/rooms/"+room.slug+"/entries", tok, "")
		require.Equal(t, http.StatusUnauthorized, status)
		code, _, _ = sessionError(t, out)
		require.Equal(t, "CREDENTIAL_ROTATED", code)
	}
}

func TestRoomTokenSessions_RevokingAnAgentIsNotReportedAsARotation(t *testing.T) {
	a := startRoomInstance(t, RoomRelayOptions{SweepInterval: -1})
	room := newAccessRoom(t, a, true)

	status, out := handshakeBody(t, a, room, room.executorKey, `{}`)
	require.Equal(t, http.StatusCreated, status, "%v", out)
	status, out = handshakeBody(t, a, room, room.executorKey, `{"rotate":true}`)
	require.Equal(t, http.StatusCreated, status, "%v", out)
	current := tokenOf(out)

	status, out = doJSON(t, "DELETE", revokeTokenURL(a, room, room.executorID), room.ownerJWT, "")
	require.Equal(t, http.StatusNoContent, status, "owner revokes the agent's tokens: %v", out)

	// Both the token the revoke ended and the one an earlier rotation had ended are simply
	// dead: telling their holders to "just handshake again" would hide that the owner acted.
	for name, tok := range map[string]string{"current": current, "already rotated": room.executorTok} {
		status, out = doJSON(t, "GET", a.ts.URL+"/v1/rooms/"+room.slug+"/entries", tok, "")
		require.Equal(t, http.StatusUnauthorized, status, name)
		code, _, _ := sessionError(t, out)
		require.Equal(t, "UNAUTHORIZED", code, "%s token after an owner revoke", name)
	}
}

func TestRoomHandshake_RotateMustBeABoolean(t *testing.T) {
	a := startRoomInstance(t, RoomRelayOptions{SweepInterval: -1})
	room := newAccessRoom(t, a, true)
	status, out := handshakeBody(t, a, room, room.executorKey, `{"rotate":"yes"}`)
	require.Equal(t, http.StatusBadRequest, status, "%v", out)
	code, _, _ := sessionError(t, out)
	require.Equal(t, "VALIDATION_ERROR", code)
	status, _ = doJSON(t, "GET", a.ts.URL+"/v1/rooms/"+room.slug+"/entries", room.executorTok, "")
	require.Equal(t, http.StatusOK, status, "a refused handshake replaces nobody")
}
