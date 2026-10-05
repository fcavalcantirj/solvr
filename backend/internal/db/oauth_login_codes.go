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

// LoginCodeOrigin is what an OAuth callback knew when it issued a code: the provider the person
// signed in through, and whether that sign-in created the account. It is stored with the code
// so the exchange can tell a sign-up from a login (SPEC.md 5.2). The zero value is a login
// through no named provider.
type LoginCodeOrigin struct {
	Provider  string // models.AuthProviderGitHub or models.AuthProviderGoogle
	IsNewUser bool
}

// OAuthLoginUser is the account a redeemed login code stands for: exactly what a JWT is minted
// from, and the origin the code was issued with.
type OAuthLoginUser struct {
	ID    string
	Email string
	Role  string
	// Provider and IsNewUser are the LoginCodeOrigin of the code that was redeemed.
	Provider  string
	IsNewUser bool
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
// its SHA-256 is stored, beside the origin the callback knew. Expiry is computed by the
// database clock, so every instance agrees on it. Codes that expired more than an hour ago are
// swept in the same call, so the table stays small without a scheduler.
func (r *OAuthLoginCodeRepository) Issue(ctx context.Context, userID string, ttl time.Duration, origin LoginCodeOrigin) (string, error) {
	plaintext, hash, err := token.GenerateLoginCode()
	if err != nil {
		return "", err
	}
	if _, err := r.pool.Exec(ctx, `DELETE FROM oauth_login_codes WHERE expires_at < NOW() - INTERVAL '1 hour'`); err != nil {
		LogQueryError(ctx, "Issue", "oauth_login_codes", err)
		return "", err
	}
	if _, err := r.pool.Exec(ctx,
		`INSERT INTO oauth_login_codes (code_hash, user_id, expires_at, is_new_user, auth_provider)
		 VALUES ($1, $2, NOW() + make_interval(secs => $3), $4, $5)`,
		hash, userID, ttl.Seconds(), origin.IsNewUser, origin.Provider); err != nil {
		LogQueryError(ctx, "Issue", "oauth_login_codes", err)
		return "", err
	}
	return plaintext, nil
}

// Redeem consumes a code and returns its account with the origin the code was issued with. The
// consuming UPDATE is the only writer of consumed_at and matches only an unconsumed, unexpired
// row, so of any number of racing redeems (two tabs, two API instances) exactly one wins. A code
// of a soft-deleted account is consumed but yields ErrLoginCodeInvalid, so a deletion after the
// redirect still refuses the login.
func (r *OAuthLoginCodeRepository) Redeem(ctx context.Context, code string) (*OAuthLoginUser, error) {
	if code == "" {
		return nil, ErrLoginCodeInvalid
	}
	var user OAuthLoginUser
	err := r.pool.QueryRow(ctx,
		`WITH consumed AS (
			UPDATE oauth_login_codes SET consumed_at = NOW()
			WHERE code_hash = $1 AND consumed_at IS NULL AND expires_at > NOW()
			RETURNING user_id, is_new_user, auth_provider
		)
		SELECT u.id::text, u.email, COALESCE(u.role, 'user'), c.is_new_user, c.auth_provider
		FROM consumed c
		JOIN users u ON u.id = c.user_id AND u.deleted_at IS NULL`,
		token.HashToken(code)).Scan(&user.ID, &user.Email, &user.Role, &user.IsNewUser, &user.Provider)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrLoginCodeInvalid
	}
	if err != nil {
		LogQueryError(ctx, "Redeem", "oauth_login_codes", err)
		return nil, err
	}
	return &user, nil
}
