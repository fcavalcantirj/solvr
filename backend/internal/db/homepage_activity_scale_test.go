package db

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

// The public room activity feed (spec.json idx 77 step 1: room activity, measured on representative
// data). Measured on HEAD before this change (slice 14 spike: 20k rooms, 2M room entries): the
// newest page took 1.6-2.0 s and the New activity count 225 ms, because every request read every
// eligible live message and event of every public room, then sorted them all to keep a handful.
// A page of the newest entries must cost about as many entries as it shows, not the whole history.

// seedRoomFeed writes 2,000 rooms (10% private, some deleted, some expired), 50 users and 600k room
// entries five seconds apart: half in 20 hot rooms, the rest spread; heartbeats, allow-listed work
// events in mixed case, system messages, human messages (some by unknown accounts), agent messages,
// and 1% deleted messages mixed in. The kind is a hash of the entry, not of its room or its block, so
// every page mixes kinds the way live traffic does.
func seedRoomFeed(ctx context.Context, t *testing.T, pool *Pool) {
	t.Helper()
	tx, err := pool.BeginTx(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(ctx) }()
	for _, step := range []struct{ name, sql string }{
		{"replica", `SET LOCAL session_replication_role = replica`},
		{"users", `
			INSERT INTO users (id, username, display_name, email, referral_code)
			SELECT gen_random_uuid(), 'rf' || g, 'Feed user ' || g, 'rf' || g || '@example.test',
			       'F' || lpad(g::text, 7, '0')
			  FROM generate_series(1, 50) g`},
		{"rooms", `
			INSERT INTO rooms (slug, display_name, description, is_private, created_at, last_active_at,
			                   deleted_at, expires_at)
			SELECT 'rf-' || g, 'Feed room ' || g, 'What feed room ' || g || ' is for', g % 10 = 0,
			       NOW() - INTERVAL '40 days', NOW() - g * INTERVAL '1 minute',
			       CASE WHEN g % 40 = 7 THEN NOW() END,
			       CASE WHEN g % 25 = 3 THEN NOW() - INTERVAL '1 day' WHEN g % 25 = 4 THEN NOW() + INTERVAL '1 day' END
			  FROM generate_series(1, 2000) g`},
		{"entries", `
			WITH listed AS (SELECT id, row_number() OVER (ORDER BY last_active_at DESC) rn FROM rooms WHERE slug LIKE 'rf-%'),
			     people AS (SELECT id::text AS id, row_number() OVER (ORDER BY username) - 1 AS n FROM users WHERE username LIKE 'rf%'),
			     g AS (SELECT g, (hashint4(g) & 1048575) % 20 AS m FROM generate_series(1, 600000) g)
			INSERT INTO room_entries (room_id, sequence, kind, author_type, author_id, actor_label, body, event_type,
			                          issue, created_at, deleted_at)
			SELECT r.id, g.g, CASE WHEN g.m IN (0, 1, 2) THEN 'event' ELSE 'message' END,
			       CASE WHEN g.m = 3 THEN 'system' WHEN g.m = 4 THEN 'human' ELSE 'agent' END,
			       CASE WHEN g.m IN (0, 1, 2, 3) OR g.g % 7 = 0 THEN NULL
			            WHEN g.m = 4 THEN COALESCE(p.id, 'gone-' || g.g)
			            ELSE 'rf_agent_' || (r.rn * 8 + g.g % 8) END,
			       'Author ' || (g.g % 8), CASE WHEN g.m NOT IN (0, 1, 2) THEN 'Body ' || g.g END,
			       CASE WHEN g.m = 0 THEN 'heartbeat'
			            WHEN g.m IN (1, 2) THEN (ARRAY['CLAIM', 'building', 'Done', 'PR', 'token.rotated'])[1 + g.g % 5] END,
			       CASE WHEN g.m IN (1, 2) THEN 'issue-' || (g.g % 13) ELSE '' END,
			       NOW() - (600000 - g.g) * INTERVAL '5 seconds', CASE WHEN g.g % 100 = 9 THEN NOW() END
			  FROM g JOIN listed r ON r.rn = CASE WHEN g.g % 2 = 0 THEN 1 + (g.g / 2 % 20) ELSE 1 + (g.g % 2000) END
			  LEFT JOIN people p ON g.m = 4 AND p.n = g.g % 60`},
	} {
		_, err := tx.Exec(ctx, step.sql)
		require.NoError(t, err, "seed %s", step.name)
	}
	require.NoError(t, tx.Commit(ctx))
	for _, table := range []string{"users", "rooms", "room_entries"} {
		_, err = pool.Exec(ctx, "VACUUM ANALYZE "+table)
		require.NoError(t, err)
	}
}

