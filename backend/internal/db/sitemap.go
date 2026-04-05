package db

import (
	"context"
	"fmt"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// SitemapRepository provides sitemap URL data from the database.
type SitemapRepository struct {
	pool *Pool
}

// NewSitemapRepository creates a new SitemapRepository.
func NewSitemapRepository(pool *Pool) *SitemapRepository {
	return &SitemapRepository{pool: pool}
}

// GetSitemapURLs returns all indexable content URLs for sitemap generation.
// Excludes drafts and soft-deleted content.
func (r *SitemapRepository) GetSitemapURLs(ctx context.Context) (*models.SitemapURLs, error) {
	result := &models.SitemapURLs{
		Posts:     []models.SitemapPost{},
		Agents:    []models.SitemapAgent{},
		Users:     []models.SitemapUser{},
		BlogPosts: []models.SitemapBlogPost{},
		Rooms:     []models.SitemapRoom{},
	}

	// Get quality posts only: solved problems, ideas with votes/responses
	postRows, err := r.pool.Query(ctx, `
		SELECT id, type, updated_at
		FROM posts
		WHERE deleted_at IS NULL
		AND status NOT IN ('draft', 'pending_review', 'rejected')
		AND (
			(type = 'problem' AND (status = 'solved' OR upvotes - downvotes >= 1))
			OR (type = 'idea' AND (upvotes - downvotes >= 2))
			OR (type = 'question')
		)
		ORDER BY updated_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer postRows.Close()

	for postRows.Next() {
		var p models.SitemapPost
		var updatedAt time.Time
		if err := postRows.Scan(&p.ID, &p.Type, &updatedAt); err != nil {
			return nil, err
		}
		p.UpdatedAt = updatedAt
		result.Posts = append(result.Posts, p)
	}
	if err := postRows.Err(); err != nil {
		return nil, err
	}

	// Get agents with actual contributions (reputation > 0)
	agentRows, err := r.pool.Query(ctx, `
		SELECT id, COALESCE(updated_at, created_at) as updated_at
		FROM agents
		WHERE status = 'active' AND reputation > 0
		ORDER BY COALESCE(updated_at, created_at) DESC
	`)
	if err != nil {
		return nil, err
	}
	defer agentRows.Close()

	for agentRows.Next() {
		var a models.SitemapAgent
		var updatedAt time.Time
		if err := agentRows.Scan(&a.ID, &updatedAt); err != nil {
			return nil, err
		}
		a.UpdatedAt = updatedAt
		result.Agents = append(result.Agents, a)
	}
	if err := agentRows.Err(); err != nil {
		return nil, err
	}

	// Users excluded from sitemap — profile pages have no SEO value

	// Get all published, non-deleted blog posts
	blogRows, err := r.pool.Query(ctx, `
		SELECT slug, updated_at
		FROM blog_posts
		WHERE deleted_at IS NULL
		AND status = 'published'
		ORDER BY updated_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer blogRows.Close()

	for blogRows.Next() {
		var bp models.SitemapBlogPost
		var updatedAt time.Time
		if err := blogRows.Scan(&bp.Slug, &updatedAt); err != nil {
			return nil, err
		}
		bp.UpdatedAt = updatedAt
		result.BlogPosts = append(result.BlogPosts, bp)
	}
	if err := blogRows.Err(); err != nil {
		return nil, err
	}

	// Get all public, non-deleted rooms
	roomRows, err := r.pool.Query(ctx, `
		SELECT slug, last_active_at
		FROM rooms
		WHERE is_private = false
		AND deleted_at IS NULL
		ORDER BY last_active_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer roomRows.Close()

	for roomRows.Next() {
		var rm models.SitemapRoom
		var lastActiveAt time.Time
		if err := roomRows.Scan(&rm.Slug, &lastActiveAt); err != nil {
			return nil, err
		}
		rm.LastActiveAt = lastActiveAt
		result.Rooms = append(result.Rooms, rm)
	}
	if err := roomRows.Err(); err != nil {
		return nil, err
	}

	return result, nil
}

// GetSitemapCounts returns counts of indexable content per type.
// Uses the same WHERE filters as GetSitemapURLs.
func (r *SitemapRepository) GetSitemapCounts(ctx context.Context) (*models.SitemapCounts, error) {
	counts := &models.SitemapCounts{}

	// Count quality posts only
	err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM posts
		WHERE deleted_at IS NULL
		AND status NOT IN ('draft', 'pending_review', 'rejected')
		AND (
			(type = 'problem' AND (status = 'solved' OR upvotes - downvotes >= 1))
			OR (type = 'idea' AND (upvotes - downvotes >= 2))
			OR (type = 'question')
		)
	`).Scan(&counts.Posts)
	if err != nil {
		return nil, err
	}

	// Count agents with contributions
	err = r.pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM agents
		WHERE status = 'active' AND reputation > 0
	`).Scan(&counts.Agents)
	if err != nil {
		return nil, err
	}

	// Users excluded from sitemap
	counts.Users = 0
	if err != nil {
		return nil, err
	}

	// Count published, non-deleted blog posts
	err = r.pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM blog_posts
		WHERE deleted_at IS NULL
		AND status = 'published'
	`).Scan(&counts.BlogPosts)
	if err != nil {
		return nil, err
	}

	// Count public, non-deleted rooms
	err = r.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM rooms WHERE is_private = false AND deleted_at IS NULL
	`).Scan(&counts.Rooms)
	if err != nil {
		return nil, err
	}

	return counts, nil
}

// GetPaginatedSitemapURLs returns paginated sitemap URLs for a single content type.
func (r *SitemapRepository) GetPaginatedSitemapURLs(ctx context.Context, opts models.SitemapURLsOptions) (*models.SitemapURLs, error) {
	result := &models.SitemapURLs{
		Posts:     []models.SitemapPost{},
		Agents:    []models.SitemapAgent{},
		Users:     []models.SitemapUser{},
		BlogPosts: []models.SitemapBlogPost{},
		Rooms:     []models.SitemapRoom{},
	}

	offset := (opts.Page - 1) * opts.PerPage

	switch opts.Type {
	case "posts":
		rows, err := r.pool.Query(ctx, `
			SELECT id, type, updated_at
			FROM posts
			WHERE deleted_at IS NULL
			AND status NOT IN ('draft', 'pending_review', 'rejected')
			AND (
				(type = 'problem' AND (status = 'solved' OR upvotes - downvotes >= 1))
				OR (type = 'idea' AND (upvotes - downvotes >= 2))
				OR (type = 'question')
			)
			ORDER BY updated_at DESC
			LIMIT $1 OFFSET $2
		`, opts.PerPage, offset)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		for rows.Next() {
			var p models.SitemapPost
			var updatedAt time.Time
			if err := rows.Scan(&p.ID, &p.Type, &updatedAt); err != nil {
				return nil, err
			}
			p.UpdatedAt = updatedAt
			result.Posts = append(result.Posts, p)
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}

	case "agents":
		rows, err := r.pool.Query(ctx, `
			SELECT id, COALESCE(updated_at, created_at) as updated_at
			FROM agents
			WHERE status = 'active' AND reputation > 0
			ORDER BY COALESCE(updated_at, created_at) DESC
			LIMIT $1 OFFSET $2
		`, opts.PerPage, offset)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		for rows.Next() {
			var a models.SitemapAgent
			var updatedAt time.Time
			if err := rows.Scan(&a.ID, &updatedAt); err != nil {
				return nil, err
			}
			a.UpdatedAt = updatedAt
			result.Agents = append(result.Agents, a)
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}

	case "users":
		rows, err := r.pool.Query(ctx, `
			SELECT id::text, COALESCE(updated_at, created_at) as updated_at
			FROM users
			ORDER BY COALESCE(updated_at, created_at) DESC
			LIMIT $1 OFFSET $2
		`, opts.PerPage, offset)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		for rows.Next() {
			var u models.SitemapUser
			var updatedAt time.Time
			if err := rows.Scan(&u.ID, &updatedAt); err != nil {
				return nil, err
			}
			u.UpdatedAt = updatedAt
			result.Users = append(result.Users, u)
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}

	case "blog_posts":
		rows, err := r.pool.Query(ctx, `
			SELECT slug, updated_at
			FROM blog_posts
			WHERE deleted_at IS NULL
			AND status = 'published'
			ORDER BY updated_at DESC
			LIMIT $1 OFFSET $2
		`, opts.PerPage, offset)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		for rows.Next() {
			var bp models.SitemapBlogPost
			var updatedAt time.Time
			if err := rows.Scan(&bp.Slug, &updatedAt); err != nil {
				return nil, err
			}
			bp.UpdatedAt = updatedAt
			result.BlogPosts = append(result.BlogPosts, bp)
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}

	case "rooms":
		rows, err := r.pool.Query(ctx, `
			SELECT slug, last_active_at
			FROM rooms
			WHERE is_private = false
			AND deleted_at IS NULL
			ORDER BY last_active_at DESC
			LIMIT $1 OFFSET $2
		`, opts.PerPage, offset)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		for rows.Next() {
			var rm models.SitemapRoom
			var lastActiveAt time.Time
			if err := rows.Scan(&rm.Slug, &lastActiveAt); err != nil {
				return nil, err
			}
			rm.LastActiveAt = lastActiveAt
			result.Rooms = append(result.Rooms, rm)
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}

	default:
		return nil, fmt.Errorf("invalid sitemap type: %s", opts.Type)
	}

	return result, nil
}
