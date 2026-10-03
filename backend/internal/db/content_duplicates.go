// Package db provides database access for Solvr.
package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/jackc/pgx/v5"
)

// ContentDuplicateRepository finds the earlier live post or reply that a new one repeats,
// reading the authoritative posts and replies tables (no separate hash store to rebuild).
type ContentDuplicateRepository struct {
	pool *Pool
}

// NewContentDuplicateRepository creates a ContentDuplicateRepository.
func NewContentDuplicateRepository(pool *Pool) *ContentDuplicateRepository {
	return &ContentDuplicateRepository{pool: pool}
}

// FindPost returns the earliest live post created at or after since, other than excludeID,
// with the same title and description; nil when there is none.
func (r *ContentDuplicateRepository) FindPost(ctx context.Context, title, description string, since time.Time, excludeID string) (*models.ContentDuplicate, error) {
	match := models.ContentDuplicate{TargetType: "post"}
	err := r.pool.QueryRow(ctx, `
		SELECT id::text, created_at FROM posts
		WHERE deleted_at IS NULL AND title = $1 AND description = $2
			AND created_at >= $3 AND id::text <> $4
		ORDER BY created_at, id
		LIMIT 1`, title, description, since, excludeID).Scan(&match.TargetID, &match.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find duplicate post: %w", err)
	}
	match.PostID = match.TargetID
	return &match, nil
}

// FindReply returns the earliest live reply on postID created at or after since, other
// than excludeID, with the same body; nil when there is none. System replies are skipped:
// moderation writes the same verdict text on every post.
func (r *ContentDuplicateRepository) FindReply(ctx context.Context, postID, body string, since time.Time, excludeID string) (*models.ContentDuplicate, error) {
	match := models.ContentDuplicate{TargetType: "reply"}
	err := r.pool.QueryRow(ctx, `
		SELECT id::text, post_id::text, created_at FROM replies
		WHERE post_id = $1 AND deleted_at IS NULL AND author_type <> 'system' AND body = $2
			AND created_at >= $3 AND id::text <> $4
		ORDER BY created_at, id
		LIMIT 1`, postID, body, since, excludeID).Scan(&match.TargetID, &match.PostID, &match.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find duplicate reply: %w", err)
	}
	return &match, nil
}

// Anti-abuse W1 (contentgate): same-author repeats across every live post or contribution,
// with the normalization done here in SQL on both sides so one definition decides equality.
// A title is compared lowercased, each digit replaced by '#', whitespace collapsed and
// trimmed (purge rule R4); a body lowercased with whitespace collapsed and trimmed (digits
// kept: code and versions matter in answers).
const (
	normTitleSQL = `btrim(regexp_replace(regexp_replace(lower(%s), '[0-9]', '#', 'g'), '\s+', ' ', 'g'))`
	normBodySQL  = `btrim(regexp_replace(lower(%s), '\s+', ' ', 'g'))`
)

func normTitle(expr string) string { return fmt.Sprintf(normTitleSQL, expr) }
func normBody(expr string) string  { return fmt.Sprintf(normBodySQL, expr) }

// findOne scans (target type, id, post id, created at) from a query, nil on no rows.
func (r *ContentDuplicateRepository) findOne(ctx context.Context, op, query string, args ...any) (*models.ContentDuplicate, error) {
	var match models.ContentDuplicate
	err := r.pool.QueryRow(ctx, query, args...).Scan(&match.TargetType, &match.TargetID, &match.PostID, &match.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		LogQueryError(ctx, op, "content_duplicates", err)
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	return &match, nil
}

// FindAuthorPostByTitle returns the author's earliest live post (any moderation state) whose
// normalized title equals title's.
func (r *ContentDuplicateRepository) FindAuthorPostByTitle(ctx context.Context, authorType, authorID, title string) (*models.ContentDuplicate, error) {
	return r.findOne(ctx, "FindAuthorPostByTitle", `
		SELECT 'post', id::text, id::text, created_at FROM posts
		WHERE posted_by_type = $1 AND posted_by_id = $2 AND deleted_at IS NULL
		  AND `+normTitle("title")+` = `+normTitle("$3::text")+`
		ORDER BY created_at, id LIMIT 1`, authorType, authorID, title)
}

// FindAuthorBlogByTitle is FindAuthorPostByTitle over the author's blog posts.
func (r *ContentDuplicateRepository) FindAuthorBlogByTitle(ctx context.Context, authorType, authorID, title string) (*models.ContentDuplicate, error) {
	return r.findOne(ctx, "FindAuthorBlogByTitle", `
		SELECT 'blog_post', id::text, '', created_at FROM blog_posts
		WHERE posted_by_type = $1 AND posted_by_id = $2 AND deleted_at IS NULL
		  AND `+normTitle("title")+` = `+normTitle("$3::text")+`
		ORDER BY created_at, id LIMIT 1`, authorType, authorID, title)
}

// FindAuthorCounterTitle returns the author's earliest live post (table "posts") or blog post
// (table "blog_posts") whose lowercased title matches pattern (contentgate.DayCounterPattern).
func (r *ContentDuplicateRepository) FindAuthorCounterTitle(ctx context.Context, table, authorType, authorID, pattern string) (*models.ContentDuplicate, error) {
	targetType, postID := "post", "id::text"
	switch table {
	case "posts":
	case "blog_posts":
		targetType, postID = "blog_post", "''"
	default:
		return nil, fmt.Errorf("FindAuthorCounterTitle: unknown table %q", table)
	}
	return r.findOne(ctx, "FindAuthorCounterTitle", `
		SELECT '`+targetType+`', id::text, `+postID+`, created_at FROM `+table+`
		WHERE posted_by_type = $1 AND posted_by_id = $2 AND deleted_at IS NULL AND lower(title) ~ $3
		ORDER BY created_at, id LIMIT 1`, authorType, authorID, pattern)
}

// FindAuthorContribution returns the author's earliest live reply, on any post, whose
// normalized text equals body's.
func (r *ContentDuplicateRepository) FindAuthorContribution(ctx context.Context, authorType, authorID, body string) (*models.ContentDuplicate, error) {
	return r.findOne(ctx, "FindAuthorContribution", `
		WITH n AS (SELECT `+normBody("$3::text")+` AS b)
		SELECT 'reply', x.id::text, x.post_id::text, x.created_at FROM replies x, n
		WHERE x.author_type = $1 AND x.author_id = $2 AND x.deleted_at IS NULL AND `+normBody("x.body")+` = n.b
		ORDER BY x.created_at, x.id LIMIT 1`, authorType, authorID, body)
}

// RecentTitlesByAuthor returns up to limit of the author's live post titles other than
// excludePostID, newest first: content moderation sees them to judge repeats (prompt rule 7).
func (r *ContentDuplicateRepository) RecentTitlesByAuthor(ctx context.Context, authorType, authorID, excludePostID string, limit int) ([]string, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT title FROM posts
		WHERE posted_by_type = $1 AND posted_by_id = $2 AND deleted_at IS NULL AND id::text <> $3
		ORDER BY created_at DESC, id DESC LIMIT $4`, authorType, authorID, excludePostID, limit)
	if err != nil {
		LogQueryError(ctx, "RecentTitlesByAuthor", "posts", err)
		return nil, fmt.Errorf("recent titles: %w", err)
	}
	defer rows.Close()
	titles := []string{}
	for rows.Next() {
		var title string
		if err := rows.Scan(&title); err != nil {
			return nil, fmt.Errorf("recent titles: %w", err)
		}
		titles = append(titles, title)
	}
	return titles, rows.Err()
}
