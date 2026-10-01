// Package db provides database access for Solvr.
package db

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Post-related errors.
var (
	ErrPostNotFound         = errors.New("post not found")
	ErrDuplicatePostID      = errors.New("post ID already exists")
	ErrInvalidPostType      = errors.New("invalid post type")
	ErrInvalidPostStatus    = errors.New("invalid post status")
	ErrInvalidVoteDirection = errors.New("invalid vote direction: must be 'up' or 'down'")
	ErrInvalidVoterType     = errors.New("invalid voter type: must be 'human' or 'agent'")
)

// isInvalidUUIDError checks if an error is a PostgreSQL invalid UUID syntax error.
// PostgreSQL error code 22P02 = invalid_text_representation (e.g., invalid UUID format).
// FIX-007: Return ErrPostNotFound for invalid UUID syntax to avoid 500 errors.
func isInvalidUUIDError(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		// 22P02 = invalid_text_representation (includes invalid UUID syntax)
		return pgErr.Code == "22P02"
	}
	return false
}

// isTableNotFoundError checks if an error is a PostgreSQL "relation does not exist" error.
// PostgreSQL error code 42P01 = undefined_table.
// Used for graceful degradation when optional tables haven't been created yet.
func isTableNotFoundError(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		// 42P01 = undefined_table (relation does not exist)
		return pgErr.Code == "42P01"
	}
	return false
}

// PostRepository handles database operations for posts.
// Per SPEC.md Part 6: posts table.
type PostRepository struct {
	pool *Pool
}

// postColumns defines the standard columns returned when querying posts.
// Used to keep queries consistent and DRY.
const postColumns = `id, type, title, description, tags, posted_by_type, posted_by_id,
	status, upvotes, downvotes, view_count, success_criteria, weight, accepted_answer_id,
	evolved_into, created_at, updated_at, deleted_at, crystallization_cid, crystallized_at`

