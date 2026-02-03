package db

import (
	"context"
)

// ClaimTokenRepository handles database operations for claim tokens.
// Claim tokens are used for agent-human linking flow.
// See SPEC.md Part 12.3 and PRD AGENT-LINKING category.
type ClaimTokenRepository struct {
	pool *Pool
}

// NewClaimTokenRepository creates a new ClaimTokenRepository.
func NewClaimTokenRepository(pool *Pool) *ClaimTokenRepository {
	return &ClaimTokenRepository{pool: pool}
}

// DeleteExpiredTokens deletes all claim tokens that have expired and are unused.
// Per prd-v2.json requirement: "Delete where expires_at < NOW() AND used_at IS NULL"
// Returns the number of deleted tokens.
func (r *ClaimTokenRepository) DeleteExpiredTokens(ctx context.Context) (int64, error) {
	query := `
		DELETE FROM claim_tokens
		WHERE expires_at < NOW() AND used_at IS NULL
	`

	result, err := r.pool.Exec(ctx, query)
	if err != nil {
		return 0, err
	}

	return result.RowsAffected(), nil
}
