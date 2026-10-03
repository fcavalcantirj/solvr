package db

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// Task idx 68 step 5: the legacy archive migration moves every legacy row out of live storage
// (never just drops it) with a manifest, narrows posts and the target checks, and its down
// migration restores exactly what it moved.

// legacyArchiveDigestSQL is legacy_archive.digest's expression over any relation.
const legacyArchiveDigestSQL = `SELECT count(*), encode(sha256(convert_to(coalesce(string_agg(t::text, E'\n' ORDER BY t::text COLLATE "C"), ''), 'UTF8')), 'hex') FROM %s t`

type tableDigest struct {
	Rows   int64
	SHA256 string
}

func digestOf(t *testing.T, pool *Pool, relation string) tableDigest {
	t.Helper()
	var d tableDigest
	require.NoError(t, pool.QueryRow(context.Background(), fmt.Sprintf(legacyArchiveDigestSQL, relation)).Scan(&d.Rows, &d.SHA256), relation)
	return d
}

// legacyArchiveState is everything the archive migration moves or narrows, read in a form
// that does not depend on column order or schema.
type legacyArchiveState struct {
	Tables      map[string]tableDigest
	PostFields  tableDigest
	PostStamps  tableDigest
	Targets     map[string]tableDigest
	Constraints map[string]string
	Indexes     map[string]string
	Functions   map[string]string
}

func readLegacyArchiveState(t *testing.T, pool *Pool, schema string) legacyArchiveState {
	t.Helper()
	ctx := context.Background()
	s := legacyArchiveState{Tables: map[string]tableDigest{}, Targets: map[string]tableDigest{},
		Constraints: map[string]string{}, Indexes: map[string]string{}, Functions: map[string]string{}}
	for _, table := range LegacyTables {
		s.Tables[table] = digestOf(t, pool, schema+"."+table)
	}
	s.PostFields = digestOf(t, pool, `(SELECT id, type, status, success_criteria, weight, accepted_answer_id, evolved_into FROM posts)`)
	s.PostStamps = digestOf(t, pool, `(SELECT id, created_at, updated_at, upvotes, downvotes FROM posts)`)
	s.Targets["votes"] = digestOf(t, pool, "votes")
	s.Targets["reports"] = digestOf(t, pool, "reports")
	s.Targets["flags"] = digestOf(t, pool, "flags")
	relations := []string{"public.posts", "public.votes", "public.reports", "public.flags"}
	for _, table := range LegacyTables {
		relations = append(relations, schema+"."+table)
	}
	rows, err := pool.Query(ctx, `SELECT c.relname || '.' || con.conname, pg_get_constraintdef(con.oid)
		FROM pg_constraint con JOIN pg_class c ON c.oid = con.conrelid
		WHERE con.conrelid = ANY (SELECT to_regclass(r) FROM unnest($1::text[]) r) AND con.contype IN ('c', 'f', 'u', 'p')`,
		relations)
	require.NoError(t, err)
	for rows.Next() {
		var name, def string
		require.NoError(t, rows.Scan(&name, &def))
		s.Constraints[name] = def
	}
	require.NoError(t, rows.Err())
	rows, err = pool.Query(ctx, `SELECT indexname, indexdef FROM pg_indexes
		WHERE (schemaname = $1 AND tablename = ANY ($2)) OR (schemaname = 'public' AND tablename IN ('posts', 'votes', 'reports', 'flags'))`,
		schema, LegacyTables)
	require.NoError(t, err)
	for rows.Next() {
		var name, def string
		require.NoError(t, rows.Scan(&name, &def))
		s.Indexes[name] = def
	}
	require.NoError(t, rows.Err())
	rows, err = pool.Query(ctx, `SELECT proname, pg_get_functiondef(oid) FROM pg_proc
		WHERE pronamespace = 'public'::regnamespace AND proname IN ('hybrid_search_answers', 'hybrid_search_approaches')`)
	require.NoError(t, err)
	for rows.Next() {
		var name, def string
		require.NoError(t, rows.Scan(&name, &def))
		s.Functions[name] = def
	}
	require.NoError(t, rows.Err())
	return s
}

