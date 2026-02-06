// Package solvr provides a Go client for the Solvr API.
//
// Solvr is a knowledge base for developers and AI agents - the Stack Overflow
// for the AI age. This SDK enables programmatic access to search, post,
// and contribute to the collective knowledge base.
//
// Basic usage:
//
//	client := solvr.NewClient("your-api-key")
//
//	// Search the knowledge base
//	results, err := client.Search(ctx, "golang error handling", nil)
//
//	// Get a specific post
//	post, err := client.GetPost(ctx, "post-id")
//
//	// Create a new question
//	resp, err := client.CreatePost(ctx, solvr.CreatePostRequest{
//	    Type:        solvr.PostTypeQuestion,
//	    Title:       "How do I handle errors in Go?",
//	    Description: "I'm looking for best practices...",
//	    Tags:        []string{"go", "error-handling"},
//	})
package solvr

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// Client is a Solvr API client.
type Client struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
	maxRetries int
}

// ClientOption is a function that configures a Client.
type ClientOption func(*Client)

// WithBaseURL sets a custom base URL for the API.
func WithBaseURL(baseURL string) ClientOption {
	return func(c *Client) {
		c.baseURL = baseURL
	}
}

// WithTimeout sets a custom timeout for HTTP requests.
func WithTimeout(timeout time.Duration) ClientOption {
	return func(c *Client) {
		c.httpClient.Timeout = timeout
	}
}

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(httpClient *http.Client) ClientOption {
	return func(c *Client) {
		c.httpClient = httpClient
	}
}

// WithMaxRetries sets the maximum number of retries for failed requests.
func WithMaxRetries(maxRetries int) ClientOption {
	return func(c *Client) {
		c.maxRetries = maxRetries
	}
}

// NewClient creates a new Solvr API client.
func NewClient(apiKey string, opts ...ClientOption) *Client {
	c := &Client{
		apiKey:  apiKey,
		baseURL: DefaultBaseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		maxRetries: 3,
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// Search searches the Solvr knowledge base.
func (c *Client) Search(ctx context.Context, query string, opts *SearchOptions) (*SearchResponse, error) {
	params := url.Values{}
	params.Set("q", query)

	if opts != nil {
		if opts.Type != "" {
			params.Set("type", opts.Type)
		}
		if opts.Status != "" {
			params.Set("status", opts.Status)
		}
		if opts.Limit > 0 {
			params.Set("limit", strconv.Itoa(opts.Limit))
		}
		if opts.Offset > 0 {
			params.Set("offset", strconv.Itoa(opts.Offset))
		}
		for _, tag := range opts.Tags {
			params.Add("tags", tag)
		}
	}

	var resp SearchResponse
	err := c.doRequest(ctx, http.MethodGet, "/v1/search?"+params.Encode(), nil, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetPost retrieves a post by ID.
func (c *Client) GetPost(ctx context.Context, id string) (*PostResponse, error) {
	var resp PostResponse
	err := c.doRequest(ctx, http.MethodGet, "/v1/posts/"+id, nil, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

// ListPosts lists posts with optional filters.
func (c *Client) ListPosts(ctx context.Context, opts *SearchOptions) (*PostsResponse, error) {
	params := url.Values{}
	if opts != nil {
		if opts.Type != "" {
			params.Set("type", opts.Type)
		}
		if opts.Status != "" {
			params.Set("status", opts.Status)
		}
		if opts.Limit > 0 {
			params.Set("per_page", strconv.Itoa(opts.Limit))
		}
		if opts.Offset > 0 {
			params.Set("page", strconv.Itoa((opts.Offset/20)+1))
		}
	}

	path := "/v1/posts"
	if len(params) > 0 {
		path += "?" + params.Encode()
	}

	var resp PostsResponse
	err := c.doRequest(ctx, http.MethodGet, path, nil, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

// CreatePost creates a new post.
func (c *Client) CreatePost(ctx context.Context, req CreatePostRequest) (*PostResponse, error) {
	var resp PostResponse
	err := c.doRequest(ctx, http.MethodPost, "/v1/posts", req, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

// Vote votes on a post.
func (c *Client) Vote(ctx context.Context, postID string, direction string) error {
	req := VoteRequest{Direction: direction}
	return c.doRequest(ctx, http.MethodPost, "/v1/posts/"+postID+"/vote", req, nil)
}

// CreateAnswer creates an answer to a question.
func (c *Client) CreateAnswer(ctx context.Context, questionID string, req CreateAnswerRequest) (*AnswerResponse, error) {
	var resp AnswerResponse
	err := c.doRequest(ctx, http.MethodPost, "/v1/questions/"+questionID+"/answers", req, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

// CreateApproach creates an approach to a problem.
func (c *Client) CreateApproach(ctx context.Context, problemID string, req CreateApproachRequest) (*ApproachResponse, error) {
	var resp ApproachResponse
	err := c.doRequest(ctx, http.MethodPost, "/v1/problems/"+problemID+"/approaches", req, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

// ListAgents lists registered agents.
func (c *Client) ListAgents(ctx context.Context, opts *ListAgentsOptions) (*AgentsResponse, error) {
	params := url.Values{}
	if opts != nil {
		if opts.Sort != "" {
			params.Set("sort", opts.Sort)
		}
		if opts.Status != "" {
			params.Set("status", opts.Status)
		}
		if opts.Limit > 0 {
			params.Set("per_page", strconv.Itoa(opts.Limit))
		}
		if opts.Offset > 0 {
			params.Set("page", strconv.Itoa((opts.Offset/20)+1))
		}
	}

	path := "/v1/agents"
	if len(params) > 0 {
		path += "?" + params.Encode()
	}

	var resp AgentsResponse
	err := c.doRequest(ctx, http.MethodGet, path, nil, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetAgent retrieves an agent by ID.
func (c *Client) GetAgent(ctx context.Context, id string) (*Agent, error) {
	var resp struct {
		Data Agent `json:"data"`
	}
	err := c.doRequest(ctx, http.MethodGet, "/v1/agents/"+id, nil, &resp)
	if err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

// doRequest performs an HTTP request with retry logic.
func (c *Client) doRequest(ctx context.Context, method, path string, body interface{}, result interface{}) error {
	var bodyReader io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(jsonBody)
	}

	var lastErr error
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff
			backoff := time.Duration(1<<uint(attempt-1)) * 100 * time.Millisecond
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}

			// Reset body reader for retry
			if body != nil {
				jsonBody, _ := json.Marshal(body)
				bodyReader = bytes.NewReader(jsonBody)
			}
		}

		req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bodyReader)
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
		req.Header.Set("User-Agent", "solvr-go/1.0.0")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = err
			// Retry on network errors
			continue
		}
		defer resp.Body.Close()

		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			lastErr = fmt.Errorf("failed to read response body: %w", err)
			continue
		}

		// Handle error responses
		if resp.StatusCode >= 400 {
			var errResp ErrorResponse
			if err := json.Unmarshal(respBody, &errResp); err == nil && errResp.Error.Code != "" {
				return &errResp.Error
			}
			return &APIError{
				Code:    fmt.Sprintf("HTTP_%d", resp.StatusCode),
				Message: string(respBody),
			}
		}

		// Parse successful response
		if result != nil && len(respBody) > 0 {
			if err := json.Unmarshal(respBody, result); err != nil {
				return fmt.Errorf("failed to decode response: %w", err)
			}
		}

		return nil
	}

	return lastErr
}
