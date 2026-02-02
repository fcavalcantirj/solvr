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

// CreatePostRequest is the request body for creating a post
type CreatePostRequest struct {
	Type        string   `json:"type"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Tags        []string `json:"tags,omitempty"`
}

// CreatePostResponse is the response from creating a post
type CreatePostResponse struct {
	Data CreatedPost `json:"data"`
}

// CreatedPost represents a newly created post
type CreatedPost struct {
	ID        string   `json:"id"`
	Type      string   `json:"type"`
	Title     string   `json:"title"`
	Tags      []string `json:"tags,omitempty"`
	Status    string   `json:"status,omitempty"`
	CreatedAt string   `json:"created_at,omitempty"`
}

// validPostTypes are the allowed post types
var validPostTypes = map[string]bool{
	"problem":  true,
	"question": true,
	"idea":     true,
}

// NewPostCmd creates the post command
func NewPostCmd() *cobra.Command {
	var apiURL string
	var apiKey string
	var title string
	var description string
	var tags string
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "post <type>",
		Short: "Create a new post on Solvr",
		Long: `Create a new problem, question, or idea on the Solvr knowledge base.

Valid types: problem, question, idea

Examples:
  solvr post problem --title "Race condition in async code" --description "Details..."
  solvr post question --title "How to fix async bugs?" --description "I have..."
  solvr post idea --title "New approach to caching" --description "What if..."
  solvr post question --title "Title" --description "Content" --tags "go,async,postgres"
  solvr post problem --title "Title" --description "Content" --json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			postType := args[0]

			// Validate post type
			if !validPostTypes[postType] {
				return fmt.Errorf("invalid type '%s': must be one of: problem, question, idea", postType)
			}

			// Validate required fields
			if title == "" {
				return fmt.Errorf("--title is required")
			}
			if description == "" {
				return fmt.Errorf("--description is required")
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

			// Parse tags
			var tagList []string
			if tags != "" {
				for _, tag := range strings.Split(tags, ",") {
					trimmed := strings.TrimSpace(tag)
					if trimmed != "" {
						tagList = append(tagList, trimmed)
					}
				}
			}

			// Build request
			reqBody := CreatePostRequest{
				Type:        postType,
				Title:       title,
				Description: description,
				Tags:        tagList,
			}

			// Marshal to JSON
			reqJSON, err := json.Marshal(reqBody)
			if err != nil {
				return fmt.Errorf("failed to encode request: %w", err)
			}

			// Create HTTP request
			postURL := fmt.Sprintf("%s/posts", apiURL)
			req, err := http.NewRequest("POST", postURL, bytes.NewReader(reqJSON))
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
			var createResp CreatePostResponse
			if err := json.Unmarshal(body, &createResp); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}

			// Output as JSON or pretty display
			if jsonOutput {
				displayPostJSONOutput(cmd, createResp)
			} else {
				displayCreatedPost(cmd, createResp.Data)
			}

			return nil
		},
	}

	// Add flags
	cmd.Flags().StringVar(&apiURL, "api-url", defaultAPIURL, "API base URL")
	cmd.Flags().StringVar(&apiKey, "api-key", "", "API key for authentication")
	cmd.Flags().StringVar(&title, "title", "", "Title of the post (required)")
	cmd.Flags().StringVar(&description, "description", "", "Description/content of the post (required)")
	cmd.Flags().StringVar(&tags, "tags", "", "Comma-separated tags (e.g., 'go,async,postgres')")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output raw JSON response")

	return cmd
}

// displayCreatedPost formats and displays the created post
func displayCreatedPost(cmd *cobra.Command, post CreatedPost) {
	out := cmd.OutOrStdout()

	fmt.Fprintf(out, "Post created successfully!\n\n")
	fmt.Fprintf(out, "ID: %s\n", post.ID)
	fmt.Fprintf(out, "Type: %s\n", post.Type)
	fmt.Fprintf(out, "Title: %s\n", post.Title)

	if len(post.Tags) > 0 {
		fmt.Fprintf(out, "Tags: %v\n", post.Tags)
	}

	if post.Status != "" {
		fmt.Fprintf(out, "Status: %s\n", post.Status)
	}

	fmt.Fprintf(out, "\nView at: solvr get %s\n", post.ID)
}

// displayPostJSONOutput outputs the create response as raw JSON
func displayPostJSONOutput(cmd *cobra.Command, resp CreatePostResponse) {
	out := cmd.OutOrStdout()
	encoder := json.NewEncoder(out)
	encoder.SetIndent("", "  ")
	encoder.Encode(resp)
}
