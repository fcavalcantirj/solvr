package handlers

import (
	"context"
	"log/slog"
	"net/http"
	"regexp"
	"time"

	"github.com/fcavalcantirj/solvr/internal/growth"
)

// The operator growth reports.
//
//	GET /admin/growth/participants?end=<RFC3339>  monthly active participants (spec.json idx 86)
//	GET /admin/growth/stages?end=<RFC3339>        staged growth gates (spec.json idx 89)
//	GET /admin/growth/model?month=YYYY-MM         the monthly acquisition model (spec.json idx 90)
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

// StageReader measures the stage gates for the window ending at end.
type StageReader interface {
	Measure(ctx context.Context, end time.Time) (growth.StageMeasures, error)
}

// ModelReader reads observed monthly flows for the month starting at monthStart.
type ModelReader interface {
	MonthlyFlows(ctx context.Context, monthStart time.Time) (growth.MonthlyFlows, error)
}

// GrowthReaders are the measurements the growth reports read.
type GrowthReaders struct {
	Participants ParticipantReader
	Stages       StageReader
	Model        ModelReader
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

// GetStages handles GET /admin/growth/stages: the four stages and their separately verifiable
// gates for the window ending at end (default now). The participant gates read the same counter
// as GetParticipants.
func (h *GrowthReportsHandler) GetStages(w http.ResponseWriter, r *http.Request) {
	end, ok := h.authorizeAndReadEnd(w, r)
	if !ok {
		return
	}
	p, err := h.readers.Participants.Measure(r.Context(), end)
	if err != nil {
		slog.Error("growth stages report failed: participants", "error", err)
		writeOperatorError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to compute the stage report")
		return
	}
	m, err := h.readers.Stages.Measure(r.Context(), end)
	if err != nil {
		slog.Error("growth stages report failed: stages", "error", err)
		writeOperatorError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to compute the stage report")
		return
	}
	writeActivationJSON(w, http.StatusOK, map[string]any{
		"data": growth.EvaluateStages(m, participantTargetInputs(p)),
	})
}

// GetModel handles GET /admin/growth/model?month=YYYY-MM: the monthly acquisition model — observed
// flows per population, the worked arithmetic, hypothetical scenarios, the channel comparison and
// the current bottleneck read from the stage gates at the month's end. month defaults to the last
// complete calendar month (UTC).
func (h *GrowthReportsHandler) GetModel(w http.ResponseWriter, r *http.Request) {
	ApplyOperatorReportCachePolicy(w)
	if !checkActivationAnalyticsAuth(w, r) {
		return
	}
	month, ok := readReportMonth(w, r)
	if !ok {
		return
	}
	flows, err := h.readers.Model.MonthlyFlows(r.Context(), month)
	if err != nil {
		slog.Error("growth model report failed: flows", "error", err)
		writeOperatorError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to compute the acquisition model")
		return
	}
	stages, err := h.readers.Stages.Measure(r.Context(), month.AddDate(0, 1, 0))
	if err != nil {
		slog.Error("growth model report failed: stages", "error", err)
		writeOperatorError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to compute the acquisition model")
		return
	}
	writeActivationJSON(w, http.StatusOK, map[string]any{"data": growth.BuildModelReport(flows, stages)})
}

// reportMonthPattern is a calendar month, YYYY-MM.
var reportMonthPattern = regexp.MustCompile(`^\d{4}-\d{2}$`)

// readReportMonth reads ?month=YYYY-MM as the first instant of that month (UTC), defaulting to the
// last complete month. It answers a malformed month itself.
func readReportMonth(w http.ResponseWriter, r *http.Request) (time.Time, bool) {
	raw := r.URL.Query().Get("month")
	if raw == "" {
		now := time.Now().UTC()
		return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, -1, 0), true
	}
	month, err := time.Parse("2006-01", raw)
	if !reportMonthPattern.MatchString(raw) || err != nil {
		writeOperatorError(w, http.StatusBadRequest, "INVALID_MONTH", "month must be a calendar month, YYYY-MM")
		return time.Time{}, false
	}
	return month.UTC(), true
}

// participantTargetInputs reduces participant measures to the identity sums a target reads.
func participantTargetInputs(p growth.ParticipantMeasures) growth.TargetInputs {
	return growth.TargetInputs{
		Current:   p.Humans + p.Agents,
		Previous:  p.PreviousHumans + p.PreviousAgents,
		Returning: p.ReturningHumans + p.ReturningAgents,
	}
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
