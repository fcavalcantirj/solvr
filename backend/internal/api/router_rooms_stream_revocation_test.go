package api

import (
	"context"
	"net/http"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// Access changes reach streams that are ALREADY open (task: "Keep room and content
// permissions authoritative on the server", step 3). Removing an agent from a private
// room, making a room private or deleting it must end the established streams of the
// callers who lost read access — on every API instance, not only the one that served the
// change — while every other participant keeps streaming.

// accessStream is an open SSE stream whose end can be observed.
type accessStream struct {
	mu     sync.Mutex
	frames []sseFrame
	done   chan struct{}
	stop   func()
}

func openAccessStream(t *testing.T, url, bearer string) *accessStream {
	t.Helper()
	s := &accessStream{done: make(chan struct{})}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	ready := make(chan struct{})
	go func() {
		defer close(s.done)
		openSSEFrames(ctx, t, url, bearer, &s.frames, &s.mu, ready)
	}()
	<-ready
	s.stop = sync.OnceFunc(func() { cancel(); <-s.done })
	t.Cleanup(s.stop)
	time.Sleep(200 * time.Millisecond) // subscribed before the test changes access
	return s
}

// endedWithin reports whether the server ended the stream within d.
func (s *accessStream) endedWithin(d time.Duration) bool {
	select {
	case <-s.done:
		return true
	case <-time.After(d):
		return false
	}
}

func (s *accessStream) events() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []string
	for _, f := range s.frames {
		out = append(out, f.Event)
	}
	return out
}

func (s *accessStream) ids() []int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return timelineIDs(s.frames)
}

// accessRoom is a private room whose owner JWT and executor agent id are kept, so the
// test can remove and readmit the executor.
type accessRoom struct {
	*twoAgentRoom
	ownerJWT   string
	executorID string
}

func newAccessRoom(t *testing.T, a *roomInstance, private bool) *accessRoom {
	t.Helper()
	roomPreCleanup(t, a.pool)
	_, ownerJWT := createRoomTestUser(t, a.pool)
	slug := entriesTestRoom(t, a.ts.URL, ownerJWT, private)
	r := &accessRoom{twoAgentRoom: &twoAgentRoom{slug: slug, pool: a.pool}, ownerJWT: ownerJWT}
	var plannerID string
	plannerID, r.plannerKey = registerRoomTestAgent(t, a.ts)
	r.executorID, r.executorKey = registerRoomTestAgent(t, a.ts)
	for _, id := range []string{plannerID, r.executorID} {
		status, out := doJSON(t, "POST", a.ts.URL+"/v1/rooms/"+slug+"/members", ownerJWT, `{"agent_id":"`+id+`"}`)
		require.Equal(t, http.StatusCreated, status, "admit %s: %v", id, out)
	}
	r.plannerTok = handshakeRoomToken(t, a.ts, slug, r.plannerKey)
	r.executorTok = handshakeRoomToken(t, a.ts, slug, r.executorKey)
	return r
}

func (r *accessRoom) streamURL(inst *roomInstance) string {
	return inst.ts.URL + "/v1/rooms/" + r.slug + "/stream"
}

