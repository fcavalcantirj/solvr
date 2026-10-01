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

// Blog post scores join the vote score projection (idx 77, migration 000119): the same
// triggers, drift check and rebuild that keep post and reply scores equal to their
// confirmed votes. Before it, BlogPostRepository.Vote read the voter's previous vote outside
// its transaction and moved the counters itself: five rounds of "up, then eight concurrent
// downs" from one voter left a blog post at upvotes = -34 while its votes said 1 (measured
// on HEAD, idx 77 slice 3). These tests run on scratch databases.

// blogVoteTarget creates a blog post to vote on.
func blogVoteTarget(ctx context.Context, t *testing.T, pool *Pool, label string) string {
	t.Helper()
	post, err := NewBlogPostRepository(pool).Create(ctx, &models.BlogPost{
		Slug: "blog-score-" + label, Title: "Blog score target " + label,
		Body:         "A blog post whose score is derived from its votes (" + label + ")",
		PostedByType: models.AuthorTypeAgent, PostedByID: authorAgent(ctx, t, pool, "blog_score_author"),
	})
	require.NoError(t, err)
	return post.ID
}

func TestBlogVoteScores_EveryVoteEventMovesTheScoreInItsOwnTransaction(t *testing.T) {
	pool, _ := newMigratedScratchDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	blogID := blogVoteTarget(ctx, t, pool, "events")
	blogs := NewBlogPostRepository(pool)
	vote := func(voter, direction string) error { return blogs.Vote(ctx, blogID, "agent", voter, direction) }
	score := func() voteScore { return readVoteScore(ctx, t, pool, "blog_post", blogID) }
	exec := func(sql string, args ...any) {
		t.Helper()
		_, err := pool.Exec(ctx, sql, args...)
		require.NoError(t, err)
	}

	require.NoError(t, vote("alice", "up"))
	require.NoError(t, vote("bob", "up"))
	require.Equal(t, voteScore{2, 0}, score(), "two upvotes")
	require.NoError(t, vote("alice", "down"))
	require.Equal(t, voteScore{1, 1}, score(), "a flip moves one vote across")
	require.NoError(t, vote("alice", "down"))
	require.Equal(t, voteScore{1, 1}, score(), "repeating a vote changes nothing")

	// Only confirmed votes count, whoever writes the vote row.
	exec(`UPDATE votes SET confirmed = false WHERE target_id = $1 AND voter_id = 'alice'`, blogID)
	require.Equal(t, voteScore{1, 0}, score(), "an unconfirmed vote is not counted")
	exec(`UPDATE votes SET confirmed = true WHERE target_id = $1 AND voter_id = 'alice'`, blogID)
	require.Equal(t, voteScore{1, 1}, score(), "a confirmed vote is counted again")
	exec(`INSERT INTO votes (target_type, target_id, voter_type, voter_id, direction, confirmed)
		VALUES ('blog_post', $1, 'agent', 'carol', 'up', false)`, blogID)
	require.Equal(t, voteScore{1, 1}, score(), "an unconfirmed insert is not counted")
	exec(`INSERT INTO votes (target_type, target_id, voter_type, voter_id, direction, confirmed)
		VALUES ('blog_post', $1, 'agent', 'dave', 'down', true)`, blogID)
	require.Equal(t, voteScore{1, 2}, score(), "a confirmed insert is counted")
	exec(`DELETE FROM votes WHERE target_id = $1 AND voter_id IN ('bob', 'dave')`, blogID)
	require.Equal(t, voteScore{0, 1}, score(), "a deleted vote is uncounted")

	// A vote whose transaction rolls back never counted.
	tx, err := pool.BeginTx(ctx)
	require.NoError(t, err)
	_, err = tx.Exec(ctx, `INSERT INTO votes (target_type, target_id, voter_type, voter_id, direction, confirmed)
		VALUES ('blog_post', $1, 'agent', 'erin', 'up', true)`, blogID)
	require.NoError(t, err)
	require.NoError(t, tx.Rollback(ctx))
	require.Equal(t, voteScore{0, 1}, score(), "a rolled-back vote is not counted")

	require.Equal(t, voteCount(ctx, t, pool, "blog_post", blogID), score(), "the score is its votes")
	require.Empty(t, readVoteScoreDrift(ctx, t, pool, nil, nil))
}

