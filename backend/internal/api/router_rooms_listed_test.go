package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fcavalcantirj/solvr/internal/db"
)

// listTestRoom lists a public fixture room, as its owner's PATCH {"listed": true} does
// (SPEC.md 27.1). The fixtures that read rooms on public surfaces were written when every
// public room was listed; since owner decision F2 (2026-10-17) a new room starts unlisted.
func listTestRoom(t *testing.T, pool *db.Pool, slug string) {
	t.Helper()
	_, err := pool.Exec(context.Background(), `UPDATE rooms SET listed = true WHERE slug = $1`, slug)
	require.NoError(t, err)
}

// getJSONData reads url without credentials and answers its "data" member.
func getJSONData(t *testing.T, url string) any {
	t.Helper()
	resp, err := http.Get(url) //nolint:noctx // test client
	require.NoError(t, err)
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	require.Equal(t, http.StatusOK, resp.StatusCode, "%s: %s", url, raw)
	var out map[string]any
	require.NoError(t, json.Unmarshal(raw, &out))
	return out["data"]
}

// hasSlug reports whether a JSON array of rooms holds one with slug.
func hasSlug(t *testing.T, rooms any, slug string) bool {
	t.Helper()
	list, ok := rooms.([]any)
	require.True(t, ok, "a list of rooms")
	for _, r := range list {
		m, ok := r.(map[string]any)
		require.True(t, ok, "a room object")
		if m["slug"] == slug {
			return true
		}
	}
	return false
}

// listedProbe reads, without credentials, the three public surfaces that list a room.
func listedProbe(t *testing.T, base, slug string) (indexable, inSitemap, inList bool) {
	t.Helper()
	seo, ok := getJSONData(t, base+"/v1/rooms/"+slug+"/seo").(map[string]any)
	require.True(t, ok, "the seo verdict is an object")
	indexable, ok = seo["indexable"].(bool)
	require.True(t, ok, "indexable is a boolean")
	sitemap, ok := getJSONData(t, base+"/v1/sitemap/urls?type=rooms&per_page=5000").(map[string]any)
	require.True(t, ok, "the sitemap answer is an object")
	return indexable, hasSlug(t, sitemap["rooms"], slug), hasSlug(t, getJSONData(t, base+"/v1/rooms?q="+slug), slug)
}

// SPEC.md 27.1 (owner decision F2, 2026-10-17: unlisted by default). A new public room is read
// and joined by its link as before, but it is not indexable, not in the rooms sitemap and not
// in the public room list until its owner lists it with PATCH {"listed": true}. Nobody else
// may list it.
func TestRooms_ANewPublicRoomIsUnlistedUntilItsOwnerListsIt(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	_, ownerKey := registerRoomTestAgent(t, ts)
	_, otherKey := registerRoomTestAgent(t, ts)
	slug, _ := createTestRoomWithAgentKey(t, ts, ownerKey)
	for _, who := range []string{"planner-listed", "executor-listed"} {
		_, err := pool.Exec(context.Background(), `INSERT INTO messages (room_id, agent_name, author_type, author_id, content)
			SELECT id, $2, 'agent', $2, 'hello' FROM rooms WHERE slug = $1`, slug, who)
		require.NoError(t, err)
	}

	status, read := seoGet(t, ts.URL+"/v1/rooms/"+slug) // seoGet answers the "data" object
	require.Equal(t, http.StatusOK, status, "an unlisted public room is still read by its link")
	readRoom, ok := read["room"].(map[string]any)
	require.True(t, ok, "the read answers the room")
	assert.Equal(t, false, readRoom["listed"], "a new room starts unlisted")
	indexable, inSitemap, inList := listedProbe(t, ts.URL, slug)
	assert.False(t, indexable, "unlisted: not indexable, although it carries a two-way exchange")
	assert.False(t, inSitemap, "unlisted: not in the rooms sitemap")
	assert.False(t, inList, "unlisted: not in the public room list")

	resp := doRoomRequestAtCurrentVersion(t, http.MethodPatch, ts.URL+"/v1/rooms/"+slug, `{"listed":true}`, otherKey)
	require.NoError(t, resp.Body.Close())
	assert.Equal(t, http.StatusForbidden, resp.StatusCode, "only the owner lists a room")

	resp = doRoomRequestAtCurrentVersion(t, http.MethodPatch, ts.URL+"/v1/rooms/"+slug, `{"listed":true}`, ownerKey)
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	require.Equal(t, http.StatusOK, resp.StatusCode, string(raw))
	var patched struct {
		Data struct {
			Listed bool `json:"listed"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(raw, &patched))
	assert.True(t, patched.Data.Listed)
	indexable, inSitemap, inList = listedProbe(t, ts.URL, slug)
	assert.True(t, indexable, "listed and two-way: indexable")
	assert.True(t, inSitemap, "listed: in the rooms sitemap")
	assert.True(t, inList, "listed: in the public room list")
}
