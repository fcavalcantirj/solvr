package api

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// Room expiry is enforced by the server itself (task: "Keep room and content permissions
// authoritative on the server", step 2). A room whose expires_at has passed is gone on
// every surface at once — not only after the presence reaper deletes it up to a minute
// later — and streams that are already open end.

func expireRoom(t *testing.T, room *accessRoom) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	tag, err := room.pool.Exec(ctx, `UPDATE rooms SET expires_at = NOW() - INTERVAL '1 second' WHERE slug = $1`, room.slug)
	require.NoError(t, err)
	require.Equal(t, int64(1), tag.RowsAffected(), "expire room %s", room.slug)
}

// sitemapRoomSlugs returns every room slug GET /v1/sitemap/urls advertises (flat and paged).
func sitemapRoomSlugs(t *testing.T, base string) []string {
	t.Helper()
	var slugs []string
	for _, path := range []string{"/v1/sitemap/urls", "/v1/sitemap/urls?type=rooms&page=1&per_page=2500"} {
		status, out := doJSON(t, "GET", base+path, "", "")
		require.Equal(t, http.StatusOK, status, "%s: %v", path, out)
		data, _ := out["data"].(map[string]any)
		rooms, _ := data["rooms"].([]any)
		for _, r := range rooms {
			if m, ok := r.(map[string]any); ok {
				slug, _ := m["slug"].(string)
				slugs = append(slugs, path+" "+slug)
			}
		}
	}
	return slugs
}

func containsSlug(entries []string, slug string) bool {
	for _, e := range entries {
		if strings.HasSuffix(e, " "+slug) {
			return true
		}
	}
	return false
}

func sitemapRoomCount(t *testing.T, base string) float64 {
	t.Helper()
	status, out := doJSON(t, "GET", base+"/v1/sitemap/counts", "", "")
	require.Equal(t, http.StatusOK, status, "counts: %v", out)
	data, _ := out["data"].(map[string]any)
	n, _ := data["rooms"].(float64)
	return n
}

func TestRoomExpiry_ExpiredRoomIsGoneOnEverySurfaceBeforeTheReaper(t *testing.T) {
	opts := RoomRelayOptions{SweepInterval: -1}
	a := startRoomInstance(t, opts)
	b := startRoomInstance(t, opts)
	room := newAccessRoom(t, a, false)
	// The sitemap lists a public room once it carries a two-way exchange (task idx 80).
	for _, tok := range []string{room.plannerTok, room.executorTok} {
		status, _, err := postEntryRaw(a.ts.URL, room.slug, tok, map[string]any{"body": "before expiry"})
		require.NoError(t, err)
		require.Less(t, status, 300, "a member posts before the room expires")
	}

	require.True(t, containsSlug(sitemapRoomSlugs(t, a.ts.URL), room.slug), "precondition: the live public room is in the sitemap")
	countBefore := sitemapRoomCount(t, a.ts.URL)

	anonymous := openAccessStream(t, room.streamURL(b), "")
	member := openAccessStream(t, room.streamURL(b), room.plannerTok)
	adapter := openAccessStream(t, b.ts.URL+"/r/"+room.slug+"/stream", room.executorTok)

	expireRoom(t, room)

	for name, s := range map[string]*accessStream{"anonymous": anonymous, "member token": member, "/r adapter": adapter} {
		require.True(t, s.endedWithin(3*time.Second), "the %s stream on B ends when the room expires", name)
		require.Contains(t, s.events(), "access_revoked", "the %s stream says why it ended", name)
	}

	base := "/v1/rooms/" + room.slug
	for _, inst := range []*roomInstance{a, b} {
		for _, c := range []struct{ name, method, path, bearer, body string }{
			{"anonymous room view", "GET", base, "", ""},
			{"owner room view", "GET", base, room.ownerJWT, ""},
			{"member entries", "GET", base + "/entries", room.plannerTok, ""},
			{"account-key entries", "GET", base + "/entries", room.plannerKey, ""},
			{"anonymous entries", "GET", base + "/entries", "", ""},
			{"handshake", "POST", base + "/handshake", room.executorKey, `{}`},
			{"owner update", "PATCH", base, room.ownerJWT, `{"description":"still here?"}`},
		} {
			status, out := doJSON(t, c.method, inst.ts.URL+c.path, c.bearer, c.body)
			require.Equal(t, http.StatusNotFound, status, "%s on an expired room: %v", c.name, out)
		}
		// The token-only /r adapter answers a room that is gone exactly as for a deleted
		// room: the token no longer authorizes anything.
		status, out := doJSON(t, "GET", inst.ts.URL+"/r/"+room.slug+"/messages", room.executorTok, "")
		require.Equal(t, http.StatusUnauthorized, status, "/r messages on an expired room: %v", out)
		status, _, err := postEntryRaw(inst.ts.URL, room.slug, room.plannerTok, map[string]any{"body": "after expiry"})
		require.NoError(t, err)
		require.Equal(t, http.StatusNotFound, status, "a member cannot write to an expired room")
	}

	require.False(t, containsSlug(sitemapRoomSlugs(t, a.ts.URL), room.slug), "the expired room leaves the sitemap")
	require.Equal(t, countBefore-1, sitemapRoomCount(t, a.ts.URL), "the sitemap count drops with it")
}