// Concurrent requests from one voter (double clicks, client retries) count once, and
// concurrent first votes do not fail on the unique vote key.
func TestBlogVoteScores_ConcurrentVotesByOneVoterCountOnce(t *testing.T) {
	pool, _ := newMigratedScratchDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	blogID := blogVoteTarget(ctx, t, pool, "concurrent")
	blogs := NewBlogPostRepository(pool)

	const racers = 8
	concurrently := func(voter, direction string) {
		t.Helper()
		var wg sync.WaitGroup
		errs := make(chan error, racers)
		start := make(chan struct{})
		for range racers {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				errs <- blogs.Vote(ctx, blogID, "agent", voter, direction)
			}()
		}
		close(start)
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
	require.Equal(t, voteScore{2, 2}, voteCount(ctx, t, pool, "blog_post", blogID))
	require.Equal(t, voteScore{2, 2}, readVoteScore(ctx, t, pool, "blog_post", blogID), "every voter counted exactly once")
	require.Empty(t, readVoteScoreDrift(ctx, t, pool, nil, nil))
}

// The drift check and the rebuild cover blog posts next to posts and replies, so one
// operator command (and the cutover's vote score step) reconciles every projected score.
func TestBlogVoteScores_DriftNamesBrokenBlogScoresAndTheRebuildRepairsThem(t *testing.T) {
	pool, _ := newMigratedScratchDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	healthyBlog, brokenBlog := blogVoteTarget(ctx, t, pool, "healthy"), blogVoteTarget(ctx, t, pool, "broken")
	brokenPost, _ := voteTargets(ctx, t, pool, "broken-post")
	blogs := NewBlogPostRepository(pool)
	require.NoError(t, blogs.Vote(ctx, healthyBlog, "agent", "v1", "up"))
	require.NoError(t, blogs.Vote(ctx, brokenBlog, "agent", "v1", "up"))
	require.NoError(t, blogs.Vote(ctx, brokenBlog, "agent", "v2", "down"))
	require.NoError(t, NewPostRepository(pool).Vote(ctx, brokenPost, "agent", "v1", "up"))

	// Stored values the votes do not support: the counters the old code drove negative.
	_, err := pool.Exec(ctx, `UPDATE blog_posts SET upvotes = -34, downvotes = 36 WHERE id = $1`, brokenBlog)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE blog_posts SET downvotes = NULL WHERE id = $1`, healthyBlog)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE posts SET upvotes = 7 WHERE id = $1`, brokenPost)
	require.NoError(t, err)

	drift := readVoteScoreDrift(ctx, t, pool, nil, nil)
	require.Len(t, drift, 3, "both broken blog posts and the broken post: %+v", drift)
	byID := map[string]voteScoreDrift{}
	for _, d := range drift {
		byID[d.targetID] = d
	}
	require.Equal(t, "blog_post", byID[brokenBlog].targetType)
	require.Equal(t, -34, *byID[brokenBlog].storedUp)
	require.Equal(t, 36, *byID[brokenBlog].storedDown)
	require.Equal(t, 1, byID[brokenBlog].voteUp)
	require.Equal(t, 1, byID[brokenBlog].voteDown)
	require.Equal(t, "blog_post", byID[healthyBlog].targetType)
	require.Nil(t, byID[healthyBlog].storedDown, "a NULL counter is drift, even when the other one is right")
	require.Equal(t, 1, *byID[healthyBlog].storedUp)
	require.Equal(t, "post", byID[brokenPost].targetType)

	require.Len(t, readVoteScoreDrift(ctx, t, pool, strPtr("blog_post"), nil), 2, "filtered by type")
	require.Len(t, readVoteScoreDrift(ctx, t, pool, strPtr("blog_post"), &brokenBlog), 1, "filtered by target")
	require.Empty(t, readVoteScoreDrift(ctx, t, pool, strPtr("post"), &brokenBlog), "a blog post is not a post")

	n, err := rebuildVoteScores(ctx, pool, strPtr("blog_post"), &brokenBlog)
	require.NoError(t, err)
	require.Equal(t, 1, n)
	require.Equal(t, voteScore{1, 1}, readVoteScore(ctx, t, pool, "blog_post", brokenBlog))
	require.Len(t, readVoteScoreDrift(ctx, t, pool, nil, nil), 2, "a one-target rebuild leaves the other broken targets alone")

	brokenBlogXmin := func() string {
		var x string
		require.NoError(t, pool.QueryRow(ctx, `SELECT xmin::text FROM blog_posts WHERE id = $1`, brokenBlog).Scan(&x))
		return x
	}()
	n, err = rebuildVoteScores(ctx, pool, nil, nil)
	require.NoError(t, err)
	require.Equal(t, 2, n, "the full rebuild repairs the blog post and the post")
	require.Equal(t, voteScore{1, 0}, readVoteScore(ctx, t, pool, "blog_post", healthyBlog))
	require.Equal(t, voteScore{1, 0}, readVoteScore(ctx, t, pool, "post", brokenPost))
	require.Empty(t, readVoteScoreDrift(ctx, t, pool, nil, nil))
	var xmin string
	require.NoError(t, pool.QueryRow(ctx, `SELECT xmin::text FROM blog_posts WHERE id = $1`, brokenBlog).Scan(&xmin))
	require.Equal(t, brokenBlogXmin, xmin, "a consistent blog post is not rewritten")

	n, err = rebuildVoteScores(ctx, pool, strPtr("blog_post"), nil)
	require.NoError(t, err)
	require.Equal(t, 0, n, "replaying the rebuild changes nothing")
}

