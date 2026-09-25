package db

import "context"

// IsActiveUser reports whether userID names a live account: present and not soft-deleted.
// Authentication asks it before trusting a JWT, which outlives the account it was minted
// for (SPEC Part 20.2). An id that is not a UUID names no account.
func (r *UserRepository) IsActiveUser(ctx context.Context, userID string) (bool, error) {
	var active bool
	err := r.pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM users WHERE id = $1 AND deleted_at IS NULL)`, userID,
	).Scan(&active)
	if err != nil {
		if isInvalidUUIDError(err) {
			return false, nil
		}
		LogQueryError(ctx, "IsActiveUser", "users", err)
		return false, err
	}
	return active, nil
}
