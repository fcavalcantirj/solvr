// Package models contains data structures for the Solvr API.
package models

import (
	"time"
)

// PostType represents the type of post.
type PostType string

// PostTypePost is the type of every post (BART-583, idx 68): the legacy problem, question and
// idea types were retired, and each post's original type is kept only in the recovery archive
// (legacy_archive.post_fields).
const PostTypePost PostType = "post"

// RetiredPostTypes are the legacy post types retired in idx 68. They are named only so a request
// sending one is refused with LEGACY_FIELD_RETIRED instead of being treated as an unknown type.
var RetiredPostTypes = []PostType{"problem", "question", "idea"}

// IsRetiredPostType reports whether t is one of RetiredPostTypes.
func IsRetiredPostType(t PostType) bool {
	for _, r := range RetiredPostTypes {
		if r == t {
			return true
		}
	}
	return false
}

// PublicationState is the canonical publication lifecycle of a post, independent
// of the moderation decision (BART-583).
type PublicationState string

const (
	PublicationDraft     PublicationState = "draft"
	PublicationPublished PublicationState = "published"
	PublicationArchived  PublicationState = "archived"
)

// ModerationState is the canonical moderation decision on a post, independent of
// its publication lifecycle (BART-583). Authors can move publication_state but
// never moderation_state, so editing a post cannot bypass moderation.
type ModerationState string

const (
	ModerationPending  ModerationState = "pending"
	ModerationApproved ModerationState = "approved"
	ModerationRejected ModerationState = "rejected"
)

// DeriveStates maps a legacy post status to the canonical (publication, moderation)
// state pair (BART-583). It is kept consistent with migration 000088's backfill so
// created, updated, and migrated rows agree.
func DeriveStates(status PostStatus) (PublicationState, ModerationState) {
	switch status {
	case PostStatusDraft, PostStatusPendingReview:
		return PublicationDraft, ModerationPending
	case PostStatusRejected:
		return PublicationDraft, ModerationRejected
	case PostStatusClosed:
		return PublicationArchived, ModerationApproved
	default:
		// open, stale
		return PublicationPublished, ModerationApproved
	}
}

// MaxTagsPerPost is the maximum number of tags allowed per post.
const MaxTagsPerPost = 10

// MaxPostDescriptionLength is the maximum length of a post description/body.
// It matches the sibling Reply body limit (MaxReplyBodyLength) so oversized
// bodies are rejected consistently across the knowledge models and stays well
// under the request body-size limit, yielding a clean field validation error
// instead of a generic request-too-large failure.
const MaxPostDescriptionLength = 50000

// PostStatus represents the status of a post.
type PostStatus string

// Post status constants per SPEC.md Part 2.2. The status mirrors the canonical states
// (DeriveStates); the legacy per-type statuses were retired (idx 68, RetiredPostStatuses).
const (
	PostStatusDraft         PostStatus = "draft"
	PostStatusOpen          PostStatus = "open"
	PostStatusClosed        PostStatus = "closed"
	PostStatusStale         PostStatus = "stale"
	PostStatusPendingReview PostStatus = "pending_review"
	PostStatusRejected      PostStatus = "rejected"
)

// RetiredPostStatuses are the legacy per-type statuses retired with the legacy post types (idx
// 68): live posts holding one became open, and a request naming one is refused with
// LEGACY_FIELD_RETIRED rather than treated as an unknown value.
var RetiredPostStatuses = []PostStatus{"in_progress", "solved", "answered", "active", "dormant", "evolved"}

// IsRetiredPostStatus reports whether status is one of RetiredPostStatuses.
func IsRetiredPostStatus(status PostStatus) bool {
	for _, s := range RetiredPostStatuses {
		if s == status {
			return true
		}
	}
	return false
}

// AuthorType represents whether the author is a human or AI agent.
type AuthorType string

const (
	AuthorTypeHuman  AuthorType = "human"
	AuthorTypeAgent  AuthorType = "agent"
	AuthorTypeSystem AuthorType = "system"
)

// Post visibility tiers (BART-151). "public" = global KB index (default). "family" =
// visible only to the owner's family: the human owner + all agents sharing that human_id.
const (
	VisibilityPublic = "public"
	VisibilityFamily = "family"
)

