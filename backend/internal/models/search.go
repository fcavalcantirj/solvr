// Package models contains data structures for the Solvr API.
package models

import "time"

// SearchResult represents a single search result item.
// This struct is used by the search repository and handler.
type SearchResult struct {
	ID           string    `json:"id"`
	Type         string    `json:"type"`
	Title        string    `json:"title"`
	Snippet      string    `json:"snippet"`
	Tags         []string  `json:"tags"`
	Status       string    `json:"status"`
	AuthorID     string    `json:"-"` // Internal field
	AuthorType   string    `json:"-"` // Internal field
	AuthorName   string    `json:"-"` // Internal field
	Score        float64   `json:"score"`
	VoteScore    int       `json:"vote_score"`
	AnswersCount int       `json:"answers_count"`
	CreatedAt    time.Time `json:"created_at"`
	SolvedAt     *time.Time `json:"solved_at,omitempty"`
	Source       string    `json:"source"` // "post", "answer", or "approach"
}

// SearchResultResponse is the JSON response format for a search result.
// It includes the author as a nested object per SPEC.md Part 5.5.
type SearchResultResponse struct {
	ID           string        `json:"id"`
	Type         string        `json:"type"`
	Title        string        `json:"title"`
	Snippet      string        `json:"snippet"`
	Tags         []string      `json:"tags"`
	Status       string        `json:"status"`
	Author       SearchAuthor  `json:"author"`
	Score        float64       `json:"score"`
	VoteScore    int           `json:"vote_score"`
	AnswersCount int           `json:"answers_count"`
	CreatedAt    time.Time     `json:"created_at"`
	SolvedAt     *time.Time    `json:"solved_at,omitempty"`
	Source       string        `json:"source"` // "post", "answer", or "approach"
}

// SearchAuthor represents the author info in search results.
type SearchAuthor struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	DisplayName string `json:"display_name"`
}

// SearchOptions represents all available search filters and options.
type SearchOptions struct {
	Type         string    // Filter by post type (problem, question, idea)
	Tags         []string  // Filter by tags
	Status       string    // Filter by status
	Author       string    // Filter by author_id
	AuthorType   string    // Filter by author_type (human, agent)
	FromDate     time.Time // Filter posts created after this date
	ToDate       time.Time // Filter posts created before this date
	Sort         string    // Sort order (relevance, newest, votes, activity)
	Page         int       // Page number (1-indexed)
	PerPage      int       // Results per page
	ContentTypes []string  // Filter by content source: "posts", "answers", "approaches" (default: all)
}

// ToResponse converts a SearchResult to a SearchResultResponse.
func (r *SearchResult) ToResponse() SearchResultResponse {
	return SearchResultResponse{
		ID:           r.ID,
		Type:         r.Type,
		Title:        r.Title,
		Snippet:      r.Snippet,
		Tags:         r.Tags,
		Status:       r.Status,
		Author: SearchAuthor{
			ID:          r.AuthorID,
			Type:        r.AuthorType,
			DisplayName: r.AuthorName,
		},
		Score:        r.Score,
		VoteScore:    r.VoteScore,
		AnswersCount: r.AnswersCount,
		CreatedAt:    r.CreatedAt,
		SolvedAt:     r.SolvedAt,
		Source:       r.Source,
	}
}
