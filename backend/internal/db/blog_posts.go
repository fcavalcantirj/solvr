package db

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Blog post errors.
var (
	ErrBlogPostNotFound = errors.New("blog post not found")
	ErrDuplicateSlug    = errors.New("slug already exists")
)

// BlogPostRepository handles database operations for blog posts.
type BlogPostRepository struct {
	pool *Pool
}

// NewBlogPostRepository creates a new BlogPostRepository.
func NewBlogPostRepository(pool *Pool) *BlogPostRepository {
	return &BlogPostRepository{pool: pool}
}

// blogPostColumns defines columns returned by RETURNING for Create/Update.
const blogPostColumns = `id, slug, title, body, excerpt, tags, cover_image_url,
	posted_by_type, posted_by_id, status, view_count, upvotes, downvotes,
	read_time_minutes, meta_description, published_at, created_at, updated_at, deleted_at`

// scanBlogPost scans a single row into a BlogPost (19 columns).
func (r *BlogPostRepository) scanBlogPost(row pgx.Row) (*models.BlogPost, error) {
	post := &models.BlogPost{}
	err := row.Scan(
		&post.ID,
		&post.Slug,
		&post.Title,
		&post.Body,
		&post.Excerpt,
		&post.Tags,
		&post.CoverImageURL,
		&post.PostedByType,
		&post.PostedByID,
		&post.Status,
		&post.ViewCount,
		&post.Upvotes,
		&post.Downvotes,
		&post.ReadTimeMinutes,
		&post.MetaDescription,
		&post.PublishedAt,
		&post.CreatedAt,
		&post.UpdatedAt,
		&post.DeletedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrBlogPostNotFound
		}
		if isInvalidUUIDError(err) {
			return nil, ErrBlogPostNotFound
		}
		return nil, err
	}
	return post, nil
}

// scanBlogPostWithAuthorRows scans rows into BlogPostWithAuthor (19 + 2 author + 1 vote = 22 cols).
func (r *BlogPostRepository) scanBlogPostWithAuthorRows(rows pgx.Rows) (*models.BlogPostWithAuthor, error) {
	var post models.BlogPostWithAuthor
	var authorDisplayName, authorAvatarURL string

	err := rows.Scan(
		&post.ID,
		&post.Slug,
		&post.Title,
		&post.Body,
		&post.Excerpt,
		&post.Tags,
		&post.CoverImageURL,
		&post.PostedByType,
		&post.PostedByID,
		&post.Status,
		&post.ViewCount,
		&post.Upvotes,
		&post.Downvotes,
		&post.ReadTimeMinutes,
		&post.MetaDescription,
		&post.PublishedAt,
		&post.CreatedAt,
		&post.UpdatedAt,
		&post.DeletedAt,
		&authorDisplayName,
		&authorAvatarURL,
		&post.UserVote,
	)
	if err != nil {
		return nil, err
	}

	post.Author = models.BlogPostAuthor{
		Type:        post.PostedByType,
		ID:          post.PostedByID,
		DisplayName: authorDisplayName,
		AvatarURL:   authorAvatarURL,
	}
	post.VoteScore = post.Upvotes - post.Downvotes

	return &post, nil
}

