// Package models contains data structures for the Solvr API.
package models

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
