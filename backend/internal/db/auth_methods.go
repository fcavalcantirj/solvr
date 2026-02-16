package db

import (
	"context"
	"errors"
	"strings"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/jackc/pgx/v5"
)

// AuthMethodRepository handles database operations for authentication methods.
// Supports multiple authentication methods per user (email, GitHub, Google).
type AuthMethodRepository struct {
	pool *Pool
}

// NewAuthMethodRepository creates a new AuthMethodRepository.
func NewAuthMethodRepository(pool *Pool) *AuthMethodRepository {
	return &AuthMethodRepository{pool: pool}
}

// Create adds a new authentication method for a user.
// Returns error if:
// - User already has this auth provider type (e.g., can't have 2 GitHub accounts)
// - OAuth provider ID is already used by another user
func (r *AuthMethodRepository) Create(ctx context.Context, method *models.AuthMethod) (*models.AuthMethod, error) {
	query := `
		INSERT INTO auth_methods (user_id, auth_provider, auth_provider_id, password_hash)
		VALUES ($1, $2, $3, $4)
		RETURNING id, user_id, auth_provider, auth_provider_id, created_at, last_used_at
	`

	var providerID *string
	if method.AuthProviderID != "" {
		providerID = &method.AuthProviderID
	}

	var passwordHash *string
	if method.PasswordHash != "" {
		passwordHash = &method.PasswordHash
	}

	row := r.pool.QueryRow(ctx, query, method.UserID, method.AuthProvider, providerID, passwordHash)

	created := &models.AuthMethod{}
	var scannedProviderID, scannedPasswordHash *string
	err := row.Scan(
		&created.ID,
		&created.UserID,
		&created.AuthProvider,
		&scannedProviderID,
		&created.CreatedAt,
		&created.LastUsedAt,
	)

	if err != nil {
		// Check for constraint violations
		errMsg := err.Error()
		if strings.Contains(errMsg, "auth_methods_unique_provider_per_user") {
			return nil, errors.New("user already has this auth provider")
		}
		if strings.Contains(errMsg, "auth_methods_unique_oauth_id") {
			return nil, errors.New("oauth provider ID already in use")
		}
		return nil, err
	}

	if scannedProviderID != nil {
		created.AuthProviderID = *scannedProviderID
	}
	if scannedPasswordHash != nil {
		created.PasswordHash = *scannedPasswordHash
	}

	return created, nil
}

// FindByUserID returns all authentication methods for a user.
// Returns empty slice if user has no auth methods.
func (r *AuthMethodRepository) FindByUserID(ctx context.Context, userID string) ([]*models.AuthMethod, error) {
	query := `
		SELECT id, user_id, auth_provider, auth_provider_id, password_hash, created_at, last_used_at
		FROM auth_methods
		WHERE user_id = $1
		ORDER BY created_at ASC
	`

	rows, err := r.pool.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	methods := []*models.AuthMethod{}
	for rows.Next() {
		method := &models.AuthMethod{}
		var providerID, passwordHash *string

		err := rows.Scan(
			&method.ID,
			&method.UserID,
			&method.AuthProvider,
			&providerID,
			&passwordHash,
			&method.CreatedAt,
			&method.LastUsedAt,
		)
		if err != nil {
			return nil, err
		}

		if providerID != nil {
			method.AuthProviderID = *providerID
		}
		if passwordHash != nil {
			method.PasswordHash = *passwordHash
		}

		methods = append(methods, method)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return methods, nil
}

// FindByProvider finds an authentication method by provider type and provider ID.
// Used for OAuth login to find which user owns a given OAuth account.
// Returns ErrNotFound if no matching auth method exists.
func (r *AuthMethodRepository) FindByProvider(ctx context.Context, provider, providerID string) (*models.AuthMethod, error) {
	query := `
		SELECT id, user_id, auth_provider, auth_provider_id, password_hash, created_at, last_used_at
		FROM auth_methods
		WHERE auth_provider = $1 AND auth_provider_id = $2
	`

	method := &models.AuthMethod{}
	var scannedProviderID, scannedPasswordHash *string

	err := r.pool.QueryRow(ctx, query, provider, providerID).Scan(
		&method.ID,
		&method.UserID,
		&method.AuthProvider,
		&scannedProviderID,
		&scannedPasswordHash,
		&method.CreatedAt,
		&method.LastUsedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, err
	}

	if scannedProviderID != nil {
		method.AuthProviderID = *scannedProviderID
	}
	if scannedPasswordHash != nil {
		method.PasswordHash = *scannedPasswordHash
	}

	return method, nil
}

// UpdateLastUsed updates the last_used_at timestamp for an auth method.
// Called when a user successfully authenticates using this method.
func (r *AuthMethodRepository) UpdateLastUsed(ctx context.Context, methodID string) error {
	query := `
		UPDATE auth_methods
		SET last_used_at = NOW()
		WHERE id = $1
	`

	result, err := r.pool.Exec(ctx, query, methodID)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return ErrNotFound
	}

	return nil
}

// Delete removes an authentication method from a user's account.
// Used for unlinking providers (e.g., user wants to remove GitHub auth).
// Returns ErrNotFound if the auth method doesn't exist.
func (r *AuthMethodRepository) Delete(ctx context.Context, methodID string) error {
	query := `
		DELETE FROM auth_methods
		WHERE id = $1
	`

	result, err := r.pool.Exec(ctx, query, methodID)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return ErrNotFound
	}

	return nil
}

// HasEmailAuth checks if a user has email/password authentication enabled.
// Useful for checking if user can login with password or is OAuth-only.
func (r *AuthMethodRepository) HasEmailAuth(ctx context.Context, userID string) (bool, error) {
	query := `
		SELECT EXISTS(
			SELECT 1 FROM auth_methods
			WHERE user_id = $1 AND auth_provider = 'email'
		)
	`

	var exists bool
	err := r.pool.QueryRow(ctx, query, userID).Scan(&exists)
	if err != nil {
		return false, err
	}

	return exists, nil
}

// GetEmailAuthMethod returns the email/password auth method for a user, if it exists.
// Returns ErrNotFound if the user doesn't have email/password auth.
func (r *AuthMethodRepository) GetEmailAuthMethod(ctx context.Context, userID string) (*models.AuthMethod, error) {
	query := `
		SELECT id, user_id, auth_provider, auth_provider_id, password_hash, created_at, last_used_at
		FROM auth_methods
		WHERE user_id = $1 AND auth_provider = 'email'
	`

	method := &models.AuthMethod{}
	var providerID, passwordHash *string

	err := r.pool.QueryRow(ctx, query, userID).Scan(
		&method.ID,
		&method.UserID,
		&method.AuthProvider,
		&providerID,
		&passwordHash,
		&method.CreatedAt,
		&method.LastUsedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, err
	}

	if providerID != nil {
		method.AuthProviderID = *providerID
	}
	if passwordHash != nil {
		method.PasswordHash = *passwordHash
	}

	return method, nil
}
