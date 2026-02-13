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

// MockProblemsRepository implements ProblemsRepositoryInterface for testing.
type MockProblemsRepository struct {
	posts           []models.PostWithAuthor
	post            *models.PostWithAuthor
	total           int
	err             error
	listOpts        models.PostListOptions
	approaches      []models.ApproachWithAuthor
	approach        *models.ApproachWithAuthor
	approachesErr   error
	createdPost     *models.Post
	createdApproach *models.Approach
	updatedApproach *models.Approach
	progressNote    *models.ProgressNote
}

func NewMockProblemsRepository() *MockProblemsRepository {
	return &MockProblemsRepository{
		posts:      []models.PostWithAuthor{},
		approaches: []models.ApproachWithAuthor{},
	}
}

func (m *MockProblemsRepository) ListProblems(ctx context.Context, opts models.PostListOptions) ([]models.PostWithAuthor, int, error) {
	m.listOpts = opts
	if m.err != nil {
		return nil, 0, m.err
	}
	return m.posts, m.total, nil
}

func (m *MockProblemsRepository) FindProblemByID(ctx context.Context, id string) (*models.PostWithAuthor, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.post == nil {
		return nil, ErrProblemNotFound
	}
	return m.post, nil
}

func (m *MockProblemsRepository) CreateProblem(ctx context.Context, post *models.Post) (*models.Post, error) {
	if m.err != nil {
		return nil, m.err
	}
	m.createdPost = post
	post.ID = "new-problem-id"
	post.CreatedAt = time.Now()
	post.UpdatedAt = time.Now()
	return post, nil
}

func (m *MockProblemsRepository) ListApproaches(ctx context.Context, problemID string, opts models.ApproachListOptions) ([]models.ApproachWithAuthor, int, error) {
	if m.approachesErr != nil {
		return nil, 0, m.approachesErr
	}
	return m.approaches, len(m.approaches), nil
}

func (m *MockProblemsRepository) CreateApproach(ctx context.Context, approach *models.Approach) (*models.Approach, error) {
	if m.err != nil {
		return nil, m.err
	}
	m.createdApproach = approach
	approach.ID = "new-approach-id"
	approach.CreatedAt = time.Now()
	approach.UpdatedAt = time.Now()
	return approach, nil
}

func (m *MockProblemsRepository) FindApproachByID(ctx context.Context, id string) (*models.ApproachWithAuthor, error) {
	if m.approachesErr != nil {
		return nil, m.approachesErr
	}
	if m.approach == nil {
		return nil, ErrApproachNotFound
	}
	return m.approach, nil
}

func (m *MockProblemsRepository) UpdateApproach(ctx context.Context, approach *models.Approach) (*models.Approach, error) {
	if m.err != nil {
		return nil, m.err
	}
	m.updatedApproach = approach
	approach.UpdatedAt = time.Now()
	return approach, nil
}

func (m *MockProblemsRepository) AddProgressNote(ctx context.Context, note *models.ProgressNote) (*models.ProgressNote, error) {
	if m.err != nil {
		return nil, m.err
	}
	m.progressNote = note
	note.ID = "new-note-id"
	note.CreatedAt = time.Now()
	return note, nil
}

func (m *MockProblemsRepository) GetProgressNotes(ctx context.Context, approachID string) ([]models.ProgressNote, error) {
	if m.err != nil {
		return nil, m.err
	}
	return []models.ProgressNote{}, nil
}

func (m *MockProblemsRepository) UpdateProblemStatus(ctx context.Context, problemID string, status models.PostStatus) error {
	if m.err != nil {
		return m.err
	}
	return nil
}

func (m *MockProblemsRepository) SetPosts(posts []models.PostWithAuthor, total int) {
	m.posts = posts
	m.total = total
}

func (m *MockProblemsRepository) SetPost(post *models.PostWithAuthor) {
	m.post = post
}

func (m *MockProblemsRepository) SetError(err error) {
	m.err = err
}

func (m *MockProblemsRepository) SetApproaches(approaches []models.ApproachWithAuthor) {
	m.approaches = approaches
}

func (m *MockProblemsRepository) SetApproach(approach *models.ApproachWithAuthor) {
	m.approach = approach
}

func (m *MockProblemsRepository) SetApproachesError(err error) {
	m.approachesErr = err
}

// Note: ErrProblemNotFound and ErrApproachNotFound are defined in errors.go

