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

// MockCommentsRepository is a mock implementation of CommentsRepositoryInterface.
type MockCommentsRepository struct {
	comments       []models.CommentWithAuthor
	createErr      error
	listErr        error
	deleteErr      error
	findByIDResult *models.CommentWithAuthor
	findByIDErr    error
	targetExists   bool
	targetExistsErr error
}

func (m *MockCommentsRepository) List(ctx context.Context, opts models.CommentListOptions) ([]models.CommentWithAuthor, int, error) {
	if m.listErr != nil {
		return nil, 0, m.listErr
	}
	// Filter by target
	var filtered []models.CommentWithAuthor
	for _, c := range m.comments {
		if c.TargetType == opts.TargetType && c.TargetID == opts.TargetID && c.DeletedAt == nil {
			filtered = append(filtered, c)
		}
	}
	return filtered, len(filtered), nil
}

func (m *MockCommentsRepository) Create(ctx context.Context, comment *models.Comment) (*models.Comment, error) {
	if m.createErr != nil {
		return nil, m.createErr
	}
	comment.ID = "comment-123"
	comment.CreatedAt = time.Now()
	return comment, nil
}

func (m *MockCommentsRepository) FindByID(ctx context.Context, id string) (*models.CommentWithAuthor, error) {
	if m.findByIDErr != nil {
		return nil, m.findByIDErr
	}
	if m.findByIDResult != nil {
		return m.findByIDResult, nil
	}
	return nil, ErrCommentNotFound
}

func (m *MockCommentsRepository) Delete(ctx context.Context, id string) error {
	return m.deleteErr
}

func (m *MockCommentsRepository) TargetExists(ctx context.Context, targetType models.CommentTargetType, targetID string) (bool, error) {
	if m.targetExistsErr != nil {
		return false, m.targetExistsErr
	}
	return m.targetExists, nil
}

// Helper to add JWT claims to context for comments tests.
func addCommentsAuthContext(r *http.Request, userID, role string) *http.Request {
	claims := &auth.Claims{
		UserID: userID,
		Email:  "test@example.com",
		Role:   role,
	}
	ctx := auth.ContextWithClaims(r.Context(), claims)
	return r.WithContext(ctx)
}

// Test List Comments

