package db

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// The public room list (spec.json idx 77 step 1: room activity, measured on representative data).
// Measured on HEAD before this change (slice 14 spike: 20k rooms, 2M room entries, half of them in
// 20 hot rooms): each listed room's unique_participant_count was a COUNT(DISTINCT author_id) over
// every live message of the room, ~75 ms for a 23k-message room, so the newest page took 1.5 s
// and an offset page 2.5 s. A room's participants are a handful of authors; counting them must
// cost one index probe per author, not one read of every message.

// seedRoomListing writes 2,000 rooms (10% private) and 600k room entries five seconds apart: half
// in 20 hot rooms, the rest spread; per room a pool of 8 agents and 2 humans, room-1 with 150
// agents; events, system messages, author-less agent messages and deleted messages mixed in.
func seedRoomListing(ctx context.Context, t *testing.T, pool *Pool) {
	t.Helper()
	tx, err := pool.BeginTx(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(ctx) }()
	for _, step := range []struct{ name, sql string }{
		{"replica", `SET LOCAL session_replication_role = replica`},
		{"rooms", `
			INSERT INTO rooms (slug, display_name, description, is_private, created_at, last_active_at)
			SELECT 'rl-' || g, 'Listed room ' || g, 'What listed room ' || g || ' is for', g % 10 = 0,
			       NOW() - INTERVAL '40 days', NOW() - g * INTERVAL '1 minute'
			  FROM generate_series(1, 2000) g`},
		{"entries", `
			WITH listed AS (SELECT id, row_number() OVER (ORDER BY last_active_at DESC) rn FROM rooms WHERE slug LIKE 'rl-%'),
			     g AS (SELECT g, (g / 40 + g / 2000) % 20 AS m FROM generate_series(1, 600000) g)
			INSERT INTO room_entries (room_id, sequence, kind, author_type, author_id, actor_label, body, event_type,
			                          created_at, deleted_at)
			SELECT r.id, g.g, CASE WHEN g.m IN (0, 1, 2) THEN 'event' ELSE 'message' END,
			       CASE WHEN g.m = 3 THEN 'system' WHEN g.m = 4 THEN 'human' ELSE 'agent' END,
			       CASE WHEN g.m IN (0, 1, 2, 3) OR g.g % 7 = 0 THEN NULL
			            WHEN g.m = 4 THEN 'human-' || (r.rn * 2 + g.g % 2)
			            WHEN r.rn = 1 THEN 'rl_agent_' || (g.g / 2 % 150)
			            ELSE 'rl_agent_' || (r.rn * 8 + g.g % 8) END,
			       'Author ' || (g.g % 8), CASE WHEN g.m NOT IN (0, 1, 2) THEN 'Body ' || g.g END,
			       CASE WHEN g.m IN (0, 1, 2) THEN 'heartbeat' END,
			       NOW() - (600000 - g.g) * INTERVAL '5 seconds', CASE WHEN g.g % 100 = 9 THEN NOW() END
			  FROM g JOIN listed r ON r.rn = CASE WHEN g.g % 2 = 0 THEN 1 + (g.g / 2 % 20) ELSE 1 + (g.g % 2000) END`},
		{"message counts", `
			UPDATE rooms ro SET message_count = c.n
			  FROM (SELECT room_id, COUNT(*) FILTER (WHERE kind = 'message' AND deleted_at IS NULL) n
			          FROM room_entries GROUP BY room_id) c
			 WHERE c.room_id = ro.id`},
	} {
		_, err := tx.Exec(ctx, step.sql)
		require.NoError(t, err, "seed %s", step.name)
	}
	require.NoError(t, tx.Commit(ctx))
	for _, table := range []string{"rooms", "room_entries"} {
		_, err = pool.Exec(ctx, "VACUUM ANALYZE "+table)
		require.NoError(t, err)
	}
}

// participantsByDefinition is a room's unique participants as the product defines them: distinct
// authors of its live messages that carry an authenticated author.
func participantsByDefinition(ctx context.Context, t *testing.T, pool *Pool, room uuid.UUID) int {
	t.Helper()
	var n int
	require.NoError(t, pool.QueryRow(ctx, `
		SELECT COUNT(DISTINCT author_id) FROM messages
		 WHERE room_id = $1 AND deleted_at IS NULL AND author_id IS NOT NULL`, room).Scan(&n))
	return n
}

func TestRoomList_AtGrowthVolumeParticipantCountsMatchTheDefinitionAndReadOnlyTheirAuthors(t *testing.T) {
	scratch, _ := newMigratedScratchDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	seedRoomListing(ctx, t, scratch)

	capture := &statementCapture{}
	traced, err := NewPool(ctx, scratch.Config().ConnString(), WithQueryTracer(capture))
	require.NoError(t, err)
	defer traced.Close()
	repo := NewRoomRepository(traced)

	var pages int
	require.NoError(t, scratch.QueryRow(ctx, `SELECT relpages FROM pg_class WHERE oid = 'room_entries'::regclass`).Scan(&pages))
	require.Greater(t, pages, 2000, "room_entries spans enough pages to tell an author probe from a scan")

	for _, c := range []struct {
		name   string
		params RoomListParams
	}{
		{"recent", RoomListParams{Limit: 20}},
		{"active", RoomListParams{Limit: 20, Sort: "active"}},
		{"query", RoomListParams{Limit: 20, Query: "listed room 1"}},
		{"offset", RoomListParams{Limit: 20, Offset: 200}},
	} {
		t.Run(c.name, func(t *testing.T) {
			capture.take()
			start := time.Now()
			got, err := repo.ListFiltered(ctx, c.params)
			took := time.Since(start)
			require.NoError(t, err)
			require.Len(t, got, 20, "the seed fills this page")
			hot := 0
			for _, room := range got {
				want := participantsByDefinition(ctx, t, scratch, room.ID)
				require.Equal(t, want, room.UniqueParticipantCount, "room %s participants", room.Slug)
				if room.MessageCount > 1000 {
					hot++
				}
			}
			t.Logf("%s: %s, %d hot rooms on the page", c.name, took.Round(time.Millisecond), hot)

			stmts := capture.take()
			require.Len(t, stmts, 1)
			buffers, seq := tableReads(ctx, t, scratch, stmts[0])
			t.Logf("  buffers %v seq %v (room_entries pages %d)", buffers, seq, pages)
			require.False(t, seq["room_entries"], "the room list scans room_entries")
			require.Less(t, buffers["room_entries"], pages/10,
				"the room list reads %d room_entries pages; the table has %d", buffers["room_entries"], pages)
		})
	}
}
