// Package db provides database access for Solvr.
package db

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Tag represents a tag in the database.
type Tag struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	UsageCount int       `json:"usage_count"`
	CreatedAt  time.Time `json:"created_at"`
}

// TagsRepository handles database operations for tags.
type TagsRepository struct {
	pool *Pool
}

// NewTagsRepository creates a new TagsRepository.
func NewTagsRepository(pool *Pool) *TagsRepository {
	return &TagsRepository{pool: pool}
}

// GetOrCreateTag retrieves an existing tag or creates a new one.
// Returns the tag with its ID.
func (r *TagsRepository) GetOrCreateTag(ctx context.Context, name string) (*Tag, error) {
	// Try to get existing tag first
	var tag Tag
	err := r.pool.QueryRow(ctx, `
		SELECT id, name, usage_count, created_at
		FROM tags
		WHERE name = $1
	`, name).Scan(&tag.ID, &tag.Name, &tag.UsageCount, &tag.CreatedAt)

	if err == nil {
		return &tag, nil
	}

	// Tag doesn't exist, create it
	id := uuid.New().String()
	err = r.pool.QueryRow(ctx, `
		INSERT INTO tags (id, name, usage_count, created_at)
		VALUES ($1, $2, 0, NOW())
		ON CONFLICT (name) DO UPDATE SET name = EXCLUDED.name
		RETURNING id, name, usage_count, created_at
	`, id, name).Scan(&tag.ID, &tag.Name, &tag.UsageCount, &tag.CreatedAt)

	if err != nil {
		return nil, fmt.Errorf("create tag: %w", err)
	}

	return &tag, nil
}

// AddTagsToPost links tags to a post and increments usage counts.
// Creates tags if they don't exist.
func (r *TagsRepository) AddTagsToPost(ctx context.Context, postID string, tagNames []string) error {
	for _, name := range tagNames {
		// Get or create the tag
		tag, err := r.GetOrCreateTag(ctx, name)
		if err != nil {
			return fmt.Errorf("get or create tag %s: %w", name, err)
		}

		// Link tag to post (ignore if already linked)
		_, err = r.pool.Exec(ctx, `
			INSERT INTO post_tags (post_id, tag_id, created_at)
			VALUES ($1, $2, NOW())
			ON CONFLICT (post_id, tag_id) DO NOTHING
		`, postID, tag.ID)
		if err != nil {
			return fmt.Errorf("link tag %s to post: %w", name, err)
		}

		// Increment usage count
		_, err = r.pool.Exec(ctx, `
			UPDATE tags SET usage_count = usage_count + 1 WHERE id = $1
		`, tag.ID)
		if err != nil {
			return fmt.Errorf("increment usage count for %s: %w", name, err)
		}
	}

	return nil
}

// GetTagsForPost returns all tags for a post.
func (r *TagsRepository) GetTagsForPost(ctx context.Context, postID string) ([]Tag, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT t.id, t.name, t.usage_count, t.created_at
		FROM tags t
		JOIN post_tags pt ON t.id = pt.tag_id
		WHERE pt.post_id = $1
		ORDER BY t.name
	`, postID)
	if err != nil {
		return nil, fmt.Errorf("query post tags: %w", err)
	}
	defer rows.Close()

	var tags []Tag
	for rows.Next() {
		var tag Tag
		if err := rows.Scan(&tag.ID, &tag.Name, &tag.UsageCount, &tag.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan tag: %w", err)
		}
		tags = append(tags, tag)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate tags: %w", err)
	}

	return tags, nil
}

// GetTrendingTags returns the most used tags sorted by usage count.
func (r *TagsRepository) GetTrendingTags(ctx context.Context, limit int) ([]Tag, error) {
	if limit <= 0 {
		limit = 10
	}
	if limit > 50 {
		limit = 50
	}

	rows, err := r.pool.Query(ctx, `
		SELECT id, name, usage_count, created_at
		FROM tags
		WHERE usage_count > 0
		ORDER BY usage_count DESC, name ASC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("query trending tags: %w", err)
	}
	defer rows.Close()

	var tags []Tag
	for rows.Next() {
		var tag Tag
		if err := rows.Scan(&tag.ID, &tag.Name, &tag.UsageCount, &tag.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan tag: %w", err)
		}
		tags = append(tags, tag)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate tags: %w", err)
	}

	return tags, nil
}

// RemoveTagsFromPost removes tags from a post and decrements usage counts.
func (r *TagsRepository) RemoveTagsFromPost(ctx context.Context, postID string) error {
	// Get current tags for the post
	rows, err := r.pool.Query(ctx, `
		SELECT tag_id FROM post_tags WHERE post_id = $1
	`, postID)
	if err != nil {
		return fmt.Errorf("query post tags: %w", err)
	}
	defer rows.Close()

	var tagIDs []string
	for rows.Next() {
		var tagID string
		if err := rows.Scan(&tagID); err != nil {
			return fmt.Errorf("scan tag id: %w", err)
		}
		tagIDs = append(tagIDs, tagID)
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate tag ids: %w", err)
	}

	// Delete post_tags entries
	_, err = r.pool.Exec(ctx, `DELETE FROM post_tags WHERE post_id = $1`, postID)
	if err != nil {
		return fmt.Errorf("delete post tags: %w", err)
	}

	// Decrement usage counts
	for _, tagID := range tagIDs {
		_, err = r.pool.Exec(ctx, `
			UPDATE tags SET usage_count = GREATEST(usage_count - 1, 0) WHERE id = $1
		`, tagID)
		if err != nil {
			return fmt.Errorf("decrement usage count: %w", err)
		}
	}

	return nil
}
