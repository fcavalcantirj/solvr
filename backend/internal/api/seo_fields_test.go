package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// seoGet decodes GET url's data object (anonymous caller) and returns the status.
func seoGet(t *testing.T, url string) (int, map[string]any) {
	t.Helper()
	resp, err := http.Get(url)
	require.NoError(t, err)
	defer resp.Body.Close()
	var body struct {
		Data map[string]any `json:"data"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&body)
	return resp.StatusCode, body.Data
}

// Task idx 80: the API, not the page, decides whether a post page may be indexed and
// what its search description says, at GET /v1/posts/{id}/seo. The post read itself is
// a contract operation and stays unchanged. A rejected post still answers 200 to a
// direct link (owner decision pending, idx 59) but tells the page not to index it.
func TestPostSEO_IndexableAndDescriptionComeFromTheAPI(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	ctx := context.Background()
	marker := fmt.Sprintf("seopost%d", time.Now().UnixNano()%1000000000)
	agentID, _ := registerRoomTestAgent(t, ts)

	insert := func(status, moderation string) string {
		t.Helper()
		var id string
		require.NoError(t, pool.QueryRow(ctx,
			`INSERT INTO posts (type,title,description,posted_by_type,posted_by_id,status,visibility,
			   publication_state,moderation_state)
			 VALUES ('post',$1,$2,'agent',$3,$4,'public','published',$5) RETURNING id::text`,
			"SEO "+status+" "+marker, "## Steps\n\nConnect a **planner** and an executor "+marker, agentID,
			status, moderation).Scan(&id))
		return id
	}
	approved := insert("open", "approved")
	rejected := insert("rejected", "approved")
	t.Cleanup(func() { pool.Exec(context.Background(), "DELETE FROM posts WHERE title LIKE '%"+marker+"%'") }) //nolint:errcheck

	status, seo := seoGet(t, ts.URL+"/v1/posts/"+approved+"/seo")
	require.Equal(t, http.StatusOK, status)
	assert.Equal(t, map[string]any{"indexable": true, "title": "SEO open " + marker,
		"description": "Steps Connect a planner and an executor " + marker}, seo)

	status, seo = seoGet(t, ts.URL+"/v1/posts/"+rejected+"/seo")
	require.Equal(t, http.StatusOK, status)
	assert.Equal(t, false, seo["indexable"], "a rejected post must not be indexed")

	status, _ = seoGet(t, ts.URL+"/v1/posts/"+uuid.NewString()+"/seo")
	assert.Equal(t, http.StatusNotFound, status, "404 exactly when the post read is")

	status, post := seoGet(t, ts.URL+"/v1/posts/"+approved)
	require.Equal(t, http.StatusOK, status)
	assert.NotContains(t, post, "seo", "the post read (a contract operation) is unchanged")
}

// Task idx 80: a public room is indexable once it carries a two-way exchange; its
// title and purpose-based description come from GET /v1/rooms/{slug}/seo, behind the
// room read's policy.
func TestRoomSEO_IndexableTitleAndDescriptionComeFromTheAPI(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	ctx := context.Background()
	slug := fmt.Sprintf("test-seo-room-%d", time.Now().UnixNano()%1000000000)
	var roomID string
	require.NoError(t, pool.QueryRow(ctx, `INSERT INTO rooms (slug, display_name, description, is_private)
		VALUES ($1, 'Kestrel firmware build', 'Plan and ship the **kestrel** firmware', false) RETURNING id::text`,
		slug).Scan(&roomID))
	t.Cleanup(func() { pool.Exec(context.Background(), `DELETE FROM rooms WHERE id = $1`, roomID) }) //nolint:errcheck

	status, seo := seoGet(t, ts.URL+"/v1/rooms/"+slug+"/seo")
	require.Equal(t, http.StatusOK, status)
	assert.Equal(t, map[string]any{"indexable": false, "title": "Kestrel firmware build",
		"description": "Plan and ship the kestrel firmware"}, seo, "an empty room is thin")

	for _, who := range []string{"planner-seo", "executor-seo"} {
		_, err := pool.Exec(ctx, `INSERT INTO messages (room_id, agent_name, author_type, author_id, content)
			VALUES ($1, $2, 'agent', $2, 'hello')`, roomID, who)
		require.NoError(t, err)
	}
	status, seo = seoGet(t, ts.URL+"/v1/rooms/"+slug+"/seo")
	require.Equal(t, http.StatusOK, status)
	assert.Equal(t, true, seo["indexable"], "two authors exchanged messages")

	status, room := seoGet(t, ts.URL+"/v1/rooms/"+slug)
	require.Equal(t, http.StatusOK, status)
	assert.NotContains(t, room, "seo", "the room read is unchanged")

	status, _ = seoGet(t, ts.URL+"/v1/rooms/test-seo-no-such-room/seo")
	assert.Equal(t, http.StatusNotFound, status)

	private := slug + "-closed"
	_, err := pool.Exec(ctx, `INSERT INTO rooms (slug, display_name, is_private) VALUES ($1, 'closed', true)`, private)
	require.NoError(t, err)
	readStatus, _ := seoGet(t, ts.URL+"/v1/rooms/"+private)
	seoStatus, _ := seoGet(t, ts.URL+"/v1/rooms/"+private+"/seo")
	assert.Equal(t, http.StatusForbidden, readStatus)
	assert.Equal(t, readStatus, seoStatus, "a private room's seo follows the room read's policy")
}
