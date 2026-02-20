// Package db provides database access for Solvr.
package db

import (
	"context"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// BadgeRepository handles database operations for milestone badges.
type BadgeRepository struct {
	pool *Pool
}

// NewBadgeRepository creates a new BadgeRepository.
func NewBadgeRepository(pool *Pool) *BadgeRepository {
	return &BadgeRepository{pool: pool}
}

// Award inserts a new badge. Uses INSERT ... ON CONFLICT DO NOTHING
// to be idempotent — duplicate awards are silently ignored.
func (r *BadgeRepository) Award(ctx context.Context, badge *models.Badge) error {
	query := `
		INSERT INTO badges (owner_type, owner_id, badge_type, badge_name, description, metadata)
		VALUES ($1, $2, $3, $4, $5, COALESCE($6::jsonb, '{}'::jsonb))
		ON CONFLICT (owner_type, owner_id, badge_type) DO NOTHING
	`

	metadata := badge.Metadata
	if metadata == nil {
		metadata = []byte("{}")
	}

	_, err := r.pool.Exec(ctx, query,
		badge.OwnerType,
		badge.OwnerID,
		badge.BadgeType,
		badge.BadgeName,
		badge.Description,
		string(metadata),
	)

	return err
}

// ListForOwner returns all badges for a given owner (agent or human).
func (r *BadgeRepository) ListForOwner(ctx context.Context, ownerType, ownerID string) ([]models.Badge, error) {
	query := `
		SELECT id, owner_type, owner_id, badge_type, badge_name, description, awarded_at, metadata
		FROM badges
		WHERE owner_type = $1 AND owner_id = $2
		ORDER BY awarded_at DESC
	`

	rows, err := r.pool.Query(ctx, query, ownerType, ownerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var badges []models.Badge
	for rows.Next() {
		var b models.Badge
		err := rows.Scan(
			&b.ID,
			&b.OwnerType,
			&b.OwnerID,
			&b.BadgeType,
			&b.BadgeName,
			&b.Description,
			&b.AwardedAt,
			&b.Metadata,
		)
		if err != nil {
			return nil, err
		}
		badges = append(badges, b)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	if badges == nil {
		badges = []models.Badge{}
	}

	return badges, nil
}

// HasBadge checks if a specific badge has been awarded to an owner.
func (r *BadgeRepository) HasBadge(ctx context.Context, ownerType, ownerID, badgeType string) (bool, error) {
	query := `
		SELECT EXISTS(
			SELECT 1 FROM badges
			WHERE owner_type = $1 AND owner_id = $2 AND badge_type = $3
		)
	`

	var exists bool
	err := r.pool.QueryRow(ctx, query, ownerType, ownerID, badgeType).Scan(&exists)
	if err != nil {
		return false, err
	}

	return exists, nil
}
