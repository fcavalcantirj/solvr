package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/fcavalcantirj/solvr/internal/covers"
	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
)

// Blog covers (SEO audit 2026-11-08, Releases A and B1).
//
//	GET    /v1/blog/{slug}/cover.jpg      the post's cover (public; 404 without one)
//	PUT    /v1/blog/{slug}/cover          the author uploads their own (multipart "image",
//	                                      JPEG/PNG/WebP, at most 2 MB); it always wins
//	DELETE /v1/blog/{slug}/cover          the author removes it
//	POST   /admin/blog/{slug}/cover       draw the post's cover now and name it on the post
//	                                      (operator-only; about a minute)
//	POST   /admin/blog/covers/backfill    {"limit", "dry_run"}: the published posts with no
//	                                      cover, oldest first; without dry_run their covers are
//	                                      drawn in the background, one at a time (operator-only)
//
// A post published without a cover gets one drawn by jobs.BlogCoverJob once its author's grace
// (24 h) has passed. Everything is one code path: CoverMaker.Make asks the image service, stores
// the bytes and names the route on the post's cover_image_url.

// BlogCoverStore is what the maker needs of the cover store (db.BlogCoverRepository).
type BlogCoverStore interface {
	Save(ctx context.Context, blogPostID string, cover *db.BlogCover) error
	NameCover(ctx context.Context, blogPostID, url string) error
	FindBySlug(ctx context.Context, slug string) (*db.BlogCover, error)
	ListPublishedWithoutCover(ctx context.Context, grace time.Duration, limit int) ([]db.BlogCoverBrief, error)
	Delete(ctx context.Context, blogPostID, servedURL string) error
}

// CoverMaker draws covers and stores them.
type CoverMaker struct {
	store  BlogCoverStore
	images covers.Service
	// publicAPI is the API's public origin, the one the cover URL names (PUBLIC_API_URL).
	publicAPI string
	// timeout bounds one drawing.
	timeout time.Duration
	// backfill serialises the background runs: one drawing at a time per instance.
	backfill sync.Mutex
}

// NewCoverMaker creates the maker. publicAPI is where the cover route is reachable from a
// browser (https://api.solvr.dev).
func NewCoverMaker(store BlogCoverStore, images covers.Service, publicAPI string) *CoverMaker {
	return &CoverMaker{store: store, images: images, publicAPI: publicAPI, timeout: covers.DefaultTimeout + 30*time.Second}
}

// CoverURL is the address that serves a post's cover.
func (m *CoverMaker) CoverURL(slug string) string {
	return m.publicAPI + "/v1/blog/" + slug + "/cover.jpg"
}

// Make draws the brief's cover, stores it and names it on the post. It answers the cover's URL.
func (m *CoverMaker) Make(ctx context.Context, brief *db.BlogCoverBrief) (string, error) {
	img, err := m.images.Generate(ctx, covers.Brief{Title: brief.Title, Excerpt: brief.Excerpt, Tags: brief.Tags})
	if err != nil {
		return "", fmt.Errorf("draw cover for %s: %w", brief.Slug, err)
	}
	if err := m.store.Save(ctx, brief.ID, &db.BlogCover{Image: img.Bytes, ContentType: img.ContentType, Model: img.Model, Prompt: img.Prompt}); err != nil {
		return "", err
	}
	url := m.CoverURL(brief.Slug)
	if err := m.store.NameCover(ctx, brief.ID, url); err != nil {
		return "", err
	}
	slog.Info("blog cover drawn", "post_id", brief.ID, "bytes", len(img.Bytes), "model", img.Model) //nolint:gosec // G706: a UUID read from the store
	return url, nil
}

func (m *CoverMaker) runOne(brief *db.BlogCoverBrief) {
	m.backfill.Lock()
	defer m.backfill.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), m.timeout)
	defer cancel()
	if _, err := m.Make(ctx, brief); err != nil {
		slog.Error("blog cover not drawn", "post_id", brief.ID, "error", err)
	}
}

// BlogCoverHandler serves the routes.
type BlogCoverHandler struct {
	store     BlogCoverStore
	maker     *CoverMaker // nil when no image service is configured
	posts     BlogPostBriefReader
	publicAPI string
}

// NewBlogCoverHandler creates the handler. maker may be nil: covers are then served but
// never drawn, and the admin routes answer 503.
func NewBlogCoverHandler(store BlogCoverStore, maker *CoverMaker) *BlogCoverHandler {
	return &BlogCoverHandler{store: store, maker: maker}
}

