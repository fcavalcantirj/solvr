package db

import (
	"context"
	"errors"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/jackc/pgx/v5"
)

// FindByIdempotencyKey returns the live post an author created earlier under the given
// Idempotency-Key, or ErrPostNotFound when none exists. Used to make "Save as post"
// retries return the existing outcome draft instead of creating a duplicate.
func (r *PostRepository) FindByIdempotencyKey(ctx context.Context, authorType, authorID, key string) (*models.PostWithAuthor, error) {
	var id string
	err := r.pool.QueryRow(ctx,
		`SELECT id FROM posts
		 WHERE posted_by_type = $1 AND posted_by_id = $2 AND idempotency_key = $3 AND deleted_at IS NULL
		 LIMIT 1`,
		authorType, authorID, key,
	).Scan(&id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrPostNotFound
		}
		LogQueryError(ctx, "FindByIdempotencyKey", "posts", err)
		return nil, err
	}
	return r.FindByID(ctx, id)
}

// FindPublishedBySourceRoom returns the publicly eligible outcome posts saved from a room,
// newest first. This backs the "room links to the published outcome" relationship; drafts,
// rejected, family, and deleted posts are excluded so no unpublished content leaks.
func (r *PostRepository) FindPublishedBySourceRoom(ctx context.Context, roomID string) ([]*models.PostWithAuthor, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id FROM posts
		 WHERE source_room_id = $1::uuid
		   AND publication_state = 'published'
		   AND moderation_state = 'approved'
		   AND visibility = 'public'
		   AND deleted_at IS NULL
		 ORDER BY created_at DESC`,
		roomID,
	)
	if err != nil {
		LogQueryError(ctx, "FindPublishedBySourceRoom", "posts", err)
		return nil, err
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	posts := make([]*models.PostWithAuthor, 0, len(ids))
	for _, id := range ids {
		p, err := r.FindByID(ctx, id)
		if err != nil {
			if errors.Is(err, ErrPostNotFound) {
				continue
			}
			return nil, err
		}
		posts = append(posts, p)
	}
	return posts, nil
}
