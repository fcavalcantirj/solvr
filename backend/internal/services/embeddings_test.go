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

func TestNewVoyageEmbeddingService(t *testing.T) {
	svc := NewVoyageEmbeddingService("test-api-key")

	if svc == nil {
		t.Fatal("expected non-nil service")
	}
	if svc.apiKey != "test-api-key" {
		t.Errorf("expected apiKey 'test-api-key', got %q", svc.apiKey)
	}
	if svc.model != DefaultEmbeddingModel {
		t.Errorf("expected model %q, got %q", DefaultEmbeddingModel, svc.model)
	}
	if svc.baseURL != DefaultVoyageBaseURL {
		t.Errorf("expected baseURL %q, got %q", DefaultVoyageBaseURL, svc.baseURL)
	}
}

func TestVoyageEmbeddingService_GenerateEmbedding(t *testing.T) {
	// Set up a test server that returns a valid embedding response
	expectedEmbedding := make([]float32, 1024)
	for i := range expectedEmbedding {
		expectedEmbedding[i] = float32(i) * 0.001
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request method and path
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/embeddings" {
			t.Errorf("expected path /embeddings, got %s", r.URL.Path)
		}

		// Verify auth header
		authHeader := r.Header.Get("Authorization")
		if authHeader != "Bearer test-api-key" {
			t.Errorf("expected auth header 'Bearer test-api-key', got %q", authHeader)
		}

		// Verify content type
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("expected Content-Type 'application/json', got %q", ct)
		}

		// Parse request body
		var reqBody voyageEmbeddingRequest
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}

		// Verify input_type is "document" for GenerateEmbedding
		if reqBody.InputType != "document" {
			t.Errorf("expected input_type 'document', got %q", reqBody.InputType)
		}
		if reqBody.Model != DefaultEmbeddingModel {
			t.Errorf("expected model %q, got %q", DefaultEmbeddingModel, reqBody.Model)
		}

		// Return embedding response
		resp := voyageEmbeddingResponse{
			Data: []voyageEmbeddingData{
				{Embedding: expectedEmbedding},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	svc := &VoyageEmbeddingService{
		apiKey:     "test-api-key",
		baseURL:    server.URL,
		model:      DefaultEmbeddingModel,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}

	embedding, err := svc.GenerateEmbedding(context.Background(), "test input text")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(embedding) != 1024 {
		t.Errorf("expected 1024-dim embedding, got %d", len(embedding))
	}
	if embedding[0] != 0 {
		t.Errorf("expected embedding[0] = 0, got %f", embedding[0])
	}
}

func TestVoyageEmbeddingService_GenerateQueryEmbedding(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var reqBody voyageEmbeddingRequest
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}

		// Verify input_type is "query" for GenerateQueryEmbedding
		if reqBody.InputType != "query" {
			t.Errorf("expected input_type 'query', got %q", reqBody.InputType)
		}

		resp := voyageEmbeddingResponse{
			Data: []voyageEmbeddingData{
				{Embedding: make([]float32, 1024)},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	svc := &VoyageEmbeddingService{
		apiKey:     "test-api-key",
		baseURL:    server.URL,
		model:      DefaultEmbeddingModel,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}

	embedding, err := svc.GenerateQueryEmbedding(context.Background(), "search query")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(embedding) != 1024 {
		t.Errorf("expected 1024-dim embedding, got %d", len(embedding))
	}
}

func TestVoyageEmbeddingService_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": "internal server error"}`))
	}))
	defer server.Close()

	svc := &VoyageEmbeddingService{
		apiKey:     "test-api-key",
		baseURL:    server.URL,
		model:      DefaultEmbeddingModel,
		httpClient: &http.Client{Timeout: 10 * time.Second},
		maxRetries: 0, // no retries for faster test
	}

	_, err := svc.GenerateEmbedding(context.Background(), "test input")
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("expected error to contain '500', got: %v", err)
	}
}