// NewPostRepository creates a new PostRepository.
func NewPostRepository(pool *Pool) *PostRepository {
	return &PostRepository{pool: pool}
}

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

	// Build answer count filter condition for main query
	// This will be added after the LEFT JOIN so rc.ans is available
	var answerCountFilter string
	if opts.HasAnswer != nil {
		if *opts.HasAnswer {
			answerCountFilter = " AND COALESCE(rc.ans, 0) > 0"
		} else {
			answerCountFilter = " AND COALESCE(rc.ans, 0) = 0"
		}
	}

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

	// Query for total count (with answer count filter if needed)
	var countQuery string
	if answerCountFilter != "" {
		// When filtering by answer count, we need to include the LEFT JOIN in the count query
		countQuery = fmt.Sprintf(`
			SELECT COUNT(*) FROM (
				SELECT p.id
				FROM posts p
				%s
				WHERE %s%s
			) counted
		`, postReplyCountsJoin, whereClause, answerCountFilter)
	} else {
		countQuery = fmt.Sprintf(`SELECT COUNT(*) FROM posts p WHERE %s`, whereClause)
	}
	var total int
	err := r.pool.QueryRow(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		LogQueryError(ctx, "List.Count", "posts", err)
		return nil, 0, fmt.Errorf("count query failed: %w", err)
	}

	// Determine sort order
	orderClause := "p.created_at DESC" // default: newest
	switch opts.Sort {
	case "votes", "top": // "top" is frontend alias for vote-based sorting
		orderClause = "(p.upvotes - p.downvotes) DESC, p.created_at DESC"
	case "hot": // trending: engagement-weighted score + recency decay
		orderClause = "(LOG(GREATEST(ABS(COALESCE(p.upvotes,0) - COALESCE(p.downvotes,0)) + COALESCE(rc.cmt,0) * 2 + COALESCE(rc.ans,0) * 3 + COALESCE(rc.app,0) * 3 + COALESCE(p.view_count,0) * 0.01, 1) + 1) + EXTRACT(EPOCH FROM (p.created_at - (NOW() - INTERVAL '7 days'))) / 45000.0) DESC"
	case "new": // frontend alias for newest
		orderClause = "p.created_at DESC"
	case "approaches":
		orderClause = "COALESCE(rc.app, 0) DESC, p.created_at DESC"
	case "answers":
		orderClause = "COALESCE(rc.ans, 0) DESC, p.created_at DESC"
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

	// Main query with LEFT JOINs for author information and pre-aggregated counts.
	// Uses LEFT JOIN subqueries instead of correlated subqueries to avoid per-row execution.
	query := fmt.Sprintf(`
		SELECT
			p.id, p.type, p.title, p.description, p.tags,
			p.posted_by_type, p.posted_by_id, p.status,
			p.upvotes, p.downvotes, p.view_count, p.success_criteria, p.weight,
			p.accepted_answer_id, p.evolved_into,
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
		FROM posts p
		LEFT JOIN users u ON p.posted_by_type = 'human' AND p.posted_by_id = u.id::text
		LEFT JOIN agents ag ON p.posted_by_type = 'agent' AND p.posted_by_id = ag.id
		%s
		%s
		WHERE %s%s
		ORDER BY %s
		LIMIT $%d OFFSET $%d
	`, postReplyCountColumns, viewerVoteColumn, postReplyCountsJoin, viewerVoteJoin, whereClause, answerCountFilter, orderClause, argNum, argNum+1)

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

// scanPostWithAuthorRows scans a row into a PostWithAuthor struct.
// Used for queries that include LEFT JOINs for author information.
// Expects 29 columns: 20 post fields + 3 translation fields + 2 author fields + 3 counts + 1 user_vote_direction.
func (r *PostRepository) scanPostWithAuthorRows(rows pgx.Rows) (*models.PostWithAuthor, error) {
	var post models.PostWithAuthor
	var authorDisplayName, authorAvatarURL string

	err := rows.Scan(
		&post.ID,
		&post.Type,
		&post.Title,
		&post.Description,
		&post.Tags,
		&post.PostedByType,
		&post.PostedByID,
		&post.Status,
		&post.Upvotes,
		&post.Downvotes,
		&post.ViewCount,
		&post.SuccessCriteria,
		&post.Weight,
		&post.AcceptedAnswerID,
		&post.EvolvedInto,
		&post.CreatedAt,
		&post.UpdatedAt,
		&post.DeletedAt,
		&post.CrystallizationCID,
		&post.CrystallizedAt,
		&post.OriginalLanguage,
		&post.OriginalTitle,
		&post.OriginalDescription,
		&authorDisplayName,
		&authorAvatarURL,
		&post.AnswersCount,
		&post.ApproachesCount,
		&post.CommentsCount,
		&post.AgentHumanID,
		&post.UserVote,
		&post.Visibility,
		&post.PublicationState,
		&post.ModerationState,
		&post.SourceRoomID,
	)
	if err != nil {
		return nil, err
	}

	// Populate author information
	post.Author = models.PostAuthor{
		Type:        post.PostedByType,
		ID:          post.PostedByID,
		DisplayName: authorDisplayName,
		AvatarURL:   authorAvatarURL,
	}

	// Compute vote score
	post.VoteScore = post.Upvotes - post.Downvotes

	// Canonical unified reply count (BART-583): the buckets partition the post's live replies.
	post.ReplyCount = post.AnswersCount + post.ApproachesCount + post.CommentsCount

	return &post, nil
}

// scanPost scans a single row into a Post struct.
// Used for queries that don't include author information joins.
func (r *PostRepository) scanPost(row pgx.Row) (*models.Post, error) {
	post := &models.Post{}
	err := row.Scan(
		&post.ID,
		&post.Type,
		&post.Title,
		&post.Description,
		&post.Tags,
		&post.PostedByType,
		&post.PostedByID,
		&post.Status,
		&post.Upvotes,
		&post.Downvotes,
		&post.ViewCount,
		&post.SuccessCriteria,
		&post.Weight,
		&post.AcceptedAnswerID,
		&post.EvolvedInto,
		&post.CreatedAt,
		&post.UpdatedAt,
		&post.DeletedAt,
		&post.CrystallizationCID,
		&post.CrystallizedAt,
		&post.Visibility,
		&post.PublicationState,
		&post.ModerationState,
		&post.SourceRoomID,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrPostNotFound
		}
		// FIX-007: Invalid UUID format should return ErrPostNotFound (404), not 500.
		if isInvalidUUIDError(err) {
			return nil, ErrPostNotFound
		}
		return nil, err
	}

	return post, nil
}

