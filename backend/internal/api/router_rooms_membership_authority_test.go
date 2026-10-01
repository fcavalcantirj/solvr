package api

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

// The final owner of a live room cannot be revoked or demoted through the members API;
// ownership must be handed to another member first (migration 000095 guard).
func TestRoomMembers_FinalOwnerCannotBeRemovedUntilTransfer(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	roomPreCleanup(t, pool)

	ownerID, ownerKey := registerRoomTestAgent(t, ts) // self-registered, never claimed
	slug, _ := createTestRoomWithAgentKey(t, ts, ownerKey)
	membersURL := ts.URL + "/v1/rooms/" + slug + "/members"

	status, out := doJSON(t, "DELETE", membersURL+"/"+ownerID, ownerKey, "")
	require.Equal(t, http.StatusConflict, status, "removing the final owner must be refused: %v", out)
	errObj, _ := out["error"].(map[string]any)
	require.Equal(t, "LAST_OWNER", errObj["code"])

	status, out = doJSON(t, "POST", membersURL, ownerKey, `{"agent_id":"`+ownerID+`","role":"member"}`)
	require.Equal(t, http.StatusConflict, status, "demoting the final owner must be refused: %v", out)

	// Transfer: promote an heir, then the original owner may leave.
	heirID, heirKey := registerRoomTestAgent(t, ts)
	status, _ = doJSON(t, "POST", membersURL, ownerKey, `{"agent_id":"`+heirID+`","role":"owner"}`)
	require.Equal(t, http.StatusCreated, status)
	status, _ = doJSON(t, "DELETE", membersURL+"/"+ownerID, heirKey, "")
	require.Equal(t, http.StatusNoContent, status)

	patch := doRoomRequestAtCurrentVersion(t, "PATCH", ts.URL+"/v1/rooms/"+slug, `{"display_name":"Heir Renamed"}`, heirKey)
	patch.Body.Close()
	require.Equal(t, http.StatusOK, patch.StatusCode, "heir owner manages the room")
	patch = doRoomRequest(t, "PATCH", ts.URL+"/v1/rooms/"+slug, `{"display_name":"Former Owner"}`, ownerKey)
	patch.Body.Close()
	require.Equal(t, http.StatusForbidden, patch.StatusCode, "revoked former owner can no longer manage")
}

// A revoked agent stays out until an owner explicitly readmits it; readmission restores
// handshake and closed-room reads with a fresh individual credential.
func TestRoomMembers_RevokedAgentNeedsExplicitReadmission(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	roomPreCleanup(t, pool)
	t.Cleanup(func() { pool.Exec(context.Background(), "DELETE FROM agents WHERE id LIKE 'agent_%hs%'") }) //nolint:errcheck

	_, ownerJWT := createRoomTestUser(t, pool)
	slug, _ := createClosedRoom(t, ts, ownerJWT)
	agentID, agentKey := registerTestAgent(t, ts, uniqName("agent_hs_readmit"))
	membersURL := ts.URL + "/v1/rooms/" + slug + "/members"

	status, _ := doJSON(t, "POST", membersURL, ownerJWT, `{"agent_id":"`+agentID+`"}`)
	require.Equal(t, http.StatusCreated, status)
	status, _ = doJSON(t, "DELETE", membersURL+"/"+agentID, ownerJWT, "")
	require.Equal(t, http.StatusNoContent, status)

	hs, _ := handshake(t, ts.URL, slug, agentKey, "")
	require.Equal(t, http.StatusForbidden, hs, "revoked agent cannot handshake back in by itself")
	status, _ = doJSON(t, "DELETE", membersURL+"/"+agentID, ownerJWT, "")
	require.Equal(t, http.StatusNotFound, status, "a revoked membership is not active")

	status, _ = doJSON(t, "POST", membersURL, ownerJWT, `{"agent_id":"`+agentID+`"}`)
	require.Equal(t, http.StatusCreated, status)
	hs, tok := handshake(t, ts.URL, slug, agentKey, "")
	require.Equal(t, http.StatusCreated, hs)
	require.Equal(t, http.StatusOK, getStatus(t, ts.URL+"/v1/rooms/"+slug, tok))
}
