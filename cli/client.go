package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// resolveAPISettings fills the API key and URL from ~/.solvr/config when they
// were not given as flags (an explicit --api-url always wins).
func resolveAPISettings(apiURL, apiKey string) (string, string) {
	config, err := loadConfig()
	if err != nil {
		return apiURL, apiKey
	}
	if apiKey == "" {
		apiKey = config["api-key"]
	}
	if apiURL == defaultAPIURL {
		if url, ok := config["api-url"]; ok {
			apiURL = url
		}
	}
	return apiURL, apiKey
}

// callAPI sends one request and returns the response body of a 2xx reply. Any
// other status becomes an error carrying the API's code and message.
func callAPI(method, url, apiKey string, payload any) ([]byte, error) {
	var reqBody io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("failed to encode request: %w", err)
		}
		reqBody = bytes.NewReader(encoded)
	}

	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call API: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, apiErrorFrom(resp.StatusCode, body)
	}
	return body, nil
}

// apiErrorFrom turns an error response into "API error: CODE: message", or
// the bare status when the body is not the API's error envelope.
func apiErrorFrom(status int, body []byte) error {
	var apiErr APIError
	if json.Unmarshal(body, &apiErr) == nil && apiErr.Error.Message != "" {
		if apiErr.Error.Code != "" {
			return fmt.Errorf("API error: %s: %s", apiErr.Error.Code, apiErr.Error.Message)
		}
		return fmt.Errorf("API error: %s", apiErr.Error.Message)
	}
	return fmt.Errorf("API returned status %d", status)
}

// printJSON writes v as indented JSON.
func printJSON(out io.Writer, v any) {
	encoder := json.NewEncoder(out)
	encoder.SetIndent("", "  ")
	encoder.Encode(v)
}
