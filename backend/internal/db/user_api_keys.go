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
// Uses SHA256 for O(1) indexed lookup. Falls back to O(n) bcrypt scan for keys
// that haven't been backfilled yet, and lazy-backfills their SHA256 on match.
// Returns nil, nil, nil if no matching key is found.
func (r *UserAPIKeyRepository) GetUserByAPIKey(ctx context.Context, plainKey string) (*models.User, *models.UserAPIKey, error) {
	keySHA256 := auth.SHA256APIKey(plainKey)

	// Fast path: O(1) lookup by SHA256 index
	user, key, err := r.getUserByKeySHA256(ctx, keySHA256)
	if err != nil {
		return nil, nil, err
	}
	if user != nil {
		return user, key, nil
	}

	// Slow path: bcrypt scan for keys without SHA256 (lazy backfill)
	return r.getUserByKeyBcryptFallback(ctx, plainKey, keySHA256)
}

// getUserByKeySHA256 does an O(1) indexed lookup by SHA256 hash.
func (r *UserAPIKeyRepository) getUserByKeySHA256(ctx context.Context, keySHA256 string) (*models.User, *models.UserAPIKey, error) {
	query := `
		SELECT k.id, k.user_id, k.name, k.key_hash, k.last_used_at, k.revoked_at, k.created_at, k.updated_at,
		       u.id, u.username, u.display_name, u.email, u.auth_provider, u.auth_provider_id,
		       u.avatar_url, u.bio, u.role, u.created_at, u.updated_at
		FROM user_api_keys k
		JOIN users u ON k.user_id = u.id
		WHERE k.key_sha256 = $1 AND k.revoked_at IS NULL
	`

	key := &models.UserAPIKey{}
	user := &models.User{}

	err := r.pool.QueryRow(ctx, query, keySHA256).Scan(
		&key.ID, &key.UserID, &key.Name, &key.KeyHash,
		&key.LastUsedAt, &key.RevokedAt, &key.CreatedAt, &key.UpdatedAt,
		&user.ID, &user.Username, &user.DisplayName, &user.Email,
		&user.AuthProvider, &user.AuthProviderID, &user.AvatarURL,
		&user.Bio, &user.Role, &user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil, nil // Not found via SHA256, try fallback
		}
		LogQueryError(ctx, "getUserByKeySHA256", "user_api_keys", err)
		return nil, nil, err
	}

	return user, key, nil
}

// getUserByKeyBcryptFallback scans all keys without SHA256 and compares via bcrypt.
// On match, lazy-backfills the SHA256 for future O(1) lookups.
func (r *UserAPIKeyRepository) getUserByKeyBcryptFallback(ctx context.Context, plainKey, keySHA256 string) (*models.User, *models.UserAPIKey, error) {
	query := `
		SELECT k.id, k.user_id, k.name, k.key_hash, k.last_used_at, k.revoked_at, k.created_at, k.updated_at,
		       u.id, u.username, u.display_name, u.email, u.auth_provider, u.auth_provider_id,
		       u.avatar_url, u.bio, u.role, u.created_at, u.updated_at
		FROM user_api_keys k
		JOIN users u ON k.user_id = u.id
		WHERE k.revoked_at IS NULL AND k.key_sha256 IS NULL
	`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		LogQueryError(ctx, "getUserByKeyBcryptFallback", "user_api_keys", err)
		return nil, nil, err
	}
	defer rows.Close()

	for rows.Next() {
		key := &models.UserAPIKey{}
		user := &models.User{}

		err := rows.Scan(
			&key.ID, &key.UserID, &key.Name, &key.KeyHash,
			&key.LastUsedAt, &key.RevokedAt, &key.CreatedAt, &key.UpdatedAt,
			&user.ID, &user.Username, &user.DisplayName, &user.Email,
			&user.AuthProvider, &user.AuthProviderID, &user.AvatarURL,
			&user.Bio, &user.Role, &user.CreatedAt, &user.UpdatedAt,
		)
		if err != nil {
			LogQueryError(ctx, "getUserByKeyBcryptFallback.Scan", "user_api_keys", err)
			return nil, nil, err
		}

		if auth.CompareAPIKey(plainKey, key.KeyHash) == nil {
			// Match found — lazy backfill SHA256 for future O(1) lookups
			r.backfillKeySHA256(ctx, key.ID, keySHA256)
			return user, key, nil
		}
	}

	if err := rows.Err(); err != nil {
		LogQueryError(ctx, "getUserByKeyBcryptFallback.Rows", "user_api_keys", err)
		return nil, nil, err
	}

	return nil, nil, nil
}

// backfillKeySHA256 stores the SHA256 hash for a key that was matched via bcrypt.
func (r *UserAPIKeyRepository) backfillKeySHA256(ctx context.Context, keyID, keySHA256 string) {
	query := `UPDATE user_api_keys SET key_sha256 = $1 WHERE id = $2 AND key_sha256 IS NULL`
	if _, err := r.pool.Exec(ctx, query, keySHA256, keyID); err != nil {
		slog.Warn("failed to backfill key_sha256", "keyID", keyID, "error", err)
	}
}
