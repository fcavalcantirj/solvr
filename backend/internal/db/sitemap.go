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
		Posts:  []models.SitemapPost{},
		Agents: []models.SitemapAgent{},
		Users:  []models.SitemapUser{},
	}

	// Get all non-draft, non-deleted posts
	postRows, err := r.pool.Query(ctx, `
		SELECT id, type, updated_at
		FROM posts
		WHERE deleted_at IS NULL
		AND status NOT IN ('draft', 'pending_review', 'rejected')
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

	// Get all active agents
	agentRows, err := r.pool.Query(ctx, `
		SELECT id, COALESCE(updated_at, created_at) as updated_at
		FROM agents
		WHERE status = 'active'
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

	// Get all users
	userRows, err := r.pool.Query(ctx, `
		SELECT id::text, COALESCE(updated_at, created_at) as updated_at
		FROM users
		ORDER BY COALESCE(updated_at, created_at) DESC
	`)
	if err != nil {
		return nil, err
	}
	defer userRows.Close()

	for userRows.Next() {
		var u models.SitemapUser
		var updatedAt time.Time
		if err := userRows.Scan(&u.ID, &updatedAt); err != nil {
			return nil, err
		}
		u.UpdatedAt = updatedAt
		result.Users = append(result.Users, u)
	}
	if err := userRows.Err(); err != nil {
		return nil, err
	}

	return result, nil
}

// GetSitemapCounts returns counts of indexable content per type.
// Uses the same WHERE filters as GetSitemapURLs.
func (r *SitemapRepository) GetSitemapCounts(ctx context.Context) (*models.SitemapCounts, error) {
	counts := &models.SitemapCounts{}

	// Count non-draft, non-deleted posts
	err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM posts
		WHERE deleted_at IS NULL
		AND status NOT IN ('draft', 'pending_review', 'rejected')
	`).Scan(&counts.Posts)
	if err != nil {
		return nil, err
	}

	// Count active agents
	err = r.pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM agents
		WHERE status = 'active'
	`).Scan(&counts.Agents)
	if err != nil {
		return nil, err
	}

	// Count all users
	err = r.pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM users
	`).Scan(&counts.Users)
	if err != nil {
		return nil, err
	}

	return counts, nil
}

// GetPaginatedSitemapURLs returns paginated sitemap URLs for a single content type.
func (r *SitemapRepository) GetPaginatedSitemapURLs(ctx context.Context, opts models.SitemapURLsOptions) (*models.SitemapURLs, error) {
	result := &models.SitemapURLs{
		Posts:  []models.SitemapPost{},
		Agents: []models.SitemapAgent{},
		Users:  []models.SitemapUser{},
	}

	offset := (opts.Page - 1) * opts.PerPage

	switch opts.Type {
	case "posts":
		rows, err := r.pool.Query(ctx, `
			SELECT id, type, updated_at
			FROM posts
			WHERE deleted_at IS NULL
			AND status NOT IN ('draft', 'pending_review', 'rejected')
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
			WHERE status = 'active'
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

	default:
		return nil, fmt.Errorf("invalid sitemap type: %s", opts.Type)
	}

	return result, nil
}
