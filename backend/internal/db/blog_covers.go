package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// The generated cover of a blog post (000148, SEO audit 2026-11-08). One row per post holds
// the image bytes and what drew them; the post's cover_image_url then names the route that
// serves it (GET /v1/blog/{slug}/cover.jpg), so the page and the link preview show it.

// ErrBlogCoverNotFound is answered for a post with no cover, or no such post.
var ErrBlogCoverNotFound = errors.New("blog cover not found")

// BlogCover is a stored cover.
type BlogCover struct {
	ContentType string
	Model       string
	Prompt      string
	CreatedAt   time.Time
	Image       []byte
}

// BlogCoverBrief is a published post that has no cover yet: what a cover is drawn from.
type BlogCoverBrief struct {
	ID      string
	Slug    string
	Title   string
	Excerpt string
	Tags    []string
}

// BlogCoverRepository stores covers.
type BlogCoverRepository struct {
	pool *Pool
}

// NewBlogCoverRepository creates the store.
func NewBlogCoverRepository(pool *Pool) *BlogCoverRepository {
	return &BlogCoverRepository{pool: pool}
}

// Save stores a post's cover, replacing an earlier one.
func (r *BlogCoverRepository) Save(ctx context.Context, blogPostID string, cover *BlogCover) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO blog_post_covers (blog_post_id, image, content_type, model, prompt)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (blog_post_id) DO UPDATE
		   SET image = EXCLUDED.image, content_type = EXCLUDED.content_type, model = EXCLUDED.model,
		       prompt = EXCLUDED.prompt, created_at = NOW()`,
		blogPostID, cover.Image, cover.ContentType, cover.Model, cover.Prompt)
	if err != nil {
		LogQueryError(ctx, "BlogCovers.Save", "blog_post_covers", err)
		return fmt.Errorf("save blog cover: %w", err)
	}
	return nil
}

// NameCover sets the post's cover_image_url to the address that serves its cover.
func (r *BlogCoverRepository) NameCover(ctx context.Context, blogPostID, url string) error {
	tag, err := r.pool.Exec(ctx, `UPDATE blog_posts SET cover_image_url = $2, updated_at = NOW() WHERE id = $1 AND deleted_at IS NULL`, blogPostID, url)
	if err != nil {
		LogQueryError(ctx, "BlogCovers.NameCover", "blog_posts", err)
		return fmt.Errorf("name blog cover: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrBlogPostNotFound
	}
	return nil
}

// FindBySlug reads a live post's cover.
func (r *BlogCoverRepository) FindBySlug(ctx context.Context, slug string) (*BlogCover, error) {
	var c BlogCover
	err := r.pool.QueryRow(ctx, `
		SELECT c.image, c.content_type, c.model, c.prompt, c.created_at
		  FROM blog_post_covers c JOIN blog_posts p ON p.id = c.blog_post_id
		 WHERE p.slug = $1 AND p.deleted_at IS NULL`, slug).Scan(&c.Image, &c.ContentType, &c.Model, &c.Prompt, &c.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrBlogCoverNotFound
	}
	if err != nil {
		LogQueryError(ctx, "BlogCovers.FindBySlug", "blog_post_covers", err)
		return nil, fmt.Errorf("find blog cover: %w", err)
	}
	return &c, nil
}

// MaxCoverAttempts is how many times the job tries to draw one post's cover.
const MaxCoverAttempts = 3

// blogCoverLock names the advisory lock one cover run holds (the job's or the backfill's).
const blogCoverLock = "solvr:blog_covers"

// TryLock takes the cover lock; ok is false while another run holds it.
func (r *BlogCoverRepository) TryLock(ctx context.Context) (unlock func(), ok bool, err error) {
	return tryAdvisoryLock(ctx, r.pool, blogCoverLock)
}

// ListPublishedWithoutCover lists the published posts with no stored cover, no cover URL of
// their own, published at least grace ago (the author's time to upload one; 0 lists them all),
// and under MaxCoverAttempts tries, oldest publication first.
func (r *BlogCoverRepository) ListPublishedWithoutCover(ctx context.Context, grace time.Duration, limit int) ([]BlogCoverBrief, error) {
	if limit <= 0 {
		limit = 10
	}
	rows, err := r.pool.Query(ctx, `
		SELECT p.id, p.slug, p.title, COALESCE(p.excerpt, ''), COALESCE(p.tags, '{}')
		  FROM blog_posts p
		 WHERE p.status = 'published' AND p.deleted_at IS NULL
		   AND COALESCE(p.cover_image_url, '') = ''
		   AND p.cover_attempts < $2
		   AND COALESCE(p.published_at, p.created_at) <= NOW() - $3::interval
		   AND NOT EXISTS (SELECT 1 FROM blog_post_covers c WHERE c.blog_post_id = p.id)
		 ORDER BY COALESCE(p.published_at, p.created_at) ASC
		 LIMIT $1`, limit, MaxCoverAttempts, grace.String())
	if err != nil {
		LogQueryError(ctx, "BlogCovers.ListPublishedWithoutCover", "blog_posts", err)
		return nil, fmt.Errorf("list posts without cover: %w", err)
	}
	defer rows.Close()
	out := []BlogCoverBrief{}
	for rows.Next() {
		var b BlogCoverBrief
		if err := rows.Scan(&b.ID, &b.Slug, &b.Title, &b.Excerpt, &b.Tags); err != nil {
			return nil, fmt.Errorf("scan post without cover: %w", err)
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// IncrementCoverAttempts counts one failed try at drawing a post's cover.
func (r *BlogCoverRepository) IncrementCoverAttempts(ctx context.Context, blogPostID string) error {
	if _, err := r.pool.Exec(ctx, `UPDATE blog_posts SET cover_attempts = cover_attempts + 1 WHERE id = $1`, blogPostID); err != nil {
		LogQueryError(ctx, "BlogCovers.IncrementCoverAttempts", "blog_posts", err)
		return fmt.Errorf("count cover attempt: %w", err)
	}
	return nil
}

// Delete removes a post's stored cover and clears a cover_image_url that named it.
func (r *BlogCoverRepository) Delete(ctx context.Context, blogPostID, servedURL string) error {
	tx, err := r.pool.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer func() {
		if rbErr := tx.Rollback(ctx); rbErr != nil && !errors.Is(rbErr, pgx.ErrTxClosed) {
			LogQueryError(ctx, "BlogCovers.Delete.Rollback", "blog_post_covers", rbErr)
		}
	}()
	if _, err := tx.Exec(ctx, `DELETE FROM blog_post_covers WHERE blog_post_id = $1`, blogPostID); err != nil {
		LogQueryError(ctx, "BlogCovers.Delete", "blog_post_covers", err)
		return fmt.Errorf("delete blog cover: %w", err)
	}
	if _, err := tx.Exec(ctx, `UPDATE blog_posts SET cover_image_url = '', updated_at = NOW() WHERE id = $1 AND cover_image_url = $2`, blogPostID, servedURL); err != nil {
		LogQueryError(ctx, "BlogCovers.Delete", "blog_posts", err)
		return fmt.Errorf("clear blog cover url: %w", err)
	}
	return tx.Commit(ctx)
}
