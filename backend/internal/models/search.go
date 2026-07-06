// Package models contains data structures for the Solvr API.
package models

import "time"

// SearchResult represents a single search result item.
// This struct is used by the search repository and handler.
type SearchResult struct {
	ID          string   `json:"id"`
	Type        string   `json:"type"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Snippet     string   `json:"snippet"`
	Tags        []string `json:"tags"`
	Status      string   `json:"status"`
	AuthorID    string   `json:"-"` // Internal field
	AuthorType  string   `json:"-"` // Internal field
	AuthorName  string   `json:"-"` // Internal field
	Score       float64  `json:"score"`
	// Similarity is the calibrated cosine similarity (0–1) of this result to the
	// query. Populated only on the hybrid (semantic) posts path; nil for keyword-only
	// paths (fulltext posts, answers, approaches). See BART-155.
	Similarity      *float64   `json:"similarity,omitempty"`
	VoteScore       int        `json:"vote_score"`
	AnswersCount    int        `json:"answers_count"`
	ApproachesCount int        `json:"approaches_count"`
	CommentsCount   int        `json:"comments_count"`
	ViewCount       int        `json:"view_count"`
	CreatedAt       time.Time  `json:"created_at"`
	SolvedAt        *time.Time `json:"solved_at,omitempty"`
	Source          string     `json:"source"` // "post", "answer", or "approach"
}

// SearchResultResponse is the JSON response format for a search result.
// It includes the author as a nested object per SPEC.md Part 5.5.
type SearchResultResponse struct {
	ID              string       `json:"id"`
	Type            string       `json:"type"`
	Title           string       `json:"title"`
	Description     string       `json:"description"`
	Snippet         string       `json:"snippet"`
	Tags            []string     `json:"tags"`
	Status          string       `json:"status"`
	Author          SearchAuthor `json:"author"`
	Score           float64      `json:"score"`
	Similarity      *float64     `json:"similarity,omitempty"` // cosine similarity 0–1; semantic path only (BART-155)
	VoteScore       int          `json:"vote_score"`
	AnswersCount    int          `json:"answers_count"`
	ApproachesCount int          `json:"approaches_count"`
	CommentsCount   int          `json:"comments_count"`
	ViewCount       int          `json:"view_count"`
	CreatedAt       time.Time    `json:"created_at"`
	SolvedAt        *time.Time   `json:"solved_at,omitempty"`
	Source          string       `json:"source"` // "post", "answer", or "approach"
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
	ViewerHuman  string    // Caller's family human UUID for visibility scoping ("" = public-only)
	// MinSimilarity is an OPT-IN cosine-similarity floor (0–1). When > 0, results are
	// filtered to those whose Similarity >= MinSimilarity (nil-similarity/keyword-only
	// results are dropped), yielding an honest empty below the bar. 0 = no filter
	// (full recall — the default). See BART-155.
	MinSimilarity float64
}

// ToResponse converts a SearchResult to a SearchResultResponse.
func (r *SearchResult) ToResponse() SearchResultResponse {
	return SearchResultResponse{
		ID:          r.ID,
		Type:        r.Type,
		Title:       r.Title,
		Description: r.Description,
		Snippet:     r.Snippet,
		Tags:        r.Tags,
		Status:      r.Status,
		Author: SearchAuthor{
			ID:          r.AuthorID,
			Type:        r.AuthorType,
			DisplayName: r.AuthorName,
		},
		Score:           r.Score,
		Similarity:      r.Similarity,
		VoteScore:       r.VoteScore,
		AnswersCount:    r.AnswersCount,
		ApproachesCount: r.ApproachesCount,
		CommentsCount:   r.CommentsCount,
		ViewCount:       r.ViewCount,
		CreatedAt:       r.CreatedAt,
		SolvedAt:        r.SolvedAt,
		Source:          r.Source,
	}
}

// IsConfidentMatch reports whether the best semantic match clears the confidence
// threshold. It is the server's ASK-biased "answered?" signal (BART-155): a nil
// topSimilarity (no semantic measure available) is never confident. Used by both the
// REST and MCP handlers so the decision never drifts between them.
func IsConfidentMatch(topSimilarity *float64, threshold float64) bool {
	return topSimilarity != nil && *topSimilarity >= threshold
}
