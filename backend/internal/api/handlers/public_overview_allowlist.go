package handlers

import (
	"log/slog"
	"regexp"
	"strings"
)

// What Solvr publishes about itself, and what it never publishes.
//
// Solvr measures two different things and only one of them is public:
//
//   - PRODUCT ACTIVITY — what agents and people DO inside Solvr. Rooms,
//     participation in public rooms, the searches Solvr is allowed to repeat,
//     the volume of API calls, and the labeled totals of what the product has
//     accumulated. This is the product working in the open, and the homepage
//     is built to show it.
//   - WEBSITE TRAFFIC AND GROWTH INTELLIGENCE — what an audience does TO the
//     website. Visitors, sessions, page views, daily/weekly/monthly active
//     audience estimates, bounce, where the audience came from, which campaign
//     brought it, which country it is in, what share of it converted, how much
//     of it came back, and the growth targets set against all of that. This is
//     operator analytics. It is not published anywhere a visitor can reach:
//     not on the homepage, not on /data, not through a public API, not in
//     server-rendered HTML or a hydration payload, not in page metadata and
//     not in a downloadable export.
//
// Two rules follow from that, and both are enforced here rather than in the
// browser:
//
//  1. The public payload has a DEDICATED SCHEMA. Internal analytics fields are
//     never serialized and then hidden by the UI — a field the browser does not
//     draw is still a field anyone can read with curl.
//  2. The published metrics are an ALLOWLIST. A metric reaches a visitor only
//     because somebody named it here and filed it under a public category;
//     anything else is withheld on the way out and logged.
//
// Publishing a traffic figure in the future is an owner's decision about an
// exact metric, an exact window and an exact claim. There is deliberately no
// threshold in this code that starts publishing traffic once it is large
// enough, and the internal report the redesign was planned from is not a
// website asset.

// PublicOverviewCategory is one of the five kinds of thing Solvr publishes
// about itself.
type PublicOverviewCategory string

const (
	// CategoryRoomActivity is what happened in public rooms over a window.
	CategoryRoomActivity PublicOverviewCategory = "room activity"

	// CategoryPublicRoomParticipation is who is in a public room right now.
	CategoryPublicRoomParticipation PublicOverviewCategory = "participation in public rooms"

	// CategoryEligibleSearchActivity is search the publishing rules allow.
	CategoryEligibleSearchActivity PublicOverviewCategory = "eligible search activity"

	// CategoryAggregateAPIUsage is call volume at the API boundary, never
	// people and never independent successful tasks.
	CategoryAggregateAPIUsage PublicOverviewCategory = "aggregate API usage"

	// CategoryProductTotals is cumulative product and community scale, each
	// total labeled with what it counts.
	CategoryProductTotals PublicOverviewCategory = "labeled product and community totals"
)

// PublicOverviewSections are the top-level sections the public overview may
// carry, and what each one is for. A new section cannot appear in the payload
// without being named here.
var PublicOverviewSections = map[string]string{
	"rooms":        "public room statistics: presence now, and activity over the selected window",
	"activity":     "recent activity in public rooms, bounded and paginated",
	"previews":     "the editorially selected public rooms, quoted from public messages only",
	"api_usage":    "aggregate call volume at the API boundary",
	"search":       "eligible search activity and the terms the publishing rules allow",
	"community":    "labeled all-time product totals",
	"posts":        "public posts an agent can reuse",
	"closing":      "the connection proposition and its control",
	"generated_at": "when this snapshot was read",
}

// PublicOverviewMetrics is every metric the public overview may publish, filed
// under the category that makes it publishable.
//
// A registration total is product scale, never audience: it says an account
// exists, and the metric that carries it also carries the qualifier saying so.
var PublicOverviewMetrics = map[string]PublicOverviewCategory{
	// Who is in a public room at this moment.
	"agents_online_now":            CategoryPublicRoomParticipation,
	"rooms_with_agents_online_now": CategoryPublicRoomParticipation,

	// What happened in public rooms over the selected window.
	"rooms_with_conversation":      CategoryRoomActivity,
	"agent_messages":               CategoryRoomActivity,
	"human_messages":               CategoryRoomActivity,
	"rooms_with_two_way_exchanges": CategoryRoomActivity,

	// Search Solvr is allowed to report.
	"searches":            CategoryEligibleSearchActivity,
	"agent_searches":      CategoryEligibleSearchActivity,
	"human_searches":      CategoryEligibleSearchActivity,
	"anonymous_searches":  CategoryEligibleSearchActivity,
	"monitoring_searches": CategoryEligibleSearchActivity,

	// Call volume at the API boundary.
	"agent_searches_7d": CategoryAggregateAPIUsage,
	"human_searches_7d": CategoryAggregateAPIUsage,
	"room_messages_24h": CategoryAggregateAPIUsage,

	// Cumulative product scale.
	"public_rooms":        CategoryProductTotals,
	"published_posts":     CategoryProductTotals,
	"registered_agents":   CategoryProductTotals,
	"registered_humans":   CategoryProductTotals,
	"total_contributions": CategoryProductTotals,
	"problems_solved":     CategoryProductTotals,
	"crystallized_posts":  CategoryProductTotals,
}

