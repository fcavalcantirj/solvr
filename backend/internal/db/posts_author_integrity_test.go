package db

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/stretchr/testify/require"
)

// A post names its author through a foreign key (idx 68 step 3, migration 000117): a post links
// exactly one existing human or agent account, whatever its legacy type. A post stored before
// 000117 whose author has no account keeps its label as historical attribution; it is never
// given an invented account, and no new post can be written that way.

type postAuthorLink struct {
	Human, Agent *string
	Historical   bool
}

func postAuthorOf(ctx context.Context, t *testing.T, pool *Pool, id string) postAuthorLink {
	t.Helper()
	var l postAuthorLink
	require.NoError(t, pool.QueryRow(ctx, `SELECT author_human_id::text, author_agent_id, historical_author
		FROM posts WHERE id = $1`, id).Scan(&l.Human, &l.Agent, &l.Historical))
	return l
}

func createAuthoredPost(ctx context.Context, pool *Pool, postType models.PostType, authorType models.AuthorType, authorID string) (*models.Post, error) {
	return NewPostRepository(pool).Create(ctx, &models.Post{
		Type: postType, Title: "a post by " + authorID, Description: "A post that names its author",
		PostedByType: authorType, PostedByID: authorID, Status: models.PostStatusOpen,
	})
}

func TestPostAuthors_ANewPostNamesExactlyOneExistingAccount(t *testing.T) {
	pool, _ := newMigratedScratchDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	agent := authorAgent(ctx, t, pool, "post_author_agent")
	human := authorHuman(ctx, t, pool, "Post Author Human")

	byAgent, err := createAuthoredPost(ctx, pool, models.PostTypePost, models.AuthorTypeAgent, agent)
	require.NoError(t, err)
	require.Equal(t, postAuthorLink{Agent: &agent}, postAuthorOf(ctx, t, pool, byAgent.ID))
	byHuman, err := createAuthoredPost(ctx, pool, models.PostTypePost, models.AuthorTypeHuman, human)
	require.NoError(t, err)
	require.Equal(t, postAuthorLink{Human: &human}, postAuthorOf(ctx, t, pool, byHuman.ID))
	upper, err := createAuthoredPost(ctx, pool, models.PostTypePost, models.AuthorTypeHuman, strings.ToUpper(human))
	require.NoError(t, err, "a human id is a UUID, whatever its case")
	require.Equal(t, postAuthorLink{Human: &human}, postAuthorOf(ctx, t, pool, upper.ID))

	// No account behind the author: refused, for a canonical post and a legacy-typed one alike.
	for _, postType := range []models.PostType{models.PostTypePost, models.PostTypePost} {
		_, err = createAuthoredPost(ctx, pool, postType, models.AuthorTypeAgent, "no_such_agent")
		requireConstraintViolation(t, err, "posts_author_agent_fkey")
		_, err = createAuthoredPost(ctx, pool, postType, models.AuthorTypeHuman, "1b4e28ba-2fa1-41d2-883f-0016d3cca427")
		requireConstraintViolation(t, err, "posts_author_human_fkey")
		_, err = createAuthoredPost(ctx, pool, postType, models.AuthorTypeHuman, "not-a-uuid")
		requireConstraintViolation(t, err, "posts_author_human_fkey")
		_, err = createAuthoredPost(ctx, pool, postType, models.AuthorTypeAgent, strings.Repeat("x", 60))
		requireConstraintViolation(t, err, "posts_author_agent_fkey")
	}
	_, err = pool.Exec(ctx, `UPDATE posts SET posted_by_id = 'no_such_agent' WHERE id = $1`, byAgent.ID)
	requireConstraintViolation(t, err, "posts_author_agent_fkey")
	_, err = pool.Exec(ctx, `UPDATE posts SET posted_by_type = 'human' WHERE id = $1`, byAgent.ID)
	requireConstraintViolation(t, err, "posts_author_human_fkey")

	// The link is derived, never written: a post cannot be relabelled historical.
	_, err = pool.Exec(ctx, `UPDATE posts SET historical_author = true, author_agent_id = NULL WHERE id = $1`, byAgent.ID)
	require.NoError(t, err)
	require.Equal(t, postAuthorLink{Agent: &agent}, postAuthorOf(ctx, t, pool, byAgent.ID))

	// Re-attribution to another existing account moves the link with it.
	_, err = pool.Exec(ctx, `UPDATE posts SET posted_by_type = 'human', posted_by_id = $2 WHERE id = $1`, byAgent.ID, human)
	require.NoError(t, err)
	require.Equal(t, postAuthorLink{Human: &human}, postAuthorOf(ctx, t, pool, byAgent.ID))
}

