package db

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// postIndexableRule is the one rule for a post a search engine may index, which is the rule
// for what the post sitemap lists: the canonical public-eligibility rule
// (models.Post.PublicEligible: published, moderation-approved, public, not deleted) for a
// post of any type, whatever its votes or solved state. The legacy hidden statuses stay
// excluded while writers that set status alone exist (cmd/moderate-existing rejects that
// way). BART-151: private posts are never in the public sitemap. "{p}" stands for the
// qualifier of the query that reads the rule (postIndexablePredicate).
const postIndexableRule = `{p}deleted_at IS NULL
		AND {p}visibility = 'public'
		AND {p}publication_state = 'published'
		AND {p}moderation_state = 'approved'
		AND {p}status NOT IN ('draft', 'pending_review', 'rejected')`

// postIndexablePredicate writes postIndexableRule for a query: against an unaliased posts
// table (alias ""), or against the alias the query gives posts ("p"). The sitemap and
// GET /v1/posts?indexable=true (SPEC.md 27.2, posts_list.go) both read it, so the post
// archive links exactly the posts the sitemap lists.
func postIndexablePredicate(alias string) string {
	qualifier := ""
	if alias != "" {
		qualifier = alias + "."
	}
	return strings.ReplaceAll(postIndexableRule, "{p}", qualifier)
}

// sitemapPostEligible is the rule as the sitemap's queries read it, on an unaliased posts.
var sitemapPostEligible = postIndexablePredicate("")

// sitemapAgentEligible is the one rule for an agent profile in the sitemap, the rule of the
// profile's verdict (agentProfileIndexable, SPEC.md 27.1): an active agent with public content,
// not deleted or banned (both set deleted_at). It used to read the stored reputation bonus,
// which is neither content nor the reputation people see.
var sitemapAgentEligible = agentProfileIndexable

// sitemapUserEligible is the same rule for a person's profile (userProfileIndexable).
var sitemapUserEligible = userProfileIndexable

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
		SELECT id, type, `+sitemapPostLastmod+` AS lastmod
		FROM posts
		WHERE `+sitemapPostEligible+`
		ORDER BY lastmod DESC
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

	// Agent profiles with public content (sitemapAgentEligible)
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

	// People's profiles with public content (sitemapUserEligible)
	userRows, err := r.pool.Query(ctx, `
		SELECT id::text, COALESCE(updated_at, created_at)
		FROM users
		WHERE `+sitemapUserEligible+`
		ORDER BY COALESCE(updated_at, created_at) DESC
	`)
	if err != nil {
		return nil, err
	}
	defer userRows.Close()
	for userRows.Next() {
		var u models.SitemapUser
		if err := userRows.Scan(&u.ID, &u.UpdatedAt); err != nil {
			return nil, err
		}
		result.Users = append(result.Users, u)
	}
	if err := userRows.Err(); err != nil {
		return nil, err
	}

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

	// Count agent and user profiles with public content
	err = r.pool.QueryRow(ctx, `
		SELECT (SELECT COUNT(*) FROM agents WHERE `+sitemapAgentEligible+`),
		       (SELECT COUNT(*) FROM users WHERE `+sitemapUserEligible+`)
	`).Scan(&counts.Agents, &counts.Users)
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

	// Each type's newest material change, for the sitemap index (task idx 83).
	if counts.Lastmod, err = r.sitemapLastmods(ctx); err != nil {
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
			SELECT id, type, `+sitemapPostLastmod+` AS lastmod
			FROM posts
			WHERE `+sitemapPostEligible+`
			ORDER BY lastmod DESC
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
			WHERE `+sitemapUserEligible+`
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
