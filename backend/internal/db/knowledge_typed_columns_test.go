package db

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"
)

// The knowledge schema keeps its domain in typed columns (idx 68 step 4, migration 000118):
// a post's body, visibility, publication and moderation states, timestamps and ownership, and a
// reply's body, timestamps and author, are typed columns the database constrains. JSON is only
// the bounded provenance of a reply migrated from a legacy contribution: a native reply has
// none, and a migrated one holds only the keys its legacy type's migration writes.

// requireNotNullViolation asserts that err is the database refusing a NULL in column.
func requireNotNullViolation(t *testing.T, err error, column string) {
	t.Helper()
	require.Error(t, err, "the database accepted a NULL %s", column)
	var pgErr *pgconn.PgError
	require.True(t, errors.As(err, &pgErr), "not a database error: %v", err)
	require.Equal(t, "23502", pgErr.Code, pgErr.Message)
	require.Equal(t, column, pgErr.ColumnName, pgErr.Message)
}

type typedColumn struct {
	Type     string
	Nullable bool
}

func typedColumnsOf(ctx context.Context, t *testing.T, pool *Pool, table string) map[string]typedColumn {
	t.Helper()
	rows, err := pool.Query(ctx, `SELECT column_name, data_type, is_nullable = 'YES' FROM information_schema.columns
		WHERE table_schema = 'public' AND table_name = $1`, table)
	require.NoError(t, err)
	defer rows.Close()
	cols := map[string]typedColumn{}
	for rows.Next() {
		var name string
		var c typedColumn
		require.NoError(t, rows.Scan(&name, &c.Type, &c.Nullable))
		cols[name] = c
	}
	require.NoError(t, rows.Err())
	return cols
}

func TestKnowledgeSchema_TheDomainLivesInTypedColumns(t *testing.T) {
	pool, _ := newMigratedScratchDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	uuidCol, text, varchar, ts := "uuid", "text", "character varying", "timestamp with time zone"
	posts := typedColumnsOf(ctx, t, pool, "posts")
	for name, want := range map[string]typedColumn{
		"id": {uuidCol, false}, "title": {varchar, false}, "description": {text, false},
		"visibility": {varchar, false}, "publication_state": {varchar, false}, "moderation_state": {varchar, false},
		"created_at": {ts, false}, "updated_at": {ts, false}, "deleted_at": {ts, true},
		"posted_by_type": {varchar, false}, "posted_by_id": {varchar, false},
		"author_human_id": {uuidCol, true}, "author_agent_id": {varchar, true}, "owner_human_id": {uuidCol, true},
	} {
		require.Equal(t, want, posts[name], "posts.%s", name)
	}
	replies := typedColumnsOf(ctx, t, pool, "replies")
	for name, want := range map[string]typedColumn{
		"id": {uuidCol, false}, "post_id": {uuidCol, false}, "parent_reply_id": {uuidCol, true}, "body": {text, false},
		"created_at": {ts, false}, "updated_at": {ts, false}, "deleted_at": {ts, true},
		"author_type": {varchar, false}, "author_id": {varchar, false},
		"author_human_id": {uuidCol, true}, "author_agent_id": {varchar, true},
		"legacy_type": {varchar, true}, "legacy_id": {uuidCol, true}, "provenance": {"jsonb", true},
	} {
		require.Equal(t, want, replies[name], "replies.%s", name)
	}

	// JSON is never the domain: the only JSON column of either table is reply provenance.
	var jsonColumns []string
	rows, err := pool.Query(ctx, `SELECT table_name || '.' || column_name FROM information_schema.columns
		WHERE table_schema = 'public' AND table_name IN ('posts', 'replies') AND data_type IN ('json', 'jsonb')`)
	require.NoError(t, err)
	for rows.Next() {
		var c string
		require.NoError(t, rows.Scan(&c))
		jsonColumns = append(jsonColumns, c)
	}
	rows.Close()
	require.NoError(t, rows.Err())
	require.Equal(t, []string{"replies.provenance"}, jsonColumns)

	// The states are closed sets, and a post always has its timestamps.
	post := authorPost(ctx, t, pool, authorAgent(ctx, t, pool, "typed_columns_agent"), "a post in typed columns")
	for column, constraint := range map[string]string{"visibility": "posts_visibility_check",
		"publication_state": "posts_publication_state_check", "moderation_state": "posts_moderation_state_check"} {
		_, err = pool.Exec(ctx, `UPDATE posts SET `+column+` = 'unheard_of' WHERE id = $1`, post)
		requireConstraintViolation(t, err, constraint)
	}
	for _, column := range []string{"created_at", "updated_at"} {
		_, err = pool.Exec(ctx, `UPDATE posts SET `+column+` = NULL WHERE id = $1`, post)
		requireNotNullViolation(t, err, column)
	}
	_, err = pool.Exec(ctx, `INSERT INTO posts (type, title, description, posted_by_type, posted_by_id, created_at)
		VALUES ('post', 'no time', 'a post without a creation time', 'agent', 'typed_columns_agent', NULL)`)
	requireNotNullViolation(t, err, "created_at")
}

