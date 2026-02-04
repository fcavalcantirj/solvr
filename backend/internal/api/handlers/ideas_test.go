// Package handlers contains HTTP request handlers for the Solvr API.
package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/auth"
	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/go-chi/chi/v5"
)

// Error specific to ideas testing - ErrResponseNotFound is not in errors.go
var ErrResponseNotFound = errors.New("response not found")

// MockIdeasRepository implements IdeasRepositoryInterface for testing.
type MockIdeasRepository struct {
	ideas           []models.PostWithAuthor
	idea            *models.PostWithAuthor
	total           int
	err             error
	listOpts        models.PostListOptions
	responses       []models.ResponseWithAuthor
	response        *models.ResponseWithAuthor
	responsesErr    error
	createdPost     *models.Post
	createdResponse *models.Response
	updatedPost     *models.Post
	evolvedPostID   string
	evolvedIntoID   string
}

func NewMockIdeasRepository() *MockIdeasRepository {
	return &MockIdeasRepository{
		ideas:     []models.PostWithAuthor{},
		responses: []models.ResponseWithAuthor{},
	}
}

func (m *MockIdeasRepository) ListIdeas(ctx context.Context, opts models.PostListOptions) ([]models.PostWithAuthor, int, error) {
	m.listOpts = opts
	if m.err != nil {
		return nil, 0, m.err
	}
	return m.ideas, m.total, nil
}

func (m *MockIdeasRepository) FindIdeaByID(ctx context.Context, id string) (*models.PostWithAuthor, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.idea == nil {
		return nil, ErrIdeaNotFound
	}
	return m.idea, nil
}

func (m *MockIdeasRepository) CreateIdea(ctx context.Context, post *models.Post) (*models.Post, error) {
	if m.err != nil {
		return nil, m.err
	}
	m.createdPost = post
	post.ID = "new-idea-id"
	post.CreatedAt = time.Now()
	post.UpdatedAt = time.Now()
	return post, nil
}

func (m *MockIdeasRepository) ListResponses(ctx context.Context, ideaID string, opts models.ResponseListOptions) ([]models.ResponseWithAuthor, int, error) {
	if m.responsesErr != nil {
		return nil, 0, m.responsesErr
	}
	return m.responses, len(m.responses), nil
}

func (m *MockIdeasRepository) CreateResponse(ctx context.Context, response *models.Response) (*models.Response, error) {
	if m.err != nil {
		return nil, m.err
	}
	m.createdResponse = response
	response.ID = "new-response-id"
	response.CreatedAt = time.Now()
	return response, nil
}

func (m *MockIdeasRepository) AddEvolvedInto(ctx context.Context, ideaID, evolvedPostID string) error {
	if m.err != nil {
		return m.err
	}
	m.evolvedPostID = ideaID
	m.evolvedIntoID = evolvedPostID
	return nil
}

func (m *MockIdeasRepository) FindPostByID(ctx context.Context, id string) (*models.PostWithAuthor, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.idea != nil && m.idea.ID == id {
		return m.idea, nil
	}
	if id == "evolved-post-id" {
		now := time.Now()
		return &models.PostWithAuthor{
			Post: models.Post{
				ID:           id,
				Type:         models.PostTypeProblem,
				Title:        "Evolved Post",
				Description:  "A problem that evolved from an idea",
				PostedByType: models.AuthorTypeHuman,
				PostedByID:   "user-123",
				Status:       models.PostStatusOpen,
				CreatedAt:    now,
				UpdatedAt:    now,
			},
			Author: models.PostAuthor{
				Type:        models.AuthorTypeHuman,
				ID:          "user-123",
				DisplayName: "Test User",
			},
		}, nil
	}
	return nil, ErrIdeaNotFound
}

func (m *MockIdeasRepository) SetIdeas(ideas []models.PostWithAuthor, total int) {
	m.ideas = ideas
	m.total = total
}

func (m *MockIdeasRepository) SetIdea(idea *models.PostWithAuthor) {
	m.idea = idea
}

func (m *MockIdeasRepository) SetError(err error) {
	m.err = err
}

func (m *MockIdeasRepository) SetResponses(responses []models.ResponseWithAuthor) {
	m.responses = responses
}

func (m *MockIdeasRepository) SetResponse(response *models.ResponseWithAuthor) {
	m.response = response
}

func (m *MockIdeasRepository) SetResponsesError(err error) {
	m.responsesErr = err
}

