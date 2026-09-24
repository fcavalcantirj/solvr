package api

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A consumer streaming one issue must be able to resume precisely (task: "Let a consumer
// follow one room issue forward without holes and never ignore a paging parameter").
// Every timeline frame carries the entry id (the SSE id line) and the per-room sequence,
// and a reconnect with Last-Event-ID replays exactly the missed entries matching the
// stream's filters, however many non-matching entries were written in between.

// frameSequence decodes the per-room sequence from a frame's data.
func frameSequence(t *testing.T, f sseFrame) int {
	t.Helper()
	var data struct {
		ID       int64 `json:"id"`
		Sequence int   `json:"sequence"`
	}
	require.NoError(t, json.Unmarshal([]byte(f.Data), &data), "frame data: %s", f.Data)
	require.Equal(t, f.ID, data.ID, "id line must equal the frame's entry id: %+v", f)
	return data.Sequence
}

func TestRoomSSE_IssueStreamResumesWithoutMissOrRepeat(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	roomPreCleanup(t, pool)

	_, jwt := createRoomTestUser(t, pool)
	slug, tok := createTestRoomWithToken(t, ts, jwt)
	stream := ts.URL + "/r/" + slug + "/stream"

	// Live: stream issue APP-X while other traffic interleaves.
	stop := streamCapture(t, stream+"?issue=APP-X", tok)
	time.Sleep(300 * time.Millisecond) // let the subscription register
	x1, _ := issuePagingEvent(t, ts.URL, slug, tok, "CLAIM", "APP-X")
	issuePagingEvent(t, ts.URL, slug, tok, "CLAIM", "APP-Y")
	postRoomMessage(t, ts.URL, slug, tok, "w1", "chatter")
	x2, _ := issuePagingEvent(t, ts.URL, slug, tok, "BUILDING", "APP-X")
	time.Sleep(500 * time.Millisecond)
	live := stop() // the connection drops mid-issue
	require.Equal(t, []int64{x1, x2}, frameIDs(live, "event"), "live issue frames")
	for _, f := range timelineFrames(live) {
		assert.Positive(t, frameSequence(t, f), "live event frame must carry the sequence")
	}

	// While disconnected: far more non-matching entries than one replay batch, then the
	// issue continues.
	_, err := pool.Exec(context.Background(), `INSERT INTO room_entries (room_id, kind, actor_label, event_type, issue)
		SELECT r.id, 'event', 'seed', 'CLAIM', 'APP-Y' FROM rooms r, generate_series(1, 150) WHERE r.slug = $1`, slug)
	require.NoError(t, err)
	postRoomMessage(t, ts.URL, slug, tok, "w1", "more chatter")
	x3, _ := issuePagingEvent(t, ts.URL, slug, tok, "BUILDING", "APP-X")
	x4, _ := issuePagingEvent(t, ts.URL, slug, tok, "DONE", "APP-X")

	// Reconnect with the last delivered id; one more issue event arrives live.
	stop = streamCapture(t, fmt.Sprintf("%s?issue=APP-X&after=%d", stream, x2), tok)
	time.Sleep(500 * time.Millisecond)
	x5, _ := issuePagingEvent(t, ts.URL, slug, tok, "REVIEW", "APP-X")
	time.Sleep(500 * time.Millisecond)
	resumed := stop()
	assert.Equal(t, []int64{x3, x4, x5}, frameIDs(resumed, "event"),
		"resume must deliver every missed issue event exactly once, in order, then live")

	// An unfiltered resume replays the whole gap (messages and events), each frame with
	// its id line and a strictly increasing sequence — no replay cap leaves a hole.
	stop = streamCapture(t, fmt.Sprintf("%s?after=%d", stream, x2), tok)
	time.Sleep(800 * time.Millisecond)
	all := timelineFrames(stop())
	require.Len(t, all, 150+1+3, "every entry after the cursor is replayed")
	last := 0
	for _, f := range all {
		require.Positive(t, f.ID, "every timeline frame carries an id line: %+v", f)
		seq := frameSequence(t, f)
		require.Greater(t, seq, last, "sequence strictly increases")
		last = seq
	}
	assert.Len(t, frameIDs(all, "message"), 1, "the message frame is replayed with its id and sequence too")
}
