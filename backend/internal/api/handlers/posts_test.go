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

// MockPostsRepository implements PostsRepositoryInterface for testing.
type MockPostsRepository struct {
	posts       []models.PostWithAuthor
	post        *models.PostWithAuthor
	total       int
	err         error
	listOpts    models.PostListOptions
	createdPost *models.Post
	updatedPost *models.Post
	deletedID   string
	vote        *models.Vote
	voteErr     error
}

func NewMockPostsRepository() *MockPostsRepository {
	return &MockPostsRepository{
		posts: []models.PostWithAuthor{},
	}
}

func (m *MockPostsRepository) List(ctx context.Context, opts models.PostListOptions) ([]models.PostWithAuthor, int, error) {
	m.listOpts = opts
	if m.err != nil {
		return nil, 0, m.err
	}
	return m.posts, m.total, nil
}

func (m *MockPostsRepository) FindByID(ctx context.Context, id string) (*models.PostWithAuthor, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.post == nil {
		return nil, ErrPostNotFound
	}
	return m.post, nil
}

func (m *MockPostsRepository) Create(ctx context.Context, post *models.Post) (*models.Post, error) {
	if m.err != nil {
		return nil, m.err
	}
	m.createdPost = post
	post.ID = "new-post-id"
	post.CreatedAt = time.Now()
	post.UpdatedAt = time.Now()
	return post, nil
}

func (m *MockPostsRepository) Update(ctx context.Context, post *models.Post) (*models.Post, error) {
	if m.err != nil {
		return nil, m.err
	}
	m.updatedPost = post
	post.UpdatedAt = time.Now()
	return post, nil
}

func (m *MockPostsRepository) Delete(ctx context.Context, id string) error {
	if m.err != nil {
		return m.err
	}
	m.deletedID = id
	return nil
}

func (m *MockPostsRepository) Vote(ctx context.Context, postID string, voterType models.AuthorType, voterID string, direction string) error {
	if m.voteErr != nil {
		return m.voteErr
	}
	m.vote = &models.Vote{
		TargetType: "post",
		TargetID:   postID,
		VoterType:  string(voterType),
		VoterID:    voterID,
		Direction:  direction,
	}
	return nil
}

func (m *MockPostsRepository) SetPosts(posts []models.PostWithAuthor, total int) {
	m.posts = posts
	m.total = total
}

func (m *MockPostsRepository) SetPost(post *models.PostWithAuthor) {
	m.post = post
}

func (m *MockPostsRepository) SetError(err error) {
	m.err = err
}

func (m *MockPostsRepository) SetVoteError(err error) {
	m.voteErr = err
}

// Error definitions for testing
var (
	ErrPostNotFound    = errors.New("post not found")
	ErrDuplicateVote   = errors.New("already voted")
	ErrCannotVoteOwn   = errors.New("cannot vote on own content")
)

