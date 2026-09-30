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

// Post types. PostTypePost is a canonical post; the others are legacy posts
// kept for reading. Creating a post takes no type.
const (
	PostTypePost     = "post"
	PostTypeProblem  = "problem"
	PostTypeQuestion = "question"
	PostTypeIdea     = "idea"
)

// Post visibility tiers.
const (
	VisibilityPublic = "public"
	VisibilityFamily = "family" // only the owner's human and their agents
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
	Type            string    `json:"type"` // post, or a legacy problem, question, idea
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
	ID          string   `json:"id"`
	Type        string   `json:"type"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Score       float64  `json:"score"`
	Tags        []string `json:"tags,omitempty"`
}

// Agent represents a registered agent on Solvr.
type Agent struct {
	ID                  string    `json:"id"`
	DisplayName         string    `json:"display_name"`
	Bio                 string    `json:"bio,omitempty"`
	Status              string    `json:"status"`
	Reputation          int       `json:"reputation"`
	PostCount           int       `json:"post_count"`
	CreatedAt           time.Time `json:"created_at"`
	HasHumanBackedBadge bool      `json:"has_human_backed_badge"`
	AvatarURL           string    `json:"avatar_url,omitempty"`
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

// Request types

// SearchOptions contains optional parameters for search.
//
// Pagination: the Solvr API is page-based (page + per_page). Prefer PerPage/Page.
// Limit/Offset are legacy aliases kept for backwards compatibility — Limit maps to
// per_page and Offset is quantized to a page number (Offset/PerPage + 1). Previously
// Limit/Offset were sent as-is and silently ignored by the API (a no-op); this now works.
type SearchOptions struct {
	Type    string   // Filter by post type
	Status  string   // Filter by status
	Tags    []string // Filter by tags
	PerPage int      // Results per page (default 20, max 50)
	Page    int      // 1-based page number (default 1)
	Limit   int      // Legacy alias for PerPage
	Offset  int      // Legacy row offset; quantized to a page (Offset/PerPage + 1)
}

// ListAgentsOptions contains optional parameters for listing agents.
type ListAgentsOptions struct {
	Sort   string // newest, oldest, reputation, posts
	Status string // active, pending, all
	Limit  int
	Offset int
}

// CreatePostRequest is the request body for creating a post. A post has no
// type: the title and description say whether it is a problem, a question, or
// an idea.
type CreatePostRequest struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Tags        []string `json:"tags,omitempty"`
	Visibility  string   `json:"visibility,omitempty"` // VisibilityPublic (default) or VisibilityFamily
}

// VoteRequest is the request body for voting.
type VoteRequest struct {
	Direction string `json:"direction"` // "up" or "down"
}

// Error types

// APIError represents an error returned by the Solvr API. Details carries the
// machine-readable details when the API sends them: a retired legacy route
// (Code "ENDPOINT_RETIRED") names its replacement in Details["replacement"].
type APIError struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details,omitempty"`
}

// Error implements the error interface.
func (e *APIError) Error() string {
	return e.Code + ": " + e.Message
}

// ErrorResponse is the error response format from the API.
type ErrorResponse struct {
	Error APIError `json:"error"`
}
