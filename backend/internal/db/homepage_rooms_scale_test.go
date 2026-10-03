package db

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// The homepage room section (spec.json idx 77 step 1: room activity, measured on representative
// data). Measured on HEAD before this change (slice 14 spike: 20k rooms, 2M room entries):
// GetRoomPulse took 4.4-5.2 s whatever the window, because each of its five message figures was its
// own full scan of room_entries, and the two activation figures read the whole (room_id,
// event_type) index to find one event type. A call may read the selected window's messages once
// for its figures and once for its series, not the table five times over.

// pulseByDefinition is the room section's thirteen figures as the product defines them, written
// plainly, one subquery per figure: HEAD's statement before this change. $1 is the window, $2 the
// activation event type.
const pulseByDefinition = `
	SELECT
		(SELECT COUNT(DISTINCT ap.agent_id) FROM agent_presence ap JOIN rooms r ON r.id = ap.room_id
		  WHERE ` + countedLiveRoomPredicate + ` AND ` + unexpiredPresence + `),
		(SELECT COUNT(DISTINCT ap.agent_id) FROM agent_presence ap JOIN rooms r ON r.id = ap.room_id
		  WHERE ` + liveRoomPredicate + ` AND ` + unexpiredPresence + `),
		(SELECT COUNT(DISTINCT ap.agent_id) FROM agent_presence ap JOIN rooms r ON r.id = ap.room_id
		  WHERE ` + countedLiveRoomPredicate + ` AND ` + unexpiredPresence + ` AND ` + verifiedPresence + `),
		(SELECT COUNT(DISTINCT ap.room_id) FROM agent_presence ap JOIN rooms r ON r.id = ap.room_id
		  WHERE ` + countedLiveRoomPredicate + ` AND ` + unexpiredPresence + `),
		(SELECT COUNT(DISTINCT m.room_id) FROM messages m JOIN rooms r ON r.id = m.room_id
		  WHERE r.deleted_at IS NULL AND m.deleted_at IS NULL AND m.author_type <> 'system'
		    AND m.created_at > NOW() - $1::interval),
		(SELECT COUNT(*) FROM messages m JOIN rooms r ON r.id = m.room_id
		  WHERE r.deleted_at IS NULL AND m.deleted_at IS NULL AND m.author_type = 'agent'
		    AND m.created_at > NOW() - $1::interval),
		(SELECT COUNT(*) FROM messages m JOIN rooms r ON r.id = m.room_id
		  WHERE r.deleted_at IS NULL AND m.deleted_at IS NULL AND m.author_type = 'agent' AND m.author_id IS NULL
		    AND m.created_at > NOW() - $1::interval),
		(SELECT COUNT(*) FROM messages m JOIN rooms r ON r.id = m.room_id
		  WHERE r.deleted_at IS NULL AND m.deleted_at IS NULL AND m.author_type = 'human'
		    AND m.created_at > NOW() - $1::interval),
		(SELECT COUNT(DISTINCT e.room_id) FROM room_events e JOIN rooms r ON r.id = e.room_id
		  WHERE r.deleted_at IS NULL AND e.event_type = $2 AND e.created_at > NOW() - $1::interval),
		(SELECT MIN(e.created_at) FROM room_events e JOIN rooms r ON r.id = e.room_id
		  WHERE r.deleted_at IS NULL AND e.event_type = $2),
		(SELECT COUNT(*) FROM rooms r WHERE r.deleted_at IS NULL),
		(SELECT COUNT(*) FROM rooms r WHERE r.deleted_at IS NULL AND r.is_private = FALSE),
		(SELECT COUNT(*) FROM messages m JOIN rooms r ON r.id = m.room_id
		  WHERE r.deleted_at IS NULL AND m.deleted_at IS NULL AND m.author_type <> 'system'
		    AND m.created_at > NOW() - INTERVAL '24 hours'),
		(SELECT COUNT(DISTINCT m.room_id) FROM messages m JOIN rooms r ON r.id = m.room_id
		  WHERE r.deleted_at IS NULL AND m.deleted_at IS NULL AND m.author_type <> 'system'
		    AND m.created_at > NOW() - INTERVAL '24 hours')`

