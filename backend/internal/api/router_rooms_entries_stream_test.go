package api

import (
	"bufio"
	"context"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Canonical room stream (task: "Offer one canonical room API with thin adapters for the
// existing transport routes"). GET /v1/rooms/{slug}/stream accepts human, agent-account
// and room-scoped credentials through the same authorization policy as the canonical
// entries routes; GET /r/{slug}/stream is the room-token transport adapter over the same
// stream (same hub, same frames, same replay).

// sseFrames opens an SSE stream and records every frame's "id" (0 when absent) and
// "event" until ctx ends. ready is closed once response headers arrived.
type sseFrame struct {
	ID    int64
	Event string
	Data  string
}

func openSSEFrames(ctx context.Context, t *testing.T, url, bearer string, frames *[]sseFrame, mu *sync.Mutex, ready chan<- struct{}) {
	t.Helper()
	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		close(ready)
		return
	}
	defer resp.Body.Close()
	close(ready)
	if resp.StatusCode != http.StatusOK {
		return
	}
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	var cur sseFrame
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case strings.HasPrefix(line, "id:"):
			cur.ID, _ = strconv.ParseInt(strings.TrimSpace(strings.TrimPrefix(line, "id:")), 10, 64)
		case strings.HasPrefix(line, "event:"):
			cur.Event = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		case strings.HasPrefix(line, "data:"):
			cur.Data = strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		case line == "" && cur.Event != "":
			mu.Lock()
			*frames = append(*frames, cur)
			mu.Unlock()
			cur = sseFrame{}
		}
	}
}

// streamCapture runs openSSEFrames in the background and returns a stop function that
// ends the stream and returns the captured frames.
func streamCapture(t *testing.T, url, bearer string) func() []sseFrame {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	var frames []sseFrame
	var mu sync.Mutex
	ready := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		openSSEFrames(ctx, t, url, bearer, &frames, &mu, ready)
	}()
	<-ready
	return func() []sseFrame {
		cancel()
		<-done
		mu.Lock()
		defer mu.Unlock()
		return append([]sseFrame(nil), frames...)
	}
}

// frameIDs returns the ids of the frames of the given SSE event type, in order.
func frameIDs(frames []sseFrame, event string) []int64 {
	var ids []int64
	for _, f := range frames {
		if f.Event == event && f.ID > 0 {
			ids = append(ids, f.ID)
		}
	}
	return ids
}

// timelineFrames keeps only the message and event frames.
func timelineFrames(frames []sseFrame) []sseFrame {
	var out []sseFrame
	for _, f := range frames {
		if f.Event == "message" || f.Event == "event" {
			out = append(out, f)
		}
	}
	return out
}

func TestRoomEntriesStream_OneAuthorizationPolicyAcrossCredentials(t *testing.T) {
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
	publicTok := handshakeRoomToken(t, ts, public, outsiderKey) // valid, but only for the public room
	const invalidTok = "solvr_rt_notarealtoken"
	publicTicket := mintTicketAt(t, ts.URL, public, publicTok) // bound to the public room
	memberTicket := mintTicketAt(t, ts.URL, private, memberTok)
	ownerTicket := mintTicketAt(t, ts.URL, private, ownerJWT)

	privStream := ts.URL + "/v1/rooms/" + private + "/stream"
	pubStream := ts.URL + "/v1/rooms/" + public + "/stream"
	privAdapter := ts.URL + "/r/" + private + "/stream"

	cases := []struct {
		name   string
		url    string
		bearer string
		want   int
	}{
		// Canonical stream, private room.
		{"anonymous on private", privStream, "", http.StatusForbidden},
		{"outsider agent key on private", privStream, outsiderKey, http.StatusForbidden},
		{"outsider human on private", privStream, outsiderJWT, http.StatusForbidden},
		{"other room's token on private", privStream, publicTok, http.StatusForbidden},
		{"other room's ticket", privStream + "?ticket=" + publicTicket, "", http.StatusForbidden},
		{"invalid room token on private", privStream, invalidTok, http.StatusUnauthorized},
		{"member room token header", privStream, memberTok, http.StatusOK},
		{"member room token ticket", privStream + "?ticket=" + memberTicket, "", http.StatusOK},
		{"member room token ?token is retired", privStream + "?token=" + memberTok, "", http.StatusBadRequest},
		{"member agent key", privStream, memberKey, http.StatusOK},
		{"owner human JWT header", privStream, ownerJWT, http.StatusOK},
		{"owner human JWT ticket", privStream + "?ticket=" + ownerTicket, "", http.StatusOK},
		// Canonical stream, public room: reads are open, but a presented room token must
		// be valid and for THIS room — the same decision as GET /v1/rooms/{slug}/entries.
		{"anonymous on public", pubStream, "", http.StatusOK},
		{"public room's own token", pubStream, publicTok, http.StatusOK},
		{"other room's token on public", pubStream, memberTok, http.StatusForbidden},
		{"invalid room token on public", pubStream, invalidTok, http.StatusUnauthorized},
		// /r adapter: room tokens only, same token decisions.
		{"adapter without token", privAdapter, "", http.StatusUnauthorized},
		{"adapter invalid token", privAdapter, invalidTok, http.StatusUnauthorized},
		{"adapter other room's token", privAdapter, publicTok, http.StatusForbidden},
		{"adapter member token", privAdapter, memberTok, http.StatusOK},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, getStatus(t, c.url, c.bearer), c.name)
	}

	// The same decisions hold for the canonical entries list (one policy, not two).
	for _, c := range []struct {
		name   string
		url    string
		bearer string
		want   int
	}{
		{"entries: other room's token on public", ts.URL + "/v1/rooms/" + public + "/entries", memberTok, http.StatusForbidden},
		{"entries: invalid room token on public", ts.URL + "/v1/rooms/" + public + "/entries", invalidTok, http.StatusUnauthorized},
	} {
		assert.Equal(t, c.want, getStatus(t, c.url, c.bearer), c.name)
	}
}

