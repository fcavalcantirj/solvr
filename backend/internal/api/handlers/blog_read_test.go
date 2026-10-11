package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/go-chi/chi/v5"
)

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