// roomPulseByDefinition reads the figures and the series as defined. The series is returned as
// bucket start -> count, quiet buckets absent.
func roomPulseByDefinition(ctx context.Context, t *testing.T, pool *Pool, w RoomStatsWindow) (RoomPulse, map[time.Time]int) {
	t.Helper()
	p := RoomPulse{Window: w}
	require.NoError(t, pool.QueryRow(ctx, pulseByDefinition, w.interval(), RoomActivationEventType).Scan(
		&p.Presence.AgentsOnline, &p.Presence.PublicAgentsOnline, &p.Presence.VerifiedAgentsOnline,
		&p.Presence.RoomsWithAgentsOnline, &p.Stats.RoomsWithConversation, &p.Stats.AgentMessages,
		&p.Stats.UnverifiedAgentMessages, &p.Stats.HumanMessages, &p.Stats.TwoWayExchangeRooms,
		&p.Stats.ActivationInstrumentedSince, &p.AllRooms, &p.PublicRooms, &p.Messages24h, &p.ActiveRooms24h))
	p.Presence.UnverifiedAgentsOnline = p.Presence.AgentsOnline - p.Presence.VerifiedAgentsOnline

	rows, err := pool.Query(ctx, `
		SELECT date_trunc($1, m.created_at), COUNT(*)
		  FROM messages m JOIN rooms r ON r.id = m.room_id
		 WHERE r.deleted_at IS NULL AND m.deleted_at IS NULL AND m.author_type <> 'system'
		   AND m.created_at >= date_trunc($1, NOW()) - ($2::int - 1) * $3::interval
		 GROUP BY 1`, w.BucketUnit, w.Buckets, w.bucketInterval())
	require.NoError(t, err)
	defer rows.Close()
	series := map[time.Time]int{}
	for rows.Next() {
		var b time.Time
		var n int
		require.NoError(t, rows.Scan(&b, &n))
		series[b.UTC()] = n
	}
	require.NoError(t, rows.Err())
	return p, series
}

// seedRoomPulseExtras adds what the feed seed lacks: activation milestones in a quarter of the
// rooms over the last 40 days, and live and expired presence, verified by a live room token, by a
// rotated one, or by none.
func seedRoomPulseExtras(ctx context.Context, t *testing.T, pool *Pool) {
	t.Helper()
	tx, err := pool.BeginTx(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(ctx) }()
	for _, step := range []struct{ name, sql string }{
		{"replica", `SET LOCAL session_replication_role = replica`},
		{"activations", `
			INSERT INTO room_entries (room_id, sequence, kind, actor_label, event_type, issue, created_at)
			SELECT r.id, 1000000 + r.rn, 'event', 'solvr', 'room.activated', '',
			       NOW() - (r.rn * 1440) * INTERVAL '1 second'
			  FROM (SELECT id, row_number() OVER (ORDER BY slug) rn FROM rooms WHERE slug LIKE 'rf-%') r
			 WHERE r.rn % 4 = 1`},
		{"presence", `
			INSERT INTO agent_presence (room_id, agent_name, agent_id, card_json, last_seen, ttl_seconds)
			SELECT r.id, 'presence-' || a, 'rf_agent_' || (r.rn % 300) || '_' || a,
			       '{}'::jsonb, NOW() - CASE WHEN a % 5 = 0 THEN INTERVAL '2 hours' ELSE INTERVAL '1 minute' END, 900
			  FROM (SELECT id, row_number() OVER (ORDER BY slug) rn FROM rooms WHERE slug LIKE 'rf-%') r,
			       generate_series(1, 3) a
			 WHERE r.rn % 3 = 0`},
		{"tokens", `
			INSERT INTO room_agent_tokens (room_id, agent_id, token_hash, rotated_at)
			SELECT ap.room_id, ap.agent_id, md5(ap.room_id::text || ap.agent_id),
			       CASE WHEN ap.agent_name = 'presence-2' THEN NOW() END
			  FROM agent_presence ap WHERE ap.agent_name IN ('presence-1', 'presence-2')`},
	} {
		_, err := tx.Exec(ctx, step.sql)
		require.NoError(t, err, "seed %s", step.name)
	}
	require.NoError(t, tx.Commit(ctx))
	for _, table := range []string{"room_entries", "agent_presence", "room_agent_tokens"} {
		_, err = pool.Exec(ctx, "VACUUM ANALYZE "+table)
		require.NoError(t, err)
	}
}

