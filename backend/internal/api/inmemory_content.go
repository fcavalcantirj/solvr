package api

/**
 * In-memory repositories for content endpoints: problems, questions, ideas, comments.
 * Used for testing when no database is available.
 */

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/fcavalcantirj/solvr/internal/api/handlers"
	"github.com/fcavalcantirj/solvr/internal/models"
)

// Error types for content operations - use handlers package errors for consistency
var (
	errProblemNotFound  = handlers.ErrProblemNotFound
	errQuestionNotFound = handlers.ErrQuestionNotFound
	errIdeaNotFound     = handlers.ErrIdeaNotFound
	errCommentNotFound  = errors.New("comment not found") // Not in handlers, keep local
	errApproachNotFound = handlers.ErrApproachNotFound
	errAnswerNotFound   = handlers.ErrAnswerNotFound
	errResponseNotFound = errors.New("response not found") // Not in handlers, keep local
)

// InMemoryProblemsRepository is an in-memory implementation of ProblemsRepositoryInterface.
type InMemoryProblemsRepository struct {
	mu         sync.RWMutex
	posts      map[string]*models.Post
	approaches map[string]*models.Approach
	notes      map[string][]*models.ProgressNote
}

// NewInMemoryProblemsRepository creates a new in-memory problems repository.
func NewInMemoryProblemsRepository() *InMemoryProblemsRepository {
	return &InMemoryProblemsRepository{
		posts:      make(map[string]*models.Post),
		approaches: make(map[string]*models.Approach),
		notes:      make(map[string][]*models.ProgressNote),
	}
}

// ListProblems returns problems matching the given options.
func (r *InMemoryProblemsRepository) ListProblems(ctx context.Context, opts models.PostListOptions) ([]models.PostWithAuthor, int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var results []models.PostWithAuthor
	for _, post := range r.posts {
		if post.DeletedAt != nil || post.Type != models.PostTypeProblem {
			continue
		}
		if opts.Status != "" && post.Status != opts.Status {
			continue
		}
		results = append(results, models.PostWithAuthor{
			Post: *post,
			Author: models.PostAuthor{
				Type: post.PostedByType,
				ID:   post.PostedByID,
			},
			VoteScore: post.Upvotes - post.Downvotes,
		})
	}

	total := len(results)
	page := opts.Page
	if page < 1 {
		page = 1
	}
	perPage := opts.PerPage
	if perPage < 1 {
		perPage = 20
	}

	start := (page - 1) * perPage
	if start >= len(results) {
		return []models.PostWithAuthor{}, total, nil
	}
	end := start + perPage
	if end > len(results) {
		end = len(results)
	}

	return results[start:end], total, nil
}

// FindProblemByID returns a single problem by ID.
func (r *InMemoryProblemsRepository) FindProblemByID(ctx context.Context, id string) (*models.PostWithAuthor, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	post, exists := r.posts[id]
	if !exists || post.DeletedAt != nil || post.Type != models.PostTypeProblem {
		return nil, errProblemNotFound
	}

	return &models.PostWithAuthor{
		Post: *post,
		Author: models.PostAuthor{
			Type: post.PostedByType,
			ID:   post.PostedByID,
		},
		VoteScore: post.Upvotes - post.Downvotes,
	}, nil
}

// CreateProblem creates a new problem and returns it.
func (r *InMemoryProblemsRepository) CreateProblem(ctx context.Context, post *models.Post) (*models.Post, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if post.ID == "" {
		post.ID = "prob_" + time.Now().Format("20060102150405")
	}
	post.Type = models.PostTypeProblem
	now := time.Now()
	if post.CreatedAt.IsZero() {
		post.CreatedAt = now
	}
	post.UpdatedAt = now

	postCopy := *post
	r.posts[post.ID] = &postCopy
	return &postCopy, nil
}

// ListApproaches returns approaches for a problem.
func (r *InMemoryProblemsRepository) ListApproaches(ctx context.Context, problemID string, opts models.ApproachListOptions) ([]models.ApproachWithAuthor, int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var results []models.ApproachWithAuthor
	for _, appr := range r.approaches {
		if appr.ProblemID != problemID || appr.DeletedAt != nil {
			continue
		}
		results = append(results, models.ApproachWithAuthor{
			Approach: *appr,
			Author: models.ApproachAuthor{
				Type: appr.AuthorType,
				ID:   appr.AuthorID,
			},
		})
	}

	total := len(results)
	return results, total, nil
}

