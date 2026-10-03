package db

import (
	"context"
	"errors"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/jackc/pgx/v5"
)

// UpdateIfUnmodified writes a post's mutable fields (title, description, tags,
// status, the embedding when one is given, and the canonical states derived from status)
// only while the row is still at the expected version (its updated_at), in the
// same statement, so two edits that read the same version cannot both land
// (spec.json idx 74 step 5). A nil expected writes unconditionally.
// Returns ErrPostNotFound if the post doesn't exist or is soft-deleted, and a
// *models.VersionConflictError carrying the current version when it moved.
func (r *PostRepository) UpdateIfUnmodified(ctx context.Context, post *models.Post, expected *time.Time) (*models.Post, error) {
	query := `
		UPDATE posts
		SET
			title = $2,
			description = $3,
			tags = $4,
			status = $5,
			embedding = COALESCE($6::vector, embedding),
			publication_state = $7,
			moderation_state = $8,
			updated_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL
		  AND ($9::timestamptz IS NULL OR updated_at = $9::timestamptz)
		RETURNING id, type, title, description, tags,
			posted_by_type, posted_by_id, status,
			upvotes, downvotes, view_count,
			created_at, updated_at, deleted_at,
			crystallization_cid, crystallized_at, visibility,
			publication_state, moderation_state, source_room_id
	`

	// Keep the canonical states consistent with the new status (BART-583). The Update
	// request never carries moderation_state, so an author edit cannot self-approve.
	pub, mod := models.DeriveStates(post.Status)

	row := r.pool.QueryRow(ctx, query,
		post.ID,
		post.Title,
		post.Description,
		post.Tags,
		post.Status,
		post.EmbeddingStr,
		pub,
		mod,
		expected,
	)

	updated, err := r.scanPost(row)
	if errors.Is(err, ErrPostNotFound) && expected != nil {
		return nil, r.versionConflictOrNotFound(ctx, post.ID)
	}
	return updated, err
}

// versionConflictOrNotFound explains a conditional post write that matched no
// row: a live post that moved is a version conflict, anything else not found.
func (r *PostRepository) versionConflictOrNotFound(ctx context.Context, id string) error {
	var current time.Time
	err := r.pool.QueryRow(ctx,
		"SELECT updated_at FROM posts WHERE id = $1 AND deleted_at IS NULL", id).Scan(&current)
	if err == nil {
		return &models.VersionConflictError{Current: current}
	}
	if errors.Is(err, pgx.ErrNoRows) || isInvalidUUIDError(err) {
		return ErrPostNotFound
	}
	LogQueryError(ctx, "UpdateIfUnmodified.version", "posts", err)
	return err
}