func TestRoomPulse_AtGrowthVolumeEveryWindowMatchesTheDefinitionAndReadsItsWindowOnce(t *testing.T) {
	scratch, _ := newMigratedScratchDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	seedRoomFeed(ctx, t, scratch)
	seedRoomPulseExtras(ctx, t, scratch)

	capture := &statementCapture{}
	traced, err := NewPool(ctx, scratch.Config().ConnString(), WithQueryTracer(capture))
	require.NoError(t, err)
	defer traced.Close()
	repo := NewHomepageRepository(traced)

	var pages int
	require.NoError(t, scratch.QueryRow(ctx, `SELECT relpages FROM pg_class WHERE oid = 'room_entries'::regclass`).Scan(&pages))
	require.Greater(t, pages, 2000, "room_entries spans enough pages to tell a window read from a scan")

	for _, w := range RoomStatsWindows {
		t.Run(w.Value, func(t *testing.T) {
			// The window slides between two reads; a figure is compared only when the definition
			// read the same before and after the repository did.
			var got RoomPulse
			var took time.Duration
			var series map[time.Time]int
			for attempt := 0; ; attempt++ {
				before, beforeSeries := roomPulseByDefinition(ctx, t, scratch, w)
				capture.take()
				start := time.Now()
				got, err = repo.GetRoomPulse(ctx, w)
				took = time.Since(start)
				require.NoError(t, err)
				after, afterSeries := roomPulseByDefinition(ctx, t, scratch, w)
				if before.Stats.ActivationInstrumentedSince != nil && after.Stats.ActivationInstrumentedSince != nil &&
					before.Presence == after.Presence && before.AllRooms == after.AllRooms &&
					before.Messages24h == after.Messages24h && before.Stats.AgentMessages == after.Stats.AgentMessages &&
					before.Stats.HumanMessages == after.Stats.HumanMessages &&
					before.Stats.UnverifiedAgentMessages == after.Stats.UnverifiedAgentMessages &&
					before.Stats.RoomsWithConversation == after.Stats.RoomsWithConversation &&
					before.Stats.TwoWayExchangeRooms == after.Stats.TwoWayExchangeRooms &&
					len(beforeSeries) == len(afterSeries) && equalCounts(beforeSeries, afterSeries) {
					want := before
					want.Stats.Series = got.Stats.Series
					require.True(t, want.Stats.ActivationInstrumentedSince.Equal(*got.Stats.ActivationInstrumentedSince))
					want.Stats.ActivationInstrumentedSince = got.Stats.ActivationInstrumentedSince
					require.Equal(t, want, got)
					series = beforeSeries
					break
				}
				require.Less(t, attempt, 4, "the definition never held still across a call")
			}

			require.Len(t, got.Stats.Series, w.Buckets)
			total := 0
			for _, b := range got.Stats.Series {
				require.Equal(t, series[b.BucketStart], b.Count, "bucket %s", b.BucketStart)
				total += b.Count
			}
			for start := range series {
				require.False(t, start.Before(got.Stats.Series[0].BucketStart), "a counted bucket %s is missing", start)
			}
			require.Positive(t, got.Stats.AgentMessages)
			require.Positive(t, got.Stats.HumanMessages)
			require.Positive(t, got.Stats.UnverifiedAgentMessages)
			require.Positive(t, got.Stats.TwoWayExchangeRooms)
			require.Positive(t, got.Presence.VerifiedAgentsOnline)
			require.Positive(t, got.Presence.UnverifiedAgentsOnline)
			t.Logf("%s: %s; messages %d agent + %d human, %d rooms, series total %d",
				w.Value, took.Round(time.Millisecond), got.Stats.AgentMessages, got.Stats.HumanMessages,
				got.Stats.RoomsWithConversation, total)

			// The read bound: the figures read the window's messages once and the series once more,
			// each with the index pages above them: at most three times the heap pages the window's
			// entries live on. A 24-hour window is a few percent of this table and may not scan it.
			var windowPages int
			require.NoError(t, scratch.QueryRow(ctx, `SELECT COUNT(DISTINCT (ctid::text::point)[0]) FROM room_entries
				WHERE created_at > NOW() - $1::interval`, w.interval()).Scan(&windowPages))
			stmts := capture.take()
			require.Len(t, stmts, 2)
			sum := 0
			for i, s := range stmts {
				buffers, seq := tableReads(ctx, t, scratch, s)
				t.Logf("  statement %d room_entries buffers %d seq %v (window pages %d, table pages %d)", i+1,
					buffers["room_entries"], seq["room_entries"], windowPages, pages)
				sum += buffers["room_entries"]
				if w.Value == "24h" {
					require.False(t, seq["room_entries"], "a 24-hour statement scans room_entries")
				}
			}
			require.LessOrEqual(t, sum, 3*windowPages,
				"the %s section reads %d room_entries pages; its window lives on %d of the table's %d",
				w.Value, sum, windowPages, pages)
		})
	}
}

func equalCounts(a, b map[time.Time]int) bool {
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}
