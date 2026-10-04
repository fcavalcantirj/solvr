package db

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// Task idx 93 step 4 (rollback that accounts for writes created after cutover). The cutover
// rehearsal on the restored production dump showed the down path aborting on the first row
// the new model can write and the old schema cannot hold: 000089.down re-adds the vote and
// report target checks without 'reply', and 000088.down re-adds the typed-only posts check
// although new posts default to the canonical type 'post'. Every down migration from the
// legacy archive (000138) to 85 must now run over such rows, and each row the old schema cannot
// hold must be archived in rollback_archive rather than dropped silently.
func TestCutoverRollback_DownPathKeepsOrArchivesEveryPostCutoverWrite(t *testing.T) {
	pool, archiveLegacy := newPreArchiveScratchDatabase(t)
	ctx := context.Background()

	exec := func(sql string, args ...any) {
		t.Helper()
		_, err := pool.Exec(ctx, sql, args...)
		require.NoError(t, err, sql)
	}
	id := func(sql string, args ...any) string {
		t.Helper()
		var v string
		require.NoError(t, pool.QueryRow(ctx, sql, args...).Scan(&v), sql)
		return v
	}

	// A post (000117) and a native reply (000116) name an existing author.
	exec(`INSERT INTO agents (id, display_name, status) VALUES ('rb-agent-1', 'Rollback Agent 1', 'active')`)
	exec(`INSERT INTO agents (id, display_name, status) VALUES ('rb-agent-2', 'Rollback Agent 2', 'active')`)

	// Legacy content, migrated the way the cutover does it.
	question := id(`INSERT INTO posts (type, title, description, posted_by_type, posted_by_id, status)
		VALUES ('question', 'rollback question', 'body', 'agent', 'rb-agent-1', 'open') RETURNING id::text`)
	answer := id(`INSERT INTO answers (question_id, author_type, author_id, content)
		VALUES ($1, 'agent', 'rb-agent-1', 'legacy answer') RETURNING id::text`, question)
	problem := id(`INSERT INTO posts (type, title, description, posted_by_type, posted_by_id, status)
		VALUES ('problem', 'rollback problem', 'body', 'agent', 'rb-agent-1', 'open') RETURNING id::text`)
	approach := id(`INSERT INTO approaches (problem_id, author_type, author_id, angle)
		VALUES ($1, 'agent', 'rb-agent-1', 'legacy angle') RETURNING id::text`, problem)
	note := id(`INSERT INTO progress_notes (approach_id, content) VALUES ($1, 'legacy note') RETURNING id::text`, approach)
	notify := func(link string) string {
		t.Helper()
		return id(`INSERT INTO notifications (agent_id, type, title, link) VALUES ('rb-agent-1', 'reply', 'n', $1)
			RETURNING id::text`, link)
	}
	nQuestion := notify("/questions/" + question)
	nProblemQuery := notify("/problems/" + problem + "?tab=approaches")
	nAnswerAnchor := notify("/questions/" + question + "#answer-" + answer)
	nRoom := notify("/rooms/some-room")
	_, err := MigrateContributions(ctx, pool)
	require.NoError(t, err)
	_, err = RemapLegacyRelations(ctx, pool)
	require.NoError(t, err)
	answerReply := id(`SELECT id::text FROM replies WHERE legacy_type = 'answer' AND legacy_id = $1`, answer)
	noteReply := id(`SELECT id::text FROM replies WHERE legacy_type = 'progress_note' AND legacy_id = $1`, note)

	// Writes that only exist after cutover.
	canonical := id(`INSERT INTO posts (type, title, description, posted_by_type, posted_by_id, status)
		VALUES ('post', 'canonical post', 'untyped body', 'agent', 'rb-agent-1', 'open') RETURNING id::text`)
	native := id(`INSERT INTO replies (post_id, author_type, author_id, body)
		VALUES ($1, 'agent', 'rb-agent-2', 'native reply') RETURNING id::text`, question)
	child := id(`INSERT INTO replies (post_id, parent_reply_id, author_type, author_id, body)
		VALUES ($1, $2, 'agent', 'rb-agent-1', 'native child') RETURNING id::text`, question, native)
	// A reply to a migrated progress note: 000109.down deletes the note's derived copy, and
	// the cascade would take this reply with it.
	underNote := id(`INSERT INTO replies (post_id, parent_reply_id, author_type, author_id, body)
		VALUES ($1, $2, 'agent', 'rb-agent-2', 'reply under a note') RETURNING id::text`, problem, noteReply)
	nCanonical := notify("/posts/" + canonical)
	nNativeAnchor := notify("/posts/" + question + "#" + native)
	for _, target := range []string{native, answerReply} {
		exec(`INSERT INTO votes (target_type, target_id, voter_type, voter_id, direction, confirmed)
			VALUES ('reply', $1, 'agent', 'rb-agent-3', 'up', true)`, target)
	}
	exec(`INSERT INTO reports (target_type, target_id, reporter_type, reporter_id, reason, status)
		VALUES ('reply', $1, 'agent', 'rb-agent-3', 'spam', 'pending')`, child)
	for _, target := range []string{native, answerReply} {
		exec(`INSERT INTO flags (target_type, target_id, reporter_type, reporter_id, reason, status)
			VALUES ('reply', $1, 'agent', 'rb-agent-3', 'spam', 'pending')`, target)
	}

	// Anti-abuse (000114): a ban row and a flag reason the old CHECK rejects are archived too.
	exec(`INSERT INTO banned_identities (kind, value, reason) VALUES ('agent_id', 'rb-banned-agent', 'rollback test')`)
	exec(`INSERT INTO flags (target_type, target_id, reporter_type, reporter_id, reason, status)
		VALUES ('answer', $1, 'system', 'content-moderation', 'moderation_rejected', 'pending')`, answer)

	// The legacy archive (idx 68) runs last, as on production; the rollback passes through its
	// down migration, which restores the legacy tables, the posts' legacy fields and the
	// legacy-target flag exactly, before the rest of the chain runs as it did without it.
	archiveLegacy()
	require.Equal(t, 0, countRows(t, pool, ctx, `SELECT count(*) FROM pg_class
		WHERE relnamespace = 'public'::regnamespace AND relname = 'answers'`), "archived before the rollback")

	applied := migrateDownTo84(ctx, t, pool)
	require.Equal(t, 57, applied, "down migrations 000141..000085")

	var replies *string
	require.NoError(t, pool.QueryRow(ctx, `SELECT to_regclass('replies')::text`).Scan(&replies))
	require.Nil(t, replies, "the replies table is gone at 000084")

	// What the old schema cannot hold is archived, one row each, never silently dropped.
	archived := map[string]int{}
	rows, err := pool.Query(ctx, `SELECT source_table, count(*) FROM rollback_archive GROUP BY source_table`)
	require.NoError(t, err)
	for rows.Next() {
		var table string
		var n int
		require.NoError(t, rows.Scan(&table, &n))
		archived[table] = n
	}
	rows.Close()
	require.Equal(t, map[string]int{"votes": 2, "reports": 1, "replies": 3, "flags": 2, "posts": 1, "banned_identities": 1}, archived)
	require.Equal(t, 1, countRows(t, pool, ctx, `SELECT count(*) FROM rollback_archive
		WHERE source_table = 'flags' AND row_data->>'reason' = 'moderation_rejected'`))

	// The emergency ban trigger outlives the rollback, back on its original body (000114.down).
	require.Equal(t, 1, countRows(t, pool, ctx, `SELECT count(*) FROM pg_trigger WHERE tgname = 'users_refuse_tombstoned_email'`))
	require.Equal(t, 0, countRows(t, pool, ctx, `SELECT count(*) FROM pg_proc
		WHERE proname = 'users_refuse_tombstoned_email' AND prosrc LIKE '%banned_identities%'`))
	require.Equal(t, 1, countRows(t, pool, ctx, `SELECT count(*) FROM rollback_archive
		WHERE source_table = 'replies' AND row_data->>'id' = $1 AND row_data->>'body' = 'native reply'`, native))
	require.Equal(t, 1, countRows(t, pool, ctx, `SELECT count(*) FROM rollback_archive
		WHERE source_table = 'flags' AND row_data->>'target_id' = $1`, native))
	require.Equal(t, 1, countRows(t, pool, ctx, `SELECT count(*) FROM rollback_archive
		WHERE source_table = 'replies' AND row_data->>'id' = $1 AND row_data->>'body' = 'reply under a note'`, underNote))
	require.Equal(t, 1, countRows(t, pool, ctx, `SELECT count(*) FROM progress_notes WHERE id = $1 AND content = 'legacy note'`, note))

	// The canonical post keeps its content under a type the old schema accepts, and the
	// archive records the type it had.
	require.Equal(t, "idea", id(`SELECT type FROM posts WHERE id = $1`, canonical))
	require.Equal(t, 1, countRows(t, pool, ctx, `SELECT count(*) FROM rollback_archive
		WHERE source_table = 'posts' AND row_data->>'id' = $1 AND row_data->>'type' = 'post'`, canonical))

	// Notification links the cutover rewrote to /posts (and links written after it) point at
	// the legacy pages again: the old frontend has no /posts route. A legacy anchor comes
	// back; an anchor on a reply the old schema cannot hold is dropped with that reply.
	for nid, want := range map[string]string{
		nQuestion:     "/questions/" + question,
		nProblemQuery: "/problems/" + problem + "?tab=approaches",
		nAnswerAnchor: "/questions/" + question + "#answer-" + answer,
		nRoom:         "/rooms/some-room",
		nCanonical:    "/ideas/" + canonical,
		nNativeAnchor: "/questions/" + question,
	} {
		require.Equal(t, want, id(`SELECT link FROM notifications WHERE id = $1`, nid))
	}

	// The flag on the migrated reply returns to its legacy answer (000109.down), and the
	// legacy rows themselves are untouched.
	require.Equal(t, 1, countRows(t, pool, ctx, `SELECT count(*) FROM flags WHERE target_type = 'answer' AND target_id = $1`, answer))
	require.Equal(t, 1, countRows(t, pool, ctx, `SELECT count(*) FROM answers WHERE id = $1 AND content = 'legacy answer'`, answer))
	require.Equal(t, "question", id(`SELECT type FROM posts WHERE id = $1`, question))
}

