package main

import (
	"encoding/json"
	"fmt"
	"net/url"
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

Contributions to the post (answers, approaches, reviews, discussion) are
replies: list them with "solvr replies <id>". No API key is needed; a
configured one is sent.

Examples:
  solvr get post-123
  solvr get post-123 --api-key solvr_xxx
  solvr get post-123 --api-url http://localhost:8080/v1
  solvr get post-123 --json
  solvr replies post-123`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			postID := args[0]
			apiURL, apiKey = resolveAPISettings(apiURL, apiKey)

			body, err := callAPI("GET", fmt.Sprintf("%s/posts/%s", apiURL, url.PathEscape(postID)), apiKey, nil)
			if err != nil {
				return err
			}

			// --json prints the answer exactly as the API returned it
			if jsonOutput {
				return printAnswer(cmd.OutOrStdout(), body)
			}

			var getResp GetAPIResponse
			if err := json.Unmarshal(body, &getResp); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}
			displayPostDetails(cmd, getResp.Data)
			fmt.Fprintf(cmd.OutOrStdout(), "\nReplies: solvr replies %s\n", getResp.Data.ID)
			return nil
		},
	}

	cmd.Flags().StringVar(&apiURL, "api-url", defaultAPIURL, "API base URL")
	cmd.Flags().StringVar(&apiKey, "api-key", "", "API key for authentication")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output the API's answer as JSON")
	removeFlag(cmd, "include", "read the replies with: solvr replies <id>")

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

// truncateString truncates a string to a maximum length
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
