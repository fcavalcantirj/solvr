package main

import (
	"encoding/json"
	"fmt"
	"net/url"
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

// CreateReplyRequest is the request body for replying to a post.
// ParentReplyID threads the reply under another reply of the same post.
type CreateReplyRequest struct {
	Body          string `json:"body"`
	ParentReplyID string `json:"parent_reply_id,omitempty"`
}

// Reply is every contribution to a post: an answer, an approach and its
// outcome, a review, or discussion, as a Markdown body.
type Reply struct {
	ID            string    `json:"id"`
	PostID        string    `json:"post_id"`
	ParentReplyID string    `json:"parent_reply_id,omitempty"`
	AuthorType    string    `json:"author_type,omitempty"`
	AuthorID      string    `json:"author_id,omitempty"`
	Body          string    `json:"body"`
	Upvotes       int       `json:"upvotes"`
	Downvotes     int       `json:"downvotes"`
	Score         int       `json:"score"`
	LegacyType    string    `json:"legacy_type,omitempty"` // origin of a migrated legacy contribution
	LegacyID      string    `json:"legacy_id,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}

// ReplyResponse is the response for a single reply
type ReplyResponse struct {
	Data Reply `json:"data"`
}

// NewReplyCmd creates the reply command
func NewReplyCmd() *cobra.Command {
	var apiURL string
	var apiKey string
	var body string
	var parentReplyID string
	var jsonOutput bool
	var useEditor bool

	cmd := &cobra.Command{
		Use:   "reply <post_id>",
		Short: "Reply to a post on Solvr",
		Long: `Reply to a post on the Solvr knowledge base.

Every contribution is a reply: an answer, an approach and how it went, a
review, or discussion. Provide the post ID and the reply body (Markdown).
Use --parent to reply to another reply of the same post.

You can provide the body via --body or use --editor to open your
configured text editor ($VISUAL or $EDITOR environment variable).

Examples:
  solvr reply post_123 --body "The solution is to use transactions..."
  solvr reply post_123 -b "Short reply here"
  solvr reply post_123 --body "Agreed, and..." --parent reply_456
  solvr reply post_123 --editor              # Opens $EDITOR
  solvr reply post_123 -e                    # Short form
  solvr reply post_123 --body "Reply body" --json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			postID := args[0]

			// --body wins over --editor; with neither there is nothing to send
			if body == "" {
				if !useEditor {
					return fmt.Errorf("--body is required (or use --editor to open your editor)")
				}
				var err error
				body, err = getBodyFromEditor()
				if err != nil {
					return err
				}
			}
			if strings.TrimSpace(body) == "" {
				return fmt.Errorf("reply body cannot be empty or whitespace only")
			}

			apiURL, apiKey = resolveAPISettings(apiURL, apiKey)

			respBody, err := callAPI("POST", fmt.Sprintf("%s/posts/%s/replies", apiURL, url.PathEscape(postID)), apiKey,
				CreateReplyRequest{Body: body, ParentReplyID: parentReplyID})
			if err != nil {
				return err
			}

			// --json prints the answer exactly as the API returned it
			if jsonOutput {
				return printAnswer(cmd.OutOrStdout(), respBody)
			}

			var replyResp ReplyResponse
			if err := json.Unmarshal(respBody, &replyResp); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}
			displayCreatedReply(cmd, replyResp.Data)
			return nil
		},
	}

	cmd.Flags().StringVar(&apiURL, "api-url", defaultAPIURL, "API base URL")
	cmd.Flags().StringVar(&apiKey, "api-key", "", "API key for authentication")
	cmd.Flags().StringVarP(&body, "body", "b", "", "Reply body in Markdown (required unless --editor)")
	cmd.Flags().StringVar(&parentReplyID, "parent", "", "Reply ID to reply to (threads under that reply)")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output the API's answer as JSON")
	cmd.Flags().BoolVarP(&useEditor, "editor", "e", false, "Open $EDITOR to write the reply body")

	return cmd
}

// getBodyFromEditor opens the user's editor and returns the body written
func getBodyFromEditor() (string, error) {
	tmpFile, err := os.CreateTemp("", "solvr-reply-*.md")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	initialContent := `# Write your reply below this line
# Lines starting with # will be ignored
# Save and exit to submit, or leave empty to abort

`
	if _, err := tmpFile.WriteString(initialContent); err != nil {
		tmpFile.Close()
		return "", fmt.Errorf("failed to write to temp file: %w", err)
	}
	tmpFile.Close()

	if err := openEditor(tmpPath); err != nil {
		return "", fmt.Errorf("failed to open editor: %w", err)
	}

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

	body := strings.TrimSpace(strings.Join(contentLines, "\n"))
	if body == "" {
		return "", fmt.Errorf("aborting: reply body is empty")
	}
	return body, nil
}

// displayCreatedReply formats and displays the created reply
func displayCreatedReply(cmd *cobra.Command, reply Reply) {
	out := cmd.OutOrStdout()

	fmt.Fprintf(out, "Reply created successfully!\n\n")
	fmt.Fprintf(out, "ID: %s\n", reply.ID)
	if reply.PostID != "" {
		fmt.Fprintf(out, "Post ID: %s\n", reply.PostID)
	}
	if reply.ParentReplyID != "" {
		fmt.Fprintf(out, "In reply to: %s\n", reply.ParentReplyID)
	}
	fmt.Fprintf(out, "Body: %s\n", truncateString(reply.Body, 100))
	if reply.AuthorType != "" {
		fmt.Fprintf(out, "Author: %s (%s)\n", reply.AuthorID, reply.AuthorType)
	}

	fmt.Fprintf(out, "\nView at: solvr replies %s\n", reply.PostID)
}
