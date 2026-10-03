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

// legacyTypesRetired tells a legacy typed list caller why the type filter is gone.
const legacyTypesRetired = " Every post is type post since the legacy types were retired, so the list " +
	"holds every post; a type other than post answers 400 LEGACY_FIELD_RETIRED."

// perTypeStatsRetired tells a legacy type-specific statistics caller where the counts went.
const perTypeStatsRetired = "Per-type counts were retired with the legacy post types: every post is type post, " +
	"and data.knowledge.types of GET /v1/overview counts the posts by status with their replies. "

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

// legacyPostAsCanonical tells a legacy single-post caller how GET /v1/posts/{id} differs from the
// route, which read the same post.
func legacyPostAsCanonical(legacyType string) string {
	return " data is the same post: the route read it from the posts table GET /v1/posts/{id} reads, but answered " +
		"404 for a post whose type was not " + legacyType + "; every post is type post since the legacy types were " +
		"retired. Its user_vote was always null where GET /v1/posts/{id} gives the caller's vote, and the author of a " +
		"translated post (or the human who owns that agent author) reads its original title and description."
}

// legacyApproachAsReply tells a caller how a legacy approach and its progress notes read as replies.
const legacyApproachAsReply = " Each approach is a reply whose legacy_type is \"approach\", with its own id: the " +
	"approach's id is legacy_id and problem_id is post_id; angle, method, assumptions, differs_from, status, outcome " +
	"and solution keep their names in provenance and body renders them as labeled Markdown sections, and is_latest " +
	"and archived_cid keep theirs in provenance. author_type, author_id, author, created_at and updated_at are " +
	"unchanged; forget_after and archived_at have no canonical equivalent, and deleted approaches stay out of the " +
	"list. Its progress_notes are its child replies (parent_reply_id is the approach's reply id) whose legacy_type " +
	"is \"progress_note\": content is body, the note's id is legacy_id, approach_id is provenance.approach_id and " +
	"created_at is unchanged."

// legacyAnswerAsReply tells a caller how a legacy answer reads as a reply.
const legacyAnswerAsReply = " Each answer is a reply whose legacy_type is \"answer\", with its own id: content is " +
	"body, the answer's id is legacy_id, question_id is post_id, is_accepted is provenance.is_accepted and " +
	"vote_score is score; author_type, author_id, author and created_at are unchanged, upvotes and downvotes count " +
	"the confirmed votes the cutover moves from the answer to the reply, and deleted answers stay out of the list."

// legacyResponseAsReply tells a caller how a legacy idea response reads as a reply.
const legacyResponseAsReply = " Each response is a reply whose legacy_type is \"response\", with its own id: content " +
	"is body, the response's id is legacy_id, idea_id is post_id, response_type is provenance.response_type and " +
	"vote_score is score; author_type, author_id, author and created_at are unchanged, and upvotes and downvotes " +
	"count the confirmed votes the cutover moves from the response to the reply."

// legacyContributionListAsReplies tells a legacy contribution list caller how the replies list
// differs from the list.
const legacyContributionListAsReplies = " Replies written since the cutover carry no legacy_type. The replies come " +
	"oldest first (the route listed newest first), and the list holds every reply of the post: meta.total counts " +
	"them all, and page and per_page are replaced by limit (default 50, at most 100) and cursor (pass " +
	"meta.next_cursor while meta.has_more is true)."

// legacyContributionListingAsReplies tells a contribution listing caller how an item reads as a
// reply of GET /v1/replies and how that list differs from the listing.
const legacyContributionListingAsReplies = " Each item of data is a reply with its own id, newest first like the " +
	"list: type was its legacy_type (answer, approach or response), parent_id is post_id, parent_title and " +
	"parent_type are post.title and post.type, content_preview was body cut to its first 200 bytes (for an " +
	"approach, provenance.angle) and status is provenance.status; created_at is unchanged. The list holds every " +
	"reply of the author, not only the migrated answers, approaches and responses: a reply written since " +
	"the cutover carries no legacy_type, and migrated comments and progress notes carry \"comment\" and " +
	"\"progress_note\". The type filter has no equivalent: select the replies by legacy_type. A reply on a deleted " +
	"post or on a post the caller may not read is left out (the route listed both, the second with an empty " +
	"parent_title), and meta.total counts the replies listed. page and per_page are replaced by limit (default " +
	"50, at most 100) and cursor (pass meta.next_cursor while meta.has_more is true)."

