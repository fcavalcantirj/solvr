package handlers

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// DataAnalyticsReaderInterface defines the read operations for public data analytics.
type DataAnalyticsReaderInterface interface {
	GetTrendingPublic(ctx context.Context, window string, limit int, excludeBots bool) ([]models.TrendingSearch, error)
	GetBreakdown(ctx context.Context, window string, excludeBots bool) (models.DataBreakdown, error)
	GetCategories(ctx context.Context, window string, excludeBots bool) ([]models.DataCategory, error)
}

// DataHandler handles the public /v1/data/* analytics endpoints.
type DataHandler struct {
	repo  DataAnalyticsReaderInterface
	cache sync.Map
}

// cachedEntry holds a cached response value with an expiry time.
type cachedEntry struct {
	data      interface{}
	expiresAt time.Time
}

// dataCacheTTL is the server-side cache duration for /v1/data/* endpoints.
// Prevents DB hammering from repeated public polling (T-17-06 DoS mitigation).
const dataCacheTTL = 60 * time.Second

// validWindows is the whitelist of accepted window param values (T-17-05 tamper mitigation).
var validWindows = map[string]bool{
	"1h":  true,
	"24h": true,
	"7d":  true,
}

// NewDataHandler creates a new DataHandler.
func NewDataHandler(repo DataAnalyticsReaderInterface) *DataHandler {
	return &DataHandler{repo: repo}
}

// getCached retrieves from cache or calls fetch(), stores result on miss.
func (h *DataHandler) getCached(key string, fetch func() (interface{}, error)) (interface{}, error) {
	if v, ok := h.cache.Load(key); ok {
		entry := v.(cachedEntry)
		if time.Now().Before(entry.expiresAt) {
			return entry.data, nil
		}
	}
	data, err := fetch()
	if err != nil {
		return nil, err
	}
	h.cache.Store(key, cachedEntry{data: data, expiresAt: time.Now().Add(dataCacheTTL)})
	return data, nil
}

// parseWindowParam extracts and validates the window query param.
// Returns "24h" by default if absent. Returns ("", false) if invalid.
func parseWindowParam(r *http.Request) (string, bool) {
	w := r.URL.Query().Get("window")
	if w == "" {
		return "24h", true
	}
	if !validWindows[w] {
		return "", false
	}
	return w, true
}

// parseIncludeBots returns true if include_bots=true is set in query params.
func parseIncludeBots(r *http.Request) bool {
	return r.URL.Query().Get("include_bots") == "true"
}

// publicTrending is a stripped-down trending query for the public endpoint.
// avg_results and avg_duration_ms are intentionally excluded (T-17-07).
type publicTrending struct {
	Query string `json:"query"`
	Count int    `json:"count"`
}

// GetTrending handles GET /v1/data/trending
// Returns top trending search queries for the given time window.
// Public endpoint — no auth required (T-17-08).
func (h *DataHandler) GetTrending(w http.ResponseWriter, r *http.Request) {
	window, ok := parseWindowParam(r)
	if !ok {
		writeSearchError(w, http.StatusBadRequest, "INVALID_PARAM", "window must be one of: 1h, 24h, 7d")
		return
	}
	includeBots := parseIncludeBots(r)
	cacheKey := fmt.Sprintf("trending:%s:%v", window, includeBots)

	data, err := h.getCached(cacheKey, func() (interface{}, error) {
		results, err := h.repo.GetTrendingPublic(r.Context(), window, 10, !includeBots)
		if err != nil {
			return nil, err
		}
		public := make([]publicTrending, 0, len(results))
		for _, t := range results {
			public = append(public, publicTrending{Query: t.Query, Count: t.Count})
		}
		return public, nil
	})
	if err != nil {
		writeSearchError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get trending queries")
		return
	}

	writeSearchJSON(w, http.StatusOK, map[string]any{
		"data": map[string]any{
			"trending": data,
			"window":   window,
		},
	})
}

// GetBreakdown handles GET /v1/data/breakdown
// Returns total searches, zero_result_rate, and by_searcher_type breakdown.
// Public endpoint — no auth required (T-17-08).
func (h *DataHandler) GetBreakdown(w http.ResponseWriter, r *http.Request) {
	window, ok := parseWindowParam(r)
	if !ok {
		writeSearchError(w, http.StatusBadRequest, "INVALID_PARAM", "window must be one of: 1h, 24h, 7d")
		return
	}
	includeBots := parseIncludeBots(r)
	cacheKey := fmt.Sprintf("breakdown:%s:%v", window, includeBots)

	data, err := h.getCached(cacheKey, func() (interface{}, error) {
		return h.repo.GetBreakdown(r.Context(), window, !includeBots)
	})
	if err != nil {
		writeSearchError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get search breakdown")
		return
	}

	bd := data.(models.DataBreakdown)
	writeSearchJSON(w, http.StatusOK, map[string]any{
		"data": map[string]any{
			"total_searches":   bd.TotalSearches,
			"zero_result_rate": bd.ZeroResultRate,
			"by_searcher_type": bd.BySearcherType,
			"window":           window,
		},
	})
}

// GetCategories handles GET /v1/data/categories
// Returns search counts grouped by type_filter (category).
// Public endpoint — no auth required (T-17-08).
func (h *DataHandler) GetCategories(w http.ResponseWriter, r *http.Request) {
	window, ok := parseWindowParam(r)
	if !ok {
		writeSearchError(w, http.StatusBadRequest, "INVALID_PARAM", "window must be one of: 1h, 24h, 7d")
		return
	}
	includeBots := parseIncludeBots(r)
	cacheKey := fmt.Sprintf("categories:%s:%v", window, includeBots)

	data, err := h.getCached(cacheKey, func() (interface{}, error) {
		return h.repo.GetCategories(r.Context(), window, !includeBots)
	})
	if err != nil {
		writeSearchError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get search categories")
		return
	}

	writeSearchJSON(w, http.StatusOK, map[string]any{
		"data": map[string]any{
			"categories": data,
			"window":     window,
		},
	})
}
