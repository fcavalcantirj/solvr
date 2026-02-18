// Package jobs provides background job implementations.
package jobs

import (
	"context"
	"log"
	"time"
)

// Default crystallization job configuration values.
const (
	// DefaultCrystallizationInterval is how often the crystallization scan runs.
	DefaultCrystallizationInterval = 24 * time.Hour

	// DefaultCrystallizationStabilityPeriod is how long a solved problem must be
	// unchanged before it becomes eligible for crystallization.
	DefaultCrystallizationStabilityPeriod = 7 * 24 * time.Hour

	// DefaultCrystallizationCandidateLimit is the max number of candidates to
	// crystallize per run (prevents overloading IPFS on backlog).
	DefaultCrystallizationCandidateLimit = 50
)

// CrystallizationCandidateLister lists post IDs eligible for crystallization.
type CrystallizationCandidateLister interface {
	ListCrystallizationCandidates(ctx context.Context, stabilityPeriod time.Duration, limit int) ([]string, error)
}

// ProblemCrystallizer crystallizes a single problem to IPFS.
type ProblemCrystallizer interface {
	CrystallizeProblem(ctx context.Context, problemID string) (string, error)
}

// CrystallizationJob handles periodic scanning and crystallization of solved problems.
type CrystallizationJob struct {
	lister          CrystallizationCandidateLister
	crystallizer    ProblemCrystallizer
	stabilityPeriod time.Duration
}

// NewCrystallizationJob creates a new crystallization job.
func NewCrystallizationJob(
	lister CrystallizationCandidateLister,
	crystallizer ProblemCrystallizer,
	stabilityPeriod time.Duration,
) *CrystallizationJob {
	return &CrystallizationJob{
		lister:          lister,
		crystallizer:    crystallizer,
		stabilityPeriod: stabilityPeriod,
	}
}

// RunOnce scans for crystallization candidates and crystallizes them.
// Returns the number of successfully crystallized and failed attempts.
func (j *CrystallizationJob) RunOnce(ctx context.Context) (crystallized, failed int) {
	candidates, err := j.lister.ListCrystallizationCandidates(
		ctx, j.stabilityPeriod, DefaultCrystallizationCandidateLimit,
	)
	if err != nil {
		log.Printf("Crystallization job: failed to list candidates: %v", err)
		return 0, 0
	}

	if len(candidates) == 0 {
		return 0, 0
	}

	log.Printf("Crystallization job: found %d candidates", len(candidates))

	for _, problemID := range candidates {
		cid, err := j.crystallizer.CrystallizeProblem(ctx, problemID)
		if err != nil {
			log.Printf("Crystallization job: failed to crystallize %s: %v", problemID, err)
			failed++
			continue
		}
		log.Printf("Crystallization job: crystallized %s → %s", problemID, cid)
		crystallized++
	}

	return crystallized, failed
}

// RunScheduled runs the crystallization job on a schedule.
// It runs immediately on start, then repeats at the given interval.
// The job stops when the context is cancelled.
func (j *CrystallizationJob) RunScheduled(ctx context.Context, interval time.Duration) {
	// Run immediately on start
	crystallized, failed := j.RunOnce(ctx)
	if crystallized > 0 || failed > 0 {
		log.Printf("Crystallization job: %d crystallized, %d failed", crystallized, failed)
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("Crystallization job stopped")
			return
		case <-ticker.C:
			crystallized, failed := j.RunOnce(ctx)
			if crystallized > 0 || failed > 0 {
				log.Printf("Crystallization job: %d crystallized, %d failed", crystallized, failed)
			}
		}
	}
}
