// Package db provides database access for Solvr.
package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// Reply-related errors.
var (
	// ErrReplyPostNotFound is returned when the target post does not exist.
	ErrReplyPostNotFound = errors.New("post not found")
	// ErrReplyForbidden is returned when a caller is not the reply's author.
	ErrReplyForbidden = errors.New("not the reply author")
	// ErrParentReplyInvalid is returned when parent_reply_id does not belong to
	// the same post (cross-post threading is rejected).
	ErrParentReplyInvalid = errors.New("parent reply is invalid for this post")
)

// ReplyRepository handles database operations for the canonical Reply model
// (BART-585). One create/list/update/delete family serves every contribution;
// there is no approach/answer/response/comment branching.
type ReplyRepository struct {
	pool *Pool
}

// NewReplyRepository creates a new ReplyRepository.
func NewReplyRepository(pool *Pool) *ReplyRepository {
	return &ReplyRepository{pool: pool}
}

// replyAuthorSelect is the shared author-resolution projection.
const replyAuthorSelect = `
	COALESCE(
		CASE WHEN rp.author_type = 'agent' THEN a.display_name
		     WHEN rp.author_type = 'human' THEN u.display_name
		     ELSE rp.author_id
		END,
		rp.author_id
	) AS display_name,
	CASE WHEN rp.author_type = 'human' THEN u.avatar_url ELSE NULL END AS avatar_url`

const replyAuthorJoins = `
	LEFT JOIN agents a ON rp.author_type = 'agent' AND rp.author_id = a.id
	LEFT JOIN users u ON rp.author_type = 'human' AND rp.author_id = u.id::text`

// Create inserts a new canonical reply. The post must exist and not be deleted;
// a parent reply, when supplied, must belong to the same post.
func (r *ReplyRepository) Create(ctx context.Context, reply *models.Reply) (*models.Reply, error) {
	var postExists bool
	err := r.pool.QueryRow(ctx,
		"SELECT EXISTS(SELECT 1 FROM posts WHERE id = $1 AND deleted_at IS NULL)",
		reply.PostID,
	).Scan(&postExists)
	if err != nil {
		if isInvalidUUIDError(err) {
			return nil, ErrReplyPostNotFound
		}
		LogQueryError(ctx, "Reply.Create.CheckPost", "posts", err)
		return nil, fmt.Errorf("check post existence: %w", err)
	}
	if !postExists {
		return nil, ErrReplyPostNotFound
	}

	if reply.ParentReplyID != nil {
		var parentPost string
		err = r.pool.QueryRow(ctx,
			"SELECT post_id FROM replies WHERE id = $1 AND deleted_at IS NULL",
			*reply.ParentReplyID,
		).Scan(&parentPost)
		if err != nil {
			if isInvalidUUIDError(err) || err.Error() == "no rows in result set" {
				return nil, ErrParentReplyInvalid
			}
			LogQueryError(ctx, "Reply.Create.CheckParent", "replies", err)
			return nil, fmt.Errorf("check parent reply: %w", err)
		}
		if parentPost != reply.PostID {
			return nil, ErrParentReplyInvalid
		}
	}

	var provenance any
	if len(reply.Provenance) > 0 {
		provenance = []byte(reply.Provenance)
	}

	row := r.pool.QueryRow(ctx, `
		INSERT INTO replies (post_id, parent_reply_id, author_type, author_id, body, legacy_type, legacy_id, provenance, embedding)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9::vector)
		RETURNING id, post_id, parent_reply_id, author_type, author_id, body, upvotes, downvotes,
		          legacy_type, legacy_id, provenance, created_at, updated_at, deleted_at`,
		reply.PostID, reply.ParentReplyID, reply.AuthorType, reply.AuthorID, reply.Body,
		reply.LegacyType, reply.LegacyID, provenance, reply.EmbeddingStr,
	)
	created, err := scanReply(row)
	if err != nil {
		LogQueryError(ctx, "Reply.Create.Insert", "replies", err)
		return nil, fmt.Errorf("insert reply: %w", err)
	}
	return created, nil
}