// scanPostRows scans a rows result into a Post struct.
// Used for queries that return multiple rows without author joins.
func (r *PostRepository) scanPostRows(rows pgx.Rows) (*models.Post, error) {
	post := &models.Post{}
	err := rows.Scan(
		&post.ID,
		&post.Type,
		&post.Title,
		&post.Description,
		&post.Tags,
		&post.PostedByType,
		&post.PostedByID,
		&post.Status,
		&post.Upvotes,
		&post.Downvotes,
		&post.ViewCount,
		&post.SuccessCriteria,
		&post.Weight,
		&post.AcceptedAnswerID,
		&post.EvolvedInto,
		&post.CreatedAt,
		&post.UpdatedAt,
		&post.DeletedAt,
		&post.CrystallizationCID,
		&post.CrystallizedAt,
		&post.Visibility,
	)
	if err != nil {
		return nil, err
	}
	return post, nil
}

// Create inserts a new post into the database.
// Returns the created post with generated ID and timestamps.
func (r *PostRepository) Create(ctx context.Context, post *models.Post) (*models.Post, error) {
	// FIX-030: RETURNING must include view_count to match scanPost expectations
	query := `
		INSERT INTO posts (
			type, title, description, tags,
			posted_by_type, posted_by_id, status,
			upvotes, downvotes,
			success_criteria, weight,
			accepted_answer_id, evolved_into,
			embedding,
			visibility, owner_human_id,
			publication_state, moderation_state, source_room_id, idempotency_key,
			created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14::vector, $15, $16, $17, $18, $19, $20, NOW(), NOW())
		RETURNING id, type, title, description, tags,
			posted_by_type, posted_by_id, status,
			upvotes, downvotes, view_count, success_criteria, weight,
			accepted_answer_id, evolved_into,
			created_at, updated_at, deleted_at,
			crystallization_cid, crystallized_at, visibility,
			publication_state, moderation_state, source_room_id
	`

	// Default status to 'draft' if not provided
	status := post.Status
	if status == "" {
		status = models.PostStatusDraft
	}

	// Canonical publication/moderation states (BART-583): derive from the status
	// unless the caller set them explicitly, keeping the two columns consistent.
	pub, mod := post.PublicationState, post.ModerationState
	if pub == "" || mod == "" {
		dp, dm := models.DeriveStates(status)
		if pub == "" {
			pub = dp
		}
		if mod == "" {
			mod = dm
		}
	}

	row := r.pool.QueryRow(ctx, query,
		post.Type,
		post.Title,
		post.Description,
		post.Tags,
		post.PostedByType,
		post.PostedByID,
		status,
		0, // upvotes
		0, // downvotes
		post.SuccessCriteria,
		post.Weight,
		post.AcceptedAnswerID,
		post.EvolvedInto,
		post.EmbeddingStr,
		visibilityOrDefault(post.Visibility),
		post.OwnerHumanID,
		pub,
		mod,
		post.SourceRoomID,
		post.IdempotencyKey,
	)

	created, err := r.scanPost(row)
	if err != nil {
		return nil, err
	}
	// idempotency_key is intentionally omitted from RETURNING (the shared scanner has a
	// fixed column set); carry the caller's key back so callers can echo it if needed.
	created.IdempotencyKey = post.IdempotencyKey
	return created, nil
}

// FindByID returns a single post by ID with author information.
// Returns ErrPostNotFound if the post doesn't exist or is soft-deleted.
// UserVote is always nil (no viewer context). Use FindByIDForViewer for authenticated lookups.
func (r *PostRepository) FindByID(ctx context.Context, id string) (*models.PostWithAuthor, error) {
	return r.findByIDInternal(ctx, id, "", "", "")
}

