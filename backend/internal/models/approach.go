// Package models contains data structures for the Solvr API.
package models

import (
	"time"
)

// ApproachStatus represents the status of an approach.
// Per SPEC.md Part 2.3.
type ApproachStatus string

const (
	ApproachStatusStarting  ApproachStatus = "starting"
	ApproachStatusWorking   ApproachStatus = "working"
	ApproachStatusStuck     ApproachStatus = "stuck"
	ApproachStatusFailed    ApproachStatus = "failed"
	ApproachStatusSucceeded ApproachStatus = "succeeded"
)

// IsValidApproachStatus checks if an approach status is valid.
func IsValidApproachStatus(s ApproachStatus) bool {
	switch s {
	case ApproachStatusStarting, ApproachStatusWorking, ApproachStatusStuck,
		ApproachStatusFailed, ApproachStatusSucceeded:
		return true
	default:
		return false
	}
}

// Approach represents a declared strategy for tackling a problem.
// Per SPEC.md Part 2.3 and Part 6 (approaches table).
type Approach struct {
	// ID is the unique identifier for the approach.
	ID string `json:"id"`

	// ProblemID is the ID of the problem this approach is for.
	ProblemID string `json:"problem_id"`

	// AuthorType is the author type: human or agent.
	AuthorType AuthorType `json:"author_type"`

	// AuthorID is the author's ID (user UUID or agent ID).
	AuthorID string `json:"author_id"`

	// Angle is the perspective or approach being taken.
	// Max 500 chars.
	Angle string `json:"angle"`

	// Method is the specific technique being used.
	// Max 500 chars.
	Method string `json:"method,omitempty"`

	// Assumptions are the assumptions the approach is based on.
	// Max 10 items.
	Assumptions []string `json:"assumptions,omitempty"`

	// DiffersFrom are IDs of past approaches this one differs from.
	DiffersFrom []string `json:"differs_from,omitempty"`

	// Status is the current status of the approach.
	Status ApproachStatus `json:"status"`

	// Outcome is the learnings from the approach.
	// Max 10,000 chars.
	Outcome string `json:"outcome,omitempty"`

	// Solution is the solution if the approach succeeded.
	// Max 50,000 chars.
	Solution string `json:"solution,omitempty"`

	// CreatedAt is when the approach was created.
	CreatedAt time.Time `json:"created_at"`

	// UpdatedAt is when the approach was last modified.
	UpdatedAt time.Time `json:"updated_at"`

	// DeletedAt is when the approach was soft deleted (null if not deleted).
	DeletedAt *time.Time `json:"deleted_at,omitempty"`

	// EmbeddingStr carries PostgreSQL vector literal from handler to repository.
	// Excluded from JSON responses.
	EmbeddingStr *string `json:"-"`
}

// ApproachAuthor contains author information for display.
type ApproachAuthor struct {
	Type        AuthorType `json:"type"`
	ID          string     `json:"id"`
	DisplayName string     `json:"display_name"`
	AvatarURL   string     `json:"avatar_url,omitempty"`
}

// ApproachWithAuthor is an Approach with embedded author information.
type ApproachWithAuthor struct {
	Approach
	Author        ApproachAuthor `json:"author"`
	ProgressNotes []ProgressNote `json:"progress_notes,omitempty"`
}

// ProgressNote represents a progress note on an approach.
// Per SPEC.md Part 6 (progress_notes table).
type ProgressNote struct {
	// ID is the unique identifier for the note.
	ID string `json:"id"`

	// ApproachID is the ID of the approach this note is for.
	ApproachID string `json:"approach_id"`

	// Content is the note content.
	Content string `json:"content"`

	// CreatedAt is when the note was created.
	CreatedAt time.Time `json:"created_at"`
}

// ApproachListOptions contains options for listing approaches.
type ApproachListOptions struct {
	ProblemID string         // Filter by problem ID (required)
	Status    ApproachStatus // Filter by status
	Page      int            // Page number (1-indexed)
	PerPage   int            // Results per page
}

// ApproachWithContext is an approach with parent problem context.
// Used by ListByAuthor to provide context about what problem was approached.
type ApproachWithContext struct {
	ApproachWithAuthor
	ProblemTitle string `json:"problem_title"`
}

// CreateApproachRequest is the request body for creating an approach.
type CreateApproachRequest struct {
	Angle       string   `json:"angle"`
	Method      string   `json:"method,omitempty"`
	Assumptions []string `json:"assumptions,omitempty"`
	DiffersFrom []string `json:"differs_from,omitempty"`
}

// UpdateApproachRequest is the request body for updating an approach.
type UpdateApproachRequest struct {
	Status  *string `json:"status,omitempty"`
	Outcome *string `json:"outcome,omitempty"`
	Method  *string `json:"method,omitempty"`
}
