package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

// Default translation service configuration.
const (
	DefaultTranslationModel   = "llama-3.3-70b-versatile"
	DefaultTranslationTimeout = 15 * time.Second
)

// translationSystemPrompt is the static system prompt for technical content translation.
const translationSystemPrompt = `You are a technical translator for a developer Q&A platform. Translate the given title and description to English. Keep code snippets, technical terms, URLs, variable names, and identifiers unchanged. Return only valid JSON with keys "title" and "description".`

// TranslationRateLimitError is returned when the Groq API returns a 429 for translation.
type TranslationRateLimitError struct {
	RetryAfter time.Duration
	Message    string
}

func (e *TranslationRateLimitError) Error() string {
	return fmt.Sprintf("translation: rate limited, retry after %v: %s", e.RetryAfter, e.Message)
}

// GetRetryAfter returns the duration to wait before retrying.
func (e *TranslationRateLimitError) GetRetryAfter() time.Duration {
	return e.RetryAfter
}

// TranslationInput contains the post content to be translated.
type TranslationInput struct {
	Title       string
	Description string
	Language    string // source language hint (e.g., "Portuguese", "Spanish")
}

// TranslationResult contains the translated content.
type TranslationResult struct {
	Title       string `json:"title"`
	Description string `json:"description"`
}

// TranslationService translates content using the Groq API.
type TranslationService struct {
	groqAPIKey string
	groqModel  string
	baseURL    string
	httpClient *http.Client
}

// TranslationOption is a functional option for configuring TranslationService.
type TranslationOption func(*TranslationService)

// WithTranslationBaseURL overrides the default Groq API base URL.
func WithTranslationBaseURL(url string) TranslationOption {
	return func(s *TranslationService) {
		s.baseURL = url
	}
}

// WithTranslationModel overrides the default translation model.
func WithTranslationModel(model string) TranslationOption {
	return func(s *TranslationService) {
		s.groqModel = model
	}
}

// WithTranslationHTTPTimeout overrides the default HTTP timeout.
func WithTranslationHTTPTimeout(timeout time.Duration) TranslationOption {
	return func(s *TranslationService) {
		s.httpClient.Timeout = timeout
	}
}

// NewTranslationService creates a new TranslationService.
// The TRANSLATION_MODEL env var can override the default model at startup.
func NewTranslationService(apiKey string, opts ...TranslationOption) *TranslationService {
	svc := &TranslationService{
		groqAPIKey: apiKey,
		groqModel:  DefaultTranslationModel,
		baseURL:    DefaultGroqBaseURL,
		httpClient: &http.Client{
			Timeout: DefaultTranslationTimeout,
		},
	}

	for _, opt := range opts {
		opt(svc)
	}

	return svc
}

// TranslateContent translates post content from any language to English using the Groq API.
// Returns a *TranslationRateLimitError on HTTP 429, or a generic error on other failures.
func (s *TranslationService) TranslateContent(ctx context.Context, input TranslationInput) (*TranslationResult, error) {
	langHint := ""
	if input.Language != "" {
		langHint = fmt.Sprintf(" (source language: %s)", input.Language)
	}
	userMessage := fmt.Sprintf("Translate to English%s.\nTitle: %s\nDescription: %s",
		langHint, input.Title, input.Description)

	reqBody := groqChatRequest{
		Model: s.groqModel,
		Messages: []groqMessage{
			{Role: "system", Content: translationSystemPrompt},
			{Role: "user", Content: userMessage},
		},
		ResponseFormat:      buildTranslationResponseFormat(),
		Temperature:         0.2,
		MaxCompletionTokens: 1024,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("translation: failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.baseURL+"/chat/completions", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("translation: failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.groqAPIKey)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("translation: request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("translation: failed to read response: %w", err)
	}

	// Handle HTTP 429 rate limit.
	if resp.StatusCode == http.StatusTooManyRequests {
		retryAfter := parseRetryAfterSeconds(resp.Header.Get("Retry-After"))
		return nil, &TranslationRateLimitError{
			RetryAfter: retryAfter,
			Message:    string(respBody),
		}
	}

	// Handle non-2xx responses.
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("translation: Groq API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	// Parse the Groq response envelope.
	var chatResp groqChatResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return nil, fmt.Errorf("translation: failed to parse response envelope: %w", err)
	}

	if len(chatResp.Choices) == 0 {
		return nil, fmt.Errorf("translation: empty choices in response")
	}

	// Parse the translation result from the message content.
	var result TranslationResult
	if err := json.Unmarshal([]byte(chatResp.Choices[0].Message.Content), &result); err != nil {
		return nil, fmt.Errorf("translation: failed to parse translation result: %w", err)
	}

	return &result, nil
}

// parseRetryAfterSeconds parses a Retry-After header value as seconds.
// Reuses the same logic as the moderation service but returns a duration directly.
func parseRetryAfterSeconds(value string) time.Duration {
	if value == "" {
		return 60 * time.Second
	}
	seconds, err := strconv.Atoi(value)
	if err != nil {
		return 60 * time.Second
	}
	return time.Duration(seconds) * time.Second
}

// buildTranslationResponseFormat constructs the json_schema response format for translation.
func buildTranslationResponseFormat() *groqResponseFormat {
	return &groqResponseFormat{
		Type: "json_schema",
		JSONSchema: &groqJSONSchema{
			Name:   "translation_result",
			Strict: true,
			Schema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"title": map[string]interface{}{
						"type": "string",
					},
					"description": map[string]interface{}{
						"type": "string",
					},
				},
				"required":             []string{"title", "description"},
				"additionalProperties": false,
			},
		},
	}
}
