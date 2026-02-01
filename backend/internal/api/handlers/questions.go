// Package handlers contains HTTP request handlers for the Solvr API.
package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/fcavalcantirj/solvr/internal/auth"
	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/go-chi/chi/v5"
)

// QuestionsRepositoryInterface defines the database operations for questions.
type QuestionsRepositoryInterface interface {
	// ListQuestions returns questions matching the given options.
	ListQuestions(ctx context.Context, opts models.PostListOptions) ([]models.PostWithAuthor, int, error)

	// FindQuestionByID returns a single question by ID.
	FindQuestionByID(ctx context.Context, id string) (*models.PostWithAuthor, error)

	// CreateQuestion creates a new question and returns it.
	CreateQuestion(ctx context.Context, post *models.Post) (*models.Post, error)

	// ListAnswers returns answers for a question.
	ListAnswers(ctx context.Context, questionID string, opts models.AnswerListOptions) ([]models.AnswerWithAuthor, int, error)

	// CreateAnswer creates a new answer and returns it.
	CreateAnswer(ctx context.Context, answer *models.Answer) (*models.Answer, error)

	// FindAnswerByID returns a single answer by ID.
	FindAnswerByID(ctx context.Context, id string) (*models.AnswerWithAuthor, error)

	// UpdateAnswer updates an existing answer and returns it.
	UpdateAnswer(ctx context.Context, answer *models.Answer) (*models.Answer, error)

	// DeleteAnswer soft-deletes an answer.
	DeleteAnswer(ctx context.Context, id string) error

	// AcceptAnswer marks an answer as accepted and updates question status.
	AcceptAnswer(ctx context.Context, questionID, answerID string) error

	// VoteOnAnswer records a vote on an answer.
	VoteOnAnswer(ctx context.Context, answerID, voterType, voterID, direction string) error
}

// QuestionsHandler handles question-related HTTP requests.
type QuestionsHandler struct {
	repo QuestionsRepositoryInterface
}

// NewQuestionsHandler creates a new QuestionsHandler.
func NewQuestionsHandler(repo QuestionsRepositoryInterface) *QuestionsHandler {
	return &QuestionsHandler{repo: repo}
}

// QuestionsListResponse is the response for listing questions.
type QuestionsListResponse struct {
	Data []models.PostWithAuthor `json:"data"`
	Meta QuestionsListMeta       `json:"meta"`
}

// QuestionsListMeta contains metadata for list responses.
type QuestionsListMeta struct {
	Total   int  `json:"total"`
	Page    int  `json:"page"`
	PerPage int  `json:"per_page"`
	HasMore bool `json:"has_more"`
}

// QuestionResponse is the response for a single question with answers.
type QuestionResponse struct {
	models.PostWithAuthor
	Answers []models.AnswerWithAuthor `json:"answers"`
}

// CreateQuestionRequest is the request body for creating a question.
type CreateQuestionRequest struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Tags        []string `json:"tags,omitempty"`
}

// VoteRequest is the request body for voting.
type VoteRequest struct {
	Direction string `json:"direction"`
}

// List handles GET /v1/questions - list questions.
func (h *QuestionsHandler) List(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters
	opts := models.PostListOptions{
		Type:    models.PostTypeQuestion, // Always filter by question type
		Page:    parseQuestionsIntParam(r.URL.Query().Get("page"), 1),
		PerPage: parseQuestionsIntParam(r.URL.Query().Get("per_page"), 20),
	}

	if opts.Page < 1 {
		opts.Page = 1
	}
	if opts.PerPage < 1 {
		opts.PerPage = 20
	}
	if opts.PerPage > 50 {
		opts.PerPage = 50 // Cap at 50 per SPEC.md
	}

	// Parse status filter
	if statusParam := r.URL.Query().Get("status"); statusParam != "" {
		opts.Status = models.PostStatus(statusParam)
	}

	// Parse tags filter
	if tagsParam := r.URL.Query().Get("tags"); tagsParam != "" {
		opts.Tags = strings.Split(tagsParam, ",")
		for i, tag := range opts.Tags {
			opts.Tags[i] = strings.TrimSpace(tag)
		}
	}

	// Execute query
	questions, total, err := h.repo.ListQuestions(r.Context(), opts)
	if err != nil {
		writeQuestionsError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to list questions")
		return
	}

	// Calculate has_more
	hasMore := (opts.Page * opts.PerPage) < total

	response := QuestionsListResponse{
		Data: questions,
		Meta: QuestionsListMeta{
			Total:   total,
			Page:    opts.Page,
			PerPage: opts.PerPage,
			HasMore: hasMore,
		},
	}

	writeQuestionsJSON(w, http.StatusOK, response)
}

