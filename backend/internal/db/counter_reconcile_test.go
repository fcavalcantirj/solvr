package db

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// idx 77 step 5, periodic rebuilds: the counters the cutover reconciles once are reconciled
// again on a schedule, row by row. Triggers keep every counter in its write's own
// transaction, so drift comes only from outside them: a deploy window where the old API
// still moves a counter the new trigger also moves (slices 4/5), or direct SQL, which is
// how production migrations are applied. The full rebuild_*() locks every target row for
// its duration (about 14 s over all posts, replies and blog posts at 200k posts, slice 24),
// right for a cutover window and wrong on live traffic. The reconciler reads the drift and
// repairs each listed row with the one-row rebuild, which locks only that row (0.6-1.1 ms a
// row at that size, measured in slice 25's spike).

type reconcileFixture struct {
	driftedPost, quietPost   uuid.UUID
	driftedReply, quietReply uuid.UUID
	driftedRoom, quietRoom   uuid.UUID
}

// seedReconcileFixture seeds one consistent and one about-to-drift row for every counter.
func seedReconcileFixture(ctx context.Context, t *testing.T, pool *Pool) reconcileFixture {
	t.Helper()
	exec := func(sql string, args ...any) {
		t.Helper()
		_, err := pool.Exec(ctx, sql, args...)
		require.NoError(t, err, sql)
	}
	id := func(sql string, args ...any) uuid.UUID {
		t.Helper()
		var v uuid.UUID
		require.NoError(t, pool.QueryRow(ctx, sql, args...).Scan(&v), sql)
		return v
	}
	exec(`INSERT INTO agents (id, display_name, human_claimed_at, model) VALUES
		('cr-agent', 'Reconcile Agent', NOW(), 'model-x'), ('cr-quiet', 'Quiet Agent', NOW(), NULL)`)
	exec(`SELECT replay_agent_activations('cr-agent'), replay_agent_activations('cr-quiet')`)

	var f reconcileFixture
	post := func(title string) uuid.UUID {
		return id(`INSERT INTO posts (type, title, description, posted_by_type, posted_by_id, status)
			VALUES ('post', $1, 'A post the counter reconciler reads', 'agent', 'cr-agent', 'open') RETURNING id`, title)
	}
	f.driftedPost, f.quietPost = post("Reconcile drifted post"), post("Reconcile quiet post")
	reply := func(p uuid.UUID) uuid.UUID {
		return id(`INSERT INTO replies (post_id, author_type, author_id, body) VALUES ($1, 'agent', 'cr-agent', 'A reply') RETURNING id`, p)
	}
	f.driftedReply, f.quietReply = reply(f.driftedPost), reply(f.quietPost)
	vote := func(kind string, target uuid.UUID, voter, direction string) {
		exec(`INSERT INTO votes (target_type, target_id, voter_type, voter_id, direction, confirmed)
			VALUES ($1, $2, 'agent', $3, $4, true)`, kind, target, voter, direction)
	}
	vote("post", f.driftedPost, "v1", "up")
	vote("post", f.driftedPost, "v2", "up")
	vote("post", f.driftedPost, "v3", "down")
	vote("post", f.quietPost, "v1", "up")
	vote("reply", f.driftedReply, "v1", "up")
	vote("reply", f.quietReply, "v1", "up")
	exec(`INSERT INTO post_views (post_id, viewer_type, viewer_id) VALUES ($1, 'agent', 'w1'), ($1, 'agent', 'w2'), ($2, 'agent', 'w1')`,
		f.driftedPost, f.quietPost)

	f.driftedRoom = insertActivityRoom(ctx, t, pool, "reconcile-drifted")
	f.quietRoom = insertActivityRoom(ctx, t, pool, "reconcile-quiet")
	msgs := NewMessageRepository(pool)
	for i := 0; i < 3; i++ {
		postActivityMessage(ctx, t, msgs, f.driftedRoom, "cr-agent", "drifted room message")
	}
	postActivityMessage(ctx, t, msgs, f.quietRoom, "cr-agent", "quiet room message")

	for _, c := range cutoverCounters {
		require.Equal(t, 0, countRows(t, pool, ctx, `SELECT count(*) FROM `+c.drift+`()`), "seed is consistent: %s", c.drift)
	}
	return f
}

// breakEveryCounter moves one row of every counter away from its records, as an old API
// binary or a hand-written UPDATE would.
func breakEveryCounter(ctx context.Context, t *testing.T, pool *Pool, f reconcileFixture) {
	t.Helper()
	for _, q := range []struct {
		sql string
		arg any
	}{
		{`UPDATE posts SET upvotes = upvotes + 3, view_count = view_count + 5 WHERE id = $1`, f.driftedPost},
		{`UPDATE replies SET downvotes = downvotes + 2 WHERE id = $1`, f.driftedReply},
		{`UPDATE rooms SET message_count = message_count + 4 WHERE id = $1`, f.driftedRoom},
		{`UPDATE agents SET reputation = reputation + 7 WHERE id = $1`, "cr-agent"},
	} {
		_, err := pool.Exec(ctx, q.sql, q.arg)
		require.NoError(t, err, q.sql)
	}
}

