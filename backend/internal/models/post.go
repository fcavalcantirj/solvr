// Package models contains data structures for the Solvr API.
package models

import (
	"time"
)

// PostType represents the type of post.
type PostType string

const (
	PostTypeProblem  PostType = "problem"
	PostTypeQuestion PostType = "question"
	PostTypeIdea     PostType = "idea"
)

// MaxTagsPerPost is the maximum number of tags allowed per post.
const MaxTagsPerPost = 10

// PostStatus represents the status of a post.
type PostStatus string

// Post status constants per SPEC.md Part 2.2.
const (
	// Common statuses
	PostStatusDraft PostStatus = "draft"
	PostStatusOpen  PostStatus = "open"

	// Problem statuses
	PostStatusInProgress PostStatus = "in_progress"
	PostStatusSolved     PostStatus = "solved"
	PostStatusClosed     PostStatus = "closed"
	PostStatusStale      PostStatus = "stale"

	// Question statuses
	PostStatusAnswered PostStatus = "answered"

	// Idea statuses
	PostStatusActive  PostStatus = "active"
	PostStatusDormant PostStatus = "dormant"
	PostStatusEvolved PostStatus = "evolved"

	// Content moderation statuses (valid for all post types)
	PostStatusPendingReview PostStatus = "pending_review"
	PostStatusRejected     PostStatus = "rejected"
)

// AuthorType represents whether the author is a human or AI agent.
type AuthorType string

const (
	AuthorTypeHuman  AuthorType = "human"
	AuthorTypeAgent  AuthorType = "agent"
	AuthorTypeSystem AuthorType = "system"
)

// Post represents a problem, question, or idea on Solvr.
// Per SPEC.md Part 2.2 and Part 6 (posts table).
type Post struct {
	// ID is the unique identifier for the post.
	ID string `json:"id"`

	// Type is the post type: problem, question, or idea.
	Type PostType `json:"type"`

	// Title is the post title.
	// Max 200 chars.
	Title string `json:"title"`

	// Description is the post content in markdown.
	// Max varies by type: 50,000 for problems/ideas, 20,000 for questions.
	Description string `json:"description"`

	// Tags is a list of tags for the post.
	// See MaxTagsPerPost.
	Tags []string `json:"tags,omitempty"`

	// PostedByType is the author type: human or agent.
	PostedByType AuthorType `json:"posted_by_type"`

	// PostedByID is the author's ID (user UUID or agent ID).
	PostedByID string `json:"posted_by_id"`

	// Status is the current status of the post.
	Status PostStatus `json:"status"`

	// Upvotes is the number of upvotes.
	Upvotes int `json:"upvotes"`

	// Downvotes is the number of downvotes.
	Downvotes int `json:"downvotes"`

	// ViewCount is the number of unique views.
	ViewCount int `json:"view_count"`

	// SuccessCriteria is for problems only - list of success criteria.
	// Max 10 items per SPEC.md Part 2.2.
	SuccessCriteria []string `json:"success_criteria,omitempty"`

	// Weight is for problems only - difficulty rating (1-5).
	Weight *int `json:"weight,omitempty"`

	// AcceptedAnswerID is for questions only - the accepted answer ID.
	AcceptedAnswerID *string `json:"accepted_answer_id,omitempty"`

	// EvolvedInto is for ideas only - IDs of posts this idea evolved into.
	EvolvedInto []string `json:"evolved_into,omitempty"`

	// CreatedAt is when the post was created.
	CreatedAt time.Time `json:"created_at"`

	// UpdatedAt is when the post was last modified.
	UpdatedAt time.Time `json:"updated_at"`

	// DeletedAt is when the post was soft deleted (null if not deleted).
	DeletedAt *time.Time `json:"deleted_at,omitempty"`

	// CrystallizationCID is the IPFS CID of the immutable snapshot (problems only).
	// Set when a solved problem is crystallized to IPFS for permanent archival.
	CrystallizationCID *string `json:"crystallization_cid,omitempty"`

	// CrystallizedAt is when the problem was crystallized to IPFS.
	CrystallizedAt *time.Time `json:"crystallized_at,omitempty"`

	// EmbeddingStr is the PostgreSQL vector literal for the post embedding.
	// Set during creation/update for semantic search. Not returned in JSON responses.
	EmbeddingStr *string `json:"-"`
}

// VoteScore returns the computed vote score (upvotes - downvotes).
func (p *Post) VoteScore() int {
	return p.Upvotes - p.Downvotes
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
	UserVote        *string    `json:"user_vote,omitempty"`
}

// PostListOptions contains options for listing posts.
type PostListOptions struct {
	Type          PostType   // Filter by post type
	Status        PostStatus // Filter by status
	Tags          []string   // Filter by tags
	AuthorType    AuthorType // Filter by author type (BE-003)
	AuthorID      string     // Filter by author ID (BE-003)
	HasAnswer     *bool      // Filter by answer count: nil=no filter, false=0 answers, true=1+ answers
	IncludeHidden bool       // When true, include pending_review/rejected/draft posts (author self-view)
	Sort          string     // Sort order: "newest" (default), "votes", "approaches"
	Page          int        // Page number (1-indexed)
	PerPage       int        // Results per page
	ViewerType    AuthorType // Optional: authenticated viewer's type for user_vote lookup
	ViewerID      string     // Optional: authenticated viewer's ID for user_vote lookup
}

// ValidPostTypes returns all valid post types.
func ValidPostTypes() []PostType {
	return []PostType{PostTypeProblem, PostTypeQuestion, PostTypeIdea}
}

// IsValidPostType checks if a post type is valid.
func IsValidPostType(t PostType) bool {
	switch t {
	case PostTypeProblem, PostTypeQuestion, PostTypeIdea:
		return true
	default:
		return false
	}
}

// IsValidPostStatus checks if a post status is valid for the given type.
func IsValidPostStatus(status PostStatus, postType PostType) bool {
	// Content moderation statuses are valid for all post types.
	if status == PostStatusPendingReview || status == PostStatusRejected {
		return IsValidPostType(postType)
	}

	switch postType {
	case PostTypeProblem:
		switch status {
		case PostStatusDraft, PostStatusOpen, PostStatusInProgress, PostStatusSolved, PostStatusClosed, PostStatusStale:
			return true
		}
	case PostTypeQuestion:
		switch status {
		case PostStatusDraft, PostStatusOpen, PostStatusAnswered, PostStatusClosed, PostStatusStale:
			return true
		}
	case PostTypeIdea:
		switch status {
		case PostStatusDraft, PostStatusOpen, PostStatusActive, PostStatusDormant, PostStatusEvolved:
			return true
		}
	}
	return false
}

// Vote represents a vote on content (post, answer, response).
// Per SPEC.md Part 2.9 and Part 6 (votes table).
type Vote struct {
	ID         string     `json:"id"`
	TargetType string     `json:"target_type"` // "post", "answer", "response"
	TargetID   string     `json:"target_id"`
	VoterType  string     `json:"voter_type"` // "human" or "agent"
	VoterID    string     `json:"voter_id"`
	Direction  string     `json:"direction"` // "up" or "down"
	Confirmed  bool       `json:"confirmed"`
	CreatedAt  time.Time  `json:"created_at"`
}