// CreateApproach creates a new approach and returns it.
func (r *InMemoryProblemsRepository) CreateApproach(ctx context.Context, approach *models.Approach) (*models.Approach, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if approach.ID == "" {
		approach.ID = "appr_" + time.Now().Format("20060102150405")
	}
	now := time.Now()
	if approach.CreatedAt.IsZero() {
		approach.CreatedAt = now
	}
	approach.UpdatedAt = now

	apprCopy := *approach
	r.approaches[approach.ID] = &apprCopy
	return &apprCopy, nil
}

// FindApproachByID returns a single approach by ID.
func (r *InMemoryProblemsRepository) FindApproachByID(ctx context.Context, id string) (*models.ApproachWithAuthor, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	appr, exists := r.approaches[id]
	if !exists || appr.DeletedAt != nil {
		return nil, errApproachNotFound
	}

	return &models.ApproachWithAuthor{
		Approach: *appr,
		Author: models.ApproachAuthor{
			Type: appr.AuthorType,
			ID:   appr.AuthorID,
		},
	}, nil
}

// UpdateApproach updates an existing approach and returns it.
func (r *InMemoryProblemsRepository) UpdateApproach(ctx context.Context, approach *models.Approach) (*models.Approach, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	existing, exists := r.approaches[approach.ID]
	if !exists || existing.DeletedAt != nil {
		return nil, errApproachNotFound
	}

	approach.UpdatedAt = time.Now()
	apprCopy := *approach
	r.approaches[approach.ID] = &apprCopy
	return &apprCopy, nil
}

// AddProgressNote adds a progress note to an approach.
func (r *InMemoryProblemsRepository) AddProgressNote(ctx context.Context, note *models.ProgressNote) (*models.ProgressNote, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if note.ID == "" {
		note.ID = "note_" + time.Now().Format("20060102150405")
	}
	if note.CreatedAt.IsZero() {
		note.CreatedAt = time.Now()
	}

	noteCopy := *note
	r.notes[note.ApproachID] = append(r.notes[note.ApproachID], &noteCopy)
	return &noteCopy, nil
}

// UpdateProblemStatus updates the status of a problem.
func (r *InMemoryProblemsRepository) UpdateProblemStatus(ctx context.Context, problemID string, status models.PostStatus) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	post, exists := r.posts[problemID]
	if !exists || post.DeletedAt != nil {
		return errProblemNotFound
	}

	post.Status = status
	post.UpdatedAt = time.Now()
	return nil
}

// InMemoryQuestionsRepository is an in-memory implementation of QuestionsRepositoryInterface.
type InMemoryQuestionsRepository struct {
	mu      sync.RWMutex
	posts   map[string]*models.Post
	answers map[string]*models.Answer
}

// NewInMemoryQuestionsRepository creates a new in-memory questions repository.
func NewInMemoryQuestionsRepository() *InMemoryQuestionsRepository {
	return &InMemoryQuestionsRepository{
		posts:   make(map[string]*models.Post),
		answers: make(map[string]*models.Answer),
	}
}

// ListQuestions returns questions matching the given options.
func (r *InMemoryQuestionsRepository) ListQuestions(ctx context.Context, opts models.PostListOptions) ([]models.PostWithAuthor, int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var results []models.PostWithAuthor
	for _, post := range r.posts {
		if post.DeletedAt != nil || post.Type != models.PostTypeQuestion {
			continue
		}
		if opts.Status != "" && post.Status != opts.Status {
			continue
		}
		results = append(results, models.PostWithAuthor{
			Post: *post,
			Author: models.PostAuthor{
				Type: post.PostedByType,
				ID:   post.PostedByID,
			},
			VoteScore: post.Upvotes - post.Downvotes,
		})
	}

	total := len(results)
	page := opts.Page
	if page < 1 {
		page = 1
	}
	perPage := opts.PerPage
	if perPage < 1 {
		perPage = 20
	}

	start := (page - 1) * perPage
	if start >= len(results) {
		return []models.PostWithAuthor{}, total, nil
	}
	end := start + perPage
	if end > len(results) {
		end = len(results)
	}

	return results[start:end], total, nil
}

