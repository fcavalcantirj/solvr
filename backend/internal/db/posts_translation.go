package db

import (
	"context"
	"fmt"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// UpdateOriginalLanguage sets the post status to 'draft' and records the detected language.
// Called when a post is rejected solely for language and queued for auto-translation.
func (r *PostRepository) UpdateOriginalLanguage(ctx context.Context, postID, language string) error {
	query := `
		UPDATE posts
		SET status = 'draft', original_language = $2, updated_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL
	`

	result, err := r.pool.Exec(ctx, query, postID, language)
	if err != nil {
		if isInvalidUUIDError(err) {
			return ErrPostNotFound
		}
		LogQueryError(ctx, "UpdateOriginalLanguage", "posts", err)
		return fmt.Errorf("update original language failed: %w", err)
	}

	if result.RowsAffected() == 0 {
		return ErrPostNotFound
	}

	return nil
}

// ListPostsNeedingTranslation returns draft posts that have a detected language set
// and have been attempted fewer than 3 times. Ordered by creation time (oldest first).
func (r *PostRepository) ListPostsNeedingTranslation(ctx context.Context, limit int) ([]*models.Post, error) {
	query := `
		SELECT id, type, title, description, tags, posted_by_type, posted_by_id,
		       status, original_language,
		       COALESCE(original_title, '') as original_title,
		       COALESCE(original_description, '') as original_description,
		       translation_attempts
		FROM posts
		WHERE status = 'draft'
		  AND original_language IS NOT NULL
		  AND translation_attempts < 3
		  AND deleted_at IS NULL
		ORDER BY created_at ASC
		LIMIT $1
	`

	rows, err := r.pool.Query(ctx, query, limit)
	if err != nil {
		LogQueryError(ctx, "ListPostsNeedingTranslation", "posts", err)
		return nil, fmt.Errorf("list posts needing translation failed: %w", err)
	}
	defer rows.Close()

	var posts []*models.Post
	for rows.Next() {
		post := &models.Post{}
		err := rows.Scan(
			&post.ID,
			&post.Type,
			&post.Title,
			&post.Description,
			&post.Tags,
			&post.PostedByType,
			&post.PostedByID,
			&post.Status,
			&post.OriginalLanguage,
			&post.OriginalTitle,
			&post.OriginalDescription,
			&post.TranslationAttempts,
		)
		if err != nil {
			LogQueryError(ctx, "ListPostsNeedingTranslation.Scan", "posts", err)
			return nil, fmt.Errorf("scan failed: %w", err)
		}
		posts = append(posts, post)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	return posts, nil
}

// ApplyTranslation stores the translated title/description, preserves the originals,
// and sets the post status back to 'pending_review' to re-trigger moderation.
func (r *PostRepository) ApplyTranslation(ctx context.Context, postID, translatedTitle, translatedDescription string) error {
	query := `
		UPDATE posts
		SET original_title       = COALESCE(original_title, title),
		    original_description = COALESCE(original_description, description),
		    title                = $2,
		    description          = $3,
		    status               = 'pending_review',
		    translation_attempts = translation_attempts + 1,
		    updated_at           = NOW()
		WHERE id = $1 AND deleted_at IS NULL
	`

	result, err := r.pool.Exec(ctx, query, postID, translatedTitle, translatedDescription)
	if err != nil {
		if isInvalidUUIDError(err) {
			return ErrPostNotFound
		}
		LogQueryError(ctx, "ApplyTranslation", "posts", err)
		return fmt.Errorf("apply translation failed: %w", err)
	}

	if result.RowsAffected() == 0 {
		return ErrPostNotFound
	}

	return nil
}

// IncrementTranslationAttempts increments the translation attempt counter for a post.
// Called when a translation attempt fails (non-rate-limit error).
func (r *PostRepository) IncrementTranslationAttempts(ctx context.Context, postID string) error {
	query := `
		UPDATE posts
		SET translation_attempts = translation_attempts + 1, updated_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL
	`

	result, err := r.pool.Exec(ctx, query, postID)
	if err != nil {
		if isInvalidUUIDError(err) {
			return ErrPostNotFound
		}
		LogQueryError(ctx, "IncrementTranslationAttempts", "posts", err)
		return fmt.Errorf("increment translation attempts failed: %w", err)
	}

	if result.RowsAffected() == 0 {
		return ErrPostNotFound
	}

	return nil
}
