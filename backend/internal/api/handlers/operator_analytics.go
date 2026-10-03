package handlers

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"os"
	"regexp"
	"strings"
)

// What the people who run Solvr can read, and nobody else.
//
// Solvr measures two different things. PRODUCT ACTIVITY — rooms, eligible
// search, aggregate API usage, labeled totals — is published, and
// public_overview_allowlist.go decides exactly how. OPERATOR ANALYTICS is the
// other one: website traffic, where the audience came from, how much of it
// came back, what it cost, the raw diagnostics behind an incident, and the
// growth planning written against all of that. It is reporting Solvr does
// about ITSELF.
//
// Operator analytics is not a feature of the product, so no credential the
// product issues opens it. A human account, an agent API key, a user API key,
// a room owner and a room-scoped token are all credentials for USING Solvr;
// reading Solvr's own numbers takes the operator's own key, validated here by
// the server. Hiding a link is not access control: the rule has to hold for
// somebody typing the URL.
//
// Two consequences, both enforced here rather than in a browser:
//
//  1. ONE GATE. Every operator route goes through RequireOperatorAccess, and a
//     server with no operator key configured is CLOSED rather than open.
//  2. NO SHARED CACHE ENTRY. An operator answer is never storable and is keyed
//     on the credential, so no cache in front of the API can hold one and hand
//     it to the next visitor who asks for the same URL. The public overview
//     stays cacheable precisely because it is the same answer for everybody,
//     operator included.

// OperatorAccessHeader carries the operator's own key. It is deliberately not
// on the CORS allowlist: a page in a browser cannot send it.
const OperatorAccessHeader = "X-Admin-API-Key"

// OperatorReport is one operator-analytics or growth-report surface: where it
// lives and what it publishes.
type OperatorReport struct {
	// Method is the HTTP method the router registers.
	Method string
	// Path is the route, always under /admin and never under /v1.
	Path string
	// Reports says, in one phrase, what an operator reads there.
	Reports string
}

// OperatorReports is every surface that answers with operator material. The
// router gates each of them and the contract tests walk this list, so a report
// cannot be added in one place and forgotten in the other.
//
// This is not the full list of operator routes — the router gates everything
// under /admin — it is the list of routes that REPORT.
var OperatorReports = []OperatorReport{
	{
		Method:  http.MethodGet,
		Path:    "/admin/search-analytics/trending",
		Reports: "the terms people and agents typed, including the ones that found nothing",
	},
	{
		Method:  http.MethodGet,
		Path:    "/admin/search-analytics/summary",
		Reports: "search volume, zero-result rate and searcher mix over a chosen period",
	},
	{
		Method:  http.MethodGet,
		Path:    "/admin/activation-analytics",
		Reports: "how many rooms were created and activated, milestone latency, and funnel conversion by origin",
	},
	{
		Method:  http.MethodGet,
		Path:    "/admin/cohort-comparison",
		Reports: "consistent 7-day and 28-day post-launch cohorts anchored to one launch timestamp",
	},
	{
		Method:  http.MethodGet,
		Path:    "/admin/growth/participants",
		Reports: "monthly active humans and agent identities, anonymous activity, traffic and the one-million target",
	},
	{
		Method:  http.MethodGet,
		Path:    "/admin/growth/stages",
		Reports: "the four growth stages and their gates: activated rooms, conversion, creator return, reliability, moderation load",
	},
	{
		Method:  http.MethodGet,
		Path:    "/admin/growth/model",
		Reports: "the monthly acquisition model: retained, new and reactivated participants, cohorts, scenarios, channels and the bottleneck",
	},
	{
		Method:  http.MethodGet,
		Path:    "/admin/email/history",
		Reports: "what was sent to the mailing list, when, and how much of it landed",
	},
	{
		Method:  http.MethodGet,
		Path:    "/admin/users/deleted",
		Reports: "the human accounts removed from the product",
	},
	{
		Method:  http.MethodGet,
		Path:    "/admin/agents/deleted",
		Reports: "the agents removed from the product",
	},
	{
		Method:  http.MethodPost,
		Path:    "/admin/query",
		Reports: "raw database diagnostics, which can read anything the product stores",
	},
	{
		Method:  http.MethodGet,
		Path:    "/admin/share-attribution",
		Reports: "share visits, invitations, attributed activations, new agents and humans, returns, and the k experiment metric",
	},
}

// ApplyOperatorReportCachePolicy marks a response as one caller's private
// answer that no cache may store or reuse.
func ApplyOperatorReportCachePolicy(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store, private, max-age=0, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Vary", OperatorAccessHeader+", Authorization, Cookie")
}

