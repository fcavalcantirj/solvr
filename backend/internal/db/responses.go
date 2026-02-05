// Package db provides database access for Solvr.
package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Response-related errors.
var (
	ErrResponseNotFound = errors.New("response not found")
	ErrIdeaNotExists    = errors.New("idea does not exist")
)

// ResponsesRepository handles database operations for responses.
// Per SPEC.md Part 2.5: Responses (for Ideas) and Part 6: Database Schema.
type ResponsesRepository struct {
	pool *Pool
}

// NewResponsesRepository creates a new ResponsesRepository.
func NewResponsesRepository(pool *Pool) *ResponsesRepository {
	return &ResponsesRepository{pool: pool}
}

// ListResponses returns responses for an idea with pagination.
// Returns responses ordered by created_at descending (newest first).
func (r *ResponsesRepository) ListResponses(ctx context.Context, ideaID string, opts models.ResponseListOptions) ([]models.ResponseWithAuthor, int, error) {
	// Calculate pagination
	page := opts.Page
	if page < 1 {
		page = 1
	}
	perPage := opts.PerPage
	if perPage < 1 {
		perPage = 20
	}
	if perPage > 50 {
		perPage = 50
	}
	offset := (page - 1) * perPage

	// Get total count
	var total int
	err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM responses WHERE idea_id = $1
	`, ideaID).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count responses: %w", err)
	}

	// Get responses with author info
	// For agents, we join with agents table to get display name
	// For humans, we join with users table
	rows, err := r.pool.Query(ctx, `
		SELECT
			r.id,
			r.idea_id,
			r.author_type,
			r.author_id,
			r.content,
			r.response_type,
			r.upvotes,
			r.downvotes,
			r.created_at,
			COALESCE(
				CASE WHEN r.author_type = 'agent' THEN a.name
				     WHEN r.author_type = 'human' THEN u.display_name
				     ELSE r.author_id
				END,
				r.author_id
			) as display_name,
			COALESCE(
				CASE WHEN r.author_type = 'human' THEN u.avatar_url
				     ELSE ''
				END,
				''
			) as avatar_url
		FROM responses r
		LEFT JOIN agents a ON r.author_type = 'agent' AND r.author_id = a.id
		LEFT JOIN users u ON r.author_type = 'human' AND r.author_id = u.id::text
		WHERE r.idea_id = $1
		ORDER BY r.created_at DESC
		LIMIT $2 OFFSET $3
	`, ideaID, perPage, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("query responses: %w", err)
	}
	defer rows.Close()

	responses := make([]models.ResponseWithAuthor, 0)
	for rows.Next() {
		var resp models.ResponseWithAuthor
		var displayName, avatarURL string

		err := rows.Scan(
			&resp.ID,
			&resp.IdeaID,
			&resp.AuthorType,
			&resp.AuthorID,
			&resp.Content,
			&resp.ResponseType,
			&resp.Upvotes,
			&resp.Downvotes,
			&resp.CreatedAt,
			&displayName,
			&avatarURL,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("scan response: %w", err)
		}

		resp.Author = models.ResponseAuthor{
			Type:        resp.AuthorType,
			ID:          resp.AuthorID,
			DisplayName: displayName,
			AvatarURL:   avatarURL,
		}
		resp.VoteScore = resp.Upvotes - resp.Downvotes

		responses = append(responses, resp)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate responses: %w", err)
	}

	return responses, total, nil
}

// CreateResponse creates a new response and returns it.
// The ID is auto-generated if not provided.
func (r *ResponsesRepository) CreateResponse(ctx context.Context, response *models.Response) (*models.Response, error) {
	// Generate ID if not provided
	id := response.ID
	if id == "" {
		id = uuid.New().String()
	}

	// Insert response
	err := r.pool.QueryRow(ctx, `
		INSERT INTO responses (id, idea_id, author_type, author_id, content, response_type)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, idea_id, author_type, author_id, content, response_type, upvotes, downvotes, created_at
	`,
		id,
		response.IdeaID,
		response.AuthorType,
		response.AuthorID,
		response.Content,
		response.ResponseType,
	).Scan(
		&response.ID,
		&response.IdeaID,
		&response.AuthorType,
		&response.AuthorID,
		&response.Content,
		&response.ResponseType,
		&response.Upvotes,
		&response.Downvotes,
		&response.CreatedAt,
	)

	if err != nil {
		// Check for foreign key violation (idea doesn't exist)
		if isInvalidUUIDError(err) || errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrIdeaNotExists
		}
		return nil, fmt.Errorf("insert response: %w", err)
	}

	return response, nil
}

// GetResponseCount returns the count of responses for an idea.
func (r *ResponsesRepository) GetResponseCount(ctx context.Context, ideaID string) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM responses WHERE idea_id = $1
	`, ideaID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count responses: %w", err)
	}
	return count, nil
}

// FindByID returns a response by ID with author information.
func (r *ResponsesRepository) FindByID(ctx context.Context, id string) (*models.ResponseWithAuthor, error) {
	var resp models.ResponseWithAuthor
	var displayName, avatarURL string

	err := r.pool.QueryRow(ctx, `
		SELECT
			r.id,
			r.idea_id,
			r.author_type,
			r.author_id,
			r.content,
			r.response_type,
			r.upvotes,
			r.downvotes,
			r.created_at,
			COALESCE(
				CASE WHEN r.author_type = 'agent' THEN a.name
				     WHEN r.author_type = 'human' THEN u.display_name
				     ELSE r.author_id
				END,
				r.author_id
			) as display_name,
			COALESCE(
				CASE WHEN r.author_type = 'human' THEN u.avatar_url
				     ELSE ''
				END,
				''
			) as avatar_url
		FROM responses r
		LEFT JOIN agents a ON r.author_type = 'agent' AND r.author_id = a.id
		LEFT JOIN users u ON r.author_type = 'human' AND r.author_id = u.id::text
		WHERE r.id = $1
	`, id).Scan(
		&resp.ID,
		&resp.IdeaID,
		&resp.AuthorType,
		&resp.AuthorID,
		&resp.Content,
		&resp.ResponseType,
		&resp.Upvotes,
		&resp.Downvotes,
		&resp.CreatedAt,
		&displayName,
		&avatarURL,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) || isInvalidUUIDError(err) {
			return nil, ErrResponseNotFound
		}
		return nil, fmt.Errorf("query response: %w", err)
	}

	resp.Author = models.ResponseAuthor{
		Type:        resp.AuthorType,
		ID:          resp.AuthorID,
		DisplayName: displayName,
		AvatarURL:   avatarURL,
	}
	resp.VoteScore = resp.Upvotes - resp.Downvotes

	return &resp, nil
}
