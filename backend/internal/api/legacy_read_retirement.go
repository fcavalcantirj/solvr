package api

import (
	"strings"

	"github.com/go-chi/chi/v5"
)

// legacyFeedItemsAsPosts tells a legacy feed caller how its feed items map onto the canonical
// GET /v1/posts rows, and the one query-shape difference.
const legacyFeedItemsAsPosts = " Each item of data is a post: snippet is description (the feed cut it to its " +
	"first 200 bytes), answer_count is answers_count (the feed gave 0 for every type but question), " +
	"approach_count is approaches_count and comment_count is comments_count; id, type, title, tags, status, " +
	"author, vote_score and created_at are unchanged. A page or per_page that is not a positive integer, or " +
	"per_page above 50 answers 400 VALIDATION_ERROR instead of falling back to the default or being clamped to 50."

// legacyTypedListAsPosts tells a legacy typed list caller that the canonical rows are the ones it
// got, and the one query-shape difference.
const legacyTypedListAsPosts = " The data rows and meta are unchanged: the route was served by this list. " +
	"A page or per_page that is not a positive integer, or per_page above 50 answers 400 VALIDATION_ERROR " +
	"instead of falling back to the default or being clamped to 50."

// legacyCommentsAsReplies tells a legacy comment list caller how a comment reads as a reply and how
// the replies list differs from the comment list.
const legacyCommentsAsReplies = " Each comment is a reply with its own id: content is body, the comment's id is " +
	"legacy_id, and provenance keeps its target_type and target_id; author_type, author_id, author and " +
	"created_at are unchanged, and deleted comments stay out of the list. The replies come oldest first like " +
	"the comments, but the list holds every reply of the post: meta.total counts them all, and page and " +
	"per_page are replaced by limit (default 50, at most 100) and cursor (pass meta.next_cursor while " +
	"meta.has_more is true)."

// commentsOnContribution tells a caller how to find the comments of a legacy contribution.
func commentsOnContribution(legacyType string) string {
	return "Comments are replies now: find the reply whose legacy_type is \"" + legacyType + "\" and legacy_id is " +
		"{id} in GET /v1/posts/{post_id}/replies; its comments are the replies whose parent_reply_id is that " +
		"reply's id." + legacyCommentsAsReplies
}

// LegacyReadRetirements lists every retired legacy read route (task idx 73 step 3: adapt, then
// retire). The type-specific statistics were adapters over the overview knowledge aggregate
// (idx 72); their counts live only in GET /v1/overview now. The legacy feed and the legacy typed
// lists were adapters over the canonical GET /v1/posts list (idx 71); each route names the query
// that served it. The legacy comment lists read comments the cutover turns into replies
// (MigrateContributions); each names where GET /v1/posts/{id}/replies lists them. Like the
// retired writes, each answers every caller 410 ENDPOINT_RETIRED naming the replacement, with no
// sunset period. TestLegacyReadRetirements_CoverEveryRetiredReadFamilyAndNameAServedReplacement
// pins it to the GET routes of the retired read families.
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
	{"GET /v1/feed", "GET /v1/posts",
		"Call GET /v1/posts?sort=newest with the same query parameters." + legacyFeedItemsAsPosts},
	{"GET /v1/feed/stuck", "GET /v1/posts",
		"Call GET /v1/posts?type=problem&needs_help=true&sort=newest with the same query parameters: " +
			"needs_help lists problems in status in_progress or with a stuck approach." + legacyFeedItemsAsPosts},
	{"GET /v1/feed/unanswered", "GET /v1/posts",
		"Call GET /v1/posts?type=question&has_answer=false&sort=newest with the same query parameters: " +
			"has_answer=false lists questions without an answer." + legacyFeedItemsAsPosts},
	{"GET /v1/problems", "GET /v1/posts",
		"Call GET /v1/posts?type=problem with the same query parameters other than type (the route replaced " +
			"a caller's type with problem)." + legacyTypedListAsPosts},
	{"GET /v1/questions", "GET /v1/posts",
		"Call GET /v1/posts?type=question with the same query parameters other than type (the route replaced " +
			"a caller's type with question): has_answer=true or has_answer=false still lists questions with or " +
			"without an answer." + legacyTypedListAsPosts},
	{"GET /v1/ideas", "GET /v1/posts",
		"Call GET /v1/posts?type=idea with the same query parameters other than type (the route replaced " +
			"a caller's type with idea)." + legacyTypedListAsPosts},
	{"GET /v1/posts/{id}/comments", "GET /v1/posts/{id}/replies",
		"Call GET /v1/posts/{id}/replies; the post id is unchanged. A comment on the post is a top-level reply " +
			"(no parent_reply_id) whose legacy_type is \"comment\"; replies written since the cutover carry no " +
			"legacy_type." + legacyCommentsAsReplies},
	{"GET /v1/approaches/{id}/comments", "GET /v1/posts/{id}/replies", commentsOnContribution("approach")},
	{"GET /v1/answers/{id}/comments", "GET /v1/posts/{id}/replies", commentsOnContribution("answer")},
	{"GET /v1/responses/{id}/comments", "GET /v1/posts/{id}/replies", commentsOnContribution("response")},
}

// mountRetiredLegacyReads registers every retired read on the /v1 router, outside every
// auth, rate-limit and content-gate middleware, as mountRetiredLegacyWrites does.
func mountRetiredLegacyReads(r chi.Router) {
	for _, ret := range LegacyReadRetirements {
		method, path, _ := strings.Cut(ret.Route, " ")
		r.Method(method, strings.TrimPrefix(path, "/v1"), retiredLegacyRoute(ret))
	}
}