// Get handles GET /v1/blog/{slug}/cover.jpg.
func (h *BlogCoverHandler) Get(w http.ResponseWriter, r *http.Request) {
	cover, err := h.store.FindBySlug(r.Context(), chi.URLParam(r, "slug"))
	if errors.Is(err, db.ErrBlogCoverNotFound) {
		writeBlogError(w, http.StatusNotFound, "NOT_FOUND", "this blog post has no cover")
		return
	}
	if err != nil {
		slog.Error("blog cover read failed", "error", err)
		writeBlogError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to read the cover")
		return
	}
	w.Header().Set("Content-Type", cover.ContentType)
	w.Header().Set("Content-Length", strconv.Itoa(len(cover.Image)))
	w.Header().Set("Cache-Control", "public, max-age=86400, s-maxage=604800, stale-while-revalidate=604800")
	w.Header().Set("Last-Modified", cover.CreatedAt.UTC().Format(http.TimeFormat))
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(cover.Image); err != nil { //nolint:gosec // G705: image bytes under an image Content-Type, sniffed on upload
		slog.Debug("blog cover: client went away during the write", "error", err)
	}
}

// Regenerate handles POST /admin/blog/{slug}/cover: it draws the cover now and answers its URL.
func (h *BlogCoverHandler) Regenerate(w http.ResponseWriter, r *http.Request) {
	if h.maker == nil {
		writeOperatorError(w, http.StatusServiceUnavailable, "COVERS_NOT_CONFIGURED", "no image service: set OPENAI_API_KEY")
		return
	}
	brief, err := h.briefFor(r.Context(), chi.URLParam(r, "slug"))
	if errors.Is(err, db.ErrBlogPostNotFound) {
		writeOperatorError(w, http.StatusNotFound, "NOT_FOUND", "no such published blog post")
		return
	}
	if err != nil {
		slog.Error("blog cover: read post failed", "error", err)
		writeOperatorError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to read the post")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), h.maker.timeout)
	defer cancel()
	url, drawErr := h.maker.Make(ctx, brief)
	if drawErr != nil {
		slog.Error("blog cover: draw failed", "post_id", brief.ID, "error", drawErr) //nolint:gosec // G706: a UUID read from the store
		writeOperatorError(w, http.StatusBadGateway, "COVER_NOT_DRAWN", "the image service did not draw the cover: "+drawErr.Error())
		return
	}
	writeActivationJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"slug": brief.Slug, "cover_image_url": url}})
}

// BlogPostBriefReader reads one post by slug, for Regenerate, Upload and Remove (set by the router).
type BlogPostBriefReader interface {
	FindBySlug(ctx context.Context, slug string) (*models.BlogPostWithAuthor, error)
}

// coverUploadMax bounds an author's cover file.
const coverUploadMax = 2 << 20

// Upload handles PUT /v1/blog/{slug}/cover: the author stores their own cover (multipart
// field "image"; JPEG, PNG or WebP by its bytes, at most 2 MB). It replaces a drawn one.
func (h *BlogCoverHandler) Upload(w http.ResponseWriter, r *http.Request) {
	post, ok := h.ownPost(w, r)
	if !ok {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, coverUploadMax+64<<10) // the file plus the multipart framing
	if err := r.ParseMultipartForm(coverUploadMax); err != nil {
		writeBlogError(w, http.StatusBadRequest, "VALIDATION_ERROR", "send the cover as multipart form data in the field \"image\", at most 2 MB")
		return
	}
	file, header, err := r.FormFile("image")
	if err != nil {
		writeBlogError(w, http.StatusBadRequest, "VALIDATION_ERROR", "the multipart field \"image\" is missing")
		return
	}
	defer file.Close() //nolint:errcheck // a read-only multipart part
	if header.Size > coverUploadMax {
		writeBlogError(w, http.StatusRequestEntityTooLarge, "VALIDATION_ERROR", "the cover must be at most 2 MB")
		return
	}
	data, err := io.ReadAll(io.LimitReader(file, coverUploadMax+1))
	if err != nil || len(data) == 0 {
		writeBlogError(w, http.StatusBadRequest, "VALIDATION_ERROR", "the cover could not be read")
		return
	}
	if len(data) > coverUploadMax {
		writeBlogError(w, http.StatusRequestEntityTooLarge, "VALIDATION_ERROR", "the cover must be at most 2 MB")
		return
	}
	contentType := http.DetectContentType(data)
	switch contentType {
	case "image/jpeg", "image/png", "image/webp":
	default:
		writeBlogError(w, http.StatusUnsupportedMediaType, "VALIDATION_ERROR", "the cover must be a JPEG, PNG or WebP image")
		return
	}
	if err := h.store.Save(r.Context(), post.ID, &db.BlogCover{Image: data, ContentType: contentType, Model: "author", Prompt: ""}); err != nil {
		slog.Error("blog cover: upload not stored", "post_id", post.ID, "error", err)
		writeBlogError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to store the cover")
		return
	}
	url := h.coverURL(post.Slug)
	if err := h.store.NameCover(r.Context(), post.ID, url); err != nil {
		slog.Error("blog cover: upload not named", "post_id", post.ID, "error", err)
		writeBlogError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to name the cover on the post")
		return
	}
	writeBlogJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"slug": post.Slug, "cover_image_url": url, "content_type": contentType, "bytes": len(data)}})
}

