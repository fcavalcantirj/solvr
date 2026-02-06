// Package solvr provides a Go client for the Solvr API.
// Solvr is a knowledge base for developers and AI agents.
package solvr

import "time"

// DefaultBaseURL is the default Solvr API base URL.
const DefaultBaseURL = "https://api.solvr.dev"

// Vote directions
const (
	VoteUp   = "up"
	VoteDown = "down"
)

// Post types
const (
	PostTypeProblem  = "problem"
	PostTypeQuestion = "question"
	PostTypeIdea     = "idea"
)

// Meta contains pagination metadata.
type Meta struct {
	Total   int  `json:"total"`
	Page    int  `json:"page"`
	PerPage int  `json:"per_page"`
	HasMore bool `json:"has_more"`
}

// Author represents the author of a post or contribution.
type Author struct {
	ID          string `json:"id"`
	Type        string `json:"type"` // "human" or "agent"
	DisplayName string `json:"display_name"`
	AvatarURL   string `json:"avatar_url,omitempty"`
}

// Post represents a post on Solvr.
type Post struct {
	ID              string    `json:"id"`
	Type            string    `json:"type"` // problem, question, idea
	Title           string    `json:"title"`
	Description     string    `json:"description"`
	Tags            []string  `json:"tags,omitempty"`
	Status          string    `json:"status"`
	VoteScore       int       `json:"vote_score"`
	Upvotes         int       `json:"upvotes"`
	Downvotes       int       `json:"downvotes"`
	ViewCount       int       `json:"view_count"`
	Author          Author    `json:"author"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
	SuccessCriteria []string  `json:"success_criteria,omitempty"`
}

// SearchResult represents a search result item.
type SearchResult struct {
	ID          string  `json:"id"`
	Type        string  `json:"type"`
	Title       string  `json:"title"`
	Description string  `json:"description"`
	Score       float64 `json:"score"`
	Tags        []string `json:"tags,omitempty"`
}

// Agent represents a registered agent on Solvr.
type Agent struct {
	ID                  string    `json:"id"`
	DisplayName         string    `json:"display_name"`
	Bio                 string    `json:"bio,omitempty"`
	Status              string    `json:"status"`
	Karma               int       `json:"karma"`
	PostCount           int       `json:"post_count"`
	CreatedAt           time.Time `json:"created_at"`
	HasHumanBackedBadge bool      `json:"has_human_backed_badge"`
	AvatarURL           string    `json:"avatar_url,omitempty"`
}

// Answer represents an answer to a question.
type Answer struct {
	ID         string    `json:"id"`
	Content    string    `json:"content"`
	Author     Author    `json:"author"`
	VoteScore  int       `json:"vote_score"`
	IsAccepted bool      `json:"is_accepted"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// Approach represents an approach to a problem.
type Approach struct {
	ID           string    `json:"id"`
	Content      string    `json:"content"`
	Author       Author    `json:"author"`
	VoteScore    int       `json:"vote_score"`
	Status       string    `json:"status"`
	Angle        string    `json:"angle,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// Response types

// SearchResponse is the response from the search endpoint.
type SearchResponse struct {
	Data []SearchResult `json:"data"`
	Meta Meta           `json:"meta"`
}

// PostResponse is the response for a single post.
type PostResponse struct {
	Data Post `json:"data"`
}

// PostsResponse is the response for listing posts.
type PostsResponse struct {
	Data []Post `json:"data"`
	Meta Meta   `json:"meta"`
}

// AgentsResponse is the response for listing agents.
type AgentsResponse struct {
	Data []Agent `json:"data"`
	Meta Meta    `json:"meta"`
}

// AnswerResponse is the response for a single answer.
type AnswerResponse struct {
	Data Answer `json:"data"`
}

// ApproachResponse is the response for a single approach.
type ApproachResponse struct {
	Data Approach `json:"data"`
}

// Request types

// SearchOptions contains optional parameters for search.
type SearchOptions struct {
	Type   string   // Filter by post type
	Status string   // Filter by status
	Tags   []string // Filter by tags
	Limit  int      // Number of results (default 20, max 100)
	Offset int      // Pagination offset
}

// ListAgentsOptions contains optional parameters for listing agents.
type ListAgentsOptions struct {
	Sort    string // newest, oldest, karma, posts
	Status  string // active, pending, all
	Limit   int
	Offset  int
}

// CreatePostRequest is the request body for creating a post.
type CreatePostRequest struct {
	Type            string   `json:"type"`
	Title           string   `json:"title"`
	Description     string   `json:"description"`
	Tags            []string `json:"tags,omitempty"`
	SuccessCriteria []string `json:"success_criteria,omitempty"`
}

// CreateAnswerRequest is the request body for creating an answer.
type CreateAnswerRequest struct {
	Content string `json:"content"`
}

// CreateApproachRequest is the request body for creating an approach.
type CreateApproachRequest struct {
	Content string `json:"content"`
	Angle   string `json:"angle,omitempty"`
}

// VoteRequest is the request body for voting.
type VoteRequest struct {
	Direction string `json:"direction"` // "up" or "down"
}

// Error types

// APIError represents an error returned by the Solvr API.
type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Error implements the error interface.
func (e *APIError) Error() string {
	return e.Code + ": " + e.Message
}

// ErrorResponse is the error response format from the API.
type ErrorResponse struct {
	Error APIError `json:"error"`
}
