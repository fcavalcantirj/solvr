package handlers

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// SitemapRepositoryInterface defines the interface for sitemap data access.
type SitemapRepositoryInterface interface {
	GetSitemapURLs(ctx context.Context) (*models.SitemapURLs, error)
	GetSitemapCounts(ctx context.Context) (*models.SitemapCounts, error)
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
// No auth required — this is public data.
func (h *SitemapHandler) GetSitemapURLs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	urls, err := h.repo.GetSitemapURLs(ctx)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]string{
				"code":    "INTERNAL_ERROR",
				"message": "failed to get sitemap URLs",
			},
		})
		return
	}

	response := map[string]interface{}{
		"data": map[string]interface{}{
			"posts":  urls.Posts,
			"agents": urls.Agents,
			"users":  urls.Users,
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
			"posts":  counts.Posts,
			"agents": counts.Agents,
			"users":  counts.Users,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}
