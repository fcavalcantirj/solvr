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

// APIFailure is a failed API call: its status, the API's error code, message
// and request id, and the answer as it came. A stream the server ended (its
// access was revoked or its token rotated) has status 0.
type APIFailure struct {
	Status    int
	Code      string
	Message   string
	RequestID string
	Answer    []byte
}

func (e *APIFailure) Error() string {
	if e.Message == "" {
		return fmt.Sprintf("API returned status %d", e.Status)
	}
	if e.Code == "" {
		return "API error: " + e.Message
	}
	return fmt.Sprintf("API error: %s: %s", e.Code, e.Message)
}

// answer is the error as the API answers it: its own answer when it was the
// API's error envelope, else one with the status as the code.
func (e *APIFailure) answer() []byte {
	if e.Code != "" && json.Valid(e.Answer) {
		return e.Answer
	}
	answer, _ := json.Marshal(map[string]any{"error": map[string]string{
		"code": fmt.Sprintf("HTTP_%d", e.Status), "message": e.Error(),
	}})
	return answer
}

// callAPI sends one request and returns the response body of a 2xx reply. Any
// other status becomes an *APIFailure carrying the API's code and message.
func callAPI(method, url, credential string, payload any) ([]byte, error) {
	body, _, err := sendAPI(method, url, credential, nil, payload)
	return body, err
}

// sendAPI is callAPI with extra request headers (sent when not empty) that
// also returns the answer's headers. A nil payload sends no body.
func sendAPI(method, url, credential string, header map[string]string, payload any) ([]byte, http.Header, error) {
	var reqBody io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to encode request: %w", err)
		}
		reqBody = bytes.NewReader(encoded)
	}

	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create request: %w", err)
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if credential != "" {
		req.Header.Set("Authorization", "Bearer "+credential)
	}
	for name, value := range header {
		if value != "" {
			req.Header.Set(name, value)
		}
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to call API: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, resp.Header, apiErrorFrom(resp.StatusCode, body)
	}
	return body, resp.Header, nil
}

// apiErrorFrom turns an error response into an *APIFailure: "API error: CODE:
// message", or the bare status when the body is not the API's error envelope.
func apiErrorFrom(status int, body []byte) error {
	failure := &APIFailure{Status: status, Answer: body}
	var envelope struct {
		Error struct {
			Code      string `json:"code"`
			Message   string `json:"message"`
			RequestID string `json:"request_id"`
		} `json:"error"`
	}
	if json.Unmarshal(body, &envelope) == nil && envelope.Error.Message != "" {
		failure.Code = envelope.Error.Code
		failure.Message = envelope.Error.Message
		failure.RequestID = envelope.Error.RequestID
	}
	return failure
}

// printAnswer writes the API's answer as it came, indented.
func printAnswer(out io.Writer, body []byte) error {
	var indented bytes.Buffer
	if err := json.Indent(&indented, body, "", "  "); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}
	fmt.Fprintln(out, indented.String())
	return nil
}
