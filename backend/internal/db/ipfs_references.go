package db

import (
	"context"
	"fmt"
)

// IPFSReferenceRepository answers whether Solvr still references an IPFS CID (anti-abuse W5).
type IPFSReferenceRepository struct {
	pool *Pool
}

// NewIPFSReferenceRepository creates an IPFSReferenceRepository.
func NewIPFSReferenceRepository(pool *Pool) *IPFSReferenceRepository {
	return &IPFSReferenceRepository{pool: pool}
}

// CIDReferences counts the pins rows and the posts (crystallization_cid) that name cid, dead
// rows included: Kubo's pin set is node-wide, so an unpin must not pull a CID any row names.
func (r *IPFSReferenceRepository) CIDReferences(ctx context.Context, cid string) (pins, posts int, err error) {
	err = r.pool.QueryRow(ctx, `
		SELECT (SELECT count(*) FROM pins WHERE cid = $1),
		       (SELECT count(*) FROM posts WHERE crystallization_cid = $1)`, cid).Scan(&pins, &posts)
	if err != nil {
		LogQueryError(ctx, "CIDReferences", "pins", err)
		return 0, 0, fmt.Errorf("cid references: %w", err)
	}
	return pins, posts, nil
}
