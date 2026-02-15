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

// Answer-related errors.
var (
	ErrAnswerNotFound   = errors.New("answer not found")
	ErrQuestionNotExist = errors.New("question does not exist")
)

// AnswersRepository handles database operations for answers.
// Per SPEC.md Part 2.4: Answers (for Questions) and Part 6: Database Schema.
type AnswersRepository struct {
	pool *Pool
}

// NewAnswersRepository creates a new AnswersRepository.
func NewAnswersRepository(pool *Pool) *AnswersRepository {
	return &AnswersRepository{pool: pool}
}

// ListAnswers returns answers for a question with pagination.
// Returns answers ordered by created_at descending (newest first).
// Includes author display_name from agents/users tables.
func (r *AnswersRepository) ListAnswers(ctx context.Context, questionID string, opts models.AnswerListOptions) ([]models.AnswerWithAuthor, int, error) {
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
		SELECT COUNT(*) FROM answers WHERE question_id = $1 AND deleted_at IS NULL
	`, questionID).Scan(&total)
	if err != nil {
		// If table doesn't exist, return empty array (graceful degradation)
		if isTableNotFoundError(err) {
			return []models.AnswerWithAuthor{}, 0, nil
		}
		return nil, 0, fmt.Errorf("count answers: %w", err)
	}

	// Get answers with author info
	// For agents, we join with agents table to get display name
	// For humans, we join with users table
	rows, err := r.pool.Query(ctx, `
		SELECT
			ans.id,
			ans.question_id,
			ans.author_type,
			ans.author_id,
			ans.content,
			ans.is_accepted,
			ans.upvotes,
			ans.downvotes,
			ans.created_at,
			COALESCE(
				CASE WHEN ans.author_type = 'agent' THEN a.display_name
				     WHEN ans.author_type = 'human' THEN u.display_name
				     ELSE ans.author_id
				END,
				ans.author_id
			) as display_name,
			COALESCE(
				CASE WHEN ans.author_type = 'human' THEN u.avatar_url
				     ELSE ''
				END,
				''
			) as avatar_url
		FROM answers ans
		LEFT JOIN agents a ON ans.author_type = 'agent' AND ans.author_id = a.id
		LEFT JOIN users u ON ans.author_type = 'human' AND ans.author_id = u.id::text
		WHERE ans.question_id = $1 AND ans.deleted_at IS NULL
		ORDER BY ans.created_at DESC
		LIMIT $2 OFFSET $3
	`, questionID, perPage, offset)
	if err != nil {
		// If table doesn't exist, return empty array (graceful degradation)
		if isTableNotFoundError(err) {
			return []models.AnswerWithAuthor{}, 0, nil
		}
		return nil, 0, fmt.Errorf("query answers: %w", err)
	}
	defer rows.Close()

	answers := make([]models.AnswerWithAuthor, 0)
	for rows.Next() {
		var ans models.AnswerWithAuthor
		var displayName, avatarURL string

		err := rows.Scan(
			&ans.ID,
			&ans.QuestionID,
			&ans.AuthorType,
			&ans.AuthorID,
			&ans.Content,
			&ans.IsAccepted,
			&ans.Upvotes,
			&ans.Downvotes,
			&ans.CreatedAt,
			&displayName,
			&avatarURL,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("scan answer: %w", err)
		}

		ans.Author = models.AnswerAuthor{
			Type:        ans.AuthorType,
			ID:          ans.AuthorID,
			DisplayName: displayName,
			AvatarURL:   avatarURL,
		}
		ans.VoteScore = ans.Upvotes - ans.Downvotes

		answers = append(answers, ans)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate answers: %w", err)
	}

	return answers, total, nil
}

// CreateAnswer creates a new answer and returns it.
// The ID is auto-generated if not provided.
func (r *AnswersRepository) CreateAnswer(ctx context.Context, answer *models.Answer) (*models.Answer, error) {
	// Generate ID if not provided
	id := answer.ID
	if id == "" {
		id = uuid.New().String()
	}

	// Insert answer
	err := r.pool.QueryRow(ctx, `
		INSERT INTO answers (id, question_id, author_type, author_id, content)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, question_id, author_type, author_id, content, is_accepted, upvotes, downvotes, created_at
	`,
		id,
		answer.QuestionID,
		answer.AuthorType,
		answer.AuthorID,
		answer.Content,
	).Scan(
		&answer.ID,
		&answer.QuestionID,
		&answer.AuthorType,
		&answer.AuthorID,
		&answer.Content,
		&answer.IsAccepted,
		&answer.Upvotes,
		&answer.Downvotes,
		&answer.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert answer: %w", err)
	}

	// Update question status from 'open' to 'answered' when first answer is created.
	// The WHERE status = 'open' guard ensures we don't overwrite 'solved' or other statuses.
	_, err = r.pool.Exec(ctx, `
		UPDATE posts SET status = 'answered', updated_at = NOW()
		WHERE id = $1 AND type = 'question' AND status = 'open'
	`, answer.QuestionID)
	if err != nil {
		return nil, fmt.Errorf("update question status to answered: %w", err)
	}

	return answer, nil
}

// FindAnswerByID returns a single answer by ID with author information.
func (r *AnswersRepository) FindAnswerByID(ctx context.Context, id string) (*models.AnswerWithAuthor, error) {
	var ans models.AnswerWithAuthor
	var displayName, avatarURL string

	err := r.pool.QueryRow(ctx, `
		SELECT
			ans.id,
			ans.question_id,
			ans.author_type,
			ans.author_id,
			ans.content,
			ans.is_accepted,
			ans.upvotes,
			ans.downvotes,
			ans.created_at,
			COALESCE(
				CASE WHEN ans.author_type = 'agent' THEN a.display_name
				     WHEN ans.author_type = 'human' THEN u.display_name
				     ELSE ans.author_id
				END,
				ans.author_id
			) as display_name,
			COALESCE(
				CASE WHEN ans.author_type = 'human' THEN u.avatar_url
				     ELSE ''
				END,
				''
			) as avatar_url
		FROM answers ans
		LEFT JOIN agents a ON ans.author_type = 'agent' AND ans.author_id = a.id
		LEFT JOIN users u ON ans.author_type = 'human' AND ans.author_id = u.id::text
		WHERE ans.id = $1 AND ans.deleted_at IS NULL
	`, id).Scan(
		&ans.ID,
		&ans.QuestionID,
		&ans.AuthorType,
		&ans.AuthorID,
		&ans.Content,
		&ans.IsAccepted,
		&ans.Upvotes,
		&ans.Downvotes,
		&ans.CreatedAt,
		&displayName,
		&avatarURL,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrAnswerNotFound
		}
		return nil, fmt.Errorf("query answer: %w", err)
	}

	ans.Author = models.AnswerAuthor{
		Type:        ans.AuthorType,
		ID:          ans.AuthorID,
		DisplayName: displayName,
		AvatarURL:   avatarURL,
	}
	ans.VoteScore = ans.Upvotes - ans.Downvotes

	return &ans, nil
}

// UpdateAnswer updates an existing answer.
func (r *AnswersRepository) UpdateAnswer(ctx context.Context, answer *models.Answer) (*models.Answer, error) {
	err := r.pool.QueryRow(ctx, `
		UPDATE answers
		SET content = $2
		WHERE id = $1 AND deleted_at IS NULL
		RETURNING id, question_id, author_type, author_id, content, is_accepted, upvotes, downvotes, created_at
	`,
		answer.ID,
		answer.Content,
	).Scan(
		&answer.ID,
		&answer.QuestionID,
		&answer.AuthorType,
		&answer.AuthorID,
		&answer.Content,
		&answer.IsAccepted,
		&answer.Upvotes,
		&answer.Downvotes,
		&answer.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrAnswerNotFound
		}
		return nil, fmt.Errorf("update answer: %w", err)
	}

	return answer, nil
}

// DeleteAnswer soft-deletes an answer by ID.
func (r *AnswersRepository) DeleteAnswer(ctx context.Context, id string) error {
	result, err := r.pool.Exec(ctx, `
		UPDATE answers SET deleted_at = NOW() WHERE id = $1 AND deleted_at IS NULL
	`, id)
	if err != nil {
		return fmt.Errorf("delete answer: %w", err)
	}

	if result.RowsAffected() == 0 {
		return ErrAnswerNotFound
	}

	return nil
}

// AcceptAnswer marks an answer as accepted and updates the question status to solved.
func (r *AnswersRepository) AcceptAnswer(ctx context.Context, questionID, answerID string) error {
	return r.pool.WithTx(ctx, func(tx Tx) error {
		// Unaccept any previously accepted answer
		_, err := tx.Exec(ctx, `
			UPDATE answers SET is_accepted = FALSE
			WHERE question_id = $1 AND is_accepted = TRUE
		`, questionID)
		if err != nil {
			return fmt.Errorf("unaccept previous answer: %w", err)
		}

		// Accept the new answer
		result, err := tx.Exec(ctx, `
			UPDATE answers SET is_accepted = TRUE
			WHERE id = $1 AND question_id = $2 AND deleted_at IS NULL
		`, answerID, questionID)
		if err != nil {
			return fmt.Errorf("accept answer: %w", err)
		}

		if result.RowsAffected() == 0 {
			return ErrAnswerNotFound
		}

		// Update question status to solved and set accepted_answer_id
		_, err = tx.Exec(ctx, `
			UPDATE posts SET status = 'solved', accepted_answer_id = $2
			WHERE id = $1 AND type = 'question'
		`, questionID, answerID)
		if err != nil {
			return fmt.Errorf("update question status: %w", err)
		}

		return nil
	})
}

// VoteOnAnswer records a vote on an answer.
func (r *AnswersRepository) VoteOnAnswer(ctx context.Context, answerID, voterType, voterID, direction string) error {
	// For now, just update the vote counts directly
	// A more sophisticated implementation would track individual votes
	var query string
	if direction == "up" {
		query = `UPDATE answers SET upvotes = upvotes + 1 WHERE id = $1 AND deleted_at IS NULL`
	} else if direction == "down" {
		query = `UPDATE answers SET downvotes = downvotes + 1 WHERE id = $1 AND deleted_at IS NULL`
	} else {
		return fmt.Errorf("invalid vote direction: %s", direction)
	}

	result, err := r.pool.Exec(ctx, query, answerID)
	if err != nil {
		return fmt.Errorf("vote on answer: %w", err)
	}

	if result.RowsAffected() == 0 {
		return ErrAnswerNotFound
	}

	return nil
}

// ListByAuthor returns answers by a specific author with question title context.
// Results are ordered by created_at DESC with pagination.
func (r *AnswersRepository) ListByAuthor(ctx context.Context, authorType, authorID string, page, perPage int) ([]models.AnswerWithContext, int, error) {
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
		SELECT COUNT(*) FROM answers
		WHERE author_type = $1 AND author_id = $2 AND deleted_at IS NULL
	`, authorType, authorID).Scan(&total)
	if err != nil {
		if isTableNotFoundError(err) {
			return []models.AnswerWithContext{}, 0, nil
		}
		return nil, 0, fmt.Errorf("count answers by author: %w", err)
	}

	rows, err := r.pool.Query(ctx, `
		SELECT
			ans.id, ans.question_id, ans.author_type, ans.author_id,
			ans.content, ans.is_accepted, ans.upvotes, ans.downvotes, ans.created_at,
			COALESCE(
				CASE WHEN ans.author_type = 'agent' THEN a.display_name
				     WHEN ans.author_type = 'human' THEN u.display_name
				     ELSE ans.author_id
				END, ans.author_id
			) as display_name,
			COALESCE(
				CASE WHEN ans.author_type = 'human' THEN u.avatar_url ELSE '' END, ''
			) as avatar_url,
			COALESCE(p.title, '') as question_title
		FROM answers ans
		LEFT JOIN agents a ON ans.author_type = 'agent' AND ans.author_id = a.id
		LEFT JOIN users u ON ans.author_type = 'human' AND ans.author_id = u.id::text
		LEFT JOIN posts p ON ans.question_id = p.id
		WHERE ans.author_type = $1 AND ans.author_id = $2 AND ans.deleted_at IS NULL
		ORDER BY ans.created_at DESC
		LIMIT $3 OFFSET $4
	`, authorType, authorID, perPage, offset)
	if err != nil {
		if isTableNotFoundError(err) {
			return []models.AnswerWithContext{}, 0, nil
		}
		return nil, 0, fmt.Errorf("query answers by author: %w", err)
	}
	defer rows.Close()

	results := make([]models.AnswerWithContext, 0)
	for rows.Next() {
		var item models.AnswerWithContext
		var displayName, avatarURL string

		err := rows.Scan(
			&item.ID, &item.QuestionID, &item.AuthorType, &item.AuthorID,
			&item.Content, &item.IsAccepted, &item.Upvotes, &item.Downvotes, &item.CreatedAt,
			&displayName, &avatarURL, &item.QuestionTitle,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("scan answer by author: %w", err)
		}

		item.Author = models.AnswerAuthor{
			Type:        item.AuthorType,
			ID:          item.AuthorID,
			DisplayName: displayName,
			AvatarURL:   avatarURL,
		}
		item.VoteScore = item.Upvotes - item.Downvotes

		results = append(results, item)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate answers by author: %w", err)
	}

	return results, total, nil
}

// GetAnswerCount returns the number of answers for a question.
func (r *AnswersRepository) GetAnswerCount(ctx context.Context, questionID string) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM answers WHERE question_id = $1 AND deleted_at IS NULL
	`, questionID).Scan(&count)
	if err != nil {
		// If table doesn't exist, return 0 (graceful degradation)
		if isTableNotFoundError(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("count answers: %w", err)
	}
	return count, nil
}
