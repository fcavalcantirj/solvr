package db

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"strings"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// Timeline replay, membership lookup and token lookup (spec.json idx 77 step 1), measured on
// representative data. Measured before this change (slice 17 spike: 2,000 rooms, 600k room
// entries, 16k members, 64k room tokens): every read below already sought its index and read a
// few to a few hundred buffers. The room entry index on (room_id, sequence) existed twice, once
// as the unique constraint and once as a plain index, and every room entry write paid for both:
// 100k inserts took 1,257 / 1,193 ms with both and 1,078 / 997 ms with one, and wrote 103-104 MB
// of WAL against 94 MB. Migration 000129 drops the plain copies of that and five other indexes.

// keptDuplicateIndexes are duplicates this schema keeps on purpose, with the reason.
var keptDuplicateIndexes = map[string]string{
	"idx_users_username": "TestMigrations_UsersTable pins it by name; users_username_key serves the same lookups",
	"idx_users_email":    "TestMigrations_UsersTable pins it by name; users_email_key serves the same lookups",
}

// TestSchema_NoIndexDuplicatesAnotherIndexOfItsTable: two indexes with the same access method,
// columns, expressions, operator classes, collations, orderings and predicate answer exactly the
// same lookups, so the second one is pure write cost.
func TestSchema_NoIndexDuplicatesAnotherIndexOfItsTable(t *testing.T) {
	scratch, _ := newMigratedScratchDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	rows, err := scratch.Query(ctx, `
		SELECT a.indrelid::regclass::text, ia.relname, ib.relname, a.indisunique, b.indisunique
		  FROM pg_index a
		  JOIN pg_index b ON b.indrelid = a.indrelid AND b.indexrelid > a.indexrelid
		  JOIN pg_class ia ON ia.oid = a.indexrelid
		  JOIN pg_class ib ON ib.oid = b.indexrelid
		  JOIN pg_class t ON t.oid = a.indrelid
		  JOIN pg_namespace n ON n.oid = t.relnamespace AND n.nspname = 'public'
		 WHERE ia.relam = ib.relam
		   AND a.indkey::text = b.indkey::text AND a.indclass::text = b.indclass::text
		   AND a.indcollation::text = b.indcollation::text AND a.indoption::text = b.indoption::text
		   AND COALESCE(pg_get_expr(a.indexprs, a.indrelid), '') = COALESCE(pg_get_expr(b.indexprs, b.indrelid), '')
		   AND COALESCE(pg_get_expr(a.indpred, a.indrelid), '') = COALESCE(pg_get_expr(b.indpred, b.indrelid), '')
		 ORDER BY 1, 2`)
	require.NoError(t, err)
	defer rows.Close()
	var found int
	for rows.Next() {
		var table, first, second string
		var firstUnique, secondUnique bool
		require.NoError(t, rows.Scan(&table, &first, &second, &firstUnique, &secondUnique))
		found++
		plain := second
		if secondUnique && !firstUnique {
			plain = first
		}
		if reason, kept := keptDuplicateIndexes[plain]; kept {
			t.Logf("kept duplicate %s.%s (of %s/%s): %s", table, plain, first, second, reason)
			continue
		}
		t.Errorf("%s: %s duplicates %s (unique %v / %v)", table, first, second, firstUnique, secondUnique)
	}
	require.NoError(t, rows.Err())
	require.Equal(t, len(keptDuplicateIndexes), found, "every kept duplicate is still in the schema")
}

