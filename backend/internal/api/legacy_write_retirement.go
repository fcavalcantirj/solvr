package api

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/fcavalcantirj/solvr/internal/api/response"
)

// ErrCodeEndpointRetired is the error code of a retired legacy route.
const ErrCodeEndpointRetired = "ENDPOINT_RETIRED"

// LegacyRouteRetirement is the migration answer of one retired legacy route: a write (task
// idx 52 step 3) or a read (legacy_read_retirement.go, task idx 73 step 3). Owner decision
// 2026-09-30: the legacy writes are deleted at cutover with no transition adapters, so every
// call answers 410 ENDPOINT_RETIRED naming what to use instead.
type LegacyRouteRetirement struct {
	// Route is the "METHOD /path" template exactly as RouteFamilies lists it.
	Route string
	// Replacement is the canonical "METHOD /path" to call instead; "" when the legacy
	// command has no canonical equivalent.
	Replacement string
	// Instructions tell an old client how to move the call to the canonical model.
	Instructions string
}

const (
	createPostInstead = "Send title, description and tags to POST /v1/posts and leave type out: " +
		"a canonical post has no legacy type."
	replyOnPostInstead = "Send the text as body to POST /v1/posts/{id}/replies; the post id is unchanged."
	threadReplyInstead = "Contributions are replies now, with their own ids: find the reply whose legacy_type is " +
		"%q and legacy_id is {id} in GET /v1/posts/{post_id}/replies, then POST /v1/posts/{post_id}/replies " +
		"with body and parent_reply_id set to that reply's id."
	editReplyInstead = "Answers are replies now, with their own ids: find the reply whose legacy_type is " +
		"\"answer\" and legacy_id is {id} in GET /v1/posts/{post_id}/replies, then call %s on that reply's id."
	outcomeAsReplyInstead = "Record the outcome as a reply (POST /v1/posts/{id}/replies) or a new post (POST /v1/posts)."
)

// LegacyWriteRetirements lists every retired legacy write route. The test
// TestLegacyWriteRetirements_CoverEveryLegacyWriteRouteAndNameAServedReplacement pins it to the
// non-GET routes of the legacy-typed-writes, legacy-comments and legacy-status-commands families.
var LegacyWriteRetirements = []LegacyRouteRetirement{
	{"POST /v1/problems", "POST /v1/posts", createPostInstead},
	{"POST /v1/questions", "POST /v1/posts", createPostInstead},
	{"POST /v1/ideas", "POST /v1/posts", createPostInstead},
	{"POST /v1/problems/{id}/approaches", "POST /v1/posts/{id}/replies",
		"Send the approach (angle, method, outcome) as one Markdown body to POST /v1/posts/{id}/replies; the post id is unchanged."},
	{"POST /v1/questions/{id}/answers", "POST /v1/posts/{id}/replies", replyOnPostInstead},
	{"POST /v1/ideas/{id}/responses", "POST /v1/posts/{id}/replies", replyOnPostInstead},
	{"POST /v1/approaches/{id}/progress", "POST /v1/posts/{id}/replies", threadInstructions("approach")},
	{"PATCH /v1/answers/{id}", "PATCH /v1/replies/{id}", editInstructions("PATCH /v1/replies/{id} with body")},
	{"DELETE /v1/answers/{id}", "DELETE /v1/replies/{id}", editInstructions("DELETE /v1/replies/{id}")},
	{"POST /v1/answers/{id}/vote", "POST /v1/replies/{id}/vote", editInstructions("POST /v1/replies/{id}/vote")},
	{"POST /v1/posts/{id}/comments", "POST /v1/posts/{id}/replies", replyOnPostInstead},
	{"POST /v1/approaches/{id}/comments", "POST /v1/posts/{id}/replies", threadInstructions("approach")},
	{"POST /v1/answers/{id}/comments", "POST /v1/posts/{id}/replies", threadInstructions("answer")},
	{"POST /v1/responses/{id}/comments", "POST /v1/posts/{id}/replies", threadInstructions("response")},
	{"DELETE /v1/comments/{id}", "DELETE /v1/replies/{id}",
		"Comments are replies now, with their own ids: find the reply whose legacy_type is \"comment\" and " +
			"legacy_id is {id} in GET /v1/posts/{post_id}/replies, then DELETE /v1/replies/{reply_id}."},
	{"PATCH /v1/approaches/{id}", "", "Approach status has no canonical field. " + outcomeAsReplyInstead},
	{"POST /v1/approaches/{id}/verify", "", "Approach verification has no canonical field. " + outcomeAsReplyInstead},
	{"POST /v1/questions/{id}/accept/{aid}", "", "Accepting an answer has no canonical command. " + outcomeAsReplyInstead},
	{"POST /v1/ideas/{id}/evolve", "", "Idea evolution has no canonical command. " + outcomeAsReplyInstead},
}

func threadInstructions(legacyType string) string { return fmt.Sprintf(threadReplyInstead, legacyType) }

func editInstructions(call string) string { return fmt.Sprintf(editReplyInstead, call) }

// retiredRouteDetails is error.details of a retired route; replacement is null when there is
// no canonical equivalent.
type retiredRouteDetails struct {
	RetiredRoute string  `json:"retired_route"`
	Replacement  *string `json:"replacement"`
	Instructions string  `json:"instructions"`
}

// retirementAnswer is the error message and details every call to ret's route receives; the
// served API document (openapi_retired_writes.go) publishes the same message.
func retirementAnswer(ret LegacyRouteRetirement) (string, retiredRouteDetails) {
	details := retiredRouteDetails{RetiredRoute: ret.Route, Instructions: ret.Instructions}
	message := ret.Route + " was retired with the canonical knowledge model"
	if ret.Replacement != "" {
		replacement := ret.Replacement
		details.Replacement = &replacement
		message += "; use " + replacement + " instead."
	} else {
		message += " and has no canonical equivalent."
	}
	return message, details
}

// retiredLegacyRoute answers every call to ret's route with 410 ENDPOINT_RETIRED. It reads
// no body, checks no credential and touches no storage: the answer is the same for everyone.
func retiredLegacyRoute(ret LegacyRouteRetirement) http.HandlerFunc {
	message, details := retirementAnswer(ret)
	return func(w http.ResponseWriter, _ *http.Request) {
		response.WriteErrorWithDetails(w, http.StatusGone, ErrCodeEndpointRetired, message, details)
	}
}

// mountRetiredLegacyWrites registers every retired route on the /v1 router, outside every
// auth, rate-limit and content-gate middleware.
func mountRetiredLegacyWrites(r chi.Router) {
	for _, ret := range LegacyWriteRetirements {
		method, path, _ := strings.Cut(ret.Route, " ")
		r.Method(method, strings.TrimPrefix(path, "/v1"), retiredLegacyRoute(ret))
	}
}
