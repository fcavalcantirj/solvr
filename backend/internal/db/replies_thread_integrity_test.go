package db

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"
)

// Reply threads are enforced by the database (idx 68 step 2), not only by the reply
// repository: a parent reply belongs to the same post as its child, and no parent reference
// makes a reply its own ancestor. Every write below goes around ReplyRepository on purpose,
// the way the cutover, an operator or a future code path would.

// requireConstraintViolation asserts that err is the database refusing a write because of
// the named constraint.
func requireConstraintViolation(t *testing.T, err error, constraint string) {
	t.Helper()
	require.Error(t, err, "the database accepted a write %s forbids", constraint)
	var pgErr *pgconn.PgError
	require.True(t, errors.As(err, &pgErr), "not a database error: %v", err)
	require.Equal(t, constraint, pgErr.ConstraintName, "refused by %s (%s), want %s", pgErr.ConstraintName, pgErr.Message, constraint)
}

// replyFixture is a reply on postID, threaded under parent when it is set.
func replyFixture(postID string, parent *string, label string) *models.Reply {
	return &models.Reply{
		PostID: postID, ParentReplyID: parent, AuthorType: models.AuthorTypeAgent,
		AuthorID: "thread_author", Body: "reply " + label,
	}
}

func replyParent(ctx context.Context, t *testing.T, pool *Pool, id string) *string {
	t.Helper()
	var parent *string
	require.NoError(t, pool.QueryRow(ctx, `SELECT parent_reply_id::text FROM replies WHERE id = $1`, id).Scan(&parent))
	return parent
}

func TestReplyThreads_AParentBelongsToTheSamePost(t *testing.T) {
	pool, _ := newMigratedScratchDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	postA, replyA := voteTargets(ctx, t, pool, "thread post A")
	postB, replyB := voteTargets(ctx, t, pool, "thread post B")

	// A new reply on post B threaded under a reply of post A.
	_, err := pool.Exec(ctx, `
		INSERT INTO replies (post_id, parent_reply_id, author_type, author_id, body)
		VALUES ($1, $2, 'agent', 'thread_author', 'cross-post child')`, postB, replyA)
	requireConstraintViolation(t, err, "replies_parent_same_post_fkey")

	// An existing reply re-parented under a reply of another post.
	_, err = pool.Exec(ctx, `UPDATE replies SET parent_reply_id = $2 WHERE id = $1`, replyB, replyA)
	requireConstraintViolation(t, err, "replies_parent_same_post_fkey")
	require.Nil(t, replyParent(ctx, t, pool, replyB))

	// A parent moved to another post while its child stays behind.
	child := extraReply(ctx, t, pool, postA, "child of A")
	_, err = pool.Exec(ctx, `UPDATE replies SET parent_reply_id = $2 WHERE id = $1`, child, replyA)
	require.NoError(t, err, "threading under a reply of the same post")
	_, err = pool.Exec(ctx, `UPDATE replies SET post_id = $2 WHERE id = $1`, replyA, postB)
	requireConstraintViolation(t, err, "replies_parent_same_post_fkey")

	// What stays allowed: a thread inside one post, and deleting a parent deletes its thread.
	grandchild, err := NewReplyRepository(pool).Create(ctx, replyFixture(postA, &child, "grandchild"))
	require.NoError(t, err, "a reply threaded under a reply of the same post")
	_, err = pool.Exec(ctx, `DELETE FROM replies WHERE id = $1`, replyA)
	require.NoError(t, err)
	var left int
	require.NoError(t, pool.QueryRow(ctx, `SELECT COUNT(*) FROM replies WHERE id = ANY($1::uuid[])`,
		[]string{child, grandchild.ID}).Scan(&left))
	require.Zero(t, left, "the thread under a deleted parent goes with it")
}

func TestReplyThreads_NoReplyIsItsOwnAncestor(t *testing.T) {
	pool, _ := newMigratedScratchDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	postID, root := voteTargets(ctx, t, pool, "cycle post")
	mid, err := NewReplyRepository(pool).Create(ctx, replyFixture(postID, &root, "mid"))
	require.NoError(t, err)
	leaf, err := NewReplyRepository(pool).Create(ctx, replyFixture(postID, &mid.ID, "leaf"))
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `UPDATE replies SET parent_reply_id = id WHERE id = $1`, root)
	requireConstraintViolation(t, err, "replies_parent_acyclic")

	_, err = pool.Exec(ctx, `UPDATE replies SET parent_reply_id = $2 WHERE id = $1`, root, leaf.ID)
	requireConstraintViolation(t, err, "replies_parent_acyclic")
	require.Nil(t, replyParent(ctx, t, pool, root), "the refused re-parent left no trace")

	// Two new replies naming each other in one statement: each parent exists by the end of
	// the statement, so only the cycle check can refuse it.
	_, err = pool.Exec(ctx, `
		INSERT INTO replies (id, post_id, parent_reply_id, author_type, author_id, body) VALUES
			('00000000-0000-4000-8000-00000000000a', $1, '00000000-0000-4000-8000-00000000000b', 'agent', 'thread_author', 'a'),
			('00000000-0000-4000-8000-00000000000b', $1, '00000000-0000-4000-8000-00000000000a', 'agent', 'thread_author', 'b')`,
		postID)
	requireConstraintViolation(t, err, "replies_parent_acyclic")

	// Re-parenting that keeps the thread a tree is allowed.
	_, err = pool.Exec(ctx, `UPDATE replies SET parent_reply_id = $2 WHERE id = $1`, leaf.ID, root)
	require.NoError(t, err)
	require.Equal(t, root, *replyParent(ctx, t, pool, leaf.ID))
}

// Two transactions that each add one half of a cycle must not both commit: the second one
// waits for the first and then sees the parent it committed.
func TestReplyThreads_ConcurrentReparentsCannotCloseACycle(t *testing.T) {
	pool, _ := newMigratedScratchDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	postID, first := voteTargets(ctx, t, pool, "concurrent cycle post")
	second := extraReply(ctx, t, pool, postID, "second")

	tx1, err := pool.BeginTx(ctx)
	require.NoError(t, err)
	defer func() { _ = tx1.Rollback(ctx) }()
	_, err = tx1.Exec(ctx, `UPDATE replies SET parent_reply_id = $2 WHERE id = $1`, first, second)
	require.NoError(t, err)

	done := make(chan error, 1)
	go func() {
		tx2, err := pool.BeginTx(ctx)
		if err != nil {
			done <- err
			return
		}
		defer func() { _ = tx2.Rollback(ctx) }()
		if _, err := tx2.Exec(ctx, `UPDATE replies SET parent_reply_id = $2 WHERE id = $1 /* cycle-half-2 */`, second, first); err != nil {
			done <- err
			return
		}
		done <- tx2.Commit(ctx)
	}()

	waitForLockWait(ctx, t, pool, "%cycle-half-2%", done)
	require.NoError(t, tx1.Commit(ctx))

	select {
	case err = <-done:
	case <-ctx.Done():
		t.Fatal("the second re-parent never finished")
	}
	requireConstraintViolation(t, err, "replies_parent_acyclic")
	require.Equal(t, second, *replyParent(ctx, t, pool, first))
	require.Nil(t, replyParent(ctx, t, pool, second), "the second half of the cycle was not stored")
}
