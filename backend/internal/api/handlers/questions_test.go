// Package handlers contains HTTP request handlers for the Solvr API.
package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/auth"
	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/go-chi/chi/v5"
)

// Note: ErrQuestionNotFound and ErrAnswerNotFound are defined in errors.go

// MockQuestionsRepository implements QuestionsRepositoryInterface for testing.
type MockQuestionsRepository struct {
	questions       []models.PostWithAuthor
	question        *models.PostWithAuthor
	total           int
	err             error
	listOpts        models.PostListOptions
	answers         []models.AnswerWithAuthor
	answer          *models.AnswerWithAuthor
	answersErr      error
	createdPost     *models.Post
	createdAnswer   *models.Answer
	updatedAnswer   *models.Answer
	updatedQuestion *models.Post
	deletedAnswerID string
}

func NewMockQuestionsRepository() *MockQuestionsRepository {
	return &MockQuestionsRepository{
		questions: []models.PostWithAuthor{},
		answers:   []models.AnswerWithAuthor{},
	}
}

func (m *MockQuestionsRepository) ListQuestions(ctx context.Context, opts models.PostListOptions) ([]models.PostWithAuthor, int, error) {
	m.listOpts = opts
	if m.err != nil {
		return nil, 0, m.err
	}
	return m.questions, m.total, nil
}

func (m *MockQuestionsRepository) FindQuestionByID(ctx context.Context, id string) (*models.PostWithAuthor, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.question == nil {
		return nil, ErrQuestionNotFound
	}
	return m.question, nil
}

func (m *MockQuestionsRepository) CreateQuestion(ctx context.Context, post *models.Post) (*models.Post, error) {
	if m.err != nil {
		return nil, m.err
	}
	m.createdPost = post
	post.ID = "new-question-id"
	post.CreatedAt = time.Now()
	post.UpdatedAt = time.Now()
	return post, nil
}

func (m *MockQuestionsRepository) ListAnswers(ctx context.Context, questionID string, opts models.AnswerListOptions) ([]models.AnswerWithAuthor, int, error) {
	if m.answersErr != nil {
		return nil, 0, m.answersErr
	}
	return m.answers, len(m.answers), nil
}

func (m *MockQuestionsRepository) CreateAnswer(ctx context.Context, answer *models.Answer) (*models.Answer, error) {
	if m.err != nil {
		return nil, m.err
	}
	m.createdAnswer = answer
	answer.ID = "new-answer-id"
	answer.CreatedAt = time.Now()
	return answer, nil
}

func (m *MockQuestionsRepository) FindAnswerByID(ctx context.Context, id string) (*models.AnswerWithAuthor, error) {
	if m.answersErr != nil {
		return nil, m.answersErr
	}
	if m.answer == nil {
		return nil, ErrAnswerNotFound
	}
	return m.answer, nil
}

func (m *MockQuestionsRepository) UpdateAnswer(ctx context.Context, answer *models.Answer) (*models.Answer, error) {
	if m.err != nil {
		return nil, m.err
	}
	m.updatedAnswer = answer
	return answer, nil
}

func (m *MockQuestionsRepository) DeleteAnswer(ctx context.Context, id string) error {
	if m.answersErr != nil {
		return m.answersErr
	}
	m.deletedAnswerID = id
	return nil
}

func (m *MockQuestionsRepository) AcceptAnswer(ctx context.Context, questionID, answerID string) error {
	if m.err != nil {
		return m.err
	}
	return nil
}

func (m *MockQuestionsRepository) VoteOnAnswer(ctx context.Context, answerID, voterType, voterID, direction string) error {
	if m.err != nil {
		return m.err
	}
	return nil
}

func (m *MockQuestionsRepository) SetQuestions(questions []models.PostWithAuthor, total int) {
	m.questions = questions
	m.total = total
}

func (m *MockQuestionsRepository) SetQuestion(question *models.PostWithAuthor) {
	m.question = question
}

func (m *MockQuestionsRepository) SetError(err error) {
	m.err = err
}

func (m *MockQuestionsRepository) SetAnswers(answers []models.AnswerWithAuthor) {
	m.answers = answers
}

func (m *MockQuestionsRepository) SetAnswer(answer *models.AnswerWithAuthor) {
	m.answer = answer
}