// A rebuild and a blog vote that meet must not lose the vote, whichever holds the row first.
func TestBlogVoteScores_ARebuildAndAVoteNeverLoseEachOther(t *testing.T) {
	pool, _ := newMigratedScratchDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	blogID := blogVoteTarget(ctx, t, pool, "inflight")
	blogs := NewBlogPostRepository(pool)
	for i := range 3 {
		require.NoError(t, blogs.Vote(ctx, blogID, "agent", fmt.Sprintf("committed_%d", i), "up"))
	}
	breakScore := func() {
		_, err := pool.Exec(ctx, `UPDATE blog_posts SET upvotes = 50 WHERE id = $1`, blogID)
		require.NoError(t, err)
	}

	// 1. The vote is in flight: the rebuild waits for it, then counts it.
	breakScore()
	tx, err := pool.BeginTx(ctx)
	require.NoError(t, err)
	defer tx.Rollback(context.Background()) //nolint:errcheck
	_, err = tx.Exec(ctx, `INSERT INTO votes (target_type, target_id, voter_type, voter_id, direction, confirmed)
		VALUES ('blog_post', $1, 'agent', 'in_flight', 'up', true)`, blogID)
	require.NoError(t, err)
	done := make(chan error, 1)
	go func() { _, err := rebuildVoteScores(ctx, pool, strPtr("blog_post"), &blogID); done <- err }()
	waitForLockWait(ctx, t, pool, "%rebuild_vote_scores%", done)
	require.NoError(t, tx.Commit(ctx))
	require.NoError(t, <-done)
	require.Equal(t, voteScore{4, 0}, readVoteScore(ctx, t, pool, "blog_post", blogID), "the in-flight vote is counted")

	// 2. The rebuild is in flight: the vote waits for it, then lands on the rebuilt score.
	breakScore()
	tx2, err := pool.BeginTx(ctx)
	require.NoError(t, err)
	defer tx2.Rollback(context.Background()) //nolint:errcheck
	var repaired int
	require.NoError(t, tx2.QueryRow(ctx, `SELECT rebuild_vote_scores('blog_post', $1::uuid)`, blogID).Scan(&repaired))
	require.Equal(t, 1, repaired)
	voted := make(chan error, 1)
	go func() { voted <- blogs.Vote(ctx, blogID, "agent", "late_voter", "up") }()
	waitForLockWait(ctx, t, pool, "%INSERT INTO votes%", voted)
	require.NoError(t, tx2.Commit(ctx))
	require.NoError(t, <-voted)
	require.Equal(t, voteScore{5, 0}, readVoteScore(ctx, t, pool, "blog_post", blogID), "the late vote lands on the rebuilt score")
	require.Empty(t, readVoteScoreDrift(ctx, t, pool, nil, nil))
}
