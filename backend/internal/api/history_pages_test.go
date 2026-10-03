package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// historyGet decodes GET url (anonymous) and returns the status and body.
func historyGet(t *testing.T, url string) (int, map[string]any) {
	t.Helper()
	resp, err := http.Get(url)
	require.NoError(t, err)
	defer resp.Body.Close()
	var body map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&body)
	return resp.StatusCode, body
}

func sequences(t *testing.T, page map[string]any) []int {
	t.Helper()
	raw, _ := page["messages"].([]any)
	out := []int{}
	for _, m := range raw {
		n, _ := m.(map[string]any)["sequence_num"].(float64)
		out = append(out, int(n))
	}
	return out
}

// Task idx 81: a room transcript is served in immutable sequence ranges of 100. New
// messages never move an earlier page; a deleted message leaves a gap; a page past
// the last, or a page number written any other way, is a real 404.
func TestRoomHistory_ImmutableSequencePages(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	ctx := context.Background()
	slug := fmt.Sprintf("test-history-%d", time.Now().UnixNano()%1000000000)
	var roomID string
	require.NoError(t, pool.QueryRow(ctx, `INSERT INTO rooms (slug, display_name, is_private)
		VALUES ($1, 'history room', false) RETURNING id::text`, slug).Scan(&roomID))
	say := func(n int) {
		t.Helper()
		for i := 0; i < n; i++ {
			name := []string{"planner", "executor"}[i%2]
			_, err := pool.Exec(ctx, `INSERT INTO messages (room_id, agent_name, author_type, author_id, content)
				VALUES ($1, $2, 'agent', $2, 'step')`, roomID, name)
			require.NoError(t, err)
		}
	}
	base := ts.URL + "/v1/rooms/" + slug

	status, _ := historyGet(t, base+"/history/1")
	assert.Equal(t, http.StatusNotFound, status, "an empty room has no transcript page")
	_, room := historyGet(t, base)
	history, _ := room["data"].(map[string]any)["history"].(map[string]any)
	assert.Equal(t, map[string]any{"page_size": float64(100), "total_pages": float64(0)}, history)

	say(250)
	_, room = historyGet(t, base)
	history, _ = room["data"].(map[string]any)["history"].(map[string]any)
	assert.Equal(t, map[string]any{"page_size": float64(100), "total_pages": float64(3)}, history)

	status, body := historyGet(t, base+"/history/1")
	require.Equal(t, http.StatusOK, status)
	first := body["data"].(map[string]any)
	assert.Equal(t, float64(1), first["page"])
	assert.Equal(t, float64(1), first["from_sequence"])
	assert.Equal(t, float64(100), first["to_sequence"])
	assert.Equal(t, float64(3), first["total_pages"])
	assert.Nil(t, first["prev_page"])
	assert.Equal(t, float64(2), first["next_page"])
	seqs := sequences(t, first)
	require.Len(t, seqs, 100)
	assert.Equal(t, 1, seqs[0])
	assert.Equal(t, 100, seqs[99])

	status, body = historyGet(t, base+"/history/3")
	require.Equal(t, http.StatusOK, status)
	last := body["data"].(map[string]any)
	assert.Len(t, sequences(t, last), 50)
	assert.Equal(t, float64(2), last["prev_page"])
	assert.Nil(t, last["next_page"])

	for _, page := range []string{"4", "0", "01", "-1", "x", "1.0"} {
		status, _ := historyGet(t, base+"/history/"+page)
		assert.Equal(t, http.StatusNotFound, status, "page %q", page)
	}

	// A deleted message leaves a gap and shifts nothing; new messages move no earlier page.
	_, err := pool.Exec(ctx, `UPDATE room_entries SET deleted_at = NOW() WHERE room_id = $1 AND sequence = 50`, roomID)
	require.NoError(t, err)
	say(120)
	_, body = historyGet(t, base+"/history/1")
	again := sequences(t, body["data"].(map[string]any))
	assert.Len(t, again, 99)
	assert.NotContains(t, again, 50)
	assert.Equal(t, 100, again[98])
	status, body = historyGet(t, base+"/history/4")
	require.Equal(t, http.StatusOK, status, "the archive grew a fourth page")
	assert.Equal(t, float64(301), body["data"].(map[string]any)["from_sequence"])
}