func TestPostAuthors_TheMigrationLinksStoredPostsAndLabelsTheRest(t *testing.T) {
	pool, _ := newMigratedScratchDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	migration := func(direction string) string {
		sql, err := os.ReadFile(filepath.Join(backendRoot(t), "migrations", "000117_post_author_fks."+direction+".sql"))
		require.NoError(t, err)
		return string(sql)
	}

	agent := authorAgent(ctx, t, pool, "stored_post_author_agent")
	human := authorHuman(ctx, t, pool, "Stored Post Author Human")
	_, err := pool.Exec(ctx, migration("down"))
	require.NoError(t, err)

	// Posts stored before 000117, one per kind of author.
	stored := map[string]string{}
	for _, a := range [][3]string{{"post", "agent", agent}, {"post", "human", human},
		{"post", "agent", "stored_agent_without_account"}, {"post", "human", "stored-human-name"}} {
		stored[a[2]] = insertTestPostWithAuthor(t, pool, ctx, a[0], "stored post", "a post stored before 000117", nil, "open", a[1], a[2])
	}
	_, err = pool.Exec(ctx, migration("up"))
	require.NoError(t, err)

	require.Equal(t, postAuthorLink{Agent: &agent}, postAuthorOf(ctx, t, pool, stored[agent]))
	require.Equal(t, postAuthorLink{Human: &human}, postAuthorOf(ctx, t, pool, stored[human]))
	gone, named := stored["stored_agent_without_account"], stored["stored-human-name"]
	require.Equal(t, postAuthorLink{Historical: true}, postAuthorOf(ctx, t, pool, gone))
	require.Equal(t, postAuthorLink{Historical: true}, postAuthorOf(ctx, t, pool, named))
	var accounts int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM agents WHERE id = 'stored_agent_without_account'`).Scan(&accounts))
	require.Zero(t, accounts, "no account is invented for a historical author")

	// A historical post keeps its label through edits, and cannot be made to link nothing
	// by writing the derived columns.
	_, err = pool.Exec(ctx, `UPDATE posts SET title = 'edited stored post', historical_author = false WHERE id = $1`, gone)
	require.NoError(t, err)
	require.Equal(t, postAuthorLink{Historical: true}, postAuthorOf(ctx, t, pool, gone))
	var label string
	require.NoError(t, pool.QueryRow(ctx, `SELECT posted_by_id FROM posts WHERE id = $1`, gone).Scan(&label))
	require.Equal(t, "stored_agent_without_account", label)

	// Relabelling it names an account: another missing one is refused, a real one links.
	_, err = pool.Exec(ctx, `UPDATE posts SET posted_by_id = 'another_agent_without_account' WHERE id = $1`, gone)
	requireConstraintViolation(t, err, "posts_author_agent_fkey")
	_, err = pool.Exec(ctx, `UPDATE posts SET posted_by_id = $2 WHERE id = $1`, gone, agent)
	require.NoError(t, err)
	require.Equal(t, postAuthorLink{Agent: &agent}, postAuthorOf(ctx, t, pool, gone))

	// A new post is never historical, even with the label a historical post carries.
	_, err = createAuthoredPost(ctx, pool, models.PostTypePost, models.AuthorTypeHuman, "stored-human-name")
	requireConstraintViolation(t, err, "posts_author_human_fkey")

	// With the trigger bypassed, the checks still refuse a post that links nothing or links an
	// account its label does not name.
	withoutTrigger := func(sql string, args ...any) error {
		tx, err := pool.BeginTx(ctx)
		require.NoError(t, err)
		defer tx.Rollback(ctx) //nolint:errcheck
		_, err = tx.Exec(ctx, `SET LOCAL session_replication_role = replica`)
		require.NoError(t, err)
		_, err = tx.Exec(ctx, sql, args...)
		return err
	}
	requireConstraintViolation(t, withoutTrigger(`UPDATE posts SET historical_author = false WHERE id = $1`, named),
		"posts_exactly_one_author")
	requireConstraintViolation(t, withoutTrigger(`UPDATE posts SET author_human_id = NULL, author_agent_id = $2 WHERE id = $1`,
		stored[human], agent), "posts_author_matches_label")
}

func TestPostAuthors_AnAccountThatAuthorsPostsIsNotHardDeleted(t *testing.T) {
	pool, _ := newMigratedScratchDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	agent := authorAgent(ctx, t, pool, "hard_delete_post_agent")
	human := authorHuman(ctx, t, pool, "Hard Delete Post Human")
	byAgent, err := createAuthoredPost(ctx, pool, models.PostTypePost, models.AuthorTypeAgent, agent)
	require.NoError(t, err)
	byHuman, err := createAuthoredPost(ctx, pool, models.PostTypePost, models.AuthorTypeHuman, human)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE posts SET deleted_at = NOW() WHERE id = $1`, byHuman.ID)
	require.NoError(t, err, "a soft-deleted post still names its author")

	require.ErrorIs(t, NewAgentRepository(pool).HardDelete(ctx, agent), ErrAccountAuthorsContent)
	require.ErrorIs(t, NewUserRepository(pool).HardDelete(ctx, human), ErrAccountAuthorsContent)
	require.Equal(t, postAuthorLink{Agent: &agent}, postAuthorOf(ctx, t, pool, byAgent.ID), "the post keeps its author")
	require.Equal(t, postAuthorLink{Human: &human}, postAuthorOf(ctx, t, pool, byHuman.ID), "the post keeps its author")

	// An account with no posts is still hard-deleted.
	require.NoError(t, NewAgentRepository(pool).HardDelete(ctx, authorAgent(ctx, t, pool, "hard_delete_idle_post_agent")))
	require.NoError(t, NewUserRepository(pool).HardDelete(ctx, authorHuman(ctx, t, pool, "Idle Post Human")))
}
