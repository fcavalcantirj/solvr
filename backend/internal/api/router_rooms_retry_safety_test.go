package api

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/stretchr/testify/require"
)

// keyedRequest sends method path with a bearer and an Idempotency-Key and returns
// (status, Idempotent-Replayed, raw body).
func keyedRequest(t *testing.T, ts *httptest.Server, method, path, bearer, key, body string) (int, string, string) {
	t.Helper()
	var rdr io.Reader
	if body != "" {
		rdr = strings.NewReader(body)
	}
	req, _ := http.NewRequest(method, ts.URL+path, rdr)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Authorization", "Bearer "+bearer)
	req.Header.Set("Idempotency-Key", key)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, resp.Header.Get("Idempotent-Replayed"), string(raw)
}

func outcomeBody(title string) string {
	return fmt.Sprintf(`{"title":%q,"summary":%q,"tags":["outcome"]}`, title, outcomeSummary)
}

// Saving a room outcome is a post creation: its Idempotency-Key follows the same contract
// as POST /v1/posts. A retry replays the first result; the same key with another payload
// or for another room is refused and never hands back a different room's draft.
func TestSaveAsPost_IdempotencyKeyNamesOneOutcome(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	roomPreCleanup(t, pool)

	agentID, apiKey := registerRoomTestAgent(t, ts)
	t.Cleanup(func() {
		ctx := context.Background()
		pool.Exec(ctx, "DELETE FROM idempotency_keys WHERE actor_id = $1", agentID) //nolint:errcheck
		pool.Exec(ctx, "DELETE FROM posts WHERE posted_by_id = $1", agentID)        //nolint:errcheck
	})
	slugA, roomA := createRoomReturningID(t, ts, apiKey, false)
	slugB, _ := createRoomReturningID(t, ts, apiKey, false)
	posts := func() int {
		return countRows(t, pool, "SELECT COUNT(*) FROM posts WHERE posted_by_id = $1", agentID)
	}

	body := outcomeBody("Outcome saved exactly once")
	s1, rp1, raw1 := keyedRequest(t, ts, "POST", "/v1/rooms/"+slugA+"/save-as-post", apiKey, "outcome-key", body)
	require.Equal(t, http.StatusCreated, s1, raw1)
	require.Empty(t, rp1)
	require.Contains(t, raw1, roomA)

	s2, rp2, raw2 := keyedRequest(t, ts, "POST", "/v1/rooms/"+slugA+"/save-as-post", apiKey, "outcome-key", body)
	require.Equal(t, http.StatusCreated, s2, "a retry replays the original 201: %s", raw2)
	require.Equal(t, "true", rp2)
	require.Equal(t, raw1, raw2, "a retry replays the stored response byte for byte")
	require.Equal(t, 1, posts())

	s3, _, raw3 := keyedRequest(t, ts, "POST", "/v1/rooms/"+slugA+"/save-as-post", apiKey, "outcome-key", outcomeBody("Another outcome, same key"))
	require.Equal(t, http.StatusConflict, s3, raw3)
	require.Contains(t, raw3, "IDEMPOTENCY_KEY_REUSED")
	require.Contains(t, raw3, "request_id")
	require.Equal(t, 1, posts())

	s4, _, raw4 := keyedRequest(t, ts, "POST", "/v1/rooms/"+slugB+"/save-as-post", apiKey, "outcome-key", body)
	require.Equal(t, http.StatusConflict, s4, "the key already names room A's outcome: %s", raw4)
	require.Contains(t, raw4, "IDEMPOTENCY_KEY_REUSED")
	require.NotContains(t, raw4, roomA, "room B's save must never answer with room A's draft")
	require.Equal(t, 1, posts())

	// Past the 24h retention the stored response is gone, but the draft still carries the
	// key: another room is still refused, the original room still gets its own draft.
	_, err := pool.Exec(context.Background(),
		"UPDATE idempotency_keys SET created_at = NOW() - INTERVAL '25 hours' WHERE actor_id = $1", agentID)
	require.NoError(t, err)
	s5, _, raw5 := keyedRequest(t, ts, "POST", "/v1/rooms/"+slugB+"/save-as-post", apiKey, "outcome-key", body)
	require.Equal(t, http.StatusConflict, s5, raw5)
	require.Contains(t, raw5, "IDEMPOTENCY_KEY_REUSED")
	require.NotContains(t, raw5, roomA)
	require.Equal(t, 1, posts())
	s6, _, raw6 := keyedRequest(t, ts, "POST", "/v1/rooms/"+slugA+"/save-as-post", apiKey, "outcome-key", body)
	require.Less(t, s6, 300, raw6)
	require.Contains(t, raw6, roomA)
	require.Equal(t, 1, posts())
}

