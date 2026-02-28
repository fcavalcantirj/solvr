package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/auth"
	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/go-chi/chi/v5"
)

// ============================================================================
// Mock Blog Post Repository
// ============================================================================

// MockBlogPostRepository implements BlogPostRepositoryInterface for testing.
type MockBlogPostRepository struct {
	posts          []models.BlogPostWithAuthor
	post           *models.BlogPostWithAuthor
	total          int
	err            error
	voteErr        error
	createdPost    *models.BlogPost
	updatedPost    *models.BlogPost
	deletedSlug    string
	listOpts       models.BlogPostListOptions
	viewSlug       string
	tags           []models.BlogTag
	featuredPost   *models.BlogPostWithAuthor
	slugExistsVal  bool
	votedID        string
	votedDirection string
	votedType      string
	votedVoterID   string
}

func NewMockBlogPostRepository() *MockBlogPostRepository {
	return &MockBlogPostRepository{
		posts: []models.BlogPostWithAuthor{},
		tags:  []models.BlogTag{},
	}
}

func (m *MockBlogPostRepository) List(ctx context.Context, opts models.BlogPostListOptions) ([]models.BlogPostWithAuthor, int, error) {
	m.listOpts = opts
	if m.err != nil {
		return nil, 0, m.err
	}
	return m.posts, m.total, nil
}

func (m *MockBlogPostRepository) FindBySlug(ctx context.Context, slug string) (*models.BlogPostWithAuthor, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.post == nil {
		return nil, db.ErrBlogPostNotFound
	}
	return m.post, nil
}

func (m *MockBlogPostRepository) FindBySlugForViewer(ctx context.Context, slug string, viewerType models.AuthorType, viewerID string) (*models.BlogPostWithAuthor, error) {
	return m.FindBySlug(ctx, slug)
}

func (m *MockBlogPostRepository) Create(ctx context.Context, post *models.BlogPost) (*models.BlogPost, error) {
	if m.err != nil {
		return nil, m.err
	}
	m.createdPost = post
	post.ID = "new-blog-id"
	post.CreatedAt = time.Now()
	post.UpdatedAt = time.Now()
	if post.ReadTimeMinutes == 0 {
		post.ReadTimeMinutes = models.CalculateReadTime(post.Body)
	}
	if post.Excerpt == "" {
		post.Excerpt = models.GenerateExcerpt(post.Body, 500)
	}
	return post, nil
}

func (m *MockBlogPostRepository) Update(ctx context.Context, post *models.BlogPost) (*models.BlogPost, error) {
	if m.err != nil {
		return nil, m.err
	}
	m.updatedPost = post
	post.UpdatedAt = time.Now()
	return post, nil
}

func (m *MockBlogPostRepository) Delete(ctx context.Context, slug string) error {
	if m.err != nil {
		return m.err
	}
	m.deletedSlug = slug
	return nil
}

func (m *MockBlogPostRepository) Vote(ctx context.Context, blogPostID, voterType, voterID, direction string) error {
	if m.voteErr != nil {
		return m.voteErr
	}
	m.votedID = blogPostID
	m.votedDirection = direction
	m.votedType = voterType
	m.votedVoterID = voterID
	return nil
}

func (m *MockBlogPostRepository) IncrementViewCount(ctx context.Context, slug string) error {
	if m.err != nil {
		return m.err
	}
	m.viewSlug = slug
	return nil
}

func (m *MockBlogPostRepository) ListTags(ctx context.Context) ([]models.BlogTag, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.tags, nil
}

func (m *MockBlogPostRepository) GetFeatured(ctx context.Context) (*models.BlogPostWithAuthor, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.featuredPost == nil {
		return nil, db.ErrBlogPostNotFound
	}
	return m.featuredPost, nil
}

func (m *MockBlogPostRepository) SlugExists(ctx context.Context, slug string) (bool, error) {
	if m.err != nil {
		return false, m.err
	}
	return m.slugExistsVal, nil
}

// Setter helpers for tests.
func (m *MockBlogPostRepository) SetPosts(posts []models.BlogPostWithAuthor, total int) {
	m.posts = posts
	m.total = total
}

func (m *MockBlogPostRepository) SetPost(post *models.BlogPostWithAuthor) {
	m.post = post
}

