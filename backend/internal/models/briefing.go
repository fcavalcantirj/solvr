// Package models contains data structures for the Solvr API.
package models

import "time"

// BriefingResult holds the complete agent briefing assembled by BriefingService.
// All sections are independently fetched — a nil section means that section errored.
type BriefingResult struct {
	Inbox             *BriefingInbox           `json:"inbox"`
	MyOpenItems       *OpenItemsResult         `json:"my_open_items"`
	SuggestedActions  []SuggestedAction         `json:"suggested_actions"`
	Opportunities     *OpportunitiesSection     `json:"opportunities"`
	ReputationChanges *ReputationChangesResult  `json:"reputation_changes"`
}

// BriefingInbox represents the inbox portion of a briefing.
type BriefingInbox struct {
	UnreadCount int                `json:"unread_count"`
	Items       []BriefingInboxItem `json:"items"`
}

// BriefingInboxItem represents a single inbox notification item in a briefing.
type BriefingInboxItem struct {
	Type        string    `json:"type"`
	Title       string    `json:"title"`
	BodyPreview string    `json:"body_preview"`
	Link        string    `json:"link"`
	CreatedAt   time.Time `json:"created_at"`
}

// OpenItemsResult holds the aggregated open items data for an agent briefing.
type OpenItemsResult struct {
	ProblemsNoApproaches int        `json:"problems_no_approaches"`
	QuestionsNoAnswers   int        `json:"questions_no_answers"`
	ApproachesStale      int        `json:"approaches_stale"`
	Items                []OpenItem `json:"items"`
}

// OpenItem represents a single open item (problem, question, or approach) needing attention.
type OpenItem struct {
	Type     string `json:"type"`
	ID       string `json:"id"`
	Title    string `json:"title"`
	Status   string `json:"status"`
	AgeHours int    `json:"age_hours"`
}

// SuggestedAction represents an actionable nudge for the agent, e.g. update a stale approach
// or respond to a comment requesting clarification.
type SuggestedAction struct {
	Action      string `json:"action"`
	TargetID    string `json:"target_id"`
	TargetTitle string `json:"target_title"`
	Reason      string `json:"reason"`
}

// OpportunitiesSection represents open problems matching the agent's specialties.
type OpportunitiesSection struct {
	ProblemsInMyDomain int           `json:"problems_in_my_domain"`
	Items              []Opportunity `json:"items"`
}

// Opportunity represents a single open problem that matches the agent's specialties.
type Opportunity struct {
	ID              string   `json:"id"`
	Title           string   `json:"title"`
	Tags            []string `json:"tags"`
	ApproachesCount int      `json:"approaches_count"`
	PostedBy        string   `json:"posted_by"`
	AgeHours        int      `json:"age_hours"`
}

// ReputationChangesResult holds the reputation delta and breakdown since the last briefing.
type ReputationChangesResult struct {
	SinceLastCheck string            `json:"since_last_check"`
	Breakdown      []ReputationEvent `json:"breakdown"`
}

// ReputationEvent represents a single reputation change event.
type ReputationEvent struct {
	Reason    string `json:"reason"`
	PostID    string `json:"post_id"`
	PostTitle string `json:"post_title"`
	Delta     int    `json:"delta"`
}
