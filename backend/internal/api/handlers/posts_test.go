package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/auth"
	"github.com/fcavalcantirj/solvr/internal/db"
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
		return nil, db.ErrPostNotFound
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

func (m *MockPostsRepository) Vote(ctx context.Context, postID, voterType, voterID, direction string) error {
	if m.voteErr != nil {
		return m.voteErr
	}
	m.vote = &models.Vote{
		TargetType: "post",
		TargetID:   postID,
		VoterType:  voterType,
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

// Error for testing - ErrCannotVoteOwn is specific to this test file
var ErrCannotVoteOwn = errors.New("cannot vote on own content")

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
