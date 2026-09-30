package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

// RepliesMeta is the cursor pagination metadata of a reply list
type RepliesMeta struct {
	Total      int    `json:"total"`
	HasMore    bool   `json:"has_more"`
	NextCursor string `json:"next_cursor,omitempty"`
}

// RepliesResponse is one page of the replies of a post
type RepliesResponse struct {
	Data []Reply     `json:"data"`
	Meta RepliesMeta `json:"meta"`
}

// NewRepliesCmd creates the replies command
func NewRepliesCmd() *cobra.Command {
	var apiURL string
	var apiKey string
	var cursor string
	var limit int
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "replies <post_id>",
		Short: "List the replies of a Solvr post",
		Long: `List the replies of a post, oldest first, one page at a time.

Answers, approaches and responses from before the canonical model are
replies too; they are labeled with their origin.

Examples:
  solvr replies post_123
  solvr replies post_123 --limit 10
  solvr replies post_123 --cursor <next_cursor>   # The following page
  solvr replies post_123 --json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			postID := args[0]
			apiURL, apiKey = resolveAPISettings(apiURL, apiKey)

			params := url.Values{}
			if cursor != "" {
				params.Set("cursor", cursor)
			}
			if limit > 0 {
				params.Set("limit", strconv.Itoa(limit))
			}
			listURL := fmt.Sprintf("%s/posts/%s/replies", apiURL, postID)
			if len(params) > 0 {
				listURL += "?" + params.Encode()
			}

			respBody, err := callAPI("GET", listURL, apiKey, nil)
			if err != nil {
				return err
			}

			// --json prints the page exactly as the API returned it
			if jsonOutput {
				var indented bytes.Buffer
				if err := json.Indent(&indented, respBody, "", "  "); err != nil {
					return fmt.Errorf("failed to parse response: %w", err)
				}
				fmt.Fprintln(cmd.OutOrStdout(), indented.String())
				return nil
			}

			var page RepliesResponse
			if err := json.Unmarshal(respBody, &page); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}
			displayReplies(cmd, postID, page)
			return nil
		},
	}

	cmd.Flags().StringVar(&apiURL, "api-url", defaultAPIURL, "API base URL")
	cmd.Flags().StringVar(&apiKey, "api-key", "", "API key for authentication")
	cmd.Flags().StringVar(&cursor, "cursor", "", "Next-page cursor from a previous page")
	cmd.Flags().IntVar(&limit, "limit", 0, "Replies per page (server default 50, maximum 100)")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output raw JSON")

	return cmd
}

// displayReplies formats one page of replies
func displayReplies(cmd *cobra.Command, postID string, page RepliesResponse) {
	out := cmd.OutOrStdout()

	if len(page.Data) == 0 {
		fmt.Fprintln(out, "No replies yet.")
		return
	}

	if page.Meta.HasMore || len(page.Data) < page.Meta.Total {
		fmt.Fprintf(out, "Showing %d of %d replies\n", len(page.Data), page.Meta.Total)
	} else if page.Meta.Total == 1 {
		fmt.Fprintln(out, "1 reply")
	} else {
		fmt.Fprintf(out, "%d replies\n", page.Meta.Total)
	}

	for i, reply := range page.Data {
		header := fmt.Sprintf("\n%d. %s", i+1, reply.ID)
		if reply.LegacyType != "" {
			header += fmt.Sprintf(" [migrated %s]", reply.LegacyType)
		}
		if reply.ParentReplyID != "" {
			header += fmt.Sprintf(" (in reply to %s)", reply.ParentReplyID)
		}
		fmt.Fprintln(out, header)
		fmt.Fprintf(out, "   By: %s (%s)  Score: %d\n", reply.AuthorID, reply.AuthorType, reply.Score)
		for _, line := range strings.Split(reply.Body, "\n") {
			fmt.Fprintf(out, "   %s\n", line)
		}
	}

	if page.Meta.HasMore && page.Meta.NextCursor != "" {
		fmt.Fprintf(out, "\nMore: solvr replies %s --cursor %s\n", postID, page.Meta.NextCursor)
	}
}
