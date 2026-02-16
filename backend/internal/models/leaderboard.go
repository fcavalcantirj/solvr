package models

// LeaderboardEntry represents a single entry in the leaderboard.
// Contains rank, entity info, and key statistics.
type LeaderboardEntry struct {
	// Rank is the position on the leaderboard (1-indexed).
	Rank int `json:"rank"`

	// ID is the agent ID or user ID.
	ID string `json:"id"`

	// Type is either "agent" or "user".
	Type string `json:"type"`

	// DisplayName is the human-readable name.
	DisplayName string `json:"display_name"`

	// AvatarURL is the profile picture URL.
	AvatarURL string `json:"avatar_url"`

	// Reputation is the total reputation score.
	Reputation int `json:"reputation"`

	// KeyStats contains activity statistics.
	KeyStats LeaderboardStats `json:"key_stats"`
}

// LeaderboardStats contains key statistics for a leaderboard entry.
type LeaderboardStats struct {
	ProblemsSolved    int `json:"problems_solved"`
	AnswersAccepted   int `json:"answers_accepted"`
	UpvotesReceived   int `json:"upvotes_received"`
	TotalContributions int `json:"total_contributions"`
}

// LeaderboardOptions defines query parameters for fetching leaderboard.
type LeaderboardOptions struct {
	// Type filters by entity type: "all", "agents", or "users".
	// Default: "all"
	Type string

	// Timeframe filters by time period: "all_time", "monthly", or "weekly".
	// Default: "all_time"
	Timeframe string

	// Limit is the maximum number of entries to return.
	// Default: 50, Max: 100
	Limit int

	// Offset is the number of entries to skip.
	// Default: 0
	Offset int
}
