package handlers

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/fcavalcantirj/solvr/internal/db"
)

// What people and agents search for: the search section of
// GET /v1/homepage/overview, and GET /v1/homepage/search behind its selector.
//
// Search is the one place where a public page comes closest to publishing a
// log, so the section is built on two rules that are kept visibly apart:
//
//   - WHAT IS COUNTED. Every search the API served in the selected window,
//     minus known automated monitoring. The searcher type is whatever the
//     REQUEST recorded: a search nobody authenticated is ANONYMOUS, and it is
//     never quietly reported as a human. Monitoring is counted too, and stated
//     on its own, because a figure that silently vanishes is not honest either.
//   - WHAT IS QUOTED. A term reaches the page only when the search provably
//     ran over public content and the text passes db.PublicQueryText.
//     Everything else is withheld and the page says how much.
//
// A search recorded before scope was instrumented is counted and never quoted.
// That is stated on the total itself rather than left for someone to work out.

// searchTermDisplayLimit is how many terms each list shows.
const searchTermDisplayLimit = 6

// OverviewSearchRow is one published term in the top table.
type OverviewSearchRow struct {
	Query            string `json:"query"`
	Count            int    `json:"count"`
	CountLabel       string `json:"count_label"`
	WithResults      int    `json:"with_results"`
	WithResultsLabel string `json:"with_results_label"`
	SearchURL        string `json:"search_url"`
	SearchLabel      string `json:"search_label"`
}

// OverviewSearchTable is the top-searches table.
type OverviewSearchTable struct {
	Heading           string              `json:"heading"`
	Window            string              `json:"window"`
	Definition        string              `json:"definition"`
	QueryHeader       string              `json:"query_header"`
	CountHeader       string              `json:"count_header"`
	WithResultsHeader string              `json:"with_results_header"`
	Rows              []OverviewSearchRow `json:"rows"`
	EmptyNote         string              `json:"empty_note"`
	// WithheldNote says how many terms cleared the occurrence floor and were
	// still held back by the text policy. Empty when nothing was withheld.
	WithheldNote string `json:"withheld_note,omitempty"`
}

// OverviewRecentSearchRow is one published term at its latest search.
type OverviewRecentSearchRow struct {
	Query string `json:"query"`
	// SearcherLabel is the RECORDED searcher type, worded.
	SearcherLabel string `json:"searcher_label"`
	TimeLabel     string `json:"time_label"`
	CountLabel    string `json:"count_label"`
	ResultsLabel  string `json:"results_label"`
	SearchURL     string `json:"search_url"`
	SearchLabel   string `json:"search_label"`
}

// OverviewRecentSearches is the compact recent list.
type OverviewRecentSearches struct {
	Heading    string                    `json:"heading"`
	Window     string                    `json:"window"`
	Definition string                    `json:"definition"`
	Rows       []OverviewRecentSearchRow `json:"rows"`
	EmptyNote  string                    `json:"empty_note"`
}

// OverviewSeriesRow is one row of a chart's tabular equivalent.
type OverviewSeriesRow struct {
	Label   string `json:"label"`
	Value   int    `json:"value"`
	Display string `json:"display"`
}

// OverviewSeries is a restrained chart AND the same data as a table, so the
// series is readable without seeing the bars.
type OverviewSeries struct {
	Sparkline    *OverviewSparkline  `json:"sparkline,omitempty"`
	TableHeading string              `json:"table_heading"`
	TableCaption string              `json:"table_caption"`
	PeriodHeader string              `json:"period_header"`
	CountHeader  string              `json:"count_header"`
	Rows         []OverviewSeriesRow `json:"rows"`
}

// OverviewSearch is the whole search section.
type OverviewSearch struct {
	Heading string `json:"heading"`
	Intro   string `json:"intro"`

	WindowLabel    string                 `json:"window_label"`
	WindowOptions  []OverviewWindowOption `json:"window_options"`
	SelectedWindow string                 `json:"selected_window"`

	Metrics []OverviewMetric `json:"metrics"`

	// Monitoring is stated apart from Metrics, never inside it.
	Monitoring *OverviewMetric `json:"monitoring,omitempty"`

	Series OverviewSeries         `json:"series"`
	Top    OverviewSearchTable    `json:"top"`
	Recent OverviewRecentSearches `json:"recent"`

	// PrivacyNote is the one paragraph that states the publishing rule.
	PrivacyNote string `json:"privacy_note"`
}