// migrateDownTo84 runs every down migration above 000084, newest first, as `migrate down`
// would, and returns how many it applied. On a database that has golang-migrate's
// schema_migrations table (production after `migrate force 84` and `up`), each step is
// recorded the way migrate records it: the version below the file marked dirty before the
// file runs, and clean after it.
func migrateDownTo84(ctx context.Context, t *testing.T, pool *Pool) int {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(backendRoot(t), "migrations", "*.down.sql"))
	require.NoError(t, err)
	sort.Sort(sort.Reverse(sort.StringSlice(files)))
	var down []string
	for _, f := range files {
		if filepath.Base(f) < "000085" {
			break
		}
		down = append(down, f)
	}
	var tracked bool
	require.NoError(t, pool.QueryRow(ctx, `SELECT to_regclass('schema_migrations') IS NOT NULL`).Scan(&tracked))
	for i, f := range down {
		below := int64(84)
		if i+1 < len(down) {
			prefix, _, _ := strings.Cut(filepath.Base(down[i+1]), "_")
			below, err = strconv.ParseInt(prefix, 10, 64)
			require.NoError(t, err, "version of %s", filepath.Base(down[i+1]))
		}
		if tracked {
			_, err = pool.Exec(ctx, `UPDATE schema_migrations SET version = $1, dirty = true`, below)
			require.NoError(t, err)
		}
		sql, err := os.ReadFile(f)
		require.NoError(t, err)
		_, err = pool.Exec(ctx, string(sql))
		require.NoError(t, err, "apply %s", filepath.Base(f))
		if tracked {
			_, err = pool.Exec(ctx, `UPDATE schema_migrations SET dirty = false`)
			require.NoError(t, err)
		}
	}
	return len(down)
}