func TestCounterReconciler_RepairsOnlyTheDriftedRowsWithoutWaitingForTheRest(t *testing.T) {
	pool, _ := newMigratedScratchDatabase(t)
	ctx := context.Background()
	f := seedReconcileFixture(ctx, t, pool)
	breakEveryCounter(ctx, t, pool, f)
	rec := NewCounterReconciler(pool)

	// Live traffic holds the consistent rows. A full rebuild would wait for every one of them;
	// a row-by-row repair never touches them.
	holder, err := pool.BeginTx(ctx)
	require.NoError(t, err)
	defer func() { _ = holder.Rollback(ctx) }()
	for _, q := range []struct {
		sql string
		arg any
	}{
		{`SELECT 1 FROM posts WHERE id = $1 FOR UPDATE`, f.quietPost},
		{`SELECT 1 FROM replies WHERE id = $1 FOR UPDATE`, f.quietReply},
		{`SELECT 1 FROM rooms WHERE id = $1 FOR UPDATE`, f.quietRoom},
		{`SELECT 1 FROM agents WHERE id = $1 FOR UPDATE`, "cr-quiet"},
	} {
		_, err := holder.Exec(ctx, q.sql, q.arg)
		require.NoError(t, err, q.sql)
	}

	want := map[string]int{"vote_scores": 2, "room_activity": 1, "view_counts": 1, "agent_reputation": 1}
	require.Len(t, rec.Counters(), len(want))
	for _, name := range rec.Counters() {
		c, cancel := context.WithTimeout(ctx, 10*time.Second)
		drifted, repaired, err := rec.Reconcile(c, name, 100)
		cancel()
		require.NoError(t, err, "%s must not wait for rows it does not repair", name)
		require.Equal(t, want[name], drifted, name)
		require.Equal(t, want[name], repaired, name)
	}
	require.NoError(t, holder.Commit(ctx))

	for _, c := range cutoverCounters {
		require.Equal(t, 0, countRows(t, pool, ctx, `SELECT count(*) FROM `+c.drift+`()`), c.drift)
	}
	row := func(sql string, arg any) int {
		t.Helper()
		return countRows(t, pool, ctx, sql, arg)
	}
	require.Equal(t, 2, row(`SELECT upvotes FROM posts WHERE id = $1`, f.driftedPost))
	require.Equal(t, 1, row(`SELECT downvotes FROM posts WHERE id = $1`, f.driftedPost))
	require.Equal(t, 2, row(`SELECT view_count FROM posts WHERE id = $1`, f.driftedPost))
	require.Equal(t, 0, row(`SELECT downvotes FROM replies WHERE id = $1`, f.driftedReply))
	require.Equal(t, 3, row(`SELECT message_count FROM rooms WHERE id = $1`, f.driftedRoom))
	require.Equal(t, 60, row(`SELECT reputation FROM agents WHERE id = $1`, "cr-agent"), "claim 50 + model 10")
	require.Equal(t, 1, row(`SELECT upvotes FROM posts WHERE id = $1`, f.quietPost))
	require.Equal(t, 1, row(`SELECT view_count FROM posts WHERE id = $1`, f.quietPost))
	require.Equal(t, 1, row(`SELECT message_count FROM rooms WHERE id = $1`, f.quietRoom))
	require.Equal(t, 50, row(`SELECT reputation FROM agents WHERE id = $1`, "cr-quiet"))

	// A replayed run finds nothing and counts nothing twice.
	for _, name := range rec.Counters() {
		drifted, repaired, err := rec.Reconcile(ctx, name, 100)
		require.NoError(t, err)
		require.Zero(t, drifted, name)
		require.Zero(t, repaired, name)
	}
	require.Equal(t, 2, row(`SELECT upvotes FROM posts WHERE id = $1`, f.driftedPost))
	require.Equal(t, 60, row(`SELECT reputation FROM agents WHERE id = $1`, "cr-agent"))
}

