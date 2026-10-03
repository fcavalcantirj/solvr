package db

import (
	"context"
	"fmt"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// sitemapPostEligible is the sitemap's post rule: the canonical public-eligibility rule
// (models.Post.PublicEligible: published, moderation-approved, public, not deleted) for a
// post of any type, whatever its votes or solved state. The legacy hidden statuses stay
// excluded while writers that set status alone exist (cmd/moderate-existing rejects that
// way). BART-151: private posts are never in the public sitemap.
const sitemapPostEligible = `deleted_at IS NULL
		AND visibility = 'public'
		AND publication_state = 'published'
		AND moderation_state = 'approved'
		AND status NOT IN ('draft', 'pending_review', 'rejected')`

// sitemapAgentEligible is the one rule for an agent profile in the sitemap: an active agent
// with contributions that has not been deleted or banned (both set deleted_at).
const sitemapAgentEligible = `status = 'active' AND reputation > 0 AND deleted_at IS NULL`

// SitemapRepository provides sitemap URL data from the database.
type SitemapRepository struct {
	pool *Pool
}

// NewSitemapRepository creates a new SitemapRepository.
func NewSitemapRepository(pool *Pool) *SitemapRepository {
	return &SitemapRepository{pool: pool}
}

// GetSitemapURLs returns all indexable content URLs for sitemap generation.
// Posts follow sitemapPostEligible; drafts and soft-deleted content are excluded.
func (r *SitemapRepository) GetSitemapURLs(ctx context.Context) (*models.SitemapURLs, error) {
	result := &models.SitemapURLs{
		Posts:     []models.SitemapPost{},
		Agents:    []models.SitemapAgent{},
		Users:     []models.SitemapUser{},
		BlogPosts: []models.SitemapBlogPost{},
		Rooms:     []models.SitemapRoom{},
	}

	postRows, err := r.pool.Query(ctx, `
		SELECT id, type, updated_at
		FROM posts
		WHERE `+sitemapPostEligible+`
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
		WHERE `+sitemapAgentEligible+`
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

	// Rooms whose page may be indexed (roomIndexablePredicate): public, live, two-way.
	roomRows, err := r.pool.Query(ctx, `
		SELECT slug, last_active_at
		FROM rooms
		WHERE `+roomIndexablePredicate+`
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

	err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM posts WHERE `+sitemapPostEligible+`
	`).Scan(&counts.Posts)
	if err != nil {
		return nil, err
	}

	// Count agents with contributions
	err = r.pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM agents
		WHERE `+sitemapAgentEligible+`
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

	// Count indexable rooms (roomIndexablePredicate)
	err = r.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM rooms WHERE `+roomIndexablePredicate+`
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
			WHERE `+sitemapPostEligible+`
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
			WHERE `+sitemapAgentEligible+`
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
			WHERE deleted_at IS NULL
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
			WHERE `+roomIndexablePredicate+`
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
