// Package models contains data structures for the Solvr API.
package models

import (
	"time"
)

// CommentTargetType represents the type of entity a comment is attached to.
type CommentTargetType string

// Comment target types per SPEC.md Part 2.6.
// FIX-019: Added "post" target type for comments directly on posts (problems/questions/ideas).
const (
	CommentTargetApproach CommentTargetType = "approach"
	CommentTargetAnswer   CommentTargetType = "answer"
	CommentTargetResponse CommentTargetType = "response"
	CommentTargetPost     CommentTargetType = "post"
)

// IsValidCommentTargetType checks if a target type is valid.
func IsValidCommentTargetType(t CommentTargetType) bool {
	switch t {
	case CommentTargetApproach, CommentTargetAnswer, CommentTargetResponse, CommentTargetPost:
		return true
	}
	return false
}

// ValidCommentTargetTypes returns all valid target types.
func ValidCommentTargetTypes() []CommentTargetType {
	return []CommentTargetType{
		CommentTargetApproach,
		CommentTargetAnswer,
		CommentTargetResponse,
		CommentTargetPost,
	}
}

// Comment represents a comment on an approach, answer, or response.
// SPEC.md Part 2.6: Comments
type Comment struct {
	ID         string            `json:"id"`
	TargetType CommentTargetType `json:"target_type"`
	TargetID   string            `json:"target_id"`
	AuthorType AuthorType        `json:"author_type"`
	AuthorID   string            `json:"author_id"`
	Content    string            `json:"content"`
	CreatedAt  time.Time         `json:"created_at"`
	DeletedAt  *time.Time        `json:"deleted_at,omitempty"`
}

// CommentAuthor represents the author information for display.
type CommentAuthor struct {
	ID          string     `json:"id"`
	Type        AuthorType `json:"type"`
	DisplayName string     `json:"display_name"`
	AvatarURL   *string    `json:"avatar_url,omitempty"`
}

// CommentWithAuthor combines a comment with its author information.
type CommentWithAuthor struct {
	Comment
	Author CommentAuthor `json:"author"`
}

// CommentListOptions for filtering and pagination.
type CommentListOptions struct {
	TargetType CommentTargetType
	TargetID   string
	Page       int
	PerPage    int
}

// CreateCommentRequest is the request body for creating a comment.
type CreateCommentRequest struct {
	Content string `json:"content"`
}

// MaxCommentContentLength is the maximum content length per SPEC.md.
const MaxCommentContentLength = 2000
