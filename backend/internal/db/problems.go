// Package db provides database access for Solvr.
package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// Problem-related errors.
var (
	ErrProblemNotFound = errors.New("problem not found")
)

// ProblemsRepository handles database operations for problems.
// It wraps PostRepository (for posts with type='problem') and ApproachesRepository.
// Per SPEC.md Part 2.1: Problems are a post type, stored in the posts table.
type ProblemsRepository struct {
	pool         *Pool
	postRepo     *PostRepository
	approachRepo *ApproachesRepository
}

// NewProblemsRepository creates a new ProblemsRepository.
func NewProblemsRepository(pool *Pool) *ProblemsRepository {
	return &ProblemsRepository{
		pool:         pool,
		postRepo:     NewPostRepository(pool),
		approachRepo: NewApproachesRepository(pool),
	}
}

// FindProblemByID returns a single problem by ID.
// Returns ErrProblemNotFound if the post doesn't exist or is not a problem.
func (r *ProblemsRepository) FindProblemByID(ctx context.Context, id string) (*models.PostWithAuthor, error) {
	post, err := r.postRepo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, ErrPostNotFound) {
			return nil, ErrProblemNotFound
		}
		return nil, err
	}

	// Verify it's actually a problem
	if post.Type != models.PostTypeProblem {
		return nil, ErrProblemNotFound
	}

	return post, nil
}

// CreateProblem creates a new problem and returns it.
// Automatically sets the type to 'problem'.
func (r *ProblemsRepository) CreateProblem(ctx context.Context, post *models.Post) (*models.Post, error) {
	// Force type to 'problem'
	post.Type = models.PostTypeProblem

	return r.postRepo.Create(ctx, post)
}

// ListApproaches returns approaches for a problem.
// Delegates to ApproachesRepository.
func (r *ProblemsRepository) ListApproaches(ctx context.Context, problemID string, opts models.ApproachListOptions) ([]models.ApproachWithAuthor, int, error) {
	return r.approachRepo.ListApproaches(ctx, problemID, opts)
}

// FindApproachByID returns a single approach by ID.
// Delegates to ApproachesRepository.
func (r *ProblemsRepository) FindApproachByID(ctx context.Context, id string) (*models.ApproachWithAuthor, error) {
	return r.approachRepo.FindApproachByID(ctx, id)
}

// ApproachVisibleTo reports whether an approach's owning problem is visible to the caller.
func (r *ProblemsRepository) ApproachVisibleTo(ctx context.Context, approachID, callerHuman string) (bool, error) {
	return r.approachRepo.ApproachVisibleTo(ctx, approachID, callerHuman)
}

// CreateApproach creates a new approach and returns it.
// Delegates to ApproachesRepository.
func (r *ProblemsRepository) CreateApproach(ctx context.Context, approach *models.Approach) (*models.Approach, error) {
	return r.approachRepo.CreateApproach(ctx, approach)
}

// UpdateApproach updates an existing approach and returns it.
// Delegates to ApproachesRepository.
func (r *ProblemsRepository) UpdateApproach(ctx context.Context, approach *models.Approach) (*models.Approach, error) {
	return r.approachRepo.UpdateApproach(ctx, approach)
}

// AddProgressNote adds a progress note to an approach.
// Delegates to ApproachesRepository.
func (r *ProblemsRepository) AddProgressNote(ctx context.Context, note *models.ProgressNote) (*models.ProgressNote, error) {
	return r.approachRepo.AddProgressNote(ctx, note)
}

// GetProgressNotes returns progress notes for an approach.
// Delegates to ApproachesRepository.
func (r *ProblemsRepository) GetProgressNotes(ctx context.Context, approachID string) ([]models.ProgressNote, error) {
	return r.approachRepo.GetProgressNotes(ctx, approachID)
}

// UpdateProblemStatus updates the status of a problem.
func (r *ProblemsRepository) UpdateProblemStatus(ctx context.Context, problemID string, status models.PostStatus) error {
	result, err := r.pool.Exec(ctx, `
		UPDATE posts
		SET status = $2, updated_at = NOW()
		WHERE id = $1 AND type = 'problem' AND deleted_at IS NULL
	`, problemID, status)

	if err != nil {
		return fmt.Errorf("update problem status: %w", err)
	}

	if result.RowsAffected() == 0 {
		return ErrProblemNotFound
	}

	return nil
}

// Update updates a problem post.
func (r *ProblemsRepository) Update(ctx context.Context, post *models.Post) (*models.Post, error) {
	return r.postRepo.Update(ctx, post)
}

// ListCrystallizationCandidates returns post IDs of solved problems that are
// eligible for crystallization: type=problem, status=solved, not deleted,
// not already crystallized, and stable for at least stabilityPeriod.
// Results are ordered by oldest updated_at first (crystallize oldest stable problems first).
func (r *PostRepository) ListCrystallizationCandidates(ctx context.Context, stabilityPeriod time.Duration, limit int) ([]string, error) {
	if limit <= 0 {
		limit = 50
	}

	query := `
		SELECT id::text
		FROM posts
		WHERE type = 'problem'
		  AND status = 'solved'
		  AND deleted_at IS NULL
		  AND visibility = 'public' -- BART-151: never pin family-private posts to public IPFS
		  AND crystallization_cid IS NULL
		  AND updated_at < NOW() - $1::interval
		  AND EXISTS (
		    SELECT 1 FROM approaches
		    WHERE problem_id = posts.id AND status = 'succeeded' AND deleted_at IS NULL
		  )
		ORDER BY updated_at ASC
		LIMIT $2
	`

	// Convert Go time.Duration to PostgreSQL interval string
	intervalStr := fmt.Sprintf("%d seconds", int(stabilityPeriod.Seconds()))

	rows, err := r.pool.Query(ctx, query, intervalStr, limit)
	if err != nil {
		LogQueryError(ctx, "ListCrystallizationCandidates", "posts", err)
		return nil, fmt.Errorf("list crystallization candidates: %w", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan crystallization candidate: %w", err)
		}
		ids = append(ids, id)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate crystallization candidates: %w", err)
	}

	// Return empty slice instead of nil for consistent API
	if ids == nil {
		ids = []string{}
	}

	return ids, nil
}
