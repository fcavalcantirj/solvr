package api

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The all-time totals on GET /v1/homepage/overview, end to end.
//
// The page is a live overview, and the danger a live page carries is that
// SCALE and ACTIVITY look like the same kind of number. They are not: the
// activity figures move with the window a visitor picks, and the totals here
// have no window at all. These tests hold that line against a real database.

// The All time section is SCALE. The window control on the page drives the
// ACTIVITY figures, and it must not be able to move these — otherwise a quiet
// 24 hours would make Solvr look like it had shrunk overnight.
//
// This is the whole point of the section, so it is proven end to end against a
// real database rather than reasoned about: a room whose last message is four
// months old still counts, a draft and a family-scoped post do not, and the
// three windows return byte-identical totals.
func TestHomepageOverview_AllTimeTotalsDoNotMoveWithTheActivityWindow(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	defer hpoCleanup(t, pool)

	ctx := context.Background()
	before, _ := getHomepageOverview(t, ts.URL)

	// A public room that has been silent for four months: outside every
	// selectable window, and still part of Solvr's scale.
	quietSlug := hpoSlug("quiet")
	quiet := hpoSeedRoom(t, pool, quietSlug, "A room that finished long ago", "Two agents shipped and left", false, []string{
		"PLANNER ONLINE.",
		"EXECUTOR ONLINE.",
	})
	_, err := pool.Exec(ctx,
		`UPDATE messages SET created_at = NOW() - INTERVAL '120 days' WHERE room_id = $1`, quiet.ID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx,
		`UPDATE rooms SET created_at = NOW() - INTERVAL '120 days',
		                  last_active_at = NOW() - INTERVAL '120 days'
		 WHERE id = $1`, quiet.ID)
	require.NoError(t, err)

	// One published post, and two that were never published to anyone.
	hpoInsertPostWithReply(t, pool, "hpo published all-time post", "public")
	for _, unpublished := range []struct{ title, status, visibility string }{
		{"hpo draft all-time post", "draft", "public"},
		{"hpo family all-time post", "open", "family"},
	} {
		_, err := pool.Exec(ctx,
			`INSERT INTO posts (type, title, description, posted_by_type, posted_by_id, status, visibility)
			 VALUES ('problem', $1, 'seeded by the all-time overview test', 'agent', 'agent_hpotest', $2, $3)`,
			unpublished.title, unpublished.status, unpublished.visibility)
		require.NoError(t, err)
	}

	byKey := func(ov hpoOverview) map[string]hpoMetric {
		out := map[string]hpoMetric{}
		for _, m := range ov.Community.Metrics {
			out[m.Key] = m
		}
		return out
	}

	day := getHomepageOverviewWindow(t, ts.URL, "24h")
	week := getHomepageOverviewWindow(t, ts.URL, "7d")
	month := getHomepageOverviewWindow(t, ts.URL, "30d")

	// Choosing a window changes the ACTIVITY figures' own window...
	assert.Equal(t, "24h", day.Rooms.SelectedWindow)
	assert.Equal(t, "30d", month.Rooms.SelectedWindow)
	// ...and leaves every all-time total exactly where it was.
	require.Equal(t, day.Community.Metrics, week.Community.Metrics,
		"7 days must not move a total that has no window")
	require.Equal(t, day.Community.Metrics, month.Community.Metrics,
		"30 days must not move a total that has no window")

	now, was := byKey(day), byKey(before)

	assert.Equal(t, 1, now["public_rooms"].Value-was["public_rooms"].Value,
		"a room with no activity in any window is archived, not erased")
	assert.Equal(t, 1, now["published_posts"].Value-was["published_posts"].Value,
		"the draft and the family-scoped post were never published")

	// The totals are the database's answer, not a snapshot written into the code.
	var publicRooms, publishedPosts, registeredAgents, registeredHumans int
	require.NoError(t, pool.QueryRow(ctx, `
		SELECT
			(SELECT COUNT(*) FROM rooms WHERE is_private = FALSE AND deleted_at IS NULL),
			(SELECT COUNT(*) FROM posts WHERE deleted_at IS NULL AND visibility = 'public'
				AND status NOT IN ('draft', 'pending_review', 'rejected')),
			(SELECT COUNT(*) FROM agents WHERE deleted_at IS NULL AND status <> 'suspended'),
			(SELECT COUNT(*) FROM users WHERE deleted_at IS NULL)
	`).Scan(&publicRooms, &publishedPosts, &registeredAgents, &registeredHumans))

	assert.Equal(t, publicRooms, now["public_rooms"].Value)
	assert.Equal(t, publishedPosts, now["published_posts"].Value)
	assert.Equal(t, registeredAgents, now["registered_agents"].Value)
	assert.Equal(t, registeredHumans, now["registered_humans"].Value)
}

// A registered account is a registration. The public payload may say how many
// there are and nothing whatsoever about who they are.
func TestHomepageOverview_AllTimeTotalsPublishNoAccountAttributes(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	defer hpoCleanup(t, pool)

	ov, _ := getHomepageOverview(t, ts.URL)

	section, err := json.Marshal(ov.Community)
	require.NoError(t, err)
	serialized := string(section)

	assert.NotContains(t, serialized, "@", "no address of any kind reaches the all-time totals")
	assert.NotRegexp(t, `[0-9a-f]{8}-[0-9a-f]{4}-`, serialized, "no account identifier reaches the all-time totals")

	// Whatever the database actually holds, none of it is named here.
	var username, email string
	err = pool.QueryRow(context.Background(),
		`SELECT username, email FROM users WHERE deleted_at IS NULL LIMIT 1`).Scan(&username, &email)
	if err == nil {
		assert.NotContains(t, serialized, username)
		assert.NotContains(t, serialized, email)
	}

	// And an account total never presents itself as an audience.
	for _, m := range ov.Community.Metrics {
		if !strings.HasPrefix(m.Key, "registered_") {
			continue
		}
		assert.NotContains(t, strings.ToLower(m.Label), "active")
		assert.NotContains(t, strings.ToLower(m.Label), "visitor")
		assert.NotContains(t, strings.ToLower(m.Definition), "active")
		assert.NotEmpty(t, m.Qualifier, "%s states that a registration is not use", m.Key)
	}
}
