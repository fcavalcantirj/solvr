// Package services provides business logic for the Solvr application.
package services

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Default embedding service configuration values.
const (
	DefaultVoyageBaseURL  = "https://api.voyageai.com/v1"
	DefaultEmbeddingModel = "voyage-code-3"
	DefaultEmbedTimeout   = 30 * time.Second
	DefaultEmbedRetries   = 3
	DefaultEmbedRetryBase = 500 * time.Millisecond

	// MaxInputTokens is the maximum number of tokens for Voyage code-3 input.
	// We use a conservative character-based estimate: ~4 chars per token.
	MaxInputTokens = 8000
	charsPerToken  = 4
	maxInputChars  = MaxInputTokens * charsPerToken
)

// Embedding service errors.
var (
	ErrEmptyInput            = errors.New("embedding: input text must not be empty")
	ErrEmptyEmbeddingResponse = errors.New("embedding: API returned empty embedding data")
	ErrMissingAPIKey         = errors.New("embedding: API key must not be empty")
)

// EmbeddingService defines the interface for generating text embeddings.
type EmbeddingService interface {
	// GenerateEmbedding generates a document embedding for the given text.
	// Uses input_type "document" for content being stored/indexed.
	GenerateEmbedding(ctx context.Context, text string) ([]float32, error)

	// GenerateQueryEmbedding generates a query embedding for the given text.
	// Uses input_type "query" for search queries (asymmetric search support).
	GenerateQueryEmbedding(ctx context.Context, text string) ([]float32, error)
}

// VoyageEmbeddingService implements EmbeddingService using the Voyage AI API.
// Voyage code-3 uses asymmetric embeddings: documents and queries use different input_type values.
type VoyageEmbeddingService struct {
	apiKey     string
	baseURL    string
	model      string
	httpClient *http.Client
	maxRetries int
	retryDelay time.Duration
}

// NewVoyageEmbeddingService creates a new VoyageEmbeddingService with default settings.
func NewVoyageEmbeddingService(apiKey string) *VoyageEmbeddingService {
	return &VoyageEmbeddingService{
		apiKey:  apiKey,
		baseURL: DefaultVoyageBaseURL,
		model:   DefaultEmbeddingModel,
		httpClient: &http.Client{
			Timeout: DefaultEmbedTimeout,
		},
		maxRetries: DefaultEmbedRetries,
		retryDelay: DefaultEmbedRetryBase,
	}
}

// GenerateEmbedding generates a document embedding for the given text.
func (s *VoyageEmbeddingService) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	return s.generateEmbedding(ctx, text, "document")
}

// GenerateQueryEmbedding generates a query embedding for search queries.
func (s *VoyageEmbeddingService) GenerateQueryEmbedding(ctx context.Context, text string) ([]float32, error) {
	return s.generateEmbedding(ctx, text, "query")
}

// generateEmbedding is the internal method that handles both document and query embeddings.
func (s *VoyageEmbeddingService) generateEmbedding(ctx context.Context, text, inputType string) ([]float32, error) {
	if text == "" {
		return nil, ErrEmptyInput
	}

	// Truncate to max tokens using character-based estimation
	text = truncateToMaxTokens(text)

	reqBody := voyageEmbeddingRequest{
		Input:     text,
		Model:     s.model,
		InputType: inputType,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("embedding: failed to marshal request: %w", err)
	}

	respBody, err := s.doWithRetry(ctx, s.baseURL+"/embeddings", bodyBytes)
	if err != nil {
		return nil, err
	}

	var resp voyageEmbeddingResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("embedding: failed to parse response: %w", err)
	}

	if len(resp.Data) == 0 {
		return nil, ErrEmptyEmbeddingResponse
	}

	return resp.Data[0].Embedding, nil
}

// doWithRetry performs a POST request with retry logic for transient failures and 429 rate limits.
func (s *VoyageEmbeddingService) doWithRetry(ctx context.Context, url string, body []byte) ([]byte, error) {
	var lastErr error

	for attempt := 0; attempt <= s.maxRetries; attempt++ {
		if attempt > 0 {
			delay := s.retryDelay * time.Duration(1<<uint(attempt-1)) // exponential backoff
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
		}

		respBody, statusCode, err := s.doPost(ctx, url, body)
		if err != nil {
			lastErr = err
			continue
		}

		if statusCode >= 200 && statusCode < 300 {
			return respBody, nil
		}

		lastErr = fmt.Errorf("embedding: API returned status %d: %s", statusCode, string(respBody))

		// Retry on 429 rate limit
		if statusCode == http.StatusTooManyRequests {
			continue
		}

		// Don't retry other client errors (4xx) or server errors without retry semantics
		if statusCode >= 400 && statusCode < 500 {
			return nil, lastErr
		}
	}

	return nil, lastErr
}

// doPost executes a single POST request and returns the response body and status code.
func (s *VoyageEmbeddingService) doPost(ctx context.Context, url string, body []byte) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, 0, fmt.Errorf("embedding: failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.apiKey)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("embedding: request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, 0, fmt.Errorf("embedding: failed to read response: %w", err)
	}

	return respBody, resp.StatusCode, nil
}

// truncateToMaxTokens truncates text to approximately MaxInputTokens using
// a character-based estimation (~4 characters per token for English text).
func truncateToMaxTokens(text string) string {
	if len(text) <= maxInputChars {
		return text
	}
	return text[:maxInputChars]
}

// Voyage AI API request/response types.

type voyageEmbeddingRequest struct {
	Input     string `json:"input"`
	Model     string `json:"model"`
	InputType string `json:"input_type"`
}

type voyageEmbeddingResponse struct {
	Data  []voyageEmbeddingData `json:"data"`
	Usage *voyageUsage          `json:"usage,omitempty"`
}

type voyageEmbeddingData struct {
	Embedding []float32 `json:"embedding"`
	Index     int       `json:"index"`
}

type voyageUsage struct {
	TotalTokens int `json:"total_tokens"`
}
