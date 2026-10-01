package jobs

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

type reconcileCall struct {
	counter string
	limit   int
}

type fakeCounterReconciler struct {
	mu       sync.Mutex
	counters []string
	locked   bool // another instance holds the lock
	lockErr  error
	fail     map[string]error
	drift    map[string]int
	calls    []reconcileCall
	unlocks  int
}

func (f *fakeCounterReconciler) TryLock(context.Context) (func(), bool, error) {
	if f.lockErr != nil {
		return nil, false, f.lockErr
	}
	if f.locked {
		return nil, false, nil
	}
	return func() {
		f.mu.Lock()
		f.unlocks++
		f.mu.Unlock()
	}, true, nil
}

func (f *fakeCounterReconciler) runs() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.unlocks
}

func (f *fakeCounterReconciler) Counters() []string { return f.counters }

func (f *fakeCounterReconciler) Reconcile(_ context.Context, counter string, limit int) (int, int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, reconcileCall{counter, limit})
	if err := f.fail[counter]; err != nil {
		return 0, 0, err
	}
	n := f.drift[counter]
	return n, n, nil
}

func TestCounterReconcileJob_ReconcilesEveryCounterWithItsLimit(t *testing.T) {
	f := &fakeCounterReconciler{counters: []string{"vote_scores", "view_counts"}, drift: map[string]int{"view_counts": 3}}
	run, err := NewCounterReconcileJob(f, 7).RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	want := []reconcileCall{{"vote_scores", 7}, {"view_counts", 7}}
	if len(f.calls) != len(want) || f.calls[0] != want[0] || f.calls[1] != want[1] {
		t.Fatalf("calls = %v, want %v", f.calls, want)
	}
	if run.Busy || len(run.Counters) != 2 || run.Counters[1] != (CounterReconcileResult{"view_counts", 3, 3}) ||
		run.Counters[0] != (CounterReconcileResult{"vote_scores", 0, 0}) {
		t.Fatalf("run = %+v", run)
	}
	if f.unlocks != 1 {
		t.Fatalf("unlocks = %d, want 1", f.unlocks)
	}
}

func TestCounterReconcileJob_DefaultLimit(t *testing.T) {
	f := &fakeCounterReconciler{counters: []string{"room_activity"}}
	if _, err := NewCounterReconcileJob(f, 0).RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if len(f.calls) != 1 || f.calls[0].limit != DefaultCounterReconcileLimit {
		t.Fatalf("calls = %v, want the default limit %d", f.calls, DefaultCounterReconcileLimit)
	}
}

func TestCounterReconcileJob_AnotherInstanceHoldsTheLock(t *testing.T) {
	f := &fakeCounterReconciler{counters: []string{"vote_scores"}, locked: true}
	run, err := NewCounterReconcileJob(f, 10).RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if !run.Busy || len(f.calls) != 0 {
		t.Fatalf("run = %+v, calls = %v; want busy and nothing reconciled", run, f.calls)
	}
}

func TestCounterReconcileJob_ALockErrorIsReturned(t *testing.T) {
	f := &fakeCounterReconciler{counters: []string{"vote_scores"}, lockErr: errors.New("connection refused")}
	if _, err := NewCounterReconcileJob(f, 10).RunOnce(context.Background()); err == nil || len(f.calls) != 0 {
		t.Fatalf("err = %v, calls = %v; want the lock error and nothing reconciled", err, f.calls)
	}
}

// One counter that cannot be reconciled (its function missing, a statement timeout) must not
// leave the others drifting.
func TestCounterReconcileJob_OneFailingCounterDoesNotStopTheOthers(t *testing.T) {
	f := &fakeCounterReconciler{
		counters: []string{"vote_scores", "room_activity", "view_counts"},
		fail:     map[string]error{"room_activity": errors.New("statement timeout")},
		drift:    map[string]int{"view_counts": 2},
	}
	run, err := NewCounterReconcileJob(f, 10).RunOnce(context.Background())
	if err == nil {
		t.Fatal("RunOnce returned no error for the failing counter")
	}
	if len(f.calls) != 3 {
		t.Fatalf("calls = %v, want every counter tried", f.calls)
	}
	if len(run.Counters) != 2 || run.Counters[1] != (CounterReconcileResult{"view_counts", 2, 2}) {
		t.Fatalf("run = %+v, want the two counters that ran", run)
	}
	if f.unlocks != 1 {
		t.Fatalf("unlocks = %d, want 1", f.unlocks)
	}
}

func TestCounterReconcileJob_RunScheduledRunsAtOnceThenStopsWithItsContext(t *testing.T) {
	f := &fakeCounterReconciler{
		counters: []string{"vote_scores", "view_counts"},
		fail:     map[string]error{"vote_scores": errors.New("statement timeout")},
		drift:    map[string]int{"view_counts": 1},
	}
	job := NewCounterReconcileJob(f, 10)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		job.RunScheduled(ctx, time.Hour)
		close(done)
	}()
	deadline := time.Now().Add(5 * time.Second)
	for f.runs() == 0 {
		if time.Now().After(deadline) {
			t.Fatalf("RunScheduled did not run at start")
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatalf("RunScheduled did not stop with its context")
	}
	if n := f.runs(); n != 1 {
		t.Fatalf("runs = %d, want only the run at start within the hour", n)
	}
}
