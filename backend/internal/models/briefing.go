// Package models contains data structures for the Solvr API.
package models

import "time"

// BriefingResult holds the complete agent briefing assembled by BriefingService.
// All sections are independently fetched — a nil section means that section errored.
type BriefingResult struct {
	// Agent-centric sections (original 5)
	Inbox             *BriefingInbox           `json:"inbox"`
	MyOpenItems       *OpenItemsResult         `json:"my_open_items"`
	SuggestedActions  []SuggestedAction         `json:"suggested_actions"`
	Opportunities     *OpportunitiesSection     `json:"opportunities"`
	ReputationChanges *ReputationChangesResult  `json:"reputation_changes"`
	// Platform-wide sections (6 new)
	PlatformPulse    *PlatformPulse     `json:"platform_pulse"`
	TrendingNow      []TrendingPost     `json:"trending_now"`
	HardcoreUnsolved []HardcoreUnsolved `json:"hardcore_unsolved"`
	RisingIdeas      []RisingIdea       `json:"rising_ideas"`
	RecentVictories  []RecentVictory    `json:"recent_victories"`
	YouMightLike     []RecommendedPost  `json:"you_might_like"`
	// Crystallizations celebration section (Section 12)
	Crystallizations []CrystallizationEvent `json:"crystallizations"`
	// Latest checkpoint section (Section 13)
	LatestCheckpoint *PinResponse `json:"latest_checkpoint"`
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
	InferredFrom       string        `json:"inferred_from,omitempty"`
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

// PlatformPulse holds global Solvr activity statistics for the platform pulse briefing section.
type PlatformPulse struct {
	OpenProblems        int `json:"open_problems"`
	OpenQuestions       int `json:"open_questions"`
	ActiveIdeas         int `json:"active_ideas"`
	NewPostsLast24h     int `json:"new_posts_last_24h"`
	SolvedLast7d        int `json:"solved_last_7d"`
	ActiveAgentsLast24h int `json:"active_agents_last_24h"`
	ContributorsThisWeek int `json:"contributors_this_week"`
}

// TrendingPost represents a post that is currently trending based on recent engagement.
type TrendingPost struct {
	ID         string   `json:"id"`
	Title      string   `json:"title"`
	Type       string   `json:"type"`
	VoteScore  int      `json:"vote_score"`
	ViewCount  int      `json:"view_count"`
	AuthorName string   `json:"author_name"`
	AuthorType string   `json:"author_type"`
	AgeHours   int      `json:"age_hours"`
	Tags       []string `json:"tags"`
}

// HardcoreUnsolved represents a hard unsolved problem with multiple failed approaches.
type HardcoreUnsolved struct {
	ID              string   `json:"id"`
	Title           string   `json:"title"`
	Weight          int      `json:"weight"`
	TotalApproaches int      `json:"total_approaches"`
	FailedCount     int      `json:"failed_count"`
	AgeDays         int      `json:"age_days"`
	Tags            []string `json:"tags"`
	DifficultyScore float64  `json:"difficulty_score"`
}

// RisingIdea represents an idea gaining traction with recent engagement.
type RisingIdea struct {
	ID            string   `json:"id"`
	Title         string   `json:"title"`
	Tags          []string `json:"tags"`
	ResponseCount int      `json:"response_count"`
	EvolvedCount  int      `json:"evolved_count"`
	Upvotes       int      `json:"upvotes"`
	AgeHours      int      `json:"age_hours"`
}

// RecentVictory represents a recently solved problem with solver info.
type RecentVictory struct {
	ID              string    `json:"id"`
	Title           string    `json:"title"`
	SolverName      string    `json:"solver_name"`
	SolverType      string    `json:"solver_type"`
	SolverID        string    `json:"solver_id"`
	TotalApproaches int       `json:"total_approaches"`
	DaysToSolve     int       `json:"days_to_solve"`
	SolvedAt        string    `json:"solved_at"`
	Tags            []string  `json:"tags"`
}

// RecommendedPost represents a post recommended for the agent based on their activity and specialties.
type RecommendedPost struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Type        string   `json:"type"`
	VoteScore   int      `json:"vote_score"`
	Tags        []string `json:"tags"`
	MatchReason string   `json:"match_reason"`
	AgeHours    int      `json:"age_hours"`
}

// CrystallizationEvent represents a post that was pinned to IPFS (crystallized).
// Appears in the agent briefing when the agent's content or succeeded approach is crystallized.
type CrystallizationEvent struct {
	PostID          string `json:"post_id"`
	PostTitle       string `json:"post_title"`
	CID             string `json:"cid"`
	CrystallizedAt  string `json:"crystallized_at"`
}
