package db

import (
	"context"
	"fmt"
	"strings"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// List returns a paginated list of posts with author information.
// Supports filtering by type, status, and tags.
// Excludes soft-deleted posts (deleted_at IS NULL).
func (r *PostRepository) List(ctx context.Context, opts models.PostListOptions) ([]models.PostWithAuthor, int, error) {
	// Build dynamic query with filters
	var conditions []string
	var args []any
	argNum := 1

	// Always exclude deleted posts
	conditions = append(conditions, "p.deleted_at IS NULL")

	// Exclude hidden statuses (pending_review, rejected, draft) unless IncludeHidden is set
	if !opts.IncludeHidden {
		conditions = append(conditions, "p.status NOT IN ('pending_review', 'rejected', 'draft')")
	}

	// BART-151: family-scoped visibility — public posts, plus the caller's own family
	// (ViewerHuman == "" for anonymous/cross-family → public-only).
	appendVisibilityFilter(&conditions, &args, &argNum, "p", opts.ViewerHuman)

	// Filter by type
	if opts.Type != "" {
		conditions = append(conditions, fmt.Sprintf("p.type = $%d", argNum))
		args = append(args, opts.Type)
		argNum++
	}

	// Filter by status
	if opts.Status != "" {
		conditions = append(conditions, fmt.Sprintf("p.status = $%d", argNum))
		args = append(args, opts.Status)
		argNum++
	}

	// Filter by tags (PostgreSQL array overlap operator)
	if len(opts.Tags) > 0 {
		conditions = append(conditions, fmt.Sprintf("p.tags && $%d", argNum))
		args = append(args, opts.Tags)
		argNum++
	}

	// Filter by author (BE-003: user profile endpoints)
	if opts.AuthorType != "" && opts.AuthorID != "" {
		conditions = append(conditions, fmt.Sprintf("p.posted_by_type = $%d AND p.posted_by_id = $%d", argNum, argNum+1))
		args = append(args, opts.AuthorType, opts.AuthorID)
		argNum += 2
	}

	appendNeedsHelpFilter(&conditions, opts.NeedsHelp)
	appendHasAnswerFilter(&conditions, opts.HasAnswer)

	// Filter by timeframe
	if opts.Timeframe != "" {
		switch opts.Timeframe {
		case "today":
			conditions = append(conditions, "p.created_at > NOW() - INTERVAL '1 day'")
		case "week":
			conditions = append(conditions, "p.created_at > NOW() - INTERVAL '7 days'")
		case "month":
			conditions = append(conditions, "p.created_at > NOW() - INTERVAL '30 days'")
		}
	}

	whereClause := strings.Join(conditions, " AND ")

	// Calculate pagination
	page := opts.Page
	if page < 1 {
		page = 1
	}
	perPage := opts.PerPage
	if perPage < 1 {
		perPage = 20
	}
	if perPage > 100 {
		perPage = 100
	}
	offset := (page - 1) * perPage

	// Query for total count. It reads every listed post (idx 77 slice 21, 200k posts: 26 ms warm,
	// 40-64 ms cold). A covering partial index made it an index-only scan of 17 ms warm, not
	// worth an 11 MB index on every post write (step 6), so the total stays a plain count.
	countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM posts p WHERE %s`, whereClause)
	var total int
	err := r.pool.QueryRow(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		LogQueryError(ctx, "List.Count", "posts", err)
		return nil, 0, fmt.Errorf("count query failed: %w", err)
	}

	// Determine sort order. byCounts: the order reads every candidate's reply counts.
	orderClause := "p.created_at DESC" // default: newest
	byCounts := false
	switch opts.Sort {
	case "votes", "top": // "top" is frontend alias for vote-based sorting
		orderClause = "(p.upvotes - p.downvotes) DESC, p.created_at DESC"
	case "hot": // trending: engagement-weighted score + recency decay
		// In float8: in numeric arithmetic the score made a hot page 2.9 s at 200k posts, this
		// way 0.6 s with the same order (idx 77 slice 13).
		orderClause = "(LOG(GREATEST(ABS(COALESCE(p.upvotes,0) - COALESCE(p.downvotes,0)) + COALESCE(rc.cmt,0) * 2 + COALESCE(rc.ans,0) * 3 + COALESCE(rc.app,0) * 3 + COALESCE(p.view_count,0) * 0.01::float8, 1::float8) + 1) + EXTRACT(EPOCH FROM (p.created_at - (NOW() - INTERVAL '7 days')))::float8 / 45000.0::float8) DESC"
		byCounts = true
	case "new": // frontend alias for newest
		orderClause = "p.created_at DESC"
	case "approaches":
		orderClause = "COALESCE(rc.app, 0) DESC, p.created_at DESC"
		byCounts = true
	case "answers":
		orderClause = "COALESCE(rc.ans, 0) DESC, p.created_at DESC"
		byCounts = true
	}

	// Build viewer vote column and JOIN
	var viewerVoteColumn, viewerVoteJoin string
	if opts.ViewerType != "" && opts.ViewerID != "" {
		viewerVoteColumn = "v.direction as user_vote_direction"
		viewerVoteJoin = fmt.Sprintf(`LEFT JOIN votes v ON v.target_type = 'post' AND v.target_id = p.id AND v.voter_type = $%d AND v.voter_id = $%d`, argNum, argNum+1)
		args = append(args, string(opts.ViewerType), opts.ViewerID)
		argNum += 2
	} else {
		viewerVoteColumn = "NULL::text as user_vote_direction"
		viewerVoteJoin = ""
	}

	// The page is chosen from posts alone (plus every post's reply counts when the order
	// reads them), then only its rows get authors, reply counts and the viewer's vote,
	// and are put back in order. Joining those first cost a scan of users per listed post and
	// an aggregate of every live reply per page (idx 77 slice 13, 200k posts and 1M replies: a
	// newest page's statement 586 ms, 0.4 ms this way).
	pageCounts := ""
	if byCounts {
		pageCounts = postReplyCountsJoin
	}
	query := fmt.Sprintf(`
		SELECT
			p.id, p.type, p.title, p.description, p.tags,
			p.posted_by_type, p.posted_by_id, p.status,
			p.upvotes, p.downvotes, p.view_count,
			p.created_at, p.updated_at, p.deleted_at,
			p.crystallization_cid, p.crystallized_at,
			COALESCE(p.original_language, '') as original_language,
			COALESCE(p.original_title, '') as original_title,
			COALESCE(p.original_description, '') as original_description,
			COALESCE(u.display_name, ag.display_name, '') as author_display_name,
			COALESCE(u.avatar_url, ag.avatar_url, '') as author_avatar_url,
			%s,
			COALESCE(ag.human_id::text, '') as agent_human_id,
			%s,
			p.visibility,
			p.publication_state,
			p.moderation_state,
			p.source_room_id::text
		FROM (
			SELECT p.* FROM posts p
			%s
			WHERE %s
			ORDER BY %s
			LIMIT $%d OFFSET $%d
		) p
		LEFT JOIN users u ON p.posted_by_type = 'human' AND p.posted_by_id = u.id::text
		LEFT JOIN agents ag ON p.posted_by_type = 'agent' AND p.posted_by_id = ag.id
		%s
		%s
		ORDER BY %s
	`, postReplyCountColumns, viewerVoteColumn, pageCounts, whereClause, orderClause, argNum, argNum+1,
		postPageReplyCountsJoin, viewerVoteJoin, orderClause)

	args = append(args, perPage, offset)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		LogQueryError(ctx, "List", "posts", err)
		return nil, 0, fmt.Errorf("list query failed: %w", err)
	}
	defer rows.Close()

	var posts []models.PostWithAuthor
	for rows.Next() {
		post, err := r.scanPostWithAuthorRows(rows)
		if err != nil {
			LogQueryError(ctx, "List.Scan", "posts", err)
			return nil, 0, fmt.Errorf("scan failed: %w", err)
		}
		posts = append(posts, *post)
	}

	if err := rows.Err(); err != nil {
		LogQueryError(ctx, "List.Rows", "posts", err)
		return nil, 0, fmt.Errorf("rows iteration failed: %w", err)
	}

	// Return empty slice if no results (not nil)
	if posts == nil {
		posts = []models.PostWithAuthor{}
	}

	return posts, total, nil
}
