package db

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/stretchr/testify/require"
)

// A reply names its author through a foreign key (idx 68 step 3, migration 000116): a native
// reply links exactly one existing human or agent account, a system note links none and
// keeps its label, and a migrated reply whose author has no account keeps its label as
// historical attribution instead of inventing one.

type replyAuthorLink struct {
	Human, Agent *string
	Historical   bool
}

func replyAuthorOf(ctx context.Context, t *testing.T, pool *Pool, id string) replyAuthorLink {
	t.Helper()
	var l replyAuthorLink
	require.NoError(t, pool.QueryRow(ctx, `SELECT author_human_id::text, author_agent_id, historical_author
		FROM replies WHERE id = $1`, id).Scan(&l.Human, &l.Agent, &l.Historical))
	return l
}

func authorPost(ctx context.Context, t *testing.T, pool *Pool, agentID, title string) string {
	t.Helper()
	post, err := NewPostRepository(pool).Create(ctx, &models.Post{
		Type: models.PostTypePost, Title: title, Description: "A post whose replies name their authors",
		PostedByType: models.AuthorTypeAgent, PostedByID: agentID, Status: models.PostStatusOpen,
	})
	require.NoError(t, err)
	return post.ID
}

func TestReplyAuthors_ANativeReplyNamesExactlyOneExistingAccount(t *testing.T) {
	pool, _ := newMigratedScratchDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	agent := authorAgent(ctx, t, pool, "reply_author_agent")
	human := authorHuman(ctx, t, pool, "Reply Author Human")
	postID := authorPost(ctx, t, pool, agent, "native reply authors")
	replies := NewReplyRepository(pool)
	create := func(authorType models.AuthorType, authorID string) (*models.Reply, error) {
		return replies.Create(ctx, &models.Reply{PostID: postID, AuthorType: authorType, AuthorID: authorID, Body: "reply by " + authorID})
	}

	byAgent, err := create(models.AuthorTypeAgent, agent)
	require.NoError(t, err)
	require.Equal(t, replyAuthorLink{Agent: &agent}, replyAuthorOf(ctx, t, pool, byAgent.ID))
	byHuman, err := create(models.AuthorTypeHuman, human)
	require.NoError(t, err)
	require.Equal(t, replyAuthorLink{Human: &human}, replyAuthorOf(ctx, t, pool, byHuman.ID))
	note, err := create(models.AuthorTypeSystem, "solvr-moderator")
	require.NoError(t, err)
	require.Equal(t, replyAuthorLink{}, replyAuthorOf(ctx, t, pool, note.ID), "a system note names no account")

	// No account behind the author: refused, whatever the writer.
	_, err = create(models.AuthorTypeAgent, "no_such_agent")
	requireConstraintViolation(t, err, "replies_author_agent_fkey")
	_, err = create(models.AuthorTypeHuman, "1b4e28ba-2fa1-41d2-883f-0016d3cca427")
	requireConstraintViolation(t, err, "replies_author_human_fkey")
	_, err = create(models.AuthorTypeHuman, "not-a-uuid")
	requireConstraintViolation(t, err, "replies_author_human_fkey")
	_, err = create(models.AuthorTypeSystem, "  ")
	requireConstraintViolation(t, err, "replies_exactly_one_author")
	_, err = pool.Exec(ctx, `UPDATE replies SET author_id = 'no_such_agent' WHERE id = $1`, byAgent.ID)
	requireConstraintViolation(t, err, "replies_author_agent_fkey")

	// The link is derived, never written: a native reply cannot be relabelled historical.
	_, err = pool.Exec(ctx, `UPDATE replies SET historical_author = true, author_agent_id = NULL WHERE id = $1`, byAgent.ID)
	require.NoError(t, err)
	require.Equal(t, replyAuthorLink{Agent: &agent}, replyAuthorOf(ctx, t, pool, byAgent.ID))

	// Re-attribution to another existing account moves the link with it.
	other := authorAgent(ctx, t, pool, "reply_author_agent_2")
	_, err = pool.Exec(ctx, `UPDATE replies SET author_id = $2 WHERE id = $1`, byAgent.ID, other)
	require.NoError(t, err)
	require.Equal(t, replyAuthorLink{Agent: &other}, replyAuthorOf(ctx, t, pool, byAgent.ID))
}

