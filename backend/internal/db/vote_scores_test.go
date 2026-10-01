package db

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/stretchr/testify/require"
)

// Post and reply scores (upvotes, downvotes) are a projection of their confirmed votes
// (idx 77). These tests run on a scratch database so a drift check or rebuild of every
// target sees only rows they created.

type voteScore struct{ up, down int }

// scoreTable maps a vote target type to the table holding its projected score.
var scoreTable = map[string]string{"post": "posts", "reply": "replies", "blog_post": "blog_posts"}

func readVoteScore(ctx context.Context, t *testing.T, pool *Pool, targetType, id string) voteScore {
	t.Helper()
	var s voteScore
	require.NoError(t, pool.QueryRow(ctx, fmt.Sprintf(
		`SELECT upvotes, downvotes FROM %s WHERE id = $1`, scoreTable[targetType]), id,
	).Scan(&s.up, &s.down))
	return s
}

// voteCount is what the votes table says: confirmed votes by direction.
func voteCount(ctx context.Context, t *testing.T, pool *Pool, targetType, id string) voteScore {
	t.Helper()
	var s voteScore
	require.NoError(t, pool.QueryRow(ctx, `
		SELECT COUNT(*) FILTER (WHERE direction = 'up'), COUNT(*) FILTER (WHERE direction = 'down')
		FROM votes WHERE target_type = $1 AND target_id = $2 AND confirmed = true`, targetType, id,
	).Scan(&s.up, &s.down))
	return s
}

type voteScoreDrift struct {
	targetType, targetID string
	storedUp, storedDown *int
	voteUp, voteDown     int
}

func readVoteScoreDrift(ctx context.Context, t *testing.T, pool *Pool, targetType, id *string) []voteScoreDrift {
	t.Helper()
	rows, err := pool.Query(ctx, `
		SELECT target_type, target_id::text, stored_upvotes, vote_upvotes, stored_downvotes, vote_downvotes
		FROM vote_score_drift($1, $2::uuid)`, targetType, id)
	require.NoError(t, err)
	defer rows.Close()
	var out []voteScoreDrift
	for rows.Next() {
		var d voteScoreDrift
		require.NoError(t, rows.Scan(&d.targetType, &d.targetID, &d.storedUp, &d.voteUp, &d.storedDown, &d.voteDown))
		out = append(out, d)
	}
	require.NoError(t, rows.Err())
	return out
}

func rebuildVoteScores(ctx context.Context, pool *Pool, targetType, id *string) (int, error) {
	var n int
	err := pool.QueryRow(ctx, `SELECT rebuild_vote_scores($1, $2::uuid)`, targetType, id).Scan(&n)
	return n, err
}

// voteTargets creates a post and a reply on it to vote on.
func voteTargets(ctx context.Context, t *testing.T, pool *Pool, label string) (postID, replyID string) {
	t.Helper()
	post, err := NewPostRepository(pool).Create(ctx, &models.Post{
		Type: models.PostTypePost, Title: "Vote score target " + label,
		Description:  "A post whose score is derived from its votes (" + label + ")",
		PostedByType: models.AuthorTypeAgent, PostedByID: authorAgent(ctx, t, pool, "score_author"), Status: models.PostStatusOpen,
	})
	require.NoError(t, err)
	return post.ID, extraReply(ctx, t, pool, post.ID, label)
}

func extraReply(ctx context.Context, t *testing.T, pool *Pool, postID, label string) string {
	t.Helper()
	reply, err := NewReplyRepository(pool).Create(ctx, &models.Reply{
		PostID: postID, AuthorType: models.AuthorTypeAgent, AuthorID: authorAgent(ctx, t, pool, "score_author"), Body: "reply " + label,
	})
	require.NoError(t, err)
	return reply.ID
}

func strPtr(s string) *string { return &s }

