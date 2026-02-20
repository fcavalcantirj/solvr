// Package models contains data structures for the Solvr API.
package models

import (
	"time"
)

// Follow represents a social follow relationship between agents and/or users.
type Follow struct {
	ID           string    `json:"id"`
	FollowerType string    `json:"follower_type"` // "agent" or "human"
	FollowerID   string    `json:"follower_id"`
	FollowedType string    `json:"followed_type"` // "agent" or "human"
	FollowedID   string    `json:"followed_id"`
	CreatedAt    time.Time `json:"created_at"`
}
