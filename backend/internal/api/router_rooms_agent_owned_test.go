package api

import (
	"context"
	"net/http"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// An agent that no human owns or claims creates a room and manages it end to end with
// its own credentials alone; room_members is the only owner source (rooms.owner_id is
// retired); a normal member cannot perform owner-only operations; and a later claim
// keeps the room, its messages, its memberships and its URL intact.

// agentOwnedRoomMembers returns the room's active memberships as "agent:<id>:<role>" or
// "user:<id>:<role>" strings.
func agentOwnedRoomMembers(t *testing.T, pool *db.Pool, slug string) []string {
	t.Helper()
	rows, err := pool.Query(context.Background(), `
		SELECT COALESCE('agent:' || m.agent_id, 'user:' || m.user_id::text) || ':' || m.role
		FROM room_members m JOIN rooms r ON r.id = m.room_id
		WHERE r.slug = $1 AND m.revoked_at IS NULL ORDER BY 1`, slug)
	require.NoError(t, err)
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		require.NoError(t, rows.Scan(&s))
		out = append(out, s)
	}
	require.NoError(t, rows.Err())
	return out
}

func agentOwnedRoomMessageCount(t *testing.T, pool *db.Pool, slug string) int {
	t.Helper()
	var n int
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM messages m JOIN rooms r ON r.id = m.room_id WHERE r.slug = $1`, slug).Scan(&n))
	return n
}

func agentOwnedRoomPostMessage(t *testing.T, ts string, slug, roomToken, agentName, content string) {
	t.Helper()
	status, out := doJSON(t, "POST", ts+"/r/"+slug+"/message", roomToken,
		`{"agent_name":"`+agentName+`","content":"`+content+`"}`)
	require.Equal(t, http.StatusCreated, status, "post message: %v", out)
}

func TestAgentOwnedRoom_UnclaimedAgentManagesRoomAndSurvivesLaterClaim(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	roomPreCleanup(t, pool)
	ctx := context.Background()

	// Step 1: a self-registered, unclaimed agent creates the room.
	ownerID, ownerKey := registerRoomTestAgent(t, ts)
	var humanID *string
	require.NoError(t, pool.QueryRow(ctx, `SELECT human_id::text FROM agents WHERE id = $1`, ownerID).Scan(&humanID))
	require.Nil(t, humanID, "the creating agent must be unclaimed")

	slug := "test-agentowned-" + ownerID[len(ownerID)-6:]
	status, out := doJSON(t, "POST", ts.URL+"/v1/rooms", ownerKey,
		`{"display_name":"Agent Owned Room","slug":"`+slug+`"}`)
	require.Equal(t, http.StatusCreated, status, "create room: %v", out)
	data, _ := out["data"].(map[string]any)
	roomID, _ := data["id"].(string)
	require.NotEmpty(t, roomID)
	assert.Nil(t, data["owner_id"], "no human owns the room")

	// Step 2: creation established exactly the agent's owner membership, no human; the
	// retired columns are gone from the final schema.
	assert.Equal(t, []string{"agent:" + ownerID + ":owner"}, agentOwnedRoomMembers(t, pool, slug))
	var retired int
	require.NoError(t, pool.QueryRow(ctx, `SELECT COUNT(*) FROM information_schema.columns
		WHERE table_name = 'rooms' AND column_name IN ('owner_id', 'token_hash')`).Scan(&retired))
	assert.Zero(t, retired, "rooms.owner_id and rooms.token_hash are retired")

	// Step 3: the owner connects with its own key and posts.
	ownerTok := handshakeRoomToken(t, ts, slug, ownerKey)
	agentOwnedRoomPostMessage(t, ts.URL, slug, ownerTok, ownerID, "plan: step one")

	membersURL := ts.URL + "/v1/rooms/" + slug + "/members"
	executorID, executorKey := registerRoomTestAgent(t, ts)
	status, out = doJSON(t, "POST", membersURL, ownerKey, `{"agent_id":"`+executorID+`","role":"member"}`)
	require.Equal(t, http.StatusCreated, status, "owner adds the executor: %v", out)
	tempID, _ := registerRoomTestAgent(t, ts)
	status, _ = doJSON(t, "POST", membersURL, ownerKey, `{"agent_id":"`+tempID+`"}`)
	require.Equal(t, http.StatusCreated, status, "owner adds a second member")
	status, _ = doJSON(t, "DELETE", membersURL+"/"+tempID, ownerKey, "")
	require.Equal(t, http.StatusNoContent, status, "owner removes a member")

	status, out = doJSON(t, "PATCH", ts.URL+"/v1/rooms/"+slug, ownerKey, `{"display_name":"Agent Owned Renamed"}`)
	require.Equal(t, http.StatusOK, status, "owner edits the room: %v", out)
	status, out = doJSON(t, "PATCH", ts.URL+"/v1/rooms/"+slug, ownerKey, `{"is_private":true}`)
	require.Equal(t, http.StatusOK, status, "owner changes visibility: %v", out)
	data, _ = out["data"].(map[string]any)
	assert.Equal(t, true, data["is_private"])

	// Step 4: the executor is an admitted member of the now-private room, yet every
	// owner-only operation is refused.
	executorTok := handshakeRoomToken(t, ts, slug, executorKey)
	agentOwnedRoomPostMessage(t, ts.URL, slug, executorTok, executorID, "build: done")
	forbidden := []struct{ method, url, body string }{
		{"PATCH", ts.URL + "/v1/rooms/" + slug, `{"display_name":"Executor Rename"}`},
		{"PATCH", ts.URL + "/v1/rooms/" + slug, `{"is_private":false}`},
		{"POST", membersURL, `{"agent_id":"` + tempID + `"}`},
		{"POST", membersURL, `{"agent_id":"` + executorID + `","role":"owner"}`},
		{"DELETE", membersURL + "/" + ownerID, ""},
		{"POST", ts.URL + "/v1/rooms/" + slug + "/archive", ""},
		{"POST", ts.URL + "/v1/rooms/" + slug + "/reopen", ""},
		{"DELETE", ts.URL + "/v1/rooms/" + slug, ""},
	}
	for _, f := range forbidden {
		status, out = doJSON(t, f.method, f.url, executorKey, f.body)
		assert.Equal(t, http.StatusForbidden, status, "executor %s %s must be refused: %v", f.method, f.url, out)
	}

	// Step 3 (cont.): the owner archives and reopens, then recovers its connection with a
	// fresh handshake on its own key.
	status, out = doJSON(t, "POST", ts.URL+"/v1/rooms/"+slug+"/archive", ownerKey, "")
	require.Equal(t, http.StatusOK, status, "owner archives: %v", out)
	status, out = doJSON(t, "POST", ts.URL+"/v1/rooms/"+slug+"/reopen", ownerKey, "")
	require.Equal(t, http.StatusOK, status, "owner reopens: %v", out)
	ownerTok = handshakeRoomToken(t, ts, slug, ownerKey)
	agentOwnedRoomPostMessage(t, ts.URL, slug, ownerTok, ownerID, "review: approved")

	membersBefore := agentOwnedRoomMembers(t, pool, slug)
	assert.ElementsMatch(t, []string{"agent:" + executorID + ":member", "agent:" + ownerID + ":owner"}, membersBefore)
	messagesBefore := agentOwnedRoomMessageCount(t, pool, slug)
	require.Equal(t, 3, messagesBefore)

	// Step 5: a human claims the owner agent afterwards through the real claim flow.
	userID, userJWT := createRoomTestUser(t, pool)
	status, out = doJSON(t, "POST", ts.URL+"/v1/agents/me/claim", ownerKey, "")
	require.Contains(t, []int{http.StatusOK, http.StatusCreated}, status, "generate claim: %v", out)
	claimToken, _ := out["token"].(string)
	require.NotEmpty(t, claimToken)
	status, out = doJSON(t, "POST", ts.URL+"/v1/agents/claim", userJWT, `{"token":"`+claimToken+`"}`)
	require.Equal(t, http.StatusOK, status, "claim: %v", out)

	status, out = doJSON(t, "GET", ts.URL+"/v1/rooms/"+slug, ownerKey, "")
	require.Equal(t, http.StatusOK, status, "same URL still resolves: %v", out)
	data, _ = out["data"].(map[string]any)
	data, _ = data["room"].(map[string]any)
	assert.Equal(t, roomID, data["id"], "same room")
	assert.Equal(t, "Agent Owned Renamed", data["display_name"])
	assert.Equal(t, messagesBefore, agentOwnedRoomMessageCount(t, pool, slug), "messages intact")
	assert.ElementsMatch(t, append(membersBefore, "user:"+userID+":owner"), agentOwnedRoomMembers(t, pool, slug),
		"agent memberships intact; the claiming human joins as owner")

	status, out = doJSON(t, "PATCH", ts.URL+"/v1/rooms/"+slug, ownerKey, `{"display_name":"Still Agent Managed"}`)
	assert.Equal(t, http.StatusOK, status, "the owner agent keeps managing after the claim: %v", out)
	agentOwnedRoomPostMessage(t, ts.URL, slug, ownerTok, ownerID, "after claim")
}

// Room creation and the creator's owner membership commit together: when the owner
// membership cannot be written, no ownerless room is left behind.
func TestAgentOwnedRoom_CreationRollsBackWithoutOwnerMembership(t *testing.T) {
	_, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	roomPreCleanup(t, pool)
	ctx := context.Background()

	slug := "test-agentowned-rollback"
	_, err := db.NewRoomRepository(pool).Create(ctx, models.CreateRoomParams{
		Slug:           slug,
		DisplayName:    "Rollback Room",
		CreatorAgentID: "agent_roomtest_does_not_exist",
	})
	require.Error(t, err, "an owner membership for a missing agent must fail")

	var n int
	require.NoError(t, pool.QueryRow(ctx, `SELECT COUNT(*) FROM rooms WHERE slug = $1`, slug).Scan(&n))
	assert.Zero(t, n, "the room insert is rolled back with the failed owner membership")
}
