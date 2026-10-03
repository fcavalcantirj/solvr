package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/jackc/pgx/v5"
)

// ErrNotHideable is returned for a contribution kind moderation cannot hide: only replies
// remain contributions.
var ErrNotHideable = errors.New("contribution kind cannot be hidden")

// ContributionModerationRepository serves asynchronous moderation of replies (anti-abuse W2):
// it hides what moderation rejects, names the post a reply belongs to, and records the verdict
// as an admin flag.
type ContributionModerationRepository struct {
	pool *Pool
}

// NewContributionModerationRepository creates a ContributionModerationRepository.
func NewContributionModerationRepository(pool *Pool) *ContributionModerationRepository {
	return &ContributionModerationRepository{pool: pool}
}

// Hide soft-deletes a rejected reply, like its author's delete.
func (r *ContributionModerationRepository) Hide(ctx context.Context, kind, id string) error {
	if kind != "reply" {
		return ErrNotHideable
	}
	if _, err := r.pool.Exec(ctx, `UPDATE replies SET deleted_at = NOW() WHERE id::text = $1 AND deleted_at IS NULL`, id); err != nil {
		LogQueryError(ctx, "Hide", "replies", err)
		return fmt.Errorf("hide %s: %w", kind, err)
	}
	return nil
}

// ParentPost returns the id, type and title of the post a reply belongs to. Empty values when
// it cannot be resolved, and for any other kind.
func (r *ContributionModerationRepository) ParentPost(ctx context.Context, kind, id string) (postID, postType, title string, err error) {
	if kind != "reply" {
		return "", "", "", nil
	}
	err = r.pool.QueryRow(ctx, `SELECT p.id::text, p.type, p.title FROM posts p JOIN replies x ON x.post_id = p.id
		WHERE x.id::text = $1`, id).Scan(&postID, &postType, &title)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", "", nil
	}
	if err != nil {
		LogQueryError(ctx, "ParentPost", "posts", err)
		return "", "", "", fmt.Errorf("parent post of %s: %w", kind, err)
	}
	return postID, postType, title, nil
}

// CreateFlag records an admin flag (reporter 'system' for moderation verdicts).
func (r *ContributionModerationRepository) CreateFlag(ctx context.Context, flag *models.Flag) (*models.Flag, error) {
	created := *flag
	err := r.pool.QueryRow(ctx, `
		INSERT INTO flags (target_type, target_id, reporter_type, reporter_id, reason, details, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at`,
		flag.TargetType, flag.TargetID, flag.ReporterType, flag.ReporterID, flag.Reason, flag.Details, flag.Status,
	).Scan(&created.ID, &created.CreatedAt)
	if err != nil {
		LogQueryError(ctx, "CreateFlag", "flags", err)
		return nil, fmt.Errorf("create flag: %w", err)
	}
	return &created, nil
}
