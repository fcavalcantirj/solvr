// Package models contains data structures for the Solvr API.
package models

import (
	"time"
)

// ResponseType represents the type of response to an idea.
// Per SPEC.md Part 2.5, response types indicate how the response builds on the idea.
type ResponseType string

const (
	ResponseTypeBuild    ResponseType = "build"    // Building on the idea
	ResponseTypeCritique ResponseType = "critique" // Critiquing the idea
	ResponseTypeExpand   ResponseType = "expand"   // Expanding the idea
	ResponseTypeQuestion ResponseType = "question" // Asking a question about the idea
	ResponseTypeSupport  ResponseType = "support"  // Supporting the idea
)

// Response represents a response to an idea on Solvr.
// Per SPEC.md Part 2.5 and Part 6 (responses table).
type Response struct {
	// ID is the unique identifier for the response.
	ID string `json:"id"`

	// IdeaID is the ID of the idea this response belongs to.
	IdeaID string `json:"idea_id"`

	// AuthorType is the author type: human or agent.
	AuthorType AuthorType `json:"author_type"`

	// AuthorID is the author's ID (user UUID or agent ID).
	AuthorID string `json:"author_id"`

	// Content is the response content in markdown.
	// Max 10,000 chars per SPEC.md Part 2.5.
	Content string `json:"content"`

	// ResponseType indicates how this response builds on the idea.
	ResponseType ResponseType `json:"response_type"`

	// Upvotes is the number of upvotes.
	Upvotes int `json:"upvotes"`

	// Downvotes is the number of downvotes.
	Downvotes int `json:"downvotes"`

	// CreatedAt is when the response was created.
	CreatedAt time.Time `json:"created_at"`
}

// VoteScore returns the computed vote score (upvotes - downvotes).
func (r *Response) VoteScore() int {
	return r.Upvotes - r.Downvotes
}

// ResponseAuthor contains author information for display.
type ResponseAuthor struct {
	Type        AuthorType `json:"type"`
	ID          string     `json:"id"`
	DisplayName string     `json:"display_name"`
	AvatarURL   string     `json:"avatar_url,omitempty"`
}

// ResponseWithAuthor is a Response with embedded author information.
type ResponseWithAuthor struct {
	Response
	Author    ResponseAuthor `json:"author"`
	VoteScore int            `json:"vote_score"`
}

// ResponseListOptions contains options for listing responses.
type ResponseListOptions struct {
	IdeaID       string       // Filter by idea ID
	ResponseType ResponseType // Filter by response type
	Page         int          // Page number (1-indexed)
	PerPage      int          // Results per page
}

// ResponseWithContext is a response with parent idea context.
// Used by ListByAuthor to provide context about what idea was responded to.
type ResponseWithContext struct {
	ResponseWithAuthor
	IdeaTitle string `json:"idea_title"`
}

// CreateResponseRequest is the request body for creating a response.
type CreateResponseRequest struct {
	Content      string       `json:"content"`
	ResponseType ResponseType `json:"response_type"`
}

// ValidResponseTypes returns all valid response types.
func ValidResponseTypes() []ResponseType {
	return []ResponseType{
		ResponseTypeBuild,
		ResponseTypeCritique,
		ResponseTypeExpand,
		ResponseTypeQuestion,
		ResponseTypeSupport,
	}
}

// IsValidResponseType checks if a response type is valid.
func IsValidResponseType(t ResponseType) bool {
	switch t {
	case ResponseTypeBuild, ResponseTypeCritique, ResponseTypeExpand, ResponseTypeQuestion, ResponseTypeSupport:
		return true
	default:
		return false
	}
}
