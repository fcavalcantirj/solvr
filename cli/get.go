package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/spf13/cobra"
)

// GetAPIResponse matches the backend get post response format
type GetAPIResponse struct {
	Data PostDetail `json:"data"`
}

// PostDetail represents a single post with full details
type PostDetail struct {
	ID               string     `json:"id"`
	Type             string     `json:"type"`
	Title            string     `json:"title"`
	Description      string     `json:"description"`
	Tags             []string   `json:"tags"`
	Status           string     `json:"status"`
	Author           AuthorInfo `json:"author"`
	Upvotes          int        `json:"upvotes"`
	Downvotes        int        `json:"downvotes"`
	VoteScore        int        `json:"vote_score"`
	SuccessCriteria  []string   `json:"success_criteria,omitempty"`
	Weight           *int       `json:"weight,omitempty"`
	AcceptedAnswerID *string    `json:"accepted_answer_id,omitempty"`
	EvolvedInto      []string   `json:"evolved_into,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

// NewGetCmd creates the get command
func NewGetCmd() *cobra.Command {
	var apiURL string
	var apiKey string
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "get <id>",
		Short: "Get details of a Solvr post",
		Long: `Get the full details of a post from the Solvr knowledge base.

Use this command to view the complete content of a problem, question, or idea.

Examples:
  solvr get post-123
  solvr get post-123 --api-key solvr_xxx
  solvr get post-123 --api-url http://localhost:8080/v1
  solvr get post-123 --json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			postID := args[0]

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

			// Build get URL
			getURL := fmt.Sprintf("%s/posts/%s", apiURL, postID)

			// Create HTTP request
			req, err := http.NewRequest("GET", getURL, nil)
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
			var getResp GetAPIResponse
			if err := json.Unmarshal(body, &getResp); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}

			// Output as JSON or pretty display
			if jsonOutput {
				displayGetJSONOutput(cmd, getResp)
			} else {
				displayPostDetails(cmd, getResp.Data)
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

// displayPostDetails formats and displays post details
func displayPostDetails(cmd *cobra.Command, post PostDetail) {
	out := cmd.OutOrStdout()

	// Type badge and title
	fmt.Fprintf(out, "[%s] %s\n", post.Type, post.Title)
	fmt.Fprintf(out, "ID: %s\n", post.ID)
	fmt.Fprintf(out, "Status: %s\n", post.Status)

	// Votes
	fmt.Fprintf(out, "Votes: %d (↑%d ↓%d)\n", post.VoteScore, post.Upvotes, post.Downvotes)

	// Author
	fmt.Fprintf(out, "Author: %s (%s)\n", post.Author.DisplayName, post.Author.Type)

	// Tags
	if len(post.Tags) > 0 {
		fmt.Fprintf(out, "Tags: %v\n", post.Tags)
	}

	// Problem-specific fields
	if post.Type == "problem" {
		if post.Weight != nil {
			fmt.Fprintf(out, "Weight: %d/5\n", *post.Weight)
		}
		if len(post.SuccessCriteria) > 0 {
			fmt.Fprintln(out, "Success Criteria:")
			for i, criterion := range post.SuccessCriteria {
				fmt.Fprintf(out, "  %d. %s\n", i+1, criterion)
			}
		}
	}

	// Timestamps
	fmt.Fprintf(out, "Created: %s\n", post.CreatedAt.Format(time.RFC3339))
	fmt.Fprintf(out, "Updated: %s\n", post.UpdatedAt.Format(time.RFC3339))

	// Description
	fmt.Fprintln(out, "\n--- Description ---")
	fmt.Fprintln(out, post.Description)
}

// displayGetJSONOutput outputs the get response as raw JSON
func displayGetJSONOutput(cmd *cobra.Command, resp GetAPIResponse) {
	out := cmd.OutOrStdout()
	encoder := json.NewEncoder(out)
	encoder.SetIndent("", "  ")
	encoder.Encode(resp)
}
