package api

import (
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Reconnect replay over the canonical timeline (task: "Offer one canonical room API with
// thin adapters for the existing transport routes"). Messages and typed events share one
// entry id space, so a stream reconnecting with Last-Event-ID / ?after=<entry id> replays
// every entry it missed — messages AND events, in timeline order — on the canonical
// GET /v1/rooms/{slug}/stream and on its /r/{slug}/stream adapter alike.

// entriesAfter lists the canonical timeline (GET /v1/rooms/{slug}/entries) and returns the
// ids after afterID with the given kind ("" = both), in timeline order.
func entriesAfter(t *testing.T, base, slug, bearer string, afterID int64, kind string) []int64 {
	t.Helper()
	status, out := doJSON(t, "GET", base+"/v1/rooms/"+slug+"/entries?limit=100&kind="+kind, bearer, "")
	require.Equal(t, http.StatusOK, status, "list entries: %v", out)
	data, ok := out["data"].([]any)
	require.True(t, ok, "entries data: %v", out)
	var ids []int64
	for _, raw := range data {
		id := int64(raw.(map[string]any)["id"].(float64))
		if id > afterID {
			ids = append(ids, id)
		}
	}
	return ids
}

// timelineIDs returns the ids of the message and event frames, in stream order.
func timelineIDs(frames []sseFrame) []int64 {
	var ids []int64
	for _, f := range timelineFrames(frames) {
		ids = append(ids, f.ID)
	}
	return ids
}

func TestRoomEntriesStream_ReplayIncludesEventsInTimelineOrder(t *testing.T) {
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

	// A live subscriber records the frames as they were first delivered.
	stopLive := streamCapture(t, canonical, memberKey)
	time.Sleep(300 * time.Millisecond)

	postEvent := func(bearer, body string) int64 {
		t.Helper()
		status, out := doJSON(t, "POST", ts.URL+"/v1/rooms/"+slug+"/entries", bearer, body)
		require.Equal(t, http.StatusCreated, status, "event: %v", out)
		return entryID(t, out)
	}
	m1 := postRoomMessage(t, ts.URL, slug, memberTok, "w1", "first")
	e1 := postEvent(memberTok, `{"kind":"event","event_type":"CLAIM","issue":"ISSUE-1"}`)
	status, out = doJSON(t, "POST", ts.URL+"/v1/rooms/"+slug+"/entries", ownerJWT, `{"body":"second"}`)
	require.Equal(t, http.StatusCreated, status, "human message: %v", out)
	m2 := entryID(t, out)
	e2 := postEvent(memberKey, `{"kind":"event","event_type":"PR","issue":"ISSUE-2","extension":{"n":2}}`)

	time.Sleep(500 * time.Millisecond)
	live := stopLive()
	// Live delivery reads committed entries back from the timeline, so the live stream is
	// the whole committed timeline: the posted entries plus the room.activated milestone
	// the server recorded on the second distinct author.
	whole := entriesAfter(t, ts.URL, slug, memberKey, 0, "")
	require.Subset(t, whole, []int64{m1, e1, m2, e2})
	require.Len(t, whole, 5, "m1, e1, m2, e2 and the room.activated milestone: %v", whole)
	require.Equal(t, whole, timelineIDs(live), "live stream delivered the whole timeline")

	replay := func(url, bearer string) []sseFrame {
		stop := streamCapture(t, url, bearer)
		time.Sleep(400 * time.Millisecond)
		return stop()
	}
	after := strconv.FormatInt(m1, 10)

	// Every missed entry, messages and events, in timeline order — on both routes and
	// with both cursor spellings. The replay is the canonical timeline after the cursor,
	// server-recorded entries (the room.activated milestone) included, exactly as live.
	want := entriesAfter(t, ts.URL, slug, memberKey, m1, "")
	require.Subset(t, want, []int64{e1, m2, e2})
	require.Len(t, want, 4, "e1, m2, e2 and the room.activated milestone: %v", want)
	canonAfter := replay(canonical+"?after="+after, memberKey)
	canonLastID := replay(canonical+"?lastEventId="+after, memberKey)
	adapterAfter := replay(adapter+"?after="+after, memberTok)
	assert.Equal(t, want, timelineIDs(canonAfter), "canonical ?after replay")
	assert.Equal(t, want, timelineIDs(canonLastID), "canonical ?lastEventId replay")
	assert.Equal(t, want, timelineIDs(adapterAfter), "adapter ?after replay")
	assert.Equal(t, timelineFrames(canonAfter), timelineFrames(adapterAfter), "canonical and adapter replay frames must be identical")

	// A replayed event frame is byte-identical to the frame first delivered live.
	liveByID := map[int64]sseFrame{}
	for _, f := range timelineFrames(live) {
		liveByID[f.ID] = f
	}
	for _, id := range []int64{e1, e2} {
		var replayed sseFrame
		for _, f := range canonAfter {
			if f.ID == id {
				replayed = f
			}
		}
		assert.Equal(t, liveByID[id], replayed, "replayed event %d equals its live frame", id)
	}

	// Server-side filters apply to replay exactly as to live delivery.
	assert.Equal(t, []int64{e2}, timelineIDs(replay(canonical+"?after="+after+"&type=PR", memberKey)), "replay ?type=PR")
	assert.Equal(t, []int64{e1}, timelineIDs(replay(adapter+"?after="+after+"&issue=ISSUE-1", memberTok)), "replay ?issue=ISSUE-1")
	assert.Equal(t, []int64{m2}, timelineIDs(replay(canonical+"?after="+after+"&type=message", memberKey)), "replay ?type=message")
	assert.Equal(t, entriesAfter(t, ts.URL, slug, memberKey, m1, "event"),
		timelineIDs(replay(adapter+"?after="+after+"&type=event", memberTok)), "replay ?type=event equals entries?kind=event")

	// A cursor at an event id resumes after that event.
	assert.Equal(t, entriesAfter(t, ts.URL, slug, memberKey, e1, ""),
		timelineIDs(replay(canonical+"?after="+strconv.FormatInt(e1, 10), memberKey)), "replay after an event id")
}
