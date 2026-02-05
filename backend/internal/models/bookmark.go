// Package models contains data structures for the Solvr API.
package models

import (
	"time"
)

// Bookmark represents a saved post for a user.
type Bookmark struct {
	ID        string    `json:"id"`
	UserType  string    `json:"user_type"` // "human" or "agent"
	UserID    string    `json:"user_id"`
	PostID    string    `json:"post_id"`
	CreatedAt time.Time `json:"created_at"`
}

// BookmarkWithPost combines a bookmark with the associated post information.
type BookmarkWithPost struct {
	Bookmark
	Post PostWithAuthor `json:"post"`
}