func TestRoomStream_RemovedMemberStreamEndsOnEveryInstanceWhilePeersContinue(t *testing.T) {
	opts := RoomRelayOptions{SweepInterval: -1}
	a := startRoomInstance(t, opts)
	b := startRoomInstance(t, opts)
	room := newAccessRoom(t, a, true)

	execByToken := openAccessStream(t, room.streamURL(b), room.executorTok)
	execByKey := openAccessStream(t, room.streamURL(b), room.executorKey)
	execAdapter := openAccessStream(t, b.ts.URL+"/r/"+room.slug+"/stream", room.executorTok)
	planner := openAccessStream(t, room.streamURL(b), room.plannerTok)

	status, out := doJSON(t, "DELETE", a.ts.URL+"/v1/rooms/"+room.slug+"/members/"+room.executorID, room.ownerJWT, "")
	require.Equal(t, http.StatusNoContent, status, "remove executor through A: %v", out)

	for name, s := range map[string]*accessStream{"room token": execByToken, "account key": execByKey, "/r adapter": execAdapter} {
		require.True(t, s.endedWithin(3*time.Second), "executor's %s stream on B must end, not run for 30 min", name)
		require.Contains(t, s.events(), "access_revoked", "executor's %s stream says why it ended", name)
	}
	require.False(t, planner.endedWithin(300*time.Millisecond), "the planner keeps streaming")

	status, id, err := postEntryRaw(a.ts.URL, room.slug, room.plannerTok, map[string]any{"body": "after removal"})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, status)
	deadline := time.Now().Add(5 * time.Second)
	for !slices.Contains(planner.ids(), id) && time.Now().Before(deadline) {
		time.Sleep(25 * time.Millisecond)
	}
	require.Contains(t, planner.ids(), id, "the planner still receives new entries")
	require.NotContains(t, execByToken.ids(), id, "the removed executor never receives them")

	// Denied everywhere until the owner readmits it.
	status, _ = doJSON(t, "POST", b.ts.URL+"/v1/rooms/"+room.slug+"/handshake", room.executorKey, `{}`)
	require.Equal(t, http.StatusForbidden, status, "handshake refused after removal")
	status, _ = doJSON(t, "GET", b.ts.URL+"/v1/rooms/"+room.slug+"/entries", room.executorKey, "")
	require.Equal(t, http.StatusForbidden, status, "reads refused after removal")
	status, _ = doJSON(t, "GET", b.ts.URL+"/v1/rooms/"+room.slug+"/entries", room.executorTok, "")
	require.Equal(t, http.StatusUnauthorized, status, "the revoked token cannot be reused")
	status, _, err = postEntryRaw(b.ts.URL, room.slug, room.executorKey, map[string]any{"body": "still here?"})
	require.NoError(t, err)
	require.Equal(t, http.StatusForbidden, status, "writes refused after removal")

	status, out = doJSON(t, "POST", a.ts.URL+"/v1/rooms/"+room.slug+"/members", room.ownerJWT, `{"agent_id":"`+room.executorID+`"}`)
	require.Equal(t, http.StatusCreated, status, "owner readmits the executor: %v", out)
	tok := handshakeRoomToken(t, b.ts, room.slug, room.executorKey)
	again := openAccessStream(t, room.streamURL(b), tok)
	require.False(t, again.endedWithin(300*time.Millisecond), "the readmitted executor streams again")
}

func TestRoomStream_VisibilityChangeAndDeletionEndStreamsOfCallersWhoLostAccess(t *testing.T) {
	opts := RoomRelayOptions{SweepInterval: -1}
	a := startRoomInstance(t, opts)
	b := startRoomInstance(t, opts)
	room := newAccessRoom(t, a, false)
	_, outsiderKey := registerRoomTestAgent(t, a.ts)

	anonymous := openAccessStream(t, room.streamURL(b), "")
	outsider := openAccessStream(t, room.streamURL(b), outsiderKey)
	member := openAccessStream(t, room.streamURL(b), room.plannerTok)

	status, out := doJSONAtCurrentVersion(t, "PATCH", a.ts.URL+"/v1/rooms/"+room.slug, room.ownerJWT, `{"is_private":true}`)
	require.Equal(t, http.StatusOK, status, "make private through A: %v", out)

	require.True(t, anonymous.endedWithin(3*time.Second), "the anonymous viewer loses the now-private room")
	require.True(t, outsider.endedWithin(3*time.Second), "a non-member agent loses the now-private room")
	require.False(t, member.endedWithin(300*time.Millisecond), "a member keeps streaming")

	status, out = doJSON(t, "DELETE", a.ts.URL+"/v1/rooms/"+room.slug, room.ownerJWT, "")
	require.Equal(t, http.StatusNoContent, status, "delete through A: %v", out)
	require.True(t, member.endedWithin(3*time.Second), "nobody keeps streaming a deleted room")
}

func TestRoomStream_RevocationLostDuringListenerGapIsEnforcedOnReconnect(t *testing.T) {
	opts := RoomRelayOptions{ReconnectBackoff: 1500 * time.Millisecond, SweepInterval: -1}
	a := startRoomInstance(t, opts)
	b := startRoomInstance(t, opts)
	room := newAccessRoom(t, a, true)
	exec := openAccessStream(t, room.streamURL(b), room.executorTok)

	var killed int
	require.NoError(t, a.pool.QueryRow(context.Background(),
		`SELECT COUNT(pg_terminate_backend(pid)) FROM pg_stat_activity
		 WHERE application_name = $1 AND datname = current_database()`, RoomRelayApplicationName).Scan(&killed))
	require.GreaterOrEqual(t, killed, 2, "both instances' listeners were interrupted")

	status, out := doJSON(t, "DELETE", a.ts.URL+"/v1/rooms/"+room.slug+"/members/"+room.executorID, room.ownerJWT, "")
	require.Equal(t, http.StatusNoContent, status, "remove executor while nobody listens: %v", out)

	require.True(t, exec.endedWithin(6*time.Second),
		"B re-checks its streams when its listener reconnects instead of waiting for the next heartbeat")
}
