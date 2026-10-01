package jobs

import (
	"context"
	"errors"
	"log"
	"time"
)

// DefaultCounterReconcileInterval is how often the stored counters are reconciled.
const DefaultCounterReconcileInterval = time.Hour

// DefaultCounterReconcileLimit is the most rows of one counter a run repairs: about 5 s of
// one-row repairs at 1.1 ms a row (vote scores at 200k posts, idx 77 slice 25). Rows past it
// wait for the next run.
const DefaultCounterReconcileLimit = 5000

// CounterReconciler lists the stored counters and repairs the rows whose stored value has
// drifted from their records. Implemented by db.CounterReconciler.
type CounterReconciler interface {
	TryLock(ctx context.Context) (unlock func(), ok bool, err error)
	Counters() []string
	Reconcile(ctx context.Context, counter string, limit int) (drifted, repaired int, err error)
}

// CounterReconcileResult is what one run did to one counter.
type CounterReconcileResult struct {
	Counter  string
	Drifted  int // rows its drift function listed, at most the limit
	Repaired int // rows the one-row rebuild rewrote
}

// CounterReconcileRun is what one run did.
type CounterReconcileRun struct {
	Counters []CounterReconcileResult // the counters reconciled without an error
	Busy     bool                     // another instance (or run) holds the reconcile lock
}

// CounterReconcileJob is the periodic rebuild of the stored counters (idx 77 step 5): scores,
// view counts, room activity and agent reputation drifted from their records outside the
// triggers that keep them (a deploy window, direct SQL) come back without an operator run.
// One instance reconciles at a time; a replayed run repairs nothing twice.
type CounterReconcileJob struct {
	reconciler CounterReconciler
	limit      int
}

// NewCounterReconcileJob creates the job; a limit <= 0 uses the default.
func NewCounterReconcileJob(reconciler CounterReconciler, limit int) *CounterReconcileJob {
	if limit <= 0 {
		limit = DefaultCounterReconcileLimit
	}
	return &CounterReconcileJob{reconciler: reconciler, limit: limit}
}

// RunOnce reconciles every counter, up to the limit each. A counter that fails is reported
// in the returned error and does not stop the others.
func (j *CounterReconcileJob) RunOnce(ctx context.Context) (CounterReconcileRun, error) {
	var run CounterReconcileRun
	unlock, ok, err := j.reconciler.TryLock(ctx)
	if err != nil {
		return run, err
	}
	if !ok {
		run.Busy = true
		return run, nil
	}
	defer unlock()

	var errs []error
	for _, counter := range j.reconciler.Counters() {
		if err := ctx.Err(); err != nil {
			errs = append(errs, err)
			break
		}
		drifted, repaired, err := j.reconciler.Reconcile(ctx, counter, j.limit)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		run.Counters = append(run.Counters, CounterReconcileResult{Counter: counter, Drifted: drifted, Repaired: repaired})
	}
	return run, errors.Join(errs...)
}

// RunScheduled runs the job at once, then every interval until ctx is canceled.
func (j *CounterReconcileJob) RunScheduled(ctx context.Context, interval time.Duration) {
	j.runLogged(ctx)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			log.Println("Counter reconcile job stopped")
			return
		case <-ticker.C:
			j.runLogged(ctx)
		}
	}
}

func (j *CounterReconcileJob) runLogged(ctx context.Context) {
	run, err := j.RunOnce(ctx)
	if err != nil {
		log.Printf("Counter reconcile: %v", err)
	}
	for _, c := range run.Counters {
		if c.Drifted > 0 {
			log.Printf("Counter reconcile: %s %d drifted, %d repaired", c.Counter, c.Drifted, c.Repaired)
		}
	}
}
