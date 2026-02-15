// Package db provides database access for Solvr.
package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// Approach-related errors.
var (
	ErrApproachNotFound = errors.New("approach not found")
)

// ApproachesRepository handles database operations for approaches.
// Per SPEC.md Part 2.3 and Part 6: approaches table.
type ApproachesRepository struct {
	pool *Pool
}

// NewApproachesRepository creates a new ApproachesRepository.
func NewApproachesRepository(pool *Pool) *ApproachesRepository {
	return &ApproachesRepository{pool: pool}
}

// CreateApproach creates a new approach and returns it.
func (r *ApproachesRepository) CreateApproach(ctx context.Context, approach *models.Approach) (*models.Approach, error) {
	var id string
	var createdAt, updatedAt pgtype.Timestamptz

	err := r.pool.QueryRow(ctx, `
		INSERT INTO approaches (
			problem_id, author_type, author_id, angle, method,
			assumptions, differs_from, status, outcome, solution
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id, created_at, updated_at
	`,
		approach.ProblemID,
		approach.AuthorType,
		approach.AuthorID,
		approach.Angle,
		approach.Method,
		approach.Assumptions,
		approach.DiffersFrom,
		approach.Status,
		approach.Outcome,
		approach.Solution,
	).Scan(&id, &createdAt, &updatedAt)

	if err != nil {
		return nil, fmt.Errorf("create approach: %w", err)
	}

	approach.ID = id
	approach.CreatedAt = createdAt.Time
	approach.UpdatedAt = updatedAt.Time

	return approach, nil
}

// FindApproachByID returns a single approach by ID with author information.
func (r *ApproachesRepository) FindApproachByID(ctx context.Context, id string) (*models.ApproachWithAuthor, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT
			a.id, a.problem_id, a.author_type, a.author_id,
			a.angle, a.method, a.assumptions, a.differs_from,
			a.status, a.outcome, a.solution,
			a.created_at, a.updated_at, a.deleted_at,
			COALESCE(
				CASE WHEN a.author_type = 'agent' THEN ag.display_name
				     WHEN a.author_type = 'human' THEN u.display_name
				     ELSE a.author_id
				END,
				a.author_id
			) as display_name,
			COALESCE(
				CASE WHEN a.author_type = 'human' THEN u.avatar_url
				     ELSE ''
				END,
				''
			) as avatar_url
		FROM approaches a
		LEFT JOIN agents ag ON a.author_type = 'agent' AND a.author_id = ag.id
		LEFT JOIN users u ON a.author_type = 'human' AND a.author_id = u.id::text
		WHERE a.id = $1 AND a.deleted_at IS NULL
	`, id)

	var approach models.ApproachWithAuthor
	var displayName, avatarURL string
	var assumptions, differsFrom []string
	var createdAt, updatedAt pgtype.Timestamptz
	var deletedAt pgtype.Timestamptz

	err := row.Scan(
		&approach.ID,
		&approach.ProblemID,
		&approach.AuthorType,
		&approach.AuthorID,
		&approach.Angle,
		&approach.Method,
		&assumptions,
		&differsFrom,
		&approach.Status,
		&approach.Outcome,
		&approach.Solution,
		&createdAt,
		&updatedAt,
		&deletedAt,
		&displayName,
		&avatarURL,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrApproachNotFound
		}
		if isInvalidUUIDError(err) {
			return nil, ErrApproachNotFound
		}
		return nil, fmt.Errorf("find approach: %w", err)
	}

	approach.Assumptions = assumptions
	approach.DiffersFrom = differsFrom
	approach.CreatedAt = createdAt.Time
	approach.UpdatedAt = updatedAt.Time
	if deletedAt.Valid {
		approach.DeletedAt = &deletedAt.Time
	}

	approach.Author = models.ApproachAuthor{
		Type:        approach.AuthorType,
		ID:          approach.AuthorID,
		DisplayName: displayName,
		AvatarURL:   avatarURL,
	}

	return &approach, nil
}

// ListApproaches returns approaches for a problem with pagination.
func (r *ApproachesRepository) ListApproaches(ctx context.Context, problemID string, opts models.ApproachListOptions) ([]models.ApproachWithAuthor, int, error) {
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
		SELECT COUNT(*) FROM approaches
		WHERE problem_id = $1 AND deleted_at IS NULL
	`, problemID).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count approaches: %w", err)
	}

	// Get approaches with author info
	rows, err := r.pool.Query(ctx, `
		SELECT
			a.id, a.problem_id, a.author_type, a.author_id,
			a.angle, a.method, a.assumptions, a.differs_from,
			a.status, a.outcome, a.solution,
			a.created_at, a.updated_at,
			COALESCE(
				CASE WHEN a.author_type = 'agent' THEN ag.display_name
				     WHEN a.author_type = 'human' THEN u.display_name
				     ELSE a.author_id
				END,
				a.author_id
			) as display_name,
			COALESCE(
				CASE WHEN a.author_type = 'human' THEN u.avatar_url
				     ELSE ''
				END,
				''
			) as avatar_url
		FROM approaches a
		LEFT JOIN agents ag ON a.author_type = 'agent' AND a.author_id = ag.id
		LEFT JOIN users u ON a.author_type = 'human' AND a.author_id = u.id::text
		WHERE a.problem_id = $1 AND a.deleted_at IS NULL
		ORDER BY a.created_at DESC
		LIMIT $2 OFFSET $3
	`, problemID, perPage, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("query approaches: %w", err)
	}
	defer rows.Close()

	approaches := make([]models.ApproachWithAuthor, 0)
	for rows.Next() {
		var approach models.ApproachWithAuthor
		var displayName, avatarURL string
		var assumptions, differsFrom []string
		var createdAt, updatedAt pgtype.Timestamptz

		err := rows.Scan(
			&approach.ID,
			&approach.ProblemID,
			&approach.AuthorType,
			&approach.AuthorID,
			&approach.Angle,
			&approach.Method,
			&assumptions,
			&differsFrom,
			&approach.Status,
			&approach.Outcome,
			&approach.Solution,
			&createdAt,
			&updatedAt,
			&displayName,
			&avatarURL,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("scan approach: %w", err)
		}

		approach.Assumptions = assumptions
		approach.DiffersFrom = differsFrom
		approach.CreatedAt = createdAt.Time
		approach.UpdatedAt = updatedAt.Time

		approach.Author = models.ApproachAuthor{
			Type:        approach.AuthorType,
			ID:          approach.AuthorID,
			DisplayName: displayName,
			AvatarURL:   avatarURL,
		}

		approaches = append(approaches, approach)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate approaches: %w", err)
	}

	return approaches, total, nil
}

