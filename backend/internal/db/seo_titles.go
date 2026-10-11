package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/fcavalcantirj/solvr/internal/seo"
	"github.com/jackc/pgx/v5"
)

// TitleTwins counts the other indexable posts (sitemapPostEligible) whose search title
// (seo.SearchTitle) matches the post's, ignoring case, and how many of those share its author
// (task idx 82). seo.PostTitle uses them to keep every indexable title unique. Two titles share
// a search title only when both start with the post's title stem (seo.TitleStem), so only
// those are read.
func (r *PostRepository) TitleTwins(ctx context.Context, postID string) (twins, sameAuthor int, err error) {
	var title string
	if err = r.pool.QueryRow(ctx, `SELECT title FROM posts WHERE id = $1::uuid`, postID).Scan(&title); err != nil {
		LogQueryError(ctx, "TitleTwins", "posts", err)
		return 0, 0, fmt.Errorf("post title twins: %w", err)
	}
	rows, err := r.pool.Query(ctx, `
		SELECT o.title, o.posted_by_type = p.posted_by_type AND o.posted_by_id = p.posted_by_id
		FROM posts p
		JOIN posts o ON o.id <> p.id
		 AND starts_with(lower(regexp_replace(btrim(o.title), '\s+', ' ', 'g')), lower($2))
		WHERE p.id = $1::uuid
		  AND o.id IN (SELECT id FROM posts WHERE `+sitemapPostEligible+`)`, postID, seo.TitleStem(title))
	if err != nil {
		LogQueryError(ctx, "TitleTwins", "posts", err)
		return 0, 0, fmt.Errorf("post title twins: %w", err)
	}
	candidates, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (seo.TitleCandidate, error) {
		var c seo.TitleCandidate
		return c, row.Scan(&c.Title, &c.SameAuthor)
	})
	if err != nil {
		LogQueryError(ctx, "TitleTwins", "posts", err)
		return 0, 0, fmt.Errorf("post title twins: %w", err)
	}
	twins, sameAuthor = seo.CountTitleTwins(title, candidates)
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
