// Package models defines data types for Solvr.
package models

import (
	"time"
)

// FeedAuthor contains author information for feed items.
type FeedAuthor struct {
	Type        string `json:"type"`
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	AvatarURL   string `json:"avatar_url,omitempty"`
}

// FeedItem represents a single item in the feed.
// Per SPEC.md Part 4.4 - Post cards in feed.
type FeedItem struct {
	// ID is the post UUID.
	ID string `json:"id"`

	// Type is the post type: problem, question, or idea.
	Type string `json:"type"`

	// Title is the post title.
	Title string `json:"title"`

	// Snippet is a short preview of the description.
	Snippet string `json:"snippet"`

	// Tags are the post tags.
	Tags []string `json:"tags,omitempty"`

	// Status is the current post status.
	Status string `json:"status"`

	// Author is the post author information.
	Author FeedAuthor `json:"author"`

	// VoteScore is upvotes minus downvotes.
	VoteScore int `json:"vote_score"`

	// AnswerCount is the number of answers (for questions) or approaches (for problems).
	AnswerCount int `json:"answer_count"`

	// ApproachCount is the number of approaches (for problems).
	ApproachCount int `json:"approach_count,omitempty"`

	// CreatedAt is when the post was created.
	CreatedAt time.Time `json:"created_at"`
}

// FeedMeta contains pagination metadata for feed responses.
type FeedMeta struct {
	Total   int  `json:"total"`
	Page    int  `json:"page"`
	PerPage int  `json:"per_page"`
	HasMore bool `json:"has_more"`
}

// FeedResponse is the response for feed endpoints.
type FeedResponse struct {
	Data []FeedItem `json:"data"`
	Meta FeedMeta   `json:"meta"`
}