// UpdateApproach updates an existing approach and returns it.
func (r *ApproachesRepository) UpdateApproach(ctx context.Context, approach *models.Approach) (*models.Approach, error) {
	var updatedAt pgtype.Timestamptz

	err := r.pool.QueryRow(ctx, `
		UPDATE approaches
		SET status = COALESCE($2, status),
		    outcome = COALESCE($3, outcome),
		    solution = COALESCE($4, solution),
		    method = COALESCE($5, method),
		    updated_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL
		RETURNING status, outcome, solution, method, updated_at
	`,
		approach.ID,
		approach.Status,
		nullIfEmpty(approach.Outcome),
		nullIfEmpty(approach.Solution),
		nullIfEmpty(approach.Method),
	).Scan(
		&approach.Status,
		&approach.Outcome,
		&approach.Solution,
		&approach.Method,
		&updatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrApproachNotFound
		}
		return nil, fmt.Errorf("update approach: %w", err)
	}

	approach.UpdatedAt = updatedAt.Time
	return approach, nil
}

// DeleteApproach soft deletes an approach.
func (r *ApproachesRepository) DeleteApproach(ctx context.Context, id string) error {
	result, err := r.pool.Exec(ctx, `
		UPDATE approaches
		SET deleted_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL
	`, id)

	if err != nil {
		return fmt.Errorf("delete approach: %w", err)
	}

	if result.RowsAffected() == 0 {
		return ErrApproachNotFound
	}

	return nil
}