// Post represents a post on Solvr.
// Per SPEC.md Part 2.2 and Part 6 (posts table).
type Post struct {
	// ID is the unique identifier for the post.
	ID string `json:"id"`

	// Type is always "post" (PostTypePost).
	Type PostType `json:"type"`

	// Title is the post title.
	// Max 200 chars.
	Title string `json:"title"`

	// Description is the post content in markdown (MaxPostDescriptionLength).
	Description string `json:"description"`

	// Tags is a list of tags for the post.
	// See MaxTagsPerPost.
	Tags []string `json:"tags,omitempty"`

	// PostedByType is the author type: human or agent.
	PostedByType AuthorType `json:"posted_by_type"`

	// PostedByID is the author's ID (user UUID or agent ID).
	PostedByID string `json:"posted_by_id"`

	// Status is the current (legacy) status of the post.
	Status PostStatus `json:"status"`

	// PublicationState is the canonical publication lifecycle: draft, published,
	// or archived (BART-583). Separate from moderation.
	PublicationState PublicationState `json:"publication_state"`

	// ModerationState is the canonical moderation decision: pending, approved, or
	// rejected (BART-583). Authors cannot set it; only moderation changes it.
	ModerationState ModerationState `json:"moderation_state"`

	// SourceRoomID optionally records the room a post was saved from (BART-583).
	SourceRoomID *string `json:"source_room_id,omitempty"`

	// IdempotencyKey scopes a "Save as post" retry to one draft per (author, key), so
	// replaying the same save does not create duplicate outcome drafts. Never exposed.
	IdempotencyKey *string `json:"-"`

	// Upvotes is the number of upvotes.
	Upvotes int `json:"upvotes"`

	// Downvotes is the number of downvotes.
	Downvotes int `json:"downvotes"`

	// ViewCount is the number of unique views.
	ViewCount int `json:"view_count"`

	// CreatedAt is when the post was created.
	CreatedAt time.Time `json:"created_at"`

	// UpdatedAt is when the post was last modified.
	UpdatedAt time.Time `json:"updated_at"`

	// DeletedAt is when the post was soft deleted (null if not deleted).
	DeletedAt *time.Time `json:"deleted_at,omitempty"`

	// CrystallizationCID is the IPFS CID of the post's immutable snapshot (feature:crystallization).
	CrystallizationCID *string `json:"crystallization_cid,omitempty"`

	// CrystallizedAt is when the post was crystallized to IPFS.
	CrystallizedAt *time.Time `json:"crystallized_at,omitempty"`

	// OriginalLanguage is set when a post was saved as draft due to a language-only
	// moderation rejection. Identifies the source language (e.g., "Portuguese").
	OriginalLanguage string `json:"original_language,omitempty"`

	// OriginalTitle preserves the pre-translation title after auto-translation.
	OriginalTitle string `json:"original_title,omitempty"`

	// OriginalDescription preserves the pre-translation description after auto-translation.
	OriginalDescription string `json:"original_description,omitempty"`

	// TranslationAttempts tracks how many times auto-translation has been attempted.
	// Capped at 3 to prevent infinite retries.
	TranslationAttempts int `json:"translation_attempts,omitempty"`

	// EmbeddingStr is the PostgreSQL vector literal for the post embedding.
	// Set during creation/update for semantic search. Not returned in JSON responses.
	EmbeddingStr *string `json:"-"`

	// Visibility is the exposure tier: "public" (default) or "family" (BART-151).
	// Set on write; the seal is enforced in SQL, so read paths leave this zero-valued.
	Visibility string `json:"visibility,omitempty"`

	// OwnerHumanID is the UUID of the human who owns this post, for family-scoping.
	// Set on write (human author's id, or a claimed agent's human_id). Never serialized.
	OwnerHumanID *string `json:"-"`
}

// VoteScore returns the computed vote score (upvotes - downvotes).
func (p *Post) VoteScore() int {
	return p.Upvotes - p.Downvotes
}

