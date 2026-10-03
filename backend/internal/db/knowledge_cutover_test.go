package db

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// Task idx 93: the knowledge cutover runs as one ordered, ledgered sequence. The rehearsal on
// the restored production dump established the sequence (post states, contributions,
// relations, then the vote-score and room-activity rebuilds) and that a second run must
// change nothing, so a retry after an interruption is safe.

type cutoverSeed struct {
	question, answer, problem, approach, note, idea, room string
}

func seedCutoverLegacy(t *testing.T, pool *Pool) cutoverSeed {
	t.Helper()
	ctx := context.Background()
	id := func(sql string, args ...any) string {
		t.Helper()
		var v string
		require.NoError(t, pool.QueryRow(ctx, sql, args...).Scan(&v), sql)
		return v
	}
	var s cutoverSeed
	authorAgent(ctx, t, pool, "kc-agent") // the legacy posts name an existing author (000117)
	s.question = id(`INSERT INTO posts (type, title, description, posted_by_type, posted_by_id, status)
		VALUES ('question', 'cutover question', 'body', 'agent', 'kc-agent', 'open') RETURNING id::text`)
	// A stored upvote no vote row backs: the rebuild must repair it (rehearsal: 2 such answers).
	s.answer = id(`INSERT INTO answers (question_id, author_type, author_id, content, upvotes)
		VALUES ($1, 'agent', 'kc-agent', 'legacy answer', 1) RETURNING id::text`, s.question)
	s.problem = id(`INSERT INTO posts (type, title, description, posted_by_type, posted_by_id, status)
		VALUES ('problem', 'cutover problem', 'body', 'agent', 'kc-agent', 'open') RETURNING id::text`)
	s.approach = id(`INSERT INTO approaches (problem_id, author_type, author_id, angle)
		VALUES ($1, 'agent', 'kc-agent', 'an angle') RETURNING id::text`, s.problem)
	s.note = id(`INSERT INTO progress_notes (approach_id, content) VALUES ($1, 'a note') RETURNING id::text`, s.approach)
	s.idea = id(`INSERT INTO posts (type, title, description, posted_by_type, posted_by_id, status)
		VALUES ('idea', 'cutover idea', 'body', 'agent', 'kc-agent', 'open') RETURNING id::text`)
	id(`INSERT INTO comments (target_type, target_id, author_type, author_id, content)
		VALUES ('post', $1, 'system', 'solvr-moderator', 'Post approved') RETURNING id::text`, s.idea)
	// A stored message count the timeline does not back (rehearsal: 57 rooms drifted).
	s.room = id(`INSERT INTO rooms (slug, display_name, message_count) VALUES ('kc-room', 'kc room', 5)
		RETURNING id::text`)
	return s
}

func TestKnowledgeCutover_AppliesTheWholeSequenceAndASecondRunChangesNothing(t *testing.T) {
	pool, _ := newPreArchiveScratchDatabase(t)
	ctx := context.Background()
	s := seedCutoverLegacy(t, pool)

	rep, err := RunKnowledgeCutover(ctx, pool, KnowledgeCutoverOptions{})
	require.NoError(t, err)
	require.False(t, rep.DryRun)
	require.Zero(t, rep.PostExceptions)
	require.Equal(t, 3, rep.PendingContributions, "answer, approach and comment wait for a reply")
	require.EqualValues(t, 3, rep.RepliesCreated)
	require.EqualValues(t, 1, rep.ProgressNotes)
	require.EqualValues(t, 1, rep.VoteDriftBefore)
	require.EqualValues(t, 1, rep.VoteScoresRebuilt)
	require.Zero(t, rep.VoteDriftAfter)
	require.EqualValues(t, 1, rep.RoomDriftBefore)
	require.EqualValues(t, 1, rep.RoomActivityRebuilt)
	require.Zero(t, rep.RoomDriftAfter)
	require.Zero(t, rep.PendingAfter)

	require.Equal(t, 0, countRows(t, pool, ctx, `SELECT upvotes FROM replies WHERE legacy_type = 'answer' AND legacy_id = $1`, s.answer))
	require.Equal(t, 0, countRows(t, pool, ctx, `SELECT message_count FROM rooms WHERE id = $1`, s.room))

	// Every step is in the ledger under this run, finished and without an error.
	require.NotEmpty(t, rep.Steps)
	require.Equal(t, len(rep.Steps), countRows(t, pool, ctx, `SELECT count(*) FROM cutover_ledger
		WHERE run_id = $1 AND finished_at IS NOT NULL AND error IS NULL`, rep.RunID))

	again, err := RunKnowledgeCutover(ctx, pool, KnowledgeCutoverOptions{})
	require.NoError(t, err)
	require.NotEqual(t, rep.RunID, again.RunID)
	require.Zero(t, again.PendingContributions)
	require.Zero(t, again.RepliesCreated)
	require.Zero(t, again.ProgressNotes)
	require.Zero(t, again.PostStatesRemapped)
	require.Zero(t, again.VoteScoresRebuilt)
	require.Zero(t, again.RoomActivityRebuilt)
	require.Equal(t, 4, countRows(t, pool, ctx, `SELECT count(*) FROM replies`), "3 contributions + 1 progress note, once")
}

