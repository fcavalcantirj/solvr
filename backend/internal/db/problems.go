// Package db provides database access for Solvr.
package db

import (
	"context"
	"errors"
	"fmt"

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
	pool          *Pool
	postRepo      *PostRepository
	approachRepo  *ApproachesRepository
}

// NewProblemsRepository creates a new ProblemsRepository.
func NewProblemsRepository(pool *Pool) *ProblemsRepository {
	return &ProblemsRepository{
		pool:         pool,
		postRepo:     NewPostRepository(pool),
		approachRepo: NewApproachesRepository(pool),
	}
}

// ListProblems returns problems matching the given options.
// Automatically filters to type='problem' posts only.
func (r *ProblemsRepository) ListProblems(ctx context.Context, opts models.PostListOptions) ([]models.PostWithAuthor, int, error) {
	// Force type filter to 'problem'
	opts.Type = models.PostTypeProblem

	return r.postRepo.List(ctx, opts)
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
