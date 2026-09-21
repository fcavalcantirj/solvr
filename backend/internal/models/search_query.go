package models

import "time"

// SearchQuery records a single search request for analytics.
type SearchQuery struct {
	ID              int64     `json:"id"`
	Query           string    `json:"query"`
	QueryNormalized string    `json:"query_normalized"`
	TypeFilter      *string   `json:"type_filter,omitempty"`
	ResultsCount    int       `json:"results_count"`
	SearchMethod    string    `json:"search_method"`
	DurationMs      int       `json:"duration_ms"`
	SearcherType    string    `json:"searcher_type"`
	SearcherID      *string   `json:"searcher_id,omitempty"`
	IPAddress       string    `json:"ip_address,omitempty"`
	UserAgent       string    `json:"user_agent,omitempty"`
	Page            int       `json:"page"`
	SearchedAt      time.Time `json:"searched_at"`

	// PublicScope records whether this search ran over PUBLIC CONTENT ONLY.
	// GET /v1/search is family-scoped, so a signed-in human (or an agent that
	// human claimed) can match posts that are not public. nil means the search
	// was served before this was recorded: eligibility unknown, and its text is
	// never publishable. See db.PublicQueryText.
	PublicScope *bool `json:"public_scope,omitempty"`
}

// TrendingSearch represents an aggregated trending search term.
type TrendingSearch struct {
	Query       string  `json:"query"`
	Count       int     `json:"count"`
	AvgResults  float64 `json:"avg_results"`
	AvgDuration float64 `json:"avg_duration_ms"`
}

// SearchAnalytics is the response for the analytics summary endpoint.
type SearchAnalytics struct {
	TotalSearches  int              `json:"total_searches"`
	UniqueQueries  int              `json:"unique_queries"`
	AvgDurationMs  float64          `json:"avg_duration_ms"`
	ZeroResultRate float64          `json:"zero_result_rate"`
	BySearcherType map[string]int   `json:"by_searcher_type"`
	TopQueries     []TrendingSearch `json:"top_queries"`
	TopZeroResults []TrendingSearch `json:"top_zero_results"`
}

// DataBreakdown holds agent/human/total search breakdown for the /data page.
type DataBreakdown struct {
	TotalSearches  int            `json:"total_searches"`
	ZeroResultRate float64        `json:"zero_result_rate"`
	BySearcherType map[string]int `json:"by_searcher_type"`
}

// DataCategory holds a category (type_filter) and its search count.
type DataCategory struct {
	Category    string `json:"category"`
	SearchCount int    `json:"search_count"`
}