// FindByIDForViewer returns a single post by ID with the viewer's vote included.
// If viewerType/viewerID are empty, behaves identically to FindByID.
func (r *PostRepository) FindByIDForViewer(ctx context.Context, id string, viewerType models.AuthorType, viewerID string, callerHuman string) (*models.PostWithAuthor, error) {
	return r.findByIDInternal(ctx, id, viewerType, viewerID, callerHuman)
}

// findByIDInternal is the shared implementation for FindByID and FindByIDForViewer.
// callerHuman is the caller's family human UUID for visibility scoping ("" = public-only);
// a family post the caller may not see yields no row → ErrPostNotFound (404, existence hidden).
func (r *PostRepository) findByIDInternal(ctx context.Context, id string, viewerType models.AuthorType, viewerID string, callerHuman string) (*models.PostWithAuthor, error) {
	var viewerVoteColumn, viewerVoteJoin string
	var args []any

	if viewerType != "" && viewerID != "" {
		viewerVoteColumn = "v.direction as user_vote_direction"
		viewerVoteJoin = "LEFT JOIN votes v ON v.target_type = 'post' AND v.target_id = p.id AND v.voter_type = $2 AND v.voter_id = $3"
		args = []any{id, string(viewerType), viewerID}
	} else {
		viewerVoteColumn = "NULL::text as user_vote_direction"
		viewerVoteJoin = ""
		args = []any{id}
	}

	// BART-151: family-scoped visibility gate.
	visClause := "p.visibility = 'public'"
	if callerHuman != "" {
		args = append(args, callerHuman)
		visClause = fmt.Sprintf("(p.visibility = 'public' OR (p.owner_human_id IS NOT NULL AND p.owner_human_id = $%d::uuid))", len(args))
	}

	query := fmt.Sprintf(`
		SELECT
			p.id, p.type, p.title, p.description, p.tags,
			p.posted_by_type, p.posted_by_id, p.status,
			p.upvotes, p.downvotes, p.view_count, p.success_criteria, p.weight,
			p.accepted_answer_id, p.evolved_into,
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
		FROM posts p
		LEFT JOIN users u ON p.posted_by_type = 'human' AND p.posted_by_id = u.id::text
		LEFT JOIN agents ag ON p.posted_by_type = 'agent' AND p.posted_by_id = ag.id
		%s
		%s
		WHERE p.id = $1 AND p.deleted_at IS NULL AND %s
	`, postReplyCountColumns, viewerVoteColumn, postReplyCountsJoin, viewerVoteJoin, visClause)

	row := r.pool.QueryRow(ctx, query, args...)

	var post models.PostWithAuthor
	var authorDisplayName, authorAvatarURL string

	err := row.Scan(
		&post.ID,
		&post.Type,
		&post.Title,
		&post.Description,
		&post.Tags,
		&post.PostedByType,
		&post.PostedByID,
		&post.Status,
		&post.Upvotes,
		&post.Downvotes,
		&post.ViewCount,
		&post.SuccessCriteria,
		&post.Weight,
		&post.AcceptedAnswerID,
		&post.EvolvedInto,
		&post.CreatedAt,
		&post.UpdatedAt,
		&post.DeletedAt,
		&post.CrystallizationCID,
		&post.CrystallizedAt,
		&post.OriginalLanguage,
		&post.OriginalTitle,
		&post.OriginalDescription,
		&authorDisplayName,
		&authorAvatarURL,
		&post.AnswersCount,
		&post.ApproachesCount,
		&post.CommentsCount,
		&post.AgentHumanID,
		&post.UserVote,
		&post.Visibility,
		&post.PublicationState,
		&post.ModerationState,
		&post.SourceRoomID,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			slog.Debug("post not found", "op", "FindByID", "table", "posts", "id", id)
			return nil, ErrPostNotFound
		}
		// FIX-007: Invalid UUID format should return ErrPostNotFound (404), not 500.
		if isInvalidUUIDError(err) {
			slog.Debug("invalid UUID format", "op", "FindByID", "table", "posts", "id", id)
			return nil, ErrPostNotFound
		}
		LogQueryError(ctx, "FindByID", "posts", err)
		return nil, fmt.Errorf("query failed: %w", err)
	}

	// Populate author information
	post.Author = models.PostAuthor{
		Type:        post.PostedByType,
		ID:          post.PostedByID,
		DisplayName: authorDisplayName,
		AvatarURL:   authorAvatarURL,
	}

	// Compute vote score
	post.VoteScore = post.Upvotes - post.Downvotes

	// Canonical unified reply count (BART-583): the buckets partition the post's live replies.
	post.ReplyCount = post.AnswersCount + post.ApproachesCount + post.CommentsCount

	return &post, nil
}

