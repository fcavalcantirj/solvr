package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/fcavalcantirj/solvr/internal/referral"
)

// ErrDuplicateAuthProviderID is returned when an OAuth identity is already linked to an account.
var ErrDuplicateAuthProviderID = errors.New("oauth provider ID already in use")

// CreateWithAuthMethod inserts a new OAuth user and its auth method in one transaction, so a
// failed auth method insert (for example an OAuth id still held by a tombstoned account)
// leaves no users row behind.
func (r *UserRepository) CreateWithAuthMethod(ctx context.Context, user *models.User, method *models.AuthMethod) (*models.User, error) {
	if user.ReferralCode == "" {
		code, err := referral.GenerateCode()
		if err != nil {
			return nil, fmt.Errorf("failed to generate referral code: %w", err)
		}
		user.ReferralCode = code
	}

	created := &models.User{}
	err := r.pool.WithTx(ctx, func(tx Tx) error {
		var passwordHash, avatarURL, bio, authProvider, authProviderID, role, referralCode sql.NullString
		err := tx.QueryRow(ctx, `
			INSERT INTO users (username, display_name, email, auth_provider, auth_provider_id, password_hash, avatar_url, bio, role, referral_code)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
			RETURNING id, username, display_name, email, auth_provider, auth_provider_id, password_hash, avatar_url, bio, role, referral_code, created_at, updated_at`,
			user.Username, user.DisplayName, user.Email, user.AuthProvider, user.AuthProviderID,
			user.PasswordHash, user.AvatarURL, user.Bio, user.Role, user.ReferralCode,
		).Scan(&created.ID, &created.Username, &created.DisplayName, &created.Email, &authProvider,
			&authProviderID, &passwordHash, &avatarURL, &bio, &role, &referralCode,
			&created.CreatedAt, &created.UpdatedAt)
		if err != nil {
			return mapUserInsertError(err)
		}
		created.AuthProvider, created.AuthProviderID = authProvider.String, authProviderID.String
		created.PasswordHash, created.AvatarURL, created.Bio = passwordHash.String, avatarURL.String, bio.String
		created.Role, created.ReferralCode = role.String, referralCode.String

		var providerID *string
		if method.AuthProviderID != "" {
			providerID = &method.AuthProviderID
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO auth_methods (user_id, auth_provider, auth_provider_id, last_used_at)
			VALUES ($1, $2, $3, NOW())`, created.ID, method.AuthProvider, providerID); err != nil {
			if strings.Contains(err.Error(), "auth_methods_unique_oauth_id") {
				return ErrDuplicateAuthProviderID
			}
			return fmt.Errorf("create auth method: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return created, nil
}

// mapUserInsertError turns the users unique violations and the tombstone trigger's refusal
// into the repository's sentinel errors.
func mapUserInsertError(err error) error {
	msg := err.Error()
	switch {
	case strings.Contains(msg, "users_username_key"):
		return ErrDuplicateUsername
	case strings.Contains(msg, "users_email_key"):
		return ErrDuplicateEmail
	case strings.Contains(msg, "account suspended"):
		return ErrAccountSuspended
	}
	return err
}
