package handlers

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/fcavalcantirj/solvr/internal/growth"
)

// The operator growth reports.
//
//	GET /admin/growth/participants?end=<RFC3339>  monthly active participants (spec.json idx 86)
//
// Each report is Solvr reporting about ITSELF — participant counts, traffic, the one-million
// target — so it is operator analytics: the router gates it with RequireOperatorAccess, this
// handler checks the operator key again, and every answer is unstorable. The db readers measure
// raw figures from real tables; the growth package owns every definition and evaluation, and
// the handler only supplies the window end.

// ParticipantReader measures monthly active participants for the window ending at end.
type ParticipantReader interface {
	Measure(ctx context.Context, end time.Time) (growth.ParticipantMeasures, error)
}

// GrowthReaders are the measurements the growth reports read.
type GrowthReaders struct {
	Participants ParticipantReader
}

// GrowthReportsHandler serves the operator growth reports.
type GrowthReportsHandler struct {
	readers GrowthReaders
}

// NewGrowthReportsHandler creates the growth reports handler.
func NewGrowthReportsHandler(readers GrowthReaders) *GrowthReportsHandler {
	return &GrowthReportsHandler{readers: readers}
}

// GetParticipants handles GET /admin/growth/participants. end defaults to now; the report
// covers the rolling 30 days before it and the 30 days before those.
func (h *GrowthReportsHandler) GetParticipants(w http.ResponseWriter, r *http.Request) {
	end, ok := h.authorizeAndReadEnd(w, r)
	if !ok {
		return
	}
	m, err := h.readers.Participants.Measure(r.Context(), end)
	if err != nil {
		slog.Error("growth participants report failed", "error", err)
		writeOperatorError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to compute the participant report")
		return
	}
	writeActivationJSON(w, http.StatusOK, map[string]any{
		"data": growth.BuildParticipantReport(m, growth.MonthlyActiveParticipantGoal),
	})
}

// authorizeAndReadEnd applies the private cache policy, checks the operator key and reads the
// optional end instant. It answers the refusal itself and reports whether to continue.
func (h *GrowthReportsHandler) authorizeAndReadEnd(w http.ResponseWriter, r *http.Request) (time.Time, bool) {
	ApplyOperatorReportCachePolicy(w)
	if !checkActivationAnalyticsAuth(w, r) {
		return time.Time{}, false
	}
	raw := r.URL.Query().Get("end")
	if raw == "" {
		return time.Now().UTC().Truncate(time.Second), true
	}
	end, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		writeOperatorError(w, http.StatusBadRequest, "INVALID_END", "end must be an RFC3339 timestamp")
		return time.Time{}, false
	}
	return end.UTC(), true
}