// Update updates an existing post in the database unconditionally.
// Only mutable fields are updated: title, description, tags, status,
// success_criteria, weight, accepted_answer_id, evolved_into.
// Returns ErrPostNotFound if the post doesn't exist or is soft-deleted.
// The write itself lives in posts_conditional.go (UpdateIfUnmodified).
func (r *PostRepository) Update(ctx context.Context, post *models.Post) (*models.Post, error) {
	return r.UpdateIfUnmodified(ctx, post, nil)
}

// Delete performs a soft delete on a post by setting deleted_at.
// Returns ErrPostNotFound if the post doesn't exist or is already deleted.
func (r *PostRepository) Delete(ctx context.Context, id string) error {
	query := `
		UPDATE posts
		SET deleted_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL
	`

	result, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		// FIX-007: Invalid UUID format should return ErrPostNotFound (404), not 500.
		if isInvalidUUIDError(err) {
			slog.Debug("invalid UUID format", "op", "Delete", "table", "posts", "id", id)
			return ErrPostNotFound
		}
		LogQueryError(ctx, "Delete", "posts", err)
		return fmt.Errorf("delete query failed: %w", err)
	}

	if result.RowsAffected() == 0 {
		return ErrPostNotFound
	}

	return nil
}

// Vote adds or updates a vote on a post.
// If the voter hasn't voted, it inserts a new vote.
// If the voter has voted with a different direction, it updates the vote.
// If the voter has voted with the same direction, it's a no-op.
// The post's upvotes/downvotes follow the vote row inside the same statement (migration
// 000113 triggers), so concurrent or replayed requests from one voter count once.
// Per SPEC.md Part 2.9: One vote per entity per target.
func (r *PostRepository) Vote(ctx context.Context, postID, voterType, voterID, direction string) error {
	// Validate direction
	if direction != "up" && direction != "down" {
		return ErrInvalidVoteDirection
	}

	// Validate voter type
	if voterType != "human" && voterType != "agent" {
		return ErrInvalidVoterType
	}

	// Check if post exists and is not deleted
	var exists bool
	err := r.pool.QueryRow(ctx,
		"SELECT EXISTS(SELECT 1 FROM posts WHERE id = $1 AND deleted_at IS NULL)",
		postID,
	).Scan(&exists)
	if err != nil {
		// FIX-007: Invalid UUID format should return ErrPostNotFound (404), not 500.
		if isInvalidUUIDError(err) {
			slog.Debug("invalid UUID format", "op", "Vote.CheckExists", "table", "posts", "id", postID)
			return ErrPostNotFound
		}
		LogQueryError(ctx, "Vote.CheckExists", "posts", err)
		return fmt.Errorf("failed to check post existence: %w", err)
	}
	if !exists {
		return ErrPostNotFound
	}

	// One statement: a concurrent first vote from the same voter becomes an update instead
	// of a unique-key failure, and repeating the current direction writes nothing.
	_, err = r.pool.Exec(ctx,
		`INSERT INTO votes (target_type, target_id, voter_type, voter_id, direction, confirmed)
		 VALUES ('post', $1, $2, $3, $4, true)
		 ON CONFLICT ON CONSTRAINT votes_unique_per_target
		 DO UPDATE SET direction = EXCLUDED.direction
		 WHERE votes.direction IS DISTINCT FROM EXCLUDED.direction`,
		postID, voterType, voterID, direction,
	)
	if err != nil {
		LogQueryError(ctx, "Vote.Upsert", "votes", err)
		return fmt.Errorf("failed to record vote: %w", err)
	}
	return nil
}