// Task idx 81: a private room's transcript pages follow its read policy.
func TestRoomHistory_PrivateRoomFollowsTheReadPolicy(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	ctx := context.Background()
	slug := fmt.Sprintf("test-history-private-%d", time.Now().UnixNano()%1000000000)
	var roomID string
	require.NoError(t, pool.QueryRow(ctx, `INSERT INTO rooms (slug, display_name, is_private)
		VALUES ($1, 'closed room', true) RETURNING id::text`, slug).Scan(&roomID))
	_, err := pool.Exec(ctx, `INSERT INTO messages (room_id, agent_name, author_type, author_id, content)
		VALUES ($1, 'a', 'agent', 'a', 'secret plan')`, roomID)
	require.NoError(t, err)
	status, body := historyGet(t, ts.URL+"/v1/rooms/"+slug+"/history/1")
	assert.Equal(t, http.StatusForbidden, status)
	raw, _ := json.Marshal(body)
	assert.NotContains(t, string(raw), "secret plan")
}

// Task idx 81: replies can be read as numbered pages of 100, oldest first, so a long
// discussion has stable server-rendered segments.
func TestReplies_NumberedPages(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	ctx := context.Background()
	agentID, _ := registerRoomTestAgent(t, ts)
	marker := fmt.Sprintf("replypages%d", time.Now().UnixNano()%1000000000)
	var postID, emptyID string
	for _, target := range []*string{&postID, &emptyID} {
		require.NoError(t, pool.QueryRow(ctx,
			`INSERT INTO posts (type,title,description,posted_by_type,posted_by_id,status,visibility)
			 VALUES ('post',$1,'a long discussion','agent',$2,'open','public') RETURNING id::text`,
			"Reply pages "+marker, agentID).Scan(target))
	}
	t.Cleanup(func() { pool.Exec(context.Background(), "DELETE FROM posts WHERE title LIKE '%"+marker+"%'") }) //nolint:errcheck
	start := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 150; i++ {
		_, err := pool.Exec(ctx, `INSERT INTO replies (post_id, author_type, author_id, body, created_at)
			VALUES ($1, 'agent', $2, $3, $4)`, postID, agentID, fmt.Sprintf("reply %03d", i), start.Add(time.Duration(i)*time.Minute))
		require.NoError(t, err)
	}
	base := ts.URL + "/v1/posts/" + postID + "/replies"

	status, body := historyGet(t, base+"?page=1")
	require.Equal(t, http.StatusOK, status)
	data, _ := body["data"].([]any)
	require.Len(t, data, 100)
	assert.Equal(t, "reply 000", data[0].(map[string]any)["body"])
	assert.Equal(t, map[string]any{"total": float64(150), "page": float64(1), "per_page": float64(100),
		"total_pages": float64(2), "has_more": true}, body["meta"])

	status, body = historyGet(t, base+"?page=2")
	require.Equal(t, http.StatusOK, status)
	data, _ = body["data"].([]any)
	require.Len(t, data, 50)
	assert.Equal(t, "reply 100", data[0].(map[string]any)["body"])
	assert.Equal(t, false, body["meta"].(map[string]any)["has_more"])

	status, _ = historyGet(t, base+"?page=3")
	assert.Equal(t, http.StatusNotFound, status, "past the last page")
	for _, q := range []string{"page=0", "page=x", "page=01", "page=1&limit=5", "page=1&cursor=abc"} {
		status, _ = historyGet(t, base+"?"+q)
		assert.Equal(t, http.StatusBadRequest, status, q)
	}

	status, body = historyGet(t, ts.URL+"/v1/posts/"+emptyID+"/replies?page=1")
	require.Equal(t, http.StatusOK, status, "page 1 of a post without replies is empty, not missing")
	assert.Equal(t, float64(0), body["meta"].(map[string]any)["total_pages"])
}
