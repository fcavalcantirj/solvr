package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Default translation service configuration.
const (
	DefaultTranslationModel   = "llama-3.3-70b-versatile"
	DefaultTranslationTimeout = 15 * time.Second
)

// translationSystemPrompt is the static system prompt for technical content translation.
// Uses plain JSON instruction instead of json_schema response_format for broader model compatibility.
const translationSystemPrompt = `You are a technical translator for a developer Q&A platform. Translate the given title and description to English. Keep code snippets, technical terms, URLs, variable names, and identifiers unchanged. Respond ONLY with a valid JSON object with exactly two keys: "title" and "description". No markdown, no explanation, just the JSON object.`

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
		// No ResponseFormat: llama-3.3-70b-versatile does not support json_schema.
		// JSON output is enforced via the system prompt instead.
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
	// Strip markdown fences if the model wraps the JSON (e.g. ```json\n{...}\n```).
	content := stripMarkdownFences(chatResp.Choices[0].Message.Content)
	var result TranslationResult
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		return nil, fmt.Errorf("translation: failed to parse translation result: %w", err)
	}

	return &result, nil
}

// stripMarkdownFences removes leading ```json or ``` and trailing ``` from a string.
// llama-3.3-70b-versatile occasionally wraps JSON responses in markdown fences
// despite system prompt instructions not to.
func stripMarkdownFences(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```") {
		if idx := strings.Index(s, "\n"); idx != -1 {
			s = s[idx+1:]
		}
		if idx := strings.LastIndex(s, "```"); idx != -1 {
			s = s[:idx]
		}
		s = strings.TrimSpace(s)
	}
	return s
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
