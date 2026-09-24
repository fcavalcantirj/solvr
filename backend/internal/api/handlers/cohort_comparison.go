package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/fcavalcantirj/solvr/internal/db"
)

// CohortReader is the read side of cohort comparison the handler needs. The
// repository owns every definition and computation; the handler only supplies
// the launch instant and the current time.
type CohortReader interface {
	Compare(ctx context.Context, launch, now time.Time) (db.CohortComparisonReport, error)
}

// CohortComparisonHandler serves the operator post-launch cohort comparison:
// consistent 7-day and 28-day windows anchored to one launch timestamp, so the
// redesign can be judged without a moving all-time baseline. It is operator-only
// — the router gates it and this handler keeps its own key check as a second lock
// — because it is Solvr reporting about itself, not a product feature.
type CohortComparisonHandler struct {
	repo CohortReader
}

// NewCohortComparisonHandler creates the cohort comparison handler.
func NewCohortComparisonHandler(repo CohortReader) *CohortComparisonHandler {
	return &CohortComparisonHandler{repo: repo}
}

// GetReport handles GET /admin/cohort-comparison?launch=<RFC3339>. The launch
// timestamp is required: the comparison is meaningless without the instant its
// windows are measured from, so there is no default to guess.
func (h *CohortComparisonHandler) GetReport(w http.ResponseWriter, r *http.Request) {
	if !checkActivationAnalyticsAuth(w, r) {
		return
	}

	launchParam := r.URL.Query().Get("launch")
	if launchParam == "" {
		writeOperatorError(w, http.StatusBadRequest, "MISSING_LAUNCH", "launch query parameter (RFC3339) is required")
		return
	}
	launch, err := time.Parse(time.RFC3339, launchParam)
	if err != nil {
		writeOperatorError(w, http.StatusBadRequest, "INVALID_LAUNCH", "launch must be an RFC3339 timestamp")
		return
	}

	report, err := h.repo.Compare(r.Context(), launch, time.Now().UTC())
	if err != nil {
		writeOperatorError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to compute cohort comparison")
		return
	}

	writeActivationJSON(w, http.StatusOK, map[string]any{"data": report})
}
