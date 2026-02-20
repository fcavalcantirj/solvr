// Package jobs provides background job implementations.
package jobs

import (
	"context"
	"log"
	"time"
)

// Default stale content job configuration values.
const (
	// DefaultStaleApproachThreshold is how long an approach can be in 'working'
	// or 'starting' status before it is automatically abandoned (30 days).
	DefaultStaleApproachThreshold = 30 * 24 * time.Hour

	// DefaultDormantPostThreshold is how long an open problem with zero
	// approaches can exist before being marked dormant (60 days).
	DefaultDormantPostThreshold = 60 * 24 * time.Hour

	// DefaultWarningThreshold is how long an approach must be stale before
	// a warning notification is sent (23 days — 7 days before abandonment).
	DefaultWarningThreshold = 23 * 24 * time.Hour

	// DefaultStaleContentInterval is how often the stale content scan runs.
	DefaultStaleContentInterval = 24 * time.Hour
)

// StaleApproachUpdater abandons approaches that have been stale too long.
type StaleApproachUpdater interface {
	AbandonStaleApproaches(ctx context.Context, olderThan time.Duration) (int64, error)
}

// StaleApproachWarner sends warning notifications for approaches approaching abandonment.
type StaleApproachWarner interface {
	WarnApproachesApproachingAbandonment(ctx context.Context, warningThreshold, abandonThreshold time.Duration) (int64, error)
}

// DormantPostUpdater marks open problems with no approaches as dormant.
type DormantPostUpdater interface {
	MarkDormantPosts(ctx context.Context, olderThan time.Duration) (int64, error)
}

// StaleContentResult holds the results of a single stale content job run.
type StaleContentResult struct {
	Abandoned int64
	Warned    int64
	Dormant   int64
}

// StaleContentJob handles periodic cleanup of stale content:
// 1. Warns approach authors 7 days before auto-abandonment
// 2. Abandons approaches in 'working'/'starting' status for 30+ days
// 3. Marks open problems with zero approaches as dormant after 60 days
type StaleContentJob struct {
	updater StaleApproachUpdater
	warner  StaleApproachWarner
	dormant DormantPostUpdater
}

// NewStaleContentJob creates a new stale content cleanup job.
func NewStaleContentJob(
	updater StaleApproachUpdater,
	warner StaleApproachWarner,
	dormant DormantPostUpdater,
) *StaleContentJob {
	return &StaleContentJob{
		updater: updater,
		warner:  warner,
		dormant: dormant,
	}
}

// RunOnce executes the stale content cleanup steps in order:
// 1. Send warnings for approaches approaching abandonment (23-30 days stale)
// 2. Abandon approaches that are 30+ days stale
// 3. Mark dormant problems that are 60+ days old with zero approaches
// Each step is independent — errors in one step do not prevent others.
func (j *StaleContentJob) RunOnce(ctx context.Context) StaleContentResult {
	var result StaleContentResult

	// Step 1: Send warning notifications (23-30 day window)
	warned, err := j.warner.WarnApproachesApproachingAbandonment(
		ctx, DefaultWarningThreshold, DefaultStaleApproachThreshold,
	)
	if err != nil {
		log.Printf("Stale content job: failed to send warnings: %v", err)
	} else {
		result.Warned = warned
	}

	// Step 2: Abandon stale approaches (30+ days)
	abandoned, err := j.updater.AbandonStaleApproaches(ctx, DefaultStaleApproachThreshold)
	if err != nil {
		log.Printf("Stale content job: failed to abandon approaches: %v", err)
	} else {
		result.Abandoned = abandoned
	}

	// Step 3: Mark dormant posts (60+ days, no approaches)
	dormant, err := j.dormant.MarkDormantPosts(ctx, DefaultDormantPostThreshold)
	if err != nil {
		log.Printf("Stale content job: failed to mark dormant posts: %v", err)
	} else {
		result.Dormant = dormant
	}

	return result
}

// RunScheduled runs the stale content job on a schedule.
// It runs immediately on start, then repeats at the given interval.
// The job stops when the context is cancelled.
func (j *StaleContentJob) RunScheduled(ctx context.Context, interval time.Duration) {
	// Run immediately on start
	result := j.RunOnce(ctx)
	logStaleContentResult(result)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("Stale content job stopped")
			return
		case <-ticker.C:
			result := j.RunOnce(ctx)
			logStaleContentResult(result)
		}
	}
}

func logStaleContentResult(result StaleContentResult) {
	if result.Warned > 0 || result.Abandoned > 0 || result.Dormant > 0 {
		log.Printf("Stale content job: %d warned, %d abandoned, %d dormant",
			result.Warned, result.Abandoned, result.Dormant)
	}
}
