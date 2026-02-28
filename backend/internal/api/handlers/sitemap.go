package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/fcavalcantirj/solvr/internal/models"
)

const (
	// SitemapMaxPerPage is the maximum allowed per_page value for paginated sitemap queries.
	SitemapMaxPerPage = 5000
	// SitemapDefaultPerPage is the default per_page value when not specified.
	SitemapDefaultPerPage = 2500
)

// SitemapRepositoryInterface defines the interface for sitemap data access.
type SitemapRepositoryInterface interface {
	GetSitemapURLs(ctx context.Context) (*models.SitemapURLs, error)
	GetSitemapCounts(ctx context.Context) (*models.SitemapCounts, error)
	GetPaginatedSitemapURLs(ctx context.Context, opts models.SitemapURLsOptions) (*models.SitemapURLs, error)
}

// SitemapHandler handles sitemap endpoints.
type SitemapHandler struct {
	repo SitemapRepositoryInterface
}

// NewSitemapHandler creates a new SitemapHandler.
func NewSitemapHandler(repo SitemapRepositoryInterface) *SitemapHandler {
	return &SitemapHandler{repo: repo}
}

// GetSitemapURLs handles GET /v1/sitemap/urls
// Returns all indexable content URLs for sitemap generation.
// Supports optional pagination via type, page, per_page query params.
// No auth required — this is public data.
func (h *SitemapHandler) GetSitemapURLs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	typeParam := r.URL.Query().Get("type")

	// If type param is present, use paginated path
	if typeParam != "" {
		h.getSitemapURLsPaginated(w, r, ctx, typeParam)
		return
	}

	// Backward compat: no type param → return all URLs
	urls, err := h.repo.GetSitemapURLs(ctx)
	if err != nil {
		writeSitemapError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get sitemap URLs")
		return
	}

	writeSitemapURLsResponse(w, urls)
}

func (h *SitemapHandler) getSitemapURLsPaginated(w http.ResponseWriter, r *http.Request, ctx context.Context, typeParam string) {
	// Validate type
	validTypes := map[string]bool{"posts": true, "agents": true, "users": true, "blog_posts": true}
	if !validTypes[typeParam] {
		writeSitemapError(w, http.StatusBadRequest, "INVALID_PARAM", "type must be one of: posts, agents, users, blog_posts")
		return
	}

	// Parse page (default 1)
	page := 1
	if p := r.URL.Query().Get("page"); p != "" {
		var err error
		page, err = strconv.Atoi(p)
		if err != nil || page < 1 {
			writeSitemapError(w, http.StatusBadRequest, "INVALID_PARAM", "page must be a positive integer")
			return
		}
	}

	// Parse per_page (default SitemapDefaultPerPage)
	perPage := SitemapDefaultPerPage
	if pp := r.URL.Query().Get("per_page"); pp != "" {
		var err error
		perPage, err = strconv.Atoi(pp)
		if err != nil || perPage < 1 || perPage > SitemapMaxPerPage {
			writeSitemapError(w, http.StatusBadRequest, "INVALID_PARAM", "per_page must be between 1 and 5000")
			return
		}
	}

	opts := models.SitemapURLsOptions{
		Type:    typeParam,
		Page:    page,
		PerPage: perPage,
	}

	urls, err := h.repo.GetPaginatedSitemapURLs(ctx, opts)
	if err != nil {
		writeSitemapError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get sitemap URLs")
		return
	}

	writeSitemapURLsResponse(w, urls)
}

func writeSitemapError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]string{
			"code":    code,
			"message": message,
		},
	})
}

func writeSitemapURLsResponse(w http.ResponseWriter, urls *models.SitemapURLs) {
	response := map[string]interface{}{
		"data": map[string]interface{}{
			"posts":      urls.Posts,
			"agents":     urls.Agents,
			"users":      urls.Users,
			"blog_posts": urls.BlogPosts,
		},
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// GetSitemapCounts handles GET /v1/sitemap/counts
// Returns counts of indexable content per type.
// No auth required — this is public data.
func (h *SitemapHandler) GetSitemapCounts(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	counts, err := h.repo.GetSitemapCounts(ctx)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]string{
				"code":    "INTERNAL_ERROR",
				"message": "failed to get sitemap counts",
			},
		})
		return
	}

	response := map[string]interface{}{
		"data": map[string]interface{}{
			"posts":      counts.Posts,
			"agents":     counts.Agents,
			"users":      counts.Users,
			"blog_posts": counts.BlogPosts,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}
