package db

import (
	"fmt"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// visibilityOrDefault coerces an empty visibility to "public" so a Post created without an
// explicit visibility (e.g. legacy callers, ideas.CreateIdea, tests) still satisfies the
// NOT NULL / CHECK column.
func visibilityOrDefault(v string) string {
	if v == "" {
		return models.VisibilityPublic
	}
	return v
}

// appendVisibilityFilter adds a family-scoped visibility predicate to a hand-built query,
// mutating the conditions/args/argNum in place (the pattern used by PostRepository.List).
//
//   - callerHuman == "" (anonymous, unclaimed agent, cross-family, MCP): public-only.
//   - callerHuman set (a human UUID): public OR owned by that human's family.
//
// The uuid arg is only appended in the family-parameterized branch, so anonymous callers
// never bind an empty string (which would fail the ::uuid cast, 22P02). `alias` is the
// posts table alias in the target query (e.g. "p").
func appendVisibilityFilter(conds *[]string, args *[]any, argNum *int, alias, callerHuman string) {
	if callerHuman == "" {
		*conds = append(*conds, publicOnlyVisibility(alias))
		return
	}
	*conds = append(*conds, fmt.Sprintf(
		"(%s.visibility = 'public' OR (%s.owner_human_id IS NOT NULL AND %s.owner_human_id = $%d::uuid))",
		alias, alias, alias, *argNum,
	))
	*args = append(*args, callerHuman)
	*argNum++
}

// nullableViewer returns a value suitable for binding a uuid parameter: nil (SQL NULL)
// for an empty caller-human, else the uuid string. Binding "" would fail the ::uuid cast.
func nullableViewer(callerHuman string) any {
	if callerHuman == "" {
		return nil
	}
	return callerHuman
}

// publicOnlyVisibility is the hard public-only predicate for surfaces with no caller
// identity (anonymous feed/sitemap/stats/activity/crystallization, or cross-family
// discovery). `alias` is the posts table alias; pass "" for an unaliased `posts` query.
func publicOnlyVisibility(alias string) string {
	if alias == "" {
		return "visibility = 'public'"
	}
	return alias + ".visibility = 'public'"
}
