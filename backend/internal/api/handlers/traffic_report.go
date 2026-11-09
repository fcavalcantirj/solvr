package handlers

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/traffic"
)

// The traffic reports (SPEC.md 27.6), operator-only like every /admin route:
//
//	GET  /admin/traffic/summary?window=24h|7d|30d  who the window's traffic came from: page
//	                                               views by kind of page and class, funnel
//	                                               steps and searches by class, requests by
//	                                               client class, the top error codes
//	POST /admin/traffic/reclassify                 the behavioural rules over a window the
//	                                               operator names: {"from","to","dry_run",
//	                                               "legacy_page_views"}
//
// The reclassify call runs the same code path as the job that judges live rows every 15
// minutes (jobs.TrafficClassifyJob), one run at a time. A dry run answers what a run would set
// and writes nothing (REVIEW-r1 C4).

// reclassifyMaxWindow is the longest window one call judges.
const reclassifyMaxWindow = 366 * 24 * time.Hour

// reclassifyFutureSlack is how far past the server's clock "to" may be.
const reclassifyFutureSlack = time.Minute

// TrafficSummaryReader reads the traffic summary (db.TrafficReportRepository).
type TrafficSummaryReader interface {
	Summary(ctx context.Context, from, to time.Time) (*db.TrafficSummary, error)
}

// TrafficReclassifier runs the behavioural rules (db.TrafficRules).
type TrafficReclassifier interface {
	TryLock(ctx context.Context) (unlock func(), ok bool, err error)
	Apply(ctx context.Context, from, to time.Time, dryRun bool) (db.TrafficRun, error)
	// ApplyLegacyPageViewBurst judges the page views stored before source keys existed by
	// the minute's total (B4 legacy); it runs only when the operator asks.
	ApplyLegacyPageViewBurst(ctx context.Context, from, to time.Time, dryRun bool) (int, error)
}

// TrafficReportHandler serves the traffic reports.
type TrafficReportHandler struct {
	summary TrafficSummaryReader
	rules   TrafficReclassifier
}

// NewTrafficReportHandler creates the handler.
func NewTrafficReportHandler(summary TrafficSummaryReader, rules TrafficReclassifier) *TrafficReportHandler {
	return &TrafficReportHandler{summary: summary, rules: rules}
}

// GetSummary handles GET /admin/traffic/summary.
func (h *TrafficReportHandler) GetSummary(w http.ResponseWriter, r *http.Request) {
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
	s, err := h.summary.Summary(r.Context(), to.Add(-window.Duration), to)
	if err != nil {
		slog.Error("traffic summary failed", "error", err)
		writeOperatorError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to read the traffic summary")
		return
	}
	writeActivationJSON(w, http.StatusOK, map[string]any{"data": map[string]any{
		"window":    window.Value,
		"from":      s.From,
		"to":        s.To,
		"pageviews": s.PageViews,
		"funnel":    s.Funnel,
		"searches":  s.Searches,
		"requests":  s.Requests,
		"errors":    s.Errors,
	}})
}

// reclassifyRequest is the operator's window. With legacy_page_views the run also judges
// the keyless page views of the window by the minute's total (db.TrafficRules.
// ApplyLegacyPageViewBurst), the one-off backfill for rows stored before 000148.
type reclassifyRequest struct {
	From            string `json:"from"`
	To              string `json:"to"`
	DryRun          bool   `json:"dry_run"`
	LegacyPageViews bool   `json:"legacy_page_views"`
}

// Reclassify handles POST /admin/traffic/reclassify.
func (h *TrafficReportHandler) Reclassify(w http.ResponseWriter, r *http.Request) {
	var req reclassifyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeOperatorError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid request body")
		return
	}
	from, errFrom := time.Parse(time.RFC3339, req.From)
	to, errTo := time.Parse(time.RFC3339, req.To)
	switch {
	case errFrom != nil || errTo != nil:
		writeOperatorError(w, http.StatusBadRequest, "VALIDATION_ERROR", "from and to must be RFC 3339 times, such as 2026-10-01T00:00:00Z")
		return
	case !from.Before(to):
		writeOperatorError(w, http.StatusBadRequest, "VALIDATION_ERROR", "from must be before to")
		return
	case to.After(time.Now().Add(reclassifyFutureSlack)):
		writeOperatorError(w, http.StatusBadRequest, "VALIDATION_ERROR", "to must not be in the future")
		return
	case to.Sub(from) > reclassifyMaxWindow:
		writeOperatorError(w, http.StatusBadRequest, "VALIDATION_ERROR", "the window must be at most 366 days")
		return
	}

	unlock, ok, err := h.rules.TryLock(r.Context())
	if err != nil {
		slog.Error("traffic reclassify: lock failed", "error", err)
		writeOperatorError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to start the classification run")
		return
	}
	if !ok {
		writeOperatorError(w, http.StatusConflict, "CLASSIFICATION_RUNNING", "a classification run is in progress; retry in a minute")
		return
	}
	defer unlock()

	run, err := h.rules.Apply(r.Context(), from, to, req.DryRun)
	if err != nil {
		slog.Error("traffic reclassify failed", "error", err, "dry_run", req.DryRun)
		writeOperatorError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "the classification run failed; nothing was written")
		return
	}
	if req.LegacyPageViews {
		n, legacyErr := h.rules.ApplyLegacyPageViewBurst(r.Context(), from, to, req.DryRun)
		if legacyErr != nil {
			slog.Error("traffic reclassify: legacy page views failed", "error", legacyErr, "dry_run", req.DryRun)
			writeOperatorError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "the legacy page-view rule failed; the other rules were written")
			return
		}
		run.Rules[string(traffic.ReasonPageViewBurstLegacy)] = n
	}
	slog.Info("traffic reclassify", "from", run.From, "to", run.To, "dry_run", run.DryRun, "rules", run.Rules)
	writeActivationJSON(w, http.StatusOK, map[string]any{"data": run})
}
