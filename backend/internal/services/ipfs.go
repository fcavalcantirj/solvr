// Package services provides business logic for the Solvr application.
package services

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"time"
)

// Default IPFS client configuration values.
const (
	DefaultIPFSTimeout = 5 * time.Minute
	DefaultMaxRetries  = 3
	DefaultRetryDelay  = 1 * time.Second
)

// IPFS service errors.
var (
	ErrEmptyCID  = errors.New("CID must not be empty")
	ErrNilReader = errors.New("reader must not be nil")
)

// IPFSService defines the interface for interacting with an IPFS node.
type IPFSService interface {
	// Pin pins a CID to the IPFS node.
	Pin(ctx context.Context, cid string) error

	// Unpin removes a pin for a CID from the IPFS node.
	Unpin(ctx context.Context, cid string) error

	// PinStatus returns the pin type for a CID ("direct", "recursive", or error if not pinned).
	PinStatus(ctx context.Context, cid string) (string, error)

	// Add uploads content to IPFS and returns the CID.
	Add(ctx context.Context, reader io.Reader) (string, error)

	// ObjectStat returns the cumulative size in bytes for a CID.
	ObjectStat(ctx context.Context, cid string) (int64, error)
}

// IPFSConfig holds configuration for the IPFS client service.
type IPFSConfig struct {
	Timeout    time.Duration
	MaxRetries int
	RetryDelay time.Duration
}

// DefaultIPFSConfig returns the default IPFS configuration.
func DefaultIPFSConfig() IPFSConfig {
	return IPFSConfig{
		Timeout:    DefaultIPFSTimeout,
		MaxRetries: DefaultMaxRetries,
		RetryDelay: DefaultRetryDelay,
	}
}

// KuboIPFSService implements IPFSService by calling the Kubo HTTP API.
type KuboIPFSService struct {
	baseURL    string
	httpClient *http.Client
	maxRetries int
	retryDelay time.Duration
}

// NewKuboIPFSService creates a new KuboIPFSService with default timeout.
func NewKuboIPFSService(baseURL string) *KuboIPFSService {
	return NewKuboIPFSServiceWithConfig(baseURL, DefaultIPFSConfig())
}

// NewKuboIPFSServiceWithTimeout creates a new KuboIPFSService with a custom timeout.
func NewKuboIPFSServiceWithTimeout(baseURL string, timeout time.Duration) *KuboIPFSService {
	cfg := DefaultIPFSConfig()
	cfg.Timeout = timeout
	return NewKuboIPFSServiceWithConfig(baseURL, cfg)
}

// NewKuboIPFSServiceWithConfig creates a new KuboIPFSService with full configuration.
func NewKuboIPFSServiceWithConfig(baseURL string, cfg IPFSConfig) *KuboIPFSService {
	return &KuboIPFSService{
		baseURL: strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: cfg.Timeout,
		},
		maxRetries: cfg.MaxRetries,
		retryDelay: cfg.RetryDelay,
	}
}

// Pin pins a CID to the IPFS node via POST /api/v0/pin/add?arg={cid}&progress=false.
func (s *KuboIPFSService) Pin(ctx context.Context, cid string) error {
	if cid == "" {
		return ErrEmptyCID
	}

	url := fmt.Sprintf("%s/api/v0/pin/add?arg=%s&progress=false", s.baseURL, cid)
	_, err := s.doWithRetry(ctx, url)
	return err
}

// Unpin removes a pin for a CID via POST /api/v0/pin/rm?arg={cid}.
func (s *KuboIPFSService) Unpin(ctx context.Context, cid string) error {
	if cid == "" {
		return ErrEmptyCID
	}

	url := fmt.Sprintf("%s/api/v0/pin/rm?arg=%s", s.baseURL, cid)
	_, err := s.doWithRetry(ctx, url)
	return err
}

// PinStatus returns the pin type for a CID via POST /api/v0/pin/ls?arg={cid}.
func (s *KuboIPFSService) PinStatus(ctx context.Context, cid string) (string, error) {
	if cid == "" {
		return "", ErrEmptyCID
	}

	url := fmt.Sprintf("%s/api/v0/pin/ls?arg=%s", s.baseURL, cid)
	body, err := s.doWithRetry(ctx, url)
	if err != nil {
		return "", err
	}

	var result pinLsResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("ipfs: failed to parse pin/ls response: %w", err)
	}

	for _, info := range result.Keys {
		return info.Type, nil
	}

	return "", fmt.Errorf("ipfs: CID %s not found in pin/ls response", cid)
}

