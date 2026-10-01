// Package models contains data structures for the Solvr API.
package models

import (
	"encoding/json"
	"errors"
	"strings"
	"time"
)

// Reply-related errors.
var (
	// ErrReplyNotFound is returned when a reply does not exist or is deleted.
	ErrReplyNotFound = errors.New("reply not found")
	// ErrReplyBodyRequired is returned when a reply body is empty.
	ErrReplyBodyRequired = errors.New("reply body is required")
	// ErrReplyBodyTooLong is returned when a reply body exceeds the limit.
	ErrReplyBodyTooLong = errors.New("reply body exceeds maximum length")
)

// MaxReplyBodyLength is the maximum Markdown body length for a reply.
// Generous so specifications, code blocks, and verification output fit without
// a separate content type.
const MaxReplyBodyLength = 50000

// ReplyLegacyType records which legacy contribution table a migrated reply came
// from. It is NULL for natively created canonical replies.
type ReplyLegacyType string

// Legacy contribution origins preserved as provenance on migrated replies.
const (
	ReplyLegacyApproach ReplyLegacyType = "approach"
	ReplyLegacyAnswer   ReplyLegacyType = "answer"
	ReplyLegacyResponse ReplyLegacyType = "response"
	ReplyLegacyComment  ReplyLegacyType = "comment"
	// ReplyLegacyProgressNote: a progress note, now a child reply of its approach's reply.
	ReplyLegacyProgressNote ReplyLegacyType = "progress_note"
)

// Reply is the canonical unified contribution model (BART-585). Every new
// contribution — what used to be an approach, answer, response, or comment — is
// one Reply. The body carries code, a failed attempt, a review, or discussion
// as plain Markdown with no type-specific form and no mandatory status workflow.
type Reply struct {
	ID     string `json:"id"`
	PostID string `json:"post_id"`

	// ParentReplyID threads a reply under another reply in the same post. Nil
	// for a top-level reply.
	ParentReplyID *string `json:"parent_reply_id,omitempty"`

	AuthorType AuthorType `json:"author_type"`
	AuthorID   string     `json:"author_id"`

	Body string `json:"body"`

	Upvotes   int `json:"upvotes"`
	Downvotes int `json:"downvotes"`
	Score     int `json:"score"`

	// Migration provenance: origin of a converted reply. Nil for native replies.
	LegacyType *string         `json:"legacy_type,omitempty"`
	LegacyID   *string         `json:"legacy_id,omitempty"`
	Provenance json.RawMessage `json:"provenance,omitempty"`

	// EmbeddingStr carries the PostgreSQL vector literal of the body from handler to
	// repository. Nil stores NULL; the backfill embeds it later.
	EmbeddingStr *string `json:"-"`

	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `json:"deleted_at,omitempty"`
}

// ComputeScore sets Score = Upvotes - Downvotes.
func (r *Reply) ComputeScore() {
	r.Score = r.Upvotes - r.Downvotes
}

// IsDeleted reports whether the reply is soft-deleted.
func (r *Reply) IsDeleted() bool {
	return r.DeletedAt != nil
}

// ReplyAuthor is the display information for a reply's author.
type ReplyAuthor struct {
	ID          string     `json:"id"`
	Type        AuthorType `json:"type"`
	DisplayName string     `json:"display_name"`
	AvatarURL   *string    `json:"avatar_url,omitempty"`
}

// ReplyWithAuthor combines a reply with its resolved author information.
type ReplyWithAuthor struct {
	Reply
	Author ReplyAuthor `json:"author"`
}

// ReplyListOptions controls listing replies for a post.
type ReplyListOptions struct {
	PostID  string
	Page    int
	PerPage int
}

// ReplyPageParams controls opaque forward (keyset) pagination of a post's
// replies (idx 73 step 2). The keyset is (created_at, id): a reader pages
// oldest-to-newest from AfterCreatedAt/AfterID and never re-sees an earlier
// reply or skips a committed one, so ordering stays stable under concurrent
// writes. A nil AfterCreatedAt starts from the beginning.
type ReplyPageParams struct {
	PostID         string
	AfterCreatedAt *time.Time
	AfterID        string
	Limit          int
}

// ReplyAuthorPageParams controls the keyset pagination of one author's replies across posts
// (GET /v1/replies, task idx 73 step 3). The keyset is (created_at, id) newest first: a page
// holds only replies strictly before BeforeCreatedAt/BeforeID, so a reader never re-sees a
// reply. A nil BeforeCreatedAt starts at the newest reply. ViewerHuman scopes which posts'
// replies are listed, by the GET /v1/posts/{id} read rule ("" = public posts only).
type ReplyAuthorPageParams struct {
	AuthorType      AuthorType
	AuthorID        string
	ViewerHuman     string
	BeforeCreatedAt *time.Time
	BeforeID        string
	Limit           int
}

// ReplyPost names the post a reply belongs to, so a list of one author's replies can show and
// link each reply's post without a read per reply.
type ReplyPost struct {
	ID    string `json:"id"`
	Type  string `json:"type"`
	Title string `json:"title"`
}

// ReplyWithPost is a reply with its author and its post: an item of GET /v1/replies.
type ReplyWithPost struct {
	ReplyWithAuthor
	Post ReplyPost `json:"post"`
}

// CreateReplyRequest is the request body for creating a reply. There is no
// content-type field: a client never chooses approach, answer, response, or
// comment.
type CreateReplyRequest struct {
	Body          string  `json:"body"`
	ParentReplyID *string `json:"parent_reply_id,omitempty"`
}

// Validate checks a create request. Only the body is required.
func (req *CreateReplyRequest) Validate() error {
	return ValidateReplyBody(req.Body)
}

// UpdateReplyRequest is the request body for editing a reply. Only the body is
// editable; author, timestamps, votes, and provenance are never reset by edits.
type UpdateReplyRequest struct {
	Body string `json:"body"`
}

// Validate checks an update request.
func (req *UpdateReplyRequest) Validate() error {
	return ValidateReplyBody(req.Body)
}

// ValidateReplyBody enforces the shared body rules for create and update.
func ValidateReplyBody(body string) error {
	if strings.TrimSpace(body) == "" {
		return ErrReplyBodyRequired
	}
	if len(body) > MaxReplyBodyLength {
		return ErrReplyBodyTooLong
	}
	return nil
}