// FindQuestionByID returns a single question by ID.
func (r *InMemoryQuestionsRepository) FindQuestionByID(ctx context.Context, id string) (*models.PostWithAuthor, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	post, exists := r.posts[id]
	if !exists || post.DeletedAt != nil || post.Type != models.PostTypeQuestion {
		return nil, errQuestionNotFound
	}

	return &models.PostWithAuthor{
		Post: *post,
		Author: models.PostAuthor{
			Type: post.PostedByType,
			ID:   post.PostedByID,
		},
		VoteScore: post.Upvotes - post.Downvotes,
	}, nil
}

// CreateQuestion creates a new question and returns it.
func (r *InMemoryQuestionsRepository) CreateQuestion(ctx context.Context, post *models.Post) (*models.Post, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if post.ID == "" {
		post.ID = "ques_" + time.Now().Format("20060102150405")
	}
	post.Type = models.PostTypeQuestion
	now := time.Now()
	if post.CreatedAt.IsZero() {
		post.CreatedAt = now
	}
	post.UpdatedAt = now

	postCopy := *post
	r.posts[post.ID] = &postCopy
	return &postCopy, nil
}

// ListAnswers returns answers for a question.
func (r *InMemoryQuestionsRepository) ListAnswers(ctx context.Context, questionID string, opts models.AnswerListOptions) ([]models.AnswerWithAuthor, int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var results []models.AnswerWithAuthor
	for _, ans := range r.answers {
		if ans.QuestionID != questionID || ans.DeletedAt != nil {
			continue
		}
		results = append(results, models.AnswerWithAuthor{
			Answer: *ans,
			Author: models.AnswerAuthor{
				Type: ans.AuthorType,
				ID:   ans.AuthorID,
			},
		})
	}

	return results, len(results), nil
}

// CreateAnswer creates a new answer and returns it.
func (r *InMemoryQuestionsRepository) CreateAnswer(ctx context.Context, answer *models.Answer) (*models.Answer, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if answer.ID == "" {
		answer.ID = "ans_" + time.Now().Format("20060102150405")
	}
	if answer.CreatedAt.IsZero() {
		answer.CreatedAt = time.Now()
	}

	ansCopy := *answer
	r.answers[answer.ID] = &ansCopy
	return &ansCopy, nil
}

// FindAnswerByID returns a single answer by ID.
func (r *InMemoryQuestionsRepository) FindAnswerByID(ctx context.Context, id string) (*models.AnswerWithAuthor, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	ans, exists := r.answers[id]
	if !exists || ans.DeletedAt != nil {
		return nil, errAnswerNotFound
	}

	return &models.AnswerWithAuthor{
		Answer: *ans,
		Author: models.AnswerAuthor{
			Type: ans.AuthorType,
			ID:   ans.AuthorID,
		},
	}, nil
}

// UpdateAnswer updates an existing answer and returns it.
func (r *InMemoryQuestionsRepository) UpdateAnswer(ctx context.Context, answer *models.Answer) (*models.Answer, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	existing, exists := r.answers[answer.ID]
	if !exists || existing.DeletedAt != nil {
		return nil, errAnswerNotFound
	}

	ansCopy := *answer
	r.answers[answer.ID] = &ansCopy
	return &ansCopy, nil
}

// DeleteAnswer soft-deletes an answer.
func (r *InMemoryQuestionsRepository) DeleteAnswer(ctx context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	ans, exists := r.answers[id]
	if !exists || ans.DeletedAt != nil {
		return errAnswerNotFound
	}

	now := time.Now()
	ans.DeletedAt = &now
	return nil
}

// AcceptAnswer marks an answer as accepted and updates question status.
func (r *InMemoryQuestionsRepository) AcceptAnswer(ctx context.Context, questionID, answerID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	post, exists := r.posts[questionID]
	if !exists || post.DeletedAt != nil {
		return errQuestionNotFound
	}

	ans, exists := r.answers[answerID]
	if !exists || ans.DeletedAt != nil {
		return errAnswerNotFound
	}

	post.AcceptedAnswerID = &answerID
	post.Status = models.PostStatusAnswered
	post.UpdatedAt = time.Now()
	ans.IsAccepted = true
	return nil
}

