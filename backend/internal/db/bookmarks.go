// Package db provides database access for Solvr.
package db

import (
	"context"
	"errors"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Bookmark-related errors.
var (
	ErrBookmarkExists   = errors.New("bookmark already exists")
	ErrBookmarkNotFound = errors.New("bookmark not found")
)

// BookmarkRepository handles database operations for bookmarks.
type BookmarkRepository struct {
	pool *Pool
}

// NewBookmarkRepository creates a new BookmarkRepository.
func NewBookmarkRepository(pool *Pool) *BookmarkRepository {
	return &BookmarkRepository{pool: pool}
}

// Add creates a new bookmark.
func (r *BookmarkRepository) Add(ctx context.Context, userType, userID, postID string) (*models.Bookmark, error) {
	query := `
		INSERT INTO bookmarks (user_type, user_id, post_id)
		VALUES ($1, $2, $3)
		RETURNING id, user_type, user_id, post_id, created_at
	`

	var bookmark models.Bookmark
	err := r.pool.QueryRow(ctx, query, userType, userID, postID).Scan(
		&bookmark.ID,
		&bookmark.UserType,
		&bookmark.UserID,
		&bookmark.PostID,
		&bookmark.CreatedAt,
	)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			// Unique constraint violation
			return nil, ErrBookmarkExists
		}
		return nil, err
	}

	return &bookmark, nil
}

// Remove deletes a bookmark.
func (r *BookmarkRepository) Remove(ctx context.Context, userType, userID, postID string) error {
	query := `
		DELETE FROM bookmarks
		WHERE user_type = $1 AND user_id = $2 AND post_id = $3
	`

	result, err := r.pool.Exec(ctx, query, userType, userID, postID)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return ErrBookmarkNotFound
	}

	return nil
}

// ListByUser returns all bookmarks for a user with post details.
func (r *BookmarkRepository) ListByUser(ctx context.Context, userType, userID string, page, perPage int) ([]models.BookmarkWithPost, int, error) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 20
	}
	if perPage > 100 {
		perPage = 100
	}
	offset := (page - 1) * perPage

	// Count total bookmarks
	countQuery := `
		SELECT COUNT(*) FROM bookmarks
		WHERE user_type = $1 AND user_id = $2
	`
	var total int
	err := r.pool.QueryRow(ctx, countQuery, userType, userID).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	// Get bookmarks with post details
	query := `
		SELECT
			b.id, b.user_type, b.user_id, b.post_id, b.created_at,
			p.id, p.type, p.title, p.description, p.tags,
			p.posted_by_type, p.posted_by_id, p.status,
			p.upvotes, p.downvotes, p.created_at, p.updated_at,
			COALESCE(u.display_name, a.display_name, 'Unknown') as author_name
		FROM bookmarks b
		JOIN posts p ON b.post_id = p.id
		LEFT JOIN users u ON p.posted_by_type = 'human' AND p.posted_by_id = u.id::text
		LEFT JOIN agents a ON p.posted_by_type = 'agent' AND p.posted_by_id = a.id
		WHERE b.user_type = $1 AND b.user_id = $2 AND p.deleted_at IS NULL
		ORDER BY b.created_at DESC
		LIMIT $3 OFFSET $4
	`

	rows, err := r.pool.Query(ctx, query, userType, userID, perPage, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var bookmarks []models.BookmarkWithPost
	for rows.Next() {
		var bwp models.BookmarkWithPost
		var authorName string

		err := rows.Scan(
			&bwp.ID, &bwp.UserType, &bwp.UserID, &bwp.PostID, &bwp.CreatedAt,
			&bwp.Post.ID, &bwp.Post.Type, &bwp.Post.Title, &bwp.Post.Description, &bwp.Post.Tags,
			&bwp.Post.PostedByType, &bwp.Post.PostedByID, &bwp.Post.Status,
			&bwp.Post.Upvotes, &bwp.Post.Downvotes, &bwp.Post.CreatedAt, &bwp.Post.UpdatedAt,
			&authorName,
		)
		if err != nil {
			return nil, 0, err
		}

		bwp.Post.Author = models.PostAuthor{
			Type:        bwp.Post.PostedByType,
			ID:          bwp.Post.PostedByID,
			DisplayName: authorName,
		}
		bwp.Post.VoteScore = bwp.Post.Upvotes - bwp.Post.Downvotes

		bookmarks = append(bookmarks, bwp)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	if bookmarks == nil {
		bookmarks = []models.BookmarkWithPost{}
	}

	return bookmarks, total, nil
}

// IsBookmarked checks if a post is bookmarked by a user.
func (r *BookmarkRepository) IsBookmarked(ctx context.Context, userType, userID, postID string) (bool, error) {
	query := `
		SELECT EXISTS(
			SELECT 1 FROM bookmarks
			WHERE user_type = $1 AND user_id = $2 AND post_id = $3
		)
	`

	var exists bool
	err := r.pool.QueryRow(ctx, query, userType, userID, postID).Scan(&exists)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, err
	}

	return exists, nil
}
