// Package db provides database access for Solvr.
package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/jackc/pgx/v5"
)

// Ideas-related errors.
var (
	ErrIdeaNotFound = errors.New("idea not found")
)

// IdeasRepository handles database operations for ideas.
// It wraps PostRepository (for posts with type='idea') and ResponsesRepository.
// Per SPEC.md Part 2.2: Ideas are a post type, stored in the posts table.
type IdeasRepository struct {
	pool          *Pool
	postRepo      *PostRepository
	responsesRepo *ResponsesRepository
}

// NewIdeasRepository creates a new IdeasRepository.
func NewIdeasRepository(pool *Pool) *IdeasRepository {
	return &IdeasRepository{
		pool:          pool,
		postRepo:      NewPostRepository(pool),
		responsesRepo: NewResponsesRepository(pool),
	}
}

// ListIdeas returns ideas matching the given options.
// Automatically filters to type='idea' posts only.
func (r *IdeasRepository) ListIdeas(ctx context.Context, opts models.PostListOptions) ([]models.PostWithAuthor, int, error) {
	// Force type filter to 'idea'
	opts.Type = models.PostTypeIdea

	return r.postRepo.List(ctx, opts)
}

// FindIdeaByID returns a single idea by ID.
// Returns ErrIdeaNotFound if the post doesn't exist or is not an idea.
func (r *IdeasRepository) FindIdeaByID(ctx context.Context, id string) (*models.PostWithAuthor, error) {
	post, err := r.postRepo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, ErrPostNotFound) {
			return nil, ErrIdeaNotFound
		}
		return nil, err
	}

	// Verify it's actually an idea
	if post.Type != models.PostTypeIdea {
		return nil, ErrIdeaNotFound
	}

	return post, nil
}

// CreateIdea creates a new idea and returns it.
// Automatically sets the type to 'idea'.
func (r *IdeasRepository) CreateIdea(ctx context.Context, post *models.Post) (*models.Post, error) {
	// Force type to 'idea'
	post.Type = models.PostTypeIdea

	return r.postRepo.Create(ctx, post)
}

// ListResponses returns responses for an idea.
// Delegates to ResponsesRepository.
func (r *IdeasRepository) ListResponses(ctx context.Context, ideaID string, opts models.ResponseListOptions) ([]models.ResponseWithAuthor, int, error) {
	return r.responsesRepo.ListResponses(ctx, ideaID, opts)
}

// CreateResponse creates a new response and returns it.
// Delegates to ResponsesRepository.
func (r *IdeasRepository) CreateResponse(ctx context.Context, response *models.Response) (*models.Response, error) {
	return r.responsesRepo.CreateResponse(ctx, response)
}

// AddEvolvedInto adds a post ID to the idea's evolved_into array.
// This tracks when an idea evolves into other posts (problems, questions, etc.).
func (r *IdeasRepository) AddEvolvedInto(ctx context.Context, ideaID, evolvedPostID string) error {
	// Use PostgreSQL array_append to add to the evolved_into array
	result, err := r.pool.Exec(ctx, `
		UPDATE posts
		SET evolved_into = array_append(evolved_into, $2),
		    updated_at = NOW()
		WHERE id = $1 AND type = 'idea' AND deleted_at IS NULL
	`, ideaID, evolvedPostID)

	if err != nil {
		return fmt.Errorf("update evolved_into: %w", err)
	}

	if result.RowsAffected() == 0 {
		return ErrIdeaNotFound
	}

	return nil
}

// FindPostByID returns a single post by ID (for verifying evolved post exists).
// Delegates to PostRepository.
func (r *IdeasRepository) FindPostByID(ctx context.Context, id string) (*models.PostWithAuthor, error) {
	post, err := r.postRepo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, ErrPostNotFound) {
			return nil, ErrPostNotFound
		}
		return nil, err
	}
	return post, nil
}

// GetResponseCount returns the count of responses for an idea.
// Delegates to ResponsesRepository.
func (r *IdeasRepository) GetResponseCount(ctx context.Context, ideaID string) (int, error) {
	return r.responsesRepo.GetResponseCount(ctx, ideaID)
}

// scanIdeaWithAuthor scans a row with author information into a PostWithAuthor.
func (r *IdeasRepository) scanIdeaWithAuthor(row pgx.Row) (*models.PostWithAuthor, error) {
	var post models.PostWithAuthor
	var authorDisplayName, authorAvatarURL string

	err := row.Scan(
		&post.ID,
		&post.Type,
		&post.Title,
		&post.Description,
		&post.Tags,
		&post.PostedByType,
		&post.PostedByID,
		&post.Status,
		&post.Upvotes,
		&post.Downvotes,
		&post.ViewCount,
		&post.SuccessCriteria,
		&post.Weight,
		&post.AcceptedAnswerID,
		&post.EvolvedInto,
		&post.CreatedAt,
		&post.UpdatedAt,
		&post.DeletedAt,
		&authorDisplayName,
		&authorAvatarURL,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrIdeaNotFound
		}
		if isInvalidUUIDError(err) {
			return nil, ErrIdeaNotFound
		}
		return nil, err
	}

	// Verify it's an idea
	if post.Type != models.PostTypeIdea {
		return nil, ErrIdeaNotFound
	}

	// Populate author information
	post.Author = models.PostAuthor{
		Type:        post.PostedByType,
		ID:          post.PostedByID,
		DisplayName: authorDisplayName,
		AvatarURL:   authorAvatarURL,
	}

	// Compute vote score
	post.VoteScore = post.Upvotes - post.Downvotes

	return &post, nil
}
