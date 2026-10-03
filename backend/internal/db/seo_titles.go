package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// TitleTwins counts the other indexable posts (sitemapPostEligible) whose title matches
// the post's, ignoring case and surrounding spaces, and how many of those share its
// author (task idx 82). seo.PostTitle uses them to keep every indexable title unique.
func (r *PostRepository) TitleTwins(ctx context.Context, postID string) (twins, sameAuthor int, err error) {
	err = r.pool.QueryRow(ctx, `
		SELECT COUNT(*),
		       COUNT(*) FILTER (WHERE o.posted_by_type = p.posted_by_type AND o.posted_by_id = p.posted_by_id)
		FROM posts p
		JOIN posts o ON o.id <> p.id AND lower(btrim(o.title)) = lower(btrim(p.title))
		WHERE p.id = $1::uuid
		  AND o.id IN (SELECT id FROM posts WHERE `+sitemapPostEligible+`)`, postID).Scan(&twins, &sameAuthor)
	if err != nil {
		LogQueryError(ctx, "TitleTwins", "posts", err)
		return 0, 0, fmt.Errorf("post title twins: %w", err)
	}
	return twins, sameAuthor, nil
}

// FindPublicSourceRoom returns the public, live room a post was saved from
// (posts.source_room_id), or nil when it has none or that room is private or gone, so
// linking an outcome back to its conversation never reveals a private collaboration.
func (r *RoomRepository) FindPublicSourceRoom(ctx context.Context, postID string) (*SourceRoom, error) {
	var room SourceRoom
	err := r.pool.QueryRow(ctx, `
		SELECT r.slug, r.display_name
		FROM posts p JOIN rooms r ON r.id = p.source_room_id
		WHERE p.id = $1::uuid AND r.is_private = false
		  AND r.deleted_at IS NULL AND (r.expires_at IS NULL OR r.expires_at > NOW())`, postID).Scan(&room.Slug, &room.DisplayName)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		LogQueryError(ctx, "FindPublicSourceRoom", "rooms", err)
		return nil, fmt.Errorf("post source room: %w", err)
	}
	return &room, nil
}

// SourceRoom names the room a post was saved from.
type SourceRoom struct {
	Slug        string `json:"slug"`
	DisplayName string `json:"display_name"`
}