func (m *MockQuestionsRepository) SetAnswersError(err error) {
	m.answersErr = err
}

// createTestQuestion creates a test question with default values.
func createTestQuestion(id, title string) models.PostWithAuthor {
	now := time.Now()
	return models.PostWithAuthor{
		Post: models.Post{
			ID:           id,
			Type:         models.PostTypeQuestion,
			Title:        title,
			Description:  "Test question description with more than fifty characters for validation",
			Tags:         []string{"test", "go"},
			PostedByType: models.AuthorTypeHuman,
			PostedByID:   "user-123",
			Status:       models.PostStatusOpen,
			Upvotes:      10,
			Downvotes:    2,
			CreatedAt:    now,
			UpdatedAt:    now,
		},
		Author: models.PostAuthor{
			Type:        models.AuthorTypeHuman,
			ID:          "user-123",
			DisplayName: "Test User",
			AvatarURL:   "https://example.com/avatar.png",
		},
		VoteScore: 8,
	}
}

// createTestAnswer creates a test answer with default values.
func createTestAnswer(id, questionID string) models.AnswerWithAuthor {
	now := time.Now()
	return models.AnswerWithAuthor{
		Answer: models.Answer{
			ID:         id,
			QuestionID: questionID,
			AuthorType: models.AuthorTypeHuman,
			AuthorID:   "user-456",
			Content:    "This is a test answer with sufficient content length to be valid",
			IsAccepted: false,
			Upvotes:    5,
			Downvotes:  1,
			CreatedAt:  now,
		},
		Author: models.AnswerAuthor{
			Type:        models.AuthorTypeHuman,
			ID:          "user-456",
			DisplayName: "Test Contributor",
			AvatarURL:   "https://example.com/avatar2.png",
		},
		VoteScore: 4,
	}
}

// Helper to add auth claims to request context
func addQuestionsAuthContext(r *http.Request, userID, role string) *http.Request {
	claims := &auth.Claims{
		UserID: userID,
		Role:   role,
	}
	ctx := auth.ContextWithClaims(r.Context(), claims)
	return r.WithContext(ctx)
}

// ============================================================================
// GET /v1/questions - List Questions Tests
// ============================================================================