func (m *MockBlogPostRepository) SetError(err error) {
	m.err = err
}

func (m *MockBlogPostRepository) SetVoteError(err error) {
	m.voteErr = err
}

func (m *MockBlogPostRepository) SetTags(tags []models.BlogTag) {
	m.tags = tags
}

func (m *MockBlogPostRepository) SetFeaturedPost(post *models.BlogPostWithAuthor) {
	m.featuredPost = post
}

// ============================================================================
// Test Helpers
// ============================================================================

func createTestBlogPost(slug, title string) models.BlogPostWithAuthor {
	now := time.Now()
	body := "This is a test blog post body with enough content to pass validation checks for the test cases"
	return models.BlogPostWithAuthor{
		BlogPost: models.BlogPost{
			ID:              "blog-1",
			Slug:            slug,
			Title:           title,
			Body:            body,
			Excerpt:         models.GenerateExcerpt(body, 500),
			Tags:            []string{"go", "testing"},
			PostedByType:    models.AuthorTypeHuman,
			PostedByID:      "user-123",
			Status:          models.BlogPostStatusPublished,
			ViewCount:       10,
			Upvotes:         5,
			Downvotes:       1,
			ReadTimeMinutes: 1,
			CreatedAt:       now,
			UpdatedAt:       now,
		},
		Author: models.BlogPostAuthor{
			Type:        models.AuthorTypeHuman,
			ID:          "user-123",
			DisplayName: "Test User",
			AvatarURL:   "https://example.com/avatar.png",
		},
		VoteScore: 4,
	}
}

func addBlogAuthContext(r *http.Request, userID, role string) *http.Request {
	claims := &auth.Claims{
		UserID: userID,
		Role:   role,
	}
	ctx := auth.ContextWithClaims(r.Context(), claims)
	return r.WithContext(ctx)
}

func addBlogAgentAuthContext(r *http.Request, agentID string) *http.Request {
	agent := &models.Agent{
		ID:          agentID,
		DisplayName: "Test Agent",
	}
	ctx := auth.ContextWithAgent(r.Context(), agent)
	return r.WithContext(ctx)
}

// ============================================================================
// POST /v1/blog - Create Blog Post Tests
// ============================================================================

func TestCreateBlogPost_Success(t *testing.T) {
	repo := NewMockBlogPostRepository()
	handler := NewBlogHandler(repo)

	body := `{
		"title": "My First Blog Post on Solvr",
		"body": "This is a detailed blog post about building with Go and PostgreSQL. It has enough content to pass the minimum validation.",
		"tags": ["go", "postgresql"],
		"status": "draft"
	}`

	req := httptest.NewRequest(http.MethodPost, "/v1/blog", strings.NewReader(body))
	req = addBlogAuthContext(req, "user-123", "user")
	w := httptest.NewRecorder()

	handler.Create(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		t.Fatal("expected data object in response")
	}

	if data["title"] != "My First Blog Post on Solvr" {
		t.Errorf("expected title 'My First Blog Post on Solvr', got '%v'", data["title"])
	}

	if repo.createdPost == nil {
		t.Fatal("expected post to be created in repo")
	}
	if repo.createdPost.PostedByType != models.AuthorTypeHuman {
		t.Errorf("expected posted_by_type 'human', got '%s'", repo.createdPost.PostedByType)
	}
	if repo.createdPost.PostedByID != "user-123" {
		t.Errorf("expected posted_by_id 'user-123', got '%s'", repo.createdPost.PostedByID)
	}
}

