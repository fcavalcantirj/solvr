// Package db provides database access for Solvr.
package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/jackc/pgx/v5"
)

// Comment-related errors.
var ErrCommentNotFound = errors.New("comment not found")

// CommentsRepository handles database operations for comments.
type CommentsRepository struct {
	pool *Pool
}

// NewCommentsRepository creates a new CommentsRepository.
func NewCommentsRepository(pool *Pool) *CommentsRepository {
	return &CommentsRepository{pool: pool}
}

// Create inserts a new comment into the database.
func (r *CommentsRepository) Create(ctx context.Context, comment *models.Comment) (*models.Comment, error) {
	query := `
		INSERT INTO comments (target_type, target_id, author_type, author_id, content)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, target_type, target_id, author_type, author_id, content, created_at, deleted_at
	`

	var created models.Comment
	err := r.pool.QueryRow(ctx, query,
		comment.TargetType,
		comment.TargetID,
		comment.AuthorType,
		comment.AuthorID,
		comment.Content,
	).Scan(
		&created.ID,
		&created.TargetType,
		&created.TargetID,
		&created.AuthorType,
		&created.AuthorID,
		&created.Content,
		&created.CreatedAt,
		&created.DeletedAt,
	)
	if err != nil {
		LogQueryError(ctx, "Create", "comments", err)
		return nil, err
	}

	return &created, nil
}

// FindByID returns a single comment by ID with author info.
// Returns ErrCommentNotFound if the comment doesn't exist or is soft-deleted.
func (r *CommentsRepository) FindByID(ctx context.Context, id string) (*models.CommentWithAuthor, error) {
	query := `
		SELECT
			c.id, c.target_type, c.target_id, c.author_type, c.author_id, c.content, c.created_at, c.deleted_at,
			COALESCE(
				CASE c.author_type
					WHEN 'human' THEN u.display_name
					WHEN 'agent' THEN a.display_name
				END,
				'Unknown'
			) as author_display_name,
			CASE c.author_type
				WHEN 'human' THEN u.avatar_url
				WHEN 'agent' THEN a.avatar_url
			END as author_avatar_url
		FROM comments c
		LEFT JOIN users u ON c.author_type = 'human' AND c.author_id = u.id::text
		LEFT JOIN agents a ON c.author_type = 'agent' AND c.author_id = a.id
		WHERE c.id = $1 AND c.deleted_at IS NULL
	`

	var cwa models.CommentWithAuthor
	var avatarURL *string
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&cwa.ID,
		&cwa.TargetType,
		&cwa.TargetID,
		&cwa.AuthorType,
		&cwa.AuthorID,
		&cwa.Content,
		&cwa.CreatedAt,
		&cwa.DeletedAt,
		&cwa.Author.DisplayName,
		&avatarURL,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) || isInvalidUUIDError(err) {
			return nil, ErrCommentNotFound
		}
		LogQueryError(ctx, "FindByID", "comments", err)
		return nil, err
	}

	cwa.Author.ID = cwa.AuthorID
	cwa.Author.Type = cwa.AuthorType
	cwa.Author.AvatarURL = avatarURL

	return &cwa, nil
}

