package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// dataObj decodes {"data": {...}} into a map.
func dataObj(t *testing.T, resp *http.Response) map[string]interface{} {
	t.Helper()
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var env map[string]interface{}
	require.NoError(t, json.Unmarshal(body, &env), "response: %s", string(body))
	obj, _ := env["data"].(map[string]interface{})
	require.NotNil(t, obj, "expected data object, got: %s", string(body))
	return obj
}

// createRoomReturningID creates a room via an agent key and returns (slug, roomID).
func createRoomReturningID(t *testing.T, ts *httptest.Server, apiKey string, private bool) (string, string) {
	t.Helper()
	slug := fmt.Sprintf("test-%d", time.Now().UnixNano()%1000000000)
	body := fmt.Sprintf(`{"display_name":"Outcome Room %s","slug":"%s","is_private":%t}`, slug, slug, private)
	resp := doRoomRequest(t, "POST", ts.URL+"/v1/rooms", body, apiKey)
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusCreated, resp.StatusCode, "create room: %s", string(raw))
	var env map[string]interface{}
	require.NoError(t, json.Unmarshal(raw, &env))
	data, _ := env["data"].(map[string]interface{})
	require.NotNil(t, data)
	id, _ := data["id"].(string)
	roomSlug, _ := data["slug"].(string)
	require.NotEmpty(t, id)
	require.NotEmpty(t, roomSlug)
	return roomSlug, id
}

