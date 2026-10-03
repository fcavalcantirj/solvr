package jobs_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/jobs"
)

// Drift written outside the counters' triggers (here a hand-written UPDATE, as a production
// migration through /admin/query would be) is repaired with no operator run, once however
// many API instances reconcile, and a replay repairs nothing.
func TestCounterReconcileJob_DriftIsRepairedOnceAcrossInstances(t *testing.T) {
	url := newMigratedScratchURL(t, "solvr_counter_reconcile_")
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	open := func() *db.Pool {
		t.Helper()
		pool, err := db.NewPool(ctx, url)
		if err != nil {
			t.Fatalf("pool: %v", err)
		}
		t.Cleanup(pool.Close)
		return pool
	}
	poolA, poolB := open(), open()
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := poolA.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("%s: %v", sql, err)
		}
	}
	count := func(sql string) int {
		t.Helper()
		var n int
		if err := poolA.QueryRow(ctx, sql).Scan(&n); err != nil {
			t.Fatalf("%s: %v", sql, err)
		}
		return n
	}

	exec(`INSERT INTO agents (id, display_name, human_claimed_at) VALUES ('reconcile_agent', 'Reconcile Agent', NOW())`)
	exec(`SELECT replay_agent_activations('reconcile_agent')`)
	for i := 0; i < 3; i++ {
		exec(`WITH p AS (
			INSERT INTO posts (type, title, description, posted_by_type, posted_by_id, status)
			VALUES ('post', 'Reconcile post ' || $1::int, 'A post whose view count drifts', 'agent', 'reconcile_agent', 'open')
			RETURNING id)
			INSERT INTO post_views (post_id, viewer_type, viewer_id) SELECT id, 'agent', 'viewer' FROM p`, i)
	}
	exec(`UPDATE posts SET view_count = view_count + 4`)
	exec(`UPDATE agents SET reputation = reputation + 7 WHERE id = 'reconcile_agent'`)
	if v, r := count(`SELECT count(*) FROM view_count_drift()`), count(`SELECT count(*) FROM agent_reputation_drift()`); v != 3 || r != 1 {
		t.Fatalf("precondition: view drift %d, reputation drift %d; want 3 and 1", v, r)
	}

	jobA := jobs.NewCounterReconcileJob(db.NewCounterReconciler(poolA), jobs.DefaultCounterReconcileLimit)
	jobB := jobs.NewCounterReconcileJob(db.NewCounterReconciler(poolB), jobs.DefaultCounterReconcileLimit)

	// While another instance holds the reconcile lock, a run does nothing.
	unlock, ok, err := db.NewCounterReconciler(poolB).TryLock(ctx)
	if err != nil || !ok {
		t.Fatalf("take the lock: ok=%v err=%v", ok, err)
	}
	if run, err := jobA.RunOnce(ctx); err != nil || !run.Busy || len(run.Counters) != 0 {
		t.Fatalf("run while locked = %+v, %v; want busy", run, err)
	}
	unlock()

	type outcome struct {
		run jobs.CounterReconcileRun
		err error
	}
	results := make([]outcome, 2)
	var wg sync.WaitGroup
	for i, job := range []*jobs.CounterReconcileJob{jobA, jobB} {
		wg.Add(1)
		go func(i int, job *jobs.CounterReconcileJob) {
			defer wg.Done()
			run, err := job.RunOnce(ctx)
			results[i] = outcome{run, err}
		}(i, job)
	}
	wg.Wait()
	repaired := map[string]int{}
	for _, o := range results {
		if o.err != nil {
			t.Fatalf("concurrent run: %v", o.err)
		}
		for _, c := range o.run.Counters {
			repaired[c.Counter] += c.Repaired
		}
	}
	if repaired["view_counts"] != 3 || repaired["agent_reputation"] != 1 || repaired["vote_scores"] != 0 || repaired["room_activity"] != 0 {
		t.Fatalf("repaired across both instances = %v; want each drifted row repaired once", repaired)
	}
	if v, r := count(`SELECT count(*) FROM view_count_drift()`), count(`SELECT count(*) FROM agent_reputation_drift()`); v != 0 || r != 0 {
		t.Fatalf("drift after the runs: views %d, reputation %d", v, r)
	}
	if n := count(`SELECT sum(view_count)::int FROM posts`); n != 3 {
		t.Fatalf("view counts sum to %d, want one view per post", n)
	}
	if n := count(`SELECT reputation FROM agents WHERE id = 'reconcile_agent'`); n != 50 {
		t.Fatalf("reputation = %d, want the claim grant's 50", n)
	}

	// A replay, on either instance, repairs nothing.
	for _, job := range []*jobs.CounterReconcileJob{jobA, jobB} {
		run, err := job.RunOnce(ctx)
		if err != nil || run.Busy {
			t.Fatalf("replay = %+v, %v", run, err)
		}
		for _, c := range run.Counters {
			if c.Drifted != 0 || c.Repaired != 0 {
				t.Fatalf("replay = %+v; want nothing drifted or repaired", run)
			}
		}
	}
}