// roomFeedByDefinition is the public activity feed as the product defines it, written as the
// plainest possible query: every eligible message and allow-listed event of every live public room,
// in one order. The repository must return exactly this, however it reads it.
const roomFeedByDefinition = `
	SELECT * FROM (
		SELECT 'message' AS kind, m.id AS entry_id, m.sequence_num, r.slug, r.display_name, m.author_type,
		       CASE WHEN m.author_type = 'human' THEN COALESCE(u.username, '') ELSE m.agent_name END,
		       (m.author_id IS NOT NULL), m.content, m.metadata, '', '', m.created_at
		  FROM messages m JOIN rooms r ON r.id = m.room_id
		  LEFT JOIN users u ON m.author_type = 'human' AND u.id::text = m.author_id AND u.deleted_at IS NULL
		 WHERE m.deleted_at IS NULL AND m.author_type <> 'system'
		   AND r.deleted_at IS NULL AND NOT r.is_private AND (r.expires_at IS NULL OR r.expires_at > NOW())
		UNION ALL
		SELECT 'event', e.id, NULL::int, r.slug, r.display_name, 'agent', e.actor, FALSE, '', '{}'::jsonb,
		       e.event_type, e.issue, e.created_at
		  FROM room_events e JOIN rooms r ON r.id = e.room_id
		 WHERE upper(e.event_type) = ANY($1::text[])
		   AND r.deleted_at IS NULL AND NOT r.is_private AND (r.expires_at IS NULL OR r.expires_at > NOW())
	) feed`

func roomFeedPageByDefinition(ctx context.Context, t *testing.T, pool *Pool, limit, offset int) []PublicRoomFeedEntry {
	t.Helper()
	rows, err := pool.Query(ctx, roomFeedByDefinition+` ORDER BY 13 DESC, 2 DESC LIMIT $2 OFFSET $3`,
		PublicFeedEventTypes, limit, offset)
	require.NoError(t, err)
	defer rows.Close()
	var out []PublicRoomFeedEntry
	for rows.Next() {
		var e PublicRoomFeedEntry
		require.NoError(t, rows.Scan(&e.Kind, &e.EntryID, &e.SequenceNum, &e.RoomSlug, &e.RoomName,
			&e.AuthorType, &e.AuthorName, &e.AuthorVerified, &e.Content, &e.Metadata,
			&e.EventType, &e.Issue, &e.CreatedAt))
		out = append(out, e)
	}
	require.NoError(t, rows.Err())
	return out
}

