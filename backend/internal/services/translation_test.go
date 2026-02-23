package services

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// groqTranslationResponse builds a fake Groq chat completion response for translation.
func groqTranslationResponse(title, description string) string {
	content := map[string]interface{}{
		"title":       title,
		"description": description,
	}
	contentBytes, _ := json.Marshal(content)

	resp := map[string]interface{}{
		"id":      "chatcmpl-trans-test",
		"object":  "chat.completion",
		"created": 1700000000,
		"model":   DefaultTranslationModel,
		"choices": []map[string]interface{}{
			{
				"index": 0,
				"message": map[string]interface{}{
					"role":    "assistant",
					"content": string(contentBytes),
				},
				"finish_reason": "stop",
			},
		},
	}
	respBytes, _ := json.Marshal(resp)
	return string(respBytes)
}

func TestTranslateContent_HappyPath(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Error("expected Authorization header with API key")
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Error("expected Content-Type application/json")
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("x-ratelimit-remaining-requests", "90")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(groqTranslationResponse(
			"How to use goroutines in Go",
			"I am trying to understand goroutines and channels in Go.",
		)))
	}))
	defer server.Close()

	svc := NewTranslationService("test-key", WithTranslationBaseURL(server.URL))

	result, err := svc.TranslateContent(context.Background(), TranslationInput{
		Title:       "Como usar goroutines em Go",
		Description: "Estou tentando entender goroutines e canais em Go.",
		Language:    "Portuguese",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Title != "How to use goroutines in Go" {
		t.Errorf("expected translated title, got %q", result.Title)
	}
	if result.Description != "I am trying to understand goroutines and channels in Go." {
		t.Errorf("expected translated description, got %q", result.Description)
	}
}

func TestTranslateContent_RateLimit429(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "45")
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte(`{"error": {"message": "rate limit exceeded"}}`))
	}))
	defer server.Close()

	svc := NewTranslationService("test-key", WithTranslationBaseURL(server.URL))

	result, err := svc.TranslateContent(context.Background(), TranslationInput{
		Title:    "Some title",
		Language: "Spanish",
	})

	if err == nil {
		t.Fatal("expected error for 429 response, got nil")
	}
	if result != nil {
		t.Error("expected nil result on rate limit")
	}

	rlErr, ok := err.(*TranslationRateLimitError)
	if !ok {
		t.Fatalf("expected *TranslationRateLimitError, got %T: %v", err, err)
	}
	if rlErr.RetryAfter != 45*time.Second {
		t.Errorf("expected RetryAfter 45s, got %v", rlErr.RetryAfter)
	}
}

func TestTranslateContent_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": {"message": "internal error"}}`))
	}))
	defer server.Close()

	svc := NewTranslationService("test-key", WithTranslationBaseURL(server.URL))

	result, err := svc.TranslateContent(context.Background(), TranslationInput{
		Title:    "Some title",
		Language: "French",
	})

	if err == nil {
		t.Fatal("expected error for 500 response, got nil")
	}
	if result != nil {
		t.Error("expected nil result on API error")
	}
}

func TestTranslateContent_MalformedJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return a valid Groq envelope but with non-JSON content in the message
		resp := map[string]interface{}{
			"id":      "chatcmpl-bad",
			"object":  "chat.completion",
			"created": 1700000000,
			"model":   DefaultTranslationModel,
			"choices": []map[string]interface{}{
				{
					"index": 0,
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": "this is not valid json {{{",
					},
					"finish_reason": "stop",
				},
			},
		}
		respBytes, _ := json.Marshal(resp)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(respBytes)
	}))
	defer server.Close()

	svc := NewTranslationService("test-key", WithTranslationBaseURL(server.URL))

	result, err := svc.TranslateContent(context.Background(), TranslationInput{
		Title:    "Some title",
		Language: "German",
	})

	if err == nil {
		t.Fatal("expected error for malformed JSON content, got nil")
	}
	if result != nil {
		t.Error("expected nil result on malformed JSON")
	}
}

func TestTranslateContent_LanguageHintInPrompt(t *testing.T) {
	var capturedRequest groqChatRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&capturedRequest); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(groqTranslationResponse("Translated title", "Translated description")))
	}))
	defer server.Close()

	svc := NewTranslationService("test-key", WithTranslationBaseURL(server.URL))

	_, err := svc.TranslateContent(context.Background(), TranslationInput{
		Title:       "Título do post",
		Description: "Descrição do post",
		Language:    "Portuguese",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify the user message contains the language hint
	if len(capturedRequest.Messages) < 2 {
		t.Fatalf("expected at least 2 messages, got %d", len(capturedRequest.Messages))
	}
	userMsg := capturedRequest.Messages[1].Content
	if !strings.Contains(userMsg, "Portuguese") {
		t.Errorf("expected user message to contain language hint 'Portuguese', got %q", userMsg)
	}
	if !strings.Contains(userMsg, "Título do post") {
		t.Errorf("expected user message to contain title, got %q", userMsg)
	}
}

func TestTranslateContent_Temperature(t *testing.T) {
	var capturedRequest groqChatRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&capturedRequest)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(groqTranslationResponse("Title", "Description")))
	}))
	defer server.Close()

	svc := NewTranslationService("test-key", WithTranslationBaseURL(server.URL))
	_, _ = svc.TranslateContent(context.Background(), TranslationInput{Title: "Test"})

	if capturedRequest.Temperature != 0.2 {
		t.Errorf("expected temperature 0.2, got %f", capturedRequest.Temperature)
	}
}