type legacyArchiveSeed struct {
	cutoverSeed
	problemSolved, questionAnswered, ideaEvolved, approach2 string
}

// seedLegacyArchive seeds every shape the archive migration moves: the cutover fixture plus
// posts carrying every problem-only field and a retired status, an approach relationship, an
// orphan comment, and a vote, report and flag on legacy targets the cutover cannot retarget.
func seedLegacyArchive(t *testing.T, pool *Pool) legacyArchiveSeed {
	t.Helper()
	ctx := context.Background()
	id := func(sql string, args ...any) string {
		t.Helper()
		var v string
		require.NoError(t, pool.QueryRow(ctx, sql, args...).Scan(&v), sql)
		return v
	}
	s := legacyArchiveSeed{cutoverSeed: seedCutoverLegacy(t, pool)}
	s.problemSolved = id(`INSERT INTO posts (type, title, description, posted_by_type, posted_by_id, status, success_criteria, weight)
		VALUES ('problem', 'archived problem', 'body', 'agent', 'kc-agent', 'solved', ARRAY['it works', 'no regressions'], 4)
		RETURNING id::text`)
	s.approach2 = id(`INSERT INTO approaches (problem_id, author_type, author_id, angle, status)
		VALUES ($1, 'agent', 'kc-agent', 'the angle that worked', 'succeeded') RETURNING id::text`, s.problemSolved)
	id(`INSERT INTO approach_relationships (from_approach_id, to_approach_id, relation_type)
		VALUES ($1, $2, 'updates') RETURNING id::text`, s.approach2, s.approach)
	s.questionAnswered = id(`INSERT INTO posts (type, title, description, posted_by_type, posted_by_id, status, accepted_answer_id)
		VALUES ('question', 'archived question', 'body', 'agent', 'kc-agent', 'answered', $1) RETURNING id::text`, s.answer)
	s.ideaEvolved = id(`INSERT INTO posts (type, title, description, posted_by_type, posted_by_id, status, evolved_into)
		VALUES ('idea', 'archived idea', 'body', 'agent', 'kc-agent', 'evolved', ARRAY[$1::uuid]) RETURNING id::text`, s.problemSolved)
	id(`INSERT INTO comments (target_type, target_id, author_type, author_id, content)
		VALUES ('approach', gen_random_uuid(), 'system', 'solvr-moderator', 'orphan: its approach is gone') RETURNING id::text`)
	id(`INSERT INTO votes (target_type, target_id, voter_type, voter_id, direction, confirmed)
		VALUES ('answer', gen_random_uuid(), 'agent', 'kc-agent', 'up', true) RETURNING id::text`)
	id(`INSERT INTO reports (target_type, target_id, reporter_type, reporter_id, reason)
		VALUES ('comment', gen_random_uuid(), 'agent', 'kc-agent', 'spam') RETURNING id::text`)
	id(`INSERT INTO flags (target_type, target_id, reporter_type, reporter_id, reason)
		VALUES ('approach', gen_random_uuid(), 'agent', 'kc-agent', 'spam') RETURNING id::text`)
	return s
}

func legacyArchiveDownSQL(t *testing.T) string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(backendRoot(t), "migrations", "*_legacy_archive.down.sql"))
	require.NoError(t, err)
	require.Len(t, files, 1)
	sql, err := os.ReadFile(files[0])
	require.NoError(t, err)
	return string(sql)
}

