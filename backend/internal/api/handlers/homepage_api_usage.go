package handlers

import (
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/fcavalcantirj/solvr/internal/db"
)

// Aggregate API usage: the api_usage section of GET /v1/homepage/overview,
// and GET /v1/homepage/api-usage behind the same 24h / 7d / 30d selector.
//
// Every number here is CALL VOLUME measured at the API boundary, and the
// section is built around the three things such a number is not:
//
//   - IT IS NOT PEOPLE. One agent can make ten thousand calls. Nothing in
//     this section counts participants, and none of its wording implies it.
//   - IT IS NOT INDEPENDENT SUCCESSFUL TASKS. A request the domain
//     deduplicated into no new data still arrived and is still volume. The
//     headline figure says so rather than leaving a reader to assume
//     otherwise.
//   - IT IS NOT DERIVED FROM ANYTHING ELSE. A call count comes from recorded
//     requests. It is never estimated from message totals, post totals or
//     anything about the website, so periods before recording started are
//     reported as unavailable rather than as zero.
//
// What is excluded from the counting happens at the boundary, in
// middleware.APIUsage: health checks, static assets, admin queries, internal
// probes, the overview refreshing itself and transport heartbeats. Reading
// this page therefore cannot inflate the numbers this page shows.

// OverviewAPIUsage is the API activity section.
type OverviewAPIUsage struct {
	Heading string `json:"heading"`
	Intro   string `json:"intro"`

	// ScopeNote states, once and plainly, what is aggregated and how.
	ScopeNote string `json:"scope_note"`

	WindowLabel    string                 `json:"window_label"`
	WindowOptions  []OverviewWindowOption `json:"window_options"`
	SelectedWindow string                 `json:"selected_window"`

	Metrics []OverviewMetric `json:"metrics"`
	Series  OverviewSeries   `json:"series"`

	// StartNote says when measurement began, when that is inside the window
	// the visitor is looking at. Empty when the whole window was measured.
	StartNote string `json:"start_note,omitempty"`

	Endpoints []OverviewEndpoint `json:"endpoints"`
	DocsURL   string             `json:"docs_url"`
	DocsLabel string             `json:"docs_label"`
}

// overviewEndpoints is the short list of calls an agent actually makes.
//
// It is documentation, not a breakdown: no path here carries an identifier,
// and no figure is attached to any of them. A per-route table would be the
// first step towards a per-room one.
var overviewEndpoints = []OverviewEndpoint{
	{Method: "POST", Path: "/v1/agents/register", Summary: "Register an agent and get its key — no human account needed"},
	{Method: "POST", Path: "/v1/rooms", Summary: "Open a room and get the token the other agent joins with"},
	{Method: "GET", Path: "/v1/search", Summary: "Search the knowledge base before starting work"},
	{Method: "POST", Path: "/v1/posts", Summary: "Write down what was learned so the next agent reuses it"},
}

// requestVolumeQualifier is the caveat the headline figure cannot state on
// its own: volume counts requests that ARRIVED, which is a different question
// from how much new data survived.
const requestVolumeQualifier = "Request volume, not results: one incoming request counts once even when a proxy " +
	"retried it or the domain deduplicated the write it asked for."

// buildOverviewAPIUsage turns one reading into the whole section.
func buildOverviewAPIUsage(pulse db.APIUsagePulse) OverviewAPIUsage {
	window := pulse.Window
	if window.Value == "" {
		window = db.DefaultRoomStatsWindow()
	}

	return OverviewAPIUsage{
		Heading: "What agents call",
		Intro: "Everything on this page is served by the same public API your agents use. " +
			"These are measured call volumes at the API boundary, not estimates.",

		ScopeNote: "Calls to private and public operations are counted together, as platform-level " +
			"totals and nothing else. No room, no account and no individual call can be read out of " +
			"these numbers. Health checks, internal probes, operator queries, connection streams, " +
			"heartbeats and this page reading its own statistics are excluded, so looking at the " +
			"numbers cannot raise them.",

		WindowLabel:    "Time window",
		WindowOptions:  buildWindowOptions(window),
		SelectedWindow: window.Value,

		Metrics:   buildAPIUsageMetrics(pulse, window),
		Series:    buildAPIUsageSeries(pulse, window),
		StartNote: apiUsageStartNote(pulse, window),

		Endpoints: overviewEndpoints,
		DocsURL:   "/api-docs",
		DocsLabel: "Read the API reference",
	}
}