func TestVoyageEmbeddingService_RateLimitRetry(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			// First call: 429 rate limited
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"error": "rate limited"}`))
			return
		}
		// Second call: success
		resp := voyageEmbeddingResponse{
			Data: []voyageEmbeddingData{
				{Embedding: make([]float32, 1024)},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	svc := &VoyageEmbeddingService{
		apiKey:     "test-api-key",
		baseURL:    server.URL,
		model:      DefaultEmbeddingModel,
		httpClient: &http.Client{Timeout: 10 * time.Second},
		maxRetries: 2,
		retryDelay: 10 * time.Millisecond, // fast for tests
	}

	embedding, err := svc.GenerateEmbedding(context.Background(), "test input")
	if err != nil {
		t.Fatalf("unexpected error after retry: %v", err)
	}
	if len(embedding) != 1024 {
		t.Errorf("expected 1024-dim embedding, got %d", len(embedding))
	}
	if callCount != 2 {
		t.Errorf("expected 2 calls (1 retry), got %d", callCount)
	}
}

func TestVoyageEmbeddingService_EmptyInput(t *testing.T) {
	svc := NewVoyageEmbeddingService("test-api-key")

	_, err := svc.GenerateEmbedding(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty input")
	}
	if err != ErrEmptyInput {
		t.Errorf("expected ErrEmptyInput, got: %v", err)
	}
}

func TestVoyageEmbeddingService_EmptyResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := voyageEmbeddingResponse{
			Data: []voyageEmbeddingData{}, // empty data
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	svc := &VoyageEmbeddingService{
		apiKey:     "test-api-key",
		baseURL:    server.URL,
		model:      DefaultEmbeddingModel,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}

	_, err := svc.GenerateEmbedding(context.Background(), "test input")
	if err == nil {
		t.Fatal("expected error for empty response data")
	}
	if err != ErrEmptyEmbeddingResponse {
		t.Errorf("expected ErrEmptyEmbeddingResponse, got: %v", err)
	}
}

func TestVoyageEmbeddingService_ContextCanceled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond) // Slow response
	}))
	defer server.Close()

	svc := &VoyageEmbeddingService{
		apiKey:     "test-api-key",
		baseURL:    server.URL,
		model:      DefaultEmbeddingModel,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := svc.GenerateEmbedding(ctx, "test input")
	if err == nil {
		t.Fatal("expected error for canceled context")
	}
}

func TestVoyageEmbeddingService_TruncatesLongInput(t *testing.T) {
	var receivedInput string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var reqBody voyageEmbeddingRequest
		json.NewDecoder(r.Body).Decode(&reqBody)
		receivedInput = reqBody.Input

		resp := voyageEmbeddingResponse{
			Data: []voyageEmbeddingData{
				{Embedding: make([]float32, 1024)},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	svc := &VoyageEmbeddingService{
		apiKey:     "test-api-key",
		baseURL:    server.URL,
		model:      DefaultEmbeddingModel,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}

	// Create a very long input (well over 8000 tokens)
	longInput := strings.Repeat("This is a test sentence with multiple words. ", 5000)

	_, err := svc.GenerateEmbedding(context.Background(), longInput)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The received input should be shorter than the original due to truncation
	if len(receivedInput) >= len(longInput) {
		t.Error("expected input to be truncated, but it was not")
	}
}

func TestEmbeddingServiceInterface(t *testing.T) {
	// Verify VoyageEmbeddingService implements EmbeddingService
	var _ EmbeddingService = (*VoyageEmbeddingService)(nil)
}

func TestVoyageEmbeddingService_ClientErrorNoRetry(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error": "bad request"}`))
	}))
	defer server.Close()

	svc := &VoyageEmbeddingService{
		apiKey:     "test-api-key",
		baseURL:    server.URL,
		model:      DefaultEmbeddingModel,
		httpClient: &http.Client{Timeout: 10 * time.Second},
		maxRetries: 3,
		retryDelay: 10 * time.Millisecond,
	}

	_, err := svc.GenerateEmbedding(context.Background(), "test input")
	if err == nil {
		t.Fatal("expected error for 400 response")
	}
	// Should NOT retry on 4xx (except 429)
	if callCount != 1 {
		t.Errorf("expected 1 call (no retry for 4xx), got %d", callCount)
	}
}
