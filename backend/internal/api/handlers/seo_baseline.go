package handlers

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/fcavalcantirj/solvr/internal/db"
)

// SEOBaselineReader measures the server-side SEO baseline (db.SEOBaselineRepository).
type SEOBaselineReader interface {
	Measure(ctx context.Context, from, to time.Time, historyPageSize int) (*db.SEOBaseline, error)
}

// SEOBaselineHandler serves GET /admin/seo/baseline?window=24h|7d|30d (task idx 85,
// SPEC.md Part 27.6): the weekly SEO baseline the server can measure, operator-only.
type SEOBaselineHandler struct {
	repo SEOBaselineReader
}

// NewSEOBaselineHandler creates an SEOBaselineHandler.
func NewSEOBaselineHandler(repo SEOBaselineReader) *SEOBaselineHandler {
	return &SEOBaselineHandler{repo: repo}
}

// SEOMilestone is one of the distinct steps from publishing a page to activated
// participants. A milestone the server cannot measure says where its data lives.
type SEOMilestone struct {
	Name   string `json:"name"`
	Status string `json:"status"` // measured | unavailable
	Value  *int   `json:"value,omitempty"`
	Source string `json:"source"`
}

// GetBaseline handles GET /admin/seo/baseline.
func (h *SEOBaselineHandler) GetBaseline(w http.ResponseWriter, r *http.Request) {
	value := r.URL.Query().Get("window")
	if value == "" {
		value = "7d"
	}
	window, ok := db.RoomStatsWindowByValue(value)
	if !ok {
		writeOperatorError(w, http.StatusBadRequest, "INVALID_WINDOW", "window must be one of 24h, 7d, 30d")
		return
	}
	to := time.Now().UTC()
	b, err := h.repo.Measure(r.Context(), to.Add(-window.Duration), to, HistoryPageSize)
	if err != nil {
		slog.Error("seo baseline failed", "error", err)
		writeOperatorError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to measure the SEO baseline")
		return
	}
	published := b.Indexable.Posts + b.Indexable.Rooms + b.Indexable.RoomHistoryPages + b.Indexable.Agents + b.Indexable.BlogPosts
	activated := b.Activation.RoomsActivated
	gsc := func(name, report string) SEOMilestone {
		return SEOMilestone{Name: name, Status: "unavailable", Source: report}
	}
	milestones := []SEOMilestone{
		{Name: "publication", Status: "measured", Value: &published, Source: "indexable content pages now (static routes excluded)"},
		gsc("sitemap_acceptance", "Google Search Console: Sitemaps"),
		gsc("crawling", "Google Search Console: Crawl stats"),
		gsc("indexing", "Google Search Console: Page indexing"),
		gsc("traffic", "Google Search Console: Performance (cmd/seo-report joins the export)"),
		gsc("core_web_vitals", "Google Search Console: Core Web Vitals (Chrome UX Report)"),
		{Name: "activation", Status: "measured", Value: &activated, Source: "rooms activated in the window; distinct participants: /admin/growth/participants"},
	}
	writeActivationJSON(w, http.StatusOK, map[string]any{"data": map[string]any{
		"window":     window.Value,
		"from":       b.From,
		"to":         b.To,
		"indexable":  b.Indexable,
		"lastmod":    b.Lastmod,
		"activation": b.Activation,
		"funnel":     b.Funnel,
		"landings":   b.Landings,
		"search":     b.Search,
		"milestones": milestones,
	}})
}
