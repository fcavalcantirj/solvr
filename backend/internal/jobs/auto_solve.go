// Package jobs provides background job implementations.
package jobs

import (
	"context"
	"log"
	"time"
)

// Default auto-solve job configuration values.
const (
	// DefaultAutoSolveThreshold is how long a succeeded approach must exist
	// before its problem is auto-solved (14 days).
	DefaultAutoSolveThreshold = 14 * 24 * time.Hour

	// DefaultAutoSolveWarningThreshold is when the warning is sent
	// (7 days after succeeded approach — 7 days before auto-solve).
	DefaultAutoSolveWarningThreshold = 7 * 24 * time.Hour

	// DefaultAutoSolveInterval is how often the auto-solve scan runs.
	DefaultAutoSolveInterval = 24 * time.Hour
)

// AutoSolveWarner sends warning notifications for problems approaching auto-solve.
type AutoSolveWarner interface {
	WarnProblemsApproachingAutoSolve(ctx context.Context, warningThreshold, solveThreshold time.Duration) (int64, error)
}

// AutoSolver auto-solves problems with old succeeded approaches.
type AutoSolver interface {
	AutoSolveProblems(ctx context.Context, olderThan time.Duration) (int64, error)
}

// AutoSolveResult holds the results of a single auto-solve job run.
type AutoSolveResult struct {
	Warned int64
	Solved int64
}

// AutoSolveJob handles periodic auto-solving of problems:
// 1. Warns problem owners 7 days before auto-solve
// 2. Auto-solves problems with succeeded approaches older than 14 days
type AutoSolveJob struct {
	warner AutoSolveWarner
	solver AutoSolver
}

// NewAutoSolveJob creates a new auto-solve job.
func NewAutoSolveJob(warner AutoSolveWarner, solver AutoSolver) *AutoSolveJob {
	return &AutoSolveJob{
		warner: warner,
		solver: solver,
	}
}

// RunOnce executes the auto-solve steps in order:
// 1. Send warnings for problems approaching auto-solve (7-14 days)
// 2. Auto-solve problems with succeeded approaches older than 14 days
// Each step is independent — errors in one step do not prevent others.
func (j *AutoSolveJob) RunOnce(ctx context.Context) AutoSolveResult {
	var result AutoSolveResult

	// Step 1: Send warning notifications (7-14 day window)
	warned, err := j.warner.WarnProblemsApproachingAutoSolve(
		ctx, DefaultAutoSolveWarningThreshold, DefaultAutoSolveThreshold,
	)
	if err != nil {
		log.Printf("Auto-solve job: failed to send warnings: %v", err)
	} else {
		result.Warned = warned
	}

	// Step 2: Auto-solve problems (14+ days)
	solved, err := j.solver.AutoSolveProblems(ctx, DefaultAutoSolveThreshold)
	if err != nil {
		log.Printf("Auto-solve job: failed to auto-solve problems: %v", err)
	} else {
		result.Solved = solved
	}

	return result
}

// RunScheduled runs the auto-solve job on a schedule.
// It runs immediately on start, then repeats at the given interval.
// The job stops when the context is cancelled.
func (j *AutoSolveJob) RunScheduled(ctx context.Context, interval time.Duration) {
	// Run immediately on start
	result := j.RunOnce(ctx)
	logAutoSolveResult(result)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("Auto-solve job stopped")
			return
		case <-ticker.C:
			result := j.RunOnce(ctx)
			logAutoSolveResult(result)
		}
	}
}

func logAutoSolveResult(result AutoSolveResult) {
	if result.Warned > 0 || result.Solved > 0 {
		log.Printf("Auto-solve job: %d warned, %d solved", result.Warned, result.Solved)
	}
}
