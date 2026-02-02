package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
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

	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search the Solvr knowledge base",
		Long: `Search the Solvr knowledge base for existing solutions, questions, and ideas.

Search before you start working on a problem - someone might have already solved it!

Examples:
  solvr search "async postgres race condition"
  solvr search "ECONNREFUSED" --api-key solvr_xxx
  solvr search "error handling" --api-url http://localhost:8080/v1
  solvr search "async bug" --json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			query := args[0]

			// Try to load API key from config if not provided via flag
			if apiKey == "" {
				config, err := loadConfig()
				if err == nil {
					if key, ok := config["api-key"]; ok {
						apiKey = key
					}
				}
			}

			// Try to load API URL from config if not overridden
			if apiURL == defaultAPIURL {
				config, err := loadConfig()
				if err == nil {
					if url, ok := config["api-url"]; ok {
						apiURL = url
					}
				}
			}

			// Build search URL
			searchURL, err := buildSearchURL(apiURL, query)
			if err != nil {
				return fmt.Errorf("failed to build search URL: %w", err)
			}

			// Create HTTP request
			req, err := http.NewRequest("GET", searchURL, nil)
			if err != nil {
				return fmt.Errorf("failed to create request: %w", err)
			}

			// Add auth header if API key is set
			if apiKey != "" {
				req.Header.Set("Authorization", "Bearer "+apiKey)
			}

			// Execute request
			client := &http.Client{Timeout: 30 * time.Second}
			resp, err := client.Do(req)
			if err != nil {
				return fmt.Errorf("failed to call API: %w", err)
			}
			defer resp.Body.Close()

			// Read response body
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				return fmt.Errorf("failed to read response: %w", err)
			}

			// Check for error response
			if resp.StatusCode != http.StatusOK {
				var apiErr APIError
				if json.Unmarshal(body, &apiErr) == nil && apiErr.Error.Message != "" {
					return fmt.Errorf("API error: %s", apiErr.Error.Message)
				}
				return fmt.Errorf("API returned status %d", resp.StatusCode)
			}

			// Parse response
			var searchResp SearchAPIResponse
			if err := json.Unmarshal(body, &searchResp); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}

			// Output as JSON or pretty display
			if jsonOutput {
				displayJSONOutput(cmd, searchResp)
			} else {
				displaySearchResults(cmd, searchResp)
			}

			return nil
		},
	}

	// Add flags
	cmd.Flags().StringVar(&apiURL, "api-url", defaultAPIURL, "API base URL")
	cmd.Flags().StringVar(&apiKey, "api-key", "", "API key for authentication")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output raw JSON")

	return cmd
}

// buildSearchURL constructs the search API URL with query parameters
func buildSearchURL(baseURL, query string) (string, error) {
	u, err := url.Parse(baseURL + "/search")
	if err != nil {
		return "", err
	}

	q := u.Query()
	q.Set("q", query)
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

// displayJSONOutput outputs the search response as raw JSON
func displayJSONOutput(cmd *cobra.Command, resp SearchAPIResponse) {
	out := cmd.OutOrStdout()
	encoder := json.NewEncoder(out)
	encoder.SetIndent("", "  ")
	encoder.Encode(resp)
}