func TestVoteScores_EveryVoteEventMovesTheScoreInItsOwnTransaction(t *testing.T) {
	pool, _ := newMigratedScratchDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	postID, replyID := voteTargets(ctx, t, pool, "events")
	posts, replies := NewPostRepository(pool), NewReplyRepository(pool)
	vote := map[string]func(voter, direction string) error{
		"post":  func(voter, d string) error { return posts.Vote(ctx, postID, "agent", voter, d) },
		"reply": func(voter, d string) error { return replies.Vote(ctx, replyID, "agent", voter, d) },
	}

	for _, target := range []struct{ typ, id string }{{"post", postID}, {"reply", replyID}} {
		score := func() voteScore { return readVoteScore(ctx, t, pool, target.typ, target.id) }
		exec := func(sql string, args ...any) {
			t.Helper()
			_, err := pool.Exec(ctx, sql, args...)
			require.NoError(t, err)
		}

		require.NoError(t, vote[target.typ]("alice", "up"))
		require.NoError(t, vote[target.typ]("bob", "up"))
		require.Equal(t, voteScore{2, 0}, score(), "%s: two upvotes", target.typ)
		require.NoError(t, vote[target.typ]("alice", "down"))
		require.Equal(t, voteScore{1, 1}, score(), "%s: a flip moves one vote across", target.typ)
		require.NoError(t, vote[target.typ]("alice", "down"))
		require.Equal(t, voteScore{1, 1}, score(), "%s: repeating a vote changes nothing", target.typ)

		// Only confirmed votes count, whoever writes the vote row.
		exec(`UPDATE votes SET confirmed = false WHERE target_id = $1 AND voter_id = 'alice'`, target.id)
		require.Equal(t, voteScore{1, 0}, score(), "%s: an unconfirmed vote is not counted", target.typ)
		exec(`UPDATE votes SET confirmed = true WHERE target_id = $1 AND voter_id = 'alice'`, target.id)
		require.Equal(t, voteScore{1, 1}, score(), "%s: a confirmed vote is counted again", target.typ)
		exec(`INSERT INTO votes (target_type, target_id, voter_type, voter_id, direction, confirmed)
			VALUES ($1, $2, 'agent', 'carol', 'up', false)`, target.typ, target.id)
		require.Equal(t, voteScore{1, 1}, score(), "%s: an unconfirmed insert is not counted", target.typ)
		exec(`INSERT INTO votes (target_type, target_id, voter_type, voter_id, direction, confirmed)
			VALUES ($1, $2, 'agent', 'dave', 'down', true)`, target.typ, target.id)
		require.Equal(t, voteScore{1, 2}, score(), "%s: a confirmed insert is counted", target.typ)
		exec(`DELETE FROM votes WHERE target_id = $1 AND voter_id IN ('bob', 'dave')`, target.id)
		require.Equal(t, voteScore{0, 1}, score(), "%s: a deleted vote is uncounted", target.typ)

		// A vote whose transaction rolls back never counted.
		tx, err := pool.BeginTx(ctx)
		require.NoError(t, err)
		_, err = tx.Exec(ctx, `INSERT INTO votes (target_type, target_id, voter_type, voter_id, direction, confirmed)
			VALUES ($1, $2, 'agent', 'erin', 'up', true)`, target.typ, target.id)
		require.NoError(t, err)
		require.NoError(t, tx.Rollback(ctx))
		require.Equal(t, voteScore{0, 1}, score(), "%s: a rolled-back vote is not counted", target.typ)

		require.Equal(t, voteCount(ctx, t, pool, target.typ, target.id), score(), "%s: the score is its votes", target.typ)
	}
	require.Empty(t, readVoteScoreDrift(ctx, t, pool, nil, nil))
}

// Concurrent requests from one voter (double clicks, client retries) count once. Before
// the projection the API read the voter's previous vote outside its transaction, so N
// concurrent flips each moved the counters, and N concurrent first votes failed on the
// unique vote key.
func TestVoteScores_ConcurrentVotesByOneVoterCountOnce(t *testing.T) {
	pool, _ := newMigratedScratchDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	postID, replyID := voteTargets(ctx, t, pool, "concurrent")
	posts, replies := NewPostRepository(pool), NewReplyRepository(pool)

	const racers = 8
	concurrently := func(voter, direction string) {
		t.Helper()
		var wg sync.WaitGroup
		errs := make(chan error, 2*racers)
		for range racers {
			wg.Add(2)
			go func() { defer wg.Done(); errs <- posts.Vote(ctx, postID, "agent", voter, direction) }()
			go func() { defer wg.Done(); errs <- replies.Vote(ctx, replyID, "agent", voter, direction) }()
		}
		wg.Wait()
		close(errs)
		for err := range errs {
			require.NoError(t, err, "%s voting %s", voter, direction)
		}
	}

	for round := range 4 {
		voter := fmt.Sprintf("racer_%d", round)
		concurrently(voter, "up")
		concurrently(voter, "down")
		if round%2 == 0 {
			concurrently(voter, "up")
		}
	}
	for _, target := range []struct{ typ, id string }{{"post", postID}, {"reply", replyID}} {
		require.Equal(t, voteScore{2, 2}, voteCount(ctx, t, pool, target.typ, target.id), target.typ)
		require.Equal(t, voteScore{2, 2}, readVoteScore(ctx, t, pool, target.typ, target.id),
			"%s: every voter counted exactly once", target.typ)
	}
	require.Empty(t, readVoteScoreDrift(ctx, t, pool, nil, nil))
}

