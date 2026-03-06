package jobs

import (
	"context"
	"errors"
	"testing"
	"time"
)

// mockAutoSolveWarner implements AutoSolveWarner for testing.
type mockAutoSolveWarner struct {
	warnedCount int64
	err         error
}

func (m *mockAutoSolveWarner) WarnProblemsApproachingAutoSolve(ctx context.Context, warningThreshold, solveThreshold time.Duration) (int64, error) {
	if m.err != nil {
		return 0, m.err
	}
	return m.warnedCount, nil
}

// mockAutoSolver implements AutoSolver for testing.
type mockAutoSolver struct {
	solvedCount int64
	err         error
}

func (m *mockAutoSolver) AutoSolveProblems(ctx context.Context, olderThan time.Duration) (int64, error) {
	if m.err != nil {
		return 0, m.err
	}
	return m.solvedCount, nil
}

// TestAutoSolveJob_DefaultConstants tests the default configuration constants.
func TestAutoSolveJob_DefaultConstants(t *testing.T) {
	if DefaultAutoSolveThreshold != 14*24*time.Hour {
		t.Errorf("DefaultAutoSolveThreshold = %v, want 14 days", DefaultAutoSolveThreshold)
	}
	if DefaultAutoSolveWarningThreshold != 7*24*time.Hour {
		t.Errorf("DefaultAutoSolveWarningThreshold = %v, want 7 days", DefaultAutoSolveWarningThreshold)
	}
	if DefaultAutoSolveInterval != 24*time.Hour {
		t.Errorf("DefaultAutoSolveInterval = %v, want 24 hours", DefaultAutoSolveInterval)
	}
}

// TestAutoSolveJob_AutoSolveProblems tests that RunOnce auto-solves problems.
func TestAutoSolveJob_AutoSolveProblems(t *testing.T) {
	warner := &mockAutoSolveWarner{warnedCount: 0}
	solver := &mockAutoSolver{solvedCount: 3}

	job := NewAutoSolveJob(warner, solver)
	result := job.RunOnce(context.Background())

	if result.Solved != 3 {
		t.Errorf("RunOnce() solved = %d, want 3", result.Solved)
	}
	if result.Warned != 0 {
		t.Errorf("RunOnce() warned = %d, want 0", result.Warned)
	}
}

// TestAutoSolveJob_WarnBeforeAutoSolve tests that RunOnce sends warning notifications.
func TestAutoSolveJob_WarnBeforeAutoSolve(t *testing.T) {
	warner := &mockAutoSolveWarner{warnedCount: 2}
	solver := &mockAutoSolver{solvedCount: 0}

	job := NewAutoSolveJob(warner, solver)
	result := job.RunOnce(context.Background())

	if result.Warned != 2 {
		t.Errorf("RunOnce() warned = %d, want 2", result.Warned)
	}
	if result.Solved != 0 {
		t.Errorf("RunOnce() solved = %d, want 0", result.Solved)
	}
}

// TestAutoSolveJob_BothWarnAndAutoSolve tests both phases running together.
func TestAutoSolveJob_BothWarnAndAutoSolve(t *testing.T) {
	warner := &mockAutoSolveWarner{warnedCount: 1}
	solver := &mockAutoSolver{solvedCount: 2}

	job := NewAutoSolveJob(warner, solver)
	result := job.RunOnce(context.Background())

	if result.Warned != 1 {
		t.Errorf("RunOnce() warned = %d, want 1", result.Warned)
	}
	if result.Solved != 2 {
		t.Errorf("RunOnce() solved = %d, want 2", result.Solved)
	}
}

// TestAutoSolveJob_NoProblemsToAutoSolve tests RunOnce when nothing needs auto-solving.
func TestAutoSolveJob_NoProblemsToAutoSolve(t *testing.T) {
	warner := &mockAutoSolveWarner{warnedCount: 0}
	solver := &mockAutoSolver{solvedCount: 0}

	job := NewAutoSolveJob(warner, solver)
	result := job.RunOnce(context.Background())

	if result.Warned != 0 {
		t.Errorf("RunOnce() warned = %d, want 0", result.Warned)
	}
	if result.Solved != 0 {
		t.Errorf("RunOnce() solved = %d, want 0", result.Solved)
	}
}

// TestAutoSolveJob_ErrorsDoNotCrash tests that errors in one step don't prevent others.
func TestAutoSolveJob_ErrorsDoNotCrash(t *testing.T) {
	t.Run("warner error does not block solver", func(t *testing.T) {
		warner := &mockAutoSolveWarner{err: errors.New("db error")}
		solver := &mockAutoSolver{solvedCount: 2}

		job := NewAutoSolveJob(warner, solver)
		result := job.RunOnce(context.Background())

		if result.Warned != 0 {
			t.Errorf("RunOnce() warned = %d, want 0 (error case)", result.Warned)
		}
		if result.Solved != 2 {
			t.Errorf("RunOnce() solved = %d, want 2", result.Solved)
		}
	})

	t.Run("solver error does not block warner", func(t *testing.T) {
		warner := &mockAutoSolveWarner{warnedCount: 3}
		solver := &mockAutoSolver{err: errors.New("db error")}

		job := NewAutoSolveJob(warner, solver)
		result := job.RunOnce(context.Background())

		if result.Warned != 3 {
			t.Errorf("RunOnce() warned = %d, want 3", result.Warned)
		}
		if result.Solved != 0 {
			t.Errorf("RunOnce() solved = %d, want 0 (error case)", result.Solved)
		}
	})
}

// TestAutoSolveJob_RunScheduled tests scheduled execution and context cancellation.
func TestAutoSolveJob_RunScheduled(t *testing.T) {
	warner := &mockAutoSolveWarner{warnedCount: 0}
	solver := &mockAutoSolver{solvedCount: 1}

	job := NewAutoSolveJob(warner, solver)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	go func() {
		job.RunScheduled(ctx, 10*time.Millisecond)
		close(done)
	}()

	// Wait for at least one execution
	time.Sleep(50 * time.Millisecond)

	// Stop the job
	cancel()

	// Wait for job to finish
	select {
	case <-done:
		// Success - job stopped
	case <-time.After(1 * time.Second):
		t.Fatal("job did not stop within timeout")
	}
}