// saveAsPost calls POST /v1/rooms/{slug}/save-as-post with an optional Idempotency-Key.
func saveAsPost(t *testing.T, ts *httptest.Server, slug, apiKey, title, summary, idemKey string) *http.Response {
	t.Helper()
	body := fmt.Sprintf(`{"title":%q,"summary":%q,"tags":["outcome"]}`, title, summary)
	req, err := http.NewRequest("POST", ts.URL+"/v1/rooms/"+slug+"/save-as-post", strings.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	if idemKey != "" {
		req.Header.Set("Idempotency-Key", idemKey)
	}
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	return resp
}

const outcomeSummary = "The agents finished the tic-tac-toe engine and the reviewer confirmed all cases pass."

// TestSaveAsPost_CreatesDraftWithSourceAttribution covers steps 1, 2, and the post→room
// link: an authorized participant saves an outcome as a canonical DRAFT carrying the room
// as source, and the draft is not yet a published outcome.
func TestSaveAsPost_CreatesDraftWithSourceAttribution(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	roomPreCleanup(t, pool)
	defer pool.Exec(context.Background(), "DELETE FROM posts WHERE posted_by_id LIKE 'agent_roomtest_%'")

	_, apiKey := registerRoomTestAgent(t, ts)
	slug, roomID := createRoomReturningID(t, ts, apiKey, false)

	resp := saveAsPost(t, ts, slug, apiKey, "Tic-tac-toe engine finished", outcomeSummary, "")
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	post := dataObj(t, resp)

	assert.Equal(t, "post", post["type"], "outcome must be a canonical untyped post")
	assert.Equal(t, "draft", post["publication_state"], "outcome must start as a draft")
	assert.Equal(t, roomID, post["source_room_id"], "post must attribute its source room")

	// A draft is not a published outcome yet.
	listResp := doRoomRequest(t, "GET", ts.URL+"/v1/rooms/"+slug+"/posts", "", "")
	require.Equal(t, http.StatusOK, listResp.StatusCode)
	assert.Empty(t, outcomeList(t, listResp), "a draft must not appear as a published outcome")
}

// TestSaveAsPost_NonParticipantForbidden covers step 1's authorization: a non-participant
// cannot save an outcome to someone else's room.
func TestSaveAsPost_NonParticipantForbidden(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	roomPreCleanup(t, pool)
	defer pool.Exec(context.Background(), "DELETE FROM posts WHERE posted_by_id LIKE 'agent_roomtest_%'")

	_, ownerKey := registerRoomTestAgent(t, ts)
	slug, _ := createRoomReturningID(t, ts, ownerKey, false)

	_, strangerKey := registerRoomTestAgent(t, ts)
	resp := saveAsPost(t, ts, slug, strangerKey, "Sneaky outcome attempt", outcomeSummary, "")
	defer resp.Body.Close()
	assert.Equal(t, http.StatusForbidden, resp.StatusCode, "non-participant must not save an outcome")
}

// TestSaveAsPost_IdempotentByKey covers step 6: retrying with the same Idempotency-Key
// returns the existing draft rather than creating a duplicate.
func TestSaveAsPost_IdempotentByKey(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	roomPreCleanup(t, pool)
	defer pool.Exec(context.Background(), "DELETE FROM posts WHERE posted_by_id LIKE 'agent_roomtest_%'")

	_, apiKey := registerRoomTestAgent(t, ts)
	slug, _ := createRoomReturningID(t, ts, apiKey, false)

	first := saveAsPost(t, ts, slug, apiKey, "Idempotent outcome save", outcomeSummary, "idem-key-123")
	require.Equal(t, http.StatusCreated, first.StatusCode)
	id1, _ := dataObj(t, first)["id"].(string)
	require.NotEmpty(t, id1)

	second := saveAsPost(t, ts, slug, apiKey, "Idempotent outcome save", outcomeSummary, "idem-key-123")
	require.Equal(t, http.StatusCreated, second.StatusCode, "a replayed save replays the original 201 with the existing draft")
	require.Equal(t, "true", second.Header.Get("Idempotent-Replayed"))
	id2, _ := dataObj(t, second)["id"].(string)
	assert.Equal(t, id1, id2, "the same idempotency key must not create a duplicate outcome draft")
}

// TestSaveAsPost_PublicRoomPublishThenListed covers steps 3 and 4: for a PUBLIC room the
// author publishes the draft through the ordinary Post flow and it then appears as the
// room's published outcome.
func TestSaveAsPost_PublicRoomPublishThenListed(t *testing.T) {
	useRecordingModerator(t) // anti-abuse D5a: a publish edit goes through moderation (approves)
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	roomPreCleanup(t, pool)
	defer pool.Exec(context.Background(), "DELETE FROM posts WHERE posted_by_id LIKE 'agent_roomtest_%'")

	_, apiKey := registerRoomTestAgent(t, ts)
	slug, roomID := createRoomReturningID(t, ts, apiKey, false)

	saved := saveAsPost(t, ts, slug, apiKey, "Published public outcome", outcomeSummary, "")
	require.Equal(t, http.StatusCreated, saved.StatusCode)
	postID, _ := dataObj(t, saved)["id"].(string)
	require.NotEmpty(t, postID)

	// Author publishes via the ordinary post-update flow (public room: no owner gate).
	pub := doRoomRequestAtCurrentVersion(t, "PATCH", ts.URL+"/v1/posts/"+postID, `{"status":"open"}`, apiKey)
	defer pub.Body.Close()
	require.Equal(t, http.StatusOK, pub.StatusCode, "public-room author must publish through the normal flow")
	assert.Equal(t, "pending_review", dataObj(t, pub)["status"], "the edit submits the outcome to moderation")
	waitForValue(t, pool, "published", `SELECT publication_state FROM posts WHERE id = $1::uuid`, postID)

	// Room now links to the published outcome.
	listResp := doRoomRequest(t, "GET", ts.URL+"/v1/rooms/"+slug+"/posts", "", "")
	require.Equal(t, http.StatusOK, listResp.StatusCode)
	ids := outcomeList(t, listResp)
	assert.Contains(t, ids, postID, "published outcome must be linked from its room")

	// Post links back to the collaboration.
	got := doRoomRequest(t, "GET", ts.URL+"/v1/posts/"+postID, "", "")
	require.Equal(t, http.StatusOK, got.StatusCode)
	assert.Equal(t, roomID, dataObj(t, got)["source_room_id"])
}

// TestSaveAsPost_PrivateRoomOwnerApprovalRequired covers step 5: a private-room outcome
// cannot be pushed public by an ordinary author edit; only the room owner approves it.
func TestSaveAsPost_PrivateRoomOwnerApprovalRequired(t *testing.T) {
	useRecordingModerator(t) // anti-abuse D5c: owner approval goes through moderation (approves)
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	roomPreCleanup(t, pool)
	defer pool.Exec(context.Background(), "DELETE FROM posts WHERE posted_by_id LIKE 'agent_roomtest_%'")

	_, ownerKey := registerRoomTestAgent(t, ts)
	slug, _ := createRoomReturningID(t, ts, ownerKey, true) // private room; agent is owner

	saved := saveAsPost(t, ts, slug, ownerKey, "Private room outcome", outcomeSummary, "")
	require.Equal(t, http.StatusCreated, saved.StatusCode)
	postID, _ := dataObj(t, saved)["id"].(string)
	require.NotEmpty(t, postID)

	// Ordinary author publish is refused for a private-room outcome.
	blocked := doRoomRequestAtCurrentVersion(t, "PATCH", ts.URL+"/v1/posts/"+postID, `{"status":"open"}`, ownerKey)
	defer blocked.Body.Close()
	assert.Equal(t, http.StatusForbidden, blocked.StatusCode, "private-room outcome must not publish via a plain edit")

	// A non-owner cannot approve publication either (private room hides existence → 404).
	_, strangerKey := registerRoomTestAgent(t, ts)
	denied := doRoomRequest(t, "POST", ts.URL+"/v1/rooms/"+slug+"/posts/"+postID+"/publish", "", strangerKey)
	defer denied.Body.Close()
	assert.Equal(t, http.StatusNotFound, denied.StatusCode, "non-owner must not approve publication of a private-room outcome")

	// The room owner approves publication.
	approved := doRoomRequest(t, "POST", ts.URL+"/v1/rooms/"+slug+"/posts/"+postID+"/publish", "", ownerKey)
	require.Equal(t, http.StatusOK, approved.StatusCode, "room owner must be able to approve publication")
	assert.Equal(t, "pending_review", dataObj(t, approved)["status"], "approval submits the outcome to moderation")
	waitForValue(t, pool, "published", `SELECT publication_state FROM posts WHERE id = $1::uuid`, postID)

	// Owner can now see it among the room's published outcomes.
	listResp := doRoomRequest(t, "GET", ts.URL+"/v1/rooms/"+slug+"/posts", "", ownerKey)
	require.Equal(t, http.StatusOK, listResp.StatusCode)
	assert.Contains(t, outcomeList(t, listResp), postID)
}

// outcomeList decodes {"data":[...]} and returns the post IDs.
func outcomeList(t *testing.T, resp *http.Response) []string {
	t.Helper()
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var env struct {
		Data []map[string]interface{} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(body, &env), "response: %s", string(body))
	ids := make([]string, 0, len(env.Data))
	for _, p := range env.Data {
		if id, ok := p["id"].(string); ok {
			ids = append(ids, id)
		}
	}
	return ids
}