func TestCreateBlogPost_NoAuth(t *testing.T) {
	repo := NewMockBlogPostRepository()
	handler := NewBlogHandler(repo)

	body := `{"title": "Test Blog Post Title Here", "body": "A body that is long enough to pass validation checks and requirements."}`
	req := httptest.NewRequest(http.MethodPost, "/v1/blog", strings.NewReader(body))
	w := httptest.NewRecorder()

	handler.Create(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

func TestCreateBlogPost_TitleTooShort(t *testing.T) {
	repo := NewMockBlogPostRepository()
	handler := NewBlogHandler(repo)

	body := `{"title": "Short", "body": "A body that is long enough to pass the fifty character minimum validation check."}`
	req := httptest.NewRequest(http.MethodPost, "/v1/blog", strings.NewReader(body))
	req = addBlogAuthContext(req, "user-123", "user")
	w := httptest.NewRecorder()

	handler.Create(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestCreateBlogPost_TitleTooLong(t *testing.T) {
	repo := NewMockBlogPostRepository()
	handler := NewBlogHandler(repo)

	longTitle := strings.Repeat("a", 301)
	body := `{"title": "` + longTitle + `", "body": "A body that is long enough to pass the fifty character minimum validation check."}`
	req := httptest.NewRequest(http.MethodPost, "/v1/blog", strings.NewReader(body))
	req = addBlogAuthContext(req, "user-123", "user")
	w := httptest.NewRecorder()

	handler.Create(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestCreateBlogPost_BodyTooShort(t *testing.T) {
	repo := NewMockBlogPostRepository()
	handler := NewBlogHandler(repo)

	body := `{"title": "Valid Title For Blog Post", "body": "Too short body."}`
	req := httptest.NewRequest(http.MethodPost, "/v1/blog", strings.NewReader(body))
	req = addBlogAuthContext(req, "user-123", "user")
	w := httptest.NewRecorder()

	handler.Create(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestCreateBlogPost_TooManyTags(t *testing.T) {
	repo := NewMockBlogPostRepository()
	handler := NewBlogHandler(repo)

	tags := make([]string, 11)
	for i := range tags {
		tags[i] = "tag" + string(rune('a'+i))
	}
	tagsJSON, _ := json.Marshal(tags)

	body := `{"title": "Valid Title For Blog Post", "body": "A body that is long enough to pass the fifty character minimum validation check.", "tags": ` + string(tagsJSON) + `}`
	req := httptest.NewRequest(http.MethodPost, "/v1/blog", strings.NewReader(body))
	req = addBlogAuthContext(req, "user-123", "user")
	w := httptest.NewRecorder()

	handler.Create(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestCreateBlogPost_AutoSlugGeneration(t *testing.T) {
	repo := NewMockBlogPostRepository()
	handler := NewBlogHandler(repo)

	body := `{
		"title": "My Amazing Blog Post Title",
		"body": "A body that is long enough to pass the fifty character minimum validation check and more.",
		"status": "draft"
	}`
	req := httptest.NewRequest(http.MethodPost, "/v1/blog", strings.NewReader(body))
	req = addBlogAuthContext(req, "user-123", "user")
	w := httptest.NewRecorder()

	handler.Create(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}

	if repo.createdPost == nil {
		t.Fatal("expected post to be created")
	}
	if repo.createdPost.Slug == "" {
		t.Error("expected slug to be auto-generated")
	}
	if repo.createdPost.Slug != "my-amazing-blog-post-title" {
		t.Errorf("expected slug 'my-amazing-blog-post-title', got '%s'", repo.createdPost.Slug)
	}
}

func TestCreateBlogPost_AutoExcerptGeneration(t *testing.T) {
	repo := NewMockBlogPostRepository()
	handler := NewBlogHandler(repo)

	longBody := strings.Repeat("This is a long body for testing auto excerpt generation. ", 20)
	body := `{
		"title": "Auto Excerpt Test Blog Post",
		"body": "` + longBody + `",
		"status": "draft"
	}`
	req := httptest.NewRequest(http.MethodPost, "/v1/blog", strings.NewReader(body))
	req = addBlogAuthContext(req, "user-123", "user")
	w := httptest.NewRecorder()

	handler.Create(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}

	if repo.createdPost == nil {
		t.Fatal("expected post to be created")
	}
	if repo.createdPost.Excerpt != "" {
		// Excerpt is auto-generated by the repo, not the handler — handler leaves it for repo
	}
}

func TestCreateBlogPost_ReadTimeCalculation(t *testing.T) {
	repo := NewMockBlogPostRepository()
	handler := NewBlogHandler(repo)

	// 400 words = 2 min read time
	words := strings.Repeat("word ", 400)
	body := `{
		"title": "Read Time Calculation Test Post",
		"body": "` + strings.TrimSpace(words) + `",
		"status": "draft"
	}`
	req := httptest.NewRequest(http.MethodPost, "/v1/blog", strings.NewReader(body))
	req = addBlogAuthContext(req, "user-123", "user")
	w := httptest.NewRecorder()

	handler.Create(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCreateBlogPost_PublishedAtSetOnPublish(t *testing.T) {
	repo := NewMockBlogPostRepository()
	handler := NewBlogHandler(repo)

	body := `{
		"title": "Published Blog Post Title Here",
		"body": "A body that is long enough to pass the fifty character minimum validation check and beyond.",
		"status": "published"
	}`
	req := httptest.NewRequest(http.MethodPost, "/v1/blog", strings.NewReader(body))
	req = addBlogAuthContext(req, "user-123", "user")
	w := httptest.NewRecorder()

	handler.Create(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}

	if repo.createdPost == nil {
		t.Fatal("expected post to be created")
	}
	if string(repo.createdPost.Status) != "published" {
		t.Errorf("expected status 'published', got '%s'", repo.createdPost.Status)
	}
}

func TestCreateBlogPost_AgentAuth(t *testing.T) {
	repo := NewMockBlogPostRepository()
	handler := NewBlogHandler(repo)

	body := `{
		"title": "Agent Blog Post Title Here",
		"body": "A body that is long enough to pass the fifty character minimum validation check and beyond more.",
		"tags": ["ai", "agents"],
		"status": "draft"
	}`
	req := httptest.NewRequest(http.MethodPost, "/v1/blog", strings.NewReader(body))
	req = addBlogAgentAuthContext(req, "agent-123")
	w := httptest.NewRecorder()

	handler.Create(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}

	if repo.createdPost == nil {
		t.Fatal("expected post to be created")
	}
	if repo.createdPost.PostedByType != models.AuthorTypeAgent {
		t.Errorf("expected posted_by_type 'agent', got '%s'", repo.createdPost.PostedByType)
	}
	if repo.createdPost.PostedByID != "agent-123" {
		t.Errorf("expected posted_by_id 'agent-123', got '%s'", repo.createdPost.PostedByID)
	}
}

// ============================================================================
// GET /v1/blog - List Blog Posts Tests
// ============================================================================

func TestListBlogPosts_Success(t *testing.T) {
	repo := NewMockBlogPostRepository()
	repo.SetPosts([]models.BlogPostWithAuthor{
		createTestBlogPost("post-one", "First Blog Post Title"),
		createTestBlogPost("post-two", "Second Blog Post Title"),
	}, 2)

	handler := NewBlogHandler(repo)
	req := httptest.NewRequest(http.MethodGet, "/v1/blog", nil)
	w := httptest.NewRecorder()

	handler.List(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
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

	meta, ok := resp["meta"].(map[string]interface{})
	if !ok {
		t.Fatal("expected meta object in response")
	}
	if meta["total"] != float64(2) {
		t.Errorf("expected total 2, got %v", meta["total"])
	}
}

func TestListBlogPosts_FilterByTag(t *testing.T) {
	repo := NewMockBlogPostRepository()
	repo.SetPosts([]models.BlogPostWithAuthor{}, 0)
	handler := NewBlogHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/blog?tags=engineering", nil)
	w := httptest.NewRecorder()

	handler.List(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	if len(repo.listOpts.Tags) != 1 || repo.listOpts.Tags[0] != "engineering" {
		t.Errorf("expected tags filter ['engineering'], got %v", repo.listOpts.Tags)
	}
}

func TestListBlogPosts_Pagination(t *testing.T) {
	repo := NewMockBlogPostRepository()
	repo.SetPosts([]models.BlogPostWithAuthor{}, 0)
	handler := NewBlogHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/blog?page=2&per_page=5", nil)
	w := httptest.NewRecorder()

	handler.List(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	if repo.listOpts.Page != 2 {
		t.Errorf("expected page 2, got %d", repo.listOpts.Page)
	}
	if repo.listOpts.PerPage != 5 {
		t.Errorf("expected per_page 5, got %d", repo.listOpts.PerPage)
	}
}

func TestListBlogPosts_EmptyResult(t *testing.T) {
	repo := NewMockBlogPostRepository()
	repo.SetPosts([]models.BlogPostWithAuthor{}, 0)
	handler := NewBlogHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/blog", nil)
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
		t.Fatal("expected data array in response (not null)")
	}
	if len(data) != 0 {
		t.Errorf("expected empty array, got %d items", len(data))
	}
}

// ============================================================================
// GET /v1/blog/{slug} - Get Blog Post Tests
// ============================================================================

func TestGetBlogPost_Success(t *testing.T) {
	repo := NewMockBlogPostRepository()
	testPost := createTestBlogPost("my-slug", "Test Blog Post Title")
	repo.SetPost(&testPost)
	handler := NewBlogHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/blog/my-slug", nil)
	w := httptest.NewRecorder()

	r := chi.NewRouter()
	r.Get("/blog/{slug}", handler.GetBySlug)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		t.Fatal("expected data object in response")
	}
	if data["slug"] != "my-slug" {
		t.Errorf("expected slug 'my-slug', got '%v'", data["slug"])
	}
}

func TestGetBlogPost_NotFound(t *testing.T) {
	repo := NewMockBlogPostRepository()
	// No post set — FindBySlug returns ErrBlogPostNotFound
	handler := NewBlogHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/blog/nonexistent", nil)
	w := httptest.NewRecorder()

	r := chi.NewRouter()
	r.Get("/blog/{slug}", handler.GetBySlug)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

// ============================================================================
// PATCH /v1/blog/{slug} - Update Blog Post Tests
// ============================================================================

func TestUpdateBlogPost_Success(t *testing.T) {
	repo := NewMockBlogPostRepository()
	testPost := createTestBlogPost("my-slug", "Original Title Here")
	repo.SetPost(&testPost)
	handler := NewBlogHandler(repo)

	body := `{"title": "Updated Title For Blog Post"}`
	req := httptest.NewRequest(http.MethodPatch, "/blog/my-slug", strings.NewReader(body))
	req = addBlogAuthContext(req, "user-123", "user")
	w := httptest.NewRecorder()

	r := chi.NewRouter()
	r.Patch("/blog/{slug}", handler.Update)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	if repo.updatedPost == nil {
		t.Fatal("expected post to be updated")
	}
	if repo.updatedPost.Title != "Updated Title For Blog Post" {
		t.Errorf("expected title 'Updated Title For Blog Post', got '%s'", repo.updatedPost.Title)
	}
}

func TestUpdateBlogPost_NotOwner(t *testing.T) {
	repo := NewMockBlogPostRepository()
	testPost := createTestBlogPost("my-slug", "Original Title Here")
	repo.SetPost(&testPost)
	handler := NewBlogHandler(repo)

	body := `{"title": "Hacker Update Attempt Here"}`
	req := httptest.NewRequest(http.MethodPatch, "/blog/my-slug", strings.NewReader(body))
	req = addBlogAuthContext(req, "other-user-456", "user")
	w := httptest.NewRecorder()

	r := chi.NewRouter()
	r.Patch("/blog/{slug}", handler.Update)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected status 403, got %d", w.Code)
	}
}

func TestUpdateBlogPost_StatusTransition(t *testing.T) {
	repo := NewMockBlogPostRepository()
	testPost := createTestBlogPost("my-slug", "Draft Post Title Here")
	testPost.Status = models.BlogPostStatusDraft
	repo.SetPost(&testPost)
	handler := NewBlogHandler(repo)

	body := `{"status": "published"}`
	req := httptest.NewRequest(http.MethodPatch, "/blog/my-slug", strings.NewReader(body))
	req = addBlogAuthContext(req, "user-123", "user")
	w := httptest.NewRecorder()

	r := chi.NewRouter()
	r.Patch("/blog/{slug}", handler.Update)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	if repo.updatedPost == nil {
		t.Fatal("expected post to be updated")
	}
	if string(repo.updatedPost.Status) != "published" {
		t.Errorf("expected status 'published', got '%s'", repo.updatedPost.Status)
	}
	if repo.updatedPost.PublishedAt == nil {
		t.Error("expected published_at to be set on publish transition")
	}
}

// ============================================================================
// DELETE /v1/blog/{slug} - Delete Blog Post Tests
// ============================================================================

func TestDeleteBlogPost_Success(t *testing.T) {
	repo := NewMockBlogPostRepository()
	testPost := createTestBlogPost("my-slug", "Post to Delete Title")
	repo.SetPost(&testPost)
	handler := NewBlogHandler(repo)

	req := httptest.NewRequest(http.MethodDelete, "/blog/my-slug", nil)
	req = addBlogAuthContext(req, "user-123", "user")
	w := httptest.NewRecorder()

	r := chi.NewRouter()
	r.Delete("/blog/{slug}", handler.Delete)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected status 204, got %d: %s", w.Code, w.Body.String())
	}
	if repo.deletedSlug != "my-slug" {
		t.Errorf("expected deleted slug 'my-slug', got '%s'", repo.deletedSlug)
	}
}

func TestDeleteBlogPost_NotOwner(t *testing.T) {
	repo := NewMockBlogPostRepository()
	testPost := createTestBlogPost("my-slug", "Post to Delete Title")
	repo.SetPost(&testPost)
	handler := NewBlogHandler(repo)

	req := httptest.NewRequest(http.MethodDelete, "/blog/my-slug", nil)
	req = addBlogAuthContext(req, "other-user-456", "user")
	w := httptest.NewRecorder()

	r := chi.NewRouter()
	r.Delete("/blog/{slug}", handler.Delete)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected status 403, got %d", w.Code)
	}
}

// ============================================================================
// POST /v1/blog/{slug}/vote - Vote Blog Post Tests
// ============================================================================

func TestVoteBlogPost_Success(t *testing.T) {
	repo := NewMockBlogPostRepository()
	testPost := createTestBlogPost("my-slug", "Votable Blog Post Title")
	repo.SetPost(&testPost)
	handler := NewBlogHandler(repo)

	body := `{"direction": "up"}`
	req := httptest.NewRequest(http.MethodPost, "/blog/my-slug/vote", strings.NewReader(body))
	req = addBlogAuthContext(req, "other-user-456", "user")
	w := httptest.NewRecorder()

	r := chi.NewRouter()
	r.Post("/blog/{slug}/vote", handler.Vote)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	if repo.votedID != "blog-1" {
		t.Errorf("expected voted ID 'blog-1', got '%s'", repo.votedID)
	}
	if repo.votedDirection != "up" {
		t.Errorf("expected direction 'up', got '%s'", repo.votedDirection)
	}
}

// ============================================================================
// GET /v1/blog/featured - Get Featured Blog Post Tests
// ============================================================================

func TestGetFeatured_Success(t *testing.T) {
	repo := NewMockBlogPostRepository()
	featured := createTestBlogPost("featured-slug", "Featured Blog Post Title")
	repo.SetFeaturedPost(&featured)
	handler := NewBlogHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/blog/featured", nil)
	w := httptest.NewRecorder()

	handler.GetFeatured(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		t.Fatal("expected data object in response")
	}
	if data["slug"] != "featured-slug" {
		t.Errorf("expected slug 'featured-slug', got '%v'", data["slug"])
	}
}

func TestGetFeatured_NoPublished(t *testing.T) {
	repo := NewMockBlogPostRepository()
	// No featured post set
	handler := NewBlogHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/blog/featured", nil)
	w := httptest.NewRecorder()

	handler.GetFeatured(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

// ============================================================================
// POST /v1/blog/{slug}/view - Record View Tests
// ============================================================================

func TestRecordView_Success(t *testing.T) {
	repo := NewMockBlogPostRepository()
	handler := NewBlogHandler(repo)

	req := httptest.NewRequest(http.MethodPost, "/blog/my-slug/view", nil)
	w := httptest.NewRecorder()

	r := chi.NewRouter()
	r.Post("/blog/{slug}/view", handler.RecordView)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected status 204, got %d: %s", w.Code, w.Body.String())
	}

	if repo.viewSlug != "my-slug" {
		t.Errorf("expected view slug 'my-slug', got '%s'", repo.viewSlug)
	}
}

// ============================================================================
// GET /v1/blog/tags - List Tags Tests
// ============================================================================

func TestListTags_Success(t *testing.T) {
	repo := NewMockBlogPostRepository()
	repo.SetTags([]models.BlogTag{
		{Name: "go", Count: 5},
		{Name: "postgresql", Count: 3},
		{Name: "testing", Count: 2},
	})
	handler := NewBlogHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/blog/tags", nil)
	w := httptest.NewRecorder()

	handler.ListTags(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	data, ok := resp["data"].([]interface{})
	if !ok {
		t.Fatal("expected data array in response")
	}
	if len(data) != 3 {
		t.Errorf("expected 3 tags, got %d", len(data))
	}
}