// Get handles GET /v1/questions/:id - get a single question with answers.
func (h *QuestionsHandler) Get(w http.ResponseWriter, r *http.Request) {
	questionID := chi.URLParam(r, "id")
	if questionID == "" {
		writeQuestionsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "question ID is required")
		return
	}

	question, err := h.repo.FindQuestionByID(r.Context(), questionID)
	if err != nil {
		if errors.Is(err, ErrQuestionNotFound) {
			writeQuestionsError(w, http.StatusNotFound, "NOT_FOUND", "question not found")
			return
		}
		writeQuestionsError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get question")
		return
	}

	// Check if it's actually a question
	if question.Type != models.PostTypeQuestion {
		writeQuestionsError(w, http.StatusNotFound, "NOT_FOUND", "question not found")
		return
	}

	// Check if deleted
	if question.DeletedAt != nil {
		writeQuestionsError(w, http.StatusNotFound, "NOT_FOUND", "question not found")
		return
	}

	// Get answers for the question
	answers, _, err := h.repo.ListAnswers(r.Context(), questionID, models.AnswerListOptions{
		QuestionID: questionID,
		Page:       1,
		PerPage:    100, // Get up to 100 answers
	})
	if err != nil {
		writeQuestionsError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get answers")
		return
	}

	response := QuestionResponse{
		PostWithAuthor: *question,
		Answers:        answers,
	}

	writeQuestionsJSON(w, http.StatusOK, map[string]interface{}{
		"data": response,
	})
}

// Create handles POST /v1/questions - create a new question.
func (h *QuestionsHandler) Create(w http.ResponseWriter, r *http.Request) {
	// Require authentication
	claims := auth.ClaimsFromContext(r.Context())
	if claims == nil {
		writeQuestionsError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}

	// Parse request body
	var req CreateQuestionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeQuestionsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid JSON body")
		return
	}

	// Validate title
	if req.Title == "" {
		writeQuestionsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "title is required")
		return
	}
	if len(req.Title) < 10 {
		writeQuestionsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "title must be at least 10 characters")
		return
	}
	if len(req.Title) > 200 {
		writeQuestionsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "title must be at most 200 characters")
		return
	}

	// Validate description
	if req.Description == "" {
		writeQuestionsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "description is required")
		return
	}
	if len(req.Description) < 50 {
		writeQuestionsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "description must be at least 50 characters")
		return
	}

	// Validate tags (max 5)
	if len(req.Tags) > 5 {
		writeQuestionsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "maximum 5 tags allowed")
		return
	}

	// Create question
	post := &models.Post{
		Type:         models.PostTypeQuestion,
		Title:        req.Title,
		Description:  req.Description,
		Tags:         req.Tags,
		PostedByType: models.AuthorTypeHuman, // TODO: Support agent auth
		PostedByID:   claims.UserID,
		Status:       models.PostStatusOpen,
	}

	createdPost, err := h.repo.CreateQuestion(r.Context(), post)
	if err != nil {
		writeQuestionsError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to create question")
		return
	}

	writeQuestionsJSON(w, http.StatusCreated, map[string]interface{}{
		"data": createdPost,
	})
}