func TestKnowledgeCutover_DryRunReportsWithoutWriting(t *testing.T) {
	pool, _ := newPreArchiveScratchDatabase(t)
	ctx := context.Background()
	s := seedCutoverLegacy(t, pool)

	rep, err := RunKnowledgeCutover(ctx, pool, KnowledgeCutoverOptions{DryRun: true})
	require.NoError(t, err)
	require.True(t, rep.DryRun)
	require.Equal(t, 3, rep.PendingContributions)
	require.EqualValues(t, 1, rep.RoomDriftBefore)
	require.Zero(t, rep.RepliesCreated)

	require.Equal(t, 0, countRows(t, pool, ctx, `SELECT count(*) FROM replies`))
	require.Equal(t, 5, countRows(t, pool, ctx, `SELECT message_count FROM rooms WHERE id = $1`, s.room))
	var ledger *string
	require.NoError(t, pool.QueryRow(ctx, `SELECT to_regclass('cutover_ledger')::text`).Scan(&ledger))
	require.Nil(t, ledger, "a dry run writes nothing, not even the ledger")
}

func TestKnowledgeCutover_StopsBeforeConvertingWhenAPostCannotBeVerified(t *testing.T) {
	pool, _ := newPreArchiveScratchDatabase(t)
	ctx := context.Background()
	authorAgent(ctx, t, pool, "kc-agent")
	seedCutoverLegacy(t, pool)
	_, err := pool.Exec(ctx, `INSERT INTO posts (type, title, description, posted_by_type, posted_by_id, status)
		VALUES ('idea', '  ', 'untitled', 'agent', 'kc-agent', 'open')`)
	require.NoError(t, err)

	rep, err := RunKnowledgeCutover(ctx, pool, KnowledgeCutoverOptions{})
	require.Error(t, err)
	require.Contains(t, err.Error(), PostExceptionMissingTitle)
	require.Equal(t, 1, rep.PostExceptions)
	require.Equal(t, 0, countRows(t, pool, ctx, `SELECT count(*) FROM replies`), "nothing is converted")
	require.Equal(t, 1, countRows(t, pool, ctx, `SELECT count(*) FROM cutover_ledger
		WHERE run_id = $1 AND error LIKE '%missing_title%'`, rep.RunID))
}

func TestSchemaVersion_ReadsTheMigrateVersionTable(t *testing.T) {
	pool, _ := newMigratedScratchDatabase(t)
	ctx := context.Background()

	_, _, err := SchemaVersion(ctx, pool)
	require.Error(t, err, "no schema_migrations table: production had none before the cutover")

	_, err = pool.Exec(ctx, `CREATE TABLE schema_migrations (version bigint NOT NULL PRIMARY KEY, dirty boolean NOT NULL);
		INSERT INTO schema_migrations VALUES (113, true)`)
	require.NoError(t, err)
	version, dirty, err := SchemaVersion(ctx, pool)
	require.NoError(t, err)
	require.EqualValues(t, 113, version)
	require.True(t, dirty)
}

// Once the legacy archive migration has run the cutover has nothing live to convert: it
// refuses, dry run included, and names the archive, instead of failing on a missing table
// half way through or reporting zeros (idx 68).
func TestKnowledgeCutover_RefusesOnceTheLegacyTablesAreArchived(t *testing.T) {
	pool, _ := newMigratedScratchDatabase(t)
	ctx := context.Background()
	for _, dryRun := range []bool{true, false} {
		_, err := RunKnowledgeCutover(ctx, pool, KnowledgeCutoverOptions{DryRun: dryRun})
		require.Error(t, err, "dry run %v", dryRun)
		require.ErrorIs(t, err, ErrLegacyTablesArchived, "dry run %v", dryRun)
		require.Contains(t, err.Error(), "the cutover runs only below the legacy archive migration")
	}
	var ledger *string
	require.NoError(t, pool.QueryRow(ctx, `SELECT to_regclass('cutover_ledger')::text`).Scan(&ledger))
	require.Nil(t, ledger, "a refused run writes nothing, not even the ledger")
}
