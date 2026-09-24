package api

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

// Family access is derived from a live human's ACTIVE owner membership and the agent's
// CURRENT link (migration 000096); a historical link never keeps a closed room open.
func TestRoomFamily_DeletedHumansSiblingLosesClosedRoomAndToken(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	roomPreCleanup(t, pool)

	userA, _ := createRoomTestUser(t, pool)
	agentAID, agentAKey := registerRoomTestAgent(t, ts)
	claimAgentToUser(t, pool, agentAID, userA)
	siblingID, siblingKey := registerRoomTestAgent(t, ts)
	claimAgentToUser(t, pool, siblingID, userA)
	slug, _ := createPrivateRoomWithAgentKey(t, ts, agentAKey)

	hsStatus, siblingTok := handshake(t, ts.URL, slug, siblingKey, "")
	require.Equal(t, http.StatusCreated, hsStatus, "sibling family handshake")

	// The human deletes the account (DELETE /v1/me soft-deletes users.deleted_at).
	_, err := pool.Exec(context.Background(), `UPDATE users SET deleted_at = NOW() WHERE id = $1::uuid`, userA)
	require.NoError(t, err)

	require.Equal(t, http.StatusForbidden, getStatus(t, ts.URL+"/v1/rooms/"+slug, siblingKey),
		"a sibling of a deleted human must not keep reading the closed room")
	st, _ := doJSON(t, "POST", ts.URL+"/r/"+slug+"/message", siblingTok,
		`{"agent_name":"sibling","content":"still here?"}`)
	require.Contains(t, []int{http.StatusUnauthorized, http.StatusForbidden}, st,
		"the family-derived room token must stop working with the grant")
	hs, _ := handshake(t, ts.URL, slug, siblingKey, "")
	require.Equal(t, http.StatusForbidden, hs, "a sibling of a deleted human cannot handshake back in")
}

// Human ownership is read from room_members, not rooms.owner_id: once the human's owner
// membership is handed over, owner_id alone grants neither management nor closed reads.
func TestRoomOwnership_HumanOwnerMembershipIsAuthoritative(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	roomPreCleanup(t, pool)

	userID, ownerJWT := createRoomTestUser(t, pool)
	slug, _ := createClosedRoom(t, ts, ownerJWT)
	require.Equal(t, http.StatusOK, getStatus(t, ts.URL+"/v1/rooms/"+slug, ownerJWT), "owner reads its closed room")

	heirID, heirKey := registerRoomTestAgent(t, ts)
	status, _ := doJSON(t, "POST", ts.URL+"/v1/rooms/"+slug+"/members", ownerJWT, `{"agent_id":"`+heirID+`","role":"owner"}`)
	require.Equal(t, http.StatusCreated, status)

	// Hand over: the human's owner membership is revoked; rooms.owner_id still names it.
	_, err := pool.Exec(context.Background(),
		`UPDATE room_members SET revoked_at = NOW()
		 WHERE user_id = $1::uuid AND room_id = (SELECT id FROM rooms WHERE slug = $2)`, userID, slug)
	require.NoError(t, err)

	patch := doRoomRequest(t, "PATCH", ts.URL+"/v1/rooms/"+slug, `{"display_name":"Former Human Owner"}`, ownerJWT)
	patch.Body.Close()
	require.Equal(t, http.StatusForbidden, patch.StatusCode, "former human owner can no longer manage")
	require.Equal(t, http.StatusForbidden, getStatus(t, ts.URL+"/v1/rooms/"+slug, ownerJWT),
		"former human owner can no longer read the closed room")

	patch = doRoomRequest(t, "PATCH", ts.URL+"/v1/rooms/"+slug, `{"display_name":"Heir Renamed"}`, heirKey)
	patch.Body.Close()
	require.Equal(t, http.StatusOK, patch.StatusCode, "heir owner manages the room")
}
