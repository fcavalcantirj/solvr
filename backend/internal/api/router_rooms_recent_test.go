package api

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

// GET /v1/me/rooms is the way back to recent work (idx 92 step 1): the rooms a person owns
// or joined, most recently active first.
func TestMyRooms_ListsOwnedAndJoinedRoomsMostRecentlyActiveFirst(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	ctx := context.Background()
	me, myJWT := createRoomTestUser(t, pool)
	_, otherJWT := createRoomTestUser(t, pool)

	owned := sourceTestSlug("mine")
	code, _ := createSourceTestRoom(t, ts, myJWT, fmt.Sprintf(`{"display_name":"Mine","slug":%q}`, owned))
	require.Equal(t, http.StatusCreated, code)
	joined := sourceTestSlug("joined")
	code, _ = createSourceTestRoom(t, ts, otherJWT, fmt.Sprintf(`{"display_name":"Theirs","slug":%q,"is_private":true}`, joined))
	require.Equal(t, http.StatusCreated, code)
	_, err := pool.Exec(ctx, `INSERT INTO room_members (room_id, user_id, role, added_by)
		SELECT id, $2::uuid, 'member', 'test' FROM rooms WHERE slug = $1`, joined, me)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE rooms SET last_active_at = NOW() - INTERVAL '2 days' WHERE slug = $1`, owned)
	require.NoError(t, err)

	slugs := meRoomSlugs(t, ts, myJWT)
	require.GreaterOrEqual(t, len(slugs), 2)
	require.Equal(t, []string{joined, owned}, slugs[:2], "joined room (active now) before the owned room (2 days ago)")
}