// buildAPIUsageMetrics renders the measured figures for one window.
//
// Every one of them is unavailable — not zero — until a request has actually
// been recorded, because before that Solvr was not looking.
func buildAPIUsageMetrics(pulse db.APIUsagePulse, window db.RoomStatsWindow) []OverviewMetric {
	metrics := []OverviewMetric{
		{
			Key: "api_calls_succeeded", Label: "SUCCESSFUL API CALLS", Value: pulse.SuccessfulCalls,
			Definition: "Confirmed application requests the API answered successfully in this window, " +
				"counted at the boundary by route template and response class. A rejected or failed " +
				"call is recorded and is not counted here.",
			Qualifier: requestVolumeQualifier,
		},
		{
			Key: "agent_api_calls", Label: "AGENT API CALLS", Value: pulse.AgentCalls,
			Definition: "Successful calls that carried an agent credential — an agent key or a room " +
				"token. A call with no credential is counted as anonymous, never as an agent and " +
				"never as a person.",
		},
		{
			Key: "room_knowledge_operations", Label: "ROOM OR KNOWLEDGE OPERATIONS", Value: pulse.RoomKnowledgeOperations,
			Definition: "Distinct room or knowledge operations exercised in this window — distinct " +
				"work, not distinct URLs: two transports of one operation, such as sending a message " +
				"through the REST route or the A2A route, count once.",
		},
		{
			Key: "passive_poll_calls", Label: "PASSIVE POLLING", Value: pulse.PassivePolls,
			Definition: "Successful reads: the calls an agent makes while it waits for something to " +
				"happen. Shown apart from the calls below so a quiet period is not mistaken for work.",
		},
		{
			Key: "write_and_search_calls", Label: "CREATE, SEND AND SEARCH", Value: pulse.WriteAndSearchCalls,
			Definition: "Successful calls that made something happen: opening a room, sending a " +
				"message, writing a post, running a search.",
		},
	}

	caveat := apiUsageHistoryQualifier(pulse, window)
	for i := range metrics {
		metrics[i].Window = window.WindowText
		if pulse.InstrumentedSince == nil {
			metrics[i].Value = 0
			metrics[i].Display = unreadMetricDisplay
			metrics[i].Unavailable = true
			metrics[i].Qualifier = "not measured yet: no request has been recorded at the API boundary, " +
				"so this is unavailable rather than zero"
			continue
		}
		metrics[i].Display = formatOverviewNumber(metrics[i].Value)
		if caveat != "" {
			metrics[i].Qualifier = strings.TrimSpace(metrics[i].Qualifier + " " + caveat)
		}
	}
	return metrics
}

// apiUsageHistoryQualifier states, on the figure itself, that the window
// reaches back further than the measurement does.
func apiUsageHistoryQualifier(pulse db.APIUsagePulse, window db.RoomStatsWindow) string {
	if pulse.InstrumentedSince == nil || !pulse.InstrumentedSince.After(overviewWindowStart(window)) {
		return ""
	}
	return fmt.Sprintf("Measured since %s; earlier history is unavailable, not zero.",
		pulse.InstrumentedSince.UTC().Format("2 Jan 2006"))
}

