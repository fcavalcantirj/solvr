// Package db provides database access for Solvr.
package db

import (
	"context"
	"errors"
	"log/slog"

	"github.com/fcavalcantirj/solvr/internal/auth"
	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/jackc/pgx/v5"
)

// UserAPIKeyRepository handles database operations for user API keys.
// Per prd-v2.json API-KEYS requirements.
type UserAPIKeyRepository struct {
	pool *Pool
}

// NewUserAPIKeyRepository creates a new UserAPIKeyRepository.
func NewUserAPIKeyRepository(pool *Pool) *UserAPIKeyRepository {
	return &UserAPIKeyRepository{pool: pool}
}

// Create inserts a new API key for a user.
// Returns the created key with ID and timestamps set.
// Note: The KeyHash should already be hashed before calling this.
func (r *UserAPIKeyRepository) Create(ctx context.Context, key *models.UserAPIKey) (*models.UserAPIKey, error) {
	query := `
		INSERT INTO user_api_keys (user_id, name, key_hash)
		VALUES ($1, $2, $3)
		RETURNING id, user_id, name, key_hash, last_used_at, revoked_at, created_at, updated_at
	`

	row := r.pool.QueryRow(ctx, query,
		key.UserID,
		key.Name,
		key.KeyHash,
	)

	return r.scanUserAPIKey(row)
}

// FindByUserID returns all active API keys for a user.
// Only returns keys where revoked_at IS NULL.
func (r *UserAPIKeyRepository) FindByUserID(ctx context.Context, userID string) ([]*models.UserAPIKey, error) {
	query := `
		SELECT id, user_id, name, key_hash, last_used_at, revoked_at, created_at, updated_at
		FROM user_api_keys
		WHERE user_id = $1 AND revoked_at IS NULL
		ORDER BY created_at DESC
	`

	rows, err := r.pool.Query(ctx, query, userID)
	if err != nil {
		LogQueryError(ctx, "FindByUserID", "user_api_keys", err)
		return nil, err
	}
	defer rows.Close()

	var keys []*models.UserAPIKey
	for rows.Next() {
		key, err := r.scanUserAPIKeyFromRows(rows)
		if err != nil {
			LogQueryError(ctx, "FindByUserID.Scan", "user_api_keys", err)
			return nil, err
		}
		keys = append(keys, key)
	}

	if err := rows.Err(); err != nil {
		LogQueryError(ctx, "FindByUserID.Rows", "user_api_keys", err)
		return nil, err
	}

	// Return empty slice instead of nil for consistent JSON marshaling
	if keys == nil {
		keys = []*models.UserAPIKey{}
	}

	return keys, nil
}

// FindByID finds a single API key by its ID.
// Returns the key even if revoked (for audit purposes).
func (r *UserAPIKeyRepository) FindByID(ctx context.Context, id string) (*models.UserAPIKey, error) {
	query := `
		SELECT id, user_id, name, key_hash, last_used_at, revoked_at, created_at, updated_at
		FROM user_api_keys
		WHERE id = $1
	`

	row := r.pool.QueryRow(ctx, query, id)
	return r.scanUserAPIKey(row)
}

// Revoke soft-deletes an API key by setting revoked_at.
// Only the owner (userID) can revoke their own keys.
func (r *UserAPIKeyRepository) Revoke(ctx context.Context, id, userID string) error {
	query := `
		UPDATE user_api_keys
		SET revoked_at = NOW(), updated_at = NOW()
		WHERE id = $1 AND user_id = $2 AND revoked_at IS NULL
	`

	tag, err := r.pool.Exec(ctx, query, id, userID)
	if err != nil {
		LogQueryError(ctx, "Revoke", "user_api_keys", err)
		return err
	}

	if tag.RowsAffected() == 0 {
		slog.Debug("api key not found", "op", "Revoke", "table", "user_api_keys", "id", id)
		return ErrNotFound
	}

	return nil
}

// UpdateLastUsed updates the last_used_at timestamp for a key.
// Called during API key authentication.
func (r *UserAPIKeyRepository) UpdateLastUsed(ctx context.Context, id string) error {
	query := `
		UPDATE user_api_keys
		SET last_used_at = NOW(), updated_at = NOW()
		WHERE id = $1
	`

	_, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		LogQueryError(ctx, "UpdateLastUsed", "user_api_keys", err)
	}
	return err
}

