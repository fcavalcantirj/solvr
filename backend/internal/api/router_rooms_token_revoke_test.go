package api

import (
	"net/http"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// Revoking ONE participant's room token (task: "Keep room and content permissions
// authoritative on the server", step 4). DELETE /v1/rooms/{slug}/members/{agent_id}/token
// kills that credential everywhere — REST, the /r adapter and its open streams on every
// API instance — without touching the membership or any other participant.

func revokeTokenURL(inst *roomInstance, room *accessRoom, agentID string) string {
	return inst.ts.URL + "/v1/rooms/" + room.slug + "/members/" + agentID + "/token"
}

func TestRoomTokenRevoke_RevokedTokenIsDeadEverywhereWhilePeersContinue(t *testing.T) {
	opts := RoomRelayOptions{SweepInterval: -1}
	a := startRoomInstance(t, opts)
	b := startRoomInstance(t, opts)
	room := newAccessRoom(t, a, true)

	execByToken := openAccessStream(t, room.streamURL(b), room.executorTok)
	execAdapter := openAccessStream(t, b.ts.URL+"/r/"+room.slug+"/stream", room.executorTok)
	execByKey := openAccessStream(t, room.streamURL(b), room.executorKey)
	planner := openAccessStream(t, room.streamURL(b), room.plannerTok)

	status, out := doJSON(t, "DELETE", revokeTokenURL(a, room, room.executorID), room.ownerJWT, "")
	require.Equal(t, http.StatusNoContent, status, "owner revokes the executor's token through A: %v", out)

	for name, s := range map[string]*accessStream{"room token": execByToken, "/r adapter": execAdapter} {
		require.True(t, s.endedWithin(3*time.Second), "the executor's %s stream on B ends", name)
		require.Contains(t, s.events(), "access_revoked", "the executor's %s stream says why it ended", name)
	}
	require.False(t, execByKey.endedWithin(300*time.Millisecond), "the membership is intact: the account-key stream continues")
	require.False(t, planner.endedWithin(300*time.Millisecond), "the planner keeps streaming")

	// The planner is unaffected and still delivers to everyone who kept access.
	status, id, err := postEntryRaw(a.ts.URL, room.slug, room.plannerTok, map[string]any{"body": "after token revoke"})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, status)
	deadline := time.Now().Add(5 * time.Second)
	for !(slices.Contains(planner.ids(), id) && slices.Contains(execByKey.ids(), id)) && time.Now().Before(deadline) {
		time.Sleep(25 * time.Millisecond)
	}
	require.Contains(t, planner.ids(), id, "the planner still receives new entries")
	require.Contains(t, execByKey.ids(), id, "the executor, still a member, receives them over its account key")

	// The revoked token cannot be reused on any surface or instance.
	for _, inst := range []*roomInstance{a, b} {
		status, _ = doJSON(t, "GET", inst.ts.URL+"/v1/rooms/"+room.slug+"/entries", room.executorTok, "")
		require.Equal(t, http.StatusUnauthorized, status, "entries read with the revoked token")
		status, _ = doJSON(t, "GET", inst.ts.URL+"/r/"+room.slug+"/messages", room.executorTok, "")
		require.Equal(t, http.StatusUnauthorized, status, "/r read with the revoked token")
		status, _, err = postEntryRaw(inst.ts.URL, room.slug, room.executorTok, map[string]any{"body": "revoked?"})
		require.NoError(t, err)
		require.Equal(t, http.StatusUnauthorized, status, "entries write with the revoked token")
		status, _ = doJSON(t, "POST", inst.ts.URL+"/r/"+room.slug+"/message", room.executorTok, `{"content":"revoked?"}`)
		require.Equal(t, http.StatusUnauthorized, status, "/r write with the revoked token")
	}

	// Membership is untouched: the owner still lists the executor, and the executor can
	// prove its identity again with its own API key to get a NEW token.
	status, out = doJSON(t, "GET", a.ts.URL+"/v1/rooms/"+room.slug+"/members", room.ownerJWT, "")
	require.Equal(t, http.StatusOK, status)
	require.Contains(t, memberAgentIDs(out), room.executorID, "revoking a token does not remove the member")
	fresh := handshakeRoomToken(t, b.ts, room.slug, room.executorKey)
	require.NotEqual(t, room.executorTok, fresh, "a re-handshake issues a different token")
	status, _ = doJSON(t, "GET", b.ts.URL+"/v1/rooms/"+room.slug+"/entries", fresh, "")
	require.Equal(t, http.StatusOK, status, "the new token works")
	status, _ = doJSON(t, "GET", b.ts.URL+"/v1/rooms/"+room.slug+"/entries", room.executorTok, "")
	require.Equal(t, http.StatusUnauthorized, status, "the old token stays dead after the re-handshake")
	status, _ = doJSON(t, "GET", a.ts.URL+"/v1/rooms/"+room.slug+"/entries", room.plannerTok, "")
	require.Equal(t, http.StatusOK, status, "the planner's token was never touched")
}