// Helper to create test post
func createTestPost(id, title string, postType models.PostType) models.PostWithAuthor {
	now := time.Now()
	return models.PostWithAuthor{
		Post: models.Post{
			ID:           id,
			Type:         postType,
			Title:        title,
			Description:  "Test description with more than fifty characters for validation",
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

// Helper to add auth claims to request context
func addAuthContext(r *http.Request, userID, role string) *http.Request {
	claims := &auth.Claims{
		UserID: userID,
		Role:   role,
	}
	ctx := auth.ContextWithClaims(r.Context(), claims)
	return r.WithContext(ctx)
}

// ============================================================================
// GET /v1/posts - List Posts Tests
// ============================================================================

// TestListPosts_Success tests successful list of posts.
func TestListPosts_Success(t *testing.T) {
	repo := NewMockPostsRepository()
	repo.SetPosts([]models.PostWithAuthor{
		createTestPost("post-1", "First Post", models.PostTypeProblem),
		createTestPost("post-2", "Second Post", models.PostTypeQuestion),
	}, 2)

	handler := NewPostsHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/posts", nil)
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
		t.Errorf("expected 2 posts, got %d", len(data))
	}
}

// TestListPosts_FilterByType tests filtering by type.
func TestListPosts_FilterByType(t *testing.T) {
	repo := NewMockPostsRepository()
	repo.SetPosts([]models.PostWithAuthor{}, 0)

	handler := NewPostsHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/posts?type=problem", nil)
	w := httptest.NewRecorder()

	handler.List(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	if repo.listOpts.Type != models.PostTypeProblem {
		t.Errorf("expected type filter 'problem', got '%s'", repo.listOpts.Type)
	}
}

// TestListPosts_FilterByStatus tests filtering by status.
func TestListPosts_FilterByStatus(t *testing.T) {
	repo := NewMockPostsRepository()
	repo.SetPosts([]models.PostWithAuthor{}, 0)

	handler := NewPostsHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/posts?status=open", nil)
	w := httptest.NewRecorder()

	handler.List(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	if repo.listOpts.Status != models.PostStatusOpen {
		t.Errorf("expected status filter 'open', got '%s'", repo.listOpts.Status)
	}
}

// TestListPosts_FilterByTags tests filtering by tags.
func TestListPosts_FilterByTags(t *testing.T) {
	repo := NewMockPostsRepository()
	repo.SetPosts([]models.PostWithAuthor{}, 0)

	handler := NewPostsHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/posts?tags=go,postgresql", nil)
	w := httptest.NewRecorder()

	handler.List(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	if len(repo.listOpts.Tags) != 2 {
		t.Errorf("expected 2 tags, got %d", len(repo.listOpts.Tags))
	}

	if repo.listOpts.Tags[0] != "go" || repo.listOpts.Tags[1] != "postgresql" {
		t.Errorf("expected tags [go, postgresql], got %v", repo.listOpts.Tags)
	}
}

// TestListPosts_Pagination tests pagination parameters.
func TestListPosts_Pagination(t *testing.T) {
	repo := NewMockPostsRepository()
	repo.SetPosts([]models.PostWithAuthor{}, 100)

	handler := NewPostsHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/posts?page=2&per_page=10", nil)
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

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	meta := resp["meta"].(map[string]interface{})
	if meta["total"].(float64) != 100 {
		t.Errorf("expected total 100, got %v", meta["total"])
	}
}

// TestListPosts_PerPageMax tests per_page capped at 50.
func TestListPosts_PerPageMax(t *testing.T) {
	repo := NewMockPostsRepository()
	repo.SetPosts([]models.PostWithAuthor{}, 0)

	handler := NewPostsHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/posts?per_page=100", nil)
	w := httptest.NewRecorder()

	handler.List(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	if repo.listOpts.PerPage != 50 {
		t.Errorf("expected per_page capped at 50, got %d", repo.listOpts.PerPage)
	}
}

// ============================================================================
// GET /v1/posts/:id - Get Single Post Tests
// ============================================================================

// TestGetPost_Success tests successful retrieval of a post.
func TestGetPost_Success(t *testing.T) {
	repo := NewMockPostsRepository()
	post := createTestPost("post-123", "Test Post", models.PostTypeProblem)
	repo.SetPost(&post)

	handler := NewPostsHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/posts/post-123", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "post-123")
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
	if data["id"] != "post-123" {
		t.Errorf("expected post id 'post-123', got %v", data["id"])
	}

	if data["vote_score"].(float64) != 8 {
		t.Errorf("expected vote_score 8, got %v", data["vote_score"])
	}
}

// TestGetPost_NotFound tests 404 for non-existent post.
func TestGetPost_NotFound(t *testing.T) {
	repo := NewMockPostsRepository()
	repo.SetPost(nil) // No post found

	handler := NewPostsHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/posts/nonexistent", nil)
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

// TestGetPost_Deleted tests 404 for deleted post.
func TestGetPost_Deleted(t *testing.T) {
	repo := NewMockPostsRepository()
	now := time.Now()
	post := createTestPost("post-123", "Deleted Post", models.PostTypeProblem)
	post.DeletedAt = &now
	repo.SetPost(&post)

	handler := NewPostsHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/posts/post-123", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "post-123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	handler.Get(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

// TestGetPost_IncludesAuthorInfo tests that author info is included.
func TestGetPost_IncludesAuthorInfo(t *testing.T) {
	repo := NewMockPostsRepository()
	post := createTestPost("post-123", "Test Post", models.PostTypeProblem)
	repo.SetPost(&post)

	handler := NewPostsHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/posts/post-123", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "post-123")
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
	author := data["author"].(map[string]interface{})

	if author["type"] != "human" {
		t.Errorf("expected author type 'human', got %v", author["type"])
	}

	if author["display_name"] != "Test User" {
		t.Errorf("expected display_name 'Test User', got %v", author["display_name"])
	}
}

// ============================================================================
// POST /v1/posts - Create Post Tests
// ============================================================================

// TestCreatePost_Success tests successful post creation.
func TestCreatePost_Success(t *testing.T) {
	repo := NewMockPostsRepository()
	handler := NewPostsHandler(repo)

	body := map[string]interface{}{
		"type":        "problem",
		"title":       "Test Problem Title That Is Long Enough",
		"description": "This is a test description that needs to be at least fifty characters long to pass validation.",
		"tags":        []string{"go", "testing"},
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/posts", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req = addAuthContext(req, "user-123", "user")
	w := httptest.NewRecorder()

	handler.Create(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	data := resp["data"].(map[string]interface{})
	if data["id"] == nil {
		t.Error("expected post id in response")
	}

	if repo.createdPost.PostedByType != models.AuthorTypeHuman {
		t.Errorf("expected posted_by_type 'human', got '%s'", repo.createdPost.PostedByType)
	}

	if repo.createdPost.PostedByID != "user-123" {
		t.Errorf("expected posted_by_id 'user-123', got '%s'", repo.createdPost.PostedByID)
	}
}

// TestCreatePost_NoAuth tests 401 when not authenticated.
func TestCreatePost_NoAuth(t *testing.T) {
	repo := NewMockPostsRepository()
	handler := NewPostsHandler(repo)

	body := map[string]interface{}{
		"type":        "problem",
		"title":       "Test Problem Title",
		"description": "Test description",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/posts", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	// No auth context
	w := httptest.NewRecorder()

	handler.Create(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

// TestCreatePost_InvalidType tests 400 for invalid type.
func TestCreatePost_InvalidType(t *testing.T) {
	repo := NewMockPostsRepository()
	handler := NewPostsHandler(repo)

	body := map[string]interface{}{
		"type":        "invalid",
		"title":       "Test Title That Is Long Enough",
		"description": "This is a test description that needs to be at least fifty characters long.",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/posts", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req = addAuthContext(req, "user-123", "user")
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
	if errObj["code"] != "INVALID_TYPE" {
		t.Errorf("expected error code INVALID_TYPE, got %v", errObj["code"])
	}
}

// TestCreatePost_TitleTooShort tests 400 for title < 10 chars.
func TestCreatePost_TitleTooShort(t *testing.T) {
	repo := NewMockPostsRepository()
	handler := NewPostsHandler(repo)

	body := map[string]interface{}{
		"type":        "problem",
		"title":       "Short", // Less than 10 chars
		"description": "This is a test description that needs to be at least fifty characters long.",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/posts", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req = addAuthContext(req, "user-123", "user")
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
}

// TestCreatePost_TitleTooLong tests 400 for title > 200 chars.
func TestCreatePost_TitleTooLong(t *testing.T) {
	repo := NewMockPostsRepository()
	handler := NewPostsHandler(repo)

	longTitle := make([]byte, 201)
	for i := range longTitle {
		longTitle[i] = 'a'
	}

	body := map[string]interface{}{
		"type":        "problem",
		"title":       string(longTitle),
		"description": "This is a test description that needs to be at least fifty characters long.",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/posts", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req = addAuthContext(req, "user-123", "user")
	w := httptest.NewRecorder()

	handler.Create(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

// TestCreatePost_DescriptionTooShort tests 400 for description < 50 chars.
func TestCreatePost_DescriptionTooShort(t *testing.T) {
	repo := NewMockPostsRepository()
	handler := NewPostsHandler(repo)

	body := map[string]interface{}{
		"type":        "problem",
		"title":       "Valid Title That Is Long Enough",
		"description": "Too short", // Less than 50 chars
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/posts", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req = addAuthContext(req, "user-123", "user")
	w := httptest.NewRecorder()

	handler.Create(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

// TestCreatePost_MissingTitle tests 400 for missing title.
func TestCreatePost_MissingTitle(t *testing.T) {
	repo := NewMockPostsRepository()
	handler := NewPostsHandler(repo)

	body := map[string]interface{}{
		"type":        "problem",
		"description": "This is a test description that needs to be at least fifty characters long.",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/posts", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req = addAuthContext(req, "user-123", "user")
	w := httptest.NewRecorder()

	handler.Create(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

// TestCreatePost_InvalidJSON tests 400 for malformed JSON.
func TestCreatePost_InvalidJSON(t *testing.T) {
	repo := NewMockPostsRepository()
	handler := NewPostsHandler(repo)

	req := httptest.NewRequest(http.MethodPost, "/v1/posts", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	req = addAuthContext(req, "user-123", "user")
	w := httptest.NewRecorder()

	handler.Create(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

// ============================================================================
// PATCH /v1/posts/:id - Update Post Tests
// ============================================================================

// TestUpdatePost_Success tests successful post update.
func TestUpdatePost_Success(t *testing.T) {
	repo := NewMockPostsRepository()
	post := createTestPost("post-123", "Original Title", models.PostTypeProblem)
	repo.SetPost(&post)

	handler := NewPostsHandler(repo)

	body := map[string]interface{}{
		"title": "Updated Title That Is Long Enough",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPatch, "/v1/posts/post-123", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "post-123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = addAuthContext(req, "user-123", "user") // Same as post owner
	w := httptest.NewRecorder()

	handler.Update(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	if repo.updatedPost == nil {
		t.Error("expected post to be updated")
	}
}

// TestUpdatePost_NotOwner tests 403 for non-owner.
func TestUpdatePost_NotOwner(t *testing.T) {
	repo := NewMockPostsRepository()
	post := createTestPost("post-123", "Original Title", models.PostTypeProblem)
	repo.SetPost(&post)

	handler := NewPostsHandler(repo)

	body := map[string]interface{}{
		"title": "Updated Title",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPatch, "/v1/posts/post-123", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "post-123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = addAuthContext(req, "other-user", "user") // Different user
	w := httptest.NewRecorder()

	handler.Update(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected status 403, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	errObj := resp["error"].(map[string]interface{})
	if errObj["code"] != "FORBIDDEN" {
		t.Errorf("expected error code FORBIDDEN, got %v", errObj["code"])
	}
}

// TestUpdatePost_NoAuth tests 401 when not authenticated.
func TestUpdatePost_NoAuth(t *testing.T) {
	repo := NewMockPostsRepository()
	handler := NewPostsHandler(repo)

	body := map[string]interface{}{
		"title": "Updated Title",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPatch, "/v1/posts/post-123", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "post-123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	// No auth context
	w := httptest.NewRecorder()

	handler.Update(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

// TestUpdatePost_NotFound tests 404 when post doesn't exist.
func TestUpdatePost_NotFound(t *testing.T) {
	repo := NewMockPostsRepository()
	repo.SetPost(nil)

	handler := NewPostsHandler(repo)

	body := map[string]interface{}{
		"title": "Updated Title",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPatch, "/v1/posts/nonexistent", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "nonexistent")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = addAuthContext(req, "user-123", "user")
	w := httptest.NewRecorder()

	handler.Update(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

// ============================================================================
// DELETE /v1/posts/:id - Delete Post Tests
// ============================================================================

// TestDeletePost_OwnerCanDelete tests owner can delete their post.
func TestDeletePost_OwnerCanDelete(t *testing.T) {
	repo := NewMockPostsRepository()
	post := createTestPost("post-123", "Test Post", models.PostTypeProblem)
	repo.SetPost(&post)

	handler := NewPostsHandler(repo)

	req := httptest.NewRequest(http.MethodDelete, "/v1/posts/post-123", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "post-123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = addAuthContext(req, "user-123", "user") // Same as post owner
	w := httptest.NewRecorder()

	handler.Delete(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected status 204, got %d", w.Code)
	}

	if repo.deletedID != "post-123" {
		t.Errorf("expected deletedID 'post-123', got '%s'", repo.deletedID)
	}
}

// TestDeletePost_AdminCanDelete tests admin can delete any post.
func TestDeletePost_AdminCanDelete(t *testing.T) {
	repo := NewMockPostsRepository()
	post := createTestPost("post-123", "Test Post", models.PostTypeProblem)
	repo.SetPost(&post)

	handler := NewPostsHandler(repo)

	req := httptest.NewRequest(http.MethodDelete, "/v1/posts/post-123", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "post-123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = addAuthContext(req, "admin-user", "admin") // Admin, different from owner
	w := httptest.NewRecorder()

	handler.Delete(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected status 204, got %d", w.Code)
	}
}

// TestDeletePost_OthersForbidden tests non-owner non-admin gets 403.
func TestDeletePost_OthersForbidden(t *testing.T) {
	repo := NewMockPostsRepository()
	post := createTestPost("post-123", "Test Post", models.PostTypeProblem)
	repo.SetPost(&post)

	handler := NewPostsHandler(repo)

	req := httptest.NewRequest(http.MethodDelete, "/v1/posts/post-123", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "post-123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = addAuthContext(req, "other-user", "user") // Different user, not admin
	w := httptest.NewRecorder()

	handler.Delete(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected status 403, got %d", w.Code)
	}
}

// TestDeletePost_NoAuth tests 401 when not authenticated.
func TestDeletePost_NoAuth(t *testing.T) {
	repo := NewMockPostsRepository()
	handler := NewPostsHandler(repo)

	req := httptest.NewRequest(http.MethodDelete, "/v1/posts/post-123", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "post-123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	// No auth context
	w := httptest.NewRecorder()

	handler.Delete(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

// ============================================================================
// POST /v1/posts/:id/vote - Vote Tests
// ============================================================================

// TestVote_Upvote tests successful upvote.
func TestVote_Upvote(t *testing.T) {
	repo := NewMockPostsRepository()
	post := createTestPost("post-123", "Test Post", models.PostTypeProblem)
	post.PostedByID = "other-user" // Different from voter
	repo.SetPost(&post)

	handler := NewPostsHandler(repo)

	body := map[string]interface{}{
		"direction": "up",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/posts/post-123/vote", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "post-123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = addAuthContext(req, "user-123", "user")
	w := httptest.NewRecorder()

	handler.Vote(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	if repo.vote.Direction != "up" {
		t.Errorf("expected direction 'up', got '%s'", repo.vote.Direction)
	}
}

// TestVote_Downvote tests successful downvote.
func TestVote_Downvote(t *testing.T) {
	repo := NewMockPostsRepository()
	post := createTestPost("post-123", "Test Post", models.PostTypeProblem)
	post.PostedByID = "other-user"
	repo.SetPost(&post)

	handler := NewPostsHandler(repo)

	body := map[string]interface{}{
		"direction": "down",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/posts/post-123/vote", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "post-123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = addAuthContext(req, "user-123", "user")
	w := httptest.NewRecorder()

	handler.Vote(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	if repo.vote.Direction != "down" {
		t.Errorf("expected direction 'down', got '%s'", repo.vote.Direction)
	}
}

// TestVote_InvalidDirection tests 400 for invalid direction.
func TestVote_InvalidDirection(t *testing.T) {
	repo := NewMockPostsRepository()
	post := createTestPost("post-123", "Test Post", models.PostTypeProblem)
	repo.SetPost(&post)

	handler := NewPostsHandler(repo)

	body := map[string]interface{}{
		"direction": "sideways",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/posts/post-123/vote", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "post-123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = addAuthContext(req, "user-123", "user")
	w := httptest.NewRecorder()

	handler.Vote(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

// TestVote_NoAuth tests 401 when not authenticated.
func TestVote_NoAuth(t *testing.T) {
	repo := NewMockPostsRepository()
	handler := NewPostsHandler(repo)

	body := map[string]interface{}{
		"direction": "up",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/posts/post-123/vote", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "post-123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	// No auth context
	w := httptest.NewRecorder()

	handler.Vote(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

// TestVote_DuplicateVote tests 409 for duplicate vote.
func TestVote_DuplicateVote(t *testing.T) {
	repo := NewMockPostsRepository()
	post := createTestPost("post-123", "Test Post", models.PostTypeProblem)
	post.PostedByID = "other-user"
	repo.SetPost(&post)
	repo.SetVoteError(ErrDuplicateVote)

	handler := NewPostsHandler(repo)

	body := map[string]interface{}{
		"direction": "up",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/posts/post-123/vote", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "post-123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = addAuthContext(req, "user-123", "user")
	w := httptest.NewRecorder()

	handler.Vote(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("expected status 409, got %d", w.Code)
	}
}

// ============================================================================
// Vote struct for testing
// ============================================================================

// Vote represents a vote record per SPEC.md Part 2.9.
type Vote struct {
	TargetType string
	TargetID   string
	VoterType  string
	VoterID    string
	Direction  string
}