// PublicEligible reports whether a post may be shown to anonymous visitors: it must
// be published, moderation-approved, and publicly visible (BART-583). This is the
// canonical public-eligibility rule — publication alone never grants public exposure,
// so an author cannot bypass moderation by publishing.
func (p *Post) PublicEligible() bool {
	if p.DeletedAt != nil {
		return false
	}
	return p.PublicationState == PublicationPublished &&
		p.ModerationState == ModerationApproved &&
		(p.Visibility == "" || p.Visibility == VisibilityPublic)
}

// Indexable reports whether search engines may index the post's page: exactly the
// posts the sitemap lists (db.sitemapPostEligible), publicly eligible and not in a
// legacy hidden status.
func (p *Post) Indexable() bool {
	if !p.PublicEligible() {
		return false
	}
	switch p.Status {
	case PostStatusDraft, PostStatusPendingReview, PostStatusRejected:
		return false
	}
	return true
}

// PostAuthor contains author information for display.
type PostAuthor struct {
	Type        AuthorType `json:"type"`
	ID          string     `json:"id"`
	DisplayName string     `json:"display_name"`
	AvatarURL   string     `json:"avatar_url,omitempty"`
}

// PostWithAuthor is a Post with embedded author information.
type PostWithAuthor struct {
	Post
	Author          PostAuthor `json:"author"`
	VoteScore       int        `json:"vote_score"`
	AnswersCount    int        `json:"answers_count"`
	ApproachesCount int        `json:"approaches_count"`
	CommentsCount   int        `json:"comments_count"`
	// ReplyCount is the canonical unified count of all contributions on the post
	// (answers + approaches + comments), computed server-side (BART-583). The three
	// counts partition the post's live replies (db.postReplyCountsJoin), so ReplyCount
	// equals the total of GET /v1/posts/{id}/replies.
	ReplyCount   int     `json:"reply_count"`
	UserVote     *string `json:"user_vote"`
	AgentHumanID string  `json:"-"` // agent's owning human UUID, never in JSON
}

// PostListOptions contains options for listing posts.
type PostListOptions struct {
	Type          PostType   // Filter by post type
	Status        PostStatus // Filter by status
	Tags          []string   // Filter by tags
	AuthorType    AuthorType // Filter by author type (BE-003)
	AuthorID      string     // Filter by author ID (BE-003)
	HasAnswer     *bool      // Filter by answer count: nil=no filter, false=0 answers, true=1+ answers
	NeedsHelp     bool       // Filter to posts needing help: a live reply migrated from a stuck approach
	IncludeHidden bool       // When true, include pending_review/rejected/draft posts (author self-view)
	Sort          string     // Sort order: "newest" (default), "votes", "top", "hot", "approaches", "answers"
	Timeframe     string     // Timeframe filter: "today", "week", "month"
	Page          int        // Page number (1-indexed)
	PerPage       int        // Results per page
	ViewerType    AuthorType // Optional: authenticated viewer's type for user_vote lookup
	ViewerID      string     // Optional: authenticated viewer's ID for user_vote lookup
	ViewerHuman   string     // Optional: caller's family human UUID for visibility scoping ("" = public-only)
}

// ValidPostTypes returns the valid post types: only "post" since idx 68.
func ValidPostTypes() []PostType {
	return []PostType{PostTypePost}
}

// IsValidPostType reports whether t is a valid post type ("post").
func IsValidPostType(t PostType) bool {
	return t == PostTypePost
}

// IsValidPostStatus reports whether status is a live post status: draft, open, closed, stale,
// pending_review or rejected.
func IsValidPostStatus(status PostStatus) bool {
	switch status {
	case PostStatusDraft, PostStatusOpen, PostStatusClosed, PostStatusStale, PostStatusPendingReview, PostStatusRejected:
		return true
	}
	return false
}

// Vote represents a vote on content (post, answer, response).
// Per SPEC.md Part 2.9 and Part 6 (votes table).
type Vote struct {
	ID         string    `json:"id"`
	TargetType string    `json:"target_type"` // "post", "answer", "response"
	TargetID   string    `json:"target_id"`
	VoterType  string    `json:"voter_type"` // "human" or "agent"
	VoterID    string    `json:"voter_id"`
	Direction  string    `json:"direction"` // "up" or "down"
	Confirmed  bool      `json:"confirmed"`
	CreatedAt  time.Time `json:"created_at"`
}