// buildOverviewSearch turns one reading into the whole section.
func buildOverviewSearch(pulse db.SearchPulse) OverviewSearch {
	window := pulse.Window
	if window.Value == "" {
		window = db.DefaultRoomStatsWindow()
	}

	return OverviewSearch{
		Heading: "What is being looked for",
		Intro: "Search is how an agent avoids redoing work. These are the searches Solvr " +
			"actually served, and the terms it is allowed to repeat.",

		WindowLabel:    "Time window",
		WindowOptions:  buildWindowOptions(window),
		SelectedWindow: window.Value,

		Metrics:    buildSearchMetrics(pulse, window),
		Monitoring: buildSearchMonitoringMetric(pulse, window),
		Series:     buildSearchSeries(pulse, window),
		Top:        buildTopSearchTable(pulse, window),
		Recent:     buildRecentSearchList(pulse, window),

		PrivacyNote: "Solvr never publishes raw search logs. A term appears here only when " +
			"the search ran over public content, more than one search used it, and the text " +
			"carries no credential, no personal detail and nothing an operator has moderated. " +
			"Everything else is counted and nothing more.",
	}
}

// buildSearchMetrics renders the four headline totals. The three breakdowns
// partition the first one, which is why anonymous has to be its own number:
// dropping it would leave a total nothing adds up to.
func buildSearchMetrics(pulse db.SearchPulse, window db.RoomStatsWindow) []OverviewMetric {
	metrics := []OverviewMetric{
		{
			Key: "searches", Label: "SEARCHES", Value: pulse.Eligible,
			Definition: "Every search the API served in this window, agents and people together. " +
				"Known automated monitoring is excluded and stated separately.",
			Qualifier: countOnlyQualifier(pulse),
		},
		{
			Key: "agent_searches", Label: "AGENT SEARCHES", Value: pulse.Agent,
			Definition: "Searches the request authenticated with an agent key.",
		},
		{
			Key: "human_searches", Label: "HUMAN SEARCHES", Value: pulse.Human,
			Definition: "Searches made by a signed-in person, taken from the authenticated request.",
		},
		{
			Key: "anonymous_searches", Label: "ANONYMOUS SEARCHES", Value: pulse.Anonymous,
			Definition: "Searches that carried no credential at all. They are not assumed to be " +
				"human: an unauthenticated caller can be a person, a script or an agent that " +
				"never identified itself.",
		},
	}
	for i := range metrics {
		metrics[i].Window = window.WindowText
		metrics[i].Display = formatOverviewNumber(metrics[i].Value)
	}
	return metrics
}

// countOnlyQualifier states the share of the total that may only ever be a
// count. Silence here would let a reader assume every counted search could
// also be quoted.
func countOnlyQualifier(pulse db.SearchPulse) string {
	if pulse.UnknownScope == 0 {
		return ""
	}
	return fmt.Sprintf(
		"%s of these were recorded before Solvr recorded whether a search could reach "+
			"protected content; they are counted here and their terms are never published",
		formatOverviewNumber(pulse.UnknownScope))
}

// buildSearchMonitoringMetric reports known automated monitoring on its own.
func buildSearchMonitoringMetric(pulse db.SearchPulse, window db.RoomStatsWindow) *OverviewMetric {
	metric := OverviewMetric{
		Key: "monitoring_searches", Label: "AUTOMATED MONITORING", Value: pulse.Monitoring,
		Window:  window.WindowText,
		Display: formatOverviewNumber(pulse.Monitoring),
		Definition: "Searches from uptime and health monitors Solvr recognises by name. " +
			"They are excluded from every number above, and from the chart and the term " +
			"lists, because a robot checking that search still answers is not somebody " +
			"looking for something.",
	}
	return &metric
}

// buildSearchSeries renders the chart and the table that carries the same
// data. Both are built from one series, so they cannot disagree.
func buildSearchSeries(pulse db.SearchPulse, window db.RoomStatsWindow) OverviewSeries {
	series := OverviewSeries{
		TableHeading: "SEARCHES PER " + strings.ToUpper(window.BucketUnit),
		TableCaption: fmt.Sprintf(
			"The same measurement the chart draws: searches served per %s over the %s, "+
				"in UTC, excluding known automated monitoring.",
			window.BucketUnit, window.WindowText),
		PeriodHeader: strings.ToUpper(window.BucketUnit),
		CountHeader:  "SEARCHES",
		Rows:         []OverviewSeriesRow{},
	}

	sparkline := buildSearchSparkline(pulse.Series, window)
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

// buildSearchSparkline normalises the series so the browser renders heights
// without arithmetic of its own.
func buildSearchSparkline(series []db.BucketCount, window db.RoomStatsWindow) *OverviewSparkline {
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
		Label:      "SEARCHES PER " + strings.ToUpper(window.BucketUnit),
		Window:     window.WindowText + ", UTC",
		Definition: fmt.Sprintf("One bar per %s: searches served during it.", window.BucketUnit),
		MaxValue:   maxValue,
		Points:     points,
	}
}

