package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"time"

	"github.com/spf13/cobra"
)

// Default API URL
const defaultAPIURL = "https://api.solvr.dev/v1"

// SearchAPIResponse matches the backend search response format
type SearchAPIResponse struct {
	Data []SearchResult `json:"data"`
	Meta SearchMeta     `json:"meta"`
}

// SearchResult represents a single search result
type SearchResult struct {
	ID           string     `json:"id"`
	Type         string     `json:"type"`
	Title        string     `json:"title"`
	Snippet      string     `json:"snippet"`
	Tags         []string   `json:"tags"`
	Status       string     `json:"status"`
	Author       AuthorInfo `json:"author"`
	Score        float64    `json:"score"`
	Votes        int        `json:"votes"`
	AnswersCount int        `json:"answers_count"`
	CreatedAt    time.Time  `json:"created_at"`
	SolvedAt     *time.Time `json:"solved_at,omitempty"`
}

// AuthorInfo represents author information in search results
type AuthorInfo struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	DisplayName string `json:"display_name"`
}

// SearchMeta contains metadata about the search response
type SearchMeta struct {
	Query   string `json:"query"`
	Total   int    `json:"total"`
	Page    int    `json:"page"`
	PerPage int    `json:"per_page"`
	HasMore bool   `json:"has_more"`
	TookMs  int64  `json:"took_ms"`
}

// APIError represents an API error response
type APIError struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// NewSearchCmd creates the search command
func NewSearchCmd() *cobra.Command {
	var apiURL string
	var apiKey string
	var jsonOutput bool
	var typeFilter string
	var limit int
	var page int
	var sort string

	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search the Solvr knowledge base",
		Long: `Search the Solvr knowledge base for existing solutions, questions, and ideas.

Search before you start working on a problem - someone might have already solved it!
No API key is needed; a configured one is sent.

Examples:
  solvr search "async postgres race condition"
  solvr search "ECONNREFUSED" --api-key solvr_xxx
  solvr search "error handling" --api-url http://localhost:8080/v1
  solvr search "async bug" --json
  solvr search "bug fix" --type problem
  solvr search "test" --limit 5 --page 2
  solvr search "retry" --sort newest`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			apiURL, apiKey = resolveAPISettings(apiURL, apiKey)

			searchURL, err := buildSearchURL(apiURL, args[0], typeFilter, limit, page, sort)
			if err != nil {
				return fmt.Errorf("failed to build search URL: %w", err)
			}

			body, err := callAPI("GET", searchURL, apiKey, nil)
			if err != nil {
				return err
			}

			// --json prints the answer exactly as the API returned it
			if jsonOutput {
				return printAnswer(cmd.OutOrStdout(), body)
			}

			var searchResp SearchAPIResponse
			if err := json.Unmarshal(body, &searchResp); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}
			displaySearchResults(cmd, searchResp)
			return nil
		},
	}

	// Add flags
	cmd.Flags().StringVar(&apiURL, "api-url", defaultAPIURL, "API base URL")
	cmd.Flags().StringVar(&apiKey, "api-key", "", "API key for authentication")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output the API's answer as JSON")
	cmd.Flags().StringVar(&typeFilter, "type", "", "Filter by type: problem, question, idea, or all")
	cmd.Flags().IntVar(&limit, "limit", 0, "Results per page (1-50; the API's default when not given)")
	cmd.Flags().IntVar(&page, "page", 0, "Page number (the API's default when not given)")
	cmd.Flags().StringVar(&sort, "sort", "", "relevance (default), newest, votes or activity")

	return cmd
}

// buildSearchURL constructs the search API URL with only the parameters given:
// an empty query sends no q (the API answers VALIDATION_ERROR).
func buildSearchURL(baseURL, query, typeFilter string, limit, page int, sort string) (string, error) {
	u, err := url.Parse(baseURL + "/search")
	if err != nil {
		return "", err
	}

	q := u.Query()
	if query != "" {
		q.Set("q", query)
	}
	if typeFilter != "" {
		q.Set("type", typeFilter)
	}
	if limit > 0 {
		q.Set("per_page", strconv.Itoa(limit))
	}
	if page > 0 {
		q.Set("page", strconv.Itoa(page))
	}
	if sort != "" {
		q.Set("sort", sort)
	}
	u.RawQuery = q.Encode()

	return u.String(), nil
}

// displaySearchResults formats and displays search results
func displaySearchResults(cmd *cobra.Command, resp SearchAPIResponse) {
	out := cmd.OutOrStdout()

	if len(resp.Data) == 0 {
		fmt.Fprintf(out, "No results found for '%s'\n", resp.Meta.Query)
		return
	}

	fmt.Fprintf(out, "Found %d result(s) for '%s' (%dms)\n\n", resp.Meta.Total, resp.Meta.Query, resp.Meta.TookMs)

	for i, result := range resp.Data {
		// Type badge
		typeBadge := fmt.Sprintf("[%s]", result.Type)

		// Status indicator
		statusIcon := ""
		switch result.Status {
		case "solved", "answered":
			statusIcon = "✓"
		case "open":
			statusIcon = "○"
		case "stuck":
			statusIcon = "!"
		}

		// Display result
		fmt.Fprintf(out, "%d. %s %s %s\n", i+1, typeBadge, result.Title, statusIcon)
		fmt.Fprintf(out, "   ID: %s | Score: %.2f | Votes: %d | Answers: %d\n",
			result.ID, result.Score, result.Votes, result.AnswersCount)

		// Tags
		if len(result.Tags) > 0 {
			fmt.Fprintf(out, "   Tags: %v\n", result.Tags)
		}

		// Author
		fmt.Fprintf(out, "   By: %s (%s)\n", result.Author.DisplayName, result.Author.Type)

		// Snippet (strip HTML tags for display)
		if result.Snippet != "" {
			snippet := stripHTMLTags(result.Snippet)
			if len(snippet) > 100 {
				snippet = snippet[:100] + "..."
			}
			fmt.Fprintf(out, "   %s\n", snippet)
		}

		fmt.Fprintln(out)
	}

	if resp.Meta.HasMore {
		fmt.Fprintf(out, "Showing page %d of results. More results available.\n", resp.Meta.Page)
	}
}

// stripHTMLTags removes HTML tags from a string (simple implementation)
func stripHTMLTags(s string) string {
	var result []byte
	inTag := false
	for i := 0; i < len(s); i++ {
		if s[i] == '<' {
			inTag = true
			continue
		}
		if s[i] == '>' {
			inTag = false
			continue
		}
		if !inTag {
			result = append(result, s[i])
		}
	}
	return string(result)
}