func TestRoomEntriesStream_CanonicalAndAdapterDeliverTheSameEntries(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	roomPreCleanup(t, pool)

	_, ownerJWT := createRoomTestUser(t, pool)
	slug := entriesTestRoom(t, ts.URL, ownerJWT, true)
	memberID, memberKey := registerRoomTestAgent(t, ts)
	status, out := doJSON(t, "POST", ts.URL+"/v1/rooms/"+slug+"/members", ownerJWT, `{"agent_id":"`+memberID+`"}`)
	require.Equal(t, http.StatusCreated, status, "owner admits member: %v", out)
	memberTok := handshakeRoomToken(t, ts, slug, memberKey)

	canonical := ts.URL + "/v1/rooms/" + slug + "/stream"
	adapter := ts.URL + "/r/" + slug + "/stream"

	// Three live subscribers with three credentials on two routes.
	stopKey := streamCapture(t, canonical, memberKey)
	stopHuman := streamCapture(t, canonical+"?ticket="+mintTicketAt(t, ts.URL, slug, ownerJWT), "")
	stopAdapter := streamCapture(t, adapter, memberTok)
	time.Sleep(300 * time.Millisecond) // let the subscriptions register

	// One human message through the canonical entries route, one agent message through
	// the /r adapter, one typed event through the canonical entries route.
	status, out = doJSON(t, "POST", ts.URL+"/v1/rooms/"+slug+"/entries", ownerJWT, `{"body":"from the human"}`)
	require.Equal(t, http.StatusCreated, status, "canonical human message: %v", out)
	humanMsg := entryID(t, out)
	agentMsg := postRoomMessage(t, ts.URL, slug, memberTok, "w1", "from the agent")
	status, out = doJSON(t, "POST", ts.URL+"/v1/rooms/"+slug+"/entries", memberTok,
		`{"kind":"event","event_type":"CLAIM","issue":"ISSUE-70","extension":{"k":"v"}}`)
	require.Equal(t, http.StatusCreated, status, "canonical event: %v", out)
	eventID := entryID(t, out)

	time.Sleep(700 * time.Millisecond)
	streams := map[string][]sseFrame{"agent key": stopKey(), "human JWT": stopHuman(), "adapter": stopAdapter()}
	// Every committed event entry streams live: the posted CLAIM and the room.activated
	// milestone the server recorded when the second distinct author wrote.
	events := entriesAfter(t, ts.URL, slug, memberKey, 0, "event")
	require.Contains(t, events, eventID)
	require.Len(t, events, 2, "the CLAIM and the room.activated milestone: %v", events)
	for name, frames := range streams {
		assert.Equal(t, []int64{humanMsg, agentMsg}, frameIDs(frames, "message"), "%s: message frames", name)
		assert.Equal(t, events, frameIDs(frames, "event"), "%s: event frames", name)
	}
	// Timeline frames (not presence, which depends on subscriber timing) are byte-identical.
	assert.Equal(t, timelineFrames(streams["adapter"]), timelineFrames(streams["agent key"]), "canonical and adapter frames must be identical")

	// Reconnect replay: ?after=<id> replays the same gap on both routes.
	after := "?after=" + strconv.FormatInt(humanMsg, 10)
	stopCanonReplay := streamCapture(t, canonical+after, memberKey)
	stopAdapterReplay := streamCapture(t, adapter+after, memberTok)
	time.Sleep(500 * time.Millisecond)
	canonReplay, adapterReplay := stopCanonReplay(), stopAdapterReplay()
	assert.Equal(t, []int64{agentMsg}, frameIDs(canonReplay, "message"), "canonical replay")
	assert.Equal(t, frameIDs(adapterReplay, "message"), frameIDs(canonReplay, "message"), "adapter replay equals canonical")
}
