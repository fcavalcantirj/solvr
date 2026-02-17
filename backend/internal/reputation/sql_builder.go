package reputation

import (
	"fmt"
	"strings"
)

// SQLBuilderOptions configures SQL CTE generation
type SQLBuilderOptions struct {
	EntityType      string // "agent" or "user"
	EntityIDColumn  string // "a.id" or "u.id::text"
	AuthorType      string // "agent" or "human"
	TimeFilter      string // "" for all-time, or "AND created_at >= $3"
	IncludeBonus    bool   // true for agents, false for users
	BonusColumn     string // "agents.reputation" or "a.reputation" - only used if IncludeBonus=true
}

// BuildReputationSQL generates the complete reputation calculation SQL expression
// Returns a SQL fragment that can be used in SELECT clauses
func BuildReputationSQL(opts SQLBuilderOptions) string {
	parts := []string{}

	// Bonus (agents only)
	if opts.IncludeBonus {
		bonusCol := opts.BonusColumn
		if bonusCol == "" {
			bonusCol = "agents.reputation" // Default for backwards compat
		}
		parts = append(parts, fmt.Sprintf("COALESCE(%s, 0)", bonusCol))
	}

	// Problems solved (100 points)
	parts = append(parts, fmt.Sprintf(`
		COALESCE((
			SELECT COUNT(*)
			FROM posts p
			WHERE p.posted_by_id = %s
				AND p.posted_by_type = '%s'
				AND p.type = 'problem'
				AND p.status = 'solved'
				AND p.deleted_at IS NULL
				%s
		), 0) * %d`,
		opts.EntityIDColumn, opts.AuthorType, opts.TimeFilter, PointsProblemSolved))

	// Problems contributed (25 points)
	parts = append(parts, fmt.Sprintf(`
		COALESCE((
			SELECT COUNT(*)
			FROM posts p
			WHERE p.posted_by_id = %s
				AND p.posted_by_type = '%s'
				AND p.type = 'problem'
				AND p.deleted_at IS NULL
				%s
		), 0) * %d`,
		opts.EntityIDColumn, opts.AuthorType, opts.TimeFilter, PointsProblemContributed))

	// Answers accepted (50 points)
	parts = append(parts, fmt.Sprintf(`
		COALESCE((
			SELECT COUNT(*)
			FROM answers ans
			WHERE ans.author_id = %s
				AND ans.author_type = '%s'
				AND ans.is_accepted = true
				AND ans.deleted_at IS NULL
				%s
		), 0) * %d`,
		opts.EntityIDColumn, opts.AuthorType, opts.TimeFilter, PointsAnswerAccepted))

	// Answers given (10 points)
	parts = append(parts, fmt.Sprintf(`
		COALESCE((
			SELECT COUNT(*)
			FROM answers ans
			WHERE ans.author_id = %s
				AND ans.author_type = '%s'
				AND ans.deleted_at IS NULL
				%s
		), 0) * %d`,
		opts.EntityIDColumn, opts.AuthorType, opts.TimeFilter, PointsAnswerGiven))

	// Ideas posted (15 points)
	parts = append(parts, fmt.Sprintf(`
		COALESCE((
			SELECT COUNT(*)
			FROM posts p
			WHERE p.posted_by_id = %s
				AND p.posted_by_type = '%s'
				AND p.type = 'idea'
				AND p.deleted_at IS NULL
				%s
		), 0) * %d`,
		opts.EntityIDColumn, opts.AuthorType, opts.TimeFilter, PointsIdeaPosted))

	// Responses given (5 points)
	// Note: responses table does not have deleted_at column
	parts = append(parts, fmt.Sprintf(`
		COALESCE((
			SELECT COUNT(*)
			FROM responses r
			WHERE r.author_id = %s
				AND r.author_type = '%s'
				%s
		), 0) * %d`,
		opts.EntityIDColumn, opts.AuthorType, opts.TimeFilter, PointsResponseGiven))

	// Upvotes received (2 points each)
	parts = append(parts, fmt.Sprintf(`
		COALESCE((
			SELECT COUNT(*)
			FROM votes v
			WHERE v.confirmed = true
				AND v.direction = 'up'
				%s
				AND (
					(v.target_type = 'post' AND EXISTS (
						SELECT 1 FROM posts p WHERE p.id = v.target_id AND p.posted_by_type = '%s' AND p.posted_by_id = %s
					))
					OR (v.target_type = 'answer' AND EXISTS (
						SELECT 1 FROM answers ans WHERE ans.id = v.target_id AND ans.author_type = '%s' AND ans.author_id = %s
					))
					OR (v.target_type = 'response' AND EXISTS (
						SELECT 1 FROM responses r WHERE r.id = v.target_id AND r.author_type = '%s' AND r.author_id = %s
					))
				)
		), 0) * %d`,
		opts.TimeFilter, opts.AuthorType, opts.EntityIDColumn, opts.AuthorType, opts.EntityIDColumn, opts.AuthorType, opts.EntityIDColumn, PointsUpvoteReceived))

	// Downvotes received (-1 point each)
	parts = append(parts, fmt.Sprintf(`
		COALESCE((
			SELECT COUNT(*)
			FROM votes v
			WHERE v.confirmed = true
				AND v.direction = 'down'
				%s
				AND (
					(v.target_type = 'post' AND EXISTS (
						SELECT 1 FROM posts p WHERE p.id = v.target_id AND p.posted_by_type = '%s' AND p.posted_by_id = %s
					))
					OR (v.target_type = 'answer' AND EXISTS (
						SELECT 1 FROM answers ans WHERE ans.id = v.target_id AND ans.author_type = '%s' AND ans.author_id = %s
					))
					OR (v.target_type = 'response' AND EXISTS (
						SELECT 1 FROM responses r WHERE r.id = v.target_id AND r.author_type = '%s' AND r.author_id = %s
					))
				)
		), 0) * %d`,
		opts.TimeFilter, opts.AuthorType, opts.EntityIDColumn, opts.AuthorType, opts.EntityIDColumn, opts.AuthorType, opts.EntityIDColumn, PointsDownvoteReceived))

	// Join all parts with + operator
	var builder strings.Builder
	builder.WriteString("(")
	for i, part := range parts {
		if i > 0 {
			builder.WriteString(" + ")
		}
		builder.WriteString(part)
	}
	builder.WriteString(")")

	return builder.String()
}
