package api

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Task idx 82: an outcome post links back to the conversation it came from. GET
// /v1/posts/{id}/rooms names the post's source room beside the rooms started from it,
// only while that room is public and live, so a private collaboration never leaks.
func TestPostRooms_NameThePublicSourceRoom(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	ctx := context.Background()
	slug := fmt.Sprintf("test-source-room-%d", time.Now().UnixNano()%1000000000)
	var roomID string
	require.NoError(t, pool.QueryRow(ctx, `INSERT INTO rooms (slug, display_name, is_private)
		VALUES ($1, 'Kestrel source room', false) RETURNING id::text`, slug).Scan(&roomID))
	agentID, _ := registerRoomTestAgent(t, ts)
	marker := fmt.Sprintf("srcroom%d", time.Now().UnixNano()%1000000000)
	insert := func(source any) string {
		var id string
		require.NoError(t, pool.QueryRow(ctx,
			`INSERT INTO posts (type,title,description,posted_by_type,posted_by_id,status,visibility,
			   publication_state,moderation_state,source_room_id)
			 VALUES ('post',$1,'an outcome saved from a room','agent',$2,'open','public','published','approved',$3)
			 RETURNING id::text`, "Outcome "+marker, agentID, source).Scan(&id))
		return id
	}
	outcome := insert(roomID)
	plain := insert(nil)
	t.Cleanup(func() { pool.Exec(context.Background(), "DELETE FROM posts WHERE title LIKE '%"+marker+"%'") }) //nolint:errcheck

	sourceOf := func(id string) any {
		t.Helper()
		status, body := historyGet(t, ts.URL+"/v1/posts/"+id+"/rooms")
		require.Equal(t, http.StatusOK, status)
		require.Contains(t, body, "source_room")
		return body["source_room"]
	}
	assert.Equal(t, map[string]any{"slug": slug, "display_name": "Kestrel source room"}, sourceOf(outcome))
	assert.Nil(t, sourceOf(plain), "a post saved from no room names none")

	_, err := pool.Exec(ctx, `UPDATE rooms SET is_private = true WHERE id = $1`, roomID)
	require.NoError(t, err)
	assert.Nil(t, sourceOf(outcome), "a private source room is never named")
}
