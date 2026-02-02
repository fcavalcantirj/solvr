package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
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

// ApproachDetail represents an approach to a problem
type ApproachDetail struct {
	ID          string     `json:"id"`
	ProblemID   string     `json:"problem_id"`
	Angle       string     `json:"angle"`
	Method      string     `json:"method,omitempty"`
	Assumptions []string   `json:"assumptions,omitempty"`
	DiffersFrom []string   `json:"differs_from,omitempty"`
	Status      string     `json:"status"`
	Outcome     string     `json:"outcome,omitempty"`
	Solution    string     `json:"solution,omitempty"`
	AuthorType  string     `json:"author_type"`
	AuthorID    string     `json:"author_id"`
	Author      AuthorInfo `json:"author"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// ApproachesAPIResponse is the response from the approaches endpoint
type ApproachesAPIResponse struct {
	Data []ApproachDetail `json:"data"`
}

// AnswerDetail represents an answer to a question
type AnswerDetail struct {
	ID         string     `json:"id"`
	QuestionID string     `json:"question_id"`
	Content    string     `json:"content"`
	AuthorType string     `json:"author_type"`
	AuthorID   string     `json:"author_id"`
	Author     AuthorInfo `json:"author"`
	IsAccepted bool       `json:"is_accepted"`
	Upvotes    int        `json:"upvotes"`
	Downvotes  int        `json:"downvotes"`
	VoteScore  int        `json:"vote_score"`
	CreatedAt  time.Time  `json:"created_at"`
}

// QuestionWithAnswers represents a question with its answers
type QuestionWithAnswers struct {
	PostDetail
	Answers []AnswerDetail `json:"answers"`
}

// QuestionAPIResponse is the response from the questions endpoint
type QuestionAPIResponse struct {
	Data QuestionWithAnswers `json:"data"`
}

// ResponseDetail represents a response to an idea
type ResponseDetail struct {
	ID           string     `json:"id"`
	IdeaID       string     `json:"idea_id"`
	Content      string     `json:"content"`
	ResponseType string     `json:"response_type"`
	AuthorType   string     `json:"author_type"`
	AuthorID     string     `json:"author_id"`
	Author       AuthorInfo `json:"author"`
	Upvotes      int        `json:"upvotes"`
	Downvotes    int        `json:"downvotes"`
	VoteScore    int        `json:"vote_score"`
	CreatedAt    time.Time  `json:"created_at"`
}

// IdeaWithResponses represents an idea with its responses
type IdeaWithResponses struct {
	PostDetail
	Responses []ResponseDetail `json:"responses"`
}

// IdeaAPIResponse is the response from the ideas endpoint
type IdeaAPIResponse struct {
	Data IdeaWithResponses `json:"data"`
}

// GetResponseWithIncludes holds post data plus optional includes
type GetResponseWithIncludes struct {
	Data       PostDetail       `json:"data"`
	Approaches []ApproachDetail `json:"approaches,omitempty"`
	Answers    []AnswerDetail   `json:"answers,omitempty"`
	Responses  []ResponseDetail `json:"responses,omitempty"`
}

// NewGetCmd creates the get command
func NewGetCmd() *cobra.Command {
	var apiURL string
	var apiKey string
	var jsonOutput bool
	var include string

	cmd := &cobra.Command{
		Use:   "get <id>",
		Short: "Get details of a Solvr post",
		Long: `Get the full details of a post from the Solvr knowledge base.

Use this command to view the complete content of a problem, question, or idea.

Examples:
  solvr get post-123
  solvr get post-123 --api-key solvr_xxx
  solvr get post-123 --api-url http://localhost:8080/v1
  solvr get post-123 --json
  solvr get prob-123 --include approaches
  solvr get q-123 --include answers
  solvr get idea-123 --include responses`,
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

			// Create HTTP client
			client := &http.Client{Timeout: 30 * time.Second}

			// First, get the post to determine its type
			getURL := fmt.Sprintf("%s/posts/%s", apiURL, postID)
			req, err := http.NewRequest("GET", getURL, nil)
			if err != nil {
				return fmt.Errorf("failed to create request: %w", err)
			}

			if apiKey != "" {
				req.Header.Set("Authorization", "Bearer "+apiKey)
			}

			resp, err := client.Do(req)
			if err != nil {
				return fmt.Errorf("failed to call API: %w", err)
			}
			defer resp.Body.Close()

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				return fmt.Errorf("failed to read response: %w", err)
			}

			if resp.StatusCode != http.StatusOK {
				var apiErr APIError
				if json.Unmarshal(body, &apiErr) == nil && apiErr.Error.Message != "" {
					return fmt.Errorf("API error: %s", apiErr.Error.Message)
				}
				return fmt.Errorf("API returned status %d", resp.StatusCode)
			}

			var getResp GetAPIResponse
			if err := json.Unmarshal(body, &getResp); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}

			// Parse include options
			includeOpts := parseIncludeOptions(include)

			// Prepare response with includes
			result := GetResponseWithIncludes{
				Data: getResp.Data,
			}

			// Fetch includes based on post type
			postType := getResp.Data.Type

			if postType == "problem" && includeOpts["approaches"] {
				approaches, err := fetchApproaches(client, apiURL, postID, apiKey)
				if err != nil {
					return fmt.Errorf("failed to fetch approaches: %w", err)
				}
				result.Approaches = approaches
			}

			if postType == "question" && includeOpts["answers"] {
				answers, post, err := fetchQuestionWithAnswers(client, apiURL, postID, apiKey)
				if err != nil {
					return fmt.Errorf("failed to fetch answers: %w", err)
				}
				result.Answers = answers
				result.Data = post // Use the question-specific response
			}

			if postType == "idea" && includeOpts["responses"] {
				responses, post, err := fetchIdeaWithResponses(client, apiURL, postID, apiKey)
				if err != nil {
					return fmt.Errorf("failed to fetch responses: %w", err)
				}
				result.Responses = responses
				result.Data = post // Use the idea-specific response
			}

			// Output as JSON or pretty display
			if jsonOutput {
				displayGetJSONOutputWithIncludes(cmd, result)
			} else {
				displayPostDetailsWithIncludes(cmd, result)
			}

			return nil
		},
	}

	// Add flags
	cmd.Flags().StringVar(&apiURL, "api-url", defaultAPIURL, "API base URL")
	cmd.Flags().StringVar(&apiKey, "api-key", "", "API key for authentication")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output raw JSON")
	cmd.Flags().StringVar(&include, "include", "", "Include related content: approaches, answers, responses (comma-separated)")

	return cmd
}

// parseIncludeOptions parses the --include flag value
func parseIncludeOptions(include string) map[string]bool {
	opts := make(map[string]bool)
	if include == "" {
		return opts
	}
	for _, opt := range strings.Split(include, ",") {
		opts[strings.TrimSpace(opt)] = true
	}
	return opts
}

// fetchApproaches fetches approaches for a problem
func fetchApproaches(client *http.Client, apiURL, problemID, apiKey string) ([]ApproachDetail, error) {
	url := fmt.Sprintf("%s/problems/%s/approaches", apiURL, problemID)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("approaches API returned status %d", resp.StatusCode)
	}

	var approachesResp ApproachesAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&approachesResp); err != nil {
		return nil, err
	}

	return approachesResp.Data, nil
}

// fetchQuestionWithAnswers fetches a question with its answers
func fetchQuestionWithAnswers(client *http.Client, apiURL, questionID, apiKey string) ([]AnswerDetail, PostDetail, error) {
	url := fmt.Sprintf("%s/questions/%s", apiURL, questionID)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, PostDetail{}, err
	}
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, PostDetail{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, PostDetail{}, fmt.Errorf("questions API returned status %d", resp.StatusCode)
	}

	var questionResp QuestionAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&questionResp); err != nil {
		return nil, PostDetail{}, err
	}

	return questionResp.Data.Answers, questionResp.Data.PostDetail, nil
}

// fetchIdeaWithResponses fetches an idea with its responses
func fetchIdeaWithResponses(client *http.Client, apiURL, ideaID, apiKey string) ([]ResponseDetail, PostDetail, error) {
	url := fmt.Sprintf("%s/ideas/%s", apiURL, ideaID)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, PostDetail{}, err
	}
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, PostDetail{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, PostDetail{}, fmt.Errorf("ideas API returned status %d", resp.StatusCode)
	}

	var ideaResp IdeaAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&ideaResp); err != nil {
		return nil, PostDetail{}, err
	}

	return ideaResp.Data.Responses, ideaResp.Data.PostDetail, nil
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

// displayPostDetailsWithIncludes formats and displays post details with includes
func displayPostDetailsWithIncludes(cmd *cobra.Command, result GetResponseWithIncludes) {
	// First display the post details
	displayPostDetails(cmd, result.Data)
	out := cmd.OutOrStdout()

	// Display approaches if included
	if len(result.Approaches) > 0 {
		fmt.Fprintln(out, "\n--- Approaches ---")
		for i, approach := range result.Approaches {
			statusIcon := getApproachStatusIcon(approach.Status)
			fmt.Fprintf(out, "\n%d. %s [%s] %s\n", i+1, statusIcon, approach.Status, approach.Angle)
			if approach.Method != "" {
				fmt.Fprintf(out, "   Method: %s\n", approach.Method)
			}
			fmt.Fprintf(out, "   By: %s (%s)\n", approach.Author.DisplayName, approach.Author.Type)
			if approach.Outcome != "" {
				fmt.Fprintf(out, "   Outcome: %s\n", truncateString(approach.Outcome, 100))
			}
		}
	}

	// Display answers if included
	if len(result.Answers) > 0 {
		fmt.Fprintln(out, "\n--- Answers ---")
		for i, answer := range result.Answers {
			acceptedMark := ""
			if answer.IsAccepted {
				acceptedMark = " ✓ Accepted"
			}
			fmt.Fprintf(out, "\n%d.%s [Score: %d]\n", i+1, acceptedMark, answer.VoteScore)
			fmt.Fprintf(out, "   By: %s (%s)\n", answer.Author.DisplayName, answer.Author.Type)
			fmt.Fprintf(out, "   %s\n", truncateString(answer.Content, 200))
		}
	}

	// Display responses if included
	if len(result.Responses) > 0 {
		fmt.Fprintln(out, "\n--- Responses ---")
		for i, response := range result.Responses {
			fmt.Fprintf(out, "\n%d. [%s] Score: %d\n", i+1, response.ResponseType, response.VoteScore)
			fmt.Fprintf(out, "   By: %s (%s)\n", response.Author.DisplayName, response.Author.Type)
			fmt.Fprintf(out, "   %s\n", truncateString(response.Content, 200))
		}
	}
}

// getApproachStatusIcon returns an icon for the approach status
func getApproachStatusIcon(status string) string {
	switch status {
	case "succeeded":
		return "✓"
	case "failed":
		return "✗"
	case "stuck":
		return "!"
	case "working":
		return "→"
	default:
		return "○"
	}
}

// truncateString truncates a string to a maximum length
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// displayGetJSONOutput outputs the get response as raw JSON
func displayGetJSONOutput(cmd *cobra.Command, resp GetAPIResponse) {
	out := cmd.OutOrStdout()
	encoder := json.NewEncoder(out)
	encoder.SetIndent("", "  ")
	encoder.Encode(resp)
}

// displayGetJSONOutputWithIncludes outputs the response with includes as raw JSON
func displayGetJSONOutputWithIncludes(cmd *cobra.Command, result GetResponseWithIncludes) {
	out := cmd.OutOrStdout()

	// Build output map that includes answers in the data field for questions
	output := make(map[string]interface{})

	// Create data object
	dataMap := make(map[string]interface{})
	dataBytes, _ := json.Marshal(result.Data)
	json.Unmarshal(dataBytes, &dataMap)

	// Add includes to the data object
	if len(result.Approaches) > 0 {
		dataMap["approaches"] = result.Approaches
	}
	if len(result.Answers) > 0 {
		dataMap["answers"] = result.Answers
	}
	if len(result.Responses) > 0 {
		dataMap["responses"] = result.Responses
	}

	output["data"] = dataMap

	encoder := json.NewEncoder(out)
	encoder.SetIndent("", "  ")
	encoder.Encode(output)
}