func TestLegacyArchiveMigration_MovesEveryLegacyRowAndTheDownRestoresItExactly(t *testing.T) {
	pool, archiveLegacy := newPreArchiveScratchDatabase(t)
	ctx := context.Background()
	s := seedLegacyArchive(t, pool)
	_, err := RunKnowledgeCutover(ctx, pool, KnowledgeCutoverOptions{})
	require.NoError(t, err)
	before := readLegacyArchiveState(t, pool, "public")
	require.Len(t, before.Functions, 2)
	require.Contains(t, before.Constraints, "approaches.approaches_problem_id_fkey", "the snapshot reads the legacy foreign keys")
	require.Contains(t, before.Constraints, "progress_notes.progress_notes_approach_id_fkey")
	require.Contains(t, before.Indexes, "idx_approaches_embedding", "the snapshot reads the legacy indexes")

	archiveLegacy()

	// Up: the archive holds every legacy row, its manifest agrees with the live tables before
	// the move, and nothing legacy is left in public.
	var publicLegacy int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM pg_class
		WHERE relnamespace = 'public'::regnamespace AND relname = ANY ($1)`, LegacyTables).Scan(&publicLegacy))
	assert.Zero(t, publicLegacy, "no legacy table is left in public")
	manifest := map[string]tableDigest{}
	rows, err := pool.Query(ctx, `SELECT table_name, row_count, sha256 FROM legacy_archive.manifest`)
	require.NoError(t, err)
	for rows.Next() {
		var name string
		var d tableDigest
		require.NoError(t, rows.Scan(&name, &d.Rows, &d.SHA256))
		manifest[name] = d
	}
	require.NoError(t, rows.Err())
	for _, table := range LegacyTables {
		assert.Equal(t, before.Tables[table], manifest[table], "manifest of %s equals the live table before the move", table)
		assert.Equal(t, before.Tables[table], digestOf(t, pool, "legacy_archive."+table), "archived %s is byte-exact", table)
	}
	assert.EqualValues(t, 3, before.Tables["approaches"].Rows+before.Tables["approach_relationships"].Rows,
		"the seed reaches the archive (2 approaches, 1 relationship)")
	assert.EqualValues(t, 1, manifest["votes"].Rows, "the legacy-target vote")
	assert.EqualValues(t, 1, manifest["reports"].Rows, "the legacy-target report")
	assert.EqualValues(t, 1, manifest["flags"].Rows, "the legacy-target flag")
	assert.Equal(t, 0, countRows(t, pool, ctx, `SELECT count(*) FROM votes WHERE target_type NOT IN ('post', 'blog_post', 'reply')`))
	assert.Equal(t, 0, countRows(t, pool, ctx, `SELECT count(*) FROM pg_proc WHERE pronamespace = 'public'::regnamespace
		AND proname IN ('hybrid_search_answers', 'hybrid_search_approaches')`))

	// Posts: every post is type post, the retired statuses read open, the problem-only columns
	// are gone and their values are in post_fields. No timestamp or score moved.
	assert.Equal(t, 0, countRows(t, pool, ctx, `SELECT count(*) FROM posts WHERE type <> 'post'`))
	assert.Equal(t, 0, countRows(t, pool, ctx, `SELECT count(*) FROM posts WHERE status NOT IN ('draft', 'open', 'closed', 'stale', 'pending_review', 'rejected')`))
	for _, p := range []string{s.problemSolved, s.questionAnswered, s.ideaEvolved} {
		var status string
		require.NoError(t, pool.QueryRow(ctx, `SELECT status FROM posts WHERE id = $1`, p).Scan(&status))
		assert.Equal(t, "open", status)
	}
	assert.Equal(t, 0, countRows(t, pool, ctx, `SELECT count(*) FROM information_schema.columns WHERE table_schema = 'public'
		AND table_name = 'posts' AND column_name IN ('success_criteria', 'weight', 'accepted_answer_id', 'evolved_into')`))
	assert.Equal(t, before.PostFields, digestOf(t, pool, `(SELECT post_id, type, status, success_criteria, weight, accepted_answer_id, evolved_into FROM legacy_archive.post_fields)`))
	assert.Equal(t, before.PostStamps, digestOf(t, pool, `(SELECT id, created_at, updated_at, upvotes, downvotes FROM posts)`))
	_, err = pool.Exec(ctx, `INSERT INTO posts (type, title, description, posted_by_type, posted_by_id, status)
		VALUES ('problem', 'refused', 'body', 'agent', 'kc-agent', 'open')`)
	assert.ErrorContains(t, err, "posts_type_check")
	_, err = pool.Exec(ctx, `INSERT INTO votes (target_type, target_id, voter_type, voter_id, direction)
		VALUES ('approach', gen_random_uuid(), 'agent', 'kc-agent', 'up')`)
	assert.ErrorContains(t, err, "votes_target_type_check")

	// Down: every table, row, field, target row, check, foreign key and function is back.
	_, err = pool.Exec(ctx, legacyArchiveDownSQL(t))
	require.NoError(t, err)
	pool.pool.Reset()
	after := readLegacyArchiveState(t, pool, "public")
	assert.Equal(t, before, after)
	assert.Equal(t, 0, countRows(t, pool, ctx, `SELECT count(*) FROM pg_namespace WHERE nspname = 'legacy_archive'`))

	// Up again: the same manifest.
	archiveLegacy()
	for _, table := range LegacyTables {
		assert.Equal(t, before.Tables[table], digestOf(t, pool, "legacy_archive."+table), "second archive of %s", table)
	}
}

// A post hard-deleted after the archive takes its legacy rows' foreign key with it: the down
// migration archives those rows (and the legacy rows that reference them) into
// rollback_archive instead of failing or dropping them, and restores everything else.
func TestLegacyArchiveMigration_DownArchivesTheRowsOfAPostHardDeletedAfterTheArchive(t *testing.T) {
	pool, archiveLegacy := newPreArchiveScratchDatabase(t)
	ctx := context.Background()
	s := seedLegacyArchive(t, pool)
	_, err := RunKnowledgeCutover(ctx, pool, KnowledgeCutoverOptions{})
	require.NoError(t, err)
	before := readLegacyArchiveState(t, pool, "public")
	archiveLegacy()

	// The cutover problem holds approach (with its progress note); approach2 relates to it.
	_, err = pool.Exec(ctx, `DELETE FROM posts WHERE id = $1`, s.problem)
	require.NoError(t, err)

	_, err = pool.Exec(ctx, legacyArchiveDownSQL(t))
	require.NoError(t, err)
	pool.pool.Reset()

	archived := map[string]int{}
	rows, err := pool.Query(ctx, `SELECT source_table, count(*) FROM rollback_archive
		WHERE reason = '000138 down: post hard-deleted after the archive' GROUP BY source_table`)
	require.NoError(t, err)
	for rows.Next() {
		var table string
		var n int
		require.NoError(t, rows.Scan(&table, &n))
		archived[table] = n
	}
	require.NoError(t, rows.Err())
	assert.Equal(t, map[string]int{"approaches": 1, "progress_notes": 1, "approach_relationships": 1}, archived)

	assert.Equal(t, 0, countRows(t, pool, ctx, `SELECT count(*) FROM approaches WHERE id = $1`, s.approach))
	assert.Equal(t, 1, countRows(t, pool, ctx, `SELECT count(*) FROM approaches WHERE id = $1`, s.approach2))
	assert.Equal(t, int(before.Tables["answers"].Rows), countRows(t, pool, ctx, `SELECT count(*) FROM answers`))
	assert.Equal(t, int(before.Tables["comments"].Rows), countRows(t, pool, ctx, `SELECT count(*) FROM comments`))
	var fk int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM pg_constraint WHERE conname = 'approaches_problem_id_fkey'`).Scan(&fk))
	assert.Equal(t, 1, fk, "the foreign key is back")
}

// The archive migration refuses while a legacy contribution has no reply: archiving it then
// would take it out of live storage before the cutover converted it.
func TestLegacyArchiveMigration_RefusesWhileALegacyContributionHasNoReply(t *testing.T) {
	pool, _ := newPreArchiveScratchDatabase(t)
	ctx := context.Background()
	seedCutoverLegacy(t, pool)

	_, after := splitAtLegacyArchive(t)
	sql, err := os.ReadFile(after[0])
	require.NoError(t, err)
	_, err = pool.Exec(ctx, string(sql))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "3 legacy contributions have no reply yet: run cmd/cutover")
	assert.Equal(t, 1, countRows(t, pool, ctx, `SELECT count(*) FROM pg_class
		WHERE relnamespace = 'public'::regnamespace AND relname = 'approaches'`), "nothing moved")
}

// archivedPostStatus is the status the legacy archive migration leaves on a post: the retired
// statuses become open, every other status stays. (Every type becomes "post".)
func archivedPostStatus(status string) string {
	if models.IsRetiredPostStatus(models.PostStatus(status)) {
		return string(models.PostStatusOpen)
	}
	return status
}
