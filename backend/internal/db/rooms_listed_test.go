package db

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// insertListedTestRoom writes a public room, listed or not, holding a two-way exchange.
func insertListedTestRoom(t *testing.T, pool *Pool, slug string, listed bool, sourcePost *string) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	var id uuid.UUID
	require.NoError(t, pool.QueryRow(ctx, `INSERT INTO rooms (slug, display_name, is_private, listed, source_post_id)
		VALUES ($1, $2, false, $3, $4::uuid) RETURNING id`, slug, slug, listed, sourcePost).Scan(&id))
	t.Cleanup(func() { pool.Exec(context.Background(), `DELETE FROM rooms WHERE id = $1`, id) }) //nolint:errcheck
	for _, who := range []string{"planner-" + slug, "executor-" + slug} {
		_, err := pool.Exec(ctx, `INSERT INTO messages (room_id, agent_name, author_type, author_id, content)
			VALUES ($1, $2, 'agent', $2, 'listed rule probe')`, id, who)
		require.NoError(t, err)
	}
	return id
}

// SPEC.md 27.1: every public surface that names, lists or quotes a room reads listed rooms
// only, and featuring a room lists it in the same statement (owner decision F2, 2026-10-17).
func TestListedRule_EveryPublicListingSkipsUnlistedRoomsAndFeaturingLists(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()
	ctx := context.Background()
	tag := fmt.Sprintf("%d", time.Now().UnixNano()%1000000000)

	// Featuring an unlisted public room lists it; unlisting it later takes it off the
	// public featured list.
	featuredSlug := "listed-feat-" + tag
	featuredID := insertListedTestRoom(t, pool, featuredSlug, false, nil)
	featured := NewFeaturedRoomRepository(pool)
	_, err := featured.Feature(ctx, featuredSlug, nil, nil)
	require.NoError(t, err, "an unlisted public room can be featured")
	var listed bool
	require.NoError(t, pool.QueryRow(ctx, `SELECT listed FROM rooms WHERE id = $1`, featuredID).Scan(&listed))
	assert.True(t, listed, "featuring lists the room")
	assert.True(t, containsFeatured(t, featured, featuredSlug), "a featured, listed room is on the public list")
	_, err = pool.Exec(ctx, `UPDATE rooms SET listed = false WHERE id = $1`, featuredID)
	require.NoError(t, err)
	assert.False(t, containsFeatured(t, featured, featuredSlug), "an unlisted room leaves the public featured list")

	// The homepage's recently completed rooms and activity feed quote listed rooms only.
	unlisted := insertListedTestRoom(t, pool, "listed-no-"+tag, false, nil)
	listedID := insertListedTestRoom(t, pool, "listed-yes-"+tag, true, nil)
	home := NewHomepageRepository(pool)
	recent, err := home.GetRecentCompletedRooms(ctx, RoomStatsWindows[0])
	require.NoError(t, err)
	recentIDs := map[uuid.UUID]bool{}
	for _, r := range recent {
		recentIDs[r.RoomID] = true
	}
	assert.False(t, recentIDs[unlisted], "an unlisted room is not a recently completed room")
	assert.True(t, recentIDs[listedID], "a listed room is")
	feed, err := home.ListPublicRoomFeed(ctx, 100, 0)
	require.NoError(t, err)
	feedSlugs := map[string]bool{}
	for _, e := range feed {
		feedSlugs[e.RoomSlug] = true
	}
	assert.False(t, feedSlugs["listed-no-"+tag], "the activity feed never quotes an unlisted room")
	assert.True(t, feedSlugs["listed-yes-"+tag], "it quotes a listed one")

	// Presence discovery lists agents in listed rooms only.
	presence := NewAgentPresenceRepository(pool)
	probe := "agent_listed_probe_" + tag
	insertRemapAgent(t, pool, ctx, probe)
	t.Cleanup(func() { pool.Exec(context.Background(), `DELETE FROM agents WHERE id = $1`, probe) }) //nolint:errcheck
	for _, room := range []uuid.UUID{unlisted, listedID} {
		_, err = pool.Exec(ctx, `INSERT INTO room_members (room_id, agent_id, role, added_by)
			VALUES ($1, $2, 'member', $2)`, room, probe)
		require.NoError(t, err)
		_, err = pool.Exec(ctx, `INSERT INTO agent_presence (room_id, agent_id, agent_name, card_json, ttl_seconds)
			VALUES ($1, $2, $2, '{}'::jsonb, 600)`, room, probe)
		require.NoError(t, err)
	}
	present, err := presence.ListAllPublic(ctx)
	require.NoError(t, err)
	presentRooms := map[uuid.UUID]bool{}
	for _, p := range present {
		presentRooms[p.RoomID] = true
	}
	assert.False(t, presentRooms[unlisted], "discovery never names an unlisted room")
	assert.True(t, presentRooms[listedID])

	// A post shows its related public rooms that are listed.
	post := insertTestPost(t, pool, ctx, "post", "Listed rule source post "+tag, "body", nil, "open")
	insertListedTestRoom(t, pool, "listed-src-no-"+tag, false, &post)
	insertListedTestRoom(t, pool, "listed-src-yes-"+tag, true, &post)
	related, err := NewRoomRepository(pool).FindPublicRoomsBySourcePost(ctx, post)
	require.NoError(t, err)
	relatedSlugs := map[string]bool{}
	for _, r := range related {
		relatedSlugs[r.Slug] = true
		assert.True(t, r.Listed, "the related room is read with its listed flag")
	}
	assert.False(t, relatedSlugs["listed-src-no-"+tag])
	assert.True(t, relatedSlugs["listed-src-yes-"+tag])
}

func containsFeatured(t *testing.T, repo *FeaturedRoomRepository, slug string) bool {
	t.Helper()
	rooms, err := repo.ListPublic(context.Background())
	require.NoError(t, err)
	for _, r := range rooms {
		if r.Slug == slug {
			return true
		}
	}
	return false
}
