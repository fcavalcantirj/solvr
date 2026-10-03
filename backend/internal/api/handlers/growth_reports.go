package handlers

import (
	"context"
	"log/slog"
	"net/http"
	"regexp"
	"time"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/growth"
)

// The operator growth reports.
//
//	GET /admin/growth/participants?end=<RFC3339>  monthly active participants (spec.json idx 86)
//	GET /admin/growth/stages?end=<RFC3339>        staged growth gates (spec.json idx 89)
//	GET /admin/growth/model?month=YYYY-MM         the monthly acquisition model (spec.json idx 90)
//	GET /admin/growth/acquisition-loop?end=...    the planner-to-executor acquisition loop (spec.json idx 87)
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

// LoopReader measures the acquisition loop for the window ending at end.
type LoopReader interface {
	Measure(ctx context.Context, end time.Time, exampleSlugs []string) (growth.LoopMeasures, error)
}

// ShareReader is lane G1's share attribution (db.ShareAttributionRepository).
type ShareReader interface {
	Measure(ctx context.Context, from, to, now time.Time) (db.ShareAttributionReport, error)
}

// GrowthReaders are the measurements the growth reports read.
type GrowthReaders struct {
	Participants ParticipantReader
	Stages       StageReader
	Model        ModelReader
	Loop         LoopReader
	Share        ShareReader
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
	if err == nil {
		m.Source, err = h.readSource(r.Context(), end.AddDate(0, 0, -growth.ParticipantWindowDays), end)
	}
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
	var source growth.SourceMeasures
	if err == nil {
		source, err = h.readSource(r.Context(), month, month.AddDate(0, 1, 0))
	}
	if err != nil {
		slog.Error("growth model report failed: stages or sources", "error", err)
		writeOperatorError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to compute the acquisition model")
		return
	}
	writeActivationJSON(w, http.StatusOK, map[string]any{"data": growth.BuildModelReport(flows, stages, source)})
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

// GetAcquisitionLoop handles GET /admin/growth/acquisition-loop?end=<RFC3339>: the planner-to-
// executor loop — example-room evidence (the public demo first, then the editorial preview rooms),
// first-connection failure points, 7- and 28-day returns and same-owner agent depth.
func (h *GrowthReportsHandler) GetAcquisitionLoop(w http.ResponseWriter, r *http.Request) {
	end, ok := h.authorizeAndReadEnd(w, r)
	if !ok {
		return
	}
	m, err := h.readers.Loop.Measure(r.Context(), end, exampleRoomSlugs())
	if err == nil {
		m.Source, err = h.readSource(r.Context(), end.AddDate(0, 0, -growth.ParticipantWindowDays), end)
	}
	if err != nil {
		slog.Error("growth acquisition loop report failed", "error", err)
		writeOperatorError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to compute the acquisition loop")
		return
	}
	writeActivationJSON(w, http.StatusOK, map[string]any{"data": growth.BuildLoopReport(m)})
}

// exampleRoomSlugs is the public demo room followed by the editorial preview rooms, each once.
func exampleRoomSlugs() []string {
	slugs := []string{growth.PublicDemoRoomSlug}
	seen := map[string]bool{growth.PublicDemoRoomSlug: true}
	for _, s := range PreviewSlugsFromEnv() {
		if !seen[s] {
			seen[s] = true
			slugs = append(slugs, s)
		}
	}
	return slugs
}

// readSource reads lane G1's share attribution for [from, to) and reduces it to the source figures
// the growth reports read. With no share reader configured the figures are reported unavailable.
func (h *GrowthReportsHandler) readSource(ctx context.Context, from, to time.Time) (growth.SourceMeasures, error) {
	if h.readers.Share == nil {
		return growth.SourceMeasures{}, nil
	}
	rep, err := h.readers.Share.Measure(ctx, from, to, to)
	if err != nil {
		return growth.SourceMeasures{}, err
	}
	ret := func(r db.ShareReturns) growth.ReturnCount {
		return growth.ReturnCount{Eligible: r.Eligible, Returned: r.Returned}
	}
	return growth.SourceMeasures{
		Available:                true,
		ShareVisits:              rep.ShareVisits.Total,
		HumanShareVisits:         rep.ShareVisits.Human,
		AttributedRoomsActivated: rep.AttributedRoomsActivated,
		NewHumanActivations:      rep.NewHumanActivations,
		NewAgentActivations:      rep.NewAgentActivations,
		HumanReturns7d:           ret(rep.HumanReturns7d),
		HumanReturns28d:          ret(rep.HumanReturns28d),
		AgentReturns7d:           ret(rep.AgentReturns7d),
		AgentReturns28d:          ret(rep.AgentReturns28d),
	}, nil
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
