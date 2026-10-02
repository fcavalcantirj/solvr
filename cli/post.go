package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// CreatePostRequest is the request body for creating a post. A post has no
// type: problems, questions and ideas are all posts.
type CreatePostRequest struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Tags        []string `json:"tags,omitempty"`
	Visibility  string   `json:"visibility,omitempty"`
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

// rejectTypeArgument refuses the old "solvr post <type>" form before any
// prompt or request: posts take no type.
func rejectTypeArgument(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return nil
	}
	cmd.SilenceUsage = true
	return removedError("solvr post <type>",
		fmt.Sprintf(`posts take no type (%q is not accepted): solvr post --title "..." --description "..."`, args[0]))
}

// NewPostCmd creates the post command
func NewPostCmd() *cobra.Command {
	var apiURL string
	var apiKey string
	var title string
	var description string
	var tags string
	var visibility string
	var jsonOutput bool
	var interactive bool

	cmd := &cobra.Command{
		Use:   "post",
		Short: "Create a new post on Solvr",
		Long: `Create a new post on the Solvr knowledge base.

A post is a problem, question, idea or finding worth sharing; there is no
type to choose. Contributions to it are replies (see "solvr reply").

Use --interactive (-i) to be prompted for missing fields.

Examples:
  solvr post --title "Race condition in async code" --description "Details..."
  solvr post --title "Title" --description "Content" --tags "go,async,postgres"
  solvr post --title "Title" --description "Content" --visibility family
  solvr post --title "Title" --description "Content" --json
  solvr post --interactive  # Prompts for all fields`,
		Args: rejectTypeArgument,
		RunE: func(cmd *cobra.Command, args []string) error {
			if interactive {
				title, description, tags = runInteractiveMode(cmd, title, description, tags)
			}

			if title == "" {
				return fmt.Errorf("--title is required")
			}
			if description == "" {
				return fmt.Errorf("--description is required")
			}

			apiURL, apiKey = resolveAPISettings(apiURL, apiKey)

			var tagList []string
			if tags != "" {
				for _, tag := range strings.Split(tags, ",") {
					trimmed := strings.TrimSpace(tag)
					if trimmed != "" {
						tagList = append(tagList, trimmed)
					}
				}
			}

			respBody, err := callAPI("POST", fmt.Sprintf("%s/posts", apiURL), apiKey, CreatePostRequest{
				Title:       title,
				Description: description,
				Tags:        tagList,
				Visibility:  visibility,
			})
			if err != nil {
				return err
			}

			// --json prints the answer exactly as the API returned it
			if jsonOutput {
				return printAnswer(cmd.OutOrStdout(), respBody)
			}

			var createResp CreatePostResponse
			if err := json.Unmarshal(respBody, &createResp); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}
			displayCreatedPost(cmd, createResp.Data)
			return nil
		},
	}

	// Add flags
	cmd.Flags().StringVar(&apiURL, "api-url", defaultAPIURL, "API base URL")
	cmd.Flags().StringVar(&apiKey, "api-key", "", "API key for authentication")
	cmd.Flags().StringVar(&title, "title", "", "Title of the post (required unless --interactive)")
	cmd.Flags().StringVar(&description, "description", "", "Description/content of the post (required unless --interactive)")
	cmd.Flags().StringVar(&tags, "tags", "", "Comma-separated tags (e.g., 'go,async,postgres')")
	cmd.Flags().StringVar(&visibility, "visibility", "", "Who can read the post: public (default) or family")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output the API's answer as JSON")
	cmd.Flags().BoolVarP(&interactive, "interactive", "i", false, "Prompt for missing fields interactively")

	return cmd
}

// displayCreatedPost formats and displays the created post
func displayCreatedPost(cmd *cobra.Command, post CreatedPost) {
	out := cmd.OutOrStdout()

	fmt.Fprintf(out, "Post created successfully!\n\n")
	fmt.Fprintf(out, "ID: %s\n", post.ID)
	fmt.Fprintf(out, "Title: %s\n", post.Title)

	if len(post.Tags) > 0 {
		fmt.Fprintf(out, "Tags: %v\n", post.Tags)
	}

	if post.Status != "" {
		fmt.Fprintf(out, "Status: %s\n", post.Status)
	}

	fmt.Fprintf(out, "\nView at: solvr get %s\n", post.ID)
}

// runInteractiveMode prompts for missing fields interactively
func runInteractiveMode(cmd *cobra.Command, title, description, tags string) (string, string, string) {
	out := cmd.OutOrStdout()
	reader := bufio.NewReader(cmd.InOrStdin())

	if title == "" {
		fmt.Fprint(out, "Title: ")
		input, _ := reader.ReadString('\n')
		title = strings.TrimSpace(input)
	}

	if description == "" {
		fmt.Fprint(out, "Description: ")
		input, _ := reader.ReadString('\n')
		description = strings.TrimSpace(input)
	}

	if tags == "" {
		fmt.Fprint(out, "Tags (comma-separated, optional): ")
		input, _ := reader.ReadString('\n')
		tags = strings.TrimSpace(input)
	}

	return title, description, tags
}
