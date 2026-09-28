// Package db provides database access for Solvr.
package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/jackc/pgx/v5"
)

// ContentDuplicateRepository finds the earlier live post or reply that a new one repeats,
// reading the authoritative posts and replies tables (no separate hash store to rebuild).
type ContentDuplicateRepository struct {
	pool *Pool
}

// NewContentDuplicateRepository creates a ContentDuplicateRepository.
func NewContentDuplicateRepository(pool *Pool) *ContentDuplicateRepository {
	return &ContentDuplicateRepository{pool: pool}
}

// FindPost returns the earliest live post created at or after since, other than excludeID,
// with the same title and description; nil when there is none.
func (r *ContentDuplicateRepository) FindPost(ctx context.Context, title, description string, since time.Time, excludeID string) (*models.ContentDuplicate, error) {
	match := models.ContentDuplicate{TargetType: "post"}
	err := r.pool.QueryRow(ctx, `
		SELECT id::text, created_at FROM posts
		WHERE deleted_at IS NULL AND title = $1 AND description = $2
			AND created_at >= $3 AND id::text <> $4
		ORDER BY created_at, id
		LIMIT 1`, title, description, since, excludeID).Scan(&match.TargetID, &match.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find duplicate post: %w", err)
	}
	match.PostID = match.TargetID
	return &match, nil
}

// FindReply returns the earliest live reply on postID created at or after since, other
// than excludeID, with the same body; nil when there is none. System replies are skipped:
// moderation writes the same verdict text on every post.
func (r *ContentDuplicateRepository) FindReply(ctx context.Context, postID, body string, since time.Time, excludeID string) (*models.ContentDuplicate, error) {
	match := models.ContentDuplicate{TargetType: "reply"}
	err := r.pool.QueryRow(ctx, `
		SELECT id::text, post_id::text, created_at FROM replies
		WHERE post_id = $1 AND deleted_at IS NULL AND author_type <> 'system' AND body = $2
			AND created_at >= $3 AND id::text <> $4
		ORDER BY created_at, id
		LIMIT 1`, postID, body, since, excludeID).Scan(&match.TargetID, &match.PostID, &match.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find duplicate reply: %w", err)
	}
	return &match, nil
}
