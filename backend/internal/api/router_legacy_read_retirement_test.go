package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Task idx 73 step 3, "adapt then retire": the type-specific statistics (GET
// /v1/stats/problems|questions|ideas) were adapters over the overview knowledge aggregate
// (idx 72), and the legacy feed (GET /v1/feed, /v1/feed/stuck, /v1/feed/unanswered) and the
// legacy typed lists (GET /v1/problems, /v1/questions, /v1/ideas) adapters over the canonical
// GET /v1/posts list (idx 71). The legacy comment lists (GET /v1/{posts,approaches,answers,
// responses}/{id}/comments) read the comments the cutover turns into replies (MigrateContributions),
// so GET /v1/posts/{id}/replies serves them. The legacy typed reads (GET /v1/{problems,questions,
// ideas}/{id} and the problem's approaches, approach history and export, the question's answers
// and the idea's responses) read the same post as GET /v1/posts/{id}, or contributions the cutover
// turns into replies (MigrateContributions, RemapLegacyRelations). The contribution listings (GET
// /v1/users/{id}/contributions, GET /v1/me/contributions) listed an author's migrated replies,
// which GET /v1/replies?author_type=&author_id= lists. They are now retired the way
// the legacy writes were (idx 52): every caller gets the same 410 ENDPOINT_RETIRED naming the canonical
// replacement, where the data is there and where each legacy field went, and the API document
// and SPEC.md Part 26 publish that exact answer (step 5).

// retiredReadFamilies are the RouteFamilies whose GET routes answer the migration error. Their
// other routes, if any, are retired writes (legacy-comments: TestLegacyWriteRetirements_*).
var retiredReadFamilies = map[string]bool{
	"type-specific-statistics": true,
	"legacy-feed":              true,
	"legacy-typed-discovery":   true,
	"legacy-comments":          true,
	"legacy-typed-reads":       true,
	"contribution-listings":    true,
}

// retiredReadDestinations is where each retired read's data is served now; its instructions
// must name it exactly. A feed route or typed list names the GET /v1/posts query its adapter
// served.
var retiredReadDestinations = map[string]string{
	"GET /v1/stats/problems":      "data.knowledge.types",
	"GET /v1/stats/questions":     "data.knowledge.types",
	"GET /v1/stats/ideas":         "data.knowledge.types",
	"GET /v1/feed":                "GET /v1/posts?sort=newest",
	"GET /v1/feed/stuck":          "GET /v1/posts?type=problem&needs_help=true&sort=newest",
	"GET /v1/feed/unanswered":     "GET /v1/posts?type=question&has_answer=false&sort=newest",
	"GET /v1/problems":            "GET /v1/posts?type=problem ",
	"GET /v1/questions":           "GET /v1/posts?type=question ",
	"GET /v1/ideas":               "GET /v1/posts?type=idea ",
	"GET /v1/posts/{id}/comments": "Call GET /v1/posts/{id}/replies",
	"GET /v1/approaches/{id}/comments": `legacy_type is "approach" and legacy_id is {id} in ` +
		"GET /v1/posts/{post_id}/replies",
	"GET /v1/answers/{id}/comments": `legacy_type is "answer" and legacy_id is {id} in ` +
		"GET /v1/posts/{post_id}/replies",
	"GET /v1/responses/{id}/comments": `legacy_type is "response" and legacy_id is {id} in ` +
		"GET /v1/posts/{post_id}/replies",
	"GET /v1/problems/{id}": "Call GET /v1/posts/{id}; the post id is unchanged.",
	"GET /v1/questions/{id}": "Call GET /v1/posts/{id} for the question and GET /v1/posts/{id}/replies " +
		"for its answers",
	"GET /v1/ideas/{id}": "Call GET /v1/posts/{id} for the idea and GET /v1/posts/{id}/replies for its " +
		"responses",
	"GET /v1/problems/{id}/approaches": "Call GET /v1/posts/{id}/replies; the problem id is the post id.",
	"GET /v1/problems/{id}/approaches/{approachId}/history": `current is the reply whose legacy_type is ` +
		`"approach" and legacy_id is {approachId}`,
	"GET /v1/problems/{id}/export": "Read the problem with GET /v1/posts/{id} and its approaches and their " +
		"progress notes with GET /v1/posts/{id}/replies",
	"GET /v1/questions/{id}/answers": "Call GET /v1/posts/{id}/replies; the question id is the post id.",
	"GET /v1/ideas/{id}/responses":   "Call GET /v1/posts/{id}/replies; the idea id is the post id.",
	"GET /v1/users/{id}/contributions": "Call GET /v1/replies?author_type=human&author_id={id}; the user id is " +
		"author_id.",
	"GET /v1/me/contributions": "GET /v1/replies?author_type=agent&author_id={id} with an agent API key, where " +
		"{id} is data.id of GET /v1/me",
}

// retiredReadFields are the top-level data fields each retired read used to return; its
// instructions must say where each one went, or that it has no canonical equivalent. A typed list
// was served by the canonical list itself, so it names its envelope (data and meta) as unchanged.
var retiredReadFields = map[string][]string{
	"GET /v1/stats/problems": {"total_problems", "solved_count", "active_approaches", "avg_solve_time_days",
		"recently_solved", "top_solvers"},
	"GET /v1/stats/questions": {"total_questions", "answered_count", "response_rate", "avg_response_time_hours",
		"recently_answered", "top_answerers"},
	"GET /v1/stats/ideas": {"counts_by_status", "fresh_sparks", "ready_to_develop", "top_sparklers",
		"trending_tags", "pipeline_stats", "recently_realized"},
	"GET /v1/feed":                     legacyFeedItemFields,
	"GET /v1/feed/stuck":               legacyFeedItemFields,
	"GET /v1/feed/unanswered":          legacyFeedItemFields,
	"GET /v1/problems":                 {"data", "meta"},
	"GET /v1/questions":                {"data", "meta"},
	"GET /v1/ideas":                    {"data", "meta"},
	"GET /v1/posts/{id}/comments":      legacyCommentListFields,
	"GET /v1/approaches/{id}/comments": legacyCommentListFields,
	"GET /v1/answers/{id}/comments":    legacyCommentListFields,
	"GET /v1/responses/{id}/comments":  legacyCommentListFields,
	"GET /v1/problems/{id}":            {"data", "user_vote"},
	"GET /v1/questions/{id}":           append([]string{"data", "user_vote", "accepted_answer_id", "answers"}, legacyAnswerFields...),
	"GET /v1/ideas/{id}":               append([]string{"data", "user_vote", "responses"}, legacyResponseFields...),
	"GET /v1/problems/{id}/approaches": append(append([]string{}, legacyApproachFields...), legacyListMetaFields...),
	"GET /v1/problems/{id}/approaches/{approachId}/history": append([]string{"current", "history",
		"relationships", "from_approach_id", "to_approach_id", "relation_type", "depth"}, legacyApproachFields...),
	"GET /v1/problems/{id}/export":   {"markdown", "token_estimate"},
	"GET /v1/questions/{id}/answers": append(append([]string{}, legacyAnswerFields...), legacyListMetaFields...),
	"GET /v1/ideas/{id}/responses":   append(append([]string{}, legacyResponseFields...), legacyListMetaFields...),
	"GET /v1/users/{id}/contributions": append(append([]string{"type"}, legacyContributionItemFields...),
		legacyListMetaFields...),
	"GET /v1/me/contributions": append(append([]string{"type"}, legacyContributionItemFields...),
		legacyListMetaFields...),
}

// legacyContributionItemFields are the fields of a legacy contribution item (models.ContributionItem)
// other than type, which is also the name of the list's filter.
var legacyContributionItemFields = []string{"id", "parent_id", "parent_title", "parent_type", "content_preview",
	"status", "created_at"}

// legacyApproachFields are the fields of a legacy approach (models.ApproachWithAuthor; the list left
// deleted approaches out, so deleted_at never appeared) and of its progress notes
// (models.ProgressNote: id, approach_id, content, created_at).
var legacyApproachFields = []string{"id", "problem_id", "author_type", "author_id", "angle", "method", "assumptions",
	"differs_from", "status", "outcome", "solution", "created_at", "updated_at", "is_latest", "forget_after",
	"archived_at", "archived_cid", "author", "progress_notes", "approach_id", "content"}

// legacyAnswerFields are the fields of a legacy answer (models.AnswerWithAuthor; deleted answers
// were left out).
var legacyAnswerFields = []string{"id", "question_id", "author_type", "author_id", "content", "is_accepted",
	"upvotes", "downvotes", "created_at", "author", "vote_score"}

// legacyResponseFields are the fields of a legacy idea response (models.ResponseWithAuthor).
var legacyResponseFields = []string{"id", "idea_id", "author_type", "author_id", "content", "response_type",
	"upvotes", "downvotes", "created_at", "author", "vote_score"}

// legacyListMetaFields are the meta fields of a legacy contribution list.
var legacyListMetaFields = []string{"total", "page", "per_page", "has_more"}

// legacyCommentListFields are the fields of a legacy comment (models.CommentWithAuthor; the list
// left deleted comments out, so deleted_at never appeared) and of the list's meta.
var legacyCommentListFields = []string{"id", "target_type", "target_id", "author_type", "author_id", "content",
	"author", "created_at", "total", "page", "per_page", "has_more"}

// legacyFeedItemFields are the fields of a legacy feed item (models.FeedItem).
var legacyFeedItemFields = []string{"id", "type", "title", "snippet", "tags", "status", "author", "vote_score",
	"answer_count", "approach_count", "comment_count", "created_at"}

func TestLegacyReadRetirements_CoverEveryRetiredReadFamilyAndNameAServedReplacement(t *testing.T) {
	want := map[string]RouteFamily{}
	keep := map[string]bool{}
	var others []string
	for _, f := range RouteFamilies {
		for _, route := range f.Routes {
			if f.Disposition == DispositionKeep {
				keep[route] = true
			}
			if retiredReadFamilies[f.Name] {
				require.Equal(t, DispositionRetire, f.Disposition, "%s must be a retire family", f.Name)
				if !strings.HasPrefix(route, "GET ") {
					others = append(others, route) // this table retires reads only; see below
					continue
				}
				want[route] = f
			}
		}
	}
	require.Len(t, want, 23, "the GET routes of %v", retiredReadFamilies)

	writes := map[string]bool{}
	for _, w := range LegacyWriteRetirements {
		writes[w.Route] = true
	}
	for _, route := range others {
		require.True(t, writes[route], "%s: a retired read family's other routes are retired writes (26.6)", route)
	}
	seen := map[string]bool{}
	for _, ret := range LegacyReadRetirements {
		family, ok := want[ret.Route]
		require.True(t, ok, "%s is not a GET route of %v", ret.Route, retiredReadFamilies)
		require.False(t, seen[ret.Route], "%s is listed twice", ret.Route)
		seen[ret.Route] = true
		require.False(t, writes[ret.Route], "%s is also a retired write", ret.Route)
		require.True(t, keep[ret.Replacement], "%s: replacement %q must be a served route of a keep family",
			ret.Route, ret.Replacement)
		require.Contains(t, family.Canonical, ret.Replacement,
			"%s: the replacement must be the family's recorded canonical destination", ret.Route)
		destination, ok := retiredReadDestinations[ret.Route]
		require.True(t, ok, "%s: no recorded destination to check", ret.Route)
		require.Contains(t, ret.Instructions, destination, "%s: name where the data is served now", ret.Route)
		fields, ok := retiredReadFields[ret.Route]
		require.True(t, ok, "%s: no legacy field list to check", ret.Route)
		for _, field := range fields {
			require.Contains(t, ret.Instructions, field, "%s: say where %s went", ret.Route, field)
		}
	}
	for route := range want {
		require.True(t, seen[route], "%s has no retirement entry", route)
	}
}

// Every caller, anonymous or authenticated, gets the migration error; the replacement it
// names serves the knowledge figures the instructions point to.
func TestLegacyReadRoutes_AnswerTheMigrationErrorToEveryCaller(t *testing.T) {
	ts, _, pool := newStatusContractServer(t)
	callers := statusContractIdentities(t, ts, pool)

	for _, ret := range LegacyReadRetirements {
		_, path, _ := strings.Cut(ret.Route, " ")
		message, _ := retirementAnswer(ret)
		for name, bearer := range callers {
			got, err := callStatusContract(http.DefaultClient, http.MethodGet, ts.URL+path, bearer, "")
			require.NoError(t, err, path)
			require.Equal(t, http.StatusGone, got.status, "%s as %s: %s", ret.Route, name, got.body)
			require.Equal(t, ErrCodeEndpointRetired, got.code, "%s as %s: %s", ret.Route, name, got.body)
			require.Equal(t, message, got.message, "%s as %s", ret.Route, name)
			require.NotEmpty(t, got.requestID, "%s: the error envelope carries request_id", ret.Route)
			require.Equal(t, got.headerID, got.requestID, "%s: request_id equals X-Request-ID", ret.Route)

			var envelope struct {
				Error struct {
					Details map[string]json.RawMessage `json:"details"`
				} `json:"error"`
			}
			require.NoError(t, json.Unmarshal([]byte(got.body), &envelope), got.body)
			details := envelope.Error.Details
			require.JSONEq(t, fmt.Sprintf("%q", ret.Route), string(details["retired_route"]), got.body)
			require.JSONEq(t, fmt.Sprintf("%q", ret.Replacement), string(details["replacement"]), got.body)
			require.JSONEq(t, fmt.Sprintf("%q", ret.Instructions), string(details["instructions"]), got.body)
		}
	}

	var ov overviewKnowledgeResp
	getJSON(t, ts.URL+"/v1/overview", &ov)
	k := knowledgeByType(ov)
	for _, legacyType := range []string{"problem", "question", "idea"} {
		_, ok := k[legacyType]
		assert.True(t, ok, "GET /v1/overview data.knowledge.types has no %q entry", legacyType)
	}
}

func TestOpenAPIRetiredReads_DocumentedAsGoneWithTheServedAnswer(t *testing.T) {
	spec := servedSpec(t)
	for _, ret := range LegacyReadRetirements {
		key, op := retiredOperation(t, spec, ret)
		assert.Equal(t, true, op["deprecated"], "%s must be marked deprecated", key)

		responses := op["responses"].(map[string]interface{})
		statuses := make([]string, 0, len(responses))
		for status := range responses {
			statuses = append(statuses, status)
		}
		assert.Equal(t, []string{"410"}, statuses, "%s must document only 410, never a success", key)
		assert.Equal(t, "#/components/responses/EndpointRetired", refName(responses["410"]), key)

		retired := at(t, op, "x-solvr-retired").(map[string]interface{})
		assert.Equal(t, ret.Replacement, retired["replacement"], key)
		assert.Equal(t, ret.Instructions, retired["instructions"], key)

		method, path, _ := strings.Cut(ret.Route, " ")
		w := httptest.NewRecorder()
		retiredLegacyRoute(ret).ServeHTTP(w, httptest.NewRequest(method, path, nil))
		requireRetiredRecorder(t, w, ret.Route)
		var envelope struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &envelope))
		assert.True(t, strings.HasPrefix(op["description"].(string), envelope.Error.Message),
			"%s: description %q must open with the served message %q", key, op["description"], envelope.Error.Message)
		assert.Contains(t, op["description"], ret.Instructions, key)
	}
	gone := deref(t, spec, at(t, spec, "components", "responses", "EndpointRetired")).(map[string]interface{})
	assert.NotContains(t, gone["description"], "write route", "EndpointRetired describes retired reads too")
}

