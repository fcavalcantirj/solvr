package db

import (
	"context"
	"testing"
)

// cutoverRepliesFor inserts, for the legacy contributions on the given posts only, the replies
// the contribution cutover makes from them (MigrateContributions + RemapLegacyRelations): one
// reply per approach, answer, response and comment, keeping legacy_type, legacy_id, author,
// created_at and deleted_at; a comment on the post is top-level, a comment on a contribution
// and a progress note are children of that contribution's reply. Fixtures on the shared test
// database use it because the global migration would convert every other test's rows too; the
// scratch-database tests run the real cutover. Rows already converted are skipped.
func cutoverRepliesFor(t *testing.T, pool *Pool, ctx context.Context, postIDs ...string) {
	t.Helper()
	const onConflict = ` ON CONFLICT (legacy_type, legacy_id) WHERE legacy_id IS NOT NULL DO NOTHING`
	stmts := []string{
		`INSERT INTO replies (post_id, author_type, author_id, body, legacy_type, legacy_id, created_at, updated_at, deleted_at)
		SELECT problem_id, author_type, author_id, angle, 'approach', id, created_at, created_at, deleted_at
		FROM approaches WHERE problem_id = ANY($1::uuid[])` + onConflict,
		`INSERT INTO replies (post_id, author_type, author_id, body, legacy_type, legacy_id, created_at, updated_at, deleted_at)
		SELECT question_id, author_type, author_id, content, 'answer', id, created_at, created_at, deleted_at
		FROM answers WHERE question_id = ANY($1::uuid[])` + onConflict,
		`INSERT INTO replies (post_id, author_type, author_id, body, legacy_type, legacy_id, created_at, updated_at)
		SELECT idea_id, author_type, author_id, content, 'response', id, created_at, created_at
		FROM responses WHERE idea_id = ANY($1::uuid[])` + onConflict,
		`INSERT INTO replies (post_id, author_type, author_id, body, legacy_type, legacy_id, created_at, updated_at, deleted_at)
		SELECT target_id, author_type, author_id, content, 'comment', id, created_at, created_at, deleted_at
		FROM comments WHERE target_type = 'post' AND target_id = ANY($1::uuid[])` + onConflict,
		`INSERT INTO replies (post_id, parent_reply_id, author_type, author_id, body, legacy_type, legacy_id, created_at, updated_at, deleted_at)
		SELECT pr.post_id, pr.id, c.author_type, c.author_id, c.content, 'comment', c.id, c.created_at, c.created_at, c.deleted_at
		FROM comments c JOIN replies pr ON pr.legacy_type = c.target_type AND pr.legacy_id = c.target_id
		WHERE c.target_type <> 'post' AND pr.post_id = ANY($1::uuid[])` + onConflict,
		`INSERT INTO replies (post_id, parent_reply_id, author_type, author_id, body, legacy_type, legacy_id, created_at, updated_at, deleted_at)
		SELECT pr.post_id, pr.id, pr.author_type, pr.author_id, n.content, 'progress_note', n.id,
			COALESCE(n.created_at, pr.created_at), COALESCE(n.created_at, pr.created_at), pr.deleted_at
		FROM progress_notes n JOIN replies pr ON pr.legacy_type = 'approach' AND pr.legacy_id = n.approach_id
		WHERE pr.post_id = ANY($1::uuid[])` + onConflict,
	}
	for _, sql := range stmts {
		if _, err := pool.Exec(ctx, sql, postIDs); err != nil {
			t.Fatalf("cutover replies: %v\n%s", err, sql)
		}
	}
}
