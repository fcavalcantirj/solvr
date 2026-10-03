package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/fcavalcantirj/solvr/internal/ops"
)

// SLOReader is the read side of the operations report: observations only. Every
// judgement against a target is made by ops.BuildSLOReport.
type SLOReader interface {
	ObserveSLO(ctx context.Context, start, end, now time.Time) (ops.SLOObservations, error)
}

// OpsSLOHandler serves the reliability gates of spec.json idx 79: monthly core-API
// availability, p95 ordinary read, p95 accepted timeline write and p95 connected-client
// delivery, each met, unmet or not_yet_measurable with what is missing named, plus the
// separately measured external-model latency and the delivery queue lag. Operator-only:
// the router gates it and the handler keeps its own key check as a second lock.
type OpsSLOHandler struct {
	repo SLOReader
}

// NewOpsSLOHandler creates the operations report handler.
func NewOpsSLOHandler(repo SLOReader) *OpsSLOHandler {
	return &OpsSLOHandler{repo: repo}
}

// GetSLO handles GET /admin/ops/slo[?end=<RFC3339>]. The window is the 30 days ending
// at end (default now); the queue is read as of end too.
func (h *OpsSLOHandler) GetSLO(w http.ResponseWriter, r *http.Request) {
	if !checkActivationAnalyticsAuth(w, r) {
		return
	}

	end := time.Now().UTC()
	if raw := r.URL.Query().Get("end"); raw != "" {
		parsed, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			writeOperatorError(w, http.StatusBadRequest, "INVALID_END", "end must be an RFC3339 timestamp")
			return
		}
		end = parsed.UTC()
	}

	obs, err := h.repo.ObserveSLO(r.Context(), end.Add(-ops.AvailabilityWindow), end, end)
	if err != nil {
		writeOperatorError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to read the operations report")
		return
	}

	writeActivationJSON(w, http.StatusOK, map[string]any{"data": ops.BuildSLOReport(obs)})
}