// VoteOnAnswer records a vote on an answer.
func (r *InMemoryQuestionsRepository) VoteOnAnswer(ctx context.Context, answerID, voterType, voterID, direction string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	ans, exists := r.answers[answerID]
	if !exists || ans.DeletedAt != nil {
		return errAnswerNotFound
	}

	if direction == "up" {
		ans.Upvotes++
	} else if direction == "down" {
		ans.Downvotes++
	}
	return nil
}

// InMemoryIdeasRepository is an in-memory implementation of IdeasRepositoryInterface.
type InMemoryIdeasRepository struct {
	mu        sync.RWMutex
	posts     map[string]*models.Post
	responses map[string]*models.Response
}

// NewInMemoryIdeasRepository creates a new in-memory ideas repository.
func NewInMemoryIdeasRepository() *InMemoryIdeasRepository {
	return &InMemoryIdeasRepository{
		posts:     make(map[string]*models.Post),
		responses: make(map[string]*models.Response),
	}
}

// ListIdeas returns ideas matching the given options.
func (r *InMemoryIdeasRepository) ListIdeas(ctx context.Context, opts models.PostListOptions) ([]models.PostWithAuthor, int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var results []models.PostWithAuthor
	for _, post := range r.posts {
		if post.DeletedAt != nil || post.Type != models.PostTypeIdea {
			continue
		}
		if opts.Status != "" && post.Status != opts.Status {
			continue
		}
		results = append(results, models.PostWithAuthor{
			Post: *post,
			Author: models.PostAuthor{
				Type: post.PostedByType,
				ID:   post.PostedByID,
			},
			VoteScore: post.Upvotes - post.Downvotes,
		})
	}

	total := len(results)
	page := opts.Page
	if page < 1 {
		page = 1
	}
	perPage := opts.PerPage
	if perPage < 1 {
		perPage = 20
	}

	start := (page - 1) * perPage
	if start >= len(results) {
		return []models.PostWithAuthor{}, total, nil
	}
	end := start + perPage
	if end > len(results) {
		end = len(results)
	}

	return results[start:end], total, nil
}

// FindIdeaByID returns a single idea by ID.
func (r *InMemoryIdeasRepository) FindIdeaByID(ctx context.Context, id string) (*models.PostWithAuthor, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	post, exists := r.posts[id]
	if !exists || post.DeletedAt != nil || post.Type != models.PostTypeIdea {
		return nil, errIdeaNotFound
	}

	return &models.PostWithAuthor{
		Post: *post,
		Author: models.PostAuthor{
			Type: post.PostedByType,
			ID:   post.PostedByID,
		},
		VoteScore: post.Upvotes - post.Downvotes,
	}, nil
}

// CreateIdea creates a new idea and returns it.
func (r *InMemoryIdeasRepository) CreateIdea(ctx context.Context, post *models.Post) (*models.Post, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if post.ID == "" {
		post.ID = "idea_" + time.Now().Format("20060102150405")
	}
	post.Type = models.PostTypeIdea
	now := time.Now()
	if post.CreatedAt.IsZero() {
		post.CreatedAt = now
	}
	post.UpdatedAt = now

	postCopy := *post
	r.posts[post.ID] = &postCopy
	return &postCopy, nil
}

// ListResponses returns responses for an idea.
func (r *InMemoryIdeasRepository) ListResponses(ctx context.Context, ideaID string, opts models.ResponseListOptions) ([]models.ResponseWithAuthor, int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var results []models.ResponseWithAuthor
	for _, resp := range r.responses {
		if resp.IdeaID != ideaID {
			continue
		}
		results = append(results, models.ResponseWithAuthor{
			Response: *resp,
			Author: models.ResponseAuthor{
				Type: resp.AuthorType,
				ID:   resp.AuthorID,
			},
		})
	}

	return results, len(results), nil
}

// CreateResponse creates a new response and returns it.
func (r *InMemoryIdeasRepository) CreateResponse(ctx context.Context, response *models.Response) (*models.Response, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if response.ID == "" {
		response.ID = "resp_" + time.Now().Format("20060102150405")
	}
	if response.CreatedAt.IsZero() {
		response.CreatedAt = time.Now()
	}

	respCopy := *response
	r.responses[response.ID] = &respCopy
	return &respCopy, nil
}

