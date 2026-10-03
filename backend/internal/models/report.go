// Package models contains data structures for the Solvr API.
package models

import (
	"time"
)

// ReportReason represents the reason for flagging content.
type ReportReason string

const (
	ReportReasonSpam       ReportReason = "spam"
	ReportReasonOffensive  ReportReason = "offensive"
	ReportReasonOffTopic   ReportReason = "off_topic"
	ReportReasonMisleading ReportReason = "misleading"
	ReportReasonOther      ReportReason = "other"
)

// ReportStatus represents the status of a report.
type ReportStatus string

const (
	ReportStatusPending   ReportStatus = "pending"
	ReportStatusReviewed  ReportStatus = "reviewed"
	ReportStatusActioned  ReportStatus = "actioned"
	ReportStatusDismissed ReportStatus = "dismissed"
)

// ReportTargetType represents the type of content being reported.
type ReportTargetType string

const (
	ReportTargetPost ReportTargetType = "post"
	// ReportTargetReply is the canonical unified contribution (BART-585).
	ReportTargetReply ReportTargetType = "reply"
)

// RetiredReportTargetTypes are the legacy contribution types a report named before each
// became a reply (idx 68); the reports target check admits only post and reply.
var RetiredReportTargetTypes = []ReportTargetType{"answer", "approach", "response", "comment"}

// IsRetiredReportTargetType reports whether t is a legacy contribution type.
func IsRetiredReportTargetType(t ReportTargetType) bool {
	for _, r := range RetiredReportTargetTypes {
		if r == t {
			return true
		}
	}
	return false
}

// Report represents a user report of inappropriate content.
type Report struct {
	ID           string           `json:"id"`
	TargetType   ReportTargetType `json:"target_type"`
	TargetID     string           `json:"target_id"`
	ReporterType AuthorType       `json:"reporter_type"`
	ReporterID   string           `json:"reporter_id"`
	Reason       ReportReason     `json:"reason"`
	Details      string           `json:"details,omitempty"`
	Status       ReportStatus     `json:"status"`
	CreatedAt    time.Time        `json:"created_at"`
	ReviewedAt   *time.Time       `json:"reviewed_at,omitempty"`
	ReviewedBy   string           `json:"reviewed_by,omitempty"`
}

// IsValidReportReason checks if a report reason is valid.
func IsValidReportReason(reason ReportReason) bool {
	switch reason {
	case ReportReasonSpam, ReportReasonOffensive, ReportReasonOffTopic, ReportReasonMisleading, ReportReasonOther:
		return true
	default:
		return false
	}
}

// IsValidReportTargetType checks if a report target type is valid.
func IsValidReportTargetType(targetType ReportTargetType) bool {
	switch targetType {
	case ReportTargetPost, ReportTargetReply:
		return true
	default:
		return false
	}
}
