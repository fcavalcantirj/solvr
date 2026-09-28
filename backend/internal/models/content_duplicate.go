// Package models contains data structures for the Solvr API.
package models

import "time"

// ContentDuplicate names the earlier live post or reply whose content a new post or
// reply repeats. Duplicate detection reads it from the canonical posts and replies tables.
type ContentDuplicate struct {
	TargetType string // "post" or "reply"
	TargetID   string
	PostID     string // the post itself, or the post the reply belongs to
	CreatedAt  time.Time
}
