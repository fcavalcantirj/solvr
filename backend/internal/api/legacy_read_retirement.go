package api

import (
	"strings"

	"github.com/go-chi/chi/v5"
)

// LegacyReadRetirements lists every retired legacy read route (task idx 73 step 3: adapt, then
// retire). The type-specific statistics were adapters over the overview knowledge aggregate
// (idx 72); their counts live only in GET /v1/overview now. Like the retired writes, each
// answers every caller 410 ENDPOINT_RETIRED naming the replacement, with no sunset period.
// TestLegacyReadRetirements_CoverEveryRetiredReadFamilyAndNameAServedReplacement pins it to
// the GET routes of the retired read families.
var LegacyReadRetirements = []LegacyRouteRetirement{
	{"GET /v1/stats/problems", "GET /v1/overview",
		"Per-type counts are in data.knowledge.types of GET /v1/overview. In the entry whose type is \"problem\", " +
			"total was total_problems and by_status.solved was solved_count. active_approaches, " +
			"avg_solve_time_days, recently_solved and top_solvers have no canonical equivalent."},
	{"GET /v1/stats/questions", "GET /v1/overview",
		"Per-type counts are in data.knowledge.types of GET /v1/overview. In the entry whose type is \"question\", " +
			"total was total_questions and with_accepted_reply was answered_count; response_rate was " +
			"with_accepted_reply * 100 / total. avg_response_time_hours, recently_answered and top_answerers " +
			"have no canonical equivalent."},
	{"GET /v1/stats/ideas", "GET /v1/overview",
		"Per-type counts are in data.knowledge.types of GET /v1/overview. In the entry whose type is \"idea\", " +
			"by_status and total were counts_by_status. fresh_sparks, ready_to_develop, top_sparklers, " +
			"trending_tags, pipeline_stats and recently_realized have no canonical equivalent."},
}

// mountRetiredLegacyReads registers every retired read on the /v1 router, outside every
// auth, rate-limit and content-gate middleware, as mountRetiredLegacyWrites does.
func mountRetiredLegacyReads(r chi.Router) {
	for _, ret := range LegacyReadRetirements {
		method, path, _ := strings.Cut(ret.Route, " ")
		r.Method(method, strings.TrimPrefix(path, "/v1"), retiredLegacyRoute(ret))
	}
}
