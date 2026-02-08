// Package db provides database access for Solvr.
package db

import (
	"context"
	"errors"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// Question-related errors.
var (
	ErrQuestionNotFound = errors.New("question not found")
)

// QuestionsRepository handles database operations for questions.
// It wraps PostRepository (for posts with type='question') and AnswersRepository.
// Per SPEC.md Part 2.4: Questions are a post type, stored in the posts table.
type QuestionsRepository struct {
	pool        *Pool
	postRepo    *PostRepository
	answersRepo *AnswersRepository
}

// NewQuestionsRepository creates a new QuestionsRepository.
func NewQuestionsRepository(pool *Pool) *QuestionsRepository {
	return &QuestionsRepository{
		pool:        pool,
		postRepo:    NewPostRepository(pool),
		answersRepo: NewAnswersRepository(pool),
	}
}

// ListQuestions returns questions matching the given options.
// Automatically filters to type='question' posts only.
func (r *QuestionsRepository) ListQuestions(ctx context.Context, opts models.PostListOptions) ([]models.PostWithAuthor, int, error) {
	// Force type filter to 'question'
	opts.Type = models.PostTypeQuestion

	return r.postRepo.List(ctx, opts)
}

// FindQuestionByID returns a single question by ID.
// Returns ErrQuestionNotFound if the post doesn't exist or is not a question.
func (r *QuestionsRepository) FindQuestionByID(ctx context.Context, id string) (*models.PostWithAuthor, error) {
	post, err := r.postRepo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, ErrPostNotFound) {
			return nil, ErrQuestionNotFound
		}
		return nil, err
	}

	// Verify it's actually a question
	if post.Type != models.PostTypeQuestion {
		return nil, ErrQuestionNotFound
	}

	return post, nil
}

// CreateQuestion creates a new question and returns it.
// Automatically sets the type to 'question'.
func (r *QuestionsRepository) CreateQuestion(ctx context.Context, post *models.Post) (*models.Post, error) {
	// Force type to 'question'
	post.Type = models.PostTypeQuestion

	return r.postRepo.Create(ctx, post)
}

// ListAnswers returns answers for a question.
// Delegates to AnswersRepository which joins with agents/users tables for author names.
func (r *QuestionsRepository) ListAnswers(ctx context.Context, questionID string, opts models.AnswerListOptions) ([]models.AnswerWithAuthor, int, error) {
	return r.answersRepo.ListAnswers(ctx, questionID, opts)
}

// CreateAnswer creates a new answer and returns it.
// Delegates to AnswersRepository.
func (r *QuestionsRepository) CreateAnswer(ctx context.Context, answer *models.Answer) (*models.Answer, error) {
	return r.answersRepo.CreateAnswer(ctx, answer)
}

// FindAnswerByID returns a single answer by ID with author information.
// Delegates to AnswersRepository.
func (r *QuestionsRepository) FindAnswerByID(ctx context.Context, id string) (*models.AnswerWithAuthor, error) {
	return r.answersRepo.FindAnswerByID(ctx, id)
}

// UpdateAnswer updates an existing answer.
// Delegates to AnswersRepository.
func (r *QuestionsRepository) UpdateAnswer(ctx context.Context, answer *models.Answer) (*models.Answer, error) {
	return r.answersRepo.UpdateAnswer(ctx, answer)
}

// DeleteAnswer soft-deletes an answer.
// Delegates to AnswersRepository.
func (r *QuestionsRepository) DeleteAnswer(ctx context.Context, id string) error {
	return r.answersRepo.DeleteAnswer(ctx, id)
}

// AcceptAnswer marks an answer as accepted and updates the question status.
// Delegates to AnswersRepository.
func (r *QuestionsRepository) AcceptAnswer(ctx context.Context, questionID, answerID string) error {
	return r.answersRepo.AcceptAnswer(ctx, questionID, answerID)
}

// VoteOnAnswer records a vote on an answer.
// Delegates to AnswersRepository.
func (r *QuestionsRepository) VoteOnAnswer(ctx context.Context, answerID, voterType, voterID, direction string) error {
	return r.answersRepo.VoteOnAnswer(ctx, answerID, voterType, voterID, direction)
}

// GetAnswerCount returns the count of answers for a question.
// Delegates to AnswersRepository.
func (r *QuestionsRepository) GetAnswerCount(ctx context.Context, questionID string) (int, error) {
	return r.answersRepo.GetAnswerCount(ctx, questionID)
}
