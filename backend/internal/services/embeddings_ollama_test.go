package services

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewOllamaEmbeddingService(t *testing.T) {
	svc := NewOllamaEmbeddingService("")

	if svc == nil {
		t.Fatal("expected non-nil service")
	}
	if svc.baseURL != DefaultOllamaBaseURL {
		t.Errorf("expected baseURL %q, got %q", DefaultOllamaBaseURL, svc.baseURL)
	}
	if svc.model != DefaultOllamaModel {
		t.Errorf("expected model %q, got %q", DefaultOllamaModel, svc.model)
	}
}

func TestNewOllamaEmbeddingService_CustomBaseURL(t *testing.T) {
	customURL := "http://my-ollama:11434/v1"
	svc := NewOllamaEmbeddingService(customURL)

	if svc.baseURL != customURL {
		t.Errorf("expected baseURL %q, got %q", customURL, svc.baseURL)
	}
}

func TestOllamaEmbeddingService_GenerateEmbedding(t *testing.T) {
	// nomic-embed-text produces 768-dimension embeddings
	expectedEmbedding := make([]float32, 768)
	for i := range expectedEmbedding {
		expectedEmbedding[i] = float32(i) * 0.001
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/embeddings" {
			t.Errorf("expected path /embeddings, got %s", r.URL.Path)
		}

		// Verify content type
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("expected Content-Type 'application/json', got %q", ct)
		}

		// Parse request body - Ollama uses OpenAI-compatible format
		var reqBody ollamaEmbeddingRequest
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}

		if reqBody.Model != DefaultOllamaModel {
			t.Errorf("expected model %q, got %q", DefaultOllamaModel, reqBody.Model)
		}
		if reqBody.Input == "" {
			t.Error("expected non-empty input")
		}

		// Ollama uses OpenAI-compatible response format
		resp := ollamaEmbeddingResponse{
			Data: []ollamaEmbeddingData{
				{Embedding: expectedEmbedding, Index: 0},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	svc := &OllamaEmbeddingService{
		baseURL:    server.URL,
		model:      DefaultOllamaModel,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}

	embedding, err := svc.GenerateEmbedding(context.Background(), "test post about golang race conditions")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(embedding) != 768 {
		t.Errorf("expected 768-dim embedding, got %d", len(embedding))
	}
	if embedding[0] != 0 {
		t.Errorf("expected embedding[0] = 0, got %f", embedding[0])
	}
}

func TestOllamaEmbeddingService_GenerateQueryEmbedding(t *testing.T) {
	// nomic-embed-text uses symmetric embeddings, so GenerateQueryEmbedding
	// should be the same as GenerateEmbedding (no input_type distinction).
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := ollamaEmbeddingResponse{
			Data: []ollamaEmbeddingData{
				{Embedding: make([]float32, 768)},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	svc := &OllamaEmbeddingService{
		baseURL:    server.URL,
		model:      DefaultOllamaModel,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}

	embedding, err := svc.GenerateQueryEmbedding(context.Background(), "race condition golang")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(embedding) != 768 {
		t.Errorf("expected 768-dim embedding, got %d", len(embedding))
	}
}

func TestOllamaEmbeddingService_EmptyInput(t *testing.T) {
	svc := NewOllamaEmbeddingService("")

	_, err := svc.GenerateEmbedding(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty input")
	}
	if err != ErrEmptyInput {
		t.Errorf("expected ErrEmptyInput, got: %v", err)
	}
}

func TestOllamaEmbeddingService_EmptyResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := ollamaEmbeddingResponse{
			Data: []ollamaEmbeddingData{}, // empty data
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	svc := &OllamaEmbeddingService{
		baseURL:    server.URL,
		model:      DefaultOllamaModel,
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

func TestOllamaEmbeddingService_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": "model not found"}`))
	}))
	defer server.Close()

	svc := &OllamaEmbeddingService{
		baseURL:    server.URL,
		model:      DefaultOllamaModel,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}

	_, err := svc.GenerateEmbedding(context.Background(), "test input")
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}

func TestOllamaEmbeddingService_Timeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond) // Slow response simulating CPU inference
	}))
	defer server.Close()

	svc := &OllamaEmbeddingService{
		baseURL:    server.URL,
		model:      DefaultOllamaModel,
		httpClient: &http.Client{Timeout: 50 * time.Millisecond}, // Short timeout for test
	}

	_, err := svc.GenerateEmbedding(context.Background(), "test input")
	if err == nil {
		t.Fatal("expected error for timeout")
	}
}

func TestOllamaEmbeddingService_ImplementsInterface(t *testing.T) {
	// Verify OllamaEmbeddingService implements EmbeddingService
	var _ EmbeddingService = (*OllamaEmbeddingService)(nil)
}
