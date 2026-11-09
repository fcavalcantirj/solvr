package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fcavalcantirj/solvr/internal/covers"
	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
)

// Blog covers (SEO audit 2026-11-08, Release A): served from the store, drawn on publish or
// on the operator's call, and backfilled for the posts that have none.

type fakeCoverStore struct {
	covers  map[string]*db.BlogCover // by post id
	slugs   map[string]string        // slug -> post id
	named   map[string]string        // post id -> url
	saveErr error
	missing []db.BlogCoverBrief
	deleted []string
	mu      sync.Mutex
	grace   time.Duration
}

func newFakeCoverStore() *fakeCoverStore {
	return &fakeCoverStore{covers: map[string]*db.BlogCover{}, slugs: map[string]string{}, named: map[string]string{}}
}

func (f *fakeCoverStore) Save(_ context.Context, id string, c *db.BlogCover) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.saveErr != nil {
		return f.saveErr
	}
	f.covers[id] = c
	return nil
}

func (f *fakeCoverStore) NameCover(_ context.Context, id, url string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.named[id] = url
	return nil
}

func (f *fakeCoverStore) FindBySlug(_ context.Context, slug string) (*db.BlogCover, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if c, ok := f.covers[f.slugs[slug]]; ok {
		return c, nil
	}
	return nil, db.ErrBlogCoverNotFound
}

func (f *fakeCoverStore) Delete(_ context.Context, id, _ string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.covers, id)
	delete(f.named, id)
	f.deleted = append(f.deleted, id)
	return nil
}

func (f *fakeCoverStore) ListPublishedWithoutCover(_ context.Context, grace time.Duration, limit int) ([]db.BlogCoverBrief, error) {
	f.grace = grace
	if len(f.missing) > limit {
		return f.missing[:limit], nil
	}
	return f.missing, nil
}

type fakeImages struct {
	err    error
	briefs []covers.Brief
	mu     sync.Mutex
}

func (f *fakeImages) Generate(_ context.Context, b covers.Brief) (*covers.Image, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return nil, f.err
	}
	f.briefs = append(f.briefs, b)
	return &covers.Image{Bytes: []byte{0xFF, 0xD8, 1, 2}, ContentType: "image/jpeg", Model: "gpt-image-2", Prompt: "p:" + b.Title}, nil
}

type fakePostReader struct{ post *models.BlogPostWithAuthor }

func (f *fakePostReader) FindBySlug(_ context.Context, slug string) (*models.BlogPostWithAuthor, error) {
	if f.post == nil || f.post.Slug != slug {
		return nil, db.ErrBlogPostNotFound
	}
	return f.post, nil
}

func withCoverSlug(req *http.Request, slug string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("slug", slug)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}

