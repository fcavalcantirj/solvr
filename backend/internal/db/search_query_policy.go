package db

import (
	"os"
	"regexp"
	"strings"
)

// The public-query filtering policy.
//
// Solvr records every search it serves. That log is operational data, and a
// public page must never become a window onto it. This file is the single
// gate a recorded search TERM has to pass before its text may be published,
// and the rule it enforces is deliberately one-directional: a term is
// published only when it is positively known to be safe. Anything unknown,
// unreadable or unrecorded stays unpublished.
//
// Four things get a term rejected, and each one is named so the caller can say
// out loud how many terms it withheld and why:
//
//   - PROTECTED SCOPE. A search made by a signed-in human, or by an agent
//     claimed by one, runs family-scoped (see searchVisibilityClause): it can
//     match posts that are NOT public. The term that found them is therefore
//     not a public search term, whatever it says.
//   - UNKNOWN SCOPE. A search recorded before scope was instrumented. We do
//     not know whether it reached protected content, so we do not publish it.
//     It may still be COUNTED — see the caller's "safe aggregate" wording.
//   - CREDENTIAL-LIKE. People paste secrets into search boxes. The policy
//     matches the SHAPE OF A VALUE, never the topic: "how to rotate an api
//     key" is the single most useful kind of thing in a knowledge base and
//     stays; "api_key = 9f2c1ab4d77e" does not.
//   - PERSONAL. Email addresses, phone numbers, card-length digit runs.
//   - MODERATED. Terms an operator has decided do not belong on the front
//     page. The built-in list is a floor; SEARCH_QUERY_BLOCKLIST extends it.
//
// Nothing here weakens: adding a rule can only remove terms from the page.

// PublicQueryRejection names why a recorded query may not be published, or is
// empty when it may be.
type PublicQueryRejection string

const (
	// PublicQueryOK means the term may be published.
	PublicQueryOK PublicQueryRejection = ""

	// RejectedProtectedScope: the search ran family-scoped and could have
	// matched content that is not public.
	RejectedProtectedScope PublicQueryRejection = "protected_scope"

	// RejectedUnknownScope: the search predates scope instrumentation.
	RejectedUnknownScope PublicQueryRejection = "unknown_scope"

	// RejectedCredential: the text carries something value-shaped enough to be
	// a secret.
	RejectedCredential PublicQueryRejection = "credential"

	// RejectedPersonal: the text carries an identifier belonging to a person.
	RejectedPersonal PublicQueryRejection = "personal"

	// RejectedModerated: the text matches the moderation blocklist.
	RejectedModerated PublicQueryRejection = "moderated"

	// RejectedUnpublishable: blank, or long enough to be a paste rather than a
	// search term.
	RejectedUnpublishable PublicQueryRejection = "unpublishable"
)

// publishableQueryMaxChars is the longest term that still reads as a search
// term. Beyond it, someone pasted a document, a stack trace or a payload.
const publishableQueryMaxChars = 120

// moderationBlocklistEnv is the operator-owned extension of the moderation
// list: a comma-separated set of terms, matched the same way the built-ins are.
const moderationBlocklistEnv = "SEARCH_QUERY_BLOCKLIST"

// defaultModerationTerms is the built-in floor. It is deliberately short and
// unambiguous: it exists so the front page of a developer knowledge base does
// not publish adult-content searches, not to be a general word filter.
var defaultModerationTerms = []string{"porn", "xxx", "nudes", "escorts"}

