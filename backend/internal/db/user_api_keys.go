// Package db provides database access for Solvr.
package db

import (
	"context"
	"errors"

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
		return nil, err
	}
	defer rows.Close()

	var keys []*models.UserAPIKey
	for rows.Next() {
		key, err := r.scanUserAPIKeyFromRows(rows)
		if err != nil {
			return nil, err
		}
		keys = append(keys, key)
	}

	if err := rows.Err(); err != nil {
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
		return err
	}

	if tag.RowsAffected() == 0 {
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
	return err
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
			return nil, ErrNotFound
		}
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
