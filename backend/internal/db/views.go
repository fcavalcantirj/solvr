// Package db provides database access for Solvr.
package db

import (
	"context"
	"errors"

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
func (r *ViewsRepository) RecordView(ctx context.Context, postID, viewerType, viewerID string) (int, error) {
	// Use ON CONFLICT DO NOTHING to handle duplicate views
	insertQuery := `
		INSERT INTO post_views (post_id, viewer_type, viewer_id)
		VALUES ($1, $2, $3)
		ON CONFLICT (post_id, viewer_type, viewer_id) DO NOTHING
	`

	result, err := r.pool.Exec(ctx, insertQuery, postID, viewerType, viewerID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			// Handle invalid UUID
			if pgErr.Code == "22P02" {
				return 0, ErrPostNotFound
			}
		}
		return 0, err
	}

	// If a row was inserted, update the view_count
	if result.RowsAffected() > 0 {
		updateQuery := `
			UPDATE posts SET view_count = view_count + 1
			WHERE id = $1
			RETURNING view_count
		`
		var viewCount int
		err = r.pool.QueryRow(ctx, updateQuery, postID).Scan(&viewCount)
		if err != nil {
			return 0, err
		}
		return viewCount, nil
	}

	// Return current view count if duplicate view
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
		return 0, err
	}

	return viewCount, nil
}