func TestListComments_Success(t *testing.T) {
	now := time.Now()
	mockRepo := &MockCommentsRepository{
		comments: []models.CommentWithAuthor{
			{
				Comment: models.Comment{
					ID:         "comment-1",
					TargetType: models.CommentTargetApproach,
					TargetID:   "approach-123",
					AuthorType: models.AuthorTypeHuman,
					AuthorID:   "user-1",
					Content:    "Great approach!",
					CreatedAt:  now,
				},
				Author: models.CommentAuthor{
					ID:          "user-1",
					Type:        models.AuthorTypeHuman,
					DisplayName: "Test User",
				},
			},
			{
				Comment: models.Comment{
					ID:         "comment-2",
					TargetType: models.CommentTargetApproach,
					TargetID:   "approach-123",
					AuthorType: models.AuthorTypeAgent,
					AuthorID:   "agent-1",
					Content:    "I agree with this approach.",
					CreatedAt:  now.Add(time.Minute),
				},
				Author: models.CommentAuthor{
					ID:          "agent-1",
					Type:        models.AuthorTypeAgent,
					DisplayName: "Claude",
				},
			},
		},
		targetExists: true,
	}

	handler := NewCommentsHandler(mockRepo)

	// Create request with chi context
	req := httptest.NewRequest(http.MethodGet, "/v1/approaches/approach-123/comments", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("target_type", "approach")
	rctx.URLParams.Add("id", "approach-123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	handler.List(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var response struct {
		Data []models.CommentWithAuthor `json:"data"`
		Meta struct {
			Total   int `json:"total"`
			Page    int `json:"page"`
			PerPage int `json:"per_page"`
		} `json:"meta"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(response.Data) != 2 {
		t.Errorf("expected 2 comments, got %d", len(response.Data))
	}
	if response.Meta.Total != 2 {
		t.Errorf("expected total 2, got %d", response.Meta.Total)
	}
}

func TestListComments_EmptyResult(t *testing.T) {
	mockRepo := &MockCommentsRepository{
		comments:     []models.CommentWithAuthor{},
		targetExists: true,
	}

	handler := NewCommentsHandler(mockRepo)

	req := httptest.NewRequest(http.MethodGet, "/v1/approaches/approach-123/comments", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("target_type", "approach")
	rctx.URLParams.Add("id", "approach-123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	handler.List(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var response struct {
		Data []models.CommentWithAuthor `json:"data"`
		Meta struct {
			Total int `json:"total"`
		} `json:"meta"`
	}
	json.NewDecoder(rec.Body).Decode(&response)

	if response.Data == nil || len(response.Data) != 0 {
		t.Errorf("expected empty data array, got %v", response.Data)
	}
}

func TestListComments_InvalidTargetType(t *testing.T) {
	mockRepo := &MockCommentsRepository{}
	handler := NewCommentsHandler(mockRepo)

	req := httptest.NewRequest(http.MethodGet, "/v1/invalid/approach-123/comments", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("target_type", "invalid")
	rctx.URLParams.Add("id", "approach-123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	handler.List(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rec.Code)
	}
}

// Test Create Comment

func TestCreateComment_Success(t *testing.T) {
	mockRepo := &MockCommentsRepository{
		targetExists: true,
	}

	handler := NewCommentsHandler(mockRepo)

	body := `{"content": "This is a helpful comment on this approach."}`
	req := httptest.NewRequest(http.MethodPost, "/v1/approaches/approach-123/comments", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req = addCommentsAuthContext(req, "user-123", "user")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("target_type", "approach")
	rctx.URLParams.Add("id", "approach-123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	handler.Create(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d; body: %s", rec.Code, rec.Body.String())
	}

	var response struct {
		Data models.Comment `json:"data"`
	}
	json.NewDecoder(rec.Body).Decode(&response)

	if response.Data.ID == "" {
		t.Error("expected comment ID to be set")
	}
	if response.Data.Content != "This is a helpful comment on this approach." {
		t.Errorf("expected content to match, got: %s", response.Data.Content)
	}
}

func TestCreateComment_NoAuth(t *testing.T) {
	mockRepo := &MockCommentsRepository{}
	handler := NewCommentsHandler(mockRepo)

	body := `{"content": "Hello world"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/approaches/approach-123/comments", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("target_type", "approach")
	rctx.URLParams.Add("id", "approach-123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	handler.Create(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rec.Code)
	}
}

func TestCreateComment_InvalidJSON(t *testing.T) {
	mockRepo := &MockCommentsRepository{}
	handler := NewCommentsHandler(mockRepo)

	req := httptest.NewRequest(http.MethodPost, "/v1/approaches/approach-123/comments", bytes.NewBufferString("not json"))
	req.Header.Set("Content-Type", "application/json")
	req = addCommentsAuthContext(req, "user-123", "user")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("target_type", "approach")
	rctx.URLParams.Add("id", "approach-123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	handler.Create(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rec.Code)
	}
}

func TestCreateComment_EmptyContent(t *testing.T) {
	mockRepo := &MockCommentsRepository{
		targetExists: true,
	}
	handler := NewCommentsHandler(mockRepo)

	body := `{"content": ""}`
	req := httptest.NewRequest(http.MethodPost, "/v1/approaches/approach-123/comments", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req = addCommentsAuthContext(req, "user-123", "user")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("target_type", "approach")
	rctx.URLParams.Add("id", "approach-123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	handler.Create(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rec.Code)
	}

	var response struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	json.NewDecoder(rec.Body).Decode(&response)

	if response.Error.Code != "VALIDATION_ERROR" {
		t.Errorf("expected error code VALIDATION_ERROR, got %s", response.Error.Code)
	}
}

func TestCreateComment_ContentTooLong(t *testing.T) {
	mockRepo := &MockCommentsRepository{
		targetExists: true,
	}
	handler := NewCommentsHandler(mockRepo)

	// Create content with more than 2000 characters
	longContent := make([]byte, 2001)
	for i := range longContent {
		longContent[i] = 'a'
	}

	body := `{"content": "` + string(longContent) + `"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/approaches/approach-123/comments", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req = addCommentsAuthContext(req, "user-123", "user")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("target_type", "approach")
	rctx.URLParams.Add("id", "approach-123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	handler.Create(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rec.Code)
	}
}

func TestCreateComment_TargetNotFound(t *testing.T) {
	mockRepo := &MockCommentsRepository{
		targetExists: false,
	}
	handler := NewCommentsHandler(mockRepo)

	body := `{"content": "This is a comment"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/approaches/nonexistent/comments", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req = addCommentsAuthContext(req, "user-123", "user")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("target_type", "approach")
	rctx.URLParams.Add("id", "nonexistent")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	handler.Create(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", rec.Code)
	}
}

func TestCreateComment_InvalidTargetType(t *testing.T) {
	mockRepo := &MockCommentsRepository{}
	handler := NewCommentsHandler(mockRepo)

	body := `{"content": "Hello"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/invalid/123/comments", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req = addCommentsAuthContext(req, "user-123", "user")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("target_type", "invalid")
	rctx.URLParams.Add("id", "123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	handler.Create(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rec.Code)
	}
}

// Test Delete Comment

func TestDeleteComment_OwnerCanDelete(t *testing.T) {
	now := time.Now()
	mockRepo := &MockCommentsRepository{
		findByIDResult: &models.CommentWithAuthor{
			Comment: models.Comment{
				ID:         "comment-123",
				TargetType: models.CommentTargetApproach,
				TargetID:   "approach-123",
				AuthorType: models.AuthorTypeHuman,
				AuthorID:   "user-123",
				Content:    "My comment",
				CreatedAt:  now,
			},
			Author: models.CommentAuthor{
				ID:          "user-123",
				Type:        models.AuthorTypeHuman,
				DisplayName: "Test User",
			},
		},
	}

	handler := NewCommentsHandler(mockRepo)

	req := httptest.NewRequest(http.MethodDelete, "/v1/comments/comment-123", nil)
	req = addCommentsAuthContext(req, "user-123", "user")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "comment-123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	handler.Delete(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("expected status 204, got %d; body: %s", rec.Code, rec.Body.String())
	}
}

func TestDeleteComment_AdminCanDelete(t *testing.T) {
	now := time.Now()
	mockRepo := &MockCommentsRepository{
		findByIDResult: &models.CommentWithAuthor{
			Comment: models.Comment{
				ID:         "comment-123",
				TargetType: models.CommentTargetApproach,
				TargetID:   "approach-123",
				AuthorType: models.AuthorTypeHuman,
				AuthorID:   "other-user",
				Content:    "Someone else's comment",
				CreatedAt:  now,
			},
			Author: models.CommentAuthor{
				ID:          "other-user",
				Type:        models.AuthorTypeHuman,
				DisplayName: "Other User",
			},
		},
	}

	handler := NewCommentsHandler(mockRepo)

	req := httptest.NewRequest(http.MethodDelete, "/v1/comments/comment-123", nil)
	req = addCommentsAuthContext(req, "admin-user", "admin")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "comment-123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	handler.Delete(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("expected status 204, got %d", rec.Code)
	}
}

func TestDeleteComment_OthersForbidden(t *testing.T) {
	now := time.Now()
	mockRepo := &MockCommentsRepository{
		findByIDResult: &models.CommentWithAuthor{
			Comment: models.Comment{
				ID:         "comment-123",
				TargetType: models.CommentTargetApproach,
				TargetID:   "approach-123",
				AuthorType: models.AuthorTypeHuman,
				AuthorID:   "other-user",
				Content:    "Someone else's comment",
				CreatedAt:  now,
			},
			Author: models.CommentAuthor{
				ID:          "other-user",
				Type:        models.AuthorTypeHuman,
				DisplayName: "Other User",
			},
		},
	}

	handler := NewCommentsHandler(mockRepo)

	req := httptest.NewRequest(http.MethodDelete, "/v1/comments/comment-123", nil)
	req = addCommentsAuthContext(req, "different-user", "user")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "comment-123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	handler.Delete(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected status 403, got %d", rec.Code)
	}
}

func TestDeleteComment_NoAuth(t *testing.T) {
	mockRepo := &MockCommentsRepository{}
	handler := NewCommentsHandler(mockRepo)

	req := httptest.NewRequest(http.MethodDelete, "/v1/comments/comment-123", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "comment-123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	handler.Delete(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rec.Code)
	}
}

func TestDeleteComment_NotFound(t *testing.T) {
	mockRepo := &MockCommentsRepository{
		findByIDErr: ErrCommentNotFound,
	}
	handler := NewCommentsHandler(mockRepo)

	req := httptest.NewRequest(http.MethodDelete, "/v1/comments/nonexistent", nil)
	req = addCommentsAuthContext(req, "user-123", "user")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "nonexistent")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	handler.Delete(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", rec.Code)
	}
}

// Test with different target types

func TestCreateComment_OnAnswer(t *testing.T) {
	mockRepo := &MockCommentsRepository{
		targetExists: true,
	}

	handler := NewCommentsHandler(mockRepo)

	body := `{"content": "Thanks for this answer!"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/answers/answer-123/comments", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req = addCommentsAuthContext(req, "user-123", "user")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("target_type", "answer")
	rctx.URLParams.Add("id", "answer-123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	handler.Create(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d; body: %s", rec.Code, rec.Body.String())
	}

	var response struct {
		Data models.Comment `json:"data"`
	}
	json.NewDecoder(rec.Body).Decode(&response)

	if response.Data.TargetType != models.CommentTargetAnswer {
		t.Errorf("expected target_type 'answer', got %s", response.Data.TargetType)
	}
}

func TestCreateComment_OnResponse(t *testing.T) {
	mockRepo := &MockCommentsRepository{
		targetExists: true,
	}

	handler := NewCommentsHandler(mockRepo)

	body := `{"content": "Great response to the idea!"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/responses/response-123/comments", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req = addCommentsAuthContext(req, "user-123", "user")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("target_type", "response")
	rctx.URLParams.Add("id", "response-123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	handler.Create(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d; body: %s", rec.Code, rec.Body.String())
	}

	var response struct {
		Data models.Comment `json:"data"`
	}
	json.NewDecoder(rec.Body).Decode(&response)

	if response.Data.TargetType != models.CommentTargetResponse {
		t.Errorf("expected target_type 'response', got %s", response.Data.TargetType)
	}
}

// Test helper functions

func TestIsValidCommentTargetType(t *testing.T) {
	validTypes := []models.CommentTargetType{
		models.CommentTargetApproach,
		models.CommentTargetAnswer,
		models.CommentTargetResponse,
	}

	for _, tt := range validTypes {
		if !models.IsValidCommentTargetType(tt) {
			t.Errorf("expected %s to be valid", tt)
		}
	}

	invalidTypes := []models.CommentTargetType{"post", "problem", "question", "idea", "invalid"}
	for _, tt := range invalidTypes {
		if models.IsValidCommentTargetType(tt) {
			t.Errorf("expected %s to be invalid", tt)
		}
	}
}

// Verify ErrCommentNotFound is defined
var _ = errors.New("comment not found")