func TestRoomFeed_AtGrowthVolumeEveryReadMatchesTheDefinitionAndReadsOnlyTheNewestEntries(t *testing.T) {
	scratch, _ := newMigratedScratchDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	seedRoomFeed(ctx, t, scratch)

	capture := &statementCapture{}
	traced, err := NewPool(ctx, scratch.Config().ConnString(), WithQueryTracer(capture))
	require.NoError(t, err)
	defer traced.Close()
	repo := NewHomepageRepository(traced)

	var pages int
	require.NoError(t, scratch.QueryRow(ctx, `SELECT relpages FROM pg_class WHERE oid = 'room_entries'::regclass`).Scan(&pages))
	require.Greater(t, pages, 2000, "room_entries spans enough pages to tell a newest-first read from a scan")

	// The read bound: a page or a capped count may touch the entries it returns, the ones it skips
	// for being private, deleted, system or not allow-listed, and the index pages above them. Not
	// a tenth of the table.
	bounded := func(t *testing.T, label string) {
		t.Helper()
		stmts := capture.take()
		require.Len(t, stmts, 1)
		buffers, seq := tableReads(ctx, t, scratch, stmts[0])
		t.Logf("  %s buffers %v seq %v (room_entries pages %d)", label, buffers, seq, pages)
		require.False(t, seq["room_entries"], "%s scans room_entries", label)
		require.Less(t, buffers["room_entries"], pages/10,
			"%s reads %d room_entries pages; the table has %d", label, buffers["room_entries"], pages)

		// The pool runs every statement as a cached prepared statement, and after five executions
		// Postgres may keep a GENERIC plan, planned without knowing the limit. EXPLAIN the seventh
		// EXECUTE: that is the plan production keeps.
		conn, err := NewPool(ctx, scratch.Config().ConnString(), func(c *pgxpool.Config) { c.MinConns, c.MaxConns = 1, 1 })
		require.NoError(t, err)
		defer conn.Close()
		_, err = conn.Exec(ctx, "PREPARE feed_read AS "+stmts[0].sql)
		require.NoError(t, err)
		literals := make([]string, len(stmts[0].args))
		for i, a := range stmts[0].args {
			switch v := a.(type) {
			case time.Time:
				literals[i] = "'" + v.Format(time.RFC3339Nano) + "'::timestamptz"
			default:
				literals[i] = fmt.Sprint(v)
			}
		}
		execute := "EXECUTE feed_read(" + strings.Join(literals, ", ") + ")"
		for i := 0; i < 6; i++ {
			_, err = conn.Exec(ctx, execute)
			require.NoError(t, err)
		}
		prepared, preparedSeq := tableReads(ctx, t, conn, capturedStatement{sql: execute})
		t.Logf("  %s 7th prepared execution buffers %v seq %v", label, prepared, preparedSeq)
		require.False(t, preparedSeq["room_entries"], "%s scans room_entries once its plan is cached", label)
		require.Less(t, prepared["room_entries"], pages/10, "%s reads %d room_entries pages once its plan is cached",
			label, prepared["room_entries"])
	}

	for _, c := range []struct {
		name          string
		limit, offset int
	}{
		{"overview", 7, 0},
		{"load more", 25, 0},
		{"offset", 25, 200},
	} {
		t.Run(c.name, func(t *testing.T) {
			capture.take()
			start := time.Now()
			got, err := repo.ListPublicRoomFeed(ctx, c.limit, c.offset)
			took := time.Since(start)
			require.NoError(t, err)
			want := roomFeedPageByDefinition(ctx, t, scratch, c.limit, c.offset)
			require.Len(t, want, c.limit, "the seed fills this page")
			require.Equal(t, want, got)
			kinds := map[string]int{}
			for _, e := range got {
				kinds[e.Kind]++
			}
			if c.limit >= 25 {
				require.Positive(t, kinds["event"], "the page mixes events in")
			}
			t.Logf("%s: %s, %v", c.name, took.Round(time.Millisecond), kinds)
			bounded(t, c.name)
		})
	}

	for _, c := range []struct {
		name string
		ago  time.Duration
	}{
		{"since a minute", time.Minute},
		{"since ten minutes", 10 * time.Minute},
		{"since a week (capped)", 7 * 24 * time.Hour},
	} {
		t.Run(c.name, func(t *testing.T) {
			since := time.Now().Add(-c.ago)
			var want int
			require.NoError(t, scratch.QueryRow(ctx, `SELECT LEAST(COUNT(*), 50) FROM (`+roomFeedByDefinition+`) d
				WHERE d.created_at > $2`, PublicFeedEventTypes, since).Scan(&want))
			capture.take()
			start := time.Now()
			got, err := repo.CountPublicRoomFeedSince(ctx, since, 50)
			took := time.Since(start)
			require.NoError(t, err)
			require.Equal(t, want, got)
			t.Logf("%s: %d in %s", c.name, got, took.Round(time.Millisecond))
			bounded(t, c.name)
		})
	}
}
