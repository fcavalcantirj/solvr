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

// searchVisibilityClause returns the family-scoped visibility predicate for the posts
// table aliased `alias`, and is the single source of truth for that security predicate
// (reused by PostRepository.List, searchPosts, searchAnswers, searchApproaches).
//
//   - callerHuman == "" (anonymous, unclaimed agent, cross-family, MCP): public-only
//     (byte-identical to publicOnlyVisibility(alias)) and NO uuid arg is bound.
//   - callerHuman set (a human UUID): public OR owned by that human's family; the uuid is
//     appended to args and *argNum advanced.
//
// Only appending the uuid in the family branch keeps anonymous callers from binding an
// empty string, which would fail the ::uuid cast (22P02).
func searchVisibilityClause(alias, callerHuman string, args *[]any, argNum *int) string {
	if callerHuman == "" {
		return publicOnlyVisibility(alias)
	}
	clause := fmt.Sprintf(
		"(%s.visibility = 'public' OR (%s.owner_human_id IS NOT NULL AND %s.owner_human_id = $%d::uuid))",
		alias, alias, alias, *argNum,
	)
	*args = append(*args, callerHuman)
	*argNum++
	return clause
}

// appendVisibilityFilter adds a family-scoped visibility predicate to a hand-built query,
// mutating the conditions/args/argNum in place (the pattern used by PostRepository.List).
// It delegates to searchVisibilityClause so every read surface shares one predicate.
func appendVisibilityFilter(conds *[]string, args *[]any, argNum *int, alias, callerHuman string) {
	*conds = append(*conds, searchVisibilityClause(alias, callerHuman, args, argNum))
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
