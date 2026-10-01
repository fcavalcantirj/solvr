// Package db provides database access for Solvr.
package db

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// ViewsRepository handles database operations for view tracking.
type ViewsRepository struct {
	pool *Pool
}

// NewViewsRepository creates a new ViewsRepository.
func NewViewsRepository(pool *Pool) *ViewsRepository {
	return &ViewsRepository{pool: pool}
}

// RecordView records a view for a post and returns the updated view count.
// If the user has already viewed the post, it returns the current count without incrementing.
// The count is moved by the view row's own insert (migration 000120's trigger), so a view
// that is stored is counted even when the caller leaves before reading the count.
func (r *ViewsRepository) RecordView(ctx context.Context, postID, viewerType, viewerID string) (int, error) {
	// Use ON CONFLICT DO NOTHING to handle duplicate views
	insertQuery := `
		INSERT INTO post_views (post_id, viewer_type, viewer_id)
		VALUES ($1, $2, $3)
		ON CONFLICT (post_id, viewer_type, viewer_id) DO NOTHING
	`

	_, err := r.pool.Exec(ctx, insertQuery, postID, viewerType, viewerID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			// Handle invalid UUID (22P02) and a post that does not exist (23503,
			// post_views.post_id references posts).
			if pgErr.Code == "22P02" || pgErr.Code == "23503" {
				return 0, ErrPostNotFound
			}
		}
		return 0, err
	}

	return r.GetViewCount(ctx, postID)
}

// RecordAnonymousView records a view from an anonymous user.
// Anonymous views are tracked by a session identifier.
func (r *ViewsRepository) RecordAnonymousView(ctx context.Context, postID, sessionID string) (int, error) {
	return r.RecordView(ctx, postID, "anonymous", sessionID)
}

// GetViewCount returns the view count for a post.
func (r *ViewsRepository) GetViewCount(ctx context.Context, postID string) (int, error) {
	query := `SELECT view_count FROM posts WHERE id = $1`

	var viewCount int
	err := r.pool.QueryRow(ctx, query, postID).Scan(&viewCount)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) || isInvalidUUIDError(err) {
			return 0, ErrPostNotFound
		}
		return 0, err
	}

	return viewCount, nil
}