func TestBlogCover_GetServesTheStoredCover(t *testing.T) {
	store := newFakeCoverStore()
	store.slugs["a-post"] = "id-1"
	store.covers["id-1"] = &db.BlogCover{Image: []byte{0xFF, 0xD8, 9}, ContentType: "image/jpeg", CreatedAt: time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC)}
	h := NewBlogCoverHandler(store, nil)

	rec := httptest.NewRecorder()
	h.Get(rec, withCoverSlug(httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/blog/a-post/cover.jpg", http.NoBody), "a-post"))
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "image/jpeg", rec.Header().Get("Content-Type"))
	assert.Equal(t, "3", rec.Header().Get("Content-Length"))
	assert.Contains(t, rec.Header().Get("Cache-Control"), "max-age=86400")
	assert.Equal(t, []byte{0xFF, 0xD8, 9}, rec.Body.Bytes())

	rec = httptest.NewRecorder()
	h.Get(rec, withCoverSlug(httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/blog/other/cover.jpg", http.NoBody), "other"))
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestBlogCover_RegenerateDrawsNow(t *testing.T) {
	store, images := newFakeCoverStore(), &fakeImages{}
	h := NewBlogCoverHandler(store, NewCoverMaker(store, images, "https://api.solvr.dev"))
	h.SetPostReader(&fakePostReader{post: &models.BlogPostWithAuthor{BlogPost: models.BlogPost{ID: "id-1", Slug: "a-post", Title: "A post", Status: models.BlogPostStatusPublished}}})

	rec := httptest.NewRecorder()
	h.Regenerate(rec, withCoverSlug(httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/admin/blog/a-post/cover", http.NoBody), "a-post"))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"cover_image_url":"https://api.solvr.dev/v1/blog/a-post/cover.jpg"`)
	assert.Equal(t, "https://api.solvr.dev/v1/blog/a-post/cover.jpg", store.named["id-1"])

	rec = httptest.NewRecorder()
	h.Regenerate(rec, withCoverSlug(httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/admin/blog/nope/cover", http.NoBody), "nope"))
	assert.Equal(t, http.StatusNotFound, rec.Code)

	images.err = errors.New("rate limited")
	rec = httptest.NewRecorder()
	h.Regenerate(rec, withCoverSlug(httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/admin/blog/a-post/cover", http.NoBody), "a-post"))
	assert.Equal(t, http.StatusBadGateway, rec.Code)
	assert.Contains(t, rec.Body.String(), "COVER_NOT_DRAWN")

	rec = httptest.NewRecorder()
	NewBlogCoverHandler(store, nil).Regenerate(rec, withCoverSlug(httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/admin/blog/a-post/cover", http.NoBody), "a-post"))
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
}

func TestBlogCover_BackfillListsThenDraws(t *testing.T) {
	store, images := newFakeCoverStore(), &fakeImages{}
	store.missing = []db.BlogCoverBrief{{ID: "id-1", Slug: "one", Title: "One"}, {ID: "id-2", Slug: "two", Title: "Two"}, {ID: "id-3", Slug: "three", Title: "Three"}}
	h := NewBlogCoverHandler(store, NewCoverMaker(store, images, "https://api.solvr.dev"))

	rec := httptest.NewRecorder()
	h.Backfill(rec, httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/admin/blog/covers/backfill", bytes.NewBufferString(`{"limit":2,"dry_run":true}`)))
	require.Equal(t, http.StatusAccepted, rec.Code, rec.Body.String())
	var out struct {
		Data struct {
			Slugs  []string `json:"slugs"`
			Queued int      `json:"queued"`
			DryRun bool     `json:"dry_run"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	assert.True(t, out.Data.DryRun)
	assert.Equal(t, []string{"one", "two"}, out.Data.Slugs)
	time.Sleep(30 * time.Millisecond)
	assert.Empty(t, images.briefs, "a dry run draws nothing")

	rec = httptest.NewRecorder()
	h.Backfill(rec, httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/admin/blog/covers/backfill", bytes.NewBufferString(`{"limit":2}`)))
	require.Equal(t, http.StatusAccepted, rec.Code)
	assert.Equal(t, time.Duration(0), store.grace, "the operator waits for no grace")
	require.Eventually(t, func() bool {
		store.mu.Lock()
		defer store.mu.Unlock()
		return len(store.named) == 2
	}, 2*time.Second, 10*time.Millisecond)
	assert.Equal(t, "https://api.solvr.dev/v1/blog/two/cover.jpg", store.named["id-2"])

	rec = httptest.NewRecorder()
	h.Backfill(rec, httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/admin/blog/covers/backfill", bytes.NewBufferString(`{"limit":500}`)))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// The author's own cover (B1): a multipart upload stored as model "author", named on the post,
// and removable; it is refused for anyone but the author, for other file types and for a file
// over 2 MB.
func multipartImage(t *testing.T, field string, data []byte) (body *bytes.Buffer, contentType string) {
	t.Helper()
	body = &bytes.Buffer{}
	mw := multipart.NewWriter(body)
	part, err := mw.CreateFormFile(field, "cover.png")
	require.NoError(t, err)
	_, err = part.Write(data)
	require.NoError(t, err)
	require.NoError(t, mw.Close())
	return body, mw.FormDataContentType()
}

var pngBytes = append([]byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}, make([]byte, 32)...)

func TestBlogCover_UploadStoresTheAuthorsCover(t *testing.T) {
	store := newFakeCoverStore()
	h := NewBlogCoverHandler(store, nil)
	h.SetPublicAPI("https://api.solvr.dev")
	h.SetPostReader(&fakePostReader{post: &models.BlogPostWithAuthor{BlogPost: models.BlogPost{ID: "id-1", Slug: "a-post", Title: "A post",
		Status: models.BlogPostStatusPublished, PostedByType: models.AuthorTypeHuman, PostedByID: "user-123"}}})

	body, ctype := multipartImage(t, "image", pngBytes)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/v1/blog/a-post/cover", body)
	req.Header.Set("Content-Type", ctype)
	req = addBlogAuthContext(withCoverSlug(req, "a-post"), "user-123", "user")
	rec := httptest.NewRecorder()
	h.Upload(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, "image/png", store.covers["id-1"].ContentType)
	assert.Equal(t, "author", store.covers["id-1"].Model)
	assert.Equal(t, "https://api.solvr.dev/v1/blog/a-post/cover.jpg", store.named["id-1"])
	assert.Contains(t, rec.Body.String(), `"cover_image_url":"https://api.solvr.dev/v1/blog/a-post/cover.jpg"`)

	// Removal clears it.
	rec = httptest.NewRecorder()
	h.Remove(rec, addBlogAuthContext(withCoverSlug(httptest.NewRequestWithContext(context.Background(), http.MethodDelete, "/v1/blog/a-post/cover", http.NoBody), "a-post"), "user-123", "user"))
	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Equal(t, []string{"id-1"}, store.deleted)
}

func TestBlogCover_UploadRefusals(t *testing.T) {
	post := &models.BlogPostWithAuthor{BlogPost: models.BlogPost{ID: "id-1", Slug: "a-post", Title: "A post",
		Status: models.BlogPostStatusPublished, PostedByType: models.AuthorTypeHuman, PostedByID: "user-123"}}
	newHandler := func() *BlogCoverHandler {
		h := NewBlogCoverHandler(newFakeCoverStore(), nil)
		h.SetPublicAPI("https://api.solvr.dev")
		h.SetPostReader(&fakePostReader{post: post})
		return h
	}
	upload := func(h *BlogCoverHandler, user string, field string, data []byte, slug string) *httptest.ResponseRecorder {
		body, ctype := multipartImage(t, field, data)
		req := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/v1/blog/"+slug+"/cover", body)
		req.Header.Set("Content-Type", ctype)
		req = withCoverSlug(req, slug)
		if user != "" {
			req = addBlogAuthContext(req, user, "user")
		}
		rec := httptest.NewRecorder()
		h.Upload(rec, req)
		return rec
	}
	assert.Equal(t, http.StatusUnauthorized, upload(newHandler(), "", "image", pngBytes, "a-post").Code)
	assert.Equal(t, http.StatusForbidden, upload(newHandler(), "someone-else", "image", pngBytes, "a-post").Code)
	assert.Equal(t, http.StatusNotFound, upload(newHandler(), "user-123", "image", pngBytes, "no-such").Code)
	assert.Equal(t, http.StatusBadRequest, upload(newHandler(), "user-123", "file", pngBytes, "a-post").Code, "the field must be named image")
	assert.Equal(t, http.StatusUnsupportedMediaType, upload(newHandler(), "user-123", "image", []byte("<svg xmlns='http://www.w3.org/2000/svg'></svg>"), "a-post").Code)
	assert.Equal(t, http.StatusRequestEntityTooLarge, upload(newHandler(), "user-123", "image", append(append([]byte{}, pngBytes...), bytes.Repeat([]byte{1}, coverUploadMax)...), "a-post").Code)
}
