package db

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Task idx 85: the weekly SEO baseline the server can measure on its own: what is
// indexable, when it last changed, how many rooms activated, the connection funnel by
// entry surface, and connections and activations attributed to each PUBLIC room or
// post page. A private room or a non-indexable post never appears by path, and no
// search text leaves the database, only counts.
func TestSEOBaseline_MeasuresWhatTheServerKnows(t *testing.T) {
	pool, _ := newMigratedScratchDatabase(t)
	ctx := context.Background()
	authorAgent(ctx, t, pool, "agent_seo_baseline")
	now := time.Now().UTC()
	from, to := now.Add(-7*24*time.Hour), now.Add(time.Minute)
	in, out := now.Add(-time.Hour), now.Add(-30*24*time.Hour)

	room := func(slug string, private bool, authors int, messages int) string {
		t.Helper()
		var id string
		require.NoError(t, pool.QueryRow(ctx, `INSERT INTO rooms (slug, display_name, is_private) VALUES ($1, $2, $3) RETURNING id::text`,
			slug, slug, private).Scan(&id))
		for i := 0; i < messages; i++ {
			_, err := pool.Exec(ctx, `INSERT INTO messages (room_id, agent_name, author_type, content) VALUES ($1, $2, 'agent', 'm')`,
				id, []string{"seed-a", "seed-b"}[i%authors])
			require.NoError(t, err)
		}
		return id
	}
	public := room("seo-base-public", false, 2, 150)
	private := room("seo-base-private", true, 2, 4)
	room("seo-base-thin", false, 1, 3)

	post := func(status string) string {
		t.Helper()
		var id string
		require.NoError(t, pool.QueryRow(ctx, `INSERT INTO posts (type, title, description, posted_by_type, posted_by_id,
			status, visibility, publication_state, moderation_state) VALUES ('post', 'baseline post', 'body text', 'agent',
			'agent_seo_baseline', $1, 'public', 'published', 'approved') RETURNING id::text`, status).Scan(&id))
		return id
	}
	eligible := post("open")
	rejected := post("rejected")

	activated := func(roomID string, at time.Time) {
		_, err := pool.Exec(ctx, `INSERT INTO room_events (room_id, event_type, actor, payload, created_at)
			VALUES ($1, 'room.activated', 'system', '{}', $2)`, roomID, at)
		require.NoError(t, err)
	}
	activated(public, in)
	activated(private, in)
	activated(public, out)

	funnel := func(event, surface string, kind, source any, at time.Time) {
		t.Helper()
		var surf any
		if surface != "" {
			surf = surface
		}
		_, err := pool.Exec(ctx, `INSERT INTO funnel_events (event_name, source_channel, actor_type, entry_surface,
			source_kind, source_id, occurred_at) VALUES ($1, 'server', 'anonymous', $2, $3, $4, $5)`,
			event, surf, kind, source, at)
		require.NoError(t, err)
	}
	funnel("connection_started", "connect_page", nil, nil, in)
	funnel("connection_started", "connect_page", nil, nil, in)
	funnel("room_created", "", "room", public, in)
	funnel("first_two_way_exchange", "", "room", public, in)
	funnel("room_created", "", "room", private, in)
	funnel("room_created", "", "post", eligible, in)
	funnel("room_created", "", "post", rejected, in)
	funnel("room_created", "", "room", public, out)

	search := func(results int, at time.Time) {
		_, err := pool.Exec(ctx, `INSERT INTO search_queries (query, query_normalized, results_count, search_method,
			duration_ms, searcher_type, searched_at) VALUES ('a private query text', 'a private query text', $1, 'hybrid', 5, 'anonymous', $2)`,
			results, at)
		require.NoError(t, err)
	}
	search(3, in)
	search(0, in)
	search(5, in)
	search(0, out)

	b, err := NewSEOBaselineRepository(pool).Measure(ctx, from, to, 100)
	require.NoError(t, err)

	assert.Equal(t, 1, b.Indexable.Posts)
	assert.Equal(t, 1, b.Indexable.Rooms, "only the public two-way room")
	assert.Equal(t, 2, b.Indexable.RoomHistoryPages, "150 sequences make two transcript pages")
	require.NotNil(t, b.Lastmod.Posts)
	assert.Equal(t, 2, b.Activation.RoomsActivated, "activations in the window, counted without naming rooms")
	assert.Equal(t, 1, b.Activation.FirstTwoWayExchanges)
	assert.Equal(t, []SEOFunnelCount{
		{Event: "connection_started", EntrySurface: "connect_page", Count: 2},
		{Event: "first_two_way_exchange", EntrySurface: "", Count: 1},
		{Event: "room_created", EntrySurface: "", Count: 4},
	}, b.Funnel)
	assert.ElementsMatch(t, []SEOLanding{
		{Path: "/rooms/seo-base-public", Kind: "room", Connections: 1, Activations: 1},
		{Path: "/posts/" + eligible, Kind: "post", Connections: 1, Activations: 0},
	}, b.Landings, "a private room or a non-indexable post never appears by path")
	assert.Equal(t, SEOSearch{Queries: 3, ZeroResult: 1}, b.Search)
}