// createTestIdea creates a test idea with default values.
func createTestIdea(id, title string) models.PostWithAuthor {
	now := time.Now()
	return models.PostWithAuthor{
		Post: models.Post{
			ID:           id,
			Type:         models.PostTypeIdea,
			Title:        title,
			Description:  "Test idea description with more than fifty characters for validation",
			Tags:         []string{"test", "idea"},
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

// createTestResponse creates a test response with default values.
func createTestResponse(id, ideaID string) models.ResponseWithAuthor {
	now := time.Now()
	return models.ResponseWithAuthor{
		Response: models.Response{
			ID:           id,
			IdeaID:       ideaID,
			AuthorType:   models.AuthorTypeHuman,
			AuthorID:     "user-456",
			Content:      "This is a test response with sufficient content length",
			ResponseType: models.ResponseTypeBuild,
			Upvotes:      5,
			Downvotes:    1,
			CreatedAt:    now,
		},
		Author: models.ResponseAuthor{
			Type:        models.AuthorTypeHuman,
			ID:          "user-456",
			DisplayName: "Test Contributor",
			AvatarURL:   "https://example.com/avatar2.png",
		},
		VoteScore: 4,
	}
}

// addIdeasAuthContext adds auth claims to request context.
func addIdeasAuthContext(r *http.Request, userID, role string) *http.Request {
	claims := &auth.Claims{UserID: userID, Role: role}
	ctx := auth.ContextWithClaims(r.Context(), claims)
	return r.WithContext(ctx)
}

// TestListIdeas_Success tests successful listing of ideas.
func TestListIdeas_Success(t *testing.T) {
	repo := NewMockIdeasRepository()
	repo.SetIdeas([]models.PostWithAuthor{
		createTestIdea("idea-1", "First Idea"),
		createTestIdea("idea-2", "Second Idea"),
	}, 2)

	handler := NewIdeasHandler(repo)
	req := httptest.NewRequest(http.MethodGet, "/v1/ideas", nil)
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
		t.Errorf("expected 2 ideas, got %d", len(data))
	}
}

// TestListIdeas_FiltersType tests that type is automatically set to idea.
func TestListIdeas_FiltersType(t *testing.T) {
	repo := NewMockIdeasRepository()
	repo.SetIdeas([]models.PostWithAuthor{}, 0)

	handler := NewIdeasHandler(repo)
	req := httptest.NewRequest(http.MethodGet, "/v1/ideas", nil)
	w := httptest.NewRecorder()

	handler.List(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
	if repo.listOpts.Type != models.PostTypeIdea {
		t.Errorf("expected type filter 'idea', got '%s'", repo.listOpts.Type)
	}
}

// TestListIdeas_FilterByStatus tests filtering by status.
func TestListIdeas_FilterByStatus(t *testing.T) {
	repo := NewMockIdeasRepository()
	repo.SetIdeas([]models.PostWithAuthor{}, 0)

	handler := NewIdeasHandler(repo)
	req := httptest.NewRequest(http.MethodGet, "/v1/ideas?status=active", nil)
	w := httptest.NewRecorder()

	handler.List(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
	if repo.listOpts.Status != models.PostStatusActive {
		t.Errorf("expected status filter 'active', got '%s'", repo.listOpts.Status)
	}
}

// TestListIdeas_Pagination tests pagination parameters.
func TestListIdeas_Pagination(t *testing.T) {
	repo := NewMockIdeasRepository()
	repo.SetIdeas([]models.PostWithAuthor{}, 100)

	handler := NewIdeasHandler(repo)
	req := httptest.NewRequest(http.MethodGet, "/v1/ideas?page=2&per_page=10", nil)
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

// TestListIdeas_HasMore tests has_more pagination flag.
func TestListIdeas_HasMore(t *testing.T) {
	repo := NewMockIdeasRepository()
	repo.SetIdeas([]models.PostWithAuthor{createTestIdea("i-1", "Idea 1")}, 50)

	handler := NewIdeasHandler(repo)
	req := httptest.NewRequest(http.MethodGet, "/v1/ideas", nil)
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

// TestGetIdea_Success tests successful retrieval of an idea.
func TestGetIdea_Success(t *testing.T) {
	repo := NewMockIdeasRepository()
	idea := createTestIdea("idea-123", "Test Idea")
	repo.SetIdea(&idea)

	handler := NewIdeasHandler(repo)
	req := httptest.NewRequest(http.MethodGet, "/v1/ideas/idea-123", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "idea-123")
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
	if data["id"] != "idea-123" {
		t.Errorf("expected idea id 'idea-123', got %v", data["id"])
	}
}

// TestGetIdea_NotFound tests 404 for non-existent idea.
func TestGetIdea_NotFound(t *testing.T) {
	repo := NewMockIdeasRepository()
	repo.SetIdea(nil)

	handler := NewIdeasHandler(repo)
	req := httptest.NewRequest(http.MethodGet, "/v1/ideas/nonexistent", nil)
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

// TestGetIdea_NotAnIdea tests 404 when post exists but is not an idea.
func TestGetIdea_NotAnIdea(t *testing.T) {
	repo := NewMockIdeasRepository()
	post := createTestIdea("post-123", "Test Problem")
	post.Type = models.PostTypeProblem
	repo.SetIdea(&post)

	handler := NewIdeasHandler(repo)
	req := httptest.NewRequest(http.MethodGet, "/v1/ideas/post-123", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "post-123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	handler.Get(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

// TestGetIdea_IncludesResponses tests that responses are included.
func TestGetIdea_IncludesResponses(t *testing.T) {
	repo := NewMockIdeasRepository()
	idea := createTestIdea("idea-123", "Test Idea")
	repo.SetIdea(&idea)
	repo.SetResponses([]models.ResponseWithAuthor{
		createTestResponse("response-1", "idea-123"),
		createTestResponse("response-2", "idea-123"),
	})

	handler := NewIdeasHandler(repo)
	req := httptest.NewRequest(http.MethodGet, "/v1/ideas/idea-123", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "idea-123")
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
	responses := data["responses"].([]interface{})
	if len(responses) != 2 {
		t.Errorf("expected 2 responses, got %d", len(responses))
	}
}

// TestCreateIdea_Success tests successful idea creation.
func TestCreateIdea_Success(t *testing.T) {
	repo := NewMockIdeasRepository()
	handler := NewIdeasHandler(repo)

	body := map[string]interface{}{
		"title":       "Test Idea Title That Is Long Enough",
		"description": "This is a test description that needs to be at least fifty characters long.",
		"tags":        []string{"brainstorm", "testing"},
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/ideas", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req = addIdeasAuthContext(req, "user-123", "user")
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
		t.Error("expected idea id in response")
	}
	if repo.createdPost.Type != models.PostTypeIdea {
		t.Errorf("expected type 'idea', got '%s'", repo.createdPost.Type)
	}
}

// TestCreateIdea_NoAuth tests 401 when not authenticated.
func TestCreateIdea_NoAuth(t *testing.T) {
	repo := NewMockIdeasRepository()
	handler := NewIdeasHandler(repo)

	body := map[string]interface{}{
		"title":       "Test Idea",
		"description": "Test description",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/ideas", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.Create(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

// TestCreateIdea_TitleTooShort tests validation for short title.
func TestCreateIdea_TitleTooShort(t *testing.T) {
	repo := NewMockIdeasRepository()
	handler := NewIdeasHandler(repo)

	body := map[string]interface{}{
		"title":       "Short",
		"description": "This is a test description that needs to be at least fifty characters long.",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/ideas", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req = addIdeasAuthContext(req, "user-123", "user")
	w := httptest.NewRecorder()

	handler.Create(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

// TestCreateIdea_DescriptionTooShort tests validation for short description.
func TestCreateIdea_DescriptionTooShort(t *testing.T) {
	repo := NewMockIdeasRepository()
	handler := NewIdeasHandler(repo)

	body := map[string]interface{}{
		"title":       "Test Idea Title That Is Long Enough",
		"description": "Too short",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/ideas", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req = addIdeasAuthContext(req, "user-123", "user")
	w := httptest.NewRecorder()

	handler.Create(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

// TestListResponses_Success tests successful listing of responses for an idea.
// Per FIX-024: GET /v1/ideas/:id/responses should return list of responses.
func TestListResponses_Success(t *testing.T) {
	repo := NewMockIdeasRepository()
	idea := createTestIdea("idea-123", "Test Idea")
	repo.SetIdea(&idea)
	repo.SetResponses([]models.ResponseWithAuthor{
		createTestResponse("response-1", "idea-123"),
		createTestResponse("response-2", "idea-123"),
	})

	handler := NewIdeasHandler(repo)
	req := httptest.NewRequest(http.MethodGet, "/v1/ideas/idea-123/responses", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "idea-123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	handler.ListResponses(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d; body: %s", w.Code, w.Body.String())
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
		t.Errorf("expected 2 responses, got %d", len(data))
	}

	meta, ok := resp["meta"].(map[string]interface{})
	if !ok {
		t.Fatal("expected meta object in response")
	}
	if meta["total"].(float64) != 2 {
		t.Errorf("expected total=2, got %v", meta["total"])
	}
}

// TestListResponses_IdeaNotFound tests 404 when idea doesn't exist.
func TestListResponses_IdeaNotFound(t *testing.T) {
	repo := NewMockIdeasRepository()
	repo.SetIdea(nil) // No idea set

	handler := NewIdeasHandler(repo)
	req := httptest.NewRequest(http.MethodGet, "/v1/ideas/nonexistent/responses", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "nonexistent")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	handler.ListResponses(w, req)

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

// TestListResponses_Pagination tests pagination parameters.
func TestListResponses_Pagination(t *testing.T) {
	repo := NewMockIdeasRepository()
	idea := createTestIdea("idea-123", "Test Idea")
	repo.SetIdea(&idea)
	repo.SetResponses([]models.ResponseWithAuthor{
		createTestResponse("response-1", "idea-123"),
	})

	handler := NewIdeasHandler(repo)
	req := httptest.NewRequest(http.MethodGet, "/v1/ideas/idea-123/responses?page=2&per_page=10", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "idea-123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	handler.ListResponses(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	meta := resp["meta"].(map[string]interface{})
	if meta["page"].(float64) != 2 {
		t.Errorf("expected page=2, got %v", meta["page"])
	}
	if meta["per_page"].(float64) != 10 {
		t.Errorf("expected per_page=10, got %v", meta["per_page"])
	}
}
