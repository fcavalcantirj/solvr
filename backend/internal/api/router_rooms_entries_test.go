package api

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Canonical room entries API (task: "Offer one canonical room API with thin adapters for
// the existing transport routes"). GET/POST /v1/rooms/{slug}/entries and
// GET /v1/rooms/{slug}/entries/{entry_id} accept human, agent-account and room-scoped
// credentials through one authorization policy; the /r/{slug}/message and
// /v1/rooms/{slug}/messages transport routes are adapters into the same submission path.

// entriesTestRoom creates a room through the API with the given bearer and returns its slug.
func entriesTestRoom(t *testing.T, base, bearer string, private bool) string {
	t.Helper()
	slug := fmt.Sprintf("test-entries-%d", time.Now().UnixNano()%100000000)
	status, out := doJSON(t, "POST", base+"/v1/rooms", bearer,
		fmt.Sprintf(`{"display_name":"Entries %s","slug":%q,"is_private":%t}`, slug, slug, private))
	require.Equal(t, http.StatusCreated, status, "create room: %v", out)
	return slug
}

func entryData(t *testing.T, out map[string]any) map[string]any {
	t.Helper()
	data, ok := out["data"].(map[string]any)
	require.True(t, ok, "expected data object, got %v", out)
	return data
}

func entryID(t *testing.T, out map[string]any) int64 {
	t.Helper()
	id, ok := entryData(t, out)["id"].(float64)
	require.True(t, ok, "expected numeric id in %v", out)
	return int64(id)
}

func entryErrorCode(out map[string]any) string {
	e, _ := out["error"].(map[string]any)
	code, _ := e["code"].(string)
	return code
}

func TestRoomEntries_CanonicalAndAdapterWritesShareOneSubmissionPath(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	roomPreCleanup(t, pool)
	ctx := context.Background()

	userID, jwt := createRoomTestUser(t, pool)
	slug := entriesTestRoom(t, ts.URL, jwt, false)
	agentID, agentKey := registerRoomTestAgent(t, ts)
	roomTok := handshakeRoomToken(t, ts, slug, agentKey)
	entriesURL := ts.URL + "/v1/rooms/" + slug + "/entries"

	messageCount := func() int {
		var n int
		require.NoError(t, pool.QueryRow(ctx, `SELECT message_count FROM rooms WHERE slug = $1`, slug).Scan(&n))
		return n
	}
	storedWithKey := func(author, key string) int {
		var n int
		require.NoError(t, pool.QueryRow(ctx, `SELECT COUNT(*) FROM room_entries e JOIN rooms r ON r.id = e.room_id
			WHERE r.slug = $1 AND e.author_id = $2 AND e.client_entry_id = $3`, slug, author, key).Scan(&n))
		return n
	}

	// Canonical write with the room-scoped credential.
	status, out := doJSON(t, "POST", entriesURL, roomTok, `{"body":"plan: step one","client_entry_id":"c1"}`)
	require.Equal(t, http.StatusCreated, status, "canonical room-token write: %v", out)
	canonical := entryData(t, out)
	assert.Equal(t, "message", canonical["kind"])
	assert.Equal(t, "agent", canonical["author_type"])
	assert.Equal(t, agentID, canonical["author_id"])
	assert.Equal(t, "plan: step one", canonical["body"])
	firstID := entryID(t, out)

	// Idempotent retry: same entry, one stored row, no second side effect.
	status, out = doJSON(t, "POST", entriesURL, roomTok, `{"body":"plan: step one","client_entry_id":"c1"}`)
	require.Equal(t, http.StatusOK, status, "canonical retry: %v", out)
	assert.Equal(t, firstID, entryID(t, out))
	meta, _ := out["meta"].(map[string]any)
	assert.Equal(t, true, meta["idempotent_replay"])
	assert.Equal(t, 1, storedWithKey(agentID, "c1"))
	assert.Equal(t, 1, messageCount(), "a replay must not count twice")

	// Adapter write with the same credential lands in the same timeline with the same
	// authoritative attribution, and is readable through the canonical entry route.
	status, out = doJSON(t, "POST", ts.URL+"/r/"+slug+"/message", roomTok,
		`{"agent_name":"`+agentID+`","content":"build: done","client_entry_id":"c2"}`)
	require.Equal(t, http.StatusCreated, status, "adapter write: %v", out)
	adapterID := entryID(t, out)
	status, out = doJSON(t, "GET", entriesURL+"/"+strconv.FormatInt(adapterID, 10), "", "")
	require.Equal(t, http.StatusOK, status, "canonical read of adapter entry: %v", out)
	adapterEntry := entryData(t, out)
	assert.Equal(t, canonical["author_type"], adapterEntry["author_type"])
	assert.Equal(t, canonical["author_id"], adapterEntry["author_id"])
	assert.Equal(t, "build: done", adapterEntry["body"])

	// The idempotency key is shared across canonical and adapter paths: a canonical
	// write reusing the adapter's key returns the adapter's entry, never a duplicate.
	status, out = doJSON(t, "POST", entriesURL, roomTok, `{"body":"build: done","client_entry_id":"c2"}`)
	require.Equal(t, http.StatusOK, status, "cross-path replay: %v", out)
	assert.Equal(t, adapterID, entryID(t, out))
	assert.Equal(t, 1, storedWithKey(agentID, "c2"))
	status, out = doJSON(t, "POST", ts.URL+"/r/"+slug+"/message", roomTok,
		`{"agent_name":"`+agentID+`","content":"plan: step one","client_entry_id":"c1"}`)
	require.Equal(t, http.StatusOK, status, "adapter replay of canonical key: %v", out)
	assert.Equal(t, firstID, entryID(t, out))
	assert.Equal(t, 2, messageCount())

	// The same agent through its account credential: identical attribution.
	status, out = doJSON(t, "POST", entriesURL, agentKey, `{"body":"via account key"}`)
	require.Equal(t, http.StatusCreated, status, "canonical agent-key write: %v", out)
	assert.Equal(t, "agent", entryData(t, out)["author_type"])
	assert.Equal(t, agentID, entryData(t, out)["author_id"])

	// A human through the canonical route and through the /messages adapter.
	status, out = doJSON(t, "POST", entriesURL, jwt, `{"body":"human canonical"}`)
	require.Equal(t, http.StatusCreated, status, "canonical human write: %v", out)
	humanCanonical := entryData(t, out)
	assert.Equal(t, "human", humanCanonical["author_type"])
	assert.Equal(t, userID, humanCanonical["author_id"])
	status, out = doJSON(t, "POST", ts.URL+"/v1/rooms/"+slug+"/messages", jwt, `{"content":"human adapter"}`)
	require.Equal(t, http.StatusCreated, status, "adapter human write: %v", out)
	status, out = doJSON(t, "GET", entriesURL+"/"+strconv.FormatInt(entryID(t, out), 10), "", "")
	require.Equal(t, http.StatusOK, status)
	assert.Equal(t, humanCanonical["author_type"], entryData(t, out)["author_type"])
	assert.Equal(t, humanCanonical["author_id"], entryData(t, out)["author_id"])
	assert.Equal(t, humanCanonical["actor_label"], entryData(t, out)["actor_label"])
	assert.Equal(t, 5, messageCount(), "one count per stored entry, across both paths")

	// Validation is shared: an empty body is refused on both paths.
	status, out = doJSON(t, "POST", entriesURL, roomTok, `{"body":""}`)
	assert.Equal(t, http.StatusBadRequest, status, "%v", out)
	status, out = doJSON(t, "POST", ts.URL+"/r/"+slug+"/message", roomTok, `{"agent_name":"x","content":""}`)
	assert.Equal(t, http.StatusBadRequest, status, "%v", out)
}

func TestRoomEntries_OneAuthorizationPolicyAcrossCredentials(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	roomPreCleanup(t, pool)

	_, ownerJWT := createRoomTestUser(t, pool)
	_, outsiderJWT := createRoomTestUser(t, pool)
	private := entriesTestRoom(t, ts.URL, ownerJWT, true)
	public := entriesTestRoom(t, ts.URL, ownerJWT, false)
	privateEntries := ts.URL + "/v1/rooms/" + private + "/entries"

	memberID, memberKey := registerRoomTestAgent(t, ts)
	status, out := doJSON(t, "POST", ts.URL+"/v1/rooms/"+private+"/members", ownerJWT, `{"agent_id":"`+memberID+`"}`)
	require.Equal(t, http.StatusCreated, status, "owner admits member: %v", out)
	memberTok := handshakeRoomToken(t, ts, private, memberKey)

	_, outsiderKey := registerRoomTestAgent(t, ts)
	publicTok := handshakeRoomToken(t, ts, public, outsiderKey) // valid, but for ANOTHER room

	body := `{"body":"hello"}`
	cases := []struct {
		name        string
		method, url string
		bearer      string
		body        string
		want        int
	}{
		{"anonymous read of private", "GET", privateEntries, "", "", http.StatusForbidden},
		{"anonymous write", "POST", privateEntries, "", body, http.StatusUnauthorized},
		{"outsider agent key read", "GET", privateEntries, outsiderKey, "", http.StatusForbidden},
		{"outsider agent key write", "POST", privateEntries, outsiderKey, body, http.StatusForbidden},
		{"other room's token read", "GET", privateEntries, publicTok, "", http.StatusForbidden},
		{"other room's token write", "POST", privateEntries, publicTok, body, http.StatusForbidden},
		{"other room's token on /r adapter", "POST", ts.URL + "/r/" + private + "/message", publicTok,
			`{"agent_name":"x","content":"hello"}`, http.StatusForbidden},
		{"invalid room token", "POST", privateEntries, "solvr_rt_notarealtoken", body, http.StatusUnauthorized},
		{"outsider human write canonical", "POST", privateEntries, outsiderJWT, body, http.StatusForbidden},
		{"outsider human write adapter", "POST", ts.URL + "/v1/rooms/" + private + "/messages", outsiderJWT,
			`{"content":"hello"}`, http.StatusForbidden},
		{"member token read", "GET", privateEntries, memberTok, "", http.StatusOK},
		{"member token write", "POST", privateEntries, memberTok, body, http.StatusCreated},
		{"member agent key write", "POST", privateEntries, memberKey, body, http.StatusCreated},
		{"owner human write", "POST", privateEntries, ownerJWT, body, http.StatusCreated},
		{"public room read anonymously", "GET", ts.URL + "/v1/rooms/" + public + "/entries", "", "", http.StatusOK},
		{"public room write with its own token", "POST", ts.URL + "/v1/rooms/" + public + "/entries", publicTok, body, http.StatusCreated},
	}
	for _, c := range cases {
		status, out := doJSON(t, c.method, c.url, c.bearer, c.body)
		assert.Equal(t, c.want, status, "%s: %v", c.name, out)
	}

	// A finished room refuses writes identically on the canonical and adapter paths.
	status, out = doJSON(t, "POST", ts.URL+"/v1/rooms/"+private+"/archive", ownerJWT, "")
	require.Equal(t, http.StatusOK, status, "archive: %v", out)
	status, out = doJSON(t, "POST", privateEntries, memberTok, body)
	assert.Equal(t, http.StatusConflict, status)
	assert.Equal(t, "ROOM_ARCHIVED", entryErrorCode(out))
	status, out = doJSON(t, "POST", ts.URL+"/r/"+private+"/message", memberTok, `{"agent_name":"x","content":"hello"}`)
	assert.Equal(t, http.StatusConflict, status)
	assert.Equal(t, "ROOM_ARCHIVED", entryErrorCode(out))
}

func TestRoomEntries_ListsTheWholeTimelineWithCursorPagination(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	roomPreCleanup(t, pool)

	_, jwt := createRoomTestUser(t, pool)
	slug := entriesTestRoom(t, ts.URL, jwt, false)
	_, agentKey := registerRoomTestAgent(t, ts)
	tok := handshakeRoomToken(t, ts, slug, agentKey)
	entriesURL := ts.URL + "/v1/rooms/" + slug + "/entries"

	for i := 1; i <= 4; i++ {
		status, out := doJSON(t, "POST", entriesURL, tok, fmt.Sprintf(`{"body":"m%d"}`, i))
		require.Equal(t, http.StatusCreated, status, "%v", out)
	}
	status, out := doJSON(t, "POST", ts.URL+"/r/"+slug+"/events", tok, `{"type":"status","actor":"x","issue":"I-1"}`)
	require.Equal(t, http.StatusCreated, status, "event: %v", out)

	// Page through the whole ordered timeline (messages AND events).
	var seen []string
	var sequences []float64
	next := ""
	for pages := 0; pages < 10; pages++ {
		url := entriesURL + "?limit=2"
		if next != "" {
			url += "&cursor=" + next
		}
		status, out = doJSON(t, "GET", url, "", "")
		require.Equal(t, http.StatusOK, status, "%v", out)
		data, _ := out["data"].([]any)
		assert.LessOrEqual(t, len(data), 2)
		for _, raw := range data {
			e := raw.(map[string]any)
			sequences = append(sequences, e["sequence"].(float64))
			if e["kind"] == "event" {
				seen = append(seen, "event:"+e["event_type"].(string))
			} else {
				seen = append(seen, e["body"].(string))
			}
		}
		meta, _ := out["meta"].(map[string]any)
		if meta["has_more"] != true {
			assert.Nil(t, meta["next_cursor"])
			break
		}
		next, _ = meta["next_cursor"].(string)
		require.NotEmpty(t, next)
	}
	assert.Equal(t, []string{"m1", "m2", "m3", "m4", "event:status"}, seen)
	for i := 1; i < len(sequences); i++ {
		assert.Less(t, sequences[i-1], sequences[i], "strictly ascending timeline order")
	}

	// Kind filter and bounds.
	status, out = doJSON(t, "GET", entriesURL+"?kind=event", "", "")
	require.Equal(t, http.StatusOK, status)
	data, _ := out["data"].([]any)
	assert.Len(t, data, 1)
	status, _ = doJSON(t, "GET", entriesURL+"?kind=bogus", "", "")
	assert.Equal(t, http.StatusBadRequest, status)
	status, _ = doJSON(t, "GET", entriesURL+"?cursor=not-a-cursor", "", "")
	assert.Equal(t, http.StatusBadRequest, status)
	status, out = doJSON(t, "GET", entriesURL+"?limit=1000", "", "")
	require.Equal(t, http.StatusOK, status)
	meta, _ := out["meta"].(map[string]any)
	assert.Equal(t, float64(100), meta["limit"], "limit is clamped to 100")
	status, out = doJSON(t, "GET", entriesURL, "", "")
	require.Equal(t, http.StatusOK, status)
	meta, _ = out["meta"].(map[string]any)
	assert.Equal(t, float64(50), meta["limit"], "default limit is 50")

	// Single entry: room-scoped, 404 for another room's id, 400 for garbage.
	other := entriesTestRoom(t, ts.URL, jwt, false)
	status, out = doJSON(t, "POST", ts.URL+"/v1/rooms/"+other+"/entries", jwt, `{"body":"elsewhere"}`)
	require.Equal(t, http.StatusCreated, status)
	foreignID := strconv.FormatInt(entryID(t, out), 10)
	status, _ = doJSON(t, "GET", entriesURL+"/"+foreignID, "", "")
	assert.Equal(t, http.StatusNotFound, status)
	status, _ = doJSON(t, "GET", entriesURL+"/abc", "", "")
	assert.Equal(t, http.StatusBadRequest, status)
}

func TestRoomEntries_HumanWriteRateLimitIsSharedWithTheAdapter(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	roomPreCleanup(t, pool)

	_, jwt := createRoomTestUser(t, pool)
	slug := entriesTestRoom(t, ts.URL, jwt, false)

	// The human write budget (10/min per IP) is one bucket for both routes.
	for i := 0; i < 10; i++ {
		status, out := doJSON(t, "POST", ts.URL+"/v1/rooms/"+slug+"/messages", jwt, fmt.Sprintf(`{"content":"a%d"}`, i))
		require.Equal(t, http.StatusCreated, status, "adapter write %d: %v", i, out)
	}
	status, _ := doJSON(t, "POST", ts.URL+"/v1/rooms/"+slug+"/entries", jwt, `{"body":"over budget"}`)
	assert.Equal(t, http.StatusTooManyRequests, status)
}
