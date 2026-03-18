package services

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestResendClient_Send_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/emails" {
			t.Errorf("expected path /emails, got %s", r.URL.Path)
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed to read request body: %v", err)
		}

		var payload map[string]interface{}
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("failed to unmarshal request body: %v", err)
		}

		from, _ := payload["from"].(string)
		if !strings.Contains(from, "noreply@solvr.dev") {
			t.Errorf("expected from to contain noreply@solvr.dev, got %q", from)
		}

		to, _ := payload["to"].([]interface{})
		if len(to) == 0 || to[0] != "user@example.com" {
			t.Errorf("expected to[0] == user@example.com, got %v", to)
		}

		subject, _ := payload["subject"].(string)
		if subject != "Test Subject" {
			t.Errorf("expected subject == 'Test Subject', got %q", subject)
		}

		html, _ := payload["html"].(string)
		if html != "<p>Hello</p>" {
			t.Errorf("expected html == '<p>Hello</p>', got %q", html)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id": "test-email-id"}`))
	}))
	defer server.Close()

	client := NewResendClient("test-api-key", "noreply@solvr.dev")
	client.SetBaseURL(server.URL)

	ctx := context.Background()
	err := client.Send(ctx, "user@example.com", "Test Subject", "<p>Hello</p>", "Hello")
	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
}

func TestResendClient_Send_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"statusCode": 422, "name": "validation_error", "message": "Invalid email"}`))
	}))
	defer server.Close()

	client := NewResendClient("test-api-key", "noreply@solvr.dev")
	client.SetBaseURL(server.URL)

	ctx := context.Background()
	err := client.Send(ctx, "invalid", "Test", "<p>x</p>", "")
	if err == nil {
		t.Error("expected error, got nil")
	}

	errMsg := err.Error()
	if !strings.Contains(errMsg, "resend") && !strings.Contains(errMsg, "send") {
		t.Errorf("expected error to contain 'resend' or 'send', got: %q", errMsg)
	}
}

func TestResendClient_Send_EmptyTextBody(t *testing.T) {
	var capturedBody map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed to read request body: %v", err)
		}
		if err := json.Unmarshal(body, &capturedBody); err != nil {
			t.Fatalf("failed to unmarshal request body: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id": "test-id"}`))
	}))
	defer server.Close()

	client := NewResendClient("test-api-key", "noreply@solvr.dev")
	client.SetBaseURL(server.URL)

	ctx := context.Background()
	err := client.Send(ctx, "user@example.com", "Subject", "<p>HTML only</p>", "")
	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}

	// text field should be empty string or absent
	if textVal, ok := capturedBody["text"]; ok {
		if textStr, _ := textVal.(string); textStr != "" {
			t.Errorf("expected text field to be empty or absent, got: %q", textStr)
		}
	}
}

func TestResendClient_Send_CustomFromEmail(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed to read request body: %v", err)
		}

		var payload map[string]interface{}
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("failed to unmarshal request body: %v", err)
		}

		from, _ := payload["from"].(string)
		if !strings.Contains(from, "admin@solvr.dev") {
			t.Errorf("expected from to contain admin@solvr.dev, got %q", from)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id": "test-id"}`))
	}))
	defer server.Close()

	client := NewResendClient("key", "admin@solvr.dev")
	client.SetBaseURL(server.URL)

	ctx := context.Background()
	err := client.Send(ctx, "user@example.com", "Subject", "<p>Test</p>", "")
	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
}
