package db

import (
	"context"
	"fmt"
)

// counterReconcileLock names the session advisory lock one API instance holds while it
// reconciles the stored counters.
const counterReconcileLock = "solvr:counter_reconcile"

// CounterReconciler repairs the stored counters row by row, without an operator (idx 77
// step 5, periodic rebuilds). It covers the counters the cutover reconciles
// (cutoverCounters): scores, view counts, room activity and agent reputation.
//
// Triggers keep each counter in its write's own transaction, so drift comes only from
// outside them: a deploy window in which the old API still moves a counter the new trigger
// also moves, or direct SQL (production migrations go through /admin/query). A run reads a
// counter's drift function, which takes no lock, and repairs each row it lists with the
// one-row rebuild, which locks that row and recounts it once the writes in flight on it have
// committed. A row that is consistent again by then is not rewritten and not counted, so a
// replayed or concurrent run repairs nothing twice.
//
// The full rebuild_*() stays the cutover's and the operator's (REBUILD PATH in each
// migration): it locks every target row while it runs, about 14 s over all posts, replies
// and blog posts at 200k posts (idx 77 slice 24). A one-row repair cost 0.6-1.1 ms at that
// size (slice 25).
type CounterReconciler struct {
	pool *Pool
}

// NewCounterReconciler creates a CounterReconciler.
func NewCounterReconciler(pool *Pool) *CounterReconciler {
	return &CounterReconciler{pool: pool}
}

// TryLock takes the reconcile lock (see tryAdvisoryLock); ok is false while another session
// holds it.
func (r *CounterReconciler) TryLock(ctx context.Context) (unlock func(), ok bool, err error) {
	return tryAdvisoryLock(ctx, r.pool, counterReconcileLock)
}

// Counters names the stored counters in the order a run reconciles them.
func (r *CounterReconciler) Counters() []string {
	names := make([]string, len(cutoverCounters))
	for i, c := range cutoverCounters {
		names[i] = c.step
	}
	return names
}

// Reconcile repairs up to limit drifted rows of one counter, in its drift function's order.
// drifted is how many rows the drift function listed (at most limit), repaired how many of
// them the one-row rebuild rewrote. Each repair is a statement of its own, so a row stays
// locked only while it is recounted.
func (r *CounterReconciler) Reconcile(ctx context.Context, counter string, limit int) (drifted, repaired int, err error) {
	if limit <= 0 {
		return 0, 0, fmt.Errorf("reconcile %s: limit must be positive, got %d", counter, limit)
	}
	var c *cutoverCounter
	for i := range cutoverCounters {
		if cutoverCounters[i].step == counter {
			c = &cutoverCounters[i]
		}
	}
	if c == nil {
		return 0, 0, fmt.Errorf("reconcile: %q is not a stored counter", counter)
	}

	rows, err := r.pool.Query(ctx, `SELECT `+c.keys+` FROM `+c.drift+`() LIMIT $1`, limit)
	if err != nil {
		return 0, 0, fmt.Errorf("reconcile %s: %s(): %w", counter, c.drift, err)
	}
	var keys [][]any
	for rows.Next() {
		k, err := rows.Values()
		if err != nil {
			rows.Close()
			return 0, 0, fmt.Errorf("reconcile %s: %s(): %w", counter, c.drift, err)
		}
		keys = append(keys, k)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, 0, fmt.Errorf("reconcile %s: %s(): %w", counter, c.drift, err)
	}

	for _, k := range keys {
		var n int32
		if err := r.pool.QueryRow(ctx, `SELECT `+c.rebuildOne, k...).Scan(&n); err != nil {
			return len(keys), repaired, fmt.Errorf("reconcile %s: %s %v: %w", counter, c.rebuild, k, err)
		}
		repaired += int(n)
	}
	return len(keys), repaired, nil
}
