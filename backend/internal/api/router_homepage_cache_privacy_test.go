package api

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// The homepage endpoints that carry room names, slugs or messages are read straight by the
// browser. A copy a browser or proxy reuses without asking would go on showing a room after
// it turns private, so every reuse must be revalidated with the API (no-cache, no max-age);
// the server-side overview snapshot (dropped on every visibility change) is the only reuse
// that skips the database (task: "Keep room and content permissions authoritative on the
// server", step 6).

var homepageRoomPaths = []string{
	"/v1/homepage/overview",
	"/v1/overview",
	"/v1/homepage/activity",
	"/v1/overview/activity",
	"/v1/homepage/rooms?window=7d",
}

func getHomepage(t *testing.T, url string) (string, string) {
	t.Helper()
	resp, err := http.Get(url) //nolint:gosec,noctx
	require.NoError(t, err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode, "%s: %s", url, string(body))
	return resp.Header.Get("Cache-Control"), string(body)
}

func TestHomepageRoomSurfaces_AreRevalidatedAndDropARoomTheMomentItTurnsPrivate(t *testing.T) {
	slug := hpoSlug("cp")
	t.Setenv("HOMEPAGE_PREVIEW_ROOM_SLUGS", slug)
	a := startRoomInstance(t, RoomRelayOptions{SweepInterval: -1})
	t.Cleanup(func() { hpoCleanup(t, a.pool) })
	ownerJWT := overviewRoom(t, a, slug)

	for _, path := range homepageRoomPaths {
		cacheControl, _ := getHomepage(t, a.ts.URL+path)
		require.Contains(t, cacheControl, "no-cache", "%s: a browser or proxy must ask the API before reusing room content", path)
		require.NotContains(t, cacheControl, "max-age", "%s: no window in which a stored copy is served unasked", path)
	}
	for _, path := range []string{"/v1/homepage/overview", "/v1/overview", "/v1/homepage/activity", "/v1/overview/activity"} {
		_, body := getHomepage(t, a.ts.URL+path)
		require.True(t, strings.Contains(body, slug), "precondition: %s shows the public room", path)
	}

	status, out := doJSON(t, "PATCH", a.ts.URL+"/v1/rooms/"+slug, ownerJWT, `{"is_private":true}`)
	require.Equal(t, http.StatusOK, status, "make private: %v", out)

	for _, path := range homepageRoomPaths {
		_, body := getHomepage(t, a.ts.URL+path)
		require.False(t, strings.Contains(body, slug), "%s drops the private room on the very next request", path)
	}
}
