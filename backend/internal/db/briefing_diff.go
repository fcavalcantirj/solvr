package db

import (
	"context"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// BriefingDiffRepository handles queries for the GET /v1/me/diff endpoint.
// Provides lightweight count-based queries for delta-only polling.
type BriefingDiffRepository struct {
	pool *Pool
}

// NewBriefingDiffRepository creates a new BriefingDiffRepository.
func NewBriefingDiffRepository(pool *Pool) *BriefingDiffRepository {
	return &BriefingDiffRepository{pool: pool}
}

// CountNewSince returns the number of notifications created for the agent since the given time.
func (r *BriefingDiffRepository) CountNewSince(ctx context.Context, agentID string, since time.Time) (int, error) {
	query := `SELECT COUNT(*) FROM notifications WHERE agent_id = $1 AND created_at > $2`
	var count int
	err := r.pool.QueryRow(ctx, query, agentID, since).Scan(&count)
	if err != nil {
		LogQueryError(ctx, "CountNewSince", "notifications", err)
		return 0, err
	}
	return count, nil
}

// CountNewOpportunitiesSince returns the count of open problems matching agent specialties
// that were created since the given time.
func (r *BriefingDiffRepository) CountNewOpportunitiesSince(ctx context.Context, agentID string, specialties []string, since time.Time) (int, error) {
	if len(specialties) == 0 {
		return 0, nil
	}

	query := `
		SELECT COUNT(*)
		FROM posts
		WHERE type = 'problem'
			AND status IN ('open', 'in_progress')
			AND deleted_at IS NULL
			AND tags && $1
			AND posted_by_id != $2
			AND created_at > $3
	`
	var count int
	err := r.pool.QueryRow(ctx, query, specialties, agentID, since).Scan(&count)
	if err != nil {
		LogQueryError(ctx, "CountNewOpportunitiesSince", "posts", err)
		return 0, err
	}
	return count, nil
}

// ListAwardedSince returns badges awarded to the given owner since the given time.
func (r *BriefingDiffRepository) ListAwardedSince(ctx context.Context, ownerType, ownerID string, since time.Time) ([]models.Badge, error) {
	query := `
		SELECT id, owner_type, owner_id, badge_type, badge_name, COALESCE(description, '') as description, awarded_at
		FROM badges
		WHERE owner_type = $1 AND owner_id = $2 AND awarded_at > $3
		ORDER BY awarded_at DESC
	`
	rows, err := r.pool.Query(ctx, query, ownerType, ownerID, since)
	if err != nil {
		LogQueryError(ctx, "ListAwardedSince", "badges", err)
		return nil, err
	}
	defer rows.Close()

	var badges []models.Badge
	for rows.Next() {
		var b models.Badge
		err := rows.Scan(&b.ID, &b.OwnerType, &b.OwnerID, &b.BadgeType, &b.BadgeName, &b.Description, &b.AwardedAt)
		if err != nil {
			return nil, err
		}
		badges = append(badges, b)
	}

	if badges == nil {
		badges = []models.Badge{}
	}
	return badges, nil
}

// CountTrendingSince returns the count of trending posts created since the given time.
// Trending is defined as posts with more than 3 upvotes created in the timeframe.
func (r *BriefingDiffRepository) CountTrendingSince(ctx context.Context, since time.Time) (int, error) {
	query := `
		SELECT COUNT(*)
		FROM posts
		WHERE deleted_at IS NULL
			AND created_at > $1
			AND upvotes >= 3
	`
	var count int
	err := r.pool.QueryRow(ctx, query, since).Scan(&count)
	if err != nil {
		LogQueryError(ctx, "CountTrendingSince", "posts", err)
		return 0, err
	}
	return count, nil
}
