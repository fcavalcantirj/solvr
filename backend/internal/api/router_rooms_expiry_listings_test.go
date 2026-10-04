package api

import (
	"context"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// A room past expires_at answers 404 on its own routes (router_rooms_expiry_test.go), so no
// listing may keep pointing at it until the reaper deletes it: not the homepage feeds, not a
// post's related rooms, not the owner's own room list (task: "Keep room and content
// permissions authoritative on the server", step 2 — expiry enforced on every surface).

func getBody(t *testing.T, url, bearer string) string {
	t.Helper()
	req, err := http.NewRequest("GET", url, nil) //nolint:noctx
	require.NoError(t, err)
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode, "%s: %s", url, string(body))
	return string(body)
}

func TestRoomExpiry_ExpiredRoomLeavesEveryListingBeforeTheReaper(t *testing.T) {
	slug := hpoSlug("exp")
	a := startRoomInstance(t, RoomRelayOptions{SweepInterval: -1})
	t.Cleanup(func() { hpoCleanup(t, a.pool) })
	ownerJWT := overviewRoom(t, a, slug)
	featureOnHomepage(t, a.pool, slug)

	// A real public post: a post's related-rooms list answers 404 for a post that does not
	// exist, exactly as GET /v1/posts/{id} does.
	// Its author is an existing agent (000117), removed after the post.
	author := "agent_expiry_fixture_" + uuid.NewString()[:8]
	_, err := a.pool.Exec(context.Background(), `INSERT INTO agents (id, display_name, status) VALUES ($1, $1, 'active')`, author)
	require.NoError(t, err)
	t.Cleanup(func() { a.pool.Exec(context.Background(), `DELETE FROM agents WHERE id = $1`, author) }) //nolint:errcheck
	sourcePost := childContractPost(t, a.pool, author, models.VisibilityPublic, "", false)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	// Rooms expire by the clock (expires_at is set at creation): nothing announces the moment.
	var expiresAt time.Time
	require.NoError(t, a.pool.QueryRow(ctx,
		`UPDATE rooms SET source_post_id = $1::uuid, expires_at = NOW() + INTERVAL '3 seconds'
		 WHERE slug = $2 RETURNING expires_at`, sourcePost, slug).Scan(&expiresAt))

	listings := []struct{ name, path, bearer string }{
		{"homepage overview", "/v1/homepage/overview", ""},
		{"overview", "/v1/overview", ""},
		{"homepage activity", "/v1/homepage/activity", ""},
		{"overview activity", "/v1/overview/activity", ""},
		{"post related rooms", "/v1/posts/" + sourcePost + "/rooms", ""},
		{"owner's rooms", "/v1/me/rooms", ownerJWT},
	}
	for _, l := range listings {
		require.Contains(t, getBody(t, a.ts.URL+l.path, l.bearer), slug, "precondition: %s lists the live room", l.name)
	}

	require.True(t, time.Now().Before(expiresAt), "precondition reads ran before the room expired")
	time.Sleep(time.Until(expiresAt) + 200*time.Millisecond)

	for _, l := range listings {
		require.NotContains(t, getBody(t, a.ts.URL+l.path, l.bearer), slug, "%s drops the room the moment it expires, with no reaper and no notice", l.name)
	}
	require.NotContains(t, getBody(t, a.ts.URL+"/v1/homepage/rooms?window=7d", ""), slug, "homepage rooms never lists the expired room")
}
