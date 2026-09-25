package db

import (
	"context"
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

// visiblePostExists returns an EXISTS predicate that is true when the post whose id is the
// SQL expression postExpr may be read by callerHuman under the rule GET /v1/posts/{id}
// applies: the post exists, is not deleted, and passes searchVisibilityClause (public, or
// owned by the caller's family). postExpr must be trusted SQL, never caller input.
func visiblePostExists(postExpr, callerHuman string, args *[]any, argNum *int) string {
	return "EXISTS(SELECT 1 FROM posts p WHERE p.id = " + postExpr + " AND p.deleted_at IS NULL AND " +
		searchVisibilityClause("p", callerHuman, args, argNum) + ")"
}

// postVisibleTo reports whether callerHuman may read the post under the rule GET
// /v1/posts/{id} applies. Anything under a post (its replies, comments, view count, related
// rooms) answers 404 exactly when this is false. A malformed id names no post, so it is
// false rather than an error.
func postVisibleTo(ctx context.Context, pool *Pool, postID, callerHuman string) (bool, error) {
	args := []any{postID}
	argNum := 2
	var visible bool
	err := pool.QueryRow(ctx, "SELECT "+visiblePostExists("$1", callerHuman, &args, &argNum), args...).Scan(&visible)
	if err != nil {
		if isInvalidUUIDError(err) {
			return false, nil
		}
		LogQueryError(ctx, "Post.VisibleTo", "posts", err)
		return false, fmt.Errorf("check post visibility: %w", err)
	}
	return visible, nil
}

// VisibleTo reports whether callerHuman ("" = anonymous or unclaimed) may read the post.
func (r *PostRepository) VisibleTo(ctx context.Context, postID, callerHuman string) (bool, error) {
	return postVisibleTo(ctx, r.pool, postID, callerHuman)
}

// PostVisibleTo reports whether callerHuman may read the post a reply belongs to (or
// would belong to), so the reply surfaces never show or accept what the post hides.
func (r *ReplyRepository) PostVisibleTo(ctx context.Context, postID, callerHuman string) (bool, error) {
	return postVisibleTo(ctx, r.pool, postID, callerHuman)
}

// PostVisibleTo reports whether callerHuman may read the post, so its view count is never
// shown or moved for a caller the post is hidden from.
func (r *ViewsRepository) PostVisibleTo(ctx context.Context, postID, callerHuman string) (bool, error) {
	return postVisibleTo(ctx, r.pool, postID, callerHuman)
}

// ApproachVisibleTo reports whether callerHuman may read the live post that owns a live
// approach. Legacy approach routes use this before reading or mutating the child so an
// approach id cannot bypass its problem's family/deletion rules.
func (r *ApproachesRepository) ApproachVisibleTo(ctx context.Context, approachID, callerHuman string) (bool, error) {
	args := []any{approachID}
	argNum := 2
	query := `SELECT EXISTS(
		SELECT 1 FROM approaches a
		WHERE a.id = $1 AND a.deleted_at IS NULL AND ` +
		visiblePostExists("a.problem_id", callerHuman, &args, &argNum) + `
	)`
	var visible bool
	if err := r.pool.QueryRow(ctx, query, args...).Scan(&visible); err != nil {
		if isInvalidUUIDError(err) {
			return false, nil
		}
		LogQueryError(ctx, "Approach.VisibleTo", "approaches", err)
		return false, fmt.Errorf("check approach visibility: %w", err)
	}
	return visible, nil
}