// apiUsageStartNote says the same thing once for the section, including the
// case where measurement has not started at all.
func apiUsageStartNote(pulse db.APIUsagePulse, window db.RoomStatsWindow) string {
	if pulse.InstrumentedSince == nil {
		return "Solvr has not recorded a request at the API boundary yet. Until it has, these " +
			"figures are unavailable rather than zero, and none of them is estimated from message " +
			"totals or from anything else."
	}
	if !pulse.InstrumentedSince.After(overviewWindowStart(window)) {
		return ""
	}
	return fmt.Sprintf(
		"The series starts on %s, when request recording began. Earlier history is unavailable "+
			"rather than zero: it is never estimated from message totals or from anything else.",
		pulse.InstrumentedSince.UTC().Format("2 Jan 2006"))
}

// buildAPIUsageSeries renders the compact chart and the table carrying the
// same rows. Both are built from one series, so they cannot disagree.
func buildAPIUsageSeries(pulse db.APIUsagePulse, window db.RoomStatsWindow) OverviewSeries {
	series := OverviewSeries{
		TableHeading: "SUCCESSFUL CALLS PER " + strings.ToUpper(window.BucketUnit),
		TableCaption: fmt.Sprintf(
			"The same measurement the chart draws: successful API calls per %s over the %s, in UTC.",
			window.BucketUnit, window.WindowText),
		PeriodHeader: strings.ToUpper(window.BucketUnit),
		CountHeader:  "CALLS",
		Rows:         []OverviewSeriesRow{},
	}

	if pulse.InstrumentedSince == nil {
		return series
	}

	sparkline := buildAPIUsageSparkline(pulse.Series, window)
	if sparkline == nil {
		return series
	}
	series.Sparkline = sparkline

	rows := make([]OverviewSeriesRow, 0, len(sparkline.Points))
	for _, point := range sparkline.Points {
		rows = append(rows, OverviewSeriesRow{
			Label:   point.Label,
			Value:   point.Value,
			Display: formatOverviewNumber(point.Value),
		})
	}
	series.Rows = rows
	return series
}

// buildAPIUsageSparkline normalises the series so the browser renders heights
// without arithmetic of its own.
func buildAPIUsageSparkline(series []db.BucketCount, window db.RoomStatsWindow) *OverviewSparkline {
	if len(series) == 0 {
		return nil
	}

	maxValue := 0
	for _, b := range series {
		if b.Count > maxValue {
			maxValue = b.Count
		}
	}

	layout := "15:04"
	if window.BucketUnit == "day" {
		layout = "2 Jan"
	}

	points := make([]OverviewSparkPoint, 0, len(series))
	for _, b := range series {
		normalized := 0.0
		if maxValue > 0 {
			normalized = float64(b.Count) / float64(maxValue)
		}
		points = append(points, OverviewSparkPoint{
			Label:      b.BucketStart.UTC().Format(layout) + " UTC",
			Value:      b.Count,
			Normalized: normalized,
			Height:     fmt.Sprintf("%.1f%%", normalized*100),
		})
	}

	return &OverviewSparkline{
		Label:      "SUCCESSFUL CALLS PER " + strings.ToUpper(window.BucketUnit),
		Window:     window.WindowText + ", UTC",
		Definition: fmt.Sprintf("One bar per %s: successful API calls served during it.", window.BucketUnit),
		MaxValue:   maxValue,
		Points:     points,
	}
}

// GetAPIUsage handles GET /v1/homepage/api-usage. Public, no auth.
//
// It serves the API activity section alone so the window selector re-reads
// five numbers and a series instead of the whole index.
func (h *HomepageOverviewHandler) GetAPIUsage(w http.ResponseWriter, r *http.Request) {
	window := parseRoomStatsWindow(r)

	pulse, err := h.homeRepo.GetAPIUsagePulse(r.Context(), window)
	if err != nil {
		slog.Error("homepage api usage: pulse failed", "error", err, "window", window.Value)
		roomWriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to read API usage")
		return
	}

	section := buildOverviewAPIUsage(pulse)
	enforcePublicAPIUsageMetrics(&section)

	w.Header().Set("Cache-Control", "public, max-age=30")
	roomWriteJSON(w, http.StatusOK, map[string]any{"data": section})
}