// Add uploads content to IPFS via POST /api/v0/add with multipart form data.
func (s *KuboIPFSService) Add(ctx context.Context, reader io.Reader) (string, error) {
	if reader == nil {
		return "", ErrNilReader
	}

	// Build multipart form with file field
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	part, err := writer.CreateFormFile("file", "data")
	if err != nil {
		return "", fmt.Errorf("ipfs: failed to create multipart form: %w", err)
	}

	if _, err := io.Copy(part, reader); err != nil {
		return "", fmt.Errorf("ipfs: failed to write content to form: %w", err)
	}

	if err := writer.Close(); err != nil {
		return "", fmt.Errorf("ipfs: failed to close multipart writer: %w", err)
	}

	url := fmt.Sprintf("%s/api/v0/add", s.baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, &buf)
	if err != nil {
		return "", fmt.Errorf("ipfs: failed to create add request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("ipfs: add request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("ipfs: failed to read add response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ipfs: add returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var result addResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("ipfs: failed to parse add response: %w", err)
	}

	if result.Hash == "" {
		return "", fmt.Errorf("ipfs: add response missing Hash field")
	}

	return result.Hash, nil
}

// ObjectStat returns the cumulative size of an object via POST /api/v0/object/stat?arg={cid}.
func (s *KuboIPFSService) ObjectStat(ctx context.Context, cid string) (int64, error) {
	if cid == "" {
		return 0, ErrEmptyCID
	}

	url := fmt.Sprintf("%s/api/v0/object/stat?arg=%s", s.baseURL, cid)
	body, err := s.doWithRetry(ctx, url)
	if err != nil {
		return 0, err
	}

	var result objectStatResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return 0, fmt.Errorf("ipfs: failed to parse object/stat response: %w", err)
	}

	return result.CumulativeSize, nil
}

// doWithRetry performs a POST request with retry logic for transient failures.
func (s *KuboIPFSService) doWithRetry(ctx context.Context, url string) ([]byte, error) {
	var lastErr error

	for attempt := 0; attempt <= s.maxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(s.retryDelay * time.Duration(attempt)):
			}
		}

		body, statusCode, err := s.doPost(ctx, url)
		if err != nil {
			lastErr = err
			continue
		}

		if statusCode >= 200 && statusCode < 300 {
			return body, nil
		}

		lastErr = fmt.Errorf("ipfs: request to %s returned status %d: %s", url, statusCode, string(body))

		// Don't retry client errors (4xx) — they won't succeed on retry
		if statusCode >= 400 && statusCode < 500 {
			return nil, lastErr
		}
	}

	return nil, lastErr
}

// doPost executes a single POST request and returns the response body and status code.
func (s *KuboIPFSService) doPost(ctx context.Context, url string) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return nil, 0, fmt.Errorf("ipfs: failed to create request: %w", err)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("ipfs: request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, 0, fmt.Errorf("ipfs: failed to read response: %w", err)
	}

	return body, resp.StatusCode, nil
}

// NodeInfoResult holds identity information returned by a kubo IPFS node.
type NodeInfoResult struct {
	PeerID          string
	AgentVersion    string
	ProtocolVersion string
}

// NodeInfo retrieves identity information from the IPFS node via POST /api/v0/id.
func (s *KuboIPFSService) NodeInfo(ctx context.Context) (*NodeInfoResult, error) {
	url := fmt.Sprintf("%s/api/v0/id", s.baseURL)
	body, err := s.doWithRetry(ctx, url)
	if err != nil {
		return nil, err
	}

	var result idResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("ipfs: failed to parse id response: %w", err)
	}

	return &NodeInfoResult{
		PeerID:          result.ID,
		AgentVersion:    result.AgentVersion,
		ProtocolVersion: result.ProtocolVersion,
	}, nil
}

// Kubo API response types.

type idResponse struct {
	ID              string `json:"ID"`
	AgentVersion    string `json:"AgentVersion"`
	ProtocolVersion string `json:"ProtocolVersion"`
}

type pinLsResponse struct {
	Keys map[string]pinLsEntry `json:"Keys"`
}

type pinLsEntry struct {
	Type string `json:"Type"`
}

type addResponse struct {
	Hash string `json:"Hash"`
	Size string `json:"Size"`
}

type objectStatResponse struct {
	Hash           string `json:"Hash"`
	NumLinks       int    `json:"NumLinks"`
	BlockSize      int64  `json:"BlockSize"`
	LinksSize      int64  `json:"LinksSize"`
	DataSize       int64  `json:"DataSize"`
	CumulativeSize int64  `json:"CumulativeSize"`
}
