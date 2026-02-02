package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// CreateAnswerRequest is the request body for creating an answer
type CreateAnswerRequest struct {
	Content string `json:"content"`
}

// CreateAnswerResponse is the response from creating an answer
type CreateAnswerResponse struct {
	Data CreatedAnswer `json:"data"`
}

// CreatedAnswer represents a newly created answer
type CreatedAnswer struct {
	ID         string `json:"id"`
	QuestionID string `json:"question_id"`
	Content    string `json:"content"`
	AuthorType string `json:"author_type,omitempty"`
	AuthorID   string `json:"author_id,omitempty"`
	Upvotes    int    `json:"upvotes"`
	Downvotes  int    `json:"downvotes"`
	IsAccepted bool   `json:"is_accepted"`
	CreatedAt  string `json:"created_at,omitempty"`
}

// NewAnswerCmd creates the answer command
func NewAnswerCmd() *cobra.Command {
	var apiURL string
	var apiKey string
	var content string
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "answer <post_id>",
		Short: "Post an answer to a question on Solvr",
		Long: `Post an answer to a question on the Solvr knowledge base.

Provide the question's post ID and your answer content.

Examples:
  solvr answer question_123 --content "The solution is to use transactions..."
  solvr answer question_123 -c "Short answer here"
  solvr answer question_123 --content "Answer content" --json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			postID := args[0]

			// Validate post_id is provided
			if postID == "" {
				return fmt.Errorf("post_id is required")
			}

			// Validate content is provided and not empty/whitespace
			if content == "" {
				return fmt.Errorf("--content is required")
			}
			if strings.TrimSpace(content) == "" {
				return fmt.Errorf("--content cannot be empty or whitespace only")
			}

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

			// Build request
			reqBody := CreateAnswerRequest{
				Content: content,
			}

			// Marshal to JSON
			reqJSON, err := json.Marshal(reqBody)
			if err != nil {
				return fmt.Errorf("failed to encode request: %w", err)
			}

			// Create HTTP request - post to questions/:id/answers endpoint
			answerURL := fmt.Sprintf("%s/questions/%s/answers", apiURL, postID)
			req, err := http.NewRequest("POST", answerURL, bytes.NewReader(reqJSON))
			if err != nil {
				return fmt.Errorf("failed to create request: %w", err)
			}

			req.Header.Set("Content-Type", "application/json")

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
			if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
				var apiErr APIError
				if json.Unmarshal(body, &apiErr) == nil && apiErr.Error.Message != "" {
					return fmt.Errorf("API error: %s", apiErr.Error.Message)
				}
				return fmt.Errorf("API returned status %d", resp.StatusCode)
			}

			// Parse response
			var answerResp CreateAnswerResponse
			if err := json.Unmarshal(body, &answerResp); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}

			// Output as JSON or pretty display
			if jsonOutput {
				displayAnswerJSONOutput(cmd, answerResp)
			} else {
				displayCreatedAnswer(cmd, answerResp.Data)
			}

			return nil
		},
	}

	// Add flags
	cmd.Flags().StringVar(&apiURL, "api-url", defaultAPIURL, "API base URL")
	cmd.Flags().StringVar(&apiKey, "api-key", "", "API key for authentication")
	cmd.Flags().StringVarP(&content, "content", "c", "", "Answer content (required)")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output raw JSON response")

	return cmd
}

// displayCreatedAnswer formats and displays the created answer
func displayCreatedAnswer(cmd *cobra.Command, answer CreatedAnswer) {
	out := cmd.OutOrStdout()

	fmt.Fprintf(out, "Answer created successfully!\n\n")
	fmt.Fprintf(out, "ID: %s\n", answer.ID)

	if answer.QuestionID != "" {
		fmt.Fprintf(out, "Question ID: %s\n", answer.QuestionID)
	}

	// Show preview of content (first 100 chars)
	contentPreview := answer.Content
	if len(contentPreview) > 100 {
		contentPreview = contentPreview[:100] + "..."
	}
	fmt.Fprintf(out, "Content: %s\n", contentPreview)

	if answer.AuthorType != "" {
		fmt.Fprintf(out, "Author: %s (%s)\n", answer.AuthorID, answer.AuthorType)
	}

	fmt.Fprintf(out, "\nView at: solvr get %s --include answers\n", answer.QuestionID)
}

// displayAnswerJSONOutput outputs the answer response as raw JSON
func displayAnswerJSONOutput(cmd *cobra.Command, resp CreateAnswerResponse) {
	out := cmd.OutOrStdout()
	encoder := json.NewEncoder(out)
	encoder.SetIndent("", "  ")
	encoder.Encode(resp)
}
