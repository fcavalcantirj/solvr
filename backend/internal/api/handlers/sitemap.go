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
