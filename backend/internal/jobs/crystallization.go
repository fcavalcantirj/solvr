// Package jobs provides background job implementations.
package jobs

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/fcavalcantirj/solvr/internal/services"
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

// CrystallizationResult holds the results of a single crystallization job run.
type CrystallizationResult struct {
	Crystallized int
	Failed       int
	Skipped      int
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
func (j *CrystallizationJob) RunOnce(ctx context.Context) CrystallizationResult {
	var result CrystallizationResult

	candidates, err := j.lister.ListCrystallizationCandidates(
		ctx, j.stabilityPeriod, DefaultCrystallizationCandidateLimit,
	)
	if err != nil {
		log.Printf("Crystallization job: failed to list candidates: %v", err)
		return result
	}

	if len(candidates) == 0 {
		return result
	}

	log.Printf("Crystallization job: found %d candidates", len(candidates))

	for _, problemID := range candidates {
		cid, err := j.crystallizer.CrystallizeProblem(ctx, problemID)
		if err != nil {
			if errors.Is(err, services.ErrNoVerifiedApproach) {
				log.Printf("Crystallization job: skipping %s (no succeeded approach)", problemID)
				result.Skipped++
				continue
			}
			log.Printf("Crystallization job: failed to crystallize %s: %v", problemID, err)
			result.Failed++
			continue
		}
		log.Printf("Crystallization job: crystallized %s → %s", problemID, cid)
		result.Crystallized++
	}

	return result
}

// RunScheduled runs the crystallization job on a schedule.
// It runs immediately on start, then repeats at the given interval.
// The job stops when the context is cancelled.
func (j *CrystallizationJob) RunScheduled(ctx context.Context, interval time.Duration) {
	// Run immediately on start
	result := j.RunOnce(ctx)
	logCrystallizationResult(result)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("Crystallization job stopped")
			return
		case <-ticker.C:
			result := j.RunOnce(ctx)
			logCrystallizationResult(result)
		}
	}
}

func logCrystallizationResult(result CrystallizationResult) {
	if result.Crystallized > 0 || result.Failed > 0 || result.Skipped > 0 {
		log.Printf("Crystallization job: %d crystallized, %d failed, %d skipped",
			result.Crystallized, result.Failed, result.Skipped)
	}
}