// Owner approval publishes a draft outcome once. A retried approval of a published
// outcome changes nothing, and a late one cannot republish an outcome that was archived
// or rejected by moderation after it was approved.
func TestApprovePublication_RetryCannotRepublish(t *testing.T) {
	useRecordingModerator(t) // anti-abuse D5c: approval submits to moderation, which approves
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	roomPreCleanup(t, pool)

	agentID, ownerKey := registerRoomTestAgent(t, ts)
	t.Cleanup(func() {
		pool.Exec(context.Background(), "DELETE FROM posts WHERE posted_by_id = $1", agentID) //nolint:errcheck
	})
	slug, _ := createRoomReturningID(t, ts, ownerKey, true)
	ctx := context.Background()
	state := func(postID string) (string, string, string, time.Time) {
		var status, pub, mod string
		var updated time.Time
		require.NoError(t, pool.QueryRow(ctx,
			"SELECT status, publication_state, moderation_state, updated_at FROM posts WHERE id = $1", postID,
		).Scan(&status, &pub, &mod, &updated))
		return status, pub, mod, updated
	}
	saveDraft := func(title string) string {
		resp := saveAsPost(t, ts, slug, ownerKey, title, outcomeSummary, "")
		require.Equal(t, http.StatusCreated, resp.StatusCode)
		id, _ := dataObj(t, resp)["id"].(string)
		require.NotEmpty(t, id)
		return id
	}
	approveURL := func(postID string) string { return ts.URL + "/v1/rooms/" + slug + "/posts/" + postID + "/publish" }

	postID := saveDraft("Outcome approved exactly once")
	st, out := doJSON(t, "POST", approveURL(postID), ownerKey, "")
	require.Equal(t, http.StatusOK, st, "%v", out)
	waitForValue(t, pool, "published", `SELECT publication_state FROM posts WHERE id = $1::uuid`, postID)
	status, pub, _, approvedAt := state(postID)
	require.Equal(t, "open", status)
	require.Equal(t, "published", pub)

	st, out = doJSON(t, "POST", approveURL(postID), ownerKey, "")
	require.Equal(t, http.StatusOK, st, "a retried approval answers with the published outcome: %v", out)
	data, _ := out["data"].(map[string]any)
	require.Equal(t, "published", data["publication_state"])
	_, _, _, after := state(postID)
	require.True(t, after.Equal(approvedAt), "a retried approval must not write again")

	// The author archives the published outcome; a late approval must not reopen it.
	st, out = doJSON(t, "PATCH", ts.URL+"/v1/posts/"+postID, ownerKey, `{"status":"closed"}`)
	require.Equal(t, http.StatusOK, st, "%v", out)
	st, out = doJSON(t, "POST", approveURL(postID), ownerKey, "")
	require.Equal(t, http.StatusConflict, st, "%v", out)
	errObj, _ := out["error"].(map[string]any)
	require.Equal(t, "PUBLICATION_STATE_CONFLICT", errObj["code"])
	status, pub, _, _ = state(postID)
	require.Equal(t, "closed", status)
	require.Equal(t, "archived", pub)

	// Moderation rejects a draft; an approval arriving afterwards must not publish it.
	rejectedID := saveDraft("Outcome rejected by moderation")
	require.NoError(t, db.NewPostRepository(pool).UpdateStatus(ctx, rejectedID, models.PostStatusRejected))
	st, out = doJSON(t, "POST", approveURL(rejectedID), ownerKey, "")
	require.Equal(t, http.StatusConflict, st, "%v", out)
	status, pub, mod, _ := state(rejectedID)
	require.Equal(t, "rejected", status)
	require.Equal(t, "draft", pub)
	require.Equal(t, "rejected", mod)
}