// PublicOverviewAllowsMetric reports whether a metric key may be published,
// and under which category.
func PublicOverviewAllowsMetric(key string) (PublicOverviewCategory, bool) {
	category, ok := PublicOverviewMetrics[key]
	return category, ok
}

// publicOverviewAuthoredTextKeys are the JSON fields whose text the API itself
// writes: headings, labels, definitions, qualifiers and notes. The excluded
// vocabulary is checked against these.
//
// Participant content — message excerpts, room names, post titles, search
// terms — is deliberately NOT in this set. What may be quoted from it is
// decided by the publishing rules in homepage_search.go and
// homepage_activity.go, not by a word list: a room is allowed to be about
// traffic analysis.
var publicOverviewAuthoredTextKeys = map[string]bool{
	"heading": true, "intro": true, "definition": true, "qualifier": true,
	"label": true, "window": true, "window_label": true, "window_heading": true,
	"presence_heading": true, "presence_note": true, "scope_label": true, "scope_note": true,
	"note": true, "empty_note": true, "privacy_note": true, "refresh_note": true,
	"outcome_note": true, "burst_note": true, "excerpt_note": true, "withheld_note": true,
	"table_heading": true, "table_caption": true, "count_header": true, "period_header": true,
	"query_header": true, "with_results_header": true, "count_label": true, "results_label": true,
	"searcher_label": true, "with_results_label": true, "load_more_label": true,
	"browse_label": true, "docs_label": true, "connect_label": true, "rooms_label": true,
	"search_label": true, "new_label": true, "summary": true, "scope_heading": true,
}

// PublicOverviewAuthoredText reports whether the text under a JSON key was
// written by the API rather than by a participant.
func PublicOverviewAuthoredText(key string) bool {
	return publicOverviewAuthoredTextKeys[key]
}

// privateAnalyticsConcept is one excluded idea and the ways it shows up.
type privateAnalyticsConcept struct {
	name     string
	patterns []*regexp.Regexp
}

// privateAnalyticsConcepts are the audience and growth ideas that never reach
// a public surface. The list is ORDERED: the first concept that matches is the
// one reported, so a failure names the most specific idea it found.
var privateAnalyticsConcepts = []privateAnalyticsConcept{
	{"visitors", []*regexp.Regexp{regexp.MustCompile(`(?i)\bvisitors?\b`)}},
	{"sessions", []*regexp.Regexp{regexp.MustCompile(`(?i)\bsessions?\b`)}},
	{"page views", []*regexp.Regexp{regexp.MustCompile(`(?i)\bpage[ _-]?views?\b`)}},
	{"daily, weekly or monthly active users", []*regexp.Regexp{
		regexp.MustCompile(`(?i)\b(dau|wau|mau)\b`),
		regexp.MustCompile(`(?i)\b(daily|weekly|monthly) active (users|people|accounts|audience)\b`),
	}},
	{"bounce", []*regexp.Regexp{regexp.MustCompile(`(?i)\bbounce\b`)}},
	{"acquisition", []*regexp.Regexp{regexp.MustCompile(`(?i)\bacquisitions?\b`)}},
	{"referrers", []*regexp.Regexp{
		regexp.MustCompile(`(?i)\breferrers?\b`),
		regexp.MustCompile(`(?i)\breferring (site|sites|domain|domains)\b`),
		regexp.MustCompile(`(?i)\butm[ _-][a-z]+\b`),
	}},
	{"geography", []*regexp.Regexp{
		regexp.MustCompile(`(?i)\bgeograph\w*\b`),
		regexp.MustCompile(`(?i)\bcountr(y|ies)\b`),
	}},
	{"conversion", []*regexp.Regexp{
		regexp.MustCompile(`(?i)\bconversions?\b`),
		regexp.MustCompile(`(?i)\bfunnels?\b`),
	}},
	{"retention", []*regexp.Regexp{
		regexp.MustCompile(`(?i)\bretention\b`),
		regexp.MustCompile(`(?i)\breturning (visitors|users|people)\b`),
	}},
	{"campaign", []*regexp.Regexp{regexp.MustCompile(`(?i)\bcampaigns?\b`)}},
	{"growth target", []*regexp.Regexp{regexp.MustCompile(`(?i)\bgrowth (target|targets|goal|goals)\b`)}},
	{"impressions", []*regexp.Regexp{
		regexp.MustCompile(`(?i)\bimpressions?\b`),
		regexp.MustCompile(`(?i)\bclick[ _-]?throughs?\b`),
		regexp.MustCompile(`(?i)\bctr\b`),
	}},
	{"website traffic", []*regexp.Regexp{regexp.MustCompile(`(?i)\btraffic\b`)}},
	{"audience", []*regexp.Regexp{regexp.MustCompile(`(?i)\baudience\b`)}},
	{"visits", []*regexp.Regexp{regexp.MustCompile(`(?i)\bvisits?\b`)}},
}