// createTestProblem creates a test problem with default values.
func createTestProblem(id, title string) models.PostWithAuthor {
	now := time.Now()
	return models.PostWithAuthor{
		Post: models.Post{
			ID:              id,
			Type:            models.PostTypeProblem,
			Title:           title,
			Description:     "Test problem description with more than fifty characters for validation",
			Tags:            []string{"test", "go"},
			PostedByType:    models.AuthorTypeHuman,
			PostedByID:      "user-123",
			Status:          models.PostStatusOpen,
			Upvotes:         10,
			Downvotes:       2,
			SuccessCriteria: []string{"Code works", "Tests pass"},
			Weight:          intPtr(3),
			CreatedAt:       now,
			UpdatedAt:       now,
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

// createTestApproach creates a test approach with default values.
func createTestApproach(id, problemID string) models.ApproachWithAuthor {
	now := time.Now()
	return models.ApproachWithAuthor{
		Approach: models.Approach{
			ID:         id,
			ProblemID:  problemID,
			AuthorType: models.AuthorTypeHuman,
			AuthorID:   "user-456",
			Angle:      "Test approach angle with sufficient length",
			Method:     "Test method",
			Status:     models.ApproachStatusStarting,
			CreatedAt:  now,
			UpdatedAt:  now,
		},
		Author: models.ApproachAuthor{
			Type:        models.AuthorTypeHuman,
			ID:          "user-456",
			DisplayName: "Test Contributor",
			AvatarURL:   "https://example.com/avatar2.png",
		},
	}
}

func intPtr(i int) *int {
	return &i
}

// Helper to add auth claims to request context
func addProblemsAuthContext(r *http.Request, userID, role string) *http.Request {
	claims := &auth.Claims{
		UserID: userID,
		Role:   role,
	}
	ctx := auth.ContextWithClaims(r.Context(), claims)
	return r.WithContext(ctx)
}

// ============================================================================
// GET /v1/problems - List Problems Tests
// ============================================================================

// TestListProblems_Success tests successful listing of problems.
func TestListProblems_Success(t *testing.T) {
	repo := NewMockProblemsRepository()
	repo.SetPosts([]models.PostWithAuthor{
		createTestProblem("problem-1", "First Problem"),
		createTestProblem("problem-2", "Second Problem"),
	}, 2)

	handler := NewProblemsHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/problems", nil)
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
		t.Errorf("expected 2 problems, got %d", len(data))
	}
}

// TestListProblems_FiltersType tests that type is automatically set to problem.
func TestListProblems_FiltersType(t *testing.T) {
	repo := NewMockProblemsRepository()
	repo.SetPosts([]models.PostWithAuthor{}, 0)

	handler := NewProblemsHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/problems", nil)
	w := httptest.NewRecorder()

	handler.List(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	// Verify the filter was set to problem
	if repo.listOpts.Type != models.PostTypeProblem {
		t.Errorf("expected type filter 'problem', got '%s'", repo.listOpts.Type)
	}
}

// TestListProblems_FilterByStatus tests filtering by status.
func TestListProblems_FilterByStatus(t *testing.T) {
	repo := NewMockProblemsRepository()
	repo.SetPosts([]models.PostWithAuthor{}, 0)

	handler := NewProblemsHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/problems?status=solved", nil)
	w := httptest.NewRecorder()

	handler.List(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	if repo.listOpts.Status != models.PostStatusSolved {
		t.Errorf("expected status filter 'solved', got '%s'", repo.listOpts.Status)
	}
}

// TestListProblems_Pagination tests pagination parameters.
func TestListProblems_Pagination(t *testing.T) {
	repo := NewMockProblemsRepository()
	repo.SetPosts([]models.PostWithAuthor{}, 100)

	handler := NewProblemsHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/problems?page=2&per_page=10", nil)
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

// TestListProblems_SortByVotes tests sorting by vote score.
func TestListProblems_SortByVotes(t *testing.T) {
	repo := NewMockProblemsRepository()
	repo.SetPosts([]models.PostWithAuthor{}, 0)

	handler := NewProblemsHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/problems?sort=votes", nil)
	w := httptest.NewRecorder()

	handler.List(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	if repo.listOpts.Sort != "votes" {
		t.Errorf("expected sort 'votes', got '%s'", repo.listOpts.Sort)
	}
}

// TestListProblems_SortByApproaches tests sorting by approach count.
func TestListProblems_SortByApproaches(t *testing.T) {
	repo := NewMockProblemsRepository()
	repo.SetPosts([]models.PostWithAuthor{}, 0)

	handler := NewProblemsHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/problems?sort=approaches", nil)
	w := httptest.NewRecorder()

	handler.List(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	if repo.listOpts.Sort != "approaches" {
		t.Errorf("expected sort 'approaches', got '%s'", repo.listOpts.Sort)
	}
}

// TestListProblems_DefaultSortNewest tests that no sort param defaults to newest.
func TestListProblems_DefaultSortNewest(t *testing.T) {
	repo := NewMockProblemsRepository()
	repo.SetPosts([]models.PostWithAuthor{}, 0)

	handler := NewProblemsHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/problems", nil)
	w := httptest.NewRecorder()

	handler.List(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	// Default sort should be empty (repo interprets as newest)
	if repo.listOpts.Sort != "" {
		t.Errorf("expected empty sort (default newest), got '%s'", repo.listOpts.Sort)
	}
}

// TestListProblems_InvalidSortIgnored tests that invalid sort values are ignored.
func TestListProblems_InvalidSortIgnored(t *testing.T) {
	repo := NewMockProblemsRepository()
	repo.SetPosts([]models.PostWithAuthor{}, 0)

	handler := NewProblemsHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/problems?sort=invalid", nil)
	w := httptest.NewRecorder()

	handler.List(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	// Invalid sort should be ignored (empty = default newest)
	if repo.listOpts.Sort != "" {
		t.Errorf("expected empty sort for invalid value, got '%s'", repo.listOpts.Sort)
	}
}

// ============================================================================
// GET /v1/problems/:id - Get Single Problem Tests
// ============================================================================

// TestGetProblem_Success tests successful retrieval of a problem.
func TestGetProblem_Success(t *testing.T) {
	repo := NewMockProblemsRepository()
	problem := createTestProblem("problem-123", "Test Problem")
	repo.SetPost(&problem)

	handler := NewProblemsHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/problems/problem-123", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "problem-123")
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
	if data["id"] != "problem-123" {
		t.Errorf("expected problem id 'problem-123', got %v", data["id"])
	}

	// Verify success_criteria is included
	if data["success_criteria"] == nil {
		t.Error("expected success_criteria in response")
	}
}

// TestGetProblem_NotFound tests 404 for non-existent problem.
func TestGetProblem_NotFound(t *testing.T) {
	repo := NewMockProblemsRepository()
	repo.SetPost(nil)

	handler := NewProblemsHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/problems/nonexistent", nil)
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

// TestGetProblem_NotAProblem tests 404 when post exists but is not a problem.
func TestGetProblem_NotAProblem(t *testing.T) {
	repo := NewMockProblemsRepository()
	// Create a question instead of a problem
	post := createTestProblem("post-123", "Test Question")
	post.Type = models.PostTypeQuestion
	repo.SetPost(&post)

	handler := NewProblemsHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/problems/post-123", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "post-123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	handler.Get(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

// ============================================================================
// POST /v1/problems - Create Problem Tests
// ============================================================================

// TestCreateProblem_Success tests successful problem creation.
func TestCreateProblem_Success(t *testing.T) {
	repo := NewMockProblemsRepository()
	handler := NewProblemsHandler(repo)

	body := map[string]interface{}{
		"title":            "Test Problem Title That Is Long Enough",
		"description":      "This is a test description that needs to be at least fifty characters long to pass validation.",
		"tags":             []string{"go", "testing"},
		"success_criteria": []string{"Code compiles", "Tests pass"},
		"weight":           3,
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/problems", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req = addProblemsAuthContext(req, "user-123", "user")
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
		t.Error("expected problem id in response")
	}

	// Verify type is set to problem
	if repo.createdPost.Type != models.PostTypeProblem {
		t.Errorf("expected type 'problem', got '%s'", repo.createdPost.Type)
	}

	// Verify success_criteria is set
	if len(repo.createdPost.SuccessCriteria) != 2 {
		t.Errorf("expected 2 success criteria, got %d", len(repo.createdPost.SuccessCriteria))
	}

	// Verify weight is set
	if repo.createdPost.Weight == nil || *repo.createdPost.Weight != 3 {
		t.Error("expected weight 3")
	}
}

// TestCreateProblem_NoAuth tests 401 when not authenticated.
func TestCreateProblem_NoAuth(t *testing.T) {
	repo := NewMockProblemsRepository()
	handler := NewProblemsHandler(repo)

	body := map[string]interface{}{
		"title":       "Test Problem",
		"description": "Test description",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/problems", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	// No auth context
	w := httptest.NewRecorder()

	handler.Create(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

// TestCreateProblem_InvalidWeight tests 400 for weight outside 1-5.
func TestCreateProblem_InvalidWeight(t *testing.T) {
	repo := NewMockProblemsRepository()
	handler := NewProblemsHandler(repo)

	body := map[string]interface{}{
		"title":       "Test Problem Title That Is Long Enough",
		"description": "This is a test description that needs to be at least fifty characters long to pass validation.",
		"weight":      10, // Invalid - must be 1-5
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/problems", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req = addProblemsAuthContext(req, "user-123", "user")
	w := httptest.NewRecorder()

	handler.Create(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

// TestCreateProblem_TooManyTags tests 400 for more than 10 tags.
func TestCreateProblem_TooManyTags(t *testing.T) {
	repo := NewMockProblemsRepository()
	handler := NewProblemsHandler(repo)

	tags := make([]string, 11)
	for i := range tags {
		tags[i] = "tag"
	}

	body := map[string]interface{}{
		"title":       "Test Problem Title That Is Long Enough",
		"description": "This is a test description that needs to be at least fifty characters long to pass validation.",
		"tags":        tags,
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/problems", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req = addProblemsAuthContext(req, "user-123", "user")
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

// TestCreateProblem_TooManySuccessCriteria tests 400 for > 10 criteria.
func TestCreateProblem_TooManySuccessCriteria(t *testing.T) {
	repo := NewMockProblemsRepository()
	handler := NewProblemsHandler(repo)

	criteria := make([]string, 11)
	for i := range criteria {
		criteria[i] = "Criteria"
	}

	body := map[string]interface{}{
		"title":            "Test Problem Title That Is Long Enough",
		"description":      "This is a test description that needs to be at least fifty characters long to pass validation.",
		"success_criteria": criteria,
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/problems", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req = addProblemsAuthContext(req, "user-123", "user")
	w := httptest.NewRecorder()

	handler.Create(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}
