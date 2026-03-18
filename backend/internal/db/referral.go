package db

import (
	"context"
)

// ReferralRepository handles database operations for the referrals table.
type ReferralRepository struct {
	pool *Pool
}

// NewReferralRepository creates a new ReferralRepository.
func NewReferralRepository(pool *Pool) *ReferralRepository {
	return &ReferralRepository{pool: pool}
}

// CountByReferrer returns the number of users referred by the given user.
func (r *ReferralRepository) CountByReferrer(ctx context.Context, referrerID string) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM referrals WHERE referrer_id = $1`,
		referrerID,
	).Scan(&count)
	if err != nil {
		LogQueryError(ctx, "CountByReferrer", "referrals", err)
		return 0, err
	}
	return count, nil
}

// GetReferralCode returns the referral code for a given user.
func (r *ReferralRepository) GetReferralCode(ctx context.Context, userID string) (string, error) {
	var code string
	err := r.pool.QueryRow(ctx,
		`SELECT referral_code FROM users WHERE id = $1 AND deleted_at IS NULL`,
		userID,
	).Scan(&code)
	if err != nil {
		LogQueryError(ctx, "GetReferralCode", "users", err)
		return "", err
	}
	return code, nil
}
