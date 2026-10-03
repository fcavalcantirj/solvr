package db

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// A post names an existing account (000117: posts_author_agent_fkey, posts_author_human_fkey),
// so a fixture that writes a post as a human needs a real user id where it used a label.

// testUserID is the human account the shared-database search, stats and translation fixtures
// post as (they used the label "test-user" before 000117). It is one fixed account, created on
// first use, so cleanupTestData still finds every post they wrote by its author.
const testUserID = "7e57c0de-0000-4000-8000-000000000001"

// testUser creates the fixture human account unless it exists, and returns its id.
func testUser(ctx context.Context, t *testing.T, pool *Pool) string {
	t.Helper()
	_, err := pool.Exec(ctx, `INSERT INTO users (id, username, display_name, email, auth_provider, auth_provider_id, referral_code)
		VALUES ($1, 'fixture_test_user', 'Test User', 'fixture-test-user@example.test', 'email', 'fixture-test-user', 'TSTUSER1')
		ON CONFLICT DO NOTHING`, testUserID)
	require.NoError(t, err, "create the fixture test user")
	return testUserID
}

// deletePostsNamingAccounts deletes the posts that name an account (their replies go with
// them), so a wholesale account wipe is not refused by posts_author_agent_fkey or
// posts_author_human_fkey.
func deletePostsNamingAccounts(ctx context.Context, pool *Pool) {
	_, _ = pool.Exec(ctx, `DELETE FROM posts WHERE author_agent_id IS NOT NULL OR author_human_id IS NOT NULL`)
}

// makePostAuthorHistorical turns postID into a post stored before 000117 whose author has no
// account: its label becomes authorType/label and it links nothing. posts_resolve_author refuses
// that for every write, so it is done the only way such a row came to exist, with the trigger
// bypassed; the checks still hold it to a labelled historical row.
func makePostAuthorHistorical(ctx context.Context, t *testing.T, pool *Pool, postID, authorType, label string) {
	t.Helper()
	tx, err := pool.BeginTx(ctx)
	require.NoError(t, err)
	defer tx.Rollback(ctx) //nolint:errcheck // no-op after Commit
	_, err = tx.Exec(ctx, `SET LOCAL session_replication_role = replica`)
	require.NoError(t, err)
	_, err = tx.Exec(ctx, `UPDATE posts SET posted_by_type = $2, posted_by_id = $3, author_human_id = NULL,
		author_agent_id = NULL, historical_author = true WHERE id = $1`, postID, authorType, label)
	require.NoError(t, err, "make post %s historical", postID)
	require.NoError(t, tx.Commit(ctx))
}
