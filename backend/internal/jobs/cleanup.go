// Package jobs provides background job implementations.
// Per SPEC.md Part 6: rate_limits table and claim_tokens table cleanup.
package jobs

import (
	"context"
	"log"
	"time"
)

// DefaultCleanupInterval is the default interval for running cleanup jobs.
// Per prd-v2.json requirement: "Run every hour"
const DefaultCleanupInterval = 1 * time.Hour

// ClaimTokenCleaner defines the interface for cleaning up claim tokens.
// This allows for easier testing with mocks.
type ClaimTokenCleaner interface {
	// DeleteExpiredTokens deletes all expired and unused claim tokens.
	// Per requirement: "Delete where expires_at < NOW() AND used_at IS NULL"
	DeleteExpiredTokens(ctx context.Context) (int64, error)
}

// CleanupJob handles periodic cleanup of expired data.
type CleanupJob struct {
	tokenRepo ClaimTokenCleaner
}

// NewCleanupJob creates a new cleanup job with the given repository.
func NewCleanupJob(tokenRepo ClaimTokenCleaner) *CleanupJob {
	return &CleanupJob{
		tokenRepo: tokenRepo,
	}
}

// CleanupExpiredTokens runs the token cleanup once.
// Returns the number of deleted tokens.
func (j *CleanupJob) CleanupExpiredTokens(ctx context.Context) (int64, error) {
	return j.tokenRepo.DeleteExpiredTokens(ctx)
}

// RunScheduled runs the cleanup job on a schedule.
// It will run cleanup immediately on start, then repeat at the given interval.
// The job stops when the context is cancelled.
func (j *CleanupJob) RunScheduled(ctx context.Context, interval time.Duration) {
	// Run immediately on start
	j.runCleanup(ctx)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("Cleanup job stopped")
			return
		case <-ticker.C:
			j.runCleanup(ctx)
		}
	}
}

// runCleanup performs the cleanup and logs results.
func (j *CleanupJob) runCleanup(ctx context.Context) {
	deleted, err := j.CleanupExpiredTokens(ctx)
	if err != nil {
		log.Printf("Failed to cleanup expired tokens: %v", err)
		return
	}
	if deleted > 0 {
		log.Printf("Cleaned up %d expired claim tokens", deleted)
	}
}
