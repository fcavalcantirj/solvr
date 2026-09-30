package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/jackc/pgx/v5"
)

// ErrNotHideable is returned for a contribution kind whose table has no soft delete
// (responses, progress notes): moderation can only flag those.
var ErrNotHideable = errors.New("contribution kind cannot be hidden")

// ContributionModerationRepository serves asynchronous moderation of replies and legacy
// contributions (anti-abuse W2): it hides what moderation rejects, names the post a
// contribution belongs to, and records the verdict as an admin flag.
type ContributionModerationRepository struct {
	pool *Pool
}

// NewContributionModerationRepository creates a ContributionModerationRepository.
func NewContributionModerationRepository(pool *Pool) *ContributionModerationRepository {
	return &ContributionModerationRepository{pool: pool}
}

var hideableContributionTables = map[string]string{
	"reply": "replies", "answer": "answers", "approach": "approaches", "comment": "comments",
}

// Hide soft-deletes a rejected reply, answer, approach or comment, like its author's delete.
func (r *ContributionModerationRepository) Hide(ctx context.Context, kind, id string) error {
	table, ok := hideableContributionTables[kind]
	if !ok {
		return ErrNotHideable
	}
	if _, err := r.pool.Exec(ctx, `UPDATE `+table+` SET deleted_at = NOW() WHERE id::text = $1 AND deleted_at IS NULL`, id); err != nil {
		LogQueryError(ctx, "Hide", table, err)
		return fmt.Errorf("hide %s: %w", kind, err)
	}
	return nil
}

// ParentPost returns the id, type and title of the post a contribution belongs to (a comment
// through the answer, approach or response it is on). Empty values when it cannot be resolved.
func (r *ContributionModerationRepository) ParentPost(ctx context.Context, kind, id string) (postID, postType, title string, err error) {
	query := `SELECT p.id::text, p.type, p.title FROM posts p JOIN replies x ON x.post_id = p.id WHERE x.id::text = $2 AND $1 = 'reply'`
	if kind != "reply" {
		legacy, lerr := legacyContributionTablesPresent(ctx, r.pool)
		if lerr != nil || !legacy {
			return "", "", "", lerr // a legacy contribution cannot outlive its table
		}
		query = legacyParentPostQuery
	}
	err = r.pool.QueryRow(ctx, query, kind, id).Scan(&postID, &postType, &title)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", "", nil
	}
	if err != nil {
		LogQueryError(ctx, "ParentPost", "posts", err)
		return "", "", "", fmt.Errorf("parent post of %s: %w", kind, err)
	}
	return postID, postType, title, nil
}

// legacyParentPostQuery resolves the post of a legacy contribution (a comment through the
// answer, approach or response it is on). Used only while the legacy tables exist.
const legacyParentPostQuery = `
		SELECT p.id::text, p.type, p.title FROM posts p
		WHERE p.id = (CASE $1
		    WHEN 'answer'   THEN (SELECT question_id FROM answers WHERE id::text = $2)
		    WHEN 'approach' THEN (SELECT problem_id FROM approaches WHERE id::text = $2)
		    WHEN 'response' THEN (SELECT idea_id FROM responses WHERE id::text = $2)
		    WHEN 'progress_note' THEN (SELECT ap.problem_id FROM progress_notes pn
		                               JOIN approaches ap ON ap.id = pn.approach_id WHERE pn.id::text = $2)
		    WHEN 'comment'  THEN (SELECT CASE c.target_type
		                               WHEN 'post'     THEN c.target_id
		                               WHEN 'answer'   THEN (SELECT question_id FROM answers WHERE id = c.target_id)
		                               WHEN 'approach' THEN (SELECT problem_id FROM approaches WHERE id = c.target_id)
		                               WHEN 'response' THEN (SELECT idea_id FROM responses WHERE id = c.target_id)
		                           END FROM comments c WHERE c.id::text = $2)
		END)`

// CreateFlag records an admin flag (reporter 'system' for moderation verdicts).
func (r *ContributionModerationRepository) CreateFlag(ctx context.Context, flag *models.Flag) (*models.Flag, error) {
	created := *flag
	err := r.pool.QueryRow(ctx, `
		INSERT INTO flags (target_type, target_id, reporter_type, reporter_id, reason, details, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at`,
		flag.TargetType, flag.TargetID, flag.ReporterType, flag.ReporterID, flag.Reason, flag.Details, flag.Status,
	).Scan(&created.ID, &created.CreatedAt)
	if err != nil {
		LogQueryError(ctx, "CreateFlag", "flags", err)
		return nil, fmt.Errorf("create flag: %w", err)
	}
	return &created, nil
}
