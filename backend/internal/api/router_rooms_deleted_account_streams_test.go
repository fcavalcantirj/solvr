package api

import (
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// idx 75 step 3: revocation on established connections covers the ACCOUNT behind a
// credential, not only the membership. A deleted agent's per-agent room token, agent-key
// stream and stream tickets, and a deleted human's stream and pending tickets, stop working
// the moment the account is deleted, on every API instance, while every other participant
// keeps streaming. (Before this, the deleted agent's room token still read, wrote and minted
// tickets, and the streams of a deleted account stayed open until the client left.)

func TestDeletedAgent_RoomCredentialsAndStreamsEndWithTheAccount(t *testing.T) {
	opts := RoomRelayOptions{SweepInterval: -1}
	a := startRoomInstance(t, opts)
	b := startRoomInstance(t, opts)
	room := newAccessRoom(t, a, true)
	stream := room.streamURL(b)

	tokenTicket := mintTicket(t, a, room.slug, room.executorTok)
	keyTicket := mintTicket(t, a, room.slug, room.executorKey)
	executor := map[string]*accessStream{
		"room token":   openAccessStream(t, stream, room.executorTok),
		"/r adapter":   openAccessStream(t, b.ts.URL+"/r/"+room.slug+"/stream", room.executorTok),
		"agent key":    openAccessStream(t, stream, room.executorKey),
		"token ticket": openAccessStream(t, stream+"?ticket="+tokenTicket, ""),
		"key ticket":   openAccessStream(t, stream+"?ticket="+keyTicket, ""),
	}
	planner := openAccessStream(t, stream, room.plannerTok)

	status, out := doJSON(t, "DELETE", a.ts.URL+"/v1/agents/me", room.executorKey, "")
	require.Equal(t, http.StatusOK, status, "the executor deletes its own account through A: %v", out)

	for name, s := range executor {
		require.True(t, s.endedWithin(3*time.Second), "the deleted agent's %s stream on B ends, not when the client leaves", name)
		require.Contains(t, s.events(), "access_revoked", "the deleted agent's %s stream says why it ended", name)
	}
	require.False(t, planner.endedWithin(300*time.Millisecond), "the planner keeps streaming")

	for _, inst := range []*roomInstance{a, b} {
		// A deleted agent's room token is worth what a token that never existed is worth.
		status, out = doJSON(t, "GET", inst.ts.URL+"/v1/rooms/"+room.slug+"/entries", room.executorTok, "")
		require.Equal(t, http.StatusUnauthorized, status, "entries read with the deleted agent's room token: %v", out)
		code, _, _ := sessionError(t, out)
		require.Equal(t, "UNAUTHORIZED", code, "a deletion is not a rotation")
		status, _ = doJSON(t, "GET", inst.ts.URL+"/r/"+room.slug+"/messages", room.executorTok, "")
		require.Equal(t, http.StatusUnauthorized, status, "/r read with the deleted agent's room token")
		status, _, err := postEntryRaw(inst.ts.URL, room.slug, room.executorTok, map[string]any{"body": "written by a deleted agent"})
		require.NoError(t, err)
		require.Equal(t, http.StatusUnauthorized, status, "a deleted agent must not publish with its room token")
		status, _ = doJSON(t, "POST", inst.ts.URL+"/v1/rooms/"+room.slug+"/stream-ticket", room.executorTok, "")
		require.Equal(t, http.StatusUnauthorized, status, "a deleted agent's room token mints no ticket")

		// Tickets minted before the deletion open nothing any more.
		require.Equal(t, http.StatusUnauthorized, getStatus(t, inst.ts.URL+"/v1/rooms/"+room.slug+"/stream?ticket="+tokenTicket, ""),
			"the deleted agent's room-token ticket reopens no stream")
		require.Equal(t, http.StatusUnauthorized, getStatus(t, inst.ts.URL+"/v1/rooms/"+room.slug+"/stream?ticket="+keyTicket, ""),
			"the deleted agent's agent-key ticket reopens no stream")
	}

	// The room carries on for everyone who is still there.
	status, id, err := postEntryRaw(a.ts.URL, room.slug, room.plannerTok, map[string]any{"body": "after the executor's account was deleted"})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, status)
	deadline := time.Now().Add(5 * time.Second)
	for !slices.Contains(planner.ids(), id) && time.Now().Before(deadline) {
		time.Sleep(25 * time.Millisecond)
	}
	require.Contains(t, planner.ids(), id, "the planner still receives new entries")
}

func TestDeletedHuman_OpenStreamsAndPendingTicketsEndWithTheAccount(t *testing.T) {
	opts := RoomRelayOptions{SweepInterval: -1}
	a := startRoomInstance(t, opts)
	b := startRoomInstance(t, opts)
	room := newAccessRoom(t, a, true)
	stream := room.streamURL(b)

	openTicket := mintTicket(t, a, room.slug, room.ownerJWT)
	pendingTicket := mintTicket(t, a, room.slug, room.ownerJWT)
	owner := map[string]*accessStream{
		"bearer header": openAccessStream(t, stream, room.ownerJWT),
		"ticket":        openAccessStream(t, stream+"?ticket="+openTicket, ""),
	}
	planner := openAccessStream(t, stream, room.plannerTok)

	status, out := doJSON(t, "DELETE", a.ts.URL+"/v1/me", room.ownerJWT, "")
	require.Equal(t, http.StatusOK, status, "the owner deletes their account through A: %v", out)

	for name, s := range owner {
		require.True(t, s.endedWithin(3*time.Second), "the deleted owner's %s stream on B ends; the room they solely owned is archived, "+
			"not left readable to an account that no longer exists", name)
		require.Contains(t, s.events(), "access_revoked", "the deleted owner's %s stream says why it ended", name)
	}
	require.False(t, planner.endedWithin(300*time.Millisecond), "the planner keeps streaming")

	for _, inst := range []*roomInstance{a, b} {
		status, out = doJSON(t, "GET", inst.ts.URL+"/v1/rooms/"+room.slug+"/stream?ticket="+pendingTicket, "", "")
		require.Equal(t, http.StatusUnauthorized, status, "a ticket minted before the deletion opens no stream: %v", out)
		status, _ = doJSON(t, "GET", inst.ts.URL+"/v1/rooms/"+room.slug+"/entries", room.plannerTok, "")
		require.Equal(t, http.StatusOK, status, "the planner still reads the room")
	}
	require.NotContains(t, strings.Join(planner.events(), " "), "access_revoked")
}