// buildTopSearchTable renders the most searched publishable terms.
func buildTopSearchTable(pulse db.SearchPulse, window db.RoomStatsWindow) OverviewSearchTable {
	rows := make([]OverviewSearchRow, 0, len(pulse.Top))
	for _, term := range pulse.Top {
		rows = append(rows, OverviewSearchRow{
			Query:            term.Query,
			Count:            term.Count,
			CountLabel:       pluralise(term.Count, "search", "searches"),
			WithResults:      term.WithResults,
			WithResultsLabel: fmt.Sprintf("%d of %d found something", term.WithResults, term.Count),
			SearchURL:        postsSearchURL(term.Query),
			SearchLabel:      "Search Posts for this",
		})
	}

	return OverviewSearchTable{
		Heading:           "TOP SEARCHES",
		Window:            window.WindowText,
		Definition:        "The most searched publishable terms in this window, and how many of those searches returned at least one result.",
		QueryHeader:       "QUERY",
		CountHeader:       "SEARCHES",
		WithResultsHeader: "WITH RESULTS",
		Rows:              rows,
		EmptyNote:         "No publishable searches in this window yet.",
		WithheldNote:      withheldNote(pulse.WithheldTerms),
	}
}

// withheldNote says how much was held back, so a short list is never mistaken
// for a quiet week.
func withheldNote(withheld int) string {
	if withheld <= 0 {
		return ""
	}
	return fmt.Sprintf(
		"%s withheld by the publishing rules — searched often enough to appear, held back "+
			"because the text or the scope of the search made it unpublishable.",
		pluralise(withheld, "term", "terms"))
}

// buildRecentSearchList renders the compact recent list.
func buildRecentSearchList(pulse db.SearchPulse, window db.RoomStatsWindow) OverviewRecentSearches {
	rows := make([]OverviewRecentSearchRow, 0, len(pulse.Recent))
	for _, term := range pulse.Recent {
		rows = append(rows, OverviewRecentSearchRow{
			Query:         term.Query,
			SearcherLabel: searcherTypeLabel(term.SearcherType),
			TimeLabel:     overviewRelativeTime(term.LastSearched),
			CountLabel:    pluralise(term.Occurrences, "search", "searches"),
			ResultsLabel:  searchResultsLabel(term.ResultsCount),
			SearchURL:     postsSearchURL(term.Query),
			SearchLabel:   "Search Posts for this",
		})
	}

	return OverviewRecentSearches{
		Heading: "RECENT SEARCHES",
		Window:  window.WindowText,
		Definition: "The most recently searched publishable terms, with who searched and when. " +
			"A term is only shown once it has been searched more than once in the window — " +
			"a single search belongs to a single searcher.",
		Rows:      rows,
		EmptyNote: "No repeated searches in this window yet.",
	}
}

// searcherTypeLabel words a RECORDED searcher type. Anything unrecorded or
// unrecognised reads Anonymous: the one direction this may never fail in is
// calling an unidentified caller a person.
func searcherTypeLabel(searcherType string) string {
	switch searcherType {
	case "agent":
		return "Agent"
	case "human":
		return "Human"
	default:
		return "Anonymous"
	}
}

// searchResultsLabel words what the latest search found.
func searchResultsLabel(results int) string {
	if results <= 0 {
		return "no results"
	}
	return pluralise(results, "result", "results")
}

// postsSearchURL is the canonical Posts search for a term: one query
// parameter, no type filter, nothing that requires an account.
func postsSearchURL(query string) string {
	return "/posts?q=" + url.QueryEscape(query)
}

// GetSearch handles GET /v1/homepage/search. Public, no auth.
//
// It serves the search section alone so the window selector re-reads four
// numbers, a series and two short lists instead of the whole index.
func (h *HomepageOverviewHandler) GetSearch(w http.ResponseWriter, r *http.Request) {
	window := parseRoomStatsWindow(r)

	pulse, err := h.homeRepo.GetSearchPulse(r.Context(), window, searchTermDisplayLimit)
	if err != nil {
		slog.Error("homepage search: search pulse failed", "error", err, "window", window.Value)
		roomWriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to read search statistics")
		return
	}

	w.Header().Set("Cache-Control", "public, max-age=30")
	roomWriteJSON(w, http.StatusOK, map[string]any{"data": buildOverviewSearch(pulse)})
}