func TestKnowledgeSchema_OnlyAMigratedReplyCarriesBoundedProvenance(t *testing.T) {
	pool, _ := newMigratedScratchDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	agent := authorAgent(ctx, t, pool, "provenance_agent")
	post := authorPost(ctx, t, pool, agent, "a post with migrated replies")
	insert := func(legacyType any, migrated bool, provenance any) error {
		_, err := pool.Exec(ctx, `INSERT INTO replies (post_id, author_type, author_id, body, legacy_type, legacy_id, provenance)
			VALUES ($1, 'agent', $2, 'a reply', $3, CASE WHEN $4::boolean THEN gen_random_uuid() END, $5::jsonb)`,
			post, agent, legacyType, migrated, provenance)
		return err
	}

	// Every key each legacy type's migration writes is accepted; so is no provenance at all.
	written := map[string]string{
		"approach": `{"legacy_table": "approaches", "angle": "a", "method": "m", "assumptions": ["x"],
			"differs_from": [], "status": "stuck", "outcome": "o", "solution": "s", "is_latest": true,
			"archived_cid": null, "approach_relationships": [{"legacy_id": "r", "relation_type": "builds_on"}]}`,
		"answer":        `{"legacy_table": "answers", "is_accepted": true}`,
		"response":      `{"legacy_table": "responses", "response_type": "build"}`,
		"comment":       `{"legacy_table": "comments", "target_type": "approach", "target_id": "c"}`,
		"progress_note": `{"legacy_table": "progress_notes", "approach_id": "p"}`,
	}
	for legacyType, provenance := range written {
		require.NoError(t, insert(legacyType, true, provenance), legacyType)
		require.NoError(t, insert(legacyType, true, nil), "a migrated %s without provenance", legacyType)
	}

	// A key another legacy type writes, or one no migration writes, is refused.
	for legacyType, key := range map[string]string{"approach": "response_type", "answer": "status",
		"response": "angle", "comment": "is_accepted", "progress_note": "status"} {
		requireConstraintViolation(t, insert(legacyType, true, `{"`+key+`": "x"}`), "replies_provenance_bounded")
		requireConstraintViolation(t, insert(legacyType, true, `{"legacy_table": "x", "visibility": "public"}`),
			"replies_provenance_bounded")
	}
	// Provenance is an object.
	for _, provenance := range []string{`["status"]`, `"stuck"`, `42`} {
		requireConstraintViolation(t, insert("approach", true, provenance), "replies_provenance_bounded")
	}

	// A native reply has no provenance, through the repository or not.
	_, err := NewReplyRepository(pool).Create(ctx, &models.Reply{PostID: post, AuthorType: models.AuthorTypeAgent,
		AuthorID: agent, Body: "a native reply", Provenance: json.RawMessage(`{"status": "stuck"}`)})
	requireConstraintViolation(t, err, "replies_provenance_bounded")
	requireConstraintViolation(t, insert(nil, false, `{}`), "replies_provenance_bounded")
	require.NoError(t, insert(nil, false, nil))

	// A migrated reply names its legacy type and id together.
	requireConstraintViolation(t, insert("comment", false, nil), "replies_legacy_pair")
	_, err = pool.Exec(ctx, `INSERT INTO replies (post_id, author_type, author_id, body, legacy_id)
		VALUES ($1, 'agent', $2, 'a reply', gen_random_uuid())`, post, agent)
	requireConstraintViolation(t, err, "replies_legacy_pair")

	// Provenance cannot be added to a native reply later either.
	var native string
	require.NoError(t, pool.QueryRow(ctx, `SELECT id::text FROM replies WHERE post_id = $1 AND legacy_id IS NULL LIMIT 1`, post).Scan(&native))
	_, err = pool.Exec(ctx, `UPDATE replies SET provenance = '{"status": "succeeded"}' WHERE id = $1`, native)
	requireConstraintViolation(t, err, "replies_provenance_bounded")
}

func TestKnowledgeSchema_TheMigrationRefusesRowsOutsideTheBoundsAndReverts(t *testing.T) {
	pool, _ := newMigratedScratchDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	migration := func(direction string) string {
		sql, err := os.ReadFile(filepath.Join(backendRoot(t), "migrations", "000118_knowledge_typed_columns."+direction+".sql"))
		require.NoError(t, err)
		return string(sql)
	}
	agent := authorAgent(ctx, t, pool, "bounds_migration_agent")
	post := authorPost(ctx, t, pool, agent, "a post stored before 000118")

	// Before 000118 a post could lose its timestamps and a native reply could hold any JSON.
	_, err := pool.Exec(ctx, migration("down"))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE posts SET updated_at = NULL WHERE id = $1`, post)
	require.NoError(t, err)
	var reply string
	require.NoError(t, pool.QueryRow(ctx, `INSERT INTO replies (post_id, author_type, author_id, body, provenance)
		VALUES ($1, 'agent', $2, 'a native reply', '{"status": "stuck"}') RETURNING id::text`, post, agent).Scan(&reply))

	// The migration rewrites nothing: it refuses to apply over such rows, all or nothing.
	_, err = pool.Exec(ctx, migration("up"))
	requireNotNullViolation(t, err, "updated_at")
	_, err = pool.Exec(ctx, `UPDATE posts SET updated_at = created_at WHERE id = $1`, post)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, migration("up"))
	requireConstraintViolation(t, err, "replies_provenance_bounded")
	var constraints int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM pg_constraint
		WHERE conname IN ('replies_legacy_pair', 'replies_provenance_bounded')`).Scan(&constraints))
	require.Zero(t, constraints, "a refused migration leaves nothing behind")

	_, err = pool.Exec(ctx, `UPDATE replies SET provenance = NULL WHERE id = $1`, reply)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, migration("up"))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE replies SET provenance = '{}' WHERE id = $1`, reply)
	requireConstraintViolation(t, err, "replies_provenance_bounded")
	_, err = pool.Exec(ctx, `UPDATE posts SET created_at = NULL WHERE id = $1`, post)
	requireNotNullViolation(t, err, "created_at")
}