// seedRoomAccess gives every rf- room of seedRoomFeed eight agent members (the first one the
// owner, the last one revoked in every third room), a human member in every third room, and four
// room tokens per agent member: one live, two replaced by rotation, one expired. One agent in 97 is
// deleted. The token hash is md5(agent_id || ':' || n), n = 0 for the live one.
func seedRoomAccess(ctx context.Context, t *testing.T, pool *Pool) {
	t.Helper()
	tx, err := pool.BeginTx(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(ctx) }()
	const listed = `WITH listed AS (SELECT id, row_number() OVER (ORDER BY last_active_at DESC) rn
	                                  FROM rooms WHERE slug LIKE 'rf-%')`
	for _, step := range []struct{ name, sql string }{
		{"replica", `SET LOCAL session_replication_role = replica`},
		{"agents", listed + `
			INSERT INTO agents (id, display_name, deleted_at)
			SELECT 'rf_agent_' || (rn * 8 + k), 'RF agent ' || (rn * 8 + k), CASE WHEN (rn * 8 + k) % 97 = 5 THEN NOW() END
			  FROM listed, generate_series(0, 7) k`},
		{"agent members", listed + `
			INSERT INTO room_members (room_id, agent_id, role, added_by, created_at, revoked_at)
			SELECT id, 'rf_agent_' || (rn * 8 + k), CASE WHEN k = 0 THEN 'owner' ELSE 'member' END, 'rf_agent_' || (rn * 8),
			       NOW() - INTERVAL '30 days' + k * INTERVAL '1 hour', CASE WHEN k = 7 AND rn % 3 = 0 THEN NOW() END
			  FROM listed, generate_series(0, 7) k`},
		{"human members", listed + `,
			people AS (SELECT id, row_number() OVER (ORDER BY username) - 1 AS n FROM users WHERE username LIKE 'rf%')
			INSERT INTO room_members (room_id, user_id, role, added_by, created_at)
			SELECT l.id, p.id, 'member', 'system', NOW() - INTERVAL '20 days'
			  FROM listed l JOIN people p ON p.n = l.rn % 50 WHERE l.rn % 3 = 1`},
		{"tokens", `
			INSERT INTO room_agent_tokens (room_id, agent_id, token_hash, expires_at, created_at, rotated_at)
			SELECT m.room_id, m.agent_id, md5(m.agent_id || ':' || v),
			       CASE WHEN v = 3 THEN NOW() - INTERVAL '1 hour' ELSE NOW() + INTERVAL '7 days' END,
			       NOW() - (4 - v) * INTERVAL '1 day', CASE WHEN v IN (1, 2) THEN NOW() - (3 - v) * INTERVAL '1 day' END
			  FROM room_members m, generate_series(0, 3) v WHERE m.agent_id IS NOT NULL`},
	} {
		_, err := tx.Exec(ctx, step.sql)
		require.NoError(t, err, "seed %s", step.name)
	}
	require.NoError(t, tx.Commit(ctx))
	for _, table := range []string{"agents", "room_members", "room_agent_tokens"} {
		_, err = pool.Exec(ctx, "VACUUM ANALYZE "+table)
		require.NoError(t, err)
	}
}

// md5Hex is the token hash seedRoomAccess stores for a plaintext.
func md5Hex(s string) string {
	sum := md5.Sum([]byte(s))
	return hex.EncodeToString(sum[:])
}

// timelineRead is one repository read and the room entries it has to walk to answer: walked is a
// COUNT over room_entries (nil for a lookup of one row).
type timelineRead struct {
	name   string
	call   func() error
	walked string
	args   []any
}

