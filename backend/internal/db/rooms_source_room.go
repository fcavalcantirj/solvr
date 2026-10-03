package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// FindPublicRoomTemplate returns the reusable task structure of a room ONLY when the room
// is public and exists (not deleted, not expired): its title, description, category, tags
// and the body of its first non-deleted message. Any other room — private, deleted,
// expired, missing — returns ErrRoomNotFound, so "Try this workflow" can never read a
// protected room. Memberships, credentials, pins, events and results are not selected.
func (r *RoomRepository) FindPublicRoomTemplate(ctx context.Context, slug string) (*models.RoomTemplate, error) {
	var t models.RoomTemplate
	err := r.pool.QueryRow(ctx, `
		SELECT rooms.id, rooms.slug, rooms.display_name, rooms.description, rooms.category,
		       rooms.tags,
		       COALESCE((SELECT m.content FROM messages m
		                  WHERE m.room_id = rooms.id AND m.deleted_at IS NULL
		                  ORDER BY m.id ASC LIMIT 1), '')
		  FROM rooms
		 WHERE rooms.slug = $1 AND rooms.is_private = false
		   AND rooms.deleted_at IS NULL AND (rooms.expires_at IS NULL OR rooms.expires_at > NOW())`,
		slug,
	).Scan(&t.RoomID, &t.Slug, &t.DisplayName, &t.Description, &t.Category, &t.Tags, &t.InitialTask)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrRoomNotFound
		}
		LogQueryError(ctx, "FindPublicRoomTemplate", "rooms", err)
		return nil, fmt.Errorf("find public room template: %w", err)
	}
	if t.Tags == nil {
		t.Tags = []string{}
	}
	return &t, nil
}

// PublicRoomSlugs reports which of the given slugs name a public, existing room, in one
// query. A slug absent from the answer is private, deleted, expired or unknown — callers
// treat all of those alike, so the answer never says which.
func (r *RoomRepository) PublicRoomSlugs(ctx context.Context, slugs []string) (map[string]bool, error) {
	out := map[string]bool{}
	if len(slugs) == 0 {
		return out, nil
	}
	rows, err := r.pool.Query(ctx, `
		SELECT slug FROM rooms
		 WHERE slug = ANY($1) AND is_private = false
		   AND deleted_at IS NULL AND (expires_at IS NULL OR expires_at > NOW())`,
		slugs,
	)
	if err != nil {
		LogQueryError(ctx, "PublicRoomSlugs", "rooms", err)
		return nil, fmt.Errorf("public room slugs: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, fmt.Errorf("scan public room slug: %w", err)
		}
		out[s] = true
	}
	return out, rows.Err()
}