// PrivateAnalyticsTermIn returns the excluded concept found in s, or "" when
// the text carries none.
func PrivateAnalyticsTermIn(s string) string {
	for _, concept := range privateAnalyticsConcepts {
		for _, pattern := range concept.patterns {
			if pattern.MatchString(s) {
				return concept.name
			}
		}
	}
	return ""
}

// PrivateAnalyticsTermInKey returns the excluded concept found in a JSON field
// name, or "". A field name is API-owned, so snake_case is read as words:
// page_views and pageviews are the same idea.
func PrivateAnalyticsTermInKey(key string) string {
	return PrivateAnalyticsTermIn(strings.ReplaceAll(key, "_", " "))
}

// overviewMetricSlices returns every metric slice the payload carries, by the
// section that publishes it. Enforcement and the contract tests read the same
// list, so a new section cannot be added to the payload and quietly skipped.
func overviewMetricSlices(o *HomepageOverview) map[string]*[]OverviewMetric {
	return map[string]*[]OverviewMetric{
		"rooms.presence_metrics": &o.Rooms.PresenceMetrics,
		"rooms.metrics":          &o.Rooms.Metrics,
		"api_usage.metrics":      &o.APIUsage.Metrics,
		"search.metrics":         &o.Search.Metrics,
		"community.metrics":      &o.Community.Metrics,
	}
}

// filterPublicMetrics keeps only the metrics the allowlist names.
func filterPublicMetrics(section string, metrics []OverviewMetric) []OverviewMetric {
	kept := make([]OverviewMetric, 0, len(metrics))
	for _, m := range metrics {
		if _, ok := PublicOverviewAllowsMetric(m.Key); !ok {
			slog.Error("public overview: metric withheld, not on the public allowlist",
				"section", section, "metric", m.Key)
			continue
		}
		kept = append(kept, m)
	}
	return kept
}

// enforcePublicOverviewMetrics withholds anything the allowlist does not name,
// immediately before the payload is serialized.
func enforcePublicOverviewMetrics(o *HomepageOverview) {
	for section, metrics := range overviewMetricSlices(o) {
		*metrics = filterPublicMetrics(section, *metrics)
	}
	if o.Search.Monitoring != nil {
		if _, ok := PublicOverviewAllowsMetric(o.Search.Monitoring.Key); !ok {
			slog.Error("public overview: metric withheld, not on the public allowlist",
				"section", "search.monitoring", "metric", o.Search.Monitoring.Key)
			o.Search.Monitoring = nil
		}
	}
}

// enforcePublicRoomsMetrics is the same rule for GET /v1/homepage/rooms, which
// serves the rooms section on its own.
func enforcePublicRoomsMetrics(s *OverviewRooms) {
	s.PresenceMetrics = filterPublicMetrics("rooms.presence_metrics", s.PresenceMetrics)
	s.Metrics = filterPublicMetrics("rooms.metrics", s.Metrics)
}

// enforcePublicSearchMetrics is the same rule for GET /v1/homepage/search.
func enforcePublicSearchMetrics(s *OverviewSearch) {
	s.Metrics = filterPublicMetrics("search.metrics", s.Metrics)
	if s.Monitoring != nil {
		if _, ok := PublicOverviewAllowsMetric(s.Monitoring.Key); !ok {
			slog.Error("public overview: metric withheld, not on the public allowlist",
				"section", "search.monitoring", "metric", s.Monitoring.Key)
			s.Monitoring = nil
		}
	}
}
