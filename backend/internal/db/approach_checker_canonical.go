package db

import (
	"context"
	"fmt"
)

// CanonicalApproachCheckerRepository answers whether a problem has a succeeded approach from
// the canonical replies (task idx 76 step 3): a live reply migrated from a succeeded approach
// (the status is kept in provenance). PATCH /v1/posts/{id} uses it to gate marking a problem
// solved. Native replies have no status, so only migrated approaches can count.
type CanonicalApproachCheckerRepository struct {
	pool *Pool
}

// NewCanonicalApproachCheckerRepository creates a CanonicalApproachCheckerRepository.
func NewCanonicalApproachCheckerRepository(pool *Pool) *CanonicalApproachCheckerRepository {
	return &CanonicalApproachCheckerRepository{pool: pool}
}

// HasSucceededApproach reports whether the post has a live reply migrated from a succeeded approach.
func (r *CanonicalApproachCheckerRepository) HasSucceededApproach(ctx context.Context, problemID string) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM replies r
			WHERE r.post_id = $1 AND `+canonicalSucceededApproachReply+`
		)`, problemID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check succeeded approach reply: %w", err)
	}
	return exists, nil
}