// SPEC.md Part 26 maps every retired read to its replacement in 26.7, and no longer
// describes the retired adapters as served.
func TestLegacyReadRetirements_PublishedInSpecMigrationNotes(t *testing.T) {
	raw, err := os.ReadFile("../../../SPEC.md")
	require.NoError(t, err)
	spec := string(raw)
	start := strings.Index(spec, "# Part 26: Canonical Knowledge API and Route Dispositions")
	require.GreaterOrEqual(t, start, 0, "SPEC.md has no Part 26")
	part := spec[start:]
	if next := strings.Index(part[1:], "\n# Part "); next >= 0 {
		part = part[:next+1]
	}
	section := func(heading string) string {
		at := strings.Index(part, heading)
		require.GreaterOrEqual(t, at, 0, "Part 26 has no %q", heading)
		s := part[at:]
		if next := strings.Index(s[1:], "\n## "); next >= 0 {
			s = s[:next+1]
		}
		return s
	}

	notes := section("## 26.7 Retired Legacy Reads")
	for _, fact := range []string{"410", "`ENDPOINT_RETIRED`", "no sunset period", "retired_route", "replacement", "instructions"} {
		assert.Contains(t, notes, fact, "the retired-read notes must state %s", fact)
	}
	for _, ret := range LegacyReadRetirements {
		assert.Contains(t, notes, "| `"+ret.Route+"` | `"+ret.Replacement+"` | "+ret.Instructions+" |",
			"26.7 does not map %s to %s with its instructions", ret.Route, ret.Replacement)
		_, path, _ := strings.Cut(ret.Route, " ")
		assert.NotContains(t, section("## 26.5 Runtime Adapters"), "`"+path+"`",
			"26.5 still describes %s as a served adapter", ret.Route)
		assert.NotContains(t, section("## 26.5 Runtime Adapters"), "`"+ret.Route+"`",
			"26.5 still describes %s as a served adapter", ret.Route)
	}
}
