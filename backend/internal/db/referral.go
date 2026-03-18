package db

import (
	"context"

	"github.com/jackc/pgx/v5"
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

// CreateReferral inserts a new referral row linking referrer to referred user.
// Returns an error if the insert fails (e.g., duplicate referred_id unique constraint).
func (r *ReferralRepository) CreateReferral(ctx context.Context, referrerID, referredID string) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO referrals (id, referrer_id, referred_id, created_at)
		 VALUES (gen_random_uuid(), $1, $2, NOW())`,
		referrerID, referredID,
	)
	if err != nil {
		LogQueryError(ctx, "CreateReferral", "referrals", err)
		return err
	}
	return nil
}

// FindUserIDByReferralCode returns the user ID for the given referral code.
// Returns ErrNotFound if no active user has this referral code.
func (r *ReferralRepository) FindUserIDByReferralCode(ctx context.Context, code string) (string, error) {
	var userID string
	err := r.pool.QueryRow(ctx,
		`SELECT id FROM users WHERE referral_code = $1 AND deleted_at IS NULL`,
		code,
	).Scan(&userID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return "", ErrNotFound
		}
		LogQueryError(ctx, "FindUserIDByReferralCode", "users", err)
		return "", err
	}
	return userID, nil
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
