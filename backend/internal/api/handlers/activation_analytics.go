package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"time"

	"github.com/fcavalcantirj/solvr/internal/db"
)

// ActivationReader is the read side of activation measurement the handler needs.
// The repository owns every definition and computation; the handler only chooses
// a window and serves the result.
type ActivationReader interface {
	Measure(ctx context.Context, from, to time.Time) (db.ActivationReport, error)
}

// ActivationAnalyticsHandler serves the operator activation report: how many
// rooms were created and activated, how long the milestones took, and how the
// funnel converted by origin. It is operator-only — the router gates it and this
// handler keeps its own key check as a second lock — because it is Solvr
// reporting about itself, not a product feature.
type ActivationAnalyticsHandler struct {
	repo ActivationReader
}

// NewActivationAnalyticsHandler creates the activation analytics handler.
func NewActivationAnalyticsHandler(repo ActivationReader) *ActivationAnalyticsHandler {
	return &ActivationAnalyticsHandler{repo: repo}
}

// defaultActivationWindow is what the report covers when no window is given.
// Activation is a slower signal than live presence, so it defaults to 30 days
// rather than the homepage's 24-hour default.
const defaultActivationWindow = "30d"

// GetReport handles GET /admin/activation-analytics?window=24h|7d|30d.
func (h *ActivationAnalyticsHandler) GetReport(w http.ResponseWriter, r *http.Request) {
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

	to := time.Now().UTC()
	from := to.Add(-window.Duration)

	report, err := h.repo.Measure(r.Context(), from, to)
	if err != nil {
		writeOperatorError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to measure activation")
		return
	}

	// Embed the report so its documented json fields sit beside the window the
	// caller selected, under one data envelope.
	type activationResponse struct {
		Window     string `json:"window"`
		WindowText string `json:"window_text"`
		db.ActivationReport
	}
	writeActivationJSON(w, http.StatusOK, map[string]any{
		"data": activationResponse{
			Window:          window.Value,
			WindowText:      window.WindowText,
			ActivationReport: report,
		},
	})
}

// checkActivationAnalyticsAuth is the handler's own operator-key check, mirroring
// the gate the router already applies, so the report is never served on a key
// misconfiguration and the handler is safe on its own.
func checkActivationAnalyticsAuth(w http.ResponseWriter, r *http.Request) bool {
	configured := os.Getenv("ADMIN_API_KEY")
	if configured == "" {
		writeOperatorError(w, http.StatusServiceUnavailable, "ADMIN_NOT_CONFIGURED", "admin API key not configured")
		return false
	}
	provided := r.Header.Get(OperatorAccessHeader)
	if provided == "" {
		writeOperatorError(w, http.StatusUnauthorized, "MISSING_API_KEY", OperatorAccessHeader+" header required")
		return false
	}
	if provided != configured {
		writeOperatorError(w, http.StatusForbidden, "INVALID_API_KEY", "invalid admin API key")
		return false
	}
	return true
}

// writeActivationJSON writes an operator report and marks it unstorable so no
// cache can hold one caller's answer and hand it to another.
func writeActivationJSON(w http.ResponseWriter, status int, payload any) {
	ApplyOperatorReportCachePolicy(w)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