// GetByID returns a single non-deleted reply with its author information.
func (r *ReplyRepository) GetByID(ctx context.Context, id string) (*models.ReplyWithAuthor, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT rp.id, rp.post_id, rp.parent_reply_id, rp.author_type, rp.author_id, rp.body,
		       rp.upvotes, rp.downvotes, rp.legacy_type, rp.legacy_id, rp.provenance,
		       rp.created_at, rp.updated_at, rp.deleted_at,`+replyAuthorSelect+`
		FROM replies rp`+replyAuthorJoins+`
		WHERE rp.id = $1 AND rp.deleted_at IS NULL`, id)
	rwa, err := scanReplyWithAuthor(row)
	if err != nil {
		if isInvalidUUIDError(err) || err.Error() == "no rows in result set" {
			return nil, models.ErrReplyNotFound
		}
		LogQueryError(ctx, "Reply.GetByID", "replies", err)
		return nil, fmt.Errorf("get reply: %w", err)
	}
	return rwa, nil
}

// ListByPost returns non-deleted replies for a post, oldest first, paginated.
func (r *ReplyRepository) ListByPost(ctx context.Context, opts models.ReplyListOptions) ([]models.ReplyWithAuthor, int, error) {
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

	var total int
	err := r.pool.QueryRow(ctx,
		"SELECT COUNT(*) FROM replies WHERE post_id = $1 AND deleted_at IS NULL",
		opts.PostID,
	).Scan(&total)
	if err != nil {
		if isTableNotFoundError(err) {
			return []models.ReplyWithAuthor{}, 0, nil
		}
		if isInvalidUUIDError(err) {
			return []models.ReplyWithAuthor{}, 0, nil
		}
		LogQueryError(ctx, "Reply.ListByPost.Count", "replies", err)
		return nil, 0, fmt.Errorf("count replies: %w", err)
	}

	rows, err := r.pool.Query(ctx, `
		SELECT rp.id, rp.post_id, rp.parent_reply_id, rp.author_type, rp.author_id, rp.body,
		       rp.upvotes, rp.downvotes, rp.legacy_type, rp.legacy_id, rp.provenance,
		       rp.created_at, rp.updated_at, rp.deleted_at,`+replyAuthorSelect+`
		FROM replies rp`+replyAuthorJoins+`
		WHERE rp.post_id = $1 AND rp.deleted_at IS NULL
		ORDER BY rp.created_at ASC, rp.id ASC
		LIMIT $2 OFFSET $3`, opts.PostID, perPage, offset)
	if err != nil {
		LogQueryError(ctx, "Reply.ListByPost.Query", "replies", err)
		return nil, 0, fmt.Errorf("list replies: %w", err)
	}
	defer rows.Close()

	replies := make([]models.ReplyWithAuthor, 0, perPage)
	for rows.Next() {
		rwa, scanErr := scanReplyWithAuthor(rows)
		if scanErr != nil {
			LogQueryError(ctx, "Reply.ListByPost.Scan", "replies", scanErr)
			return nil, 0, fmt.Errorf("scan reply: %w", scanErr)
		}
		replies = append(replies, *rwa)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate replies: %w", err)
	}
	return replies, total, nil
}

// maxReplyPageFetch bounds a single keyset page defensively; the handler clamps
// user-supplied limits to the API-wide maximum (100) before the +1 look-ahead.
const maxReplyPageFetch = 201

// ListPageByPost returns one opaque forward page of a post's replies plus the
// post-wide non-deleted total (idx 73 step 2). Rows are ordered oldest first by
// the keyset (created_at, id); when params.AfterCreatedAt is set only replies
// strictly after that keyset position are returned, so paging forward under
// concurrent writes never duplicates or skips a committed reply. The caller may
// fetch Limit+1 to detect whether more pages follow.
func (r *ReplyRepository) ListPageByPost(ctx context.Context, params models.ReplyPageParams) ([]models.ReplyWithAuthor, int, error) {
	limit := params.Limit
	if limit < 1 {
		limit = 1
	}
	if limit > maxReplyPageFetch {
		limit = maxReplyPageFetch
	}

	var total int
	err := r.pool.QueryRow(ctx,
		"SELECT COUNT(*) FROM replies WHERE post_id = $1 AND deleted_at IS NULL",
		params.PostID,
	).Scan(&total)
	if err != nil {
		if isTableNotFoundError(err) || isInvalidUUIDError(err) {
			return []models.ReplyWithAuthor{}, 0, nil
		}
		LogQueryError(ctx, "Reply.ListPageByPost.Count", "replies", err)
		return nil, 0, fmt.Errorf("count replies: %w", err)
	}

	// A single static query: $2 is the "has cursor" flag; when false the keyset
	// predicate short-circuits and every reply is eligible, so the sentinel time
	// and id in $3/$4 are never evaluated.
	hasCursor := params.AfterCreatedAt != nil
	afterTime := time.Unix(0, 0)
	afterID := "00000000-0000-0000-0000-000000000000"
	if hasCursor {
		afterTime = *params.AfterCreatedAt
		if params.AfterID != "" {
			afterID = params.AfterID
		}
	}

	rows, err := r.pool.Query(ctx, `
		SELECT rp.id, rp.post_id, rp.parent_reply_id, rp.author_type, rp.author_id, rp.body,
		       rp.upvotes, rp.downvotes, rp.legacy_type, rp.legacy_id, rp.provenance,
		       rp.created_at, rp.updated_at, rp.deleted_at,`+replyAuthorSelect+`
		FROM replies rp`+replyAuthorJoins+`
		WHERE rp.post_id = $1 AND rp.deleted_at IS NULL
		  AND ($2::boolean = false
		       OR rp.created_at > $3
		       OR (rp.created_at = $3 AND rp.id > $4::uuid))
		ORDER BY rp.created_at ASC, rp.id ASC
		LIMIT $5`, params.PostID, hasCursor, afterTime, afterID, limit)
	if err != nil {
		LogQueryError(ctx, "Reply.ListPageByPost.Query", "replies", err)
		return nil, 0, fmt.Errorf("list reply page: %w", err)
	}
	defer rows.Close()

	replies := make([]models.ReplyWithAuthor, 0, limit)
	for rows.Next() {
		rwa, scanErr := scanReplyWithAuthor(rows)
		if scanErr != nil {
			LogQueryError(ctx, "Reply.ListPageByPost.Scan", "replies", scanErr)
			return nil, 0, fmt.Errorf("scan reply: %w", scanErr)
		}
		replies = append(replies, *rwa)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate reply page: %w", err)
	}
	return replies, total, nil
}

// CountByPost returns the number of non-deleted replies for a post.
func (r *ReplyRepository) CountByPost(ctx context.Context, postID string) (int, error) {
	var total int
	err := r.pool.QueryRow(ctx,
		"SELECT COUNT(*) FROM replies WHERE post_id = $1 AND deleted_at IS NULL",
		postID,
	).Scan(&total)
	if err != nil {
		if isTableNotFoundError(err) || isInvalidUUIDError(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("count replies: %w", err)
	}
	return total, nil
}

// Update edits a reply's body. Only the author may edit; author identity,
// creation time, votes, and provenance are never reset. embedding is the vector
// literal of the new body; nil clears the stored vector so a stale one never
// describes edited text (the backfill re-embeds it). expected is the version
// (updated_at) the caller read: the write lands only while the reply is still at
// it, in the same statement, so two edits that read one version cannot both land
// (spec.json idx 74 step 5); it returns a *models.VersionConflictError with the
// current version otherwise. A nil expected writes unconditionally.
func (r *ReplyRepository) Update(ctx context.Context, id string, authorType models.AuthorType, authorID, body string, embedding *string, expected *time.Time) (*models.Reply, error) {
	owner, err := r.loadOwner(ctx, id)
	if err != nil {
		return nil, err
	}
	if owner.AuthorType != authorType || owner.AuthorID != authorID {
		return nil, ErrReplyForbidden
	}

	row := r.pool.QueryRow(ctx, `
		UPDATE replies SET body = $2, embedding = $3::vector, updated_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL
		  AND ($4::timestamptz IS NULL OR updated_at = $4::timestamptz)
		RETURNING id, post_id, parent_reply_id, author_type, author_id, body, upvotes, downvotes,
		          legacy_type, legacy_id, provenance, created_at, updated_at, deleted_at`, id, body, embedding, expected)
	updated, err := scanReply(row)
	if err != nil {
		if err.Error() == "no rows in result set" {
			if expected != nil {
				return nil, r.versionConflictOrNotFound(ctx, id)
			}
			return nil, models.ErrReplyNotFound
		}
		LogQueryError(ctx, "Reply.Update", "replies", err)
		return nil, fmt.Errorf("update reply: %w", err)
	}
	return updated, nil
}

// versionConflictOrNotFound explains a conditional reply write that matched no
// row: a live reply that moved is a version conflict, anything else not found.
func (r *ReplyRepository) versionConflictOrNotFound(ctx context.Context, id string) error {
	var current time.Time
	err := r.pool.QueryRow(ctx,
		"SELECT updated_at FROM replies WHERE id = $1 AND deleted_at IS NULL", id).Scan(&current)
	if err == nil {
		return &models.VersionConflictError{Current: current}
	}
	if err.Error() == "no rows in result set" {
		return models.ErrReplyNotFound
	}
	LogQueryError(ctx, "Reply.Update.version", "replies", err)
	return fmt.Errorf("read reply version: %w", err)
}

// Delete soft-deletes a reply. Only the author may delete it.
func (r *ReplyRepository) Delete(ctx context.Context, id string, authorType models.AuthorType, authorID string) error {
	owner, err := r.loadOwner(ctx, id)
	if err != nil {
		return err
	}
	if owner.AuthorType != authorType || owner.AuthorID != authorID {
		return ErrReplyForbidden
	}
	_, err = r.pool.Exec(ctx,
		"UPDATE replies SET deleted_at = NOW() WHERE id = $1 AND deleted_at IS NULL", id)
	if err != nil {
		LogQueryError(ctx, "Reply.Delete", "replies", err)
		return fmt.Errorf("delete reply: %w", err)
	}
	return nil
}

// loadOwner returns the author identity of a live reply for permission checks.
func (r *ReplyRepository) loadOwner(ctx context.Context, id string) (*models.Reply, error) {
	var reply models.Reply
	err := r.pool.QueryRow(ctx,
		"SELECT author_type, author_id FROM replies WHERE id = $1 AND deleted_at IS NULL", id,
	).Scan(&reply.AuthorType, &reply.AuthorID)
	if err != nil {
		if isInvalidUUIDError(err) || err.Error() == "no rows in result set" {
			return nil, models.ErrReplyNotFound
		}
		return nil, fmt.Errorf("load reply owner: %w", err)
	}
	return &reply, nil
}

// Vote records or updates a confirmed vote on a reply. Votes target the canonical
// Reply identity (target_type = 'reply'), the same polymorphic votes table posts use.
// The reply's upvote/downvote counters follow the vote row inside the same statement
// (migration 000113 triggers), so concurrent or replayed requests count once.
func (r *ReplyRepository) Vote(ctx context.Context, replyID, voterType, voterID, direction string) error {
	if direction != "up" && direction != "down" {
		return ErrInvalidVoteDirection
	}
	if voterType != "human" && voterType != "agent" {
		return ErrInvalidVoterType
	}

	var exists bool
	err := r.pool.QueryRow(ctx,
		"SELECT EXISTS(SELECT 1 FROM replies WHERE id = $1 AND deleted_at IS NULL)", replyID,
	).Scan(&exists)
	if err != nil {
		if isInvalidUUIDError(err) {
			return models.ErrReplyNotFound
		}
		LogQueryError(ctx, "Reply.Vote.CheckExists", "replies", err)
		return fmt.Errorf("check reply existence: %w", err)
	}
	if !exists {
		return models.ErrReplyNotFound
	}

	if _, err := r.pool.Exec(ctx,
		`INSERT INTO votes (target_type, target_id, voter_type, voter_id, direction, confirmed)
		 VALUES ('reply', $1, $2, $3, $4, true)
		 ON CONFLICT ON CONSTRAINT votes_unique_per_target
		 DO UPDATE SET direction = EXCLUDED.direction
		 WHERE votes.direction IS DISTINCT FROM EXCLUDED.direction`,
		replyID, voterType, voterID, direction,
	); err != nil {
		LogQueryError(ctx, "Reply.Vote.Upsert", "votes", err)
		return fmt.Errorf("record vote: %w", err)
	}
	return nil
}

// GetUserVote returns the caller's current vote direction on a reply, or nil.
func (r *ReplyRepository) GetUserVote(ctx context.Context, replyID, voterType, voterID string) (*string, error) {
	var direction string
	err := r.pool.QueryRow(ctx,
		`SELECT direction FROM votes
		 WHERE target_type = 'reply' AND target_id = $1 AND voter_type = $2 AND voter_id = $3`,
		replyID, voterType, voterID,
	).Scan(&direction)
	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, nil
		}
		if isInvalidUUIDError(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("get user vote: %w", err)
	}
	return &direction, nil
}

// scanReply scans the canonical reply column projection (no author join).
// rowScanner (defined in room_messages.go) is satisfied by pgx.Row and pgx.Rows.
func scanReply(row rowScanner) (*models.Reply, error) {
	var reply models.Reply
	var provenance []byte
	if err := row.Scan(
		&reply.ID, &reply.PostID, &reply.ParentReplyID, &reply.AuthorType, &reply.AuthorID,
		&reply.Body, &reply.Upvotes, &reply.Downvotes, &reply.LegacyType, &reply.LegacyID,
		&provenance, &reply.CreatedAt, &reply.UpdatedAt, &reply.DeletedAt,
	); err != nil {
		return nil, err
	}
	if len(provenance) > 0 {
		reply.Provenance = provenance
	}
	reply.ComputeScore()
	return &reply, nil
}

// scanReplyWithAuthor scans the reply projection plus resolved author columns, then any extra
// columns the query selects after them into extra.
func scanReplyWithAuthor(row rowScanner, extra ...any) (*models.ReplyWithAuthor, error) {
	var rwa models.ReplyWithAuthor
	var provenance []byte
	var displayName string
	var avatarURL *string
	if err := row.Scan(append([]any{
		&rwa.ID, &rwa.PostID, &rwa.ParentReplyID, &rwa.AuthorType, &rwa.AuthorID,
		&rwa.Body, &rwa.Upvotes, &rwa.Downvotes, &rwa.LegacyType, &rwa.LegacyID,
		&provenance, &rwa.CreatedAt, &rwa.UpdatedAt, &rwa.DeletedAt,
		&displayName, &avatarURL,
	}, extra...)...); err != nil {
		return nil, err
	}
	if len(provenance) > 0 {
		rwa.Provenance = provenance
	}
	rwa.ComputeScore()
	rwa.Author = models.ReplyAuthor{
		ID:          rwa.AuthorID,
		Type:        rwa.AuthorType,
		DisplayName: displayName,
		AvatarURL:   avatarURL,
	}
	return &rwa, nil
}