func TestRoomTokenRevoke_OnlyManagersRevokeAndOnlyForActiveMembers(t *testing.T) {
	a := startRoomInstance(t, RoomRelayOptions{SweepInterval: -1})
	room := newAccessRoom(t, a, true)
	outsiderID, outsiderKey := registerRoomTestAgent(t, a.ts)

	status, _ := doJSON(t, "DELETE", revokeTokenURL(a, room, room.executorID), "", "")
	require.Equal(t, http.StatusUnauthorized, status, "anonymous")
	status, _ = doJSON(t, "DELETE", revokeTokenURL(a, room, room.executorID), room.plannerKey, "")
	require.Equal(t, http.StatusForbidden, status, "a fellow member cannot revoke another participant's token")
	status, _ = doJSON(t, "DELETE", revokeTokenURL(a, room, room.executorID), outsiderKey, "")
	require.Equal(t, http.StatusForbidden, status, "an unrelated agent cannot revoke")
	status, _ = doJSON(t, "DELETE", revokeTokenURL(a, room, room.executorID), room.plannerTok, "")
	require.Equal(t, http.StatusUnauthorized, status, "a room token never authorizes management")
	status, _ = doJSON(t, "GET", a.ts.URL+"/v1/rooms/"+room.slug+"/entries", room.executorTok, "")
	require.Equal(t, http.StatusOK, status, "refused attempts left the token alive")

	status, out := doJSON(t, "DELETE", revokeTokenURL(a, room, outsiderID), room.ownerJWT, "")
	require.Equal(t, http.StatusNotFound, status, "not a member: %v", out)
	status, _ = doJSON(t, "DELETE", a.ts.URL+"/v1/rooms/no-such-room-xyz/members/"+room.executorID+"/token", room.ownerJWT, "")
	require.Equal(t, http.StatusNotFound, status, "unknown room")

	status, _ = doJSON(t, "DELETE", revokeTokenURL(a, room, room.executorID), room.ownerJWT, "")
	require.Equal(t, http.StatusNoContent, status)
	status, _ = doJSON(t, "DELETE", revokeTokenURL(a, room, room.executorID), room.ownerJWT, "")
	require.Equal(t, http.StatusNoContent, status, "revoking an already revoked token is idempotent")

	status, _ = doJSON(t, "DELETE", a.ts.URL+"/v1/rooms/"+room.slug+"/members/"+room.executorID, room.ownerJWT, "")
	require.Equal(t, http.StatusNoContent, status)
	status, _ = doJSON(t, "DELETE", revokeTokenURL(a, room, room.executorID), room.ownerJWT, "")
	require.Equal(t, http.StatusNotFound, status, "a removed member has no token to revoke")
}

func memberAgentIDs(out map[string]any) []string {
	var ids []string
	rows, _ := out["data"].([]any)
	for _, row := range rows {
		if m, ok := row.(map[string]any); ok {
			if id, ok := m["agent_id"].(string); ok {
				ids = append(ids, id)
			}
		}
	}
	return ids
}
