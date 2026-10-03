package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/fcavalcantirj/solvr/internal/db"
)

// GET /admin/share-attribution?window=24h|7d|30d — the share loop (idx 88 steps 5-6):
// share visits, invitations, attributed rooms and activations, new agents and humans
// kept apart, their 7- and 28-day returns, invitations per activated origin, invite-to-
// activation conversion and the k experiment metric. Operator-only: the router gates it
// and the handler keeps its own key check, like every operator report.

// ShareAttributionReader is the read side the handler needs; the repository owns every
// definition and computation.
type ShareAttributionReader interface {
	Measure(ctx context.Context, from, to, now time.Time) (db.ShareAttributionReport, error)
}

// ShareAttributionHandler serves the operator share-attribution report.
type ShareAttributionHandler struct {
	repo ShareAttributionReader
}

// NewShareAttributionHandler creates the share-attribution handler.
func NewShareAttributionHandler(repo ShareAttributionReader) *ShareAttributionHandler {
	return &ShareAttributionHandler{repo: repo}
}

// GetReport handles GET /admin/share-attribution.
func (h *ShareAttributionHandler) GetReport(w http.ResponseWriter, r *http.Request) {
	if !checkActivationAnalyticsAuth(w, r) {
		return
	}
	value := r.URL.Query().Get("window")
	if value == "" {
		value = defaultActivationWindow
	}
	window, ok := db.RoomStatsWindowByValue(value)
	if !ok {
		writeOperatorError(w, http.StatusBadRequest, "INVALID_WINDOW", "window must be one of 24h, 7d, 30d")
		return
	}
	now := time.Now().UTC()
	from := now.Add(-window.Duration)

	report, err := h.repo.Measure(r.Context(), from, now, now)
	if err != nil {
		writeOperatorError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to measure share attribution")
		return
	}
	type shareResponse struct {
		Window     string `json:"window"`
		WindowText string `json:"window_text"`
		db.ShareAttributionReport
	}
	writeActivationJSON(w, http.StatusOK, map[string]any{
		"data": shareResponse{Window: window.Value, WindowText: window.WindowText, ShareAttributionReport: report},
	})
}
