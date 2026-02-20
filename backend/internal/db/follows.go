// Package db provides database access for Solvr.
package db

import (
	"context"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/jackc/pgx/v5"
)

// FollowsRepository handles database operations for the follows social graph.
type FollowsRepository struct {
	pool *Pool
}

// NewFollowsRepository creates a new FollowsRepository.
func NewFollowsRepository(pool *Pool) *FollowsRepository {
	return &FollowsRepository{pool: pool}
}

// Create inserts a new follow relationship. Uses INSERT ... ON CONFLICT DO NOTHING
// to be idempotent — duplicate follows are silently ignored.
func (r *FollowsRepository) Create(ctx context.Context, followerType, followerID, followedType, followedID string) (*models.Follow, error) {
	query := `
		INSERT INTO follows (follower_type, follower_id, followed_type, followed_id)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (follower_type, follower_id, followed_type, followed_id) DO NOTHING
		RETURNING id, follower_type, follower_id, followed_type, followed_id, created_at
	`

	var follow models.Follow
	err := r.pool.QueryRow(ctx, query, followerType, followerID, followedType, followedID).Scan(
		&follow.ID,
		&follow.FollowerType,
		&follow.FollowerID,
		&follow.FollowedType,
		&follow.FollowedID,
		&follow.CreatedAt,
	)

	if err == pgx.ErrNoRows {
		// Duplicate — fetch existing row
		fetchQuery := `
			SELECT id, follower_type, follower_id, followed_type, followed_id, created_at
			FROM follows
			WHERE follower_type = $1 AND follower_id = $2
			  AND followed_type = $3 AND followed_id = $4
		`
		err = r.pool.QueryRow(ctx, fetchQuery, followerType, followerID, followedType, followedID).Scan(
			&follow.ID,
			&follow.FollowerType,
			&follow.FollowerID,
			&follow.FollowedType,
			&follow.FollowedID,
			&follow.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		return &follow, nil
	}

	if err != nil {
		return nil, err
	}

	return &follow, nil
}

// Delete removes a follow relationship.
func (r *FollowsRepository) Delete(ctx context.Context, followerType, followerID, followedType, followedID string) error {
	query := `
		DELETE FROM follows
		WHERE follower_type = $1 AND follower_id = $2
		  AND followed_type = $3 AND followed_id = $4
	`

	_, err := r.pool.Exec(ctx, query, followerType, followerID, followedType, followedID)
	return err
}

// ListFollowing returns entities that the given user/agent is following.
func (r *FollowsRepository) ListFollowing(ctx context.Context, followerType, followerID string, limit, offset int) ([]models.Follow, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	query := `
		SELECT id, follower_type, follower_id, followed_type, followed_id, created_at
		FROM follows
		WHERE follower_type = $1 AND follower_id = $2
		ORDER BY created_at DESC
		LIMIT $3 OFFSET $4
	`

	rows, err := r.pool.Query(ctx, query, followerType, followerID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var follows []models.Follow
	for rows.Next() {
		var f models.Follow
		err := rows.Scan(&f.ID, &f.FollowerType, &f.FollowerID, &f.FollowedType, &f.FollowedID, &f.CreatedAt)
		if err != nil {
			return nil, err
		}
		follows = append(follows, f)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	if follows == nil {
		follows = []models.Follow{}
	}

	return follows, nil
}

// ListFollowers returns entities that are following the given user/agent.
func (r *FollowsRepository) ListFollowers(ctx context.Context, followedType, followedID string, limit, offset int) ([]models.Follow, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	query := `
		SELECT id, follower_type, follower_id, followed_type, followed_id, created_at
		FROM follows
		WHERE followed_type = $1 AND followed_id = $2
		ORDER BY created_at DESC
		LIMIT $3 OFFSET $4
	`

	rows, err := r.pool.Query(ctx, query, followedType, followedID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var follows []models.Follow
	for rows.Next() {
		var f models.Follow
		err := rows.Scan(&f.ID, &f.FollowerType, &f.FollowerID, &f.FollowedType, &f.FollowedID, &f.CreatedAt)
		if err != nil {
			return nil, err
		}
		follows = append(follows, f)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	if follows == nil {
		follows = []models.Follow{}
	}

	return follows, nil
}

// IsFollowing checks if a follow relationship exists.
func (r *FollowsRepository) IsFollowing(ctx context.Context, followerType, followerID, followedType, followedID string) (bool, error) {
	query := `
		SELECT EXISTS(
			SELECT 1 FROM follows
			WHERE follower_type = $1 AND follower_id = $2
			  AND followed_type = $3 AND followed_id = $4
		)
	`

	var exists bool
	err := r.pool.QueryRow(ctx, query, followerType, followerID, followedType, followedID).Scan(&exists)
	if err != nil {
		return false, err
	}

	return exists, nil
}

// CountFollowers returns the number of followers for a given entity.
func (r *FollowsRepository) CountFollowers(ctx context.Context, followedType, followedID string) (int, error) {
	query := `
		SELECT COUNT(*) FROM follows
		WHERE followed_type = $1 AND followed_id = $2
	`

	var count int
	err := r.pool.QueryRow(ctx, query, followedType, followedID).Scan(&count)
	if err != nil {
		return 0, err
	}

	return count, nil
}

// CountFollowing returns the number of entities that the given user/agent is following.
func (r *FollowsRepository) CountFollowing(ctx context.Context, followerType, followerID string) (int, error) {
	query := `
		SELECT COUNT(*) FROM follows
		WHERE follower_type = $1 AND follower_id = $2
	`

	var count int
	err := r.pool.QueryRow(ctx, query, followerType, followerID).Scan(&count)
	if err != nil {
		return 0, err
	}

	return count, nil
}