// CreateAnswer handles POST /v1/questions/:id/answers - create a new answer.
func (h *QuestionsHandler) CreateAnswer(w http.ResponseWriter, r *http.Request) {
	// Require authentication
	claims := auth.ClaimsFromContext(r.Context())
	if claims == nil {
		writeQuestionsError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}

	questionID := chi.URLParam(r, "id")
	if questionID == "" {
		writeQuestionsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "question ID is required")
		return
	}

	// Verify question exists
	question, err := h.repo.FindQuestionByID(r.Context(), questionID)
	if err != nil {
		if errors.Is(err, ErrQuestionNotFound) {
			writeQuestionsError(w, http.StatusNotFound, "NOT_FOUND", "question not found")
			return
		}
		writeQuestionsError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get question")
		return
	}

	if question.Type != models.PostTypeQuestion {
		writeQuestionsError(w, http.StatusNotFound, "NOT_FOUND", "question not found")
		return
	}

	// Parse request body
	var req models.CreateAnswerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeQuestionsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid JSON body")
		return
	}

	// Validate content
	if req.Content == "" {
		writeQuestionsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "content is required")
		return
	}
	if len(req.Content) > 30000 {
		writeQuestionsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "content must be at most 30000 characters")
		return
	}

	// Create answer
	answer := &models.Answer{
		QuestionID: questionID,
		AuthorType: models.AuthorTypeHuman, // TODO: Support agent auth
		AuthorID:   claims.UserID,
		Content:    req.Content,
		IsAccepted: false,
	}

	createdAnswer, err := h.repo.CreateAnswer(r.Context(), answer)
	if err != nil {
		writeQuestionsError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to create answer")
		return
	}

	writeQuestionsJSON(w, http.StatusCreated, map[string]interface{}{
		"data": createdAnswer,
	})
}

// UpdateAnswer handles PATCH /v1/answers/:id - update an answer.
func (h *QuestionsHandler) UpdateAnswer(w http.ResponseWriter, r *http.Request) {
	// Require authentication
	claims := auth.ClaimsFromContext(r.Context())
	if claims == nil {
		writeQuestionsError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}

	answerID := chi.URLParam(r, "id")
	if answerID == "" {
		writeQuestionsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "answer ID is required")
		return
	}

	// Get existing answer
	existingAnswer, err := h.repo.FindAnswerByID(r.Context(), answerID)
	if err != nil {
		if errors.Is(err, ErrAnswerNotFound) {
			writeQuestionsError(w, http.StatusNotFound, "NOT_FOUND", "answer not found")
			return
		}
		writeQuestionsError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get answer")
		return
	}

	// Check ownership - only author can update
	if existingAnswer.AuthorType != models.AuthorTypeHuman || existingAnswer.AuthorID != claims.UserID {
		writeQuestionsError(w, http.StatusForbidden, "FORBIDDEN", "you can only update your own answers")
		return
	}

	// Parse request body
	var req models.UpdateAnswerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeQuestionsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid JSON body")
		return
	}

	// Apply updates
	updatedAnswer := existingAnswer.Answer

	if req.Content != nil {
		if len(*req.Content) > 30000 {
			writeQuestionsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "content must be at most 30000 characters")
			return
		}
		updatedAnswer.Content = *req.Content
	}

	result, err := h.repo.UpdateAnswer(r.Context(), &updatedAnswer)
	if err != nil {
		writeQuestionsError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to update answer")
		return
	}

	writeQuestionsJSON(w, http.StatusOK, map[string]interface{}{
		"data": result,
	})
}

