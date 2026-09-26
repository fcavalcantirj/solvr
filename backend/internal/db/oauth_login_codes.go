package db

import (
	"context"
	"errors"
	"time"

	"github.com/fcavalcantirj/solvr/internal/token"
	"github.com/jackc/pgx/v5"
)

// ErrLoginCodeInvalid: the code is unknown, expired, already used, or its account is gone. The
// caller cannot tell which, and must not: each is "start the login again".
var ErrLoginCodeInvalid = errors.New("invalid login code")

// OAuthLoginUser is the account a redeemed login code stands for: exactly what a JWT is minted from.
type OAuthLoginUser struct {
	ID    string
	Email string
	Role  string
}

// OAuthLoginCodeRepository stores the one-time codes the OAuth callback hands to the browser in
// place of a JWT. It is shared by every API instance through the database; nothing is held in
// process memory.
type OAuthLoginCodeRepository struct {
	pool *Pool
}

// NewOAuthLoginCodeRepository creates an OAuthLoginCodeRepository.
func NewOAuthLoginCodeRepository(pool *Pool) *OAuthLoginCodeRepository {
	return &OAuthLoginCodeRepository{pool: pool}
}

// Issue stores a new code for userID that redeems for ttl and returns the plaintext once. Only
// its SHA-256 is stored. Expiry is computed by the database clock, so every instance agrees on
// it. Codes that expired more than an hour ago are swept in the same call, so the table stays
// small without a scheduler.
func (r *OAuthLoginCodeRepository) Issue(ctx context.Context, userID string, ttl time.Duration) (string, error) {
	plaintext, hash, err := token.GenerateLoginCode()
	if err != nil {
		return "", err
	}
	if _, err := r.pool.Exec(ctx, `DELETE FROM oauth_login_codes WHERE expires_at < NOW() - INTERVAL '1 hour'`); err != nil {
		LogQueryError(ctx, "Issue", "oauth_login_codes", err)
		return "", err
	}
	if _, err := r.pool.Exec(ctx,
		`INSERT INTO oauth_login_codes (code_hash, user_id, expires_at)
		 VALUES ($1, $2, NOW() + make_interval(secs => $3))`,
		hash, userID, ttl.Seconds()); err != nil {
		LogQueryError(ctx, "Issue", "oauth_login_codes", err)
		return "", err
	}
	return plaintext, nil
}

// Redeem consumes a code and returns its account. The consuming UPDATE is the only writer of
// consumed_at and matches only an unconsumed, unexpired row, so of any number of racing redeems
// (two tabs, two API instances) exactly one wins. A code of a soft-deleted account is consumed
// but yields ErrLoginCodeInvalid, so a deletion after the redirect still refuses the login.
func (r *OAuthLoginCodeRepository) Redeem(ctx context.Context, code string) (*OAuthLoginUser, error) {
	if code == "" {
		return nil, ErrLoginCodeInvalid
	}
	var user OAuthLoginUser
	err := r.pool.QueryRow(ctx,
		`WITH consumed AS (
			UPDATE oauth_login_codes SET consumed_at = NOW()
			WHERE code_hash = $1 AND consumed_at IS NULL AND expires_at > NOW()
			RETURNING user_id
		)
		SELECT u.id::text, u.email, COALESCE(u.role, 'user')
		FROM consumed c
		JOIN users u ON u.id = c.user_id AND u.deleted_at IS NULL`,
		token.HashToken(code)).Scan(&user.ID, &user.Email, &user.Role)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrLoginCodeInvalid
	}
	if err != nil {
		LogQueryError(ctx, "Redeem", "oauth_login_codes", err)
		return nil, err
	}
	return &user, nil
}