// List returns comments for a target with author info, paginated.
func (r *CommentsRepository) List(ctx context.Context, opts models.CommentListOptions) ([]models.CommentWithAuthor, int, error) {
	if opts.Page < 1 {
		opts.Page = 1
	}
	if opts.PerPage < 1 {
		opts.PerPage = 20
	}
	if opts.PerPage > 50 {
		opts.PerPage = 50
	}
	offset := (opts.Page - 1) * opts.PerPage

	// BART-151: comments inherit the visibility of the post their target belongs to, so the
	// total and the page both count only what the caller's family may read.
	postExpr, err := commentTargetPostExpr(opts.TargetType, "c.target_id")
	if err != nil {
		return nil, 0, err
	}
	args := []any{opts.TargetType, opts.TargetID}
	argNum := 3
	visible := visiblePostExists(postExpr, opts.CallerHuman, &args, &argNum)
	where := "c.target_type = $1 AND c.target_id = $2 AND c.deleted_at IS NULL AND " + visible

	// Count total
	var total int
	err = r.pool.QueryRow(ctx, "SELECT COUNT(*) FROM comments c WHERE "+where, args...).Scan(&total)
	if err != nil {
		LogQueryError(ctx, "List.Count", "comments", err)
		return nil, 0, err
	}

	// Get comments with author info
	query := `
		SELECT
			c.id, c.target_type, c.target_id, c.author_type, c.author_id, c.content, c.created_at, c.deleted_at,
			COALESCE(
				CASE c.author_type
					WHEN 'human' THEN u.display_name
					WHEN 'agent' THEN a.display_name
				END,
				'Unknown'
			) as author_display_name,
			CASE c.author_type
				WHEN 'human' THEN u.avatar_url
				WHEN 'agent' THEN a.avatar_url
			END as author_avatar_url
		FROM comments c
		LEFT JOIN users u ON c.author_type = 'human' AND c.author_id = u.id::text
		LEFT JOIN agents a ON c.author_type = 'agent' AND c.author_id = a.id
		WHERE ` + where + fmt.Sprintf(`
		ORDER BY c.created_at ASC
		LIMIT $%d OFFSET $%d
	`, argNum, argNum+1)

	rows, err := r.pool.Query(ctx, query, append(args, opts.PerPage, offset)...)
	if err != nil {
		LogQueryError(ctx, "List.Query", "comments", err)
		return nil, 0, err
	}
	defer rows.Close()

	var comments []models.CommentWithAuthor
	for rows.Next() {
		var cwa models.CommentWithAuthor
		var avatarURL *string
		err := rows.Scan(
			&cwa.ID,
			&cwa.TargetType,
			&cwa.TargetID,
			&cwa.AuthorType,
			&cwa.AuthorID,
			&cwa.Content,
			&cwa.CreatedAt,
			&cwa.DeletedAt,
			&cwa.Author.DisplayName,
			&avatarURL,
		)
		if err != nil {
			LogQueryError(ctx, "List.Scan", "comments", err)
			return nil, 0, fmt.Errorf("scan failed: %w", err)
		}

		cwa.Author.ID = cwa.AuthorID
		cwa.Author.Type = cwa.AuthorType
		cwa.Author.AvatarURL = avatarURL
		comments = append(comments, cwa)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	if comments == nil {
		comments = []models.CommentWithAuthor{}
	}

	return comments, total, nil
}

// Delete soft-deletes a comment by setting deleted_at.
func (r *CommentsRepository) Delete(ctx context.Context, id string) error {
	query := `
		UPDATE comments SET deleted_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL
	`

	result, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		LogQueryError(ctx, "Delete", "comments", err)
		return err
	}

	if result.RowsAffected() == 0 {
		return ErrCommentNotFound
	}

	return nil
}

// commentTargetPostExpr returns the SQL expression for the id of the post a comment target
// belongs to, where ref is trusted SQL naming the target id: a post is its own post, an
// approach, answer or response belongs to its problem, question or idea. A deleted
// approach or answer belongs to no post.
func commentTargetPostExpr(targetType models.CommentTargetType, ref string) (string, error) {
	switch targetType {
	case models.CommentTargetPost:
		return ref, nil
	case models.CommentTargetApproach:
		return "(SELECT problem_id FROM approaches WHERE id = " + ref + " AND deleted_at IS NULL)", nil
	case models.CommentTargetAnswer:
		return "(SELECT question_id FROM answers WHERE id = " + ref + " AND deleted_at IS NULL)", nil
	case models.CommentTargetResponse:
		return "(SELECT idea_id FROM responses WHERE id = " + ref + ")", nil
	default:
		return "", fmt.Errorf("unknown target type: %s", targetType)
	}
}

// TargetVisibleTo reports whether callerHuman ("" = public only) may read the comment
// target: it exists, is not deleted, and the post it belongs to passes the rule GET
// /v1/posts/{id} applies. An id that names nothing (absent or malformed) is false.
func (r *CommentsRepository) TargetVisibleTo(ctx context.Context, targetType models.CommentTargetType, targetID, callerHuman string) (bool, error) {
	postExpr, err := commentTargetPostExpr(targetType, "$1")
	if err != nil {
		return false, err
	}
	args := []any{targetID}
	argNum := 2
	var visible bool
	err = r.pool.QueryRow(ctx, "SELECT "+visiblePostExists(postExpr, callerHuman, &args, &argNum), args...).Scan(&visible)
	if err != nil {
		if isInvalidUUIDError(err) {
			return false, nil
		}
		LogQueryError(ctx, "TargetVisibleTo", "comments", err)
		return false, err
	}
	return visible, nil
}