// Create inserts a new blog post into the database.
func (r *BlogPostRepository) Create(ctx context.Context, post *models.BlogPost) (*models.BlogPost, error) {
	// Default status
	status := post.Status
	if status == "" {
		status = models.BlogPostStatusDraft
	}

	// Auto-calculate read time if zero
	readTime := post.ReadTimeMinutes
	if readTime == 0 {
		readTime = models.CalculateReadTime(post.Body)
	}

	// Auto-generate excerpt if empty
	excerpt := post.Excerpt
	if excerpt == "" {
		excerpt = models.GenerateExcerpt(post.Body, 500)
	}

	query := fmt.Sprintf(`
		INSERT INTO blog_posts (
			slug, title, body, excerpt, tags, cover_image_url,
			posted_by_type, posted_by_id, status,
			read_time_minutes, meta_description,
			published_at, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, NOW(), NOW())
		RETURNING %s
	`, blogPostColumns)

	// Set published_at if status is published
	var publishedAt *time.Time
	if status == models.BlogPostStatusPublished {
		now := time.Now()
		publishedAt = &now
	}

	row := r.pool.QueryRow(ctx, query,
		post.Slug,
		post.Title,
		post.Body,
		excerpt,
		post.Tags,
		post.CoverImageURL,
		post.PostedByType,
		post.PostedByID,
		status,
		readTime,
		post.MetaDescription,
		publishedAt,
	)

	result, err := r.scanBlogPost(row)
	if err != nil {
		// Check for duplicate slug
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, ErrDuplicateSlug
		}
		LogQueryError(ctx, "Create", "blog_posts", err)
		return nil, fmt.Errorf("create blog post failed: %w", err)
	}
	return result, nil
}

// FindBySlug returns a blog post by slug with author info.
func (r *BlogPostRepository) FindBySlug(ctx context.Context, slug string) (*models.BlogPostWithAuthor, error) {
	return r.findBySlugInternal(ctx, slug, "", "")
}

// FindBySlugForViewer returns a blog post by slug with the viewer's vote included.
func (r *BlogPostRepository) FindBySlugForViewer(ctx context.Context, slug string, viewerType models.AuthorType, viewerID string) (*models.BlogPostWithAuthor, error) {
	return r.findBySlugInternal(ctx, slug, viewerType, viewerID)
}

// findBySlugInternal is the shared implementation for FindBySlug and FindBySlugForViewer.
func (r *BlogPostRepository) findBySlugInternal(ctx context.Context, slug string, viewerType models.AuthorType, viewerID string) (*models.BlogPostWithAuthor, error) {
	var viewerVoteColumn, viewerVoteJoin string
	var args []any

	if viewerType != "" && viewerID != "" {
		viewerVoteColumn = "v.direction as user_vote_direction"
		viewerVoteJoin = "LEFT JOIN votes v ON v.target_type = 'blog_post' AND v.target_id = bp.id AND v.voter_type = $2 AND v.voter_id = $3"
		args = []any{slug, string(viewerType), viewerID}
	} else {
		viewerVoteColumn = "NULL::text as user_vote_direction"
		viewerVoteJoin = ""
		args = []any{slug}
	}

	query := fmt.Sprintf(`
		SELECT
			bp.id, bp.slug, bp.title, bp.body, bp.excerpt, bp.tags, bp.cover_image_url,
			bp.posted_by_type, bp.posted_by_id, bp.status,
			bp.view_count, bp.upvotes, bp.downvotes,
			bp.read_time_minutes, bp.meta_description,
			bp.published_at, bp.created_at, bp.updated_at, bp.deleted_at,
			COALESCE(u.display_name, ag.display_name, '') as author_display_name,
			COALESCE(u.avatar_url, ag.avatar_url, '') as author_avatar_url,
			%s
		FROM blog_posts bp
		LEFT JOIN users u ON bp.posted_by_type = 'human' AND bp.posted_by_id = u.id::text
		LEFT JOIN agents ag ON bp.posted_by_type = 'agent' AND bp.posted_by_id = ag.id
		%s
		WHERE bp.slug = $1 AND bp.deleted_at IS NULL
	`, viewerVoteColumn, viewerVoteJoin)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		LogQueryError(ctx, "FindBySlug", "blog_posts", err)
		return nil, fmt.Errorf("find by slug query failed: %w", err)
	}
	defer rows.Close()

	if !rows.Next() {
		slog.Debug("blog post not found", "op", "FindBySlug", "table", "blog_posts", "slug", slug)
		return nil, ErrBlogPostNotFound
	}

	post, err := r.scanBlogPostWithAuthorRows(rows)
	if err != nil {
		LogQueryError(ctx, "FindBySlug.Scan", "blog_posts", err)
		return nil, fmt.Errorf("scan failed: %w", err)
	}

	return post, nil
}