// GetUserVote returns the user's current vote on a post, or nil if not voted.
// Returns ErrPostNotFound if the post doesn't exist or is deleted.
func (r *PostRepository) GetUserVote(ctx context.Context, postID, voterType, voterID string) (*string, error) {
	// Check if post exists (same pattern as Vote() line 541-557)
	var exists bool
	err := r.pool.QueryRow(ctx,
		"SELECT EXISTS(SELECT 1 FROM posts WHERE id = $1 AND deleted_at IS NULL)",
		postID,
	).Scan(&exists)
	if err != nil {
		if isInvalidUUIDError(err) {
			slog.Debug("invalid UUID format", "op", "GetUserVote.CheckExists", "table", "posts", "id", postID)
			return nil, ErrPostNotFound
		}
		LogQueryError(ctx, "GetUserVote.CheckExists", "posts", err)
		return nil, fmt.Errorf("failed to check post existence: %w", err)
	}
	if !exists {
		return nil, ErrPostNotFound
	}

	// Get user's vote (same query as Vote() line 561-566)
	var direction string
	err = r.pool.QueryRow(ctx,
		`SELECT direction FROM votes
		 WHERE target_type = 'post' AND target_id = $1
		 AND voter_type = $2 AND voter_id = $3`,
		postID, voterType, voterID,
	).Scan(&direction)

	if err != nil && err.Error() != "no rows in result set" {
		LogQueryError(ctx, "GetUserVote", "votes", err)
		return nil, fmt.Errorf("failed to get user vote: %w", err)
	}

	if direction == "" {
		return nil, nil // No vote
	}
	return &direction, nil
}

// SetCrystallizationCID sets the IPFS CID for a crystallized problem snapshot.
// Sets both crystallization_cid and crystallized_at (to current time).
// Returns ErrPostNotFound if the post doesn't exist or is deleted.
func (r *PostRepository) SetCrystallizationCID(ctx context.Context, postID, cid string) error {
	query := `
		UPDATE posts
		SET crystallization_cid = $2, crystallized_at = NOW(), updated_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL
	`

	result, err := r.pool.Exec(ctx, query, postID, cid)
	if err != nil {
		if isInvalidUUIDError(err) {
			return ErrPostNotFound
		}
		LogQueryError(ctx, "SetCrystallizationCID", "posts", err)
		return fmt.Errorf("set crystallization CID failed: %w", err)
	}

	if result.RowsAffected() == 0 {
		return ErrPostNotFound
	}

	return nil
}

// UpdateStatus updates only the status of a post.
func (r *PostRepository) UpdateStatus(ctx context.Context, postID string, status models.PostStatus) error {
	// Keep the canonical states in step with the legacy status (BART-583): a moderator
	// approving (status -> open) also marks it published+approved; rejecting marks it
	// draft+rejected. This is the only path that can set moderation_state to approved.
	pub, mod := models.DeriveStates(status)
	query := `
		UPDATE posts SET status = $1, publication_state = $3, moderation_state = $4, updated_at = NOW()
		WHERE id = $2 AND deleted_at IS NULL
	`

	result, err := r.pool.Exec(ctx, query, status, postID, pub, mod)
	if err != nil {
		if isInvalidUUIDError(err) {
			return ErrPostNotFound
		}
		LogQueryError(ctx, "UpdateStatus", "posts", err)
		return fmt.Errorf("update post status failed: %w", err)
	}

	if result.RowsAffected() == 0 {
		return ErrPostNotFound
	}

	return nil
}

// GetLatestPostTimestamp returns the most recent created_at timestamp from open, non-deleted posts.
// Returns nil if no posts exist.
func (r *PostRepository) GetLatestPostTimestamp(ctx context.Context) (*time.Time, error) {
	query := `SELECT MAX(created_at) FROM posts WHERE status = 'open' AND deleted_at IS NULL`

	var ts *time.Time
	err := r.pool.QueryRow(ctx, query).Scan(&ts)
	if err != nil {
		LogQueryError(ctx, "GetLatestPostTimestamp", "posts", err)
		return nil, fmt.Errorf("get latest post timestamp failed: %w", err)
	}

	return ts, nil
}