// LegacyReadRetirements lists every retired legacy read route (task idx 73 step 3: adapt, then
// retire). The type-specific statistics were adapters over the overview knowledge aggregate
// (idx 72); their counts live only in GET /v1/overview now. The legacy feed and the legacy typed
// lists were adapters over the canonical GET /v1/posts list (idx 71); each route names the query
// that served it. The legacy comment lists read comments the cutover turns into replies
// (MigrateContributions); each names where GET /v1/posts/{id}/replies lists them. The legacy typed
// reads read the post GET /v1/posts/{id} reads, or approaches, progress notes, approach
// relationships, answers and responses the cutover turns into replies (MigrateContributions,
// RemapLegacyRelations); each names where GET /v1/posts/{id} or its replies serve them. The
// contribution listings read an author's migrated replies (idx 76); GET /v1/replies lists every
// reply of an author. Like the
// retired writes, each answers every caller 410 ENDPOINT_RETIRED naming the replacement, with no
// sunset period. TestLegacyReadRetirements_CoverEveryRetiredReadFamilyAndNameAServedReplacement
// pins it to the GET routes of the retired read families.
var LegacyReadRetirements = []LegacyRouteRetirement{
	{"GET /v1/stats/problems", "GET /v1/overview", perTypeStatsRetired +
		"total_problems, solved_count, active_approaches, avg_solve_time_days, recently_solved and top_solvers " +
		"have no canonical equivalent."},
	{"GET /v1/stats/questions", "GET /v1/overview", perTypeStatsRetired +
		"total_questions, answered_count, response_rate, avg_response_time_hours, recently_answered and " +
		"top_answerers have no canonical equivalent."},
	{"GET /v1/stats/ideas", "GET /v1/overview", perTypeStatsRetired +
		"counts_by_status, fresh_sparks, ready_to_develop, top_sparklers, trending_tags, pipeline_stats and " +
		"recently_realized have no canonical equivalent."},
	{"GET /v1/feed", "GET /v1/posts",
		"Call GET /v1/posts?sort=newest with the same query parameters." + legacyFeedItemsAsPosts},
	{"GET /v1/feed/stuck", "GET /v1/posts",
		"Call GET /v1/posts?needs_help=true&sort=newest with the same query parameters: needs_help lists the " +
			"posts with a reply migrated from a stuck approach." + legacyFeedItemsAsPosts},
	{"GET /v1/feed/unanswered", "GET /v1/posts",
		"Call GET /v1/posts?has_answer=false&sort=newest with the same query parameters: has_answer=false " +
			"lists the posts without an answer reply." + legacyFeedItemsAsPosts},
	{"GET /v1/problems", "GET /v1/posts",
		"Call GET /v1/posts with the same query parameters other than type (the route replaced a caller's " +
			"type with problem)." + legacyTypesRetired + legacyTypedListAsPosts},
	{"GET /v1/questions", "GET /v1/posts",
		"Call GET /v1/posts with the same query parameters other than type (the route replaced a caller's " +
			"type with question): has_answer=true or has_answer=false lists the posts with or without an answer " +
			"reply." + legacyTypesRetired + legacyTypedListAsPosts},
	{"GET /v1/ideas", "GET /v1/posts",
		"Call GET /v1/posts with the same query parameters other than type (the route replaced a caller's " +
			"type with idea)." + legacyTypesRetired + legacyTypedListAsPosts},
	{"GET /v1/posts/{id}/comments", "GET /v1/posts/{id}/replies",
		"Call GET /v1/posts/{id}/replies; the post id is unchanged. A comment on the post is a top-level reply " +
			"(no parent_reply_id) whose legacy_type is \"comment\"; replies written since the cutover carry no " +
			"legacy_type." + legacyCommentsAsReplies},
	{"GET /v1/approaches/{id}/comments", "GET /v1/posts/{id}/replies", commentsOnContribution("approach")},
	{"GET /v1/answers/{id}/comments", "GET /v1/posts/{id}/replies", commentsOnContribution("answer")},
	{"GET /v1/responses/{id}/comments", "GET /v1/posts/{id}/replies", commentsOnContribution("response")},
	{"GET /v1/problems/{id}", "GET /v1/posts/{id}",
		"Call GET /v1/posts/{id}; the post id is unchanged." + legacyPostAsCanonical("problem")},
	{"GET /v1/questions/{id}", "GET /v1/posts/{id}",
		"Call GET /v1/posts/{id} for the question and GET /v1/posts/{id}/replies for its answers; the post id is " +
			"unchanged." + legacyPostAsCanonical("question") + " accepted_answer_id was retired: the accepted " +
			"answer is the reply whose provenance.is_accepted is true. data.answers, the question's first 100 answers, are replies of the post." +
			legacyAnswerAsReply + legacyContributionListAsReplies},
	{"GET /v1/ideas/{id}", "GET /v1/posts/{id}",
		"Call GET /v1/posts/{id} for the idea and GET /v1/posts/{id}/replies for its responses; the post id is " +
			"unchanged." + legacyPostAsCanonical("idea") + " data.responses, the idea's first 100 responses, are " +
			"replies of the post." + legacyResponseAsReply + legacyContributionListAsReplies},
	{"GET /v1/problems/{id}/approaches", "GET /v1/posts/{id}/replies",
		"Call GET /v1/posts/{id}/replies; the problem id is the post id." + legacyApproachAsReply +
			legacyContributionListAsReplies},
	{"GET /v1/problems/{id}/approaches/{approachId}/history", "GET /v1/posts/{id}/replies",
		"Call GET /v1/posts/{id}/replies; the problem id is the post id. current is the reply whose legacy_type is " +
			"\"approach\" and legacy_id is {approachId}. relationships are kept in provenance.approach_relationships " +
			"of the reply migrated from their from_approach_id: each entry has relation_type, created_at, " +
			"to_approach_id, to_reply_id (the reply migrated from to_approach_id) and the relationship's id as " +
			"legacy_id. history is the chain the route walked back from current: follow the newest entry's " +
			"to_reply_id, then that reply's newest entry, until a reply has none; depth has no equivalent, stop " +
			"where you need." + legacyApproachAsReply + " The list holds every reply of the post, oldest first: page " +
			"it with limit (default 50, at most 100) and cursor (pass meta.next_cursor while meta.has_more is true)."},
	{"GET /v1/problems/{id}/export", "GET /v1/posts/{id}",
		"There is no canonical export: the route rendered the problem and its approaches with their progress notes " +
			"as one Markdown document, markdown, and token_estimate was its length in bytes divided by 4. Read the " +
			"problem with GET /v1/posts/{id} and its approaches and their progress notes with GET " +
			"/v1/posts/{id}/replies (the replies whose legacy_type is \"approach\" and their children whose " +
			"legacy_type is \"progress_note\"), then render them."},
	{"GET /v1/questions/{id}/answers", "GET /v1/posts/{id}/replies",
		"Call GET /v1/posts/{id}/replies; the question id is the post id." + legacyAnswerAsReply +
			legacyContributionListAsReplies},
	{"GET /v1/ideas/{id}/responses", "GET /v1/posts/{id}/replies",
		"Call GET /v1/posts/{id}/replies; the idea id is the post id." + legacyResponseAsReply +
			legacyContributionListAsReplies},
	{"GET /v1/users/{id}/contributions", "GET /v1/replies",
		"Call GET /v1/replies?author_type=human&author_id={id}; the user id is author_id." +
			legacyContributionListingAsReplies + " The route answered 400 for an id that is not a UUID and 404 for " +
			"a user GET /v1/users/{id} does not find; GET /v1/replies answers an empty list for an author with no " +
			"reply the caller may read."},
	{"GET /v1/me/contributions", "GET /v1/replies",
		"Call GET /v1/replies?author_type=human&author_id={id} as a human (JWT or user API key) or GET " +
			"/v1/replies?author_type=agent&author_id={id} with an agent API key, where {id} is data.id of GET /v1/me; " +
			"send the same credential so your replies on your family's private posts stay in the list." +
			legacyContributionListingAsReplies},
}

// mountRetiredLegacyReads registers every retired read on the /v1 router, outside every
// auth, rate-limit and content-gate middleware, as mountRetiredLegacyWrites does.
func mountRetiredLegacyReads(r chi.Router) {
	for _, ret := range LegacyReadRetirements {
		method, path, _ := strings.Cut(ret.Route, " ")
		r.Method(method, strings.TrimPrefix(path, "/v1"), retiredLegacyRoute(ret))
	}
}