// Update updates an existing blog post.
func (r *BlogPostRepository) Update(ctx context.Context, post *models.BlogPost) (*models.BlogPost, error) {
	// Auto-calculate read time if zero
	readTime := post.ReadTimeMinutes
	if readTime == 0 {
		readTime = models.CalculateReadTime(post.Body)
	}

	// If transitioning to published and published_at is nil, set it
	var publishedAtExpr string
	args := []any{
		post.Slug,
		post.Title,
		post.Body,
		post.Excerpt,
		post.Tags,
		post.CoverImageURL,
		post.Status,
		post.MetaDescription,
		readTime,
	}

	if post.Status == models.BlogPostStatusPublished && post.PublishedAt == nil {
		publishedAtExpr = "COALESCE(published_at, NOW())"
	} else {
		publishedAtExpr = "published_at"
	}

	query := fmt.Sprintf(`
		UPDATE blog_posts
		SET
			title = $2,
			body = $3,
			excerpt = $4,
			tags = $5,
			cover_image_url = $6,
			status = $7,
			meta_description = $8,
			read_time_minutes = $9,
			published_at = %s,
			updated_at = NOW()
		WHERE slug = $1 AND deleted_at IS NULL
		RETURNING %s
	`, publishedAtExpr, blogPostColumns)

	row := r.pool.QueryRow(ctx, query, args...)

	result, err := r.scanBlogPost(row)
	if err != nil {
		if errors.Is(err, ErrBlogPostNotFound) {
			return nil, ErrBlogPostNotFound
		}
		LogQueryError(ctx, "Update", "blog_posts", err)
		return nil, fmt.Errorf("update blog post failed: %w", err)
	}
	return result, nil
}

// Delete performs a soft delete on a blog post by slug.
func (r *BlogPostRepository) Delete(ctx context.Context, slug string) error {
	query := `
		UPDATE blog_posts
		SET deleted_at = NOW()
		WHERE slug = $1 AND deleted_at IS NULL
	`

	result, err := r.pool.Exec(ctx, query, slug)
	if err != nil {
		LogQueryError(ctx, "Delete", "blog_posts", err)
		return fmt.Errorf("delete blog post failed: %w", err)
	}

	if result.RowsAffected() == 0 {
		return ErrBlogPostNotFound
	}

	return nil
}