// DeleteAnswer handles DELETE /v1/answers/:id - soft delete an answer.
func (h *QuestionsHandler) DeleteAnswer(w http.ResponseWriter, r *http.Request) {
	// Require authentication
	claims := auth.ClaimsFromContext(r.Context())
	if claims == nil {
		writeQuestionsError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}

	answerID := chi.URLParam(r, "id")
	if answerID == "" {
		writeQuestionsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "answer ID is required")
		return
	}

	// Get existing answer
	existingAnswer, err := h.repo.FindAnswerByID(r.Context(), answerID)
	if err != nil {
		if errors.Is(err, ErrAnswerNotFound) {
			writeQuestionsError(w, http.StatusNotFound, "NOT_FOUND", "answer not found")
			return
		}
		writeQuestionsError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get answer")
		return
	}

	// Check ownership - author or admin can delete
	isOwner := existingAnswer.AuthorType == models.AuthorTypeHuman && existingAnswer.AuthorID == claims.UserID
	isAdmin := claims.Role == "admin"

	if !isOwner && !isAdmin {
		writeQuestionsError(w, http.StatusForbidden, "FORBIDDEN", "you can only delete your own answers")
		return
	}

	// Delete answer
	if err := h.repo.DeleteAnswer(r.Context(), answerID); err != nil {
		writeQuestionsError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to delete answer")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// VoteOnAnswer handles POST /v1/answers/:id/vote - vote on an answer.
func (h *QuestionsHandler) VoteOnAnswer(w http.ResponseWriter, r *http.Request) {
	// Require authentication
	claims := auth.ClaimsFromContext(r.Context())
	if claims == nil {
		writeQuestionsError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}

	answerID := chi.URLParam(r, "id")
	if answerID == "" {
		writeQuestionsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "answer ID is required")
		return
	}

	// Parse request body
	var req VoteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeQuestionsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid JSON body")
		return
	}

	// Validate direction
	if req.Direction != "up" && req.Direction != "down" {
		writeQuestionsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "direction must be 'up' or 'down'")
		return
	}

	// Verify answer exists
	_, err := h.repo.FindAnswerByID(r.Context(), answerID)
	if err != nil {
		if errors.Is(err, ErrAnswerNotFound) {
			writeQuestionsError(w, http.StatusNotFound, "NOT_FOUND", "answer not found")
			return
		}
		writeQuestionsError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get answer")
		return
	}

	// Record vote
	if err := h.repo.VoteOnAnswer(r.Context(), answerID, "human", claims.UserID, req.Direction); err != nil {
		writeQuestionsError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to record vote")
		return
	}

	writeQuestionsJSON(w, http.StatusOK, map[string]interface{}{
		"message": "vote recorded",
	})
}

// AcceptAnswer handles POST /v1/questions/:id/accept/:aid - accept an answer.
func (h *QuestionsHandler) AcceptAnswer(w http.ResponseWriter, r *http.Request) {
	// Require authentication
	claims := auth.ClaimsFromContext(r.Context())
	if claims == nil {
		writeQuestionsError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}

	questionID := chi.URLParam(r, "id")
	answerID := chi.URLParam(r, "aid")

	if questionID == "" {
		writeQuestionsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "question ID is required")
		return
	}
	if answerID == "" {
		writeQuestionsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "answer ID is required")
		return
	}

	// Get the question to verify ownership
	question, err := h.repo.FindQuestionByID(r.Context(), questionID)
	if err != nil {
		if errors.Is(err, ErrQuestionNotFound) {
			writeQuestionsError(w, http.StatusNotFound, "NOT_FOUND", "question not found")
			return
		}
		writeQuestionsError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get question")
		return
	}

	// Check ownership - only question owner can accept
	if question.PostedByType != models.AuthorTypeHuman || question.PostedByID != claims.UserID {
		writeQuestionsError(w, http.StatusForbidden, "FORBIDDEN", "only the question owner can accept answers")
		return
	}

	// Verify answer exists
	_, err = h.repo.FindAnswerByID(r.Context(), answerID)
	if err != nil {
		if errors.Is(err, ErrAnswerNotFound) {
			writeQuestionsError(w, http.StatusNotFound, "NOT_FOUND", "answer not found")
			return
		}
		writeQuestionsError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get answer")
		return
	}

	// Accept the answer
	if err := h.repo.AcceptAnswer(r.Context(), questionID, answerID); err != nil {
		writeQuestionsError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to accept answer")
		return
	}

	writeQuestionsJSON(w, http.StatusOK, map[string]interface{}{
		"message":   "answer accepted",
		"answer_id": answerID,
	})
}

// parseQuestionsIntParam parses a string to int with a default value.
func parseQuestionsIntParam(s string, defaultVal int) int {
	if s == "" {
		return defaultVal
	}
	val, err := strconv.Atoi(s)
	if err != nil {
		return defaultVal
	}
	return val
}

// writeQuestionsJSON writes a JSON response.
func writeQuestionsJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// writeQuestionsError writes an error JSON response.
func writeQuestionsError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]interface{}{
			"code":    code,
			"message": message,
		},
	})
}
