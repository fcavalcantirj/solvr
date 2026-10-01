package db

import (
	"context"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
)

// GET /v1/overview is served from a 30 s snapshot on each API instance, and its Posts section
// lists public posts that carry a live human or agent reply: id, type, title, status, tags and the
// contribution count. The snapshots were dropped only by room changes (OverviewChanged), so
// measured on HEAD before migration 000123 (idx 77 slice 7 spike, live API): a post deleted by its
// author, a title edit, a moderation rejection and a post's only reply deleted all stayed on the
// overview for 30.0-30.2 s while GET /v1/posts/{id} already answered 404. Every write that can
// change what that section shows now announces it on OverviewChannel at commit, whoever writes it
// (handler, moderation goroutine, translation job, SQL); writes it cannot see stay quiet.

// overviewNotices LISTENs on OverviewChannel on a connection of its own.
type overviewNotices struct {
	conn *pgx.Conn
	pool *Pool
}

func listenOverviewNotices(ctx context.Context, t *testing.T, pool *Pool) *overviewNotices {
	t.Helper()
	conn, err := pgx.ConnectConfig(ctx, pool.pool.Config().ConnConfig.Copy())
	require.NoError(t, err, "connect listener")
	t.Cleanup(func() { conn.Close(context.Background()) })
	_, err = conn.Exec(ctx, "LISTEN "+OverviewChannel)
	require.NoError(t, err)
	return &overviewNotices{conn: conn, pool: pool}
}

// since counts the overview notices committed since the previous call. It sends a sentinel
// notice and reads up to it: notices arrive in commit order, so whatever comes first was sent
// by the writes before it, and nothing needs a timeout to prove a write stayed quiet.
func (n *overviewNotices) since(ctx context.Context, t *testing.T) int {
	t.Helper()
	_, err := n.pool.Exec(ctx, `SELECT pg_notify($1, 'sentinel')`, OverviewChannel)
	require.NoError(t, err)
	count := 0
	for {
		wctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		msg, err := n.conn.WaitForNotification(wctx)
		cancel()
		require.NoError(t, err, "the sentinel notice never arrived")
		if msg.Payload == "sentinel" {
			return count
		}
		count++
	}
}

