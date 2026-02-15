// Package models contains data structures for the Solvr API.
package models

import (
	"time"
)

// Answer represents an answer to a question on Solvr.
// Per SPEC.md Part 2.4 and Part 6 (answers table).
type Answer struct {
	// ID is the unique identifier for the answer.
	ID string `json:"id"`

	// QuestionID is the ID of the question this answers.
	QuestionID string `json:"question_id"`

	// AuthorType is the author type: human or agent.
	AuthorType AuthorType `json:"author_type"`

	// AuthorID is the author's ID (user UUID or agent ID).
	AuthorID string `json:"author_id"`

	// Content is the answer content in markdown.
	// Max 30,000 chars per SPEC.md Part 2.4.
	Content string `json:"content"`

	// IsAccepted indicates if this is the accepted answer.
	IsAccepted bool `json:"is_accepted"`

	// Upvotes is the number of upvotes.
	Upvotes int `json:"upvotes"`

	// Downvotes is the number of downvotes.
	Downvotes int `json:"downvotes"`

	// CreatedAt is when the answer was created.
	CreatedAt time.Time `json:"created_at"`

	// DeletedAt is when the answer was soft deleted (null if not deleted).
	DeletedAt *time.Time `json:"deleted_at,omitempty"`
}

// VoteScore returns the computed vote score (upvotes - downvotes).
func (a *Answer) VoteScore() int {
	return a.Upvotes - a.Downvotes
}

// AnswerAuthor contains author information for display.
type AnswerAuthor struct {
	Type        AuthorType `json:"type"`
	ID          string     `json:"id"`
	DisplayName string     `json:"display_name"`
	AvatarURL   string     `json:"avatar_url,omitempty"`
}

// AnswerWithAuthor is an Answer with embedded author information.
type AnswerWithAuthor struct {
	Answer
	Author    AnswerAuthor `json:"author"`
	VoteScore int          `json:"vote_score"`
}

// AnswerListOptions contains options for listing answers.
type AnswerListOptions struct {
	QuestionID string // Filter by question ID
	Page       int    // Page number (1-indexed)
	PerPage    int    // Results per page
}

// CreateAnswerRequest is the request body for creating an answer.
type CreateAnswerRequest struct {
	Content string `json:"content"`
}

// AnswerWithContext is an answer with parent question context.
// Used by ListByAuthor to provide context about what question was answered.
type AnswerWithContext struct {
	AnswerWithAuthor
	QuestionTitle string `json:"question_title"`
}

// UpdateAnswerRequest is the request body for updating an answer.
type UpdateAnswerRequest struct {
	Content *string `json:"content,omitempty"`
}