func TestCounterReconciler_ALimitBoundsOneRunAndTheNextRunContinues(t *testing.T) {
	pool, _ := newMigratedScratchDatabase(t)
	ctx := context.Background()
	f := seedReconcileFixture(ctx, t, pool)
	_, err := pool.Exec(ctx, `UPDATE posts SET view_count = view_count + 9 WHERE id IN ($1, $2)`, f.driftedPost, f.quietPost)
	require.NoError(t, err)
	require.Equal(t, 2, countRows(t, pool, ctx, `SELECT count(*) FROM view_count_drift()`))
	rec := NewCounterReconciler(pool)

	for _, want := range []int{1, 1, 0} {
		drifted, repaired, err := rec.Reconcile(ctx, "view_counts", 1)
		require.NoError(t, err)
		require.Equal(t, want, drifted)
		require.Equal(t, want, repaired)
	}
	require.Equal(t, 2, countRows(t, pool, ctx, `SELECT view_count FROM posts WHERE id = $1`, f.driftedPost))
	require.Equal(t, 1, countRows(t, pool, ctx, `SELECT view_count FROM posts WHERE id = $1`, f.quietPost))
}

// The reconciler covers exactly the counters the cutover reconciles, which
// TestCutoverCounters_CoverEveryDriftFunctionInTheSchema holds to the schema.
func TestCounterReconciler_ReconcilesExactlyTheCutoverCounters(t *testing.T) {
	pool, _ := newMigratedScratchDatabase(t)
	rec := NewCounterReconciler(pool)
	var steps []string
	for _, c := range cutoverCounters {
		steps = append(steps, c.step)
	}
	require.Equal(t, steps, rec.Counters())

	_, _, err := rec.Reconcile(context.Background(), "tag_usage", 10)
	require.ErrorContains(t, err, "tag_usage")
	_, _, err = rec.Reconcile(context.Background(), "view_counts", 0)
	require.Error(t, err, "a run that may repair nothing is a caller mistake")
}

// One API instance reconciles at a time, under its own lock: the search document sweep
// holding its lock does not stop it.
func TestCounterReconciler_OneInstanceAtATime(t *testing.T) {
	poolA, _ := newMigratedScratchDatabase(t)
	ctx := context.Background()
	poolB, err := NewPool(ctx, poolA.Config().ConnString())
	require.NoError(t, err)
	defer poolB.Close()
	a, b := NewCounterReconciler(poolA), NewCounterReconciler(poolB)

	unlockSweep, ok, err := NewSearchDocumentQueue(poolB).TryLockSweep(ctx)
	require.NoError(t, err)
	require.True(t, ok)
	defer unlockSweep()

	unlockA, ok, err := a.TryLock(ctx)
	require.NoError(t, err)
	require.True(t, ok, "the sweep's lock is not the reconciler's")
	_, ok, err = b.TryLock(ctx)
	require.NoError(t, err)
	require.False(t, ok, "a second instance waits for the next run")
	unlockA()
	unlockA()

	unlockB, ok, err := b.TryLock(ctx)
	require.NoError(t, err)
	require.True(t, ok)
	unlockB()
}

// A vote in flight on a drifted post is counted by the repair once it commits: the
// reconciler reads the drift without locks, but the one-row rebuild locks the row before it
// recounts, so its value is never older than the votes committed when it wrote.
func TestCounterReconciler_AVoteInFlightIsCountedByTheRepair(t *testing.T) {
	pool, _ := newMigratedScratchDatabase(t)
	ctx := context.Background()
	f := seedReconcileFixture(ctx, t, pool)
	breakEveryCounter(ctx, t, pool, f)
	rec := NewCounterReconciler(pool)

	voter, err := pool.BeginTx(ctx)
	require.NoError(t, err)
	defer func() { _ = voter.Rollback(ctx) }()
	_, err = voter.Exec(ctx, `INSERT INTO votes (target_type, target_id, voter_type, voter_id, direction, confirmed)
		VALUES ('post', $1, 'agent', 'in-flight', 'up', true)`, f.driftedPost)
	require.NoError(t, err)

	type outcome struct {
		drifted, repaired int
		err               error
	}
	done := make(chan outcome, 1)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		c, cancel := context.WithTimeout(ctx, 20*time.Second)
		defer cancel()
		d, r, err := rec.Reconcile(c, "vote_scores", 100)
		done <- outcome{d, r, err}
	}()
	deadline := time.Now().Add(10 * time.Second)
	for countRows(t, pool, ctx, `SELECT count(*) FROM pg_stat_activity
		WHERE datname = current_database() AND wait_event_type = 'Lock'`) == 0 {
		require.True(t, time.Now().Before(deadline), "the repair never waited for the in-flight vote")
		time.Sleep(20 * time.Millisecond)
	}
	require.NoError(t, voter.Commit(ctx))
	wg.Wait()
	got := <-done
	require.NoError(t, got.err)
	require.Equal(t, 2, got.drifted)
	require.Equal(t, 2, got.repaired)

	require.Equal(t, 3, countRows(t, pool, ctx, `SELECT upvotes FROM posts WHERE id = $1`, f.driftedPost), "v1, v2 and the in-flight vote")
	require.Equal(t, 0, countRows(t, pool, ctx, `SELECT count(*) FROM vote_score_drift()`))
}
