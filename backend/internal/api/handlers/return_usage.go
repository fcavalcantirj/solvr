package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/fcavalcantirj/solvr/internal/db"
)

// GET /admin/return-usage?window=24h|7d|30d — return usage (idx 92): the time between useful
// collaborations (agents and humans apart), resumed rooms, what became of new rooms (completed
// one-off vs onboarding failure), opt-in notifications sent (never a growth metric) and
// language demand from search. Operator-only, like every operator report.

// ReturnUsageReader is the read side the handler needs.
type ReturnUsageReader interface {
	Measure(ctx context.Context, from, to, now time.Time) (db.ReturnUsageReport, error)
}

// ReturnUsageHandler serves the operator return-usage report.
type ReturnUsageHandler struct {
	repo ReturnUsageReader
}

// NewReturnUsageHandler creates the return-usage handler.
func NewReturnUsageHandler(repo ReturnUsageReader) *ReturnUsageHandler {
	return &ReturnUsageHandler{repo: repo}
}

// GetReport handles GET /admin/return-usage.
func (h *ReturnUsageHandler) GetReport(w http.ResponseWriter, r *http.Request) {
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
	report, err := h.repo.Measure(r.Context(), now.Add(-window.Duration), now, now)
	if err != nil {
		writeOperatorError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to measure return usage")
		return
	}
	type returnResponse struct {
		Window     string `json:"window"`
		WindowText string `json:"window_text"`
		db.ReturnUsageReport
	}
	writeActivationJSON(w, http.StatusOK, map[string]any{
		"data": returnResponse{Window: window.Value, WindowText: window.WindowText, ReturnUsageReport: report},
	})
}