func TestOverviewChanges_WritesThatChangeTheListedPostsAnnounceIt(t *testing.T) {
	pool, _ := newMigratedScratchDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	author := authorAgent(ctx, t, pool, "ov_"+uuid.NewString()[:8])
	voter := authorAgent(ctx, t, pool, "ovv_"+uuid.NewString()[:8])
	posts, replies := NewPostRepository(pool), NewReplyRepository(pool)
	exec := func(sql string, args ...any) {
		t.Helper()
		_, err := pool.Exec(ctx, sql, args...)
		require.NoError(t, err, sql)
	}
	listed := func(title string) (string, string) {
		t.Helper()
		post, err := posts.Create(ctx, &models.Post{
			Type: models.PostTypeQuestion, Title: title, Description: title + ", the description of the post",
			Tags: []string{"overview"}, PostedByType: models.AuthorTypeAgent, PostedByID: author,
			Status: models.PostStatusOpen,
		})
		require.NoError(t, err)
		reply, err := replies.Create(ctx, &models.Reply{PostID: post.ID, AuthorType: models.AuthorTypeAgent,
			AuthorID: author, Body: "A live reply on " + title})
		require.NoError(t, err)
		return post.ID, reply.ID
	}

	notices := listenOverviewNotices(ctx, t, pool)
	p, r := listed("A public post the overview lists")
	q, _ := listed("A second public post the overview lists")
	require.Equal(t, 0, notices.since(ctx, t), "a post and its first reply arrive with the next snapshot; nothing listed is stale")

	// Writes the Posts section does not show stay quiet.
	require.NoError(t, posts.Vote(ctx, p, "agent", voter, "up"))
	require.Equal(t, 0, notices.since(ctx, t), "a vote")
	current, err := posts.FindByID(ctx, p)
	require.NoError(t, err)
	edit := current.Post
	edit.Description = "A rewritten description, which the overview does not show"
	_, err = posts.UpdateIfUnmodified(ctx, &edit, nil)
	require.NoError(t, err)
	require.Equal(t, 0, notices.since(ctx, t), "a description edit")
	exec(`UPDATE posts SET title = title, tags = tags, status = status WHERE id = $1`, p)
	require.Equal(t, 0, notices.since(ctx, t), "rewriting the same title, tags and status")
	_, err = replies.Update(ctx, r, models.AuthorTypeAgent, author, "An edited reply body", nil, nil)
	require.NoError(t, err)
	require.Equal(t, 0, notices.since(ctx, t), "a reply body edit")
	exec(`INSERT INTO replies (post_id, author_type, author_id, body) VALUES ($1, 'system', 'moderation', 'approved')`, p)
	exec(`UPDATE replies SET deleted_at = NOW() WHERE post_id = $1 AND author_type = 'system'`, p)
	require.Equal(t, 0, notices.since(ctx, t), "a system verdict reply removed (never counted)")

	// Writes that change what it shows announce it, once per write.
	current, err = posts.FindByID(ctx, p)
	require.NoError(t, err)
	edit = current.Post
	edit.Title = "A public post with an edited title"
	_, err = posts.UpdateIfUnmodified(ctx, &edit, nil)
	require.NoError(t, err)
	require.Equal(t, 1, notices.since(ctx, t), "a title edit")
	exec(`UPDATE posts SET tags = ARRAY['overview', 'edited'] WHERE id = $1`, p)
	require.Equal(t, 1, notices.since(ctx, t), "a tags edit")
	require.NoError(t, posts.UpdateStatus(ctx, p, models.PostStatusSolved))
	require.Equal(t, 1, notices.since(ctx, t), "a status move")
	exec(`UPDATE posts SET status = 'rejected' WHERE id = $1`, q)
	require.Equal(t, 1, notices.since(ctx, t), "a moderation rejection written by SQL")
	require.NoError(t, posts.ApplyTranslation(ctx, q, "A translated title", "A translated description"))
	require.Equal(t, 1, notices.since(ctx, t), "the translation job rewriting the title")
	require.NoError(t, replies.Delete(ctx, r, models.AuthorTypeAgent, author))
	require.Equal(t, 1, notices.since(ctx, t), "a live reply removed")
	require.NoError(t, posts.Delete(ctx, p))
	require.Equal(t, 1, notices.since(ctx, t), "a post deleted by its author")
	hard, _ := listed("A public post removed outright")
	require.Equal(t, 0, notices.since(ctx, t))
	exec(`DELETE FROM posts WHERE id = $1`, hard)
	require.Equal(t, 1, notices.since(ctx, t), "a post row removed with its replies: one notice for the transaction")

	// A family post is never on the public overview, unless it was public a moment ago.
	owner := authorHuman(ctx, t, pool, "overview family owner")
	fam, famReply := listed("A post that becomes family-only")
	require.Equal(t, 0, notices.since(ctx, t))
	exec(`UPDATE posts SET visibility = 'family', owner_human_id = $2 WHERE id = $1`, fam, owner)
	require.Equal(t, 1, notices.since(ctx, t), "public to family takes the post off the overview")
	exec(`UPDATE posts SET title = 'A family post retitled', status = 'solved' WHERE id = $1`, fam)
	require.NoError(t, replies.Delete(ctx, famReply, models.AuthorTypeAgent, author))
	exec(`UPDATE posts SET deleted_at = NOW() WHERE id = $1`, fam)
	exec(`DELETE FROM posts WHERE id = $1`, fam)
	require.Equal(t, 0, notices.since(ctx, t), "family-only edits, removals and deletes")

	// A purge in one statement is one notice, not one per row.
	var purge []string
	for range 5 {
		id, _ := listed("A spam post purged in bulk")
		purge = append(purge, id)
	}
	require.Equal(t, 0, notices.since(ctx, t))
	exec(`UPDATE posts SET deleted_at = NOW() WHERE id = ANY($1::uuid[])`, purge)
	require.Equal(t, 1, notices.since(ctx, t), "a bulk soft delete")
}
