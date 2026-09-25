package db

import (
	"context"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// FindPublicRoomsBySourcePost returns the PUBLIC, existing (not deleted, not expired) rooms started from the
// given post via "Discuss with agents", newest activity first. This backs the "post can
// display its related public rooms" relationship. Private rooms are excluded so linking a
// post to a private collaboration never reveals that collaboration to an unauthorized
// post reader. source_post_id is a soft provenance pointer (no FK), so a pruned post
// simply yields an empty list.
func (r *RoomRepository) FindPublicRoomsBySourcePost(ctx context.Context, postID string) ([]*models.Room, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, slug, display_name, description, category, tags, is_private,
			message_count, created_at, updated_at, last_active_at, archived_at, source_post_id
		 FROM rooms
		 WHERE source_post_id = $1::uuid
		   AND is_private = FALSE
		   AND `+roomExistsPredicate+`
		 ORDER BY last_active_at DESC`,
		postID,
	)
	if err != nil {
		LogQueryError(ctx, "FindPublicRoomsBySourcePost", "rooms", err)
		return nil, err
	}
	defer rows.Close()

	rooms := make([]*models.Room, 0)
	for rows.Next() {
		var room models.Room
		if err := rows.Scan(
			&room.ID,
			&room.Slug,
			&room.DisplayName,
			&room.Description,
			&room.Category,
			&room.Tags,
			&room.IsPrivate,
			&room.MessageCount,
			&room.CreatedAt,
			&room.UpdatedAt,
			&room.LastActiveAt,
			&room.ArchivedAt,
			&room.SourcePostID,
		); err != nil {
			return nil, err
		}
		rooms = append(rooms, &room)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return rooms, nil
}

// FindPublicPostRef returns the id and title of a post ONLY when it is publicly readable:
// published, approved, public visibility, and not deleted. Any other post (draft, rejected,
// family, private, deleted, missing) returns ErrPostNotFound so protected content can never
// seed the public /connect contract. This is the eligibility gate for post-seeded starts.
func (r *PostRepository) FindPublicPostRef(ctx context.Context, id string) (postID, title string, err error) {
	err = r.pool.QueryRow(ctx,
		`SELECT id::text, title FROM posts
		 WHERE id = $1::uuid
		   AND publication_state = 'published'
		   AND moderation_state = 'approved'
		   AND visibility = 'public'
		   AND deleted_at IS NULL
		 LIMIT 1`,
		id,
	).Scan(&postID, &title)
	if err != nil {
		// A malformed UUID or no matching public row both mean "not a public post" — the
		// caller degrades gracefully rather than leaking whether the id exists.
		return "", "", ErrPostNotFound
	}
	return postID, title, nil
}