// ListByAuthor returns approaches by a specific author with problem title context.
// Results are ordered by created_at DESC with pagination.
func (r *ApproachesRepository) ListByAuthor(ctx context.Context, authorType, authorID string, page, perPage int) ([]models.ApproachWithContext, int, error) {
	if page < 1 {
		page = 1
	}
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
		SELECT COUNT(*) FROM approaches
		WHERE author_type = $1 AND author_id = $2 AND deleted_at IS NULL
	`, authorType, authorID).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count approaches by author: %w", err)
	}

	rows, err := r.pool.Query(ctx, `
		SELECT
			a.id, a.problem_id, a.author_type, a.author_id,
			a.angle, a.method, a.assumptions, a.differs_from,
			a.status, a.outcome, a.solution,
			a.created_at, a.updated_at,
			COALESCE(
				CASE WHEN a.author_type = 'agent' THEN ag.display_name
				     WHEN a.author_type = 'human' THEN u.display_name
				     ELSE a.author_id
				END, a.author_id
			) as display_name,
			COALESCE(
				CASE WHEN a.author_type = 'human' THEN u.avatar_url ELSE '' END, ''
			) as avatar_url,
			COALESCE(p.title, '') as problem_title
		FROM approaches a
		LEFT JOIN agents ag ON a.author_type = 'agent' AND a.author_id = ag.id
		LEFT JOIN users u ON a.author_type = 'human' AND a.author_id = u.id::text
		LEFT JOIN posts p ON a.problem_id = p.id
		WHERE a.author_type = $1 AND a.author_id = $2 AND a.deleted_at IS NULL
		ORDER BY a.created_at DESC
		LIMIT $3 OFFSET $4
	`, authorType, authorID, perPage, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("query approaches by author: %w", err)
	}
	defer rows.Close()

	results := make([]models.ApproachWithContext, 0)
	for rows.Next() {
		var item models.ApproachWithContext
		var displayName, avatarURL string
		var assumptions, differsFrom []string
		var createdAt, updatedAt pgtype.Timestamptz

		err := rows.Scan(
			&item.ID, &item.ProblemID, &item.AuthorType, &item.AuthorID,
			&item.Angle, &item.Method, &assumptions, &differsFrom,
			&item.Status, &item.Outcome, &item.Solution,
			&createdAt, &updatedAt,
			&displayName, &avatarURL, &item.ProblemTitle,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("scan approach by author: %w", err)
		}

		item.Assumptions = assumptions
		item.DiffersFrom = differsFrom
		item.CreatedAt = createdAt.Time
		item.UpdatedAt = updatedAt.Time

		item.Author = models.ApproachAuthor{
			Type:        item.AuthorType,
			ID:          item.AuthorID,
			DisplayName: displayName,
			AvatarURL:   avatarURL,
		}

		results = append(results, item)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate approaches by author: %w", err)
	}

	return results, total, nil
}

// AddProgressNote adds a progress note to an approach.
func (r *ApproachesRepository) AddProgressNote(ctx context.Context, note *models.ProgressNote) (*models.ProgressNote, error) {
	var id string
	var createdAt pgtype.Timestamptz

	err := r.pool.QueryRow(ctx, `
		INSERT INTO progress_notes (approach_id, content)
		VALUES ($1, $2)
		RETURNING id, created_at
	`, note.ApproachID, note.Content).Scan(&id, &createdAt)

	if err != nil {
		return nil, fmt.Errorf("add progress note: %w", err)
	}

	note.ID = id
	note.CreatedAt = createdAt.Time

	return note, nil
}

// GetProgressNotes returns progress notes for an approach, ordered by created_at descending.
func (r *ApproachesRepository) GetProgressNotes(ctx context.Context, approachID string) ([]models.ProgressNote, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, approach_id, content, created_at
		FROM progress_notes
		WHERE approach_id = $1
		ORDER BY created_at DESC
	`, approachID)
	if err != nil {
		return nil, fmt.Errorf("get progress notes: %w", err)
	}
	defer rows.Close()

	notes := make([]models.ProgressNote, 0)
	for rows.Next() {
		var note models.ProgressNote
		var createdAt pgtype.Timestamptz

		err := rows.Scan(&note.ID, &note.ApproachID, &note.Content, &createdAt)
		if err != nil {
			return nil, fmt.Errorf("scan progress note: %w", err)
		}

		note.CreatedAt = createdAt.Time
		notes = append(notes, note)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate progress notes: %w", err)
	}

	return notes, nil
}

// nullIfEmpty returns nil if s is empty, otherwise returns s.
// Used for COALESCE updates.
func nullIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
