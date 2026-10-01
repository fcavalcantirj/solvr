package db

import (
	"context"
	"fmt"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// ListPageByAuthor returns one page of an author's replies across posts, newest first, plus
// the total the viewer may read (GET /v1/replies, task idx 73 step 3; it replaces the
// contribution listings). A reply is listed when it is not deleted and its post passes the
// GET /v1/posts/{id} read rule for params.ViewerHuman: the post is not deleted and is public or
// owned by the viewer's family. Rows are ordered by the keyset (created_at, id) descending;
// when params.BeforeCreatedAt is set only replies strictly before that position are returned,
// so paging never repeats a reply. The caller may fetch Limit+1 to detect a following page.
func (r *ReplyRepository) ListPageByAuthor(ctx context.Context, params models.ReplyAuthorPageParams) ([]models.ReplyWithPost, int, error) {
	limit := min(max(params.Limit, 1), maxReplyPageFetch)

	args := []any{string(params.AuthorType), params.AuthorID}
	argNum := 3
	where := `rp.author_type = $1 AND rp.author_id = $2 AND rp.deleted_at IS NULL
		AND p.deleted_at IS NULL AND ` + searchVisibilityClause("p", params.ViewerHuman, &args, &argNum)

	var total int
	err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM replies rp JOIN posts p ON p.id = rp.post_id WHERE `+where,
		args...).Scan(&total)
	if err != nil {
		LogQueryError(ctx, "Reply.ListPageByAuthor.Count", "replies", err)
		return nil, 0, fmt.Errorf("count replies by author: %w", err)
	}

	// The keyset predicate short-circuits on the "has cursor" flag, so the sentinel time and
	// id are never compared on a first page.
	hasCursor := params.BeforeCreatedAt != nil
	beforeTime := time.Unix(0, 0)
	beforeID := "00000000-0000-0000-0000-000000000000"
	if hasCursor {
		beforeTime = *params.BeforeCreatedAt
		if params.BeforeID != "" {
			beforeID = params.BeforeID
		}
	}
	pageArgs := append(append([]any{}, args...), hasCursor, beforeTime, beforeID, limit)
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT rp.id, rp.post_id, rp.parent_reply_id, rp.author_type, rp.author_id, rp.body,
		       rp.upvotes, rp.downvotes, rp.legacy_type, rp.legacy_id, rp.provenance,
		       rp.created_at, rp.updated_at, rp.deleted_at,`+replyAuthorSelect+`,
		       p.type, COALESCE(p.title, '')
		FROM replies rp
		JOIN posts p ON p.id = rp.post_id`+replyAuthorJoins+`
		WHERE `+where+`
		  AND ($%d::boolean = false
		       OR rp.created_at < $%d
		       OR (rp.created_at = $%d AND rp.id < $%d::uuid))
		ORDER BY rp.created_at DESC, rp.id DESC
		LIMIT $%d`, argNum, argNum+1, argNum+1, argNum+2, argNum+3), pageArgs...)
	if err != nil {
		if isInvalidUUIDError(err) {
			return []models.ReplyWithPost{}, total, nil // a cursor naming no reply id: nothing follows it
		}
		LogQueryError(ctx, "Reply.ListPageByAuthor.Query", "replies", err)
		return nil, 0, fmt.Errorf("list replies by author: %w", err)
	}
	defer rows.Close()

	replies := make([]models.ReplyWithPost, 0, limit)
	for rows.Next() {
		var post models.ReplyPost
		rwa, scanErr := scanReplyWithAuthor(rows, &post.Type, &post.Title)
		if scanErr != nil {
			LogQueryError(ctx, "Reply.ListPageByAuthor.Scan", "replies", scanErr)
			return nil, 0, fmt.Errorf("scan reply: %w", scanErr)
		}
		post.ID = rwa.PostID
		replies = append(replies, models.ReplyWithPost{ReplyWithAuthor: *rwa, Post: post})
	}
	if err := rows.Err(); err != nil {
		if isInvalidUUIDError(err) {
			return []models.ReplyWithPost{}, total, nil
		}
		return nil, 0, fmt.Errorf("iterate replies by author: %w", err)
	}
	return replies, total, nil
}