// List returns a paginated list of blog posts with author information.
func (r *BlogPostRepository) List(ctx context.Context, opts models.BlogPostListOptions) ([]models.BlogPostWithAuthor, int, error) {
	var conditions []string
	var args []any
	argNum := 1

	// Always exclude deleted posts
	conditions = append(conditions, "bp.deleted_at IS NULL")

	// Filter by status (default to published)
	if opts.Status != "" {
		conditions = append(conditions, fmt.Sprintf("bp.status = $%d", argNum))
		args = append(args, opts.Status)
		argNum++
	} else {
		conditions = append(conditions, "bp.status = 'published'")
	}

	// Filter by tags (PostgreSQL array overlap)
	if len(opts.Tags) > 0 {
		conditions = append(conditions, fmt.Sprintf("bp.tags && $%d", argNum))
		args = append(args, opts.Tags)
		argNum++
	}

	// Filter by author
	if opts.AuthorType != "" && opts.AuthorID != "" {
		conditions = append(conditions, fmt.Sprintf("bp.posted_by_type = $%d AND bp.posted_by_id = $%d", argNum, argNum+1))
		args = append(args, opts.AuthorType, opts.AuthorID)
		argNum += 2
	}

	whereClause := strings.Join(conditions, " AND ")

	// Pagination
	page := opts.Page
	if page < 1 {
		page = 1
	}
	perPage := opts.PerPage
	if perPage < 1 {
		perPage = 20
	}
	if perPage > 100 {
		perPage = 100
	}
	offset := (page - 1) * perPage

	// Count query
	countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM blog_posts bp WHERE %s`, whereClause)
	var total int
	err := r.pool.QueryRow(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		LogQueryError(ctx, "List.Count", "blog_posts", err)
		return nil, 0, fmt.Errorf("count query failed: %w", err)
	}

	// Sort order
	orderClause := "bp.created_at DESC" // default: newest
	switch opts.Sort {
	case "popular":
		orderClause = "(bp.upvotes - bp.downvotes) DESC, bp.created_at DESC"
	case "published":
		orderClause = "bp.published_at DESC NULLS LAST, bp.created_at DESC"
	}

	// Viewer vote column and JOIN
	var viewerVoteColumn, viewerVoteJoin string
	if opts.ViewerType != "" && opts.ViewerID != "" {
		viewerVoteColumn = "v.direction as user_vote_direction"
		viewerVoteJoin = fmt.Sprintf(
			`LEFT JOIN votes v ON v.target_type = 'blog_post' AND v.target_id = bp.id AND v.voter_type = $%d AND v.voter_id = $%d`,
			argNum, argNum+1,
		)
		args = append(args, string(opts.ViewerType), opts.ViewerID)
		argNum += 2
	} else {
		viewerVoteColumn = "NULL::text as user_vote_direction"
		viewerVoteJoin = ""
	}

	query := fmt.Sprintf(`
		SELECT
			bp.id, bp.slug, bp.title, bp.body, bp.excerpt, bp.tags, bp.cover_image_url,
			bp.posted_by_type, bp.posted_by_id, bp.status,
			bp.view_count, bp.upvotes, bp.downvotes,
			bp.read_time_minutes, bp.meta_description,
			bp.published_at, bp.created_at, bp.updated_at, bp.deleted_at,
			COALESCE(u.display_name, ag.display_name, '') as author_display_name,
			COALESCE(u.avatar_url, ag.avatar_url, '') as author_avatar_url,
			%s
		FROM blog_posts bp
		LEFT JOIN users u ON bp.posted_by_type = 'human' AND bp.posted_by_id = u.id::text
		LEFT JOIN agents ag ON bp.posted_by_type = 'agent' AND bp.posted_by_id = ag.id
		%s
		WHERE %s
		ORDER BY %s
		LIMIT $%d OFFSET $%d
	`, viewerVoteColumn, viewerVoteJoin, whereClause, orderClause, argNum, argNum+1)

	args = append(args, perPage, offset)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		LogQueryError(ctx, "List", "blog_posts", err)
		return nil, 0, fmt.Errorf("list query failed: %w", err)
	}
	defer rows.Close()

	var posts []models.BlogPostWithAuthor
	for rows.Next() {
		post, err := r.scanBlogPostWithAuthorRows(rows)
		if err != nil {
			LogQueryError(ctx, "List.Scan", "blog_posts", err)
			return nil, 0, fmt.Errorf("scan failed: %w", err)
		}
		posts = append(posts, *post)
	}

	if err := rows.Err(); err != nil {
		LogQueryError(ctx, "List.Rows", "blog_posts", err)
		return nil, 0, fmt.Errorf("rows iteration failed: %w", err)
	}

	if posts == nil {
		posts = []models.BlogPostWithAuthor{}
	}

	return posts, total, nil
}

// Vote adds or updates a vote on a blog post.
func (r *BlogPostRepository) Vote(ctx context.Context, blogPostID, voterType, voterID, direction string) error {
	// Validate direction
	if direction != "up" && direction != "down" {
		return ErrInvalidVoteDirection
	}

	// Validate voter type
	if voterType != "human" && voterType != "agent" {
		return ErrInvalidVoterType
	}

	// Check if blog post exists
	var exists bool
	err := r.pool.QueryRow(ctx,
		"SELECT EXISTS(SELECT 1 FROM blog_posts WHERE id = $1 AND deleted_at IS NULL)",
		blogPostID,
	).Scan(&exists)
	if err != nil {
		if isInvalidUUIDError(err) {
			slog.Debug("invalid UUID format", "op", "Vote.CheckExists", "table", "blog_posts", "id", blogPostID)
			return ErrBlogPostNotFound
		}
		LogQueryError(ctx, "Vote.CheckExists", "blog_posts", err)
		return fmt.Errorf("failed to check blog post existence: %w", err)
	}
	if !exists {
		return ErrBlogPostNotFound
	}

	// Check for existing vote
	var existingDirection string
	err = r.pool.QueryRow(ctx,
		`SELECT direction FROM votes
		 WHERE target_type = 'blog_post' AND target_id = $1
		 AND voter_type = $2 AND voter_id = $3`,
		blogPostID, voterType, voterID,
	).Scan(&existingDirection)

	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		LogQueryError(ctx, "Vote.CheckExisting", "votes", err)
		return fmt.Errorf("failed to check existing vote: %w", err)
	}

	// Same vote exists, nothing to do
	if existingDirection == direction {
		return nil
	}

	// Use WithTx for atomicity
	return r.pool.WithTx(ctx, func(tx Tx) error {
		if existingDirection == "" {
			// Insert new vote
			_, err = tx.Exec(ctx,
				`INSERT INTO votes (target_type, target_id, voter_type, voter_id, direction, confirmed)
				 VALUES ('blog_post', $1, $2, $3, $4, true)`,
				blogPostID, voterType, voterID, direction,
			)
			if err != nil {
				LogQueryError(ctx, "Vote.InsertVote", "votes", err)
				return fmt.Errorf("failed to insert vote: %w", err)
			}

			// Update blog post vote counts
			if direction == "up" {
				_, err = tx.Exec(ctx,
					"UPDATE blog_posts SET upvotes = upvotes + 1 WHERE id = $1",
					blogPostID,
				)
			} else {
				_, err = tx.Exec(ctx,
					"UPDATE blog_posts SET downvotes = downvotes + 1 WHERE id = $1",
					blogPostID,
				)
			}
			if err != nil {
				LogQueryError(ctx, "Vote.UpdateCounts", "blog_posts", err)
				return fmt.Errorf("failed to update blog post vote counts: %w", err)
			}
		} else {
			// Update existing vote direction
			_, err = tx.Exec(ctx,
				`UPDATE votes SET direction = $4
				 WHERE target_type = 'blog_post' AND target_id = $1
				 AND voter_type = $2 AND voter_id = $3`,
				blogPostID, voterType, voterID, direction,
			)
			if err != nil {
				LogQueryError(ctx, "Vote.UpdateDirection", "votes", err)
				return fmt.Errorf("failed to update vote: %w", err)
			}

			// Adjust counts
			if direction == "up" {
				_, err = tx.Exec(ctx,
					"UPDATE blog_posts SET upvotes = upvotes + 1, downvotes = downvotes - 1 WHERE id = $1",
					blogPostID,
				)
			} else {
				_, err = tx.Exec(ctx,
					"UPDATE blog_posts SET upvotes = upvotes - 1, downvotes = downvotes + 1 WHERE id = $1",
					blogPostID,
				)
			}
			if err != nil {
				LogQueryError(ctx, "Vote.AdjustCounts", "blog_posts", err)
				return fmt.Errorf("failed to adjust blog post vote counts: %w", err)
			}
		}
		return nil
	})
}

// IncrementViewCount increments the view count for a blog post by slug.
func (r *BlogPostRepository) IncrementViewCount(ctx context.Context, slug string) error {
	query := `
		UPDATE blog_posts
		SET view_count = view_count + 1
		WHERE slug = $1 AND deleted_at IS NULL
	`

	result, err := r.pool.Exec(ctx, query, slug)
	if err != nil {
		LogQueryError(ctx, "IncrementViewCount", "blog_posts", err)
		return fmt.Errorf("increment view count failed: %w", err)
	}

	if result.RowsAffected() == 0 {
		return ErrBlogPostNotFound
	}

	return nil
}

// GetFeatured returns the most engaging published blog post.
// Uses engagement score: (view_count + upvotes*5 - downvotes*2) / POWER((hours_since_publish + 24), 0.8)
func (r *BlogPostRepository) GetFeatured(ctx context.Context) (*models.BlogPostWithAuthor, error) {
	query := `
		SELECT
			bp.id, bp.slug, bp.title, bp.body, bp.excerpt, bp.tags, bp.cover_image_url,
			bp.posted_by_type, bp.posted_by_id, bp.status,
			bp.view_count, bp.upvotes, bp.downvotes,
			bp.read_time_minutes, bp.meta_description,
			bp.published_at, bp.created_at, bp.updated_at, bp.deleted_at,
			COALESCE(u.display_name, ag.display_name, '') as author_display_name,
			COALESCE(u.avatar_url, ag.avatar_url, '') as author_avatar_url,
			NULL::text as user_vote_direction
		FROM blog_posts bp
		LEFT JOIN users u ON bp.posted_by_type = 'human' AND bp.posted_by_id = u.id::text
		LEFT JOIN agents ag ON bp.posted_by_type = 'agent' AND bp.posted_by_id = ag.id
		WHERE bp.status = 'published' AND bp.deleted_at IS NULL AND bp.published_at IS NOT NULL
		ORDER BY
			(bp.view_count + bp.upvotes * 5 - bp.downvotes * 2)::float
			/ POWER((EXTRACT(EPOCH FROM NOW() - bp.published_at) / 3600 + 24), 0.8)
			DESC
		LIMIT 1
	`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		LogQueryError(ctx, "GetFeatured", "blog_posts", err)
		return nil, fmt.Errorf("get featured query failed: %w", err)
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, nil // No published posts
	}

	post, err := r.scanBlogPostWithAuthorRows(rows)
	if err != nil {
		LogQueryError(ctx, "GetFeatured.Scan", "blog_posts", err)
		return nil, fmt.Errorf("scan failed: %w", err)
	}

	return post, nil
}

// ListTags returns all tags used in published blog posts with their counts.
func (r *BlogPostRepository) ListTags(ctx context.Context) ([]models.BlogTag, error) {
	query := `
		SELECT UNNEST(tags) as tag, COUNT(*) as count
		FROM blog_posts
		WHERE status = 'published' AND deleted_at IS NULL
		GROUP BY tag
		ORDER BY count DESC
	`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		LogQueryError(ctx, "ListTags", "blog_posts", err)
		return nil, fmt.Errorf("list tags query failed: %w", err)
	}
	defer rows.Close()

	var tags []models.BlogTag
	for rows.Next() {
		var tag models.BlogTag
		if err := rows.Scan(&tag.Name, &tag.Count); err != nil {
			LogQueryError(ctx, "ListTags.Scan", "blog_posts", err)
			return nil, fmt.Errorf("scan tag failed: %w", err)
		}
		tags = append(tags, tag)
	}

	if err := rows.Err(); err != nil {
		LogQueryError(ctx, "ListTags.Rows", "blog_posts", err)
		return nil, fmt.Errorf("rows iteration failed: %w", err)
	}

	if tags == nil {
		tags = []models.BlogTag{}
	}

	return tags, nil
}

// SlugExists checks if a slug is already in use (among non-deleted posts).
func (r *BlogPostRepository) SlugExists(ctx context.Context, slug string) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx,
		"SELECT EXISTS(SELECT 1 FROM blog_posts WHERE slug = $1 AND deleted_at IS NULL)",
		slug,
	).Scan(&exists)
	if err != nil {
		LogQueryError(ctx, "SlugExists", "blog_posts", err)
		return false, fmt.Errorf("slug exists check failed: %w", err)
	}
	return exists, nil
}