func TestRoomTimeline_AtGrowthVolumeReplayMembershipAndTokenReadsStayOnTheirIndexes(t *testing.T) {
	scratch, _ := newMigratedScratchDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	seedRoomFeed(ctx, t, scratch)
	seedRoomAccess(ctx, t, scratch)

	var hot, cold uuid.UUID
	require.NoError(t, scratch.QueryRow(ctx, `SELECT id FROM rooms WHERE slug = 'rf-1'`).Scan(&hot))
	require.NoError(t, scratch.QueryRow(ctx, `SELECT id FROM rooms WHERE slug = 'rf-1500'`).Scan(&cold))
	var hotEntries, firstID, midID, midSeq, tailSeq, foreignID int64
	require.NoError(t, scratch.QueryRow(ctx, `
		SELECT COUNT(*), MIN(id), percentile_disc(0.5) WITHIN GROUP (ORDER BY id),
		       percentile_disc(0.5) WITHIN GROUP (ORDER BY sequence), percentile_disc(0.99) WITHIN GROUP (ORDER BY sequence)
		  FROM room_entries WHERE room_id = $1`, hot).Scan(&hotEntries, &firstID, &midID, &midSeq, &tailSeq))
	require.Greater(t, hotEntries, int64(10000), "the hot room holds a long timeline")
	require.NoError(t, scratch.QueryRow(ctx, `
		SELECT MIN(id) FROM room_entries WHERE room_id <> $1 AND id > (SELECT percentile_disc(0.5) WITHIN GROUP (ORDER BY id)
		  FROM room_entries WHERE room_id = $1)`, cold).Scan(&foreignID))
	var human string
	require.NoError(t, scratch.QueryRow(ctx, `SELECT user_id::text FROM room_members WHERE room_id = $1 AND user_id IS NOT NULL`, hot).Scan(&human))

	capture := &statementCapture{}
	traced, err := NewPool(ctx, scratch.Config().ConnString(), WithQueryTracer(capture))
	require.NoError(t, err)
	defer traced.Close()
	entries, messages := NewRoomEntryRepository(traced), NewMessageRepository(traced)
	members, tokens := NewRoomMemberRepository(traced), NewRoomAgentTokenRepository(traced)

	// page checks that a list read returned a full page in the order it promises.
	page := func(n, limit int, err error) error {
		if err == nil && n != limit {
			t.Errorf("page holds %d entries, want %d", n, limit)
		}
		return err
	}
	truth := func(want bool) func(bool, error) error {
		return func(got bool, err error) error {
			if err == nil && got != want {
				t.Errorf("got %v, want %v", got, want)
			}
			return err
		}
	}
	var last int64
	reads := []timelineRead{
		{"replay from the start", func() error {
			got, err := entries.ListPage(ctx, models.RoomEntryPageParams{RoomID: hot, Limit: 100})
			if len(got) > 0 {
				last = int64(got[len(got)-1].Sequence)
			}
			return page(len(got), 100, err)
		}, `SELECT COUNT(*) FROM room_entries WHERE room_id = $1 AND sequence <= $2`, []any{hot, &last}},
		{"replay from the middle", func() error {
			got, err := entries.ListPage(ctx, models.RoomEntryPageParams{RoomID: hot, AfterSequence: int(midSeq), Limit: 100})
			if len(got) > 0 {
				last = int64(got[len(got)-1].Sequence)
			}
			return page(len(got), 100, err)
		}, `SELECT COUNT(*) FROM room_entries WHERE room_id = $1 AND sequence > $3 AND sequence <= $2`, []any{hot, &last, midSeq}},
		{"replay the tail", func() error {
			got, err := entries.ListPage(ctx, models.RoomEntryPageParams{RoomID: hot, AfterSequence: int(tailSeq), Limit: 500})
			if err == nil && len(got) == 0 {
				t.Error("the tail is not empty")
			}
			return err
		}, `SELECT COUNT(*) FROM room_entries WHERE room_id = $1 AND sequence > $2`, []any{hot, tailSeq}},
		{"replay events only", func() error {
			got, err := entries.ListPage(ctx, models.RoomEntryPageParams{RoomID: hot, Kind: "event", Limit: 100})
			if len(got) > 0 {
				last = int64(got[len(got)-1].Sequence)
			}
			return page(len(got), 100, err)
		}, `SELECT COUNT(*) FROM room_entries WHERE room_id = $1 AND sequence <= $2`, []any{hot, &last}},
		{"replay one issue's claims", func() error {
			got, err := entries.ListPage(ctx, models.RoomEntryPageParams{RoomID: hot, Kind: "event", EventType: "CLAIM", Issue: "issue-6", Limit: 20})
			if err == nil && len(got) == 0 {
				t.Error("the issue has claims")
			}
			return err
		}, `SELECT COUNT(*) FROM room_entries WHERE room_id = $1 AND kind = 'event' AND issue = 'issue-6'`, []any{hot}},
		{"resume cursor of an entry of the room", func() error {
			_, err := entries.SequenceAfterEntry(ctx, hot, midID)
			return err
		}, "", nil},
		{"resume cursor of an id the room never held", func() error {
			_, err := entries.SequenceAfterEntry(ctx, cold, foreignID)
			return err
		}, `SELECT COUNT(*) FROM room_entries WHERE room_id = $1 AND id >= $2`, []any{cold, foreignID}},
		{"latest sequence", func() error { _, err := entries.MaxSequence(ctx, hot); return err }, "", nil},
		{"recent transcript", func() error {
			got, err := entries.ListRecent(ctx, hot, 100)
			if len(got) > 0 {
				last = int64(got[0].Sequence)
			}
			return page(len(got), 100, err)
		}, `SELECT COUNT(*) FROM room_entries WHERE room_id = $1 AND sequence >= $2`, []any{hot, &last}},
		{"events of one issue", func() error {
			got, err := entries.QueryEvents(ctx, models.QueryRoomEntryEventsParams{RoomID: hot, Issue: "issue-5", Limit: 100})
			if err == nil && len(got) == 0 {
				t.Error("the issue has events")
			}
			return err
		}, `SELECT COUNT(*) FROM room_entries WHERE room_id = $1 AND kind = 'event' AND issue = 'issue-5'`, []any{hot}},
		{"one entry", func() error { _, err := entries.GetByID(ctx, hot, midID); return err }, "", nil},
		{"messages after a cursor", func() error {
			got, err := messages.ListAfter(ctx, hot, firstID, 100)
			if len(got) > 0 {
				last = got[len(got)-1].ID
			}
			return page(len(got), 100, err)
		}, `SELECT COUNT(*) FROM room_entries WHERE room_id = $1 AND id > $3 AND id <= $2`, []any{hot, &last, firstID}},
		{"messages before a cursor", func() error {
			got, err := messages.ListBefore(ctx, hot, midID, 100)
			if len(got) > 0 {
				last = got[0].ID
			}
			return page(len(got), 100, err)
		}, `SELECT COUNT(*) FROM room_entries WHERE room_id = $1 AND id >= $2 AND id < $3`, []any{hot, &last, midID}},
		{"agent membership", func() error { return truth(true)(members.IsMember(ctx, hot, "rf_agent_9")) }, "", nil},
		{"human membership", func() error { return truth(true)(members.IsUserMember(ctx, hot, human)) }, "", nil},
		{"member list", func() error {
			got, err := members.ListByRoom(ctx, hot)
			return page(len(got), 8, err)
		}, "", nil},
		{"token resolution", func() error {
			id, err := tokens.ResolveByHash(ctx, md5Hex("rf_agent_9:0"))
			if err == nil && (id.RoomID != hot || id.AgentID != "rf_agent_9") {
				t.Errorf("token resolved to %v", id)
			}
			return err
		}, "", nil},
		{"live token check", func() error { return truth(true)(tokens.IsLive(ctx, md5Hex("rf_agent_9:0"), hot)) }, "", nil},
		{"rotated token check", func() error { return truth(true)(tokens.WasRotated(ctx, md5Hex("rf_agent_9:1"))) }, "", nil},
	}

	for _, rd := range reads {
		t.Run(rd.name, func(t *testing.T) {
			capture.take()
			require.NoError(t, rd.call())
			walked := int64(0)
			if rd.walked != "" {
				args := make([]any, len(rd.args))
				for i, a := range rd.args {
					if p, ok := a.(*int64); ok {
						a = *p
					}
					args[i] = a
				}
				require.NoError(t, scratch.QueryRow(ctx, rd.walked, args...).Scan(&walked))
			}
			// Every walked entry may cost a heap page (a hot room's entries sit one per page), plus
			// the index pages that lead to them.
			bound := int(walked + walked/50 + 30)
			stmts := capture.take()
			require.NotEmpty(t, stmts)
			for i, s := range stmts {
				buffers, seq := tableReads(ctx, t, scratch, s)
				read := 0
				for table, n := range buffers {
					require.False(t, seq[table], "statement %d scans %s", i, table)
					read += n
				}
				t.Logf("statement %d: %d buffers %v (walks %d entries, bound %d)", i, read, buffers, walked, bound)
				if !strings.HasPrefix(strings.TrimSpace(s.sql), "SELECT") {
					continue // a write (token resolution stamps last_used_at): the bound is about reads
				}
				require.LessOrEqual(t, read, bound, "statement %d reads more than the entries it walks", i)
			}
		})
	}
}
