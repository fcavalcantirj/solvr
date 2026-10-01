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
	slug := createPrivateRoomWithAgentKey(t, ts, agentAKey)

	hsStatus, siblingTok := handshake(t, ts.URL, slug, siblingKey, "")
	require.Equal(t, http.StatusCreated, hsStatus, "sibling family handshake")

	// The human deletes the account (DELETE /v1/me soft-deletes users.deleted_at).
	_, err := pool.Exec(context.Background(), `UPDATE users SET deleted_at = NOW() WHERE id = $1::uuid`, userA)
	require.NoError(t, err)

	// The owner is gone, so its agents cannot authenticate anywhere (anti-abuse W0): 401, not 403.
	require.Equal(t, http.StatusUnauthorized, getStatus(t, ts.URL+"/v1/rooms/"+slug, siblingKey),
		"a sibling of a deleted human must not keep reading the closed room")
	st, _ := doJSON(t, "POST", ts.URL+"/r/"+slug+"/message", siblingTok,
		`{"agent_name":"sibling","content":"still here?"}`)
	require.Contains(t, []int{http.StatusUnauthorized, http.StatusForbidden}, st,
		"the family-derived room token must stop working with the grant")
	hs, _ := handshake(t, ts.URL, slug, siblingKey, "")
	require.Equal(t, http.StatusUnauthorized, hs, "a sibling of a deleted human cannot handshake back in")
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

	patch = doRoomRequestAtCurrentVersion(t, "PATCH", ts.URL+"/v1/rooms/"+slug, `{"display_name":"Heir Renamed"}`, heirKey)
	patch.Body.Close()
	require.Equal(t, http.StatusOK, patch.StatusCode, "heir owner manages the room")
}

// DELETE /v1/me with a claimed agent (SPEC Part 20.2): the agent is unclaimed, and the
// human's sole-owner closed room is archived rather than left unmanageable.
func TestDeleteMe_UnclaimsAgentsAndArchivesSoleOwnerRoom(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	roomPreCleanup(t, pool)

	userID, ownerJWT := createRoomTestUser(t, pool)
	agentID, _ := registerRoomTestAgent(t, ts)
	claimAgentToUser(t, pool, agentID, userID)
	slug, _ := createClosedRoom(t, ts, ownerJWT)

	resp := doRoomRequest(t, "DELETE", ts.URL+"/v1/me", "", ownerJWT)
	resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode, "account deletion with a claimed agent")

	var humanID *string
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT human_id::text FROM agents WHERE id = $1`, agentID).Scan(&humanID))
	require.Nil(t, humanID, "the deleted human's agent is unclaimed")

	var archived bool
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT archived_at IS NOT NULL FROM rooms WHERE slug = $1`, slug).Scan(&archived))
	require.True(t, archived, "the deleted human's sole-owner room is archived")
}