// Regenerate updates an API key with a new hash value.
// Only the owner (userID) can regenerate their own keys.
// The key must not be revoked.
// Per prd-v2.json: "Generate new key value (old one invalidated), Keep same key ID/name for tracking"
func (r *UserAPIKeyRepository) Regenerate(ctx context.Context, id, userID, newKeyHash string) (*models.UserAPIKey, error) {
	query := `
		UPDATE user_api_keys
		SET key_hash = $1, updated_at = NOW()
		WHERE id = $2 AND user_id = $3 AND revoked_at IS NULL
		RETURNING id, user_id, name, key_hash, last_used_at, revoked_at, created_at, updated_at
	`

	row := r.pool.QueryRow(ctx, query, newKeyHash, id, userID)
	return r.scanUserAPIKey(row)
}

// scanUserAPIKey scans a single row into a UserAPIKey struct.
func (r *UserAPIKeyRepository) scanUserAPIKey(row pgx.Row) (*models.UserAPIKey, error) {
	key := &models.UserAPIKey{}
	err := row.Scan(
		&key.ID,
		&key.UserID,
		&key.Name,
		&key.KeyHash,
		&key.LastUsedAt,
		&key.RevokedAt,
		&key.CreatedAt,
		&key.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			slog.Debug("api key not found", "op", "scanUserAPIKey", "table", "user_api_keys")
			return nil, ErrNotFound
		}
		LogQueryError(context.Background(), "scanUserAPIKey", "user_api_keys", err)
		return nil, err
	}

	return key, nil
}

// scanUserAPIKeyFromRows scans a rows result into a UserAPIKey struct.
func (r *UserAPIKeyRepository) scanUserAPIKeyFromRows(rows pgx.Rows) (*models.UserAPIKey, error) {
	key := &models.UserAPIKey{}
	err := rows.Scan(
		&key.ID,
		&key.UserID,
		&key.Name,
		&key.KeyHash,
		&key.LastUsedAt,
		&key.RevokedAt,
		&key.CreatedAt,
		&key.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	return key, nil
}

// GetUserByAPIKey validates a plain text API key and returns the associated user and key.
// This method iterates through all active keys and compares using bcrypt.
// Returns nil, nil, nil if no matching key is found.
// Per prd-v2.json: "Hash and lookup in database, Attach user context to request, Update last_used_at"
func (r *UserAPIKeyRepository) GetUserByAPIKey(ctx context.Context, plainKey string) (*models.User, *models.UserAPIKey, error) {
	// Get all active API keys
	// Note: For production scale, consider adding a key identifier prefix for faster lookup
	query := `
		SELECT k.id, k.user_id, k.name, k.key_hash, k.last_used_at, k.revoked_at, k.created_at, k.updated_at,
		       u.id, u.username, u.display_name, u.email, u.auth_provider, u.auth_provider_id,
		       u.avatar_url, u.bio, u.role, u.created_at, u.updated_at
		FROM user_api_keys k
		JOIN users u ON k.user_id = u.id
		WHERE k.revoked_at IS NULL
	`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		LogQueryError(ctx, "GetUserByAPIKey", "user_api_keys", err)
		return nil, nil, err
	}
	defer rows.Close()

	for rows.Next() {
		key := &models.UserAPIKey{}
		user := &models.User{}

		err := rows.Scan(
			&key.ID,
			&key.UserID,
			&key.Name,
			&key.KeyHash,
			&key.LastUsedAt,
			&key.RevokedAt,
			&key.CreatedAt,
			&key.UpdatedAt,
			&user.ID,
			&user.Username,
			&user.DisplayName,
			&user.Email,
			&user.AuthProvider,
			&user.AuthProviderID,
			&user.AvatarURL,
			&user.Bio,
			&user.Role,
			&user.CreatedAt,
			&user.UpdatedAt,
		)
		if err != nil {
			LogQueryError(ctx, "GetUserByAPIKey.Scan", "user_api_keys", err)
			return nil, nil, err
		}

		// Compare the plain key with the stored hash
		err = auth.CompareAPIKey(plainKey, key.KeyHash)
		if err == nil {
			// Found matching key
			return user, key, nil
		}
		// Key doesn't match, continue to next
	}

	if err := rows.Err(); err != nil {
		LogQueryError(ctx, "GetUserByAPIKey.Rows", "user_api_keys", err)
		return nil, nil, err
	}

	// No matching key found
	return nil, nil, nil
}
