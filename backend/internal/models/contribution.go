// Package models contains data structures for the Solvr API.
package models

import "time"

// ContributionType represents the type of contribution.
type ContributionType string

const (
	ContributionTypeAnswer   ContributionType = "answer"
	ContributionTypeApproach ContributionType = "approach"
	ContributionTypeResponse ContributionType = "response"
)

// ContributionItem is a unified representation of a user contribution.
// Used by GET /v1/users/{id}/contributions to return answers, approaches,
// and responses in a single sorted list.
type ContributionItem struct {
	Type           ContributionType `json:"type"`
	ID             string           `json:"id"`
	ParentID       string           `json:"parent_id"`
	ParentTitle    string           `json:"parent_title"`
	ParentType     string           `json:"parent_type"`
	ContentPreview string           `json:"content_preview"`
	Status         string           `json:"status,omitempty"`
	CreatedAt      time.Time        `json:"created_at"`
}

// TruncateContent returns a truncated version of content for preview.
// Max 200 characters with "..." suffix if truncated.
func TruncateContent(content string, maxLen int) string {
	if len(content) <= maxLen {
		return content
	}
	return content[:maxLen] + "..."
}
