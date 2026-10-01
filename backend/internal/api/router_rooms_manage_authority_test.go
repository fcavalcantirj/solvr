package api

import (
	"context"
	"net/http"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Room management (PATCH/DELETE/archive/members) is decided by RoomHandler.canManage over
// room_members, the membership authority. These tests pin, end to end, the cases the
// retired DB-free rooms.owner_id helpers (canManageRoom, canRotateRoomToken,
// agentOwnsRoom, isRoomOwner(OrAdmin), models.SameHumanAsOwner) used to guard.

// A claimed agent that did NOT create the room manages it when its linked human is
// the room's active owner (family scope), and loses that the moment it is unlinked:
// a historical link grants nothing.
func TestRoomManageAuthority_FamilyAgentManagesUntilUnlinked(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	roomPreCleanup(t, pool)

	userID, _ := createRoomTestUser(t, pool)
	creatorID, creatorKey := registerRoomTestAgent(t, ts)
	claimAgentToUser(t, pool, creatorID, userID)
	slug, _ := createTestRoomWithAgentKey(t, ts, creatorKey)

	siblingID, siblingKey := registerRoomTestAgent(t, ts)
	claimAgentToUser(t, pool, siblingID, userID)

	resp := doRoomRequestAtCurrentVersion(t, "PATCH", ts.URL+"/v1/rooms/"+slug, `{"display_name":"Renamed by Sibling"}`, siblingKey)
	resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode, "family agent must manage the room")

	// The unlink fixture mirrors db's forceUnlink: trigger_prevent_agent_reclaim
	// forbids clearing human_id, so it is suspended inside one transaction.
	ctx := context.Background()
	require.NoError(t, pool.WithTx(ctx, func(tx db.Tx) error {
		if _, err := tx.Exec(ctx, `ALTER TABLE agents DISABLE TRIGGER trigger_prevent_agent_reclaim`); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE agents SET human_id = NULL WHERE id = $1`, siblingID); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `ALTER TABLE agents ENABLE TRIGGER trigger_prevent_agent_reclaim`)
		return err
	}))

	resp = doRoomRequest(t, "PATCH", ts.URL+"/v1/rooms/"+slug, `{"display_name":"After Unlink"}`, siblingKey)
	resp.Body.Close()
	assert.Equal(t, http.StatusForbidden, resp.StatusCode, "an unlinked agent keeps no management right")
}

// An unclaimed agent that is not a member never manages someone else's room.
func TestRoomManageAuthority_UnclaimedForeignAgentForbidden(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	roomPreCleanup(t, pool)

	_, jwt := createRoomTestUser(t, pool)
	slug, _ := createTestRoomWithToken(t, ts, jwt)
	_, strangerKey := registerRoomTestAgent(t, ts)

	resp := doRoomRequest(t, "PATCH", ts.URL+"/v1/rooms/"+slug, `{"display_name":"Hijacked"}`, strangerKey)
	resp.Body.Close()
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

// A signed-in human who holds no owner membership cannot manage the room.
func TestRoomManageAuthority_NonOwnerHumanForbidden(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	roomPreCleanup(t, pool)

	_, ownerJWT := createRoomTestUser(t, pool)
	slug, _ := createTestRoomWithToken(t, ts, ownerJWT)
	_, otherJWT := createRoomTestUser(t, pool)

	resp := doRoomRequest(t, "PATCH", ts.URL+"/v1/rooms/"+slug, `{"display_name":"Hijacked"}`, otherJWT)
	resp.Body.Close()
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

// The human owner manages through its owner MEMBERSHIP: once that membership is
// demoted (after ownership passes to another owner), management goes with it.
func TestRoomManageAuthority_HumanOwnerFollowsMembership(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	roomPreCleanup(t, pool)

	ownerID, ownerJWT := createRoomTestUser(t, pool)
	slug, _ := createTestRoomWithToken(t, ts, ownerJWT)
	newOwnerID, newOwnerJWT := createRoomTestUser(t, pool)

	// One statement = one transaction, so the deferred final-owner guard sees the
	// promotion and the demotion together (an ownership transfer).
	_, err := pool.Exec(context.Background(), `
		WITH room AS (SELECT id FROM rooms WHERE slug = $1),
		promoted AS (
			INSERT INTO room_members (room_id, user_id, role, added_by)
			SELECT id, $3::uuid, 'owner', 'system' FROM room
		)
		UPDATE room_members SET role = 'member'
		WHERE user_id = $2::uuid AND room_id = (SELECT id FROM room)`, slug, ownerID, newOwnerID)
	require.NoError(t, err)

	resp := doRoomRequest(t, "PATCH", ts.URL+"/v1/rooms/"+slug, `{"display_name":"Old Owner"}`, ownerJWT)
	resp.Body.Close()
	assert.Equal(t, http.StatusForbidden, resp.StatusCode, "a demoted former owner must not manage")

	resp = doRoomRequestAtCurrentVersion(t, "PATCH", ts.URL+"/v1/rooms/"+slug, `{"display_name":"New Owner"}`, newOwnerJWT)
	resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode, "the new owner manages")
}