// Membership changes are where room ownership lives. A plain re-add (no role) keeps the
// role the member holds, so a retried add cannot demote an owner promoted meanwhile; and
// a keyed add or removal replays its first result instead of re-applying a stale change.
func TestRoomMembers_RetryCannotChangeOwnership(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	roomPreCleanup(t, pool)

	ownerID, ownerKey := registerRoomTestAgent(t, ts)
	heirID, _ := registerRoomTestAgent(t, ts)
	peerID, _ := registerRoomTestAgent(t, ts)
	t.Cleanup(func() {
		pool.Exec(context.Background(), "DELETE FROM idempotency_keys WHERE actor_id = $1", ownerID) //nolint:errcheck
	})
	slug, _ := createTestRoomWithAgentKey(t, ts, ownerKey)
	membersPath := "/v1/rooms/" + slug + "/members"
	membersURL := ts.URL + membersPath
	role := func(agentID string) string { return memberRole(t, ts.URL, slug, ownerKey, agentID) }

	// Plain add, then promotion, then the plain add retried: the heir stays an owner.
	st, out := doJSON(t, "POST", membersURL, ownerKey, `{"agent_id":"`+heirID+`"}`)
	require.Equal(t, http.StatusCreated, st, "%v", out)
	require.Equal(t, "member", role(heirID))
	st, out = doJSON(t, "POST", membersURL, ownerKey, `{"agent_id":"`+heirID+`","role":"owner"}`)
	require.Equal(t, http.StatusCreated, st, "%v", out)
	st, out = doJSON(t, "POST", membersURL, ownerKey, `{"agent_id":"`+heirID+`"}`)
	require.Equal(t, http.StatusCreated, st, "%v", out)
	require.Equal(t, "owner", role(heirID), "a re-add without a role must not demote an owner")

	// Keyed promotion, then an explicit demotion, then the keyed promotion retried: the
	// retry replays and the newer demotion stands.
	promote := `{"agent_id":"` + peerID + `","role":"owner"}`
	s1, rp1, raw1 := keyedRequest(t, ts, "POST", membersPath, ownerKey, "promote-peer", promote)
	require.Equal(t, http.StatusCreated, s1, raw1)
	require.Empty(t, rp1)
	st, out = doJSON(t, "POST", membersURL, ownerKey, `{"agent_id":"`+peerID+`","role":"member"}`)
	require.Equal(t, http.StatusCreated, st, "%v", out)
	s2, rp2, raw2 := keyedRequest(t, ts, "POST", membersPath, ownerKey, "promote-peer", promote)
	require.Equal(t, http.StatusCreated, s2, raw2)
	require.Equal(t, "true", rp2)
	require.Equal(t, raw1, raw2)
	require.Equal(t, "member", role(peerID), "a replayed promotion must not re-promote")
	s3, _, raw3 := keyedRequest(t, ts, "POST", membersPath, ownerKey, "promote-peer", `{"agent_id":"`+heirID+`","role":"member"}`)
	require.Equal(t, http.StatusConflict, s3, raw3)
	require.Contains(t, raw3, "IDEMPOTENCY_KEY_REUSED")
	require.Equal(t, "owner", role(heirID), "a reused key must not demote anyone")

	// Keyed removal, readmission, then the keyed removal retried: the member stays.
	s4, _, raw4 := keyedRequest(t, ts, "DELETE", membersPath+"/"+peerID, ownerKey, "remove-peer", "")
	require.Equal(t, http.StatusNoContent, s4, raw4)
	require.Equal(t, "", role(peerID))
	st, out = doJSON(t, "POST", membersURL, ownerKey, `{"agent_id":"`+peerID+`"}`)
	require.Equal(t, http.StatusCreated, st, "%v", out)
	s5, rp5, raw5 := keyedRequest(t, ts, "DELETE", membersPath+"/"+peerID, ownerKey, "remove-peer", "")
	require.Equal(t, http.StatusNoContent, s5, raw5)
	require.Equal(t, "true", rp5)
	require.Equal(t, "member", role(peerID), "a replayed removal must not remove the readmitted member")
}