func TestTranslateContent_MaxTokens(t *testing.T) {
	var capturedRequest groqChatRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&capturedRequest)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(groqTranslationResponse("Title", "Description")))
	}))
	defer server.Close()

	svc := NewTranslationService("test-key", WithTranslationBaseURL(server.URL))
	_, _ = svc.TranslateContent(context.Background(), TranslationInput{Title: "Test"})

	if capturedRequest.MaxCompletionTokens != 1024 {
		t.Errorf("expected max_completion_tokens 1024, got %d", capturedRequest.MaxCompletionTokens)
	}
}

func TestTranslateContent_UsesTranslationModel(t *testing.T) {
	var capturedRequest groqChatRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&capturedRequest)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(groqTranslationResponse("Title", "Description")))
	}))
	defer server.Close()

	svc := NewTranslationService("test-key", WithTranslationBaseURL(server.URL))
	_, _ = svc.TranslateContent(context.Background(), TranslationInput{Title: "Test"})

	if capturedRequest.Model != DefaultTranslationModel {
		t.Errorf("expected model %q, got %q", DefaultTranslationModel, capturedRequest.Model)
	}
}

func TestNewTranslationService_CustomModel(t *testing.T) {
	customModel := "llama-3.1-8b-instant"
	svc := NewTranslationService("test-key", WithTranslationModel(customModel))

	var capturedRequest groqChatRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&capturedRequest)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(groqTranslationResponse("Title", "Desc")))
	}))
	defer server.Close()

	svc.baseURL = server.URL
	_, err := svc.TranslateContent(context.Background(), TranslationInput{Title: "Test"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedRequest.Model != customModel {
		t.Errorf("expected model %q, got %q", customModel, capturedRequest.Model)
	}
}

func TestTranslateContent_SystemPromptContainsTechnicalTranslator(t *testing.T) {
	var capturedRequest groqChatRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&capturedRequest)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(groqTranslationResponse("Title", "Desc")))
	}))
	defer server.Close()

	svc := NewTranslationService("test-key", WithTranslationBaseURL(server.URL))
	_, _ = svc.TranslateContent(context.Background(), TranslationInput{Title: "Test"})

	if len(capturedRequest.Messages) == 0 {
		t.Fatal("expected messages in request")
	}
	sysMsg := capturedRequest.Messages[0]
	if sysMsg.Role != "system" {
		t.Errorf("expected first message role 'system', got %q", sysMsg.Role)
	}
	if !strings.Contains(sysMsg.Content, "translat") {
		t.Errorf("expected system message to contain 'translat', got %q", sysMsg.Content)
	}
}

func TestTranslateContent_StripsMarkdownFences(t *testing.T) {
	// llama-3.3-70b-versatile wraps JSON in ```json...``` for complex titles
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fencedContent := "```json\n{\"title\":\"AI Assistant\",\"description\":\"desc\"}\n```"
		resp := map[string]interface{}{
			"id":      "chatcmpl-fenced",
			"object":  "chat.completion",
			"created": 1700000000,
			"model":   DefaultTranslationModel,
			"choices": []map[string]interface{}{
				{
					"index":         0,
					"message":       map[string]interface{}{"role": "assistant", "content": fencedContent},
					"finish_reason": "stop",
				},
			},
		}
		respBytes, _ := json.Marshal(resp)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(respBytes)
	}))
	defer server.Close()

	svc := NewTranslationService("test-key", WithTranslationBaseURL(server.URL))
	result, err := svc.TranslateContent(context.Background(), TranslationInput{Title: "AI助手", Language: "Chinese"})
	if err != nil {
		t.Fatalf("expected fence-wrapped JSON to parse successfully, got: %v", err)
	}
	if result.Title != "AI Assistant" {
		t.Errorf("expected title 'AI Assistant', got %q", result.Title)
	}
}

func TestStripMarkdownFences(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		// Previously passing cases
		{"plain json", `{"title":"t","description":"d"}`, `{"title":"t","description":"d"}`},
		{"triple backtick json fence", "```json\n{\"title\":\"t\"}\n```", `{"title":"t"}`},
		{"triple backtick plain fence", "```\n{\"title\":\"t\"}\n```", `{"title":"t"}`},
		{"triple backtick with surrounding whitespace", "  ```json\n{\"title\":\"t\"}\n```  ", `{"title":"t"}`},
		// Production failure cases — single backtick wrapping
		{"single backtick wrap", "`{\"title\":\"t\"}`", `{"title":"t"}`},
		{"single backtick with json hint", "`json\n{\"title\":\"t\"}\n`", `{"title":"t"}`},
		// Text before the opening fence
		{"preamble before triple fence", "Here is the JSON:\n```json\n{\"title\":\"t\"}\n```", `{"title":"t"}`},
		{"preamble before plain fence", "Sure:\n```\n{\"title\":\"t\"}\n```", `{"title":"t"}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := stripMarkdownFences(c.input)
			if got != c.want {
				t.Errorf("stripMarkdownFences(%q) = %q, want %q", c.input, got, c.want)
			}
		})
	}
}

func TestTranslationRateLimitError_Interface(t *testing.T) {
	err := &TranslationRateLimitError{
		RetryAfter: 30 * time.Second,
		Message:    "too many requests",
	}

	if err.Error() == "" {
		t.Error("expected non-empty error message")
	}
	if err.GetRetryAfter() != 30*time.Second {
		t.Errorf("expected RetryAfter 30s, got %v", err.GetRetryAfter())
	}
}