// RequireOperatorAccess admits only the operator's own key and applies the
// private cache policy to every answer it produces, refusals included.
func RequireOperatorAccess(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ApplyOperatorReportCachePolicy(w)

		configured := os.Getenv("ADMIN_API_KEY")
		if configured == "" {
			// No key configured means no operator, not "anyone".
			writeOperatorError(w, http.StatusServiceUnavailable, "ADMIN_NOT_CONFIGURED", "admin API key not configured")
			return
		}

		provided := r.Header.Get(OperatorAccessHeader)
		if provided == "" {
			writeOperatorError(w, http.StatusUnauthorized, "MISSING_API_KEY", OperatorAccessHeader+" header required")
			return
		}

		if subtle.ConstantTimeCompare([]byte(provided), []byte(configured)) != 1 {
			writeOperatorError(w, http.StatusForbidden, "INVALID_API_KEY", "invalid admin API key")
			return
		}

		next.ServeHTTP(w, r)
	})
}

// writeOperatorError answers in the error shape the rest of the API uses, and
// says nothing about the report it refused.
func writeOperatorError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]string{
			"code":    code,
			"message": message,
		},
	})
}

// operatorCredentialConcept is one kind of operator instrument and the ways it
// shows up in text.
type operatorCredentialConcept struct {
	name     string
	patterns []*regexp.Regexp
}

// operatorCredentialConcepts are the Google Analytics and Search Console
// materials that never reach a public surface: the credentials that read a
// property, the property itself, and the exports taken out of it.
//
// The COLLECTION TAG is deliberately absent. A gtag measurement id runs in
// every visitor's browser by design — it is how the measurement happens, not a
// secret — so flagging it would only teach the tests to lie. What must stay
// private is the ability to READ the reports.
var operatorCredentialConcepts = []operatorCredentialConcept{
	{"a Google Analytics property", []*regexp.Regexp{
		regexp.MustCompile(`(?i)\bga4\b`),
		regexp.MustCompile(`(?i)\bgoogle[ _-]?analytics\b`),
		regexp.MustCompile(`(?i)\banalytics[ _-]?propert(y|ies)\b`),
		regexp.MustCompile(`(?i)\bpropert(y|ies)[ _-]?id\b`),
		regexp.MustCompile(`(?i)\bproperties/\d+`),
		regexp.MustCompile(`(?i)\banalyticsdata\b`),
	}},
	{"a Search Console property", []*regexp.Regexp{
		regexp.MustCompile(`(?i)\bsearch[ _-]?console\b`),
		regexp.MustCompile(`(?i)\bwebmasters?\b`),
		regexp.MustCompile(`(?i)\bsite[ _-]?verification\b`),
	}},
	// A REPORTING credential, not any credential. Solvr issues private keys to
	// agents and refresh tokens to people — those are the product working, and
	// a rule that flagged them would be a rule nobody could keep. What is
	// named here is the material that reads somebody else's analytics.
	{"a reporting credential", []*regexp.Regexp{
		regexp.MustCompile(`(?i)\bservice[ _-]?account\b`),
		regexp.MustCompile(`(?i)\bclient[ _-]?secret\b`),
		regexp.MustCompile(`(?i)\bapi[ _-]?secret\b`),
		regexp.MustCompile(`(?i)\bgoogle[ _-]?application[ _-]?credentials\b`),
		regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY-----`),
	}},
	{"an analytics export", []*regexp.Regexp{
		regexp.MustCompile(`(?i)\b(analytics|traffic|audience|ga4|search[ _-]?console)[ _-]?exports?\b`),
		regexp.MustCompile(`(?i)\bexports?[ _-]?(of[ _-]?)?(analytics|traffic|audience)\b`),
		regexp.MustCompile(`(?i)\bbigquery\b`),
	}},
}

// OperatorCredentialTermIn returns the operator material found in s, or "".
func OperatorCredentialTermIn(s string) string {
	for _, concept := range operatorCredentialConcepts {
		for _, pattern := range concept.patterns {
			if pattern.MatchString(s) {
				return concept.name
			}
		}
	}
	return ""
}

// OperatorCredentialTermInKey reads a JSON field name as words, so
// ga4_property_id and "GA4 property id" are the same idea.
func OperatorCredentialTermInKey(key string) string {
	return OperatorCredentialTermIn(strings.ReplaceAll(key, "_", " "))
}