func TestReplyAuthors_AMigratedReplyWithoutAnAccountKeepsItsLabel(t *testing.T) {
	pool, _ := newMigratedScratchDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	agent := authorAgent(ctx, t, pool, "migrated_author_agent")
	problem := insertTestPostWithAuthor(t, pool, ctx, "problem", "migrated authors problem", "body", nil, "open", "agent", agent)
	question := insertTestPostWithAuthor(t, pool, ctx, "question", "migrated authors question", "body", nil, "open", "agent", agent)
	kept := insertApproach(t, pool, ctx, problem, agent, "working", false)
	gone := insertApproach(t, pool, ctx, problem, "agent_without_account", "failed", false)
	var byName string
	require.NoError(t, pool.QueryRow(ctx, `INSERT INTO answers (question_id, author_type, author_id, content)
		VALUES ($1, 'human', 'legacy-display-name', 'an answer by a human with no account') RETURNING id::text`, question).Scan(&byName))

	_, err := MigrateContributions(ctx, pool)
	require.NoError(t, err, "the cutover converts contributions whose author has no account")

	label := func(legacyType, legacyID string) (authorType, authorID string, link replyAuthorLink) {
		t.Helper()
		var id string
		require.NoError(t, pool.QueryRow(ctx, `SELECT id::text, author_type, author_id FROM replies
			WHERE legacy_type = $1 AND legacy_id = $2`, legacyType, legacyID).Scan(&id, &authorType, &authorID))
		return authorType, authorID, replyAuthorOf(ctx, t, pool, id)
	}
	ty, id, link := label("approach", kept)
	require.Equal(t, []any{"agent", agent, replyAuthorLink{Agent: &agent}}, []any{ty, id, link})
	ty, id, link = label("approach", gone)
	require.Equal(t, []any{"agent", "agent_without_account", replyAuthorLink{Historical: true}}, []any{ty, id, link})
	ty, id, link = label("answer", byName)
	require.Equal(t, []any{"human", "legacy-display-name", replyAuthorLink{Historical: true}}, []any{ty, id, link})

	var accounts int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM agents WHERE id = 'agent_without_account'`).Scan(&accounts))
	require.Zero(t, accounts, "no account is invented for a historical author")
}

func TestReplyAuthors_TheMigrationLinksStoredRepliesAndLabelsTheRest(t *testing.T) {
	pool, _ := newMigratedScratchDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	migration := func(direction string) string {
		sql, err := os.ReadFile(filepath.Join(backendRoot(t), "migrations", "000116_reply_author_fks."+direction+".sql"))
		require.NoError(t, err)
		return string(sql)
	}

	agent := authorAgent(ctx, t, pool, "stored_author_agent")
	human := authorHuman(ctx, t, pool, "Stored Author Human")
	postID := authorPost(ctx, t, pool, agent, "stored reply authors")
	_, err := pool.Exec(ctx, migration("down"))
	require.NoError(t, err)

	// Replies stored before 000116, one per kind of author.
	stored := map[string]string{}
	for _, a := range [][2]string{{"agent", agent}, {"human", human}, {"system", "solvr-moderator"},
		{"agent", "stored_agent_without_account"}, {"human", "stored-human-name"}} {
		var id string
		require.NoError(t, pool.QueryRow(ctx, `INSERT INTO replies (post_id, author_type, author_id, body)
			VALUES ($1, $2, $3, 'stored reply') RETURNING id::text`, postID, a[0], a[1]).Scan(&id))
		stored[a[1]] = id
	}
	_, err = pool.Exec(ctx, migration("up"))
	require.NoError(t, err)

	require.Equal(t, replyAuthorLink{Agent: &agent}, replyAuthorOf(ctx, t, pool, stored[agent]))
	require.Equal(t, replyAuthorLink{Human: &human}, replyAuthorOf(ctx, t, pool, stored[human]))
	require.Equal(t, replyAuthorLink{}, replyAuthorOf(ctx, t, pool, stored["solvr-moderator"]))
	require.Equal(t, replyAuthorLink{Historical: true}, replyAuthorOf(ctx, t, pool, stored["stored_agent_without_account"]))
	require.Equal(t, replyAuthorLink{Historical: true}, replyAuthorOf(ctx, t, pool, stored["stored-human-name"]))
}

func TestReplyAuthors_AnAccountThatAuthorsRepliesIsNotHardDeleted(t *testing.T) {
	pool, _ := newMigratedScratchDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	agent := authorAgent(ctx, t, pool, "hard_delete_author_agent")
	human := authorHuman(ctx, t, pool, "Hard Delete Author Human")
	postID := authorPost(ctx, t, pool, agent, "hard delete authors")
	for _, a := range [][2]string{{"agent", agent}, {"human", human}} {
		_, err := NewReplyRepository(pool).Create(ctx, &models.Reply{
			PostID: postID, AuthorType: models.AuthorType(a[0]), AuthorID: a[1], Body: "a reply that keeps its author"})
		require.NoError(t, err)
	}

	require.ErrorIs(t, NewAgentRepository(pool).HardDelete(ctx, agent), ErrAccountAuthorsContent)
	require.ErrorIs(t, NewUserRepository(pool).HardDelete(ctx, human), ErrAccountAuthorsContent)
	var replies int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM replies WHERE post_id = $1`, postID).Scan(&replies))
	require.Equal(t, 2, replies, "the replies keep their authors")

	// An account with no replies is still hard-deleted.
	idle := authorAgent(ctx, t, pool, "hard_delete_idle_agent")
	require.NoError(t, NewAgentRepository(pool).HardDelete(ctx, idle))
	require.NoError(t, NewUserRepository(pool).HardDelete(ctx, authorHuman(ctx, t, pool, "Idle Human")))
}
