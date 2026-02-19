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

	whereClause := strings.Join(conditions, " AND ")

	// Build answer count filter condition for main query
	// This will be added after the LEFT JOIN so ans_cnt.cnt is available
	var answerCountFilter string
	if opts.HasAnswer != nil {
		if *opts.HasAnswer {
			answerCountFilter = " AND COALESCE(ans_cnt.cnt, 0) > 0"
		} else {
			answerCountFilter = " AND COALESCE(ans_cnt.cnt, 0) = 0"
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
				LEFT JOIN (
					SELECT question_id, COUNT(*) as cnt
					FROM answers WHERE deleted_at IS NULL
					GROUP BY question_id
				) ans_cnt ON ans_cnt.question_id = p.id
				WHERE %s%s
			) counted
		`, whereClause, answerCountFilter)
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
	case "approaches":
		orderClause = "COALESCE(app_cnt.cnt, 0) DESC, p.created_at DESC"
	case "answers":
		orderClause = "COALESCE(ans_cnt.cnt, 0) DESC, p.created_at DESC"
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
			COALESCE(u.display_name, ag.display_name, '') as author_display_name,
			COALESCE(u.avatar_url, ag.avatar_url, '') as author_avatar_url,
			COALESCE(ans_cnt.cnt, 0) as answers_count,
			COALESCE(app_cnt.cnt, 0) as approaches_count,
			COALESCE(cmt_cnt.cnt, 0) as comments_count
		FROM posts p
		LEFT JOIN users u ON p.posted_by_type = 'human' AND p.posted_by_id = u.id::text
		LEFT JOIN agents ag ON p.posted_by_type = 'agent' AND p.posted_by_id = ag.id
		LEFT JOIN (
			SELECT question_id, COUNT(*) as cnt
			FROM answers WHERE deleted_at IS NULL
			GROUP BY question_id
		) ans_cnt ON ans_cnt.question_id = p.id
		LEFT JOIN (
			SELECT problem_id, COUNT(*) as cnt
			FROM approaches WHERE deleted_at IS NULL
			GROUP BY problem_id
		) app_cnt ON app_cnt.problem_id = p.id
		LEFT JOIN (
			SELECT target_id, COUNT(*) as cnt
			FROM comments
			WHERE target_type = 'post' AND deleted_at IS NULL
			GROUP BY target_id
		) cmt_cnt ON cmt_cnt.target_id = p.id
		WHERE %s%s
		ORDER BY %s
		LIMIT $%d OFFSET $%d
	`, whereClause, answerCountFilter, orderClause, argNum, argNum+1)

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
		&authorDisplayName,
		&authorAvatarURL,
		&post.AnswersCount,
		&post.ApproachesCount,
		&post.CommentsCount,
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
			created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14::vector, NOW(), NOW())
		RETURNING id, type, title, description, tags,
			posted_by_type, posted_by_id, status,
			upvotes, downvotes, view_count, success_criteria, weight,
			accepted_answer_id, evolved_into,
			created_at, updated_at, deleted_at,
			crystallization_cid, crystallized_at
	`

	// Default status to 'draft' if not provided
	status := post.Status
	if status == "" {
		status = models.PostStatusDraft
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
	)

	return r.scanPost(row)
}

// FindByID returns a single post by ID with author information.
// Returns ErrPostNotFound if the post doesn't exist or is soft-deleted.
func (r *PostRepository) FindByID(ctx context.Context, id string) (*models.PostWithAuthor, error) {
	query := `
		SELECT
			p.id, p.type, p.title, p.description, p.tags,
			p.posted_by_type, p.posted_by_id, p.status,
			p.upvotes, p.downvotes, p.view_count, p.success_criteria, p.weight,
			p.accepted_answer_id, p.evolved_into,
			p.created_at, p.updated_at, p.deleted_at,
			p.crystallization_cid, p.crystallized_at,
			COALESCE(u.display_name, ag.display_name, '') as author_display_name,
			COALESCE(u.avatar_url, ag.avatar_url, '') as author_avatar_url,
			COALESCE(ans_cnt.cnt, 0) as answers_count,
			COALESCE(app_cnt.cnt, 0) as approaches_count,
			COALESCE(cmt_cnt.cnt, 0) as comments_count
		FROM posts p
		LEFT JOIN users u ON p.posted_by_type = 'human' AND p.posted_by_id = u.id::text
		LEFT JOIN agents ag ON p.posted_by_type = 'agent' AND p.posted_by_id = ag.id
		LEFT JOIN (
			SELECT question_id, COUNT(*) as cnt
			FROM answers WHERE deleted_at IS NULL
			GROUP BY question_id
		) ans_cnt ON ans_cnt.question_id = p.id
		LEFT JOIN (
			SELECT problem_id, COUNT(*) as cnt
			FROM approaches WHERE deleted_at IS NULL
			GROUP BY problem_id
		) app_cnt ON app_cnt.problem_id = p.id
		LEFT JOIN (
			SELECT target_id, COUNT(*) as cnt
			FROM comments
			WHERE target_type = 'post' AND deleted_at IS NULL
			GROUP BY target_id
		) cmt_cnt ON cmt_cnt.target_id = p.id
		WHERE p.id = $1 AND p.deleted_at IS NULL
	`

	row := r.pool.QueryRow(ctx, query, id)

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
		&authorDisplayName,
		&authorAvatarURL,
		&post.AnswersCount,
		&post.ApproachesCount,
		&post.CommentsCount,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			slog.Debug("post not found", "op", "FindByID", "table", "posts", "id", id)
			return nil, ErrPostNotFound
		}
		// FIX-007: Invalid UUID format should return ErrPostNotFound (404), not 500.
		// This makes behavior consistent: any invalid or non-existent post ID returns 404.
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

	return &post, nil
}

// Update updates an existing post in the database.
// Only mutable fields are updated: title, description, tags, status,
// success_criteria, weight, accepted_answer_id, evolved_into.
// Returns ErrPostNotFound if the post doesn't exist or is soft-deleted.
func (r *PostRepository) Update(ctx context.Context, post *models.Post) (*models.Post, error) {
	query := `
		UPDATE posts
		SET
			title = $2,
			description = $3,
			tags = $4,
			status = $5,
			success_criteria = $6,
			weight = $7,
			accepted_answer_id = $8,
			evolved_into = $9,
			embedding = COALESCE($10::vector, embedding),
			updated_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL
		RETURNING id, type, title, description, tags,
			posted_by_type, posted_by_id, status,
			upvotes, downvotes, view_count, success_criteria, weight,
			accepted_answer_id, evolved_into,
			created_at, updated_at, deleted_at,
			crystallization_cid, crystallized_at
	`

	row := r.pool.QueryRow(ctx, query,
		post.ID,
		post.Title,
		post.Description,
		post.Tags,
		post.Status,
		post.SuccessCriteria,
		post.Weight,
		post.AcceptedAnswerID,
		post.EvolvedInto,
		post.EmbeddingStr,
	)

	return r.scanPost(row)
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
// If the voter has voted with a different direction, it updates the vote and adjusts counts.
// If the voter has voted with the same direction, it's a no-op.
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

	// Check for existing vote
	var existingDirection string
	err = r.pool.QueryRow(ctx,
		`SELECT direction FROM votes
		 WHERE target_type = 'post' AND target_id = $1
		 AND voter_type = $2 AND voter_id = $3`,
		postID, voterType, voterID,
	).Scan(&existingDirection)

	if err != nil && err.Error() != "no rows in result set" {
		LogQueryError(ctx, "Vote.CheckExisting", "votes", err)
		return fmt.Errorf("failed to check existing vote: %w", err)
	}

	// If same vote exists, nothing to do
	if existingDirection == direction {
		return nil
	}

	// Use WithTx for atomicity
	return r.pool.WithTx(ctx, func(tx Tx) error {
		if existingDirection == "" {
			// No existing vote - insert new vote and update post counts
			_, err = tx.Exec(ctx,
				`INSERT INTO votes (target_type, target_id, voter_type, voter_id, direction, confirmed)
				 VALUES ('post', $1, $2, $3, $4, true)`,
				postID, voterType, voterID, direction,
			)
			if err != nil {
				LogQueryError(ctx, "Vote.InsertVote", "votes", err)
				return fmt.Errorf("failed to insert vote: %w", err)
			}

			// Update post vote counts
			if direction == "up" {
				_, err = tx.Exec(ctx,
					"UPDATE posts SET upvotes = upvotes + 1 WHERE id = $1",
					postID,
				)
			} else {
				_, err = tx.Exec(ctx,
					"UPDATE posts SET downvotes = downvotes + 1 WHERE id = $1",
					postID,
				)
			}
			if err != nil {
				LogQueryError(ctx, "Vote.UpdateCounts", "posts", err)
				return fmt.Errorf("failed to update post vote counts: %w", err)
			}
		} else {
			// Existing vote with different direction - update vote and adjust counts
			_, err = tx.Exec(ctx,
				`UPDATE votes SET direction = $4
				 WHERE target_type = 'post' AND target_id = $1
				 AND voter_type = $2 AND voter_id = $3`,
				postID, voterType, voterID, direction,
			)
			if err != nil {
				LogQueryError(ctx, "Vote.UpdateDirection", "votes", err)
				return fmt.Errorf("failed to update vote: %w", err)
			}

			// Adjust post vote counts: decrement old, increment new
			if direction == "up" {
				// Was down, now up: downvotes--, upvotes++
				_, err = tx.Exec(ctx,
					"UPDATE posts SET upvotes = upvotes + 1, downvotes = downvotes - 1 WHERE id = $1",
					postID,
				)
			} else {
				// Was up, now down: upvotes--, downvotes++
				_, err = tx.Exec(ctx,
					"UPDATE posts SET upvotes = upvotes - 1, downvotes = downvotes + 1 WHERE id = $1",
					postID,
				)
			}
			if err != nil {
				LogQueryError(ctx, "Vote.AdjustCounts", "posts", err)
				return fmt.Errorf("failed to adjust post vote counts: %w", err)
			}
		}

		return nil
	})
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

// ListCrystallizationCandidates returns post IDs of solved problems that are
// eligible for crystallization: type=problem, status=solved, not deleted,
// not already crystallized, and stable for at least stabilityPeriod.
// Results are ordered by oldest updated_at first (crystallize oldest stable problems first).
func (r *PostRepository) ListCrystallizationCandidates(ctx context.Context, stabilityPeriod time.Duration, limit int) ([]string, error) {
	if limit <= 0 {
		limit = 50
	}

	query := `
		SELECT id::text
		FROM posts
		WHERE type = 'problem'
		  AND status = 'solved'
		  AND deleted_at IS NULL
		  AND crystallization_cid IS NULL
		  AND updated_at < NOW() - $1::interval
		ORDER BY updated_at ASC
		LIMIT $2
	`

	// Convert Go time.Duration to PostgreSQL interval string
	intervalStr := fmt.Sprintf("%d seconds", int(stabilityPeriod.Seconds()))

	rows, err := r.pool.Query(ctx, query, intervalStr, limit)
	if err != nil {
		LogQueryError(ctx, "ListCrystallizationCandidates", "posts", err)
		return nil, fmt.Errorf("list crystallization candidates: %w", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan crystallization candidate: %w", err)
		}
		ids = append(ids, id)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate crystallization candidates: %w", err)
	}

	// Return empty slice instead of nil for consistent API
	if ids == nil {
		ids = []string{}
	}

	return ids, nil
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
