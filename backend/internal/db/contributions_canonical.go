package db

import (
	"context"
	"fmt"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// Canonical contribution listings (task idx 76 step 3): GET /v1/users/{id}/contributions and
// GET /v1/me/contributions list an author's answers, approaches and responses. They are read
// from the live replies migrated from those rows (legacy_type; an approach's angle and status
// are kept in provenance), so the lists survive dropping the legacy tables. A native reply has
// no contribution type and is not listed. Each repository keeps the ListByAuthor contract of
// the legacy repository it replaces: page < 1 reads page 1, per_page < 1 reads 20 and is
// capped at 50, newest first, and the parent post's title only when the post is public. The
// items carry the fields the listing reads: id (the reply's), parent post, author, preview
// text, approach status, parent title and created_at.

// migratedContribution is one live reply migrated from a contribution of one legacy type.
type migratedContribution struct {
	id, postID, authorType, authorID string
	body, angle, status, title       string
	createdAt                        time.Time
}

func listMigratedContributions(ctx context.Context, pool *Pool, legacyType models.ReplyLegacyType,
	authorType, authorID string, page, perPage int) ([]migratedContribution, int, error) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 20
	}
	if perPage > 50 {
		perPage = 50
	}

	var total int
	err := pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM replies r
		WHERE r.legacy_type = $1 AND r.author_type = $2 AND r.author_id = $3 AND r.deleted_at IS NULL
	`, string(legacyType), authorType, authorID).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count %s contributions by author: %w", legacyType, err)
	}

	rows, err := pool.Query(ctx, `
		SELECT r.id::text, r.post_id::text, r.author_type, r.author_id, r.body,
			COALESCE(r.provenance->>'angle', ''), COALESCE(r.provenance->>'status', ''),
			CASE WHEN p.visibility = 'public' THEN COALESCE(p.title, '') ELSE '' END,
			r.created_at
		FROM replies r
		LEFT JOIN posts p ON p.id = r.post_id
		WHERE r.legacy_type = $1 AND r.author_type = $2 AND r.author_id = $3 AND r.deleted_at IS NULL
		ORDER BY r.created_at DESC, r.id DESC
		LIMIT $4 OFFSET $5
	`, string(legacyType), authorType, authorID, perPage, (page-1)*perPage)
	if err != nil {
		return nil, 0, fmt.Errorf("list %s contributions by author: %w", legacyType, err)
	}
	defer rows.Close()

	var out []migratedContribution
	for rows.Next() {
		var c migratedContribution
		if err := rows.Scan(&c.id, &c.postID, &c.authorType, &c.authorID, &c.body,
			&c.angle, &c.status, &c.title, &c.createdAt); err != nil {
			return nil, 0, fmt.Errorf("scan %s contribution: %w", legacyType, err)
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate %s contributions: %w", legacyType, err)
	}
	return out, total, nil
}

// CanonicalAnswerContributionsRepository lists an author's answers from the replies migrated from them.
type CanonicalAnswerContributionsRepository struct {
	pool *Pool
}

// NewCanonicalAnswerContributionsRepository creates a CanonicalAnswerContributionsRepository.
func NewCanonicalAnswerContributionsRepository(pool *Pool) *CanonicalAnswerContributionsRepository {
	return &CanonicalAnswerContributionsRepository{pool: pool}
}

// ListByAuthor lists the author's live migrated answers, newest first, with the total.
func (r *CanonicalAnswerContributionsRepository) ListByAuthor(ctx context.Context, authorType, authorID string, page, perPage int) ([]models.AnswerWithContext, int, error) {
	items, total, err := listMigratedContributions(ctx, r.pool, models.ReplyLegacyAnswer, authorType, authorID, page, perPage)
	if err != nil {
		return nil, 0, err
	}
	out := make([]models.AnswerWithContext, 0, len(items))
	for _, c := range items {
		var a models.AnswerWithContext
		a.ID, a.QuestionID, a.Content, a.CreatedAt = c.id, c.postID, c.body, c.createdAt
		a.AuthorType, a.AuthorID = models.AuthorType(c.authorType), c.authorID
		a.Author = models.AnswerAuthor{Type: a.AuthorType, ID: c.authorID}
		a.QuestionTitle = c.title
		out = append(out, a)
	}
	return out, total, nil
}

// CanonicalApproachContributionsRepository lists an author's approaches from the replies migrated from them.
type CanonicalApproachContributionsRepository struct {
	pool *Pool
}

// NewCanonicalApproachContributionsRepository creates a CanonicalApproachContributionsRepository.
func NewCanonicalApproachContributionsRepository(pool *Pool) *CanonicalApproachContributionsRepository {
	return &CanonicalApproachContributionsRepository{pool: pool}
}

// ListByAuthor lists the author's live migrated approaches (angle and status from provenance),
// newest first, with the total.
func (r *CanonicalApproachContributionsRepository) ListByAuthor(ctx context.Context, authorType, authorID string, page, perPage int) ([]models.ApproachWithContext, int, error) {
	items, total, err := listMigratedContributions(ctx, r.pool, models.ReplyLegacyApproach, authorType, authorID, page, perPage)
	if err != nil {
		return nil, 0, err
	}
	out := make([]models.ApproachWithContext, 0, len(items))
	for _, c := range items {
		var a models.ApproachWithContext
		a.ID, a.ProblemID, a.Angle, a.Status, a.CreatedAt = c.id, c.postID, c.angle, models.ApproachStatus(c.status), c.createdAt
		a.AuthorType, a.AuthorID = models.AuthorType(c.authorType), c.authorID
		a.Author = models.ApproachAuthor{Type: a.AuthorType, ID: c.authorID}
		a.ProblemTitle = c.title
		out = append(out, a)
	}
	return out, total, nil
}

// CanonicalResponseContributionsRepository lists an author's responses from the replies migrated from them.
type CanonicalResponseContributionsRepository struct {
	pool *Pool
}

// NewCanonicalResponseContributionsRepository creates a CanonicalResponseContributionsRepository.
func NewCanonicalResponseContributionsRepository(pool *Pool) *CanonicalResponseContributionsRepository {
	return &CanonicalResponseContributionsRepository{pool: pool}
}

// ListByAuthor lists the author's live migrated responses, newest first, with the total.
func (r *CanonicalResponseContributionsRepository) ListByAuthor(ctx context.Context, authorType, authorID string, page, perPage int) ([]models.ResponseWithContext, int, error) {
	items, total, err := listMigratedContributions(ctx, r.pool, models.ReplyLegacyResponse, authorType, authorID, page, perPage)
	if err != nil {
		return nil, 0, err
	}
	out := make([]models.ResponseWithContext, 0, len(items))
	for _, c := range items {
		var resp models.ResponseWithContext
		resp.ID, resp.IdeaID, resp.Content, resp.CreatedAt = c.id, c.postID, c.body, c.createdAt
		resp.AuthorType, resp.AuthorID = models.AuthorType(c.authorType), c.authorID
		resp.Author = models.ResponseAuthor{Type: resp.AuthorType, ID: c.authorID}
		resp.IdeaTitle = c.title
		out = append(out, resp)
	}
	return out, total, nil
}
