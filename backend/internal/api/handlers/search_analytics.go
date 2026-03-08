package handlers

import (
	"context"
	"net/http"
	"os"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// SearchAnalyticsReaderInterface defines the read operations for search analytics.
type SearchAnalyticsReaderInterface interface {
	GetTrending(ctx context.Context, days int, limit int) ([]models.TrendingSearch, error)
	GetZeroResults(ctx context.Context, days int, limit int) ([]models.TrendingSearch, error)
	GetSummary(ctx context.Context, days int) (models.SearchAnalytics, error)
}

// SearchAnalyticsHandler handles admin search analytics endpoints.
type SearchAnalyticsHandler struct {
	repo SearchAnalyticsReaderInterface
}

// NewSearchAnalyticsHandler creates a new SearchAnalyticsHandler.
func NewSearchAnalyticsHandler(repo SearchAnalyticsReaderInterface) *SearchAnalyticsHandler {
	return &SearchAnalyticsHandler{repo: repo}
}

// GetTrending handles GET /admin/search-analytics/trending
// Query params: days (default 7), limit (default 20)
func (h *SearchAnalyticsHandler) GetTrending(w http.ResponseWriter, r *http.Request) {
	if !checkSearchAnalyticsAuth(w, r) {
		return
	}

	days := parseIntParam(r.URL.Query().Get("days"), 7)
	limit := parseIntParam(r.URL.Query().Get("limit"), 20)

	trending, err := h.repo.GetTrending(r.Context(), days, limit)
	if err != nil {
		writeSearchError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get trending searches")
		return
	}

	zeroResults, err := h.repo.GetZeroResults(r.Context(), days, limit)
	if err != nil {
		writeSearchError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get zero-result searches")
		return
	}

	writeSearchJSON(w, http.StatusOK, map[string]any{
		"trending":     trending,
		"zero_results": zeroResults,
		"days":         days,
	})
}

// GetSummary handles GET /admin/search-analytics/summary
// Query params: days (default 30)
func (h *SearchAnalyticsHandler) GetSummary(w http.ResponseWriter, r *http.Request) {
	if !checkSearchAnalyticsAuth(w, r) {
		return
	}

	days := parseIntParam(r.URL.Query().Get("days"), 30)

	summary, err := h.repo.GetSummary(r.Context(), days)
	if err != nil {
		writeSearchError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get search analytics summary")
		return
	}

	writeSearchJSON(w, http.StatusOK, map[string]any{
		"summary": summary,
		"days":    days,
	})
}

// checkSearchAnalyticsAuth validates the X-Admin-API-Key header.
func checkSearchAnalyticsAuth(w http.ResponseWriter, r *http.Request) bool {
	adminKey := os.Getenv("ADMIN_API_KEY")
	if adminKey == "" {
		writeSearchError(w, http.StatusServiceUnavailable, "ADMIN_NOT_CONFIGURED", "admin API key not configured")
		return false
	}

	providedKey := r.Header.Get("X-Admin-API-Key")
	if providedKey == "" {
		writeSearchError(w, http.StatusUnauthorized, "MISSING_API_KEY", "X-Admin-API-Key header required")
		return false
	}

	if providedKey != adminKey {
		writeSearchError(w, http.StatusForbidden, "INVALID_API_KEY", "invalid admin API key")
		return false
	}

	return true
}