func TestVoteScores_DriftNamesEveryBrokenScoreAndTheRebuildRepairsIt(t *testing.T) {
	pool, _ := newMigratedScratchDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	healthyPost, healthyReply := voteTargets(ctx, t, pool, "healthy")
	brokenPost, brokenReply := voteTargets(ctx, t, pool, "broken")
	posts, replies := NewPostRepository(pool), NewReplyRepository(pool)
	require.NoError(t, posts.Vote(ctx, healthyPost, "agent", "v1", "up"))
	require.NoError(t, replies.Vote(ctx, healthyReply, "agent", "v1", "down"))
	require.NoError(t, posts.Vote(ctx, brokenPost, "agent", "v1", "down"))
	require.NoError(t, posts.Vote(ctx, brokenPost, "agent", "v2", "down"))

	// Stored values the votes do not support: legacy counters, fixtures, a lost write.
	_, err := pool.Exec(ctx, `UPDATE posts SET downvotes = NULL WHERE id = $1`, brokenPost)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE replies SET upvotes = 3 WHERE id = $1`, brokenReply)
	require.NoError(t, err)
	// A retargeted vote is a migration step (the contribution cutover), not a vote event:
	// nothing moves, so the drift check names both targets until a rebuild runs.
	fromReply, toReply := extraReply(ctx, t, pool, brokenPost, "from"), extraReply(ctx, t, pool, brokenPost, "to")
	require.NoError(t, replies.Vote(ctx, fromReply, "agent", "v3", "up"))
	_, err = pool.Exec(ctx, `UPDATE votes SET target_id = $1 WHERE target_id = $2`, toReply, fromReply)
	require.NoError(t, err)
	require.Equal(t, voteScore{1, 0}, readVoteScore(ctx, t, pool, "reply", fromReply))
	require.Equal(t, voteScore{0, 0}, readVoteScore(ctx, t, pool, "reply", toReply))

	xmin := func(targetType, id string) string {
		var x string
		require.NoError(t, pool.QueryRow(ctx, fmt.Sprintf(`SELECT xmin::text FROM %s WHERE id = $1`, scoreTable[targetType]), id).Scan(&x))
		return x
	}
	healthyPostXmin, healthyReplyXmin := xmin("post", healthyPost), xmin("reply", healthyReply)

	drift := readVoteScoreDrift(ctx, t, pool, nil, nil)
	require.Len(t, drift, 4, "exactly the four broken targets: %+v", drift)
	byID := map[string]voteScoreDrift{}
	for _, d := range drift {
		byID[d.targetID] = d
	}
	require.Equal(t, "post", byID[brokenPost].targetType)
	require.Equal(t, 0, *byID[brokenPost].storedUp)
	require.Nil(t, byID[brokenPost].storedDown, "a NULL counter is drift, even when the other one is right")
	require.Equal(t, 0, byID[brokenPost].voteUp)
	require.Equal(t, 2, byID[brokenPost].voteDown)
	require.Equal(t, "reply", byID[brokenReply].targetType)
	require.Equal(t, 3, *byID[brokenReply].storedUp)
	require.Equal(t, 0, byID[brokenReply].voteUp)
	require.Equal(t, 1, *byID[fromReply].storedUp)
	require.Equal(t, 0, byID[fromReply].voteUp)
	require.Equal(t, 0, *byID[toReply].storedUp)
	require.Equal(t, 1, byID[toReply].voteUp)

	require.Len(t, readVoteScoreDrift(ctx, t, pool, strPtr("reply"), nil), 3, "filtered by type")
	require.Len(t, readVoteScoreDrift(ctx, t, pool, strPtr("post"), &brokenPost), 1, "filtered by target")
	require.Empty(t, readVoteScoreDrift(ctx, t, pool, strPtr("post"), &healthyPost))

	n, err := rebuildVoteScores(ctx, pool, strPtr("post"), &brokenPost)
	require.NoError(t, err)
	require.Equal(t, 1, n)
	require.Equal(t, voteScore{0, 2}, readVoteScore(ctx, t, pool, "post", brokenPost))
	require.Len(t, readVoteScoreDrift(ctx, t, pool, nil, nil), 3, "a one-target rebuild leaves the other broken targets alone")

	n, err = rebuildVoteScores(ctx, pool, nil, nil)
	require.NoError(t, err)
	require.Equal(t, 3, n)
	require.Equal(t, voteScore{0, 0}, readVoteScore(ctx, t, pool, "reply", brokenReply))
	require.Equal(t, voteScore{0, 0}, readVoteScore(ctx, t, pool, "reply", fromReply))
	require.Equal(t, voteScore{1, 0}, readVoteScore(ctx, t, pool, "reply", toReply))
	require.Empty(t, readVoteScoreDrift(ctx, t, pool, nil, nil))
	require.Equal(t, healthyPostXmin, xmin("post", healthyPost), "a consistent post is not rewritten")
	require.Equal(t, healthyReplyXmin, xmin("reply", healthyReply), "a consistent reply is not rewritten")

	n, err = rebuildVoteScores(ctx, pool, nil, nil)
	require.NoError(t, err)
	require.Equal(t, 0, n, "replaying the rebuild changes nothing")

	_, err = rebuildVoteScores(ctx, pool, strPtr("answer"), nil)
	require.ErrorContains(t, err, "answer", "a target type without a projected score is refused, not silently skipped")
}

// waitForLockWait blocks until a statement matching pattern is waiting on a lock.
func waitForLockWait(ctx context.Context, t *testing.T, pool *Pool, pattern string, done <-chan error) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for {
		var waiting int
		require.NoError(t, pool.QueryRow(ctx, `
			SELECT COUNT(*) FROM pg_stat_activity
			WHERE datname = current_database() AND wait_event_type = 'Lock' AND query LIKE $1`,
			pattern).Scan(&waiting))
		if waiting > 0 {
			return
		}
		select {
		case err := <-done:
			t.Fatalf("%s finished (err=%v) instead of waiting for the lock", pattern, err)
		default:
		}
		require.False(t, time.Now().After(deadline), "%s never waited for the lock", pattern)
		time.Sleep(20 * time.Millisecond)
	}
}

// A rebuild and a vote that meet must not lose the vote, whichever holds the row first.
func TestVoteScores_ARebuildAndAVoteNeverLoseEachOther(t *testing.T) {
	pool, _ := newMigratedScratchDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	postID, _ := voteTargets(ctx, t, pool, "inflight")
	posts := NewPostRepository(pool)
	for i := range 3 {
		require.NoError(t, posts.Vote(ctx, postID, "agent", fmt.Sprintf("committed_%d", i), "up"))
	}
	breakScore := func() {
		_, err := pool.Exec(ctx, `UPDATE posts SET upvotes = 50 WHERE id = $1`, postID)
		require.NoError(t, err)
	}

	// 1. The vote is in flight: the rebuild waits for it, then counts it.
	breakScore()
	tx, err := pool.BeginTx(ctx)
	require.NoError(t, err)
	defer tx.Rollback(context.Background()) //nolint:errcheck
	_, err = tx.Exec(ctx, `INSERT INTO votes (target_type, target_id, voter_type, voter_id, direction, confirmed)
		VALUES ('post', $1, 'agent', 'in_flight', 'up', true)`, postID)
	require.NoError(t, err)
	done := make(chan error, 1)
	go func() { _, err := rebuildVoteScores(ctx, pool, strPtr("post"), &postID); done <- err }()
	waitForLockWait(ctx, t, pool, "%rebuild_vote_scores%", done)
	require.NoError(t, tx.Commit(ctx))
	require.NoError(t, <-done)
	require.Equal(t, voteScore{4, 0}, readVoteScore(ctx, t, pool, "post", postID), "the in-flight vote is counted")

	// 2. The rebuild is in flight: the vote waits for it, then lands on the rebuilt score.
	breakScore()
	tx2, err := pool.BeginTx(ctx)
	require.NoError(t, err)
	defer tx2.Rollback(context.Background()) //nolint:errcheck
	var repaired int
	require.NoError(t, tx2.QueryRow(ctx, `SELECT rebuild_vote_scores('post', $1::uuid)`, postID).Scan(&repaired))
	require.Equal(t, 1, repaired)
	voted := make(chan error, 1)
	go func() { voted <- posts.Vote(ctx, postID, "agent", "late_voter", "up") }()
	waitForLockWait(ctx, t, pool, "%INSERT INTO votes%", voted)
	require.NoError(t, tx2.Commit(ctx))
	require.NoError(t, <-voted)
	require.Equal(t, voteScore{5, 0}, readVoteScore(ctx, t, pool, "post", postID), "the late vote lands on the rebuilt score")
	require.Empty(t, readVoteScoreDrift(ctx, t, pool, nil, nil))
}
