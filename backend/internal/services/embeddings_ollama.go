package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Default Ollama embedding service configuration values.
const (
	DefaultOllamaBaseURL = "http://localhost:11434/v1"
	DefaultOllamaModel   = "nomic-embed-text"
	// OllamaEmbedTimeout is longer than Voyage because CPU inference can be slow.
	OllamaEmbedTimeout = 30 * time.Second
)

// OllamaEmbeddingService implements EmbeddingService using a local Ollama instance.
// nomic-embed-text produces 768-dimension vectors (vs 1024 for Voyage code-3).
// nomic-embed-text uses symmetric embeddings, so documents and queries are embedded the same way.
// If using Ollama, the pgvector migration must use vector(768) instead of vector(1024).
type OllamaEmbeddingService struct {
	baseURL    string
	model      string
	httpClient *http.Client
}

// NewOllamaEmbeddingService creates a new OllamaEmbeddingService.
// If baseURL is empty, it defaults to http://localhost:11434/v1 (Ollama local).
func NewOllamaEmbeddingService(baseURL string) *OllamaEmbeddingService {
	if baseURL == "" {
		baseURL = DefaultOllamaBaseURL
	}
	return &OllamaEmbeddingService{
		baseURL: baseURL,
		model:   DefaultOllamaModel,
		httpClient: &http.Client{
			Timeout: OllamaEmbedTimeout,
		},
	}
}

// GenerateEmbedding generates a document embedding for the given text.
// nomic-embed-text uses symmetric embeddings, so this is the same as GenerateQueryEmbedding.
func (s *OllamaEmbeddingService) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	return s.generateEmbedding(ctx, text)
}

// GenerateQueryEmbedding generates a query embedding for search queries.
// nomic-embed-text uses symmetric embeddings, so this is the same as GenerateEmbedding.
func (s *OllamaEmbeddingService) GenerateQueryEmbedding(ctx context.Context, text string) ([]float32, error) {
	return s.generateEmbedding(ctx, text)
}

// generateEmbedding is the internal method that calls the Ollama OpenAI-compatible API.
func (s *OllamaEmbeddingService) generateEmbedding(ctx context.Context, text string) ([]float32, error) {
	if text == "" {
		return nil, ErrEmptyInput
	}

	reqBody := ollamaEmbeddingRequest{
		Input: text,
		Model: s.model,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("embedding: failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.baseURL+"/embeddings", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("embedding: failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("embedding: request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("embedding: failed to read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("embedding: Ollama API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var embResp ollamaEmbeddingResponse
	if err := json.Unmarshal(respBody, &embResp); err != nil {
		return nil, fmt.Errorf("embedding: failed to parse response: %w", err)
	}

	if len(embResp.Data) == 0 {
		return nil, ErrEmptyEmbeddingResponse
	}

	return embResp.Data[0].Embedding, nil
}

// Ollama uses OpenAI-compatible API format for embeddings.

type ollamaEmbeddingRequest struct {
	Input string `json:"input"`
	Model string `json:"model"`
}

type ollamaEmbeddingResponse struct {
	Data []ollamaEmbeddingData `json:"data"`
}

type ollamaEmbeddingData struct {
	Embedding []float32 `json:"embedding"`
	Index     int       `json:"index"`
}