// AddEvolvedInto adds a post ID to the idea's evolved_into array.
func (r *InMemoryIdeasRepository) AddEvolvedInto(ctx context.Context, ideaID, evolvedPostID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	post, exists := r.posts[ideaID]
	if !exists || post.DeletedAt != nil {
		return errIdeaNotFound
	}

	post.EvolvedInto = append(post.EvolvedInto, evolvedPostID)
	post.UpdatedAt = time.Now()
	return nil
}

// FindPostByID returns a single post by ID (for verifying evolved post exists).
func (r *InMemoryIdeasRepository) FindPostByID(ctx context.Context, id string) (*models.PostWithAuthor, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	post, exists := r.posts[id]
	if !exists || post.DeletedAt != nil {
		return nil, errIdeaNotFound
	}

	return &models.PostWithAuthor{
		Post: *post,
		Author: models.PostAuthor{
			Type: post.PostedByType,
			ID:   post.PostedByID,
		},
		VoteScore: post.Upvotes - post.Downvotes,
	}, nil
}

// InMemoryCommentsRepository is an in-memory implementation of CommentsRepositoryInterface.
type InMemoryCommentsRepository struct {
	mu       sync.RWMutex
	comments map[string]*models.Comment
}

// NewInMemoryCommentsRepository creates a new in-memory comments repository.
func NewInMemoryCommentsRepository() *InMemoryCommentsRepository {
	return &InMemoryCommentsRepository{
		comments: make(map[string]*models.Comment),
	}
}

// List returns comments for a target.
func (r *InMemoryCommentsRepository) List(ctx context.Context, opts models.CommentListOptions) ([]models.CommentWithAuthor, int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var results []models.CommentWithAuthor
	for _, comment := range r.comments {
		if comment.DeletedAt != nil {
			continue
		}
		if string(comment.TargetType) != string(opts.TargetType) || comment.TargetID != opts.TargetID {
			continue
		}
		results = append(results, models.CommentWithAuthor{
			Comment: *comment,
			Author: models.CommentAuthor{
				Type: comment.AuthorType,
				ID:   comment.AuthorID,
			},
		})
	}

	total := len(results)
	page := opts.Page
	if page < 1 {
		page = 1
	}
	perPage := opts.PerPage
	if perPage < 1 {
		perPage = 20
	}

	start := (page - 1) * perPage
	if start >= len(results) {
		return []models.CommentWithAuthor{}, total, nil
	}
	end := start + perPage
	if end > len(results) {
		end = len(results)
	}

	return results[start:end], total, nil
}

// Create creates a new comment.
func (r *InMemoryCommentsRepository) Create(ctx context.Context, comment *models.Comment) (*models.Comment, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if comment.ID == "" {
		comment.ID = "comm_" + time.Now().Format("20060102150405")
	}
	if comment.CreatedAt.IsZero() {
		comment.CreatedAt = time.Now()
	}

	commentCopy := *comment
	r.comments[comment.ID] = &commentCopy
	return &commentCopy, nil
}

// FindByID returns a single comment by ID.
func (r *InMemoryCommentsRepository) FindByID(ctx context.Context, id string) (*models.CommentWithAuthor, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	comment, exists := r.comments[id]
	if !exists || comment.DeletedAt != nil {
		return nil, errCommentNotFound
	}

	return &models.CommentWithAuthor{
		Comment: *comment,
		Author: models.CommentAuthor{
			Type: comment.AuthorType,
			ID:   comment.AuthorID,
		},
	}, nil
}

// Delete soft-deletes a comment by ID.
func (r *InMemoryCommentsRepository) Delete(ctx context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	comment, exists := r.comments[id]
	if !exists || comment.DeletedAt != nil {
		return errCommentNotFound
	}

	now := time.Now()
	comment.DeletedAt = &now
	return nil
}

// TargetExists checks if the target (approach, answer, response) exists.
func (r *InMemoryCommentsRepository) TargetExists(ctx context.Context, targetType models.CommentTargetType, targetID string) (bool, error) {
	// For in-memory testing, always return true
	return true, nil
}
