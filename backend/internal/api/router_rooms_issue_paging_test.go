package api

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Following one room issue forward (task: "Let a consumer follow one room issue forward
// without holes and never ignore a paging parameter"). Global entry ids are shared by
// every room, so the ids of one issue are never contiguous; a consumer pages by opaque
// cursor over the per-room sequence and every filtered page is complete by construction.

// issuePagingEvent posts a typed event through the /r adapter and returns its id and
// per-room sequence from the legacy response envelope.
func issuePagingEvent(t *testing.T, base, slug, roomToken, eventType, issue string) (int64, float64) {
	t.Helper()
	status, out := doJSON(t, "POST", base+"/r/"+slug+"/events", roomToken,
		fmt.Sprintf(`{"type":%q,"issue":%q,"actor":"w1"}`, eventType, issue))
	require.Equal(t, http.StatusCreated, status, "post event: %v", out)
	data := entryData(t, out)
	seq, ok := data["sequence"].(float64)
	require.True(t, ok && seq > 0, "event envelope must carry the per-room sequence: %v", data)
	return entryID(t, out), seq
}

// followIssue pages url (which carries the issue filter) forward with limit, following
// meta.next_cursor until has_more=false, and returns the ids in the order received.
func followIssue(t *testing.T, pageURL, bearer string, limit int) []int64 {
	t.Helper()
	var ids []int64
	cursor := ""
	for page := 0; page < 50; page++ {
		u := fmt.Sprintf("%s&limit=%d", pageURL, limit)
		if cursor != "" {
			u += "&cursor=" + url.QueryEscape(cursor)
		}
		status, out := doJSON(t, "GET", u, bearer, "")
		require.Equal(t, http.StatusOK, status, "page %d of %s: %v", page, u, out)
		data, ok := out["data"].([]any)
		require.True(t, ok, "data must stay an array: %v", out)
		require.LessOrEqual(t, len(data), limit)
		for _, item := range data {
			id, _ := item.(map[string]any)["id"].(float64)
			ids = append(ids, int64(id))
		}
		meta, ok := out["meta"].(map[string]any)
		require.True(t, ok, "meta envelope required: %v", out)
		if meta["has_more"] != true {
			assert.Nil(t, meta["next_cursor"], "a final page carries no next_cursor")
			return ids
		}
		require.Len(t, data, limit, "a page with has_more=true is full")
		cursor, _ = meta["next_cursor"].(string)
		require.NotEmpty(t, cursor, "has_more=true requires next_cursor")
	}
	t.Fatalf("paging %s did not terminate", pageURL)
	return nil
}

func TestRoomIssuePaging_FollowsOneIssueForwardWithoutHoles(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	roomPreCleanup(t, pool)

	_, jwt := createRoomTestUser(t, pool)
	slugA, tokA := createTestRoomWithToken(t, ts, jwt)
	slugB, tokB := createTestRoomWithToken(t, ts, jwt)
	require.NotEqual(t, slugA, slugB)

	// Interleave: room A issue X, room B issue X, room A issue Y, a room A message.
	var wantX []int64
	var lastSeq float64
	for i := 0; i < 7; i++ {
		id, seq := issuePagingEvent(t, ts.URL, slugA, tokA, "BUILDING", "APP-X")
		require.Greater(t, seq, lastSeq, "sequence orders the room timeline")
		lastSeq = seq
		wantX = append(wantX, id)
		issuePagingEvent(t, ts.URL, slugB, tokB, "BUILDING", "APP-X")
		issuePagingEvent(t, ts.URL, slugA, tokA, "CLAIM", "APP-Y")
		msgID := postRoomMessage(t, ts.URL, slugA, tokA, "w1", fmt.Sprintf("chatter %d", i))
		require.Greater(t, msgID, id)
	}
	contiguous := true
	for i := 1; i < len(wantX); i++ {
		if wantX[i] != wantX[i-1]+1 {
			contiguous = false
		}
	}
	require.False(t, contiguous, "fixture must interleave, so issue ids are not contiguous")

	canonical := ts.URL + "/v1/rooms/" + slugA + "/entries?kind=event&issue=APP-X"
	adapter := ts.URL + "/r/" + slugA + "/events?issue=APP-X"
	for _, limit := range []int{1, 3, 7, 50} {
		assert.Equal(t, wantX, followIssue(t, canonical, "", limit), "canonical limit=%d", limit)
		assert.Equal(t, wantX, followIssue(t, adapter, tokA, limit), "adapter limit=%d", limit)
	}

	// The legacy event shape keeps its fields, gains the sequence, and lists oldest first.
	status, out := doJSON(t, "GET", adapter+"&limit=2", tokA, "")
	require.Equal(t, http.StatusOK, status, "%v", out)
	first := out["data"].([]any)[0].(map[string]any)
	for _, field := range []string{"id", "type", "issue", "actor", "payload", "created_at", "sequence"} {
		assert.Contains(t, first, field)
	}
	assert.Equal(t, float64(wantX[0]), first["id"])
	meta := out["meta"].(map[string]any)
	assert.Equal(t, float64(2), meta["limit"])
	assert.Equal(t, true, meta["has_more"])

	// Same default and maximum limits as the rest of the API.
	_, out = doJSON(t, "GET", ts.URL+"/r/"+slugA+"/events", tokA, "")
	assert.Equal(t, float64(50), out["meta"].(map[string]any)["limit"])
	_, out = doJSON(t, "GET", adapter+"&limit=500", tokA, "")
	assert.Equal(t, float64(100), out["meta"].(map[string]any)["limit"])
	status, out = doJSON(t, "GET", adapter+"&limit=abc", tokA, "")
	assert.Equal(t, http.StatusBadRequest, status, "a malformed limit is refused, not ignored: %v", out)
	status, out = doJSON(t, "GET", adapter+"&cursor=bogus", tokA, "")
	assert.Equal(t, http.StatusBadRequest, status, "a malformed cursor is refused, not ignored: %v", out)

	// A message envelope carries the same per-room sequence.
	status, out = doJSON(t, "GET", ts.URL+"/v1/rooms/"+slugA+"/messages?limit=1", "", "")
	require.Equal(t, http.StatusOK, status, "%v", out)
	msgs, _ := out["data"].([]any)
	require.NotEmpty(t, msgs)
	assert.NotNil(t, msgs[0].(map[string]any)["sequence_num"], "message envelope must carry the sequence")
}

func TestRoomIssuePaging_RejectsUnrecognisedQueryParameters(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	roomPreCleanup(t, pool)

	_, jwt := createRoomTestUser(t, pool)
	slug, tok := createTestRoomWithToken(t, ts, jwt)
	id, _ := issuePagingEvent(t, ts.URL, slug, tok, "CLAIM", "APP-1")

	entries := ts.URL + "/v1/rooms/" + slug + "/entries"
	events := ts.URL + "/r/" + slug + "/events"
	cases := []struct{ url, param string }{
		{entries + "?after_id=123", "after_id"},
		{entries + "?after=123", "after"},
		{entries + "?kind=event&issue=APP-1&since=5", "since"},
		{fmt.Sprintf("%s/%d?after_id=123", entries, id), "after_id"},
		{events + "?after_id=123", "after_id"},
		{events + "?after=123", "after"},
		{events + "?issue=APP-1&offset=10", "offset"},
		{ts.URL + "/v1/rooms/" + slug + "/stream?after_id=123", "after_id"},
		{ts.URL + "/r/" + slug + "/stream?after_id=123", "after_id"},
	}
	for _, c := range cases {
		status, out := doJSON(t, "GET", c.url, tok, "")
		assert.Equal(t, http.StatusBadRequest, status, "%s: %v", c.url, out)
		assert.Equal(t, "VALIDATION_ERROR", entryErrorCode(out), c.url)
		e, _ := out["error"].(map[string]any)
		msg, _ := e["message"].(string)
		assert.True(t, strings.Contains(msg, c.param), "%s: error must name %q, got %q", c.url, c.param, msg)
	}

	// Every documented parameter is still accepted; the credential now rides in the
	// Authorization header, never in the URL (idx 75 step 5, TestURLCredentials_*).
	for _, u := range []string{
		entries + "?kind=event&issue=APP-1&limit=5",
		events + "?type=CLAIM&issue=APP-1&limit=5",
	} {
		status, out := doJSON(t, "GET", u, tok, "")
		assert.Equal(t, http.StatusOK, status, "%s: %v", u, out)
	}
}