// credentialPatterns match the SHAPE of a secret value.
var credentialPatterns = []*regexp.Regexp{
	// Provider-issued keys, each with its own published prefix.
	regexp.MustCompile(`(?i)\bsk-[A-Za-z0-9_-]{16,}`),
	regexp.MustCompile(`(?i)\bsolvr_(sk_)?[A-Za-z0-9]{12,}`),
	regexp.MustCompile(`\bAKIA[0-9A-Z]{12,}`),
	regexp.MustCompile(`(?i)\bgh[pousr]_[A-Za-z0-9]{20,}`),
	regexp.MustCompile(`(?i)\bxox[baprs]-[A-Za-z0-9-]{10,}`),
	// A JWT: three base64url segments.
	regexp.MustCompile(`\beyJ[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]*`),
	// A PEM header.
	regexp.MustCompile(`(?i)-----BEGIN [A-Z ]*PRIVATE KEY`),
	// A secret assigned to a name. The name must start a word (so DATABASE_PASSWORD
	// matches and "bypass=true" does not) and the value is what condemns it.
	regexp.MustCompile(`(?i)(^|[\s_.-])(pass(word|wd)?|pwd|secret|api[_-]?key|access[_-]?token|auth[_-]?token|client[_-]?secret)\s*[:=]\s*\S`),
	// Bearer <opaque value>.
	regexp.MustCompile(`(?i)\bbearer\s+[A-Za-z0-9._~+/=-]{20,}`),
	// Credentials embedded in a URL.
	regexp.MustCompile(`://[^/\s:@]+:[^/\s@]+@`),
	// A single unbroken high-entropy run. No search term looks like this; a
	// pasted key, hash or session id does.
	regexp.MustCompile(`[A-Za-z0-9+/_=-]{40,}`),
}

// personalPatterns match identifiers that belong to a person.
var personalPatterns = []*regexp.Regexp{
	// Email address.
	regexp.MustCompile(`[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}`),
	// A telephone number: a leading + or a grouped run of 9 or more digits.
	regexp.MustCompile(`\+\d[\d\s().-]{7,}\d`),
	regexp.MustCompile(`\b\d[\d\s().-]{9,}\d\b`),
	// A card- or document-length digit run.
	regexp.MustCompile(`\b\d{11,}\b`),
}

// PublicQueryText decides whether a recorded search term may be published.
//
// publicScope is what the request recorded: true when the search ran over
// public content only, false when it ran family-scoped, nil when the search
// predates the recording. The scope gate runs FIRST, so a protected search is
// never even inspected for its text.
func PublicQueryText(query string, publicScope *bool) PublicQueryRejection {
	switch {
	case publicScope == nil:
		return RejectedUnknownScope
	case !*publicScope:
		return RejectedProtectedScope
	}

	trimmed := strings.TrimSpace(query)
	if trimmed == "" || len([]rune(trimmed)) > publishableQueryMaxChars {
		return RejectedUnpublishable
	}

	for _, re := range credentialPatterns {
		if re.MatchString(trimmed) {
			return RejectedCredential
		}
	}
	for _, re := range personalPatterns {
		if re.MatchString(trimmed) {
			return RejectedPersonal
		}
	}
	if matchesModeration(trimmed) {
		return RejectedModerated
	}
	return PublicQueryOK
}

// matchesModeration reports whether any blocklist term appears as a word or as
// the start of one ("pornography" matches "porn"; "xxhash" does not match
// "xxx"). Matching a prefix at a word boundary is the difference between a
// list that works and one that is trivially evaded by a suffix.
func matchesModeration(query string) bool {
	lowered := strings.ToLower(query)
	for _, term := range moderationTerms(defaultModerationTerms, os.Getenv(moderationBlocklistEnv)) {
		for _, start := range wordStarts(lowered) {
			if strings.HasPrefix(lowered[start:], term) {
				return true
			}
		}
	}
	return false
}

// wordStarts lists the byte offsets at which a word begins.
func wordStarts(s string) []int {
	starts := make([]int, 0, 8)
	prevWord := false
	for i, r := range s {
		isWord := r == '_' || r == '-' || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
		if isWord && !prevWord {
			starts = append(starts, i)
		}
		prevWord = isWord
	}
	return starts
}

// moderationTerms merges the built-in floor with the operator's list. The
// configured value EXTENDS the default; it can never shrink it, so no
// deployment can be configured into publishing what the floor withholds.
func moderationTerms(defaults []string, configured string) []string {
	terms := make([]string, 0, len(defaults)+4)
	terms = append(terms, defaults...)
	for _, raw := range strings.Split(configured, ",") {
		term := strings.ToLower(strings.TrimSpace(raw))
		if term != "" {
			terms = append(terms, term)
		}
	}
	return terms
}
