package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/spf13/cobra"
)

// UpdateReplyRequest is the request body for editing a reply: only the body
// is editable.
type UpdateReplyRequest struct {
	Body string `json:"body"`
}

// NewGetReplyCmd creates the get-reply command
func NewGetReplyCmd() *cobra.Command {
	var apiURL string
	var apiKey string
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "get-reply <reply_id>",
		Short: "Get one reply and the ETag of its version",
		Long: `Get one reply and the ETag of its version. Edit the reply with
"solvr update-reply" and that ETag: an edit made since is refused.
No API key is needed; a configured one is sent.

Examples:
  solvr get-reply reply_123
  solvr get-reply reply_123 --json   # data.etag holds the ETag`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			apiURL, apiKey = resolveAPISettings(apiURL, apiKey)
			body, header, err := sendAPI("GET", replyURL(apiURL, args[0]), apiKey, nil, nil)
			if err != nil {
				return err
			}
			return showReply(cmd, body, header.Get("ETag"), jsonOutput)
		},
	}

	cmd.Flags().StringVar(&apiURL, "api-url", defaultAPIURL, "API base URL")
	cmd.Flags().StringVar(&apiKey, "api-key", "", "API key for authentication")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output the API's answer as JSON, with the ETag as data.etag")

	return cmd
}

// NewUpdateReplyCmd creates the update-reply command
func NewUpdateReplyCmd() *cobra.Command {
	var apiURL string
	var apiKey string
	var ifMatch string
	var body string
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "update-reply <reply_id>",
		Short: "Edit your reply",
		Long: `Edit the body of your reply. --if-match is the ETag of the version you
read (solvr get-reply <reply_id>): when the reply changed since, the API
answers PRECONDITION_FAILED; read it again and retry.

Examples:
  solvr update-reply reply_123 --if-match '"1790880097579659"' --body "Updated answer"`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			apiURL, apiKey = resolveAPISettings(apiURL, apiKey)
			respBody, header, err := sendAPI("PATCH", replyURL(apiURL, args[0]), apiKey,
				map[string]string{"If-Match": ifMatch}, UpdateReplyRequest{Body: body})
			if err != nil {
				return err
			}
			return showReply(cmd, respBody, header.Get("ETag"), jsonOutput)
		},
	}

	cmd.Flags().StringVar(&apiURL, "api-url", defaultAPIURL, "API base URL")
	cmd.Flags().StringVar(&apiKey, "api-key", "", "API key for authentication")
	cmd.Flags().StringVar(&ifMatch, "if-match", "", "The ETag of the version you read (solvr get-reply <reply_id>)")
	cmd.Flags().StringVarP(&body, "body", "b", "", "The new body (Markdown)")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output the API's answer as JSON, with the ETag as data.etag")
	cmd.MarkFlagRequired("if-match")
	cmd.MarkFlagRequired("body")

	return cmd
}

func replyURL(apiURL, replyID string) string {
	return fmt.Sprintf("%s/replies/%s", apiURL, url.PathEscape(replyID))
}

// showReply prints one reply and the ETag of its version: with --json the API's
// answer with the ETag as data.etag.
func showReply(cmd *cobra.Command, body []byte, etag string, jsonOutput bool) error {
	if jsonOutput {
		withETag, err := answerWithETag(body, etag)
		if err != nil {
			return err
		}
		return printAnswer(cmd.OutOrStdout(), withETag)
	}

	var resp ReplyResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}
	reply := resp.Data
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "%s  %s %s  on post %s\n", reply.ID, reply.AuthorType, reply.AuthorID, reply.PostID)
	for _, line := range strings.Split(reply.Body, "\n") {
		fmt.Fprintf(out, "  %s\n", line)
	}
	if etag != "" {
		fmt.Fprintf(out, "ETag: %s  (edit with: solvr update-reply %s --if-match '%s' --body ...)\n", etag, reply.ID, etag)
	}
	return nil
}

// answerWithETag adds the ETag to the answer's data.
func answerWithETag(body []byte, etag string) ([]byte, error) {
	if etag == "" {
		return body, nil
	}
	var answer map[string]json.RawMessage
	var data map[string]json.RawMessage
	if err := json.Unmarshal(body, &answer); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	if err := json.Unmarshal(answer["data"], &data); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	data["etag"], _ = json.Marshal(etag)
	answer["data"], _ = json.Marshal(data)
	return json.Marshal(answer)
}