// Remove handles DELETE /v1/blog/{slug}/cover: the author removes the stored cover.
func (h *BlogCoverHandler) Remove(w http.ResponseWriter, r *http.Request) {
	post, ok := h.ownPost(w, r)
	if !ok {
		return
	}
	if err := h.store.Delete(r.Context(), post.ID, h.coverURL(post.Slug)); err != nil {
		slog.Error("blog cover: not removed", "post_id", post.ID, "error", err)
		writeBlogError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to remove the cover")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// coverURL is the address the post's cover is served at.
func (h *BlogCoverHandler) coverURL(slug string) string {
	if h.maker != nil {
		return h.maker.CoverURL(slug)
	}
	return h.publicAPI + "/v1/blog/" + slug + "/cover.jpg"
}

// SetPublicAPI names the API's public origin for cover URLs when no maker is configured.
func (h *BlogCoverHandler) SetPublicAPI(origin string) { h.publicAPI = origin }

// ownPost reads the post of the route and checks the caller is its author; it writes the
// refusal itself and answers ok=false.
func (h *BlogCoverHandler) ownPost(w http.ResponseWriter, r *http.Request) (*models.BlogPostWithAuthor, bool) {
	authInfo := GetAuthInfo(r)
	if authInfo == nil {
		writeBlogError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return nil, false
	}
	if h.posts == nil {
		writeBlogError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "no post reader")
		return nil, false
	}
	post, err := h.posts.FindBySlug(r.Context(), chi.URLParam(r, "slug"))
	if errors.Is(err, db.ErrBlogPostNotFound) {
		writeBlogError(w, http.StatusNotFound, "NOT_FOUND", "blog post not found")
		return nil, false
	}
	if err != nil {
		slog.Error("blog cover: read post failed", "error", err)
		writeBlogError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to read the post")
		return nil, false
	}
	if post.PostedByType != authInfo.AuthorType || post.PostedByID != authInfo.AuthorID {
		writeBlogError(w, http.StatusForbidden, "FORBIDDEN", "you can only change the cover of your own blog posts")
		return nil, false
	}
	return post, true
}

// SetPostReader gives Regenerate the post store.
func (h *BlogCoverHandler) SetPostReader(posts BlogPostBriefReader) { h.posts = posts }

func (h *BlogCoverHandler) briefFor(ctx context.Context, slug string) (*db.BlogCoverBrief, error) {
	if h.posts == nil {
		return nil, errors.New("no post reader")
	}
	post, err := h.posts.FindBySlug(ctx, slug)
	if err != nil {
		return nil, err
	}
	if post.Status != models.BlogPostStatusPublished {
		return nil, db.ErrBlogPostNotFound
	}
	return &db.BlogCoverBrief{ID: post.ID, Slug: post.Slug, Title: post.Title, Excerpt: post.Excerpt, Tags: post.Tags}, nil
}

type backfillRequest struct {
	Limit  int  `json:"limit"`
	DryRun bool `json:"dry_run"`
}

// backfillMax bounds one call: each cover takes about a minute to draw.
const backfillMax = 40

// Backfill handles POST /admin/blog/covers/backfill.
func (h *BlogCoverHandler) Backfill(w http.ResponseWriter, r *http.Request) {
	if h.maker == nil {
		writeOperatorError(w, http.StatusServiceUnavailable, "COVERS_NOT_CONFIGURED", "no image service: set OPENAI_API_KEY")
		return
	}
	req := backfillRequest{Limit: 10}
	if r.Body != nil && r.ContentLength != 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeOperatorError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid request body")
			return
		}
	}
	if req.Limit <= 0 || req.Limit > backfillMax {
		writeOperatorError(w, http.StatusBadRequest, "VALIDATION_ERROR", fmt.Sprintf("limit must be between 1 and %d", backfillMax))
		return
	}
	briefs, err := h.store.ListPublishedWithoutCover(r.Context(), 0, req.Limit) // the operator waits for no grace
	if err != nil {
		slog.Error("blog cover backfill: list failed", "error", err)
		writeOperatorError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to list the posts without a cover")
		return
	}
	slugs := make([]string, 0, len(briefs))
	for _, b := range briefs {
		slugs = append(slugs, b.Slug)
	}
	if !req.DryRun && len(briefs) > 0 {
		go func(briefs []db.BlogCoverBrief) {
			for i := range briefs {
				h.maker.runOne(&briefs[i])
			}
		}(briefs)
	}
	writeActivationJSON(w, http.StatusAccepted, map[string]any{"data": map[string]any{
		"dry_run": req.DryRun, "queued": len(slugs), "slugs": slugs,
	}})
}
