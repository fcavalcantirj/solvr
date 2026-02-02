package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// openEditor is a variable function to allow mocking in tests
var openEditor = openEditorImpl

// openEditorImpl opens the user's configured editor with the given file
func openEditorImpl(path string) error {
	editorCmd := getEditorCommand()
	if editorCmd == "" {
		return fmt.Errorf("no editor configured: set EDITOR or VISUAL environment variable")
	}

	cmd := exec.Command(editorCmd, path)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// getEditorCommand returns the editor command from environment variables
// Prefers VISUAL over EDITOR, returns empty string if neither is set
func getEditorCommand() string {
	if editor := os.Getenv("VISUAL"); editor != "" {
		return editor
	}
	if editor := os.Getenv("EDITOR"); editor != "" {
		return editor
	}
	return ""
}

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
	var useEditor bool

	cmd := &cobra.Command{
		Use:   "answer <post_id>",
		Short: "Post an answer to a question on Solvr",
		Long: `Post an answer to a question on the Solvr knowledge base.

Provide the question's post ID and your answer content.

You can provide content via --content flag or use --editor to open your
configured text editor ($VISUAL or $EDITOR environment variable).

Examples:
  solvr answer question_123 --content "The solution is to use transactions..."
  solvr answer question_123 -c "Short answer here"
  solvr answer question_123 --editor              # Opens $EDITOR
  solvr answer question_123 -e                    # Short form
  solvr answer question_123 --content "Answer content" --json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			postID := args[0]

			// Validate post_id is provided
			if postID == "" {
				return fmt.Errorf("post_id is required")
			}

			// If --content is provided, use it directly (ignore --editor)
			// If --editor is provided and no --content, open editor
			// If neither, return error
			if content == "" {
				if useEditor {
					var err error
					content, err = getContentFromEditor()
					if err != nil {
						return err
					}
				} else {
					return fmt.Errorf("--content is required (or use --editor to open your editor)")
				}
			}

			// Validate content is not empty/whitespace
			if strings.TrimSpace(content) == "" {
				return fmt.Errorf("answer content cannot be empty or whitespace only")
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
	cmd.Flags().StringVarP(&content, "content", "c", "", "Answer content (required unless --editor)")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output raw JSON response")
	cmd.Flags().BoolVarP(&useEditor, "editor", "e", false, "Open $EDITOR to write answer content")

	return cmd
}

// getContentFromEditor opens the user's editor and returns the content written
func getContentFromEditor() (string, error) {
	// Create a temporary file
	tmpFile, err := os.CreateTemp("", "solvr-answer-*.md")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	// Write a helpful comment to the temp file
	initialContent := `# Write your answer below this line
# Lines starting with # will be ignored
# Save and exit to submit, or leave empty to abort

`
	if _, err := tmpFile.WriteString(initialContent); err != nil {
		tmpFile.Close()
		return "", fmt.Errorf("failed to write to temp file: %w", err)
	}
	tmpFile.Close()

	// Open the editor
	if err := openEditor(tmpPath); err != nil {
		return "", fmt.Errorf("failed to open editor: %w", err)
	}

	// Read the content back
	contentBytes, err := os.ReadFile(tmpPath)
	if err != nil {
		return "", fmt.Errorf("failed to read temp file: %w", err)
	}

	// Filter out comment lines (lines starting with #)
	var contentLines []string
	for _, line := range strings.Split(string(contentBytes), "\n") {
		if !strings.HasPrefix(strings.TrimSpace(line), "#") {
			contentLines = append(contentLines, line)
		}
	}

	content := strings.TrimSpace(strings.Join(contentLines, "\n"))

	if content == "" {
		return "", fmt.Errorf("aborting: answer content is empty")
	}

	return content, nil
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