// TestListQuestions_Success tests successful listing of questions.
func TestListQuestions_Success(t *testing.T) {
	repo := NewMockQuestionsRepository()
	repo.SetQuestions([]models.PostWithAuthor{
		createTestQuestion("question-1", "First Question"),
		createTestQuestion("question-2", "Second Question"),
	}, 2)

	handler := NewQuestionsHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/questions", nil)
	w := httptest.NewRecorder()

	handler.List(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	data, ok := resp["data"].([]interface{})
	if !ok {
		t.Fatal("expected data array in response")
	}

	if len(data) != 2 {
		t.Errorf("expected 2 questions, got %d", len(data))
	}
}

// TestListQuestions_FiltersType tests that type is automatically set to question.
func TestListQuestions_FiltersType(t *testing.T) {
	repo := NewMockQuestionsRepository()
	repo.SetQuestions([]models.PostWithAuthor{}, 0)

	handler := NewQuestionsHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/questions", nil)
	w := httptest.NewRecorder()

	handler.List(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	// Verify the filter was set to question
	if repo.listOpts.Type != models.PostTypeQuestion {
		t.Errorf("expected type filter 'question', got '%s'", repo.listOpts.Type)
	}
}

// TestListQuestions_FilterByStatus tests filtering by status.
func TestListQuestions_FilterByStatus(t *testing.T) {
	repo := NewMockQuestionsRepository()
	repo.SetQuestions([]models.PostWithAuthor{}, 0)

	handler := NewQuestionsHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/questions?status=answered", nil)
	w := httptest.NewRecorder()

	handler.List(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	if repo.listOpts.Status != models.PostStatusAnswered {
		t.Errorf("expected status filter 'answered', got '%s'", repo.listOpts.Status)
	}
}

// TestListQuestions_Pagination tests pagination parameters.
func TestListQuestions_Pagination(t *testing.T) {
	repo := NewMockQuestionsRepository()
	repo.SetQuestions([]models.PostWithAuthor{}, 100)

	handler := NewQuestionsHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/questions?page=2&per_page=10", nil)
	w := httptest.NewRecorder()

	handler.List(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	if repo.listOpts.Page != 2 {
		t.Errorf("expected page 2, got %d", repo.listOpts.Page)
	}

	if repo.listOpts.PerPage != 10 {
		t.Errorf("expected per_page 10, got %d", repo.listOpts.PerPage)
	}
}

// TestListQuestions_SortByVotes tests sort=votes parameter.
func TestListQuestions_SortByVotes(t *testing.T) {
	repo := NewMockQuestionsRepository()
	repo.SetQuestions([]models.PostWithAuthor{}, 0)

	handler := NewQuestionsHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/questions?sort=votes", nil)
	w := httptest.NewRecorder()

	handler.List(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	if repo.listOpts.Sort != "votes" {
		t.Errorf("expected sort 'votes', got '%s'", repo.listOpts.Sort)
	}
}

// TestListQuestions_SortByAnswers tests sort=answers parameter.
func TestListQuestions_SortByAnswers(t *testing.T) {
	repo := NewMockQuestionsRepository()
	repo.SetQuestions([]models.PostWithAuthor{}, 0)

	handler := NewQuestionsHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/questions?sort=answers", nil)
	w := httptest.NewRecorder()

	handler.List(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	if repo.listOpts.Sort != "answers" {
		t.Errorf("expected sort 'answers', got '%s'", repo.listOpts.Sort)
	}
}

// TestListQuestions_SortByNewest tests sort=newest parameter.
func TestListQuestions_SortByNewest(t *testing.T) {
	repo := NewMockQuestionsRepository()
	repo.SetQuestions([]models.PostWithAuthor{}, 0)

	handler := NewQuestionsHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/questions?sort=newest", nil)
	w := httptest.NewRecorder()

	handler.List(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	if repo.listOpts.Sort != "newest" {
		t.Errorf("expected sort 'newest', got '%s'", repo.listOpts.Sort)
	}
}

// TestListQuestions_InvalidSortDefaultsToEmpty tests that invalid sort values are silently ignored.
func TestListQuestions_InvalidSortDefaultsToEmpty(t *testing.T) {
	repo := NewMockQuestionsRepository()
	repo.SetQuestions([]models.PostWithAuthor{}, 0)

	handler := NewQuestionsHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/questions?sort=invalid", nil)
	w := httptest.NewRecorder()

	handler.List(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	if repo.listOpts.Sort != "" {
		t.Errorf("expected empty sort (default), got '%s'", repo.listOpts.Sort)
	}
}

// TestListQuestions_HasMore tests has_more pagination flag.
func TestListQuestions_HasMore(t *testing.T) {
	repo := NewMockQuestionsRepository()
	repo.SetQuestions([]models.PostWithAuthor{
		createTestQuestion("q-1", "Question 1"),
	}, 50) // Total 50, showing first 20

	handler := NewQuestionsHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/questions", nil)
	w := httptest.NewRecorder()

	handler.List(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	meta := resp["meta"].(map[string]interface{})
	if meta["has_more"] != true {
		t.Errorf("expected has_more=true, got %v", meta["has_more"])
	}
}

// ============================================================================
// GET /v1/questions/:id - Get Single Question Tests
// ============================================================================

// TestGetQuestion_Success tests successful retrieval of a question.
func TestGetQuestion_Success(t *testing.T) {
	repo := NewMockQuestionsRepository()
	question := createTestQuestion("question-123", "Test Question")
	repo.SetQuestion(&question)

	handler := NewQuestionsHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/questions/question-123", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "question-123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	handler.Get(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	data := resp["data"].(map[string]interface{})
	if data["id"] != "question-123" {
		t.Errorf("expected question id 'question-123', got %v", data["id"])
	}
}

// TestGetQuestion_NotFound tests 404 for non-existent question.
func TestGetQuestion_NotFound(t *testing.T) {
	repo := NewMockQuestionsRepository()
	repo.SetQuestion(nil)

	handler := NewQuestionsHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/questions/nonexistent", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "nonexistent")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	handler.Get(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	errObj := resp["error"].(map[string]interface{})
	if errObj["code"] != "NOT_FOUND" {
		t.Errorf("expected error code NOT_FOUND, got %v", errObj["code"])
	}
}

// TestGetQuestion_NotAQuestion tests 404 when post exists but is not a question.
func TestGetQuestion_NotAQuestion(t *testing.T) {
	repo := NewMockQuestionsRepository()
	// Create a problem instead of a question
	post := createTestQuestion("post-123", "Test Problem")
	post.Type = models.PostTypeProblem
	repo.SetQuestion(&post)

	handler := NewQuestionsHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/questions/post-123", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "post-123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	handler.Get(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

// TestGetQuestion_IncludesAnswers tests that answers are included in response.
func TestGetQuestion_IncludesAnswers(t *testing.T) {
	repo := NewMockQuestionsRepository()
	question := createTestQuestion("question-123", "Test Question")
	repo.SetQuestion(&question)
	repo.SetAnswers([]models.AnswerWithAuthor{
		createTestAnswer("answer-1", "question-123"),
		createTestAnswer("answer-2", "question-123"),
	})

	handler := NewQuestionsHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/questions/question-123", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "question-123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	handler.Get(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	data := resp["data"].(map[string]interface{})
	answers := data["answers"].([]interface{})
	if len(answers) != 2 {
		t.Errorf("expected 2 answers, got %d", len(answers))
	}
}

// ============================================================================
// POST /v1/questions - Create Question Tests
// ============================================================================

// TestCreateQuestion_Success tests successful question creation.
func TestCreateQuestion_Success(t *testing.T) {
	repo := NewMockQuestionsRepository()
	handler := NewQuestionsHandler(repo)

	body := map[string]interface{}{
		"title":       "Test Question Title That Is Long Enough",
		"description": "This is a test description that needs to be at least fifty characters long to pass validation.",
		"tags":        []string{"go", "testing"},
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/questions", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req = addQuestionsAuthContext(req, "user-123", "user")
	w := httptest.NewRecorder()

	handler.Create(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d; body: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	data := resp["data"].(map[string]interface{})
	if data["id"] == nil {
		t.Error("expected question id in response")
	}

	// Verify type is set to question
	if repo.createdPost.Type != models.PostTypeQuestion {
		t.Errorf("expected type 'question', got '%s'", repo.createdPost.Type)
	}
}

// TestCreateQuestion_NoAuth tests 401 when not authenticated.
func TestCreateQuestion_NoAuth(t *testing.T) {
	repo := NewMockQuestionsRepository()
	handler := NewQuestionsHandler(repo)

	body := map[string]interface{}{
		"title":       "Test Question",
		"description": "Test description",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/questions", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	// No auth context
	w := httptest.NewRecorder()

	handler.Create(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

// TestCreateQuestion_TitleTooShort tests validation for short title.
func TestCreateQuestion_TitleTooShort(t *testing.T) {
	repo := NewMockQuestionsRepository()
	handler := NewQuestionsHandler(repo)

	body := map[string]interface{}{
		"title":       "Short", // Less than 10 characters
		"description": "This is a test description that needs to be at least fifty characters long to pass validation.",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/questions", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req = addQuestionsAuthContext(req, "user-123", "user")
	w := httptest.NewRecorder()

	handler.Create(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

// TestCreateQuestion_TooManyTags tests 400 for more than 10 tags.
func TestCreateQuestion_TooManyTags(t *testing.T) {
	repo := NewMockQuestionsRepository()
	handler := NewQuestionsHandler(repo)

	tags := make([]string, 11)
	for i := range tags {
		tags[i] = "tag"
	}

	body := map[string]interface{}{
		"title":       "Test Question Title That Is Long Enough",
		"description": "This is a test description that needs to be at least fifty characters long to pass validation.",
		"tags":        tags,
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/questions", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req = addQuestionsAuthContext(req, "user-123", "user")
	w := httptest.NewRecorder()

	handler.Create(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	errObj := resp["error"].(map[string]interface{})
	if errObj["code"] != "VALIDATION_ERROR" {
		t.Errorf("expected error code VALIDATION_ERROR, got %v", errObj["code"])
	}
	if errObj["message"] != "maximum 10 tags allowed" {
		t.Errorf("expected error message 'maximum 10 tags allowed', got %v", errObj["message"])
	}
}

// TestCreateQuestion_DescriptionTooShort tests validation for short description.
func TestCreateQuestion_DescriptionTooShort(t *testing.T) {
	repo := NewMockQuestionsRepository()
	handler := NewQuestionsHandler(repo)

	body := map[string]interface{}{
		"title":       "Test Question Title That Is Long Enough",
		"description": "Too short", // Less than 50 characters
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/questions", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req = addQuestionsAuthContext(req, "user-123", "user")
	w := httptest.NewRecorder()

	handler.Create(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

// ============================================================================
// POST /v1/questions/:id/answers - Create Answer with API Key Auth Tests
// ============================================================================

// Helper to add agent auth context to request (for API key authentication)
func addQuestionsAgentContext(r *http.Request, agentID string) *http.Request {
	agent := &models.Agent{
		ID:          agentID,
		DisplayName: "Test Agent",
	}
	ctx := auth.ContextWithAgent(r.Context(), agent)
	return r.WithContext(ctx)
}

// TestCreateAnswer_SuccessWithAPIKey tests successful answer creation with API key auth.
// Per SPEC.md Part 1.4: AI agents can answer questions just like humans.
// Per FIX-015: POST /v1/questions/:id/answers should work with API key authentication.
func TestCreateAnswer_SuccessWithAPIKey(t *testing.T) {
	repo := NewMockQuestionsRepository()
	question := createTestQuestion("question-123", "Test Question")
	repo.SetQuestion(&question)

	handler := NewQuestionsHandler(repo)

	body := map[string]interface{}{
		"content": "This is a test answer from an AI agent with sufficient content length.",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/questions/question-123/answers", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "question-123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	// Use agent auth instead of JWT
	req = addQuestionsAgentContext(req, "agent-claude")
	w := httptest.NewRecorder()

	handler.CreateAnswer(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d; body: %s", w.Code, w.Body.String())
	}

	// Verify answer was created with agent author type
	if repo.createdAnswer == nil {
		t.Fatal("expected answer to be created")
	}
	if repo.createdAnswer.AuthorType != models.AuthorTypeAgent {
		t.Errorf("expected author type 'agent', got '%s'", repo.createdAnswer.AuthorType)
	}
	if repo.createdAnswer.AuthorID != "agent-claude" {
		t.Errorf("expected author ID 'agent-claude', got '%s'", repo.createdAnswer.AuthorID)
	}
}

// ============================================================================
// FIX-023: Test that questions created via /v1/posts can receive answers
// ============================================================================

// MockPostsRepositoryForQuestions implements PostsRepositoryInterface for testing FIX-023.
// This mock simulates posts created via the main posts endpoint.
type MockPostsRepositoryForQuestions struct {
	posts map[string]*models.PostWithAuthor
}

func NewMockPostsRepositoryForQuestions() *MockPostsRepositoryForQuestions {
	return &MockPostsRepositoryForQuestions{
		posts: make(map[string]*models.PostWithAuthor),
	}
}

func (m *MockPostsRepositoryForQuestions) List(ctx context.Context, opts models.PostListOptions) ([]models.PostWithAuthor, int, error) {
	var result []models.PostWithAuthor
	for _, post := range m.posts {
		if opts.Type != "" && post.Type != opts.Type {
			continue
		}
		result = append(result, *post)
	}
	return result, len(result), nil
}

func (m *MockPostsRepositoryForQuestions) FindByID(ctx context.Context, id string) (*models.PostWithAuthor, error) {
	if post, exists := m.posts[id]; exists {
		return post, nil
	}
	return nil, ErrQuestionNotFound // Return not found error
}

func (m *MockPostsRepositoryForQuestions) Create(ctx context.Context, post *models.Post) (*models.Post, error) {
	postWithAuthor := &models.PostWithAuthor{
		Post: *post,
		Author: models.PostAuthor{
			Type: post.PostedByType,
			ID:   post.PostedByID,
		},
	}
	m.posts[post.ID] = postWithAuthor
	return post, nil
}

func (m *MockPostsRepositoryForQuestions) Update(ctx context.Context, post *models.Post) (*models.Post, error) {
	return post, nil
}

func (m *MockPostsRepositoryForQuestions) Delete(ctx context.Context, id string) error {
	delete(m.posts, id)
	return nil
}

func (m *MockPostsRepositoryForQuestions) Vote(ctx context.Context, postID, voterType, voterID, direction string) error {
	return nil
}

func (m *MockPostsRepositoryForQuestions) GetUserVote(ctx context.Context, postID, voterType, voterID string) (*string, error) {
	return nil, nil
}

func (m *MockPostsRepositoryForQuestions) AddPost(post *models.PostWithAuthor) {
	m.posts[post.ID] = post
}

// TestCreateAnswer_QuestionFromPostsRepo tests FIX-023: Creating an answer for a question
// that was created via POST /v1/posts (stored in postsRepo, not questionsRepo).
// This is the core bug: the question exists in the posts table but the handler looks in a
// separate questions repository.
func TestCreateAnswer_QuestionFromPostsRepo(t *testing.T) {
	// Create question repo (empty - question is NOT here)
	questionsRepo := NewMockQuestionsRepository()
	// Question is NOT set in questionsRepo - simulating the bug

	// Create posts repo with the question (this is where POST /v1/posts stores it)
	postsRepo := NewMockPostsRepositoryForQuestions()
	question := createTestQuestion("question-from-posts-123", "Question Created via /v1/posts")
	postsRepo.AddPost(&question)

	// Create handler and set posts repo
	handler := NewQuestionsHandler(questionsRepo)
	handler.SetPostsRepository(postsRepo)

	body := map[string]interface{}{
		"content": "This is an answer to a question created via POST /v1/posts endpoint.",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/questions/question-from-posts-123/answers", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "question-from-posts-123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = addQuestionsAuthContext(req, "user-123", "user")
	w := httptest.NewRecorder()

	handler.CreateAnswer(w, req)

	// FIX-023: This should succeed (return 201) because the question exists in postsRepo.
	// Currently this fails with 404 "question not found" because the handler only looks
	// in questionsRepo, which is empty.
	if w.Code != http.StatusCreated {
		t.Errorf("FIX-023: expected status 201, got %d; body: %s", w.Code, w.Body.String())
		t.Errorf("FIX-023: Question exists in postsRepo but handler returned 404 - handler must use postsRepo.FindByID when available")
	}
}

// TestGetQuestion_QuestionFromPostsRepo tests FIX-023: Getting a question that was created
// via POST /v1/posts (stored in postsRepo).
func TestGetQuestion_QuestionFromPostsRepo(t *testing.T) {
	// Create question repo (empty - question is NOT here)
	questionsRepo := NewMockQuestionsRepository()

	// Create posts repo with the question
	postsRepo := NewMockPostsRepositoryForQuestions()
	question := createTestQuestion("question-from-posts-456", "Question Created via /v1/posts")
	postsRepo.AddPost(&question)

	// Create handler and set posts repo
	handler := NewQuestionsHandler(questionsRepo)
	handler.SetPostsRepository(postsRepo)

	req := httptest.NewRequest(http.MethodGet, "/v1/questions/question-from-posts-456", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "question-from-posts-456")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	handler.Get(w, req)

	// FIX-023: This should succeed (return 200) because the question exists in postsRepo.
	if w.Code != http.StatusOK {
		t.Errorf("FIX-023: expected status 200, got %d; body: %s", w.Code, w.Body.String())
		t.Errorf("FIX-023: Question exists in postsRepo but handler returned 404 - handler must use postsRepo.FindByID when available")
	}
}

// TestListAnswers_QuestionFromPostsRepo tests FIX-023: Listing answers for a question
// that was created via POST /v1/posts.
func TestListAnswers_QuestionFromPostsRepo(t *testing.T) {
	// Create question repo (empty - question is NOT here)
	questionsRepo := NewMockQuestionsRepository()

	// Create posts repo with the question
	postsRepo := NewMockPostsRepositoryForQuestions()
	question := createTestQuestion("question-from-posts-789", "Question Created via /v1/posts")
	postsRepo.AddPost(&question)

	// Create handler and set posts repo
	handler := NewQuestionsHandler(questionsRepo)
	handler.SetPostsRepository(postsRepo)

	req := httptest.NewRequest(http.MethodGet, "/v1/questions/question-from-posts-789/answers", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "question-from-posts-789")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	handler.ListAnswers(w, req)

	// FIX-023: This should succeed (return 200) because the question exists in postsRepo.
	if w.Code != http.StatusOK {
		t.Errorf("FIX-023: expected status 200, got %d; body: %s", w.Code, w.Body.String())
		t.Errorf("FIX-023: Question exists in postsRepo but handler returned 404 - handler must use postsRepo.FindByID when available")
	}
}
