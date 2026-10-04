package api

import (
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The operator curates the homepage's featured rooms over the admin routes, and
// the homepage follows on the very next read (SPEC Part 26, "Featured rooms").

func featuredAdminRequest(t *testing.T, method, url, key string) (int, string) {
	t.Helper()
	req, err := http.NewRequest(method, url, nil)
	require.NoError(t, err)
	if key != "" {
		req.Header.Set("X-Admin-API-Key", key)
	}
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp.StatusCode, string(body)
}

func TestAdminFeaturedRooms_TheOperatorCuratesWhatTheHomepageShows(t *testing.T) {
	const key = "op-key-featured"
	t.Setenv("ADMIN_API_KEY", key)
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	defer hpoCleanup(t, pool)
	featureOnHomepage(t, pool) // start from an empty pool

	slug := hpoSlug("feat")
	hpoSeedRoom(t, pool, slug, "Featured Room", "a curated pick", false, []string{
		"PLAN: build the parser", "on it", "RESULT: parser shipped, tests green",
	})

	// The homepage is read (and cached) before the room is featured.
	before, _ := getHomepageOverview(t, ts.URL)
	assert.Empty(t, before.Previews.Rooms, "nothing is featured yet")

	status, body := featuredAdminRequest(t, http.MethodPut, ts.URL+"/admin/rooms/"+slug+"/featured", "")
	assert.Equal(t, http.StatusUnauthorized, status, "only the operator curates: %s", body)

	status, body = featuredAdminRequest(t, http.MethodPut, ts.URL+"/admin/rooms/"+slug+"/featured", key)
	require.Equal(t, http.StatusOK, status, body)

	after, _ := getHomepageOverview(t, ts.URL)
	require.Len(t, after.Previews.Rooms, 1, "the next read shows the featured room, not a stale snapshot")
	room := after.Previews.Rooms[0]
	assert.Equal(t, slug, room.Slug)
	require.NotNil(t, room.Ask)
	require.NotNil(t, room.Outcome)
	assert.Contains(t, room.Ask.Excerpt, "PLAN: build the parser")
	assert.Contains(t, room.Outcome.Excerpt, "RESULT: parser shipped")

	status, body = featuredAdminRequest(t, http.MethodGet, ts.URL+"/admin/rooms/featured", key)
	require.Equal(t, http.StatusOK, status, body)
	assert.Contains(t, body, `"slug":"`+slug+`"`)
	assert.Contains(t, body, `"shown_today":true`)

	status, body = featuredAdminRequest(t, http.MethodDelete, ts.URL+"/admin/rooms/"+slug+"/featured", key)
	require.Equal(t, http.StatusOK, status, body)
	assert.Contains(t, body, `"removed":true`)

	gone, raw := getHomepageOverview(t, ts.URL)
	assert.Empty(t, gone.Previews.Rooms, "unfeatured, the room leaves the homepage at once")
	assert.NotContains(t, raw, `"previews"`, "an empty pool publishes no section at all")
}
