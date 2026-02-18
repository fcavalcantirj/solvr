// Package db provides database access for Solvr.
package db

import (
	"context"
	"fmt"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// Note: Feed queries use LogQueryError from logger.go for error logging.

// FeedRepository handles database operations for the feed.
// Per SPEC.md Part 5.6: Feed endpoints.
type FeedRepository struct {
	pool *Pool
}

// NewFeedRepository creates a new FeedRepository.
func NewFeedRepository(pool *Pool) *FeedRepository {
	return &FeedRepository{pool: pool}
}

// GetRecentActivity returns recent posts ordered by created_at DESC.
// Per SPEC.md Part 5.6: GET /feed - Recent activity
func (r *FeedRepository) GetRecentActivity(ctx context.Context, page, perPage int) ([]models.FeedItem, int, error) {
	// Validate pagination
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 20
	}
	if perPage > 50 {
		perPage = 50
	}
	offset := (page - 1) * perPage

	// Count total
	countQuery := `SELECT COUNT(*) FROM posts WHERE deleted_at IS NULL`
	var total int
	err := r.pool.QueryRow(ctx, countQuery).Scan(&total)
	if err != nil {
		LogQueryError(ctx, "GetRecentActivity.Count", "posts", err)
		return nil, 0, fmt.Errorf("count query failed: %w", err)
	}

	// Query for recent activity with author info
	// Uses LEFT JOIN subqueries instead of correlated subqueries for better performance
	query := `
		SELECT
			p.id, p.type, p.title, p.description, p.tags,
			p.status, p.posted_by_type, p.posted_by_id,
			p.upvotes - p.downvotes as vote_score,
			CASE
				WHEN p.type = 'question' THEN COALESCE(ans_cnt.cnt, 0)
				WHEN p.type = 'idea' THEN COALESCE(resp_cnt.cnt, 0)
				ELSE 0
			END as answer_count,
			COALESCE(app_cnt.cnt, 0) as approach_count,
			COALESCE(cmt_cnt.cnt, 0) as comment_count,
			p.created_at,
			COALESCE(u.display_name, a.display_name, '') as author_display_name,
			COALESCE(u.avatar_url, a.avatar_url, '') as author_avatar_url
		FROM posts p
		LEFT JOIN users u ON p.posted_by_type = 'human' AND p.posted_by_id = u.id::text
		LEFT JOIN agents a ON p.posted_by_type = 'agent' AND p.posted_by_id = a.id
		LEFT JOIN (
			SELECT question_id, COUNT(*) as cnt
			FROM answers
			WHERE deleted_at IS NULL
			GROUP BY question_id
		) ans_cnt ON ans_cnt.question_id = p.id
		LEFT JOIN (
			SELECT problem_id, COUNT(*) as cnt
			FROM approaches
			WHERE deleted_at IS NULL
			GROUP BY problem_id
		) app_cnt ON app_cnt.problem_id = p.id
		LEFT JOIN (
			SELECT idea_id, COUNT(*) as cnt
			FROM responses
			GROUP BY idea_id
		) resp_cnt ON resp_cnt.idea_id = p.id
		LEFT JOIN (
			SELECT target_id, COUNT(*) as cnt
			FROM comments
			WHERE target_type = 'post' AND deleted_at IS NULL
			GROUP BY target_id
		) cmt_cnt ON cmt_cnt.target_id = p.id
		WHERE p.deleted_at IS NULL
		ORDER BY p.created_at DESC
		LIMIT $1 OFFSET $2
	`

	rows, err := r.pool.Query(ctx, query, perPage, offset)
	if err != nil {
		LogQueryError(ctx, "GetRecentActivity", "posts", err)
		return nil, 0, fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	items := make([]models.FeedItem, 0)
	for rows.Next() {
		item, err := r.scanFeedItem(rows)
		if err != nil {
			LogQueryError(ctx, "GetRecentActivity.Scan", "posts", err)
			return nil, 0, fmt.Errorf("scan failed: %w", err)
		}
		items = append(items, *item)
	}

	if err := rows.Err(); err != nil {
		LogQueryError(ctx, "GetRecentActivity.Rows", "posts", err)
		return nil, 0, fmt.Errorf("rows iteration failed: %w", err)
	}

	return items, total, nil
}

// GetStuckProblems returns problems that need help.
// Per SPEC.md Part 5.6: GET /feed/stuck - Problems needing help
// Returns problems that have approaches with status='stuck' or problems with status='in_progress'.
func (r *FeedRepository) GetStuckProblems(ctx context.Context, page, perPage int) ([]models.FeedItem, int, error) {
	// Validate pagination
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 20
	}
	if perPage > 50 {
		perPage = 50
	}
	offset := (page - 1) * perPage

	// Count total - problems that are stuck (have stuck approaches or are in_progress)
	countQuery := `
		SELECT COUNT(DISTINCT p.id)
		FROM posts p
		WHERE p.type = 'problem'
		AND p.deleted_at IS NULL
		AND (
			p.status = 'in_progress'
			OR EXISTS (
				SELECT 1 FROM approaches ap
				WHERE ap.problem_id = p.id
				AND ap.status = 'stuck'
				AND ap.deleted_at IS NULL
			)
		)
	`
	var total int
	err := r.pool.QueryRow(ctx, countQuery).Scan(&total)
	if err != nil {
		LogQueryError(ctx, "GetStuckProblems.Count", "posts", err)
		return nil, 0, fmt.Errorf("count query failed: %w", err)
	}

	// Query for stuck problems
	query := `
		SELECT DISTINCT
			p.id, p.type, p.title, p.description, p.tags,
			p.status, p.posted_by_type, p.posted_by_id,
			p.upvotes - p.downvotes as vote_score,
			0 as answer_count,
			(SELECT COUNT(*) FROM approaches ap WHERE ap.problem_id = p.id AND ap.deleted_at IS NULL) as approach_count,
			(SELECT COUNT(*) FROM comments c WHERE c.target_type = 'post' AND c.target_id = p.id AND c.deleted_at IS NULL) as comment_count,
			p.created_at,
			COALESCE(u.display_name, a.display_name, '') as author_display_name,
			COALESCE(u.avatar_url, a.avatar_url, '') as author_avatar_url
		FROM posts p
		LEFT JOIN users u ON p.posted_by_type = 'human' AND p.posted_by_id = u.id::text
		LEFT JOIN agents a ON p.posted_by_type = 'agent' AND p.posted_by_id = a.id
		WHERE p.type = 'problem'
		AND p.deleted_at IS NULL
		AND (
			p.status = 'in_progress'
			OR EXISTS (
				SELECT 1 FROM approaches ap
				WHERE ap.problem_id = p.id
				AND ap.status = 'stuck'
				AND ap.deleted_at IS NULL
			)
		)
		ORDER BY p.created_at DESC
		LIMIT $1 OFFSET $2
	`

	rows, err := r.pool.Query(ctx, query, perPage, offset)
	if err != nil {
		LogQueryError(ctx, "GetStuckProblems", "posts", err)
		return nil, 0, fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	items := make([]models.FeedItem, 0)
	for rows.Next() {
		item, err := r.scanFeedItem(rows)
		if err != nil {
			LogQueryError(ctx, "GetStuckProblems.Scan", "posts", err)
			return nil, 0, fmt.Errorf("scan failed: %w", err)
		}
		items = append(items, *item)
	}

	if err := rows.Err(); err != nil {
		LogQueryError(ctx, "GetStuckProblems.Rows", "posts", err)
		return nil, 0, fmt.Errorf("rows iteration failed: %w", err)
	}

	return items, total, nil
}

// GetUnansweredQuestions returns questions with zero answers.
// Per SPEC.md Part 5.6: GET /feed/unanswered - Unanswered questions
func (r *FeedRepository) GetUnansweredQuestions(ctx context.Context, page, perPage int) ([]models.FeedItem, int, error) {
	// Validate pagination
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 20
	}
	if perPage > 50 {
		perPage = 50
	}
	offset := (page - 1) * perPage

	// Count total - questions with zero answers
	countQuery := `
		SELECT COUNT(*)
		FROM posts p
		WHERE p.type = 'question'
		AND p.deleted_at IS NULL
		AND NOT EXISTS (
			SELECT 1 FROM answers a
			WHERE a.question_id = p.id
			AND a.deleted_at IS NULL
		)
	`
	var total int
	err := r.pool.QueryRow(ctx, countQuery).Scan(&total)
	if err != nil {
		LogQueryError(ctx, "GetUnansweredQuestions.Count", "posts", err)
		return nil, 0, fmt.Errorf("count query failed: %w", err)
	}

	// Query for unanswered questions
	query := `
		SELECT
			p.id, p.type, p.title, p.description, p.tags,
			p.status, p.posted_by_type, p.posted_by_id,
			p.upvotes - p.downvotes as vote_score,
			0 as answer_count,
			0 as approach_count,
			(SELECT COUNT(*) FROM comments c WHERE c.target_type = 'post' AND c.target_id = p.id AND c.deleted_at IS NULL) as comment_count,
			p.created_at,
			COALESCE(u.display_name, a.display_name, '') as author_display_name,
			COALESCE(u.avatar_url, a.avatar_url, '') as author_avatar_url
		FROM posts p
		LEFT JOIN users u ON p.posted_by_type = 'human' AND p.posted_by_id = u.id::text
		LEFT JOIN agents a ON p.posted_by_type = 'agent' AND p.posted_by_id = a.id
		WHERE p.type = 'question'
		AND p.deleted_at IS NULL
		AND NOT EXISTS (
			SELECT 1 FROM answers ans
			WHERE ans.question_id = p.id
			AND ans.deleted_at IS NULL
		)
		ORDER BY p.created_at DESC
		LIMIT $1 OFFSET $2
	`

	rows, err := r.pool.Query(ctx, query, perPage, offset)
	if err != nil {
		LogQueryError(ctx, "GetUnansweredQuestions", "posts", err)
		return nil, 0, fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	items := make([]models.FeedItem, 0)
	for rows.Next() {
		item, err := r.scanFeedItem(rows)
		if err != nil {
			LogQueryError(ctx, "GetUnansweredQuestions.Scan", "posts", err)
			return nil, 0, fmt.Errorf("scan failed: %w", err)
		}
		items = append(items, *item)
	}

	if err := rows.Err(); err != nil {
		LogQueryError(ctx, "GetUnansweredQuestions.Rows", "posts", err)
		return nil, 0, fmt.Errorf("rows iteration failed: %w", err)
	}

	return items, total, nil
}

// scanFeedItem scans a row into a FeedItem.
func (r *FeedRepository) scanFeedItem(rows interface{ Scan(dest ...any) error }) (*models.FeedItem, error) {
	var item models.FeedItem
	var description string
	var authorDisplayName, authorAvatarURL string

	err := rows.Scan(
		&item.ID,
		&item.Type,
		&item.Title,
		&description,
		&item.Tags,
		&item.Status,
		&item.Author.Type,
		&item.Author.ID,
		&item.VoteScore,
		&item.AnswerCount,
		&item.ApproachCount,
		&item.CommentCount,
		&item.CreatedAt,
		&authorDisplayName,
		&authorAvatarURL,
	)
	if err != nil {
		return nil, err
	}

	// Create snippet from description (first 200 chars)
	if len(description) > 200 {
		item.Snippet = description[:200] + "..."
	} else {
		item.Snippet = description
	}

	// Set author details
	item.Author.DisplayName = authorDisplayName
	item.Author.AvatarURL = authorAvatarURL

	return &item, nil
}
