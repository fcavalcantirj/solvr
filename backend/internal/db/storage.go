// Package db provides database access for Solvr.
package db

import (
	"context"
	"fmt"
)

// StorageRepository handles storage quota tracking in the database.
// Supports both users (owner_type=human) and agents (owner_type=agent).
type StorageRepository struct {
	pool *Pool
}

// NewStorageRepository creates a new StorageRepository.
func NewStorageRepository(pool *Pool) *StorageRepository {
	return &StorageRepository{pool: pool}
}

// GetStorageUsage returns the current storage usage and quota for an owner.
// For humans: reads from users table (storage_used_bytes, storage_quota_bytes).
// For agents: reads from agents table (storage_used_bytes, pinning_quota_bytes).
func (r *StorageRepository) GetStorageUsage(ctx context.Context, ownerID, ownerType string) (used int64, quota int64, err error) {
	var query string
	switch ownerType {
	case "human":
		query = `SELECT storage_used_bytes, storage_quota_bytes FROM users WHERE id = $1 AND deleted_at IS NULL`
	case "agent":
		query = `SELECT storage_used_bytes, pinning_quota_bytes FROM agents WHERE id = $1 AND deleted_at IS NULL`
	default:
		return 0, 0, fmt.Errorf("unknown owner type: %s", ownerType)
	}

	err = r.pool.QueryRow(ctx, query, ownerID).Scan(&used, &quota)
	if err != nil {
		LogQueryError(ctx, "GetStorageUsage", "storage", err)
		return 0, 0, err
	}

	return used, quota, nil
}

// UpdateStorageUsed adjusts the storage_used_bytes by deltaBytes.
// deltaBytes can be positive (adding content) or negative (removing content).
// Uses GREATEST(0, ...) to prevent going negative.
func (r *StorageRepository) UpdateStorageUsed(ctx context.Context, ownerID, ownerType string, deltaBytes int64) error {
	var query string
	switch ownerType {
	case "human":
		query = `UPDATE users SET storage_used_bytes = GREATEST(0, storage_used_bytes + $2), updated_at = NOW() WHERE id = $1 AND deleted_at IS NULL`
	case "agent":
		query = `UPDATE agents SET storage_used_bytes = GREATEST(0, storage_used_bytes + $2), updated_at = NOW() WHERE id = $1 AND deleted_at IS NULL`
	default:
		return fmt.Errorf("unknown owner type: %s", ownerType)
	}

	result, err := r.pool.Exec(ctx, query, ownerID, deltaBytes)
	if err != nil {
		LogQueryError(ctx, "UpdateStorageUsed", "storage", err)
		return err
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("owner not found: %s/%s", ownerType, ownerID)
	}

	return nil
}
