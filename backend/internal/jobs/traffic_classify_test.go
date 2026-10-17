package jobs

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// fakeTrafficRules records the windows it was asked to judge.
type fakeTrafficRules struct {
	err     error
	windows [][2]time.Time
	mu      sync.Mutex
	busy    bool
}

func (f *fakeTrafficRules) windowsSnapshot() [][2]time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([][2]time.Time(nil), f.windows...)
}

func (f *fakeTrafficRules) TryLock(context.Context) (unlock func(), ok bool, err error) {
	if f.busy {
		return nil, false, nil
	}
	return func() {}, true, nil
}

func (f *fakeTrafficRules) ApplyRules(_ context.Context, from, to time.Time) (map[string]int, error) {
	f.mu.Lock()
	f.windows = append(f.windows, [2]time.Time{from, to})
	f.mu.Unlock()
	return map[string]int{"fetched_without_copy": 2}, f.err
}

// C3 (REVIEW-r1): the rules run on live rows without a manual call, every period over a
// trailing window that is longer than a flow takes to finish (its fetch arrives 30-59 s after
// the start).
func TestTrafficClassifyJob_JudgesTheTrailingWindow(t *testing.T) {
	now := time.Date(2026, 10, 17, 10, 7, 0, 0, time.UTC)
	rules := &fakeTrafficRules{}
	job := NewTrafficClassifyJob(rules)
	job.now = func() time.Time { return now }

	run, err := job.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if run.Busy || run.Rules["fetched_without_copy"] != 2 {
		t.Fatalf("run = %+v", run)
	}
	if len(rules.windows) != 1 || !rules.windows[0][0].Equal(now.Add(-TrafficClassifyWindow)) || !rules.windows[0][1].Equal(now) {
		t.Fatalf("windows = %v; want one [now-2h, now)", rules.windows)
	}
	if DefaultTrafficClassifyInterval != 15*time.Minute || TrafficClassifyWindow != 2*time.Hour {
		t.Fatalf("interval %s, window %s; REVIEW-r1 C3 names 15 min over 2 h", DefaultTrafficClassifyInterval, TrafficClassifyWindow)
	}
}

// One run at a time across instances: a held lock skips the run.
func TestTrafficClassifyJob_SkipsWhileAnotherRunHoldsTheLock(t *testing.T) {
	rules := &fakeTrafficRules{busy: true}
	run, err := NewTrafficClassifyJob(rules).RunOnce(context.Background())
	if err != nil || !run.Busy || len(rules.windows) != 0 {
		t.Fatalf("run = %+v, err = %v, windows = %v; want a skipped run", run, err, rules.windows)
	}
}

func TestTrafficClassifyJob_ReportsAFailedRun(t *testing.T) {
	rules := &fakeTrafficRules{err: errors.New("database gone")}
	if _, err := NewTrafficClassifyJob(rules).RunOnce(context.Background()); err == nil {
		t.Fatal("a failed run must say so")
	}
}

// RunScheduled runs at once, then every interval, and stops with its context.
func TestTrafficClassifyJob_RunsAtOnceThenOnTheInterval(t *testing.T) {
	rules := &fakeTrafficRules{}
	job := NewTrafficClassifyJob(rules)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		job.RunScheduled(ctx, 20*time.Millisecond)
		close(done)
	}()
	deadline := time.After(2 * time.Second)
	for {
		if len(rules.windowsSnapshot()) >= 3 {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("only %d runs in 2 s", len(rules.windowsSnapshot()))
		case <-time.After(5 * time.Millisecond):
		}
	}
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("RunScheduled did not stop with its context")
	}
}
