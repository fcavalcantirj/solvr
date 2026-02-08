package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestClaimCommand_Exists(t *testing.T) {
	rootCmd := NewRootCmd()
	claimCmd, _, err := rootCmd.Find([]string{"claim"})
	if err != nil {
		t.Fatalf("claim command not found: %v", err)
	}
	if claimCmd == nil {
		t.Fatal("claim command is nil")
	}
}

func TestClaimCommand_RequiresAPIKey(t *testing.T) {
	// Use temp config dir with no API key
	tempDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tempDir)
	defer os.Setenv("HOME", origHome)

	// Ensure no config file exists
	configPath := filepath.Join(tempDir, ".solvr", "config")
	os.Remove(configPath)

	claimCmd := NewClaimCmd()
	buf := new(bytes.Buffer)
	claimCmd.SetOut(buf)
	claimCmd.SetErr(buf)

	err := claimCmd.Execute()
	if err == nil {
		t.Error("expected error when no API key configured")
	}
	if err.Error() != "API key not configured. Run 'solvr config set api-key <your-api-key>' first" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestClaimCommand_CallsAPI(t *testing.T) {
	// Setup temp config with API key
	tempDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tempDir)
	defer os.Setenv("HOME", origHome)

	configDir := filepath.Join(tempDir, ".solvr")
	os.MkdirAll(configDir, 0700)
	configPath := filepath.Join(configDir, "config")
	os.WriteFile(configPath, []byte("api-key=solvr_test_key_123\n"), 0600)

	// Create mock API server
	apiCalled := false
	var receivedAuthHeader string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiCalled = true
		receivedAuthHeader = r.Header.Get("Authorization")

		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/agents/me/claim" {
			t.Errorf("expected /agents/me/claim, got %s", r.URL.Path)
		}

		response := map[string]interface{}{
			"claim_url":    "https://solvr.dev/claim/abc123",
			"token":        "abc123",
			"expires_at":   "2026-02-08T22:00:00Z",
			"instructions": "Give this URL to your human to link your Solvr account.",
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	claimCmd := NewClaimCmd()
	buf := new(bytes.Buffer)
	claimCmd.SetOut(buf)
	claimCmd.SetErr(buf)
	claimCmd.Flags().Set("api-url", server.URL)

	err := claimCmd.Execute()
	if err != nil {
		t.Fatalf("claim command failed: %v", err)
	}

	if !apiCalled {
		t.Error("API was not called")
	}

	if receivedAuthHeader != "Bearer solvr_test_key_123" {
		t.Errorf("expected Bearer token, got %s", receivedAuthHeader)
	}

	output := buf.String()
	if !bytes.Contains([]byte(output), []byte("CLAIM YOUR AGENT")) {
		t.Error("output should contain 'CLAIM YOUR AGENT' header")
	}
	if !bytes.Contains([]byte(output), []byte("https://solvr.dev/claim/abc123")) {
		t.Error("output should contain claim URL")
	}
	if !bytes.Contains([]byte(output), []byte("abc123")) {
		t.Error("output should contain token")
	}
}

func TestClaimCommand_HandlesAPIError(t *testing.T) {
	// Setup temp config with API key
	tempDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tempDir)
	defer os.Setenv("HOME", origHome)

	configDir := filepath.Join(tempDir, ".solvr")
	os.MkdirAll(configDir, 0700)
	configPath := filepath.Join(configDir, "config")
	os.WriteFile(configPath, []byte("api-key=solvr_test_key\n"), 0600)

	// Create mock API server that returns an error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		response := map[string]interface{}{
			"error": map[string]interface{}{
				"code":    "INVALID_API_KEY",
				"message": "Invalid API key",
			},
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	claimCmd := NewClaimCmd()
	buf := new(bytes.Buffer)
	claimCmd.SetOut(buf)
	claimCmd.SetErr(buf)
	claimCmd.Flags().Set("api-url", server.URL)

	err := claimCmd.Execute()
	if err == nil {
		t.Error("expected error for unauthorized response")
	}
}

func TestClaimCommand_DisplaysFormattedOutput(t *testing.T) {
	// Setup temp config with API key
	tempDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tempDir)
	defer os.Setenv("HOME", origHome)

	configDir := filepath.Join(tempDir, ".solvr")
	os.MkdirAll(configDir, 0700)
	configPath := filepath.Join(configDir, "config")
	os.WriteFile(configPath, []byte("api-key=solvr_key\n"), 0600)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"claim_url":    "https://solvr.dev/claim/xyz789",
			"token":        "xyz789",
			"expires_at":   "2026-02-08T22:00:00Z",
			"instructions": "Share this with your human.",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	claimCmd := NewClaimCmd()
	buf := new(bytes.Buffer)
	claimCmd.SetOut(buf)
	claimCmd.SetErr(buf)
	claimCmd.Flags().Set("api-url", server.URL)

	err := claimCmd.Execute()
	if err != nil {
		t.Fatalf("claim command failed: %v", err)
	}

	output := buf.String()

	// Verify formatted output contains expected sections
	expectedStrings := []string{
		"=== CLAIM YOUR AGENT ===",
		"Claim URL:",
		"https://solvr.dev/claim/xyz789",
		"Token:",
		"xyz789",
		"Expires:",
	}

	for _, expected := range expectedStrings {
		if !bytes.Contains([]byte(output), []byte(expected)) {
			t.Errorf("output should contain '%s', got:\n%s", expected, output)
		}
	}
}
