package api

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// presentAgentIDs lists the agent_id -> agent_name of every live presence row in the room.
func presentAgentIDs(t *testing.T, baseURL, slug, bearer string) map[string]string {
	t.Helper()
	status, out := doJSON(t, "GET", baseURL+"/r/"+slug+"/agents", bearer, "")
	require.Equal(t, http.StatusOK, status, "list presence: %v", out)
	present := map[string]string{}
	rows, _ := out["data"].([]any)
	for _, row := range rows {
		rec, _ := row.(map[string]any)
		id, _ := rec["agent_id"].(string)
		name, _ := rec["agent_name"].(string)
		present[id] = name
	}
	return present
}

// Presence belongs to the authenticated member, never to whoever sends its agent_name.
func TestRoomPresence_BoundToAuthenticatedMember(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	roomPreCleanup(t, pool)

	_, jwt := createRoomTestUser(t, pool)
	slug, _ := createTestRoomWithToken(t, ts, jwt)
	aID, aKey := registerRoomTestAgent(t, ts)
	bID, bKey := registerRoomTestAgent(t, ts)
	aTok := handshakeRoomToken(t, ts, slug, aKey)
	bTok := handshakeRoomToken(t, ts, slug, bKey)

	status, out := doJSON(t, "POST", ts.URL+"/r/"+slug+"/join", aTok, `{"agent_name":"alpha"}`)
	require.Equal(t, http.StatusOK, status, "a join: %v", out)
	data, _ := out["data"].(map[string]any)
	assert.Equal(t, aID, data["agent_id"], "join records the authenticated agent")

	// b cannot take a's live name ...
	status, out = doJSON(t, "POST", ts.URL+"/r/"+slug+"/join", bTok, `{"agent_name":"alpha"}`)
	assert.Equal(t, http.StatusConflict, status, "b joining as a's live name: %v", out)
	// ... nor remove a by naming it.
	status, _ = doJSON(t, "POST", ts.URL+"/r/"+slug+"/leave", bTok, `{"agent_name":"alpha"}`)
	assert.Equal(t, http.StatusOK, status)
	present := presentAgentIDs(t, ts.URL, slug, aTok)
	assert.Equal(t, "alpha", present[aID], "a is still present after b's leave naming it")

	// A repeat join renames the member's single presence row.
	status, _ = doJSON(t, "POST", ts.URL+"/r/"+slug+"/join", aTok, `{"agent_name":"alpha-2"}`)
	require.Equal(t, http.StatusOK, status)
	status, _ = doJSON(t, "POST", ts.URL+"/r/"+slug+"/join", bTok, `{"agent_name":"beta"}`)
	require.Equal(t, http.StatusOK, status)
	present = presentAgentIDs(t, ts.URL, slug, aTok)
	assert.Equal(t, map[string]string{aID: "alpha-2", bID: "beta"}, present)

	// Revoking b's membership removes its presence (and its credential).
	status, out = doJSON(t, "DELETE", ts.URL+"/v1/rooms/"+slug+"/members/"+bID, jwt, "")
	require.Contains(t, []int{http.StatusOK, http.StatusNoContent}, status, "revoke b: %v", out)
	present = presentAgentIDs(t, ts.URL, slug, aTok)
	assert.NotContains(t, present, bID, "revoked member still present")
	status, _ = doJSON(t, "POST", ts.URL+"/r/"+slug+"/heartbeat", bTok, `{"agent_name":"beta"}`)
	assert.Equal(t, http.StatusUnauthorized, status, "revoked member's token still works")

	// a's own leave removes a.
	status, _ = doJSON(t, "POST", ts.URL+"/r/"+slug+"/leave", aTok, `{"agent_name":"whatever"}`)
	assert.Equal(t, http.StatusOK, status)
	_, stillA := presentAgentIDs(t, ts.URL, slug, aTok)[aID]
	assert.False(t, stillA, "a still present after its own leave")
}
